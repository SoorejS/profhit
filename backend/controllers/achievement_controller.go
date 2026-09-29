package controllers

import (
	"github.com/gin-gonic/gin"
	"profhit-backend/config"
)

// GetMyAchievements exposes only the authenticated user's earned achievements.
func GetMyAchievements(c *gin.Context) {
	type earned struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Reward      int    `json:"reward"`
	}
	items := []earned{}
	err := config.DB.Table("user_achievements ua").
		Select("a.title, a.description, a.reward").
		Joins("JOIN achievements a ON a.id = ua.achievement_id").
		Where("ua.user_id = ? AND a.deleted_at IS NULL", c.MustGet("userID")).
		Order("ua.unlocked_at DESC").Scan(&items).Error
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not load achievements"})
		return
	}
	c.JSON(200, items)
}
