package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
	"github.com/gin-gonic/gin"
)

// GetLeaderboard fetches top users ranked by Points
func GetLeaderboard(c *gin.Context) {
	var users []models.User
	
	// Fetch top 10 users ordered by points descending
	if err := config.DB.Order("points desc").Limit(10).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch leaderboard"})
		return
	}

	var leaderboard []gin.H
	for _, u := range users {
		leaderboard = append(leaderboard, gin.H{
			"id":       u.ID,
			"username": u.Username,
			"tier":     u.Tier,
			"points":   u.Points,
		})
	}

	c.JSON(http.StatusOK, leaderboard)
}

// GetActivity fetches the most recent trades across the platform
func GetActivity(c *gin.Context) {
	var trades []models.Trade
	
	// Fetch last 15 trades
	if err := config.DB.Order("created_at desc").Limit(15).Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity"})
		return
	}

	var activity []gin.H
	for _, t := range trades {
		// Fetch the associated market to get the title
		var market models.Market
		config.DB.First(&market, t.MarketID)
		
		// Fetch user
		var user models.User
		config.DB.First(&user, t.UserID)

		activity = append(activity, gin.H{
			"id":         t.ID,
			"username":   user.Username,
			"market":     market.Title,
			"outcome":    t.Outcome,
			"points":     t.Points,
			"time_ago":   time.Since(t.CreatedAt).String(),
		})
	}

	c.JSON(http.StatusOK, activity)
}
