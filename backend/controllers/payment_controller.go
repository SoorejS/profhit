package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"profhit-backend/config"
	"profhit-backend/models"

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

	// Fallback/Mock behavior if keys aren't provided
	if keyID == "" || keySecret == "" {
		c.JSON(http.StatusOK, gin.H{
			"order_id": "order_mock_123456",
			"amount":   req.Amount * 100, // paise
			"currency": "INR",
			"key":      "mock_key_123",
		})
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

	// If using mock order, blindly accept
	if keySecret == "" && req.RazorpayOrderID == "order_mock_123456" {
		addFunds(userID, req.Points, c)
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

	// Verified! Add funds to wallet.
	addFunds(userID, req.Points, c)
}

func addFunds(userID uint, points float64, c *gin.Context) {
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.Points += int(points)
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment verified! Wallet funded.",
		"balance": user.Points,
	})
}

// WithdrawFunds processes a user's request to cash out points to INR
func WithdrawFunds(c *gin.Context) {
	var req struct {
		Amount float64 `json:"amount" binding:"required"`
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

	if float64(user.Points) < req.Amount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient points balance"})
		return
	}

	// Deduct points immediately so they can't double-spend while pending
	tx := config.DB.Begin()
	user.Points -= int(req.Amount)
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process withdrawal"})
		return
	}

	withdrawalReq := models.WithdrawalRequest{
		UserID: user.ID,
		Amount: int(req.Amount),
		Status: "Pending",
	}

	if err := tx.Create(&withdrawalReq).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create withdrawal request"})
		return
	}
	
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message": "Withdrawal requested! Pending Admin approval.",
		"balance": user.Points,
	})
}
