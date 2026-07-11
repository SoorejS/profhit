package controllers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// HyperVergeTokenResponse simulates/represents the token response
type HyperVergeTokenResponse struct {
	Result struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirectUrl"` // This is the verification url
	} `json:"result"`
	Status string `json:"status"`
}

// StartKYCSession initiates the KYC flow with HyperVerge and returns the verification URL.
func StartKYCSession(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	// 1. Check if user already has an active or verified KYC session
	var existingKYC models.HyperVergeKYC
	err := config.DB.Where("user_id = ?", userID).Order("created_at desc").First(&existingKYC).Error
	if err == nil {
		if existingKYC.Status == "Verified" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User is already verified"})
			return
		}
		if existingKYC.Status == "Started" || existingKYC.Status == "Pending" {
			// We could return the existing session ID, but it's safer to create a new one if it expired.
			// For simplicity, we just create a new one below.
		}
	}

	appID := os.Getenv("HYPERVERGE_API_KEY")
	appKey := os.Getenv("HYPERVERGE_API_SECRET")
	workflowID := os.Getenv("HYPERVERGE_WORKFLOW_ID")

	if appID == "" || appKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "HyperVerge credentials not configured"})
		return
	}

	// 2. Generate a unique transaction/session ID for our side
	transactionID := fmt.Sprintf("txn_%d_%d", userID, time.Now().UnixNano())

	// 3. Make server-to-server call to HyperVerge to get the token (Simulated or Real)
	// In a real integration, we'd POST to https://auth.hyperverge.co/login to get a JWT
	// Then POST to https://kyc-api.hyperverge.co/v1/transaction to create the transaction
	// For this code, we assume the URL and auth works as per HV documentation:
	
	payload := map[string]interface{}{
		"transactionId": transactionID,
		"workflowId":    workflowID,
	}
	bodyBytes, _ := json.Marshal(payload)

	hypervergeURL := os.Getenv("HYPERVERGE_API_URL")
	if hypervergeURL == "" {
		hypervergeURL = "https://vrs.hyperverge.co/api/generateToken"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var resp *http.Response
	var reqErr error

	// Exponential backoff retry loop (max 3 attempts)
	maxRetries := 3
	backoff := 500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		req, _ := http.NewRequest("POST", hypervergeURL, bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("appId", appID)
		req.Header.Set("appKey", appKey)

		resp, reqErr = client.Do(req)

		// Break out of retry if successful or a non-transient status code is received
		if reqErr == nil && resp.StatusCode < 500 {
			break
		}
		
		// If it's the last attempt, don't sleep
		if i == maxRetries-1 {
			break
		}

		// Close body to prevent connection leaks during retries
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		time.Sleep(backoff)
		backoff *= 2
	}

	var verificationURL string
	var sessionID = transactionID

	if reqErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to HyperVerge API"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "HyperVerge returned non-200 status"})
		return
	}

	var hvResp HyperVergeTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&hvResp); err == nil && hvResp.Status == "success" {
		verificationURL = hvResp.Result.RedirectURL
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate HyperVerge session"})
		return
	}

	// 4. Create the KYC record in DB
	newKyc := models.HyperVergeKYC{
		UserID:            userID,
		Status:            "Started",
		Provider:          "HyperVerge",
		ProviderReference: transactionID,
		SessionID:         sessionID,
		WorkflowID:        workflowID,
	}
	if err := config.DB.Create(&newKyc).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create KYC session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"verification_url": verificationURL,
		"session_id":       sessionID,
	})
}

// GetKYCStatus returns the current KYC status for the authenticated user
func GetKYCStatus(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var kyc models.HyperVergeKYC
	if err := config.DB.Where("user_id = ?", userID).Order("created_at desc").First(&kyc).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "Not Started"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": kyc.Status,
		"reason": kyc.FailureReason,
	})
}

// HypervergeWebhook handles the asynchronous verification result from HyperVerge
func HypervergeWebhook(c *gin.Context) {
	secret := os.Getenv("HYPERVERGE_WEBHOOK_SECRET")
	signatureHeader := c.GetHeader("x-hyperverge-signature")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
		return
	}

	// 1. Verify Signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// In simulator mode or real mode, signature must match
	if !hmac.Equal([]byte(signatureHeader), []byte(expectedSignature)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
		return
	}

	// 2. Parse Payload
	var payload struct {
		TransactionID string `json:"transactionId"`
		Status        string `json:"status"` // "auto_approved", "needs_review", "rejected"
		Details       struct {
			PanMasked    string  `json:"panMasked"`
			AadhaarLast4 string  `json:"aadhaarLast4"`
			FullName     string  `json:"fullName"`
			Dob          string  `json:"dob"`
			MatchScore   float64 `json:"matchScore"`
			Liveness     float64 `json:"livenessScore"`
		} `json:"details"`
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format"})
		return
	}

	// 3. Find the KYC record
	var kyc models.HyperVergeKYC
	if err := config.DB.Where("provider_reference = ?", payload.TransactionID).First(&kyc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Replay protection: if already verified, acknowledge but do not re-process
	if kyc.Status == "Verified" {
		c.JSON(http.StatusOK, gin.H{"message": "Already verified"})
		return
	}

	// 4. Update the record
	tx := config.DB.Begin()

	kyc.WebhookPayload = string(body)
	kyc.VerificationResult = payload.Status
	kyc.PANNumberMasked = payload.Details.PanMasked
	kyc.AadhaarLast4 = payload.Details.AadhaarLast4
	kyc.FullName = payload.Details.FullName
	kyc.DOB = payload.Details.Dob
	kyc.FaceMatchScore = payload.Details.MatchScore
	kyc.LivenessScore = payload.Details.Liveness
	kyc.FailureReason = payload.Reason

	var user models.User
	if err := tx.First(&user, kyc.UserID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	if payload.Status == "auto_approved" {
		kyc.Status = "Verified"
		now := time.Now()
		kyc.VerifiedAt = &now
		
		user.KycStatus = true
		if user.Tier == "Bronze" {
			user.Tier = "Gold" // Auto upgrade tier upon KYC
		}
		tx.Save(&user)

		// Trigger referral bonus for KYC completion
		_ = services.TriggerReferralEvent(user.ID, models.ReferralStatusKYCCompleted, 200)
	} else if payload.Status == "rejected" {
		kyc.Status = "Rejected"
	} else {
		kyc.Status = "Pending" // Needs manual review
	}

	if err := tx.Save(&kyc).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Webhook processed successfully"})
}

// SimulatorSignWebhook generates a valid HMAC signature for testing ONLY.
// In production, HyperVerge servers generate this signature using the secret.
func SimulatorSignWebhook(c *gin.Context) {
	// Only allow in beta mode
	if os.Getenv("BETA_MODE") != "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Simulator disabled in production"})
		return
	}

	body, _ := io.ReadAll(c.Request.Body)
	secret := os.Getenv("HYPERVERGE_WEBHOOK_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	c.JSON(http.StatusOK, gin.H{"signature": hex.EncodeToString(mac.Sum(nil))})
}

