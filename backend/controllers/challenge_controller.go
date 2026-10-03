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

func CreateWeeklyChallenge(c *gin.Context) {
	var req struct {
		MarketID uint `json:"market_id" binding:"required"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "Market ID is required"})
		return
	}
	var challenge models.WeeklyChallenge
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var market models.Market
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&market, req.MarketID).Error; err != nil {
			return err
		}
		if market.WeeklyChallengeID != nil || market.LockTime == nil || !market.LockTime.After(time.Now().UTC()) || market.Volume != 0 || market.PredictionType == "" || !services.ApprovedResultURL(market.Category, market.ResolutionSource) {
			return gorm.ErrInvalidData
		}
		if market.ResolutionStatus != "Live" && market.ResolutionStatus != "Scheduled" {
			return gorm.ErrInvalidData
		}
		var count int64
		if err := tx.Model(&models.PredictionSubmission{}).Where("market_id = ?", market.ID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return gorm.ErrInvalidData
		}
		start := time.Now().UTC()
		if market.StartTime != nil {
			start = *market.StartTime
		}
		if market.LockTime.After(start.AddDate(0, 0, 7)) {
			return gorm.ErrInvalidData
		}
		challenge = models.WeeklyChallenge{MarketID: &market.ID, Title: market.Title, Description: market.Description, StartDate: start, EndDate: *market.LockTime, Status: "Active", RewardPool: market.Payout * 2}
		if err := tx.Create(&challenge).Error; err != nil {
			return err
		}
		return tx.Model(&market).Update("weekly_challenge_id", challenge.ID).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"error": "Choose a published typed market with no predictions, a future cutoff within seven days, and an approved result source"})
		return
	}
	c.JSON(201, challenge)
}

func ListWeeklyChallenges(c *gin.Context) {
	var challenges []models.WeeklyChallenge
	if err := config.DB.Order("created_at desc").Limit(100).Find(&challenges).Error; err != nil {
		c.JSON(500, gin.H{"error": "Could not load challenges"})
		return
	}
	c.JSON(200, challenges)
}

func GetWeeklyChallenge(c *gin.Context) {
	var challenge models.WeeklyChallenge
	if config.DB.First(&challenge, c.Param("id")).Error != nil {
		c.JSON(404, gin.H{"error": "Challenge not found"})
		return
	}
	var scores []struct {
		Username  string `json:"username"`
		Score     int    `json:"score"`
		RewardWon int    `json:"reward_won"`
	}
	if err := config.DB.Table("challenge_participants").Select("users.username, challenge_participants.score, challenge_participants.reward_won").Joins("JOIN users ON users.id = challenge_participants.user_id").Where("challenge_id = ? AND users.deleted_at IS NULL", challenge.ID).Order("score desc, user_id asc").Limit(100).Scan(&scores).Error; err != nil {
		c.JSON(500, gin.H{"error": "Could not load results"})
		return
	}
	c.JSON(200, gin.H{"challenge": challenge, "leaderboard": scores})
}
