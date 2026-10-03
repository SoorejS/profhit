package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"time"
)

// Editorial review cannot rewrite promises after any participation.
func UpdateMarketRules(c *gin.Context) {
	var input models.Market
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "Invalid market rules"})
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var m models.Market
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, c.Param("id")).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.PredictionSubmission{}).Where("market_id = ?", m.ID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 || m.WeeklyChallengeID != nil || (m.ResolutionStatus != "Draft" && m.ResolutionStatus != "Proposed") {
			return gorm.ErrInvalidData
		}
		originalID := m.ID
		newsURL := m.NewsURL
		published := m.NewsPublishedAt
		creator := m.CreatorID
		key := m.DailyKey
		input.ResolutionStatus = "Draft"
		if err := validateNewMarket(&input); err != nil {
			return err
		}
		if m.NewsEventID != nil {
			var event models.NewsEvent
			if err := tx.First(&event, *m.NewsEventID).Error; err != nil {
				return err
			}
			if err := services.ValidateNewsPrediction(event, &input, time.Now().UTC()); err != nil {
				return err
			}
		}
		input.ID = originalID
		input.NewsURL = newsURL
		input.NewsPublishedAt = published
		input.NewsEventID = m.NewsEventID
		input.NewsSourceName = m.NewsSourceName
		input.NewsEventTitle = m.NewsEventTitle
		input.NewsDiscoveredAt = m.NewsDiscoveredAt
		input.IsDemo = m.IsDemo
		input.ResultSpec = m.ResultSpec
		input.ResultApprovedBy = m.ResultApprovedBy
		if err := services.ConfigurePrediction(&input); err != nil {
			return err
		}
		input.CreatorID = creator
		input.DailyKey = key
		input.CreatedAt = m.CreatedAt
		return tx.Save(&input).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	services.LogAction(nil, c.MustGet("userID").(uint), "REVIEW_MARKET_RULES", "market_"+c.Param("id"), "Reviewed unpublished measurable rules", c.ClientIP())
	c.JSON(200, gin.H{"message": "Rules saved. Publish only after reviewing the source, wording, deadline and deterministic outcome."})
}
