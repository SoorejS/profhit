package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"regexp"
	"time"
)

type OrderRequest struct {
	Amount float64 `json:"amount" binding:"required"`
}

func CreateRazorpayOrder(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": "Coin purchases are disabled. PROPHIT predictions are free to play."})
}

type PaymentVerification struct {
	RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
	RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
	RazorpaySignature string `json:"razorpay_signature" binding:"required"`
	// Kept for older clients; never used to determine wallet credit.
	Points float64 `json:"points"`
}

var providerID = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func VerifyPayment(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": "Paid prediction currency is disabled; contact support for historical payment reconciliation"})
}
func RazorpayWebhook(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{"error": "Prediction coin payments are retired; reconcile historical payments outside the game economy"})
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

	if !services.RedemptionVerified(config.DB, userID, time.Now().UTC()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Redemption requires current annual KYC, phone OTP verification and email confirmation."})
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
	defer tx.Rollback()
	if err := services.LockWalletTx(tx, userID); err != nil {
		c.JSON(500, gin.H{"error": "Could not lock redemption account"})
		return
	}
	if !services.RedemptionVerified(tx, userID, time.Now().UTC()) {
		c.JSON(403, gin.H{"error": "Verification changed; re-verify before redeeming"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

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

	if err := services.DebitRedemptionTx(tx, user.ID, tierInfo.Coins, "tier", withdrawalReq.ID); err != nil {
		tx.Rollback()
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	services.BroadcastToUser(userID, "wallet_updated", "Voucher requested")
	c.JSON(http.StatusOK, gin.H{
		"message":       "Voucher requested. Expected delivery within 24–48 hours after verification and official sourcing; track the actual delivery status in your wallet.",
		"redemption_id": withdrawalReq.ID,
		"tier":          req.Tier,
		"voucher_value": tierInfo.Value,
	})
}
