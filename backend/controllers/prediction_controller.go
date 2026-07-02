package controllers

import (
	"net/http"

	"profhit-backend/config"
	"profhit-backend/models"

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

	if market.ResolutionStatus != "Open" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This market is no longer accepting predictions"})
		return
	}

	// Validate the chosen option is one of the declared market options.
	// Prevents clients from injecting arbitrary strings into the choice field.
	if !isValidOption(req.Choice, market.Options) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid choice — not an option for this market"})
		return
	}

	// Attempt insert. If the unique index fires (duplicate), GORM returns an error.
	// This is safe under concurrent requests because the constraint is enforced by PostgreSQL.
	prediction := models.PredictionSubmission{
		UserID:    userID,
		MarketID:  req.MarketID,
		Choice:    req.Choice,
		Potential: market.Payout,
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&prediction).Error; err != nil {
		tx.Rollback()
		// Unique constraint violation → user already predicted
		c.JSON(http.StatusConflict, gin.H{"error": "You have already placed a prediction on this market"})
		return
	}

	// Increment volume counter atomically inside the same transaction
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

	// Broadcast to live clients
	BroadcastUpdate("trade_placed", gin.H{"market_id": market.ID})

	c.JSON(http.StatusOK, gin.H{
		"message":            "Prediction locked! Good luck 🎯",
		"choice":             prediction.Choice,
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
