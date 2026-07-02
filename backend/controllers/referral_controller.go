package controllers

import (
	"net/http"

	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetReferralAnalytics returns stats and history for the authenticated user's referrals.
func GetReferralAnalytics(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	// Get total referred users
	var referredUsers []models.User
	if err := config.DB.Where("referred_by = ?", userID).Select("id, username, created_at, kyc_status").Find(&referredUsers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch referred users"})
		return
	}

	// Get total earnings
	var totalEarnings int64
	config.DB.Model(&models.ReferralEvent{}).Where("referrer_id = ?", userID).Select("COALESCE(SUM(earnings), 0)").Row().Scan(&totalEarnings)

	// Get detailed events history
	var history []models.ReferralEvent
	if err := config.DB.Preload("Referred", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, username")
	}).Where("referrer_id = ?", userID).Order("created_at desc").Limit(50).Find(&history).Error; err != nil {
		// Just log and continue, we can return empty history
	}

	c.JSON(http.StatusOK, gin.H{
		"total_referred": len(referredUsers),
		"total_earnings": totalEarnings,
		"users":          referredUsers,
		"history":        history,
	})
}
