package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	razorpay "github.com/razorpay/razorpay-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"net/http"
	"os"
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
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount < 10 || req.Amount > 100000 || math.Trunc(req.Amount) != req.Amount {
		c.JSON(400, gin.H{"error": "Amount must be a whole number of INR between 10 and 100000"})
		return
	}
	key, secret := os.Getenv("RAZORPAY_KEY_ID"), os.Getenv("RAZORPAY_KEY_SECRET")
	if key == "" || secret == "" {
		c.JSON(503, gin.H{"error": "Payments are not configured"})
		return
	}
	userID := c.MustGet("userID").(uint)
	amount := int(req.Amount) * 100
	body, err := razorpay.NewClient(key, secret).Order.Create(map[string]interface{}{"amount": amount, "currency": "INR", "receipt": fmt.Sprintf("r_%d_%d", userID, time.Now().UnixNano())}, nil)
	if err != nil {
		c.JSON(502, gin.H{"error": "Payment provider could not create an order"})
		return
	}
	orderID, ok := body["id"].(string)
	if !ok || orderID == "" {
		c.JSON(502, gin.H{"error": "Invalid payment provider response"})
		return
	}
	order := models.PaymentTransaction{UserID: userID, ProviderOrderID: orderID, Amount: req.Amount, AmountPaise: amount, Status: "Pending"}
	if err := config.DB.Create(&order).Error; err != nil {
		c.JSON(500, gin.H{"error": "Could not record payment order"})
		return
	}
	c.JSON(200, gin.H{"order_id": orderID, "amount": amount, "currency": "INR", "key": key})
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
	var req PaymentVerification
	if err := c.ShouldBindJSON(&req); err != nil || !providerID.MatchString(req.RazorpayPaymentID) || !providerID.MatchString(req.RazorpayOrderID) {
		c.JSON(400, gin.H{"error": "Invalid payment details"})
		return
	}
	secret := os.Getenv("RAZORPAY_KEY_SECRET")
	if secret == "" {
		c.JSON(503, gin.H{"error": "Payments are not configured"})
		return
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(req.RazorpayOrderID + "|" + req.RazorpayPaymentID))
	signature, err := hex.DecodeString(req.RazorpaySignature)
	if err != nil || !hmac.Equal(h.Sum(nil), signature) {
		c.JSON(400, gin.H{"error": "Invalid payment signature"})
		return
	}
	userID := c.MustGet("userID").(uint)
	var order models.PaymentTransaction
	if err := config.DB.Where("provider_order_id = ? AND user_id = ?", req.RazorpayOrderID, userID).First(&order).Error; err != nil {
		c.JSON(404, gin.H{"error": "Payment order not found"})
		return
	}
	if order.Status == "Completed" {
		if order.ProviderPaymentID != req.RazorpayPaymentID {
			c.JSON(409, gin.H{"error": "Order already paid by another payment"})
			return
		}
		c.JSON(200, gin.H{"message": "Payment already verified"})
		return
	}
	payment, err := razorpay.NewClient(os.Getenv("RAZORPAY_KEY_ID"), secret).Payment.Fetch(req.RazorpayPaymentID, nil, nil)
	if err != nil {
		c.JSON(502, gin.H{"error": "Could not verify payment with provider; retry verification"})
		return
	}
	amount, ok := payment["amount"].(float64)
	if !ok || amount != float64(order.AmountPaise) || payment["currency"] != "INR" || payment["status"] != "captured" || payment["order_id"] != req.RazorpayOrderID || payment["id"] != req.RazorpayPaymentID {
		c.JSON(400, gin.H{"error": "Payment is not captured or does not match the order"})
		return
	}
	if err := settlePayment(req.RazorpayOrderID, req.RazorpayPaymentID, int(amount)); err != nil {
		c.JSON(500, gin.H{"error": "Could not settle payment; retry verification"})
		return
	}
	c.JSON(200, gin.H{"message": "Payment verified! Wallet funded."})
}

// Both provider channels converge on the same locked, atomic settlement.
func settlePayment(orderID, paymentID string, amountPaise int) error {
	return config.DB.Transaction(func(tx *gorm.DB) error {
		var order models.PaymentTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("provider_order_id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		if order.Status == "Completed" {
			if order.ProviderPaymentID != paymentID {
				return fmt.Errorf("conflicting payment")
			}
			return nil
		}
		if order.Status != "Pending" || order.AmountPaise != amountPaise || amountPaise < 1000 || amountPaise%100 != 0 {
			return fmt.Errorf("invalid payment amount or state")
		}
		if err := services.LockReferralWalletsTx(tx, order.UserID); err != nil {
			return err
		}
		if err := services.CreditWalletTx(tx, order.UserID, amountPaise/100, models.TxTypePurchase, order.ID, "Wallet top-up via Razorpay", nil); err != nil {
			return err
		}
		if err := tx.Model(&order).Updates(map[string]interface{}{"status": "Completed", "provider_payment_id": paymentID}).Error; err != nil {
			return err
		}
		return services.TriggerReferralEventTx(tx, order.UserID, models.ReferralStatusFirstDeposit, 100)
	})
}

func RazorpayWebhook(c *gin.Context) {
	secret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	if secret == "" {
		c.JSON(503, gin.H{"error": "Payment webhooks are not configured"})
		return
	}
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	signature, err := hex.DecodeString(c.GetHeader("X-Razorpay-Signature"))
	if err != nil || !hmac.Equal(h.Sum(nil), signature) {
		c.JSON(400, gin.H{"error": "Invalid webhook signature"})
		return
	}
	var event struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID       string `json:"id"`
					OrderID  string `json:"order_id"`
					Amount   int    `json:"amount"`
					Currency string `json:"currency"`
					Status   string `json:"status"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}
	if event.Event == "payment.captured" || event.Event == "order.paid" {
		p := event.Payload.Payment.Entity
		if p.Status != "captured" || p.Currency != "INR" || p.OrderID == "" || p.ID == "" {
			c.JSON(400, gin.H{"error": "Invalid captured payment"})
			return
		}
		if err := settlePayment(p.OrderID, p.ID, p.Amount); err != nil {
			c.JSON(500, gin.H{"error": "Payment could not be settled; retry webhook"})
			return
		}
	}
	c.JSON(200, gin.H{"status": "ok"})
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
			panic(r)
		}
	}()

	if err := services.DebitWalletTx(tx, user.ID, tierInfo.Coins, models.TxTypeRedemption, 0, "Redeemed "+req.Tier+" Voucher", nil); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Insufficient unexpired balance or wallet unavailable"})
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
