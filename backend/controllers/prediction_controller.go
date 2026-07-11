package controllers

import (
	"net/http"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SubmitPrediction handles POST /api/predictions
// The composite UNIQUE DB index on (user_id, market_id) prevents duplicate predictions
// even under concurrent requests — eliminating the application-layer race condition.
func SubmitPrediction(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req struct {
		MarketID uint   `json:"market_id" binding:"required"`
		Choice   string `json:"choice" binding:"required"`
		Amount   int    `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var market models.Market
	if err := config.DB.First(&market, req.MarketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	acceptableStatuses := map[string]bool{"Open": true, "Live": true, "Scheduled": true}
	if !acceptableStatuses[market.ResolutionStatus] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This market is no longer accepting predictions"})
		return
	}

	// Validate the chosen option is one of the declared market options.
	if !isValidOption(req.Choice, market.Options) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid choice — not an option for this market"})
		return
	}

	prediction := models.PredictionSubmission{
		UserID:    userID,
		MarketID:  req.MarketID,
		Choice:    req.Choice,
		Amount:    req.Amount,
		Potential: market.Payout,
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Deduct coins from wallet first (safe from race conditions due to row lock)
	if err := services.DebitWalletTx(tx, userID, req.Amount, models.TxTypePredictionStake, market.ID, "Staked on market: "+market.Title, nil); err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance or wallet error"})
		return
	}

	// 2. Create the prediction record
	if err := tx.Create(&prediction).Error; err != nil {
		tx.Rollback()
		// Unique constraint violation → user already predicted
		c.JSON(http.StatusConflict, gin.H{"error": "You have already placed a prediction on this market"})
		return
	}

	// 3. Increment market volume
	if err := tx.Model(&models.Market{}).Where("id = ?", market.ID).
		UpdateColumn("volume", gorm.Expr("volume + 1")).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update market volume"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	// Trigger Referral Event for First Prediction
	go services.TriggerReferralEvent(userID, models.ReferralStatusFirstBet, 50)

	// Trigger Gamification Hooks
	go services.CheckPredictionAchievements(userID)

	// Broadcast to live clients
	services.BroadcastToAll("trade_placed", gin.H{"market_id": market.ID})

	c.JSON(http.StatusOK, gin.H{
		"message":            "Prediction locked! Good luck 🎯",
		"choice":             prediction.Choice,
		"staked":             prediction.Amount,
		"potential_payout":   prediction.Potential,
		"market_title":       market.Title,
	})
}

// GetUserPredictions returns the authenticated user's raw prediction history.
// For the enriched view (with market title, status), use GET /portfolio.
func GetUserPredictions(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var predictions []models.PredictionSubmission
	if err := config.DB.
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&predictions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load predictions"})
		return
	}

	c.JSON(http.StatusOK, predictions)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// isValidOption checks whether `choice` appears in the market's JSON options string.
func isValidOption(choice, optionsJSON string) bool {
	return len(choice) > 0 && containsOption(optionsJSON, choice)
}

func containsOption(json, opt string) bool {
	needle := `"` + opt + `"`
	for i := 0; i <= len(json)-len(needle); i++ {
		if json[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
