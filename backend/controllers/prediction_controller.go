package controllers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubmitPrediction handles POST /api/predictions
// The composite UNIQUE DB index on (user_id, market_id) prevents duplicate predictions
// even under concurrent requests — eliminating the application-layer race condition.
func SubmitPrediction(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req struct {
		MarketID uint   `json:"market_id" binding:"required"`
		Choice   string `json:"choice" binding:"required"`
		Amount   int    `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Amount != 0 {
		c.JSON(400, gin.H{"error": "Predictions are free; no stake or payment is accepted"})
		return
	}

	tx := config.DB.Begin()
	defer tx.Rollback()
	var market models.Market
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&market, req.MarketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	acceptableStatuses := map[string]bool{"Open": true, "Live": true}
	if !acceptableStatuses[market.ResolutionStatus] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This market is no longer accepting predictions"})
		return
	}

	now := time.Now().UTC()
	if (market.LockTime != nil && !now.Before(*market.LockTime)) || (!market.EndDate.IsZero() && !now.Before(market.EndDate)) || (market.StartTime != nil && now.Before(*market.StartTime)) {
		c.JSON(400, gin.H{"error": "Market is outside its prediction window"})
		return
	}
	if market.Payout <= 0 {
		c.JSON(400, gin.H{"error": "Market payout is not configured"})
		return
	}
	checked := market
	if market.PredictionType == "" || services.ConfigurePrediction(&checked) != nil || checked.Payout != market.Payout {
		c.JSON(409, gin.H{"error": "Market rules require editorial reconciliation before new predictions"})
		return
	}
	if err := services.LockReferralWalletsTx(tx, userID); err != nil {
		c.JSON(500, gin.H{"error": "Could not lock wallet"})
		return
	}
	// ── PDF §4.3: One prediction per topic/category per day ─────────────────
	// A user may only predict on one market per category per calendar day.
	todayStart := now.Truncate(24 * time.Hour)
	tomorrowStart := todayStart.Add(24 * time.Hour)

	var topicCount int64
	result := tx.Raw(`
		SELECT COUNT(ps.id)
		FROM prediction_submissions ps
		INNER JOIN markets m ON m.id = ps.market_id
		WHERE ps.user_id = ?
		  AND m.category = ?
		  AND ps.created_at >= ?
		  AND ps.created_at < ?
		  AND ps.deleted_at IS NULL
		  AND m.deleted_at IS NULL
	`, userID, market.Category, todayStart, tomorrowStart).Scan(&topicCount)
	if result.Error != nil {
		c.JSON(500, gin.H{"error": "Could not check prediction limit"})
		return
	}

	if topicCount > 0 {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "You have already placed a prediction in the '" + market.Category + "' category today. Try again tomorrow!",
		})
		return
	}

	// Validate the chosen option is one of the declared market options.
	canonical, err := services.ValidatePredictionValue(market, req.Choice, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Choice = canonical

	prediction := models.PredictionSubmission{
		UserID:    userID,
		MarketID:  req.MarketID,
		Choice:    req.Choice,
		Amount:    req.Amount,
		Potential: market.Payout,
		CreatedAt: now,
	}
	if market.WeeklyChallengeID != nil {
		var challenge models.WeeklyChallenge
		if tx.First(&challenge, *market.WeeklyChallengeID).Error != nil || challenge.Status != "Active" || now.Before(challenge.StartDate) || !now.Before(challenge.EndDate) {
			c.JSON(400, gin.H{"error": "Challenge is outside its participation window"})
			return
		}
		prediction.Potential = market.Payout * 2
		if err := tx.Create(&models.ChallengeParticipant{ChallengeID: challenge.ID, UserID: userID}).Error; err != nil {
			c.JSON(409, gin.H{"error": "Challenge participation already recorded"})
			return
		}
	}

	// Participation is free. No balance check, coin debit, or money provider call.

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

	if err := services.TriggerReferralEventTx(tx, userID, models.ReferralStatusFirstBet, 50); err != nil {
		c.JSON(500, gin.H{"error": "Could not record prediction reward"})
		return
	}
	if err := services.RecordPredictionDayTx(tx, userID, now); err != nil {
		c.JSON(500, gin.H{"error": "Could not record prediction day"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	// Trigger Referral Event for First Prediction

	// Trigger Gamification Hooks
	services.CheckPredictionAchievements(userID)

	// Broadcast to live clients
	if market.Visibility == "Public" {
		services.BroadcastToAll("prediction_count_changed", gin.H{"market_id": market.ID})
		services.BroadcastToAll("market_activity_changed", gin.H{"market_id": market.ID})
	}
	services.BroadcastToUser(userID, "wallet_updated", gin.H{"user_id": userID})
	services.BroadcastToAll("leaderboard_updated", gin.H{"market_id": market.ID})

	c.JSON(http.StatusOK, gin.H{
		"message":          "Prediction locked! Good luck 🎯",
		"choice":           prediction.Choice,
		"staked":           prediction.Amount,
		"potential_payout": prediction.Potential,
		"market_title":     market.Title,
	})
}

// GetUserPredictions returns the authenticated user's raw prediction history.
// For the enriched view (with market title, status), use GET /portfolio.
func GetUserPredictions(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "20")))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count predictions"})
		return
	}

	var predictions []models.PredictionSubmission
	if err := query.
		Order("created_at desc, id desc").
		Limit(pageSize).
		Offset(offset).
		Find(&predictions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load predictions"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"items":       predictions,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

// isValidOption checks whether `choice` appears in the market's JSON options string.
func isValidOption(choice, optionsJSON string) bool {
	return len(choice) > 0 && containsOption(optionsJSON, choice)
}

func containsOption(raw, opt string) bool {
	var options []string
	if json.Unmarshal([]byte(raw), &options) != nil {
		return false
	}
	for _, choice := range options {
		if choice == opt {
			return true
		}
	}
	return false
}
