package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
	razorpay "github.com/razorpay/razorpay-go"
)

type OrderRequest struct {
	Amount float64 `json:"amount" binding:"required"` // In INR (rupees)
}

// CreateRazorpayOrder generates a unique Order ID to hand to the frontend checkout widget.
func CreateRazorpayOrder(c *gin.Context) {
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	keyID := os.Getenv("RAZORPAY_KEY_ID")
	keySecret := os.Getenv("RAZORPAY_KEY_SECRET")

	if keyID == "" || keySecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Razorpay credentials not configured"})
		return
	}

	client := razorpay.NewClient(keyID, keySecret)

	data := map[string]interface{}{
		"amount":   int(req.Amount * 100), // convert to paise
		"currency": "INR",
		"receipt":  "receipt_prophit_1",
	}

	body, err := client.Order.Create(data, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Razorpay order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id": body["id"],
		"amount":   body["amount"],
		"currency": body["currency"],
		"key":      keyID,
	})
}

type PaymentVerification struct {
	RazorpayPaymentID string  `json:"razorpay_payment_id" binding:"required"`
	RazorpayOrderID   string  `json:"razorpay_order_id" binding:"required"`
	RazorpaySignature string  `json:"razorpay_signature" binding:"required"`
	Points            float64 `json:"points" binding:"required"`
}

// VerifyPayment checks the signature, confirms payment success, and adds points.
func VerifyPayment(c *gin.Context) {
	var req PaymentVerification
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.MustGet("userID").(uint)
	keySecret := os.Getenv("RAZORPAY_KEY_SECRET")

	if keySecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Razorpay credentials not configured"})
		return
	}

	// Real HMAC verification
	data := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if expectedSignature != req.RazorpaySignature {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature. Payment failed."})
		return
	}

	// Verified! Perform Idempotent funding.
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check if this specific Razorpay Order ID has already been credited
	var existingTx models.PaymentTransaction
	if err := tx.Where("provider_order_id = ?", req.RazorpayOrderID).First(&existingTx).Error; err == nil {
		// ALREADY PROCESSED - Idempotent response
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{
			"message": "Payment already verified",
		})
		return
	}

	// Create a new record to prevent replay attacks
	newTx := models.PaymentTransaction{
		UserID:            userID,
		ProviderOrderID:   req.RazorpayOrderID,
		ProviderPaymentID: req.RazorpayPaymentID,
		Amount:            req.Points,
		Status:            "Completed",
	}

	if err := tx.Create(&newTx).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log payment transaction"})
		return
	}

	amount := int(req.Points)
	if amount <= 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount"})
		return
	}

	// Credit via the immutable ledger inside the transaction
	if err := services.CreditWalletTx(tx, userID, amount, models.TxTypePurchase, newTx.ID, "Wallet top-up via Razorpay", nil); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to credit wallet"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	// Trigger Referral Event for First Deposit asynchronously
	go services.TriggerReferralEvent(userID, models.ReferralStatusFirstDeposit, 100)

	// Fetch fresh balance to return accurate data
	var user models.User
	if err := config.DB.First(&user, userID).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Payment verified! Wallet funded.",
			"balance": user.Points,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Payment verified! Wallet funded."})
	}
}

// RazorpayWebhook processes asynchronous payment events
func RazorpayWebhook(c *gin.Context) {
	webhookSecret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	signature := c.GetHeader("X-Razorpay-Signature")

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Verify signature
	h := hmac.New(sha256.New, []byte(webhookSecret))
	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook signature"})
		return
	}

	// For Phase 2, we just acknowledge the webhook.
	// Production systems would parse the event JSON and update the DB accordingly.
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// RedeemVoucher allows users with KYC to exchange coins for Amazon vouchers
func RedeemVoucher(c *gin.Context) {
	var req struct {
		Tier string `json:"tier" binding:"required"` // e.g., "Bronze", "Silver", etc.
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.MustGet("userID").(uint)
	var user models.User

	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !user.KycStatus {
		c.JSON(http.StatusForbidden, gin.H{"error": "KYC verification is required before redeeming vouchers."})
		return
	}

	// Map tiers to required coins and voucher value (INR)
	tierMap := map[string]struct {
		Coins int
		Value int
	}{
		"Bronze":   {Coins: 500, Value: 50},
		"Silver":   {Coins: 1200, Value: 150},
		"Gold":     {Coins: 2500, Value: 350},
		"Platinum": {Coins: 5000, Value: 800},
		"Diamond":  {Coins: 10000, Value: 2000},
	}

	tierInfo, exists := tierMap[req.Tier]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid redemption tier"})
		return
	}

	if user.Points < tierInfo.Coins {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient coins for this tier"})
		return
	}

	// CRITICAL FIX: Run BOTH the coin debit AND the withdrawal record creation
	// inside the same transaction using DebitCoinsTx. If either step fails,
	// the entire operation rolls back — no coins lost, no ghost withdrawals.
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := services.DebitWalletTx(tx, user.ID, tierInfo.Coins, models.TxTypeRedemption, 0, "Redeemed "+req.Tier+" Voucher", nil); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct coins: " + err.Error()})
		return
	}

	withdrawalReq := models.WithdrawalRequest{
		UserID:        user.ID,
		Tier:          req.Tier,
		CoinsDeducted: tierInfo.Coins,
		Amount:        tierInfo.Value, // Voucher INR value
		Status:        "Pending",      // Awaiting admin to send the voucher code
	}

	if err := tx.Create(&withdrawalReq).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create redemption request"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Voucher redemption requested successfully! Code will be sent to your email within 24 hours.",
		"tier":          req.Tier,
		"voucher_value": tierInfo.Value,
	})
}
