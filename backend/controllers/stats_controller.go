package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"strconv"
	"time"
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

// GetUnifiedLeaderboard returns a paginated leaderboard with dynamic sorting
func GetUnifiedLeaderboard(c *gin.Context) {
	sort := c.DefaultQuery("sort", "points")
	search := c.Query("search")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	var total int64
	var currentUserID uint
	if uid, exists := c.Get("userID"); exists {
		currentUserID = uid.(uint)
	}

	response := gin.H{
		"data": []gin.H{},
		"meta": gin.H{
			"page":  page,
			"limit": limit,
			"total": 0,
		},
		"current_user": nil,
	}

	if sort == "streak" {
		var streaks []models.UserStreak
		query := config.DB.Model(&models.UserStreak{}).Preload("User").Joins("JOIN users ON users.id = user_streaks.user_id").Where("users.deleted_at IS NULL")
		if search != "" {
			query = query.Where("users.username LIKE ?", "%"+search+"%")
		}

		query.Count(&total)
		query.Order("longest_streak DESC, users.created_at ASC").Limit(limit).Offset(offset).Find(&streaks)

		var data []gin.H
		for i, s := range streaks {
			if s.User.ID != 0 {
				data = append(data, gin.H{
					"id":             s.User.ID,
					"username":       s.User.Username,
					"longest_streak": s.LongestStreak,
					"rank":           offset + i + 1,
				})
			}
		}
		response["data"] = data
		response["meta"].(gin.H)["total"] = total

		if currentUserID != 0 {
			var myStreak models.UserStreak
			if err := config.DB.Where("user_id = ?", currentUserID).First(&myStreak).Error; err == nil {
				var rank int64
				config.DB.Model(&models.UserStreak{}).Where("longest_streak > ?", myStreak.LongestStreak).Count(&rank)
				response["current_user"] = gin.H{
					"id":             currentUserID,
					"longest_streak": myStreak.LongestStreak,
					"rank":           rank + 1,
				}
			}
		}

	} else {
		var users []models.User
		query := config.DB.Model(&models.User{})
		if search != "" {
			query = query.Where("username LIKE ?", "%"+search+"%")
		}

		if sort == "winrate" {
			query = query.Where("total_predictions >= 10")
			query.Order("win_rate DESC, created_at ASC")
		} else {
			query.Order("points DESC, created_at ASC")
		}

		query.Count(&total)
		query.Select("id, username, tier, points, win_rate, total_predictions").Limit(limit).Offset(offset).Find(&users)

		var data []gin.H
		for i, u := range users {
			data = append(data, gin.H{
				"id":                u.ID,
				"username":          u.Username,
				"tier":              u.Tier,
				"points":            u.Points,
				"win_rate":          u.WinRate,
				"total_predictions": u.TotalPredictions,
				"rank":              offset + i + 1,
			})
		}
		if data == nil {
			data = []gin.H{}
		}
		response["data"] = data
		response["meta"].(gin.H)["total"] = total

		if currentUserID != 0 {
			var me models.User
			if err := config.DB.Select("id, points, win_rate, total_predictions").First(&me, currentUserID).Error; err == nil {
				var rank int64
				if sort == "winrate" {
					if me.TotalPredictions >= 10 {
						config.DB.Model(&models.User{}).Where("total_predictions >= 10 AND win_rate > ?", me.WinRate).Count(&rank)
						response["current_user"] = gin.H{
							"id":       currentUserID,
							"win_rate": me.WinRate,
							"rank":     rank + 1,
						}
					}
				} else {
					config.DB.Model(&models.User{}).Where("points > ?", me.Points).Count(&rank)
					response["current_user"] = gin.H{
						"id":     currentUserID,
						"points": me.Points,
						"rank":   rank + 1,
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}
