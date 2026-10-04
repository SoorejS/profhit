package controllers

import (
	"fmt"
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
		sourceURL, sourceName, published, sourceKind := input.NewsURL, input.NewsSourceName, input.NewsPublishedAt, input.SourceKind
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
		} else if m.IsCurated {
			now := time.Now().UTC()
			if !services.PublicProviderURL(sourceURL) || sourceName == "" || len(sourceName) > 100 || (published != nil && published.After(now)) || (sourceKind != "article" && sourceKind != "official_event") || (sourceKind == "article" && (published == nil || published.Before(now.Add(-24*time.Hour)))) || (sourceKind == "official_event" && !services.ApprovedEditorialSource(input.Category, sourceURL)) {
				return fmt.Errorf("provide a current article or approved official event source")
			}
		}
		input.ID = originalID
		input.NewsURL = m.NewsURL
		input.NewsPublishedAt = m.NewsPublishedAt
		input.NewsEventID = m.NewsEventID
		input.NewsSourceName = m.NewsSourceName
		input.NewsEventTitle = m.NewsEventTitle
		input.NewsDiscoveredAt = m.NewsDiscoveredAt
		input.IsDemo = m.IsDemo
		input.IsCurated = m.IsCurated
		input.SourceKind = m.SourceKind
		if m.NewsEventID == nil && m.IsCurated {
			input.NewsURL, input.NewsSourceName, input.NewsPublishedAt, input.SourceKind = sourceURL, sourceName, published, sourceKind
		}
		input.ResultSpec = m.ResultSpec
		input.ResultApprovedBy = m.ResultApprovedBy
		if err := services.ConfigurePrediction(&input); err != nil {
			return err
		}
		input.CreatorID = creator
		input.DailyKey = key
		input.CreatedAt = m.CreatedAt
		if err := tx.Save(&input).Error; err != nil {
			return err
		}
		return services.LogAction(tx, c.MustGet("userID").(uint), "REVIEW_MARKET_RULES", "market_"+c.Param("id"), "Reviewed unpublished measurable rules", c.ClientIP())
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Rules saved. Publish only after reviewing the source, wording, deadline and deterministic outcome."})
}
