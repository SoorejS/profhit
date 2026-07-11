package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
	"github.com/gin-gonic/gin"
)

// GetLeaderboard fetches top 10 users ranked by Points
func GetLeaderboard(c *gin.Context) {
	var users []models.User

	if err := config.DB.
		Select("id, username, tier, points").
		Order("points desc").
		Limit(10).
		Find(&users).Error; err != nil {
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

	if leaderboard == nil {
		leaderboard = []gin.H{}
	}
	c.JSON(http.StatusOK, leaderboard)
}

// GetActivity fetches the 15 most recent predictions using a single JOIN query.
// Fixes the N+1 pattern where each prediction triggered separate market and user lookups.
func GetActivity(c *gin.Context) {
	type ActivityRow struct {
		ID        uint      `json:"id"`
		Username  string    `json:"username"`
		Market    string    `json:"market"`
		Choice    string    `json:"choice"`
		Potential int       `json:"potential"`
		CreatedAt time.Time `json:"created_at"`
	}

	var rows []ActivityRow

	err := config.DB.Raw(`
		SELECT
			ps.id,
			u.username,
			m.title     AS market,
			ps.choice,
			ps.potential,
			ps.created_at
		FROM prediction_submissions ps
		INNER JOIN users  u ON u.id  = ps.user_id
		INNER JOIN markets m ON m.id = ps.market_id
		WHERE ps.deleted_at IS NULL
		  AND u.deleted_at  IS NULL
		  AND m.deleted_at  IS NULL
		ORDER BY ps.created_at DESC
		LIMIT 15
	`).Scan(&rows).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity"})
		return
	}

	var activity []gin.H
	now := time.Now()
	for _, r := range rows {
		duration := now.Sub(r.CreatedAt)
		var timeAgo string
		switch {
		case duration.Hours() >= 24:
			timeAgo = itoa(int(duration.Hours()/24)) + "d"
		case duration.Hours() >= 1:
			timeAgo = itoa(int(duration.Hours())) + "h"
		case duration.Minutes() >= 1:
			timeAgo = itoa(int(duration.Minutes())) + "m"
		default:
			timeAgo = itoa(int(duration.Seconds())) + "s"
		}

		activity = append(activity, gin.H{
			"id":        r.ID,
			"username":  r.Username,
			"market":    r.Market,
			"choice":    r.Choice,
			"potential": r.Potential,
			"time_ago":  timeAgo,
		})
	}

	if activity == nil {
		activity = []gin.H{}
	}
	c.JSON(http.StatusOK, activity)
}

// GetTopStreak fetches users ranked by their longest daily login streak
func GetTopStreak(c *gin.Context) {
	var streaks []models.UserStreak
	if err := config.DB.Preload("User").Order("longest_streak desc").Limit(10).Find(&streaks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch streak leaderboard"})
		return
	}

	var leaderboard []gin.H
	for _, s := range streaks {
		if s.User.ID != 0 {
			leaderboard = append(leaderboard, gin.H{
				"id":             s.User.ID,
				"username":       s.User.Username,
				"longest_streak": s.LongestStreak,
			})
		}
	}
	if leaderboard == nil {
		leaderboard = []gin.H{}
	}
	c.JSON(http.StatusOK, leaderboard)
}

// GetTopWinRate fetches users ranked by prediction win rate (min 10 predictions)
func GetTopWinRate(c *gin.Context) {
	type WinRateRow struct {
		ID       uint    `json:"id"`
		Username string  `json:"username"`
		WinRate  float64 `json:"win_rate"`
		Total    int     `json:"total_predictions"`
	}

	var rows []WinRateRow
	err := config.DB.Raw(`
		SELECT
			u.id,
			u.username,
			COUNT(ps.id) AS total,
			(SUM(CASE WHEN ps.is_correct = true THEN 1 ELSE 0 END)::float / COUNT(ps.id)::float) * 100 AS win_rate
		FROM users u
		JOIN prediction_submissions ps ON ps.user_id = u.id
		WHERE ps.deleted_at IS NULL AND u.deleted_at IS NULL
		GROUP BY u.id, u.username
		HAVING COUNT(ps.id) >= 10
		ORDER BY win_rate DESC
		LIMIT 10
	`).Scan(&rows).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch win rate leaderboard"})
		return
	}

	if rows == nil {
		rows = []WinRateRow{}
	}
	c.JSON(http.StatusOK, rows)
}
