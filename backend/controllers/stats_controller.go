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
		Where("is_active = ?", true).
		Order("points desc, id asc").
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
           AND m.visibility = 'Public'
           AND m.resolution_status NOT IN ('Draft', 'Proposed')
		ORDER BY ps.created_at DESC
		LIMIT 15
	`).Scan(&rows).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity"})
		return
	}

	var activity []gin.H
	now := time.Now().UTC()
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

// GetTopStreak fetches users ranked by their longest prediction streak
func GetTopStreak(c *gin.Context) {
	var streaks []models.PredictionStreak
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
			SUM(CASE WHEN ps.is_correct = true THEN 1 ELSE 0 END) * 100.0 / COUNT(ps.id) AS win_rate
		FROM users u
		JOIN prediction_submissions ps ON ps.user_id = u.id
		WHERE ps.deleted_at IS NULL AND u.deleted_at IS NULL AND u.is_active = true AND ps.is_correct IS NOT NULL
		GROUP BY u.id, u.username
		HAVING COUNT(ps.id) >= 10
		ORDER BY win_rate DESC, u.id ASC
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
	var currentUsername string
	if uid, exists := c.Get("userID"); exists {
		currentUserID = uid.(uint)
		if err := config.DB.Model(&models.User{}).Where("id = ?", currentUserID).Pluck("username", &currentUsername).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not load current leaderboard user"})
			return
		}
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
		var streaks []models.PredictionStreak
		query := config.DB.Model(&models.PredictionStreak{}).Preload("User").Joins("JOIN users ON users.id = prediction_streaks.user_id").Where("users.deleted_at IS NULL AND users.is_active = true")
		if search != "" {
			query = query.Where("users.username LIKE ?", "%"+search+"%")
		}

		if err := query.Count(&total).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not load leaderboard"})
			return
		}
		if err := query.Order("longest_streak DESC, users.id ASC").Limit(limit).Offset(offset).Find(&streaks).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not load leaderboard"})
			return
		}

		data := []gin.H{}
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
			var myStreak models.PredictionStreak
			if err := config.DB.Where("user_id = ?", currentUserID).First(&myStreak).Error; err == nil {
				var rank int64
				if err := config.DB.Model(&models.PredictionStreak{}).Joins("JOIN users ON users.id = prediction_streaks.user_id").Where("users.deleted_at IS NULL AND users.is_active = true").Where("longest_streak > ? OR (longest_streak = ? AND user_id < ?)", myStreak.LongestStreak, myStreak.LongestStreak, currentUserID).Count(&rank).Error; err != nil {
					c.JSON(500, gin.H{"error": "Could not load leaderboard rank"})
					return
				}
				response["current_user"] = gin.H{
					"id":             currentUserID,
					"username":       currentUsername,
					"longest_streak": myStreak.LongestStreak,
					"rank":           rank + 1,
				}
			}
		}

	} else {
		var users []models.User
		query := config.DB.Model(&models.User{}).Where("is_active = ?", true)
		if search != "" {
			query = query.Where("username LIKE ?", "%"+search+"%")
		}

		if sort == "winrate" {
			query = query.Where("id IN (SELECT user_id FROM prediction_submissions WHERE deleted_at IS NULL AND is_correct IS NOT NULL GROUP BY user_id HAVING COUNT(*) >= 10)")
			query = query.Order("win_rate DESC, id ASC")
		} else {
			query = query.Order("points DESC, id ASC")
		}

		if err := query.Count(&total).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not load leaderboard"})
			return
		}
		if err := query.Select("id, username, tier, points, win_rate, total_predictions").Limit(limit).Offset(offset).Find(&users).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not load leaderboard"})
			return
		}

		data := []gin.H{}
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
			if err := config.DB.Select("id, username, points, win_rate, total_predictions").First(&me, currentUserID).Error; err == nil {
				var rank int64
				if sort == "winrate" {
					var resolved int64
					if err := config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ? AND is_correct IS NOT NULL", currentUserID).Count(&resolved).Error; err != nil {
						c.JSON(500, gin.H{"error": "Could not load leaderboard rank"})
						return
					}
					if resolved >= 10 {
						if err := config.DB.Model(&models.User{}).Where("is_active = true AND id IN (SELECT user_id FROM prediction_submissions WHERE deleted_at IS NULL AND is_correct IS NOT NULL GROUP BY user_id HAVING COUNT(*) >= 10)").Where("win_rate > ? OR (win_rate = ? AND id < ?)", me.WinRate, me.WinRate, me.ID).Count(&rank).Error; err != nil {
							c.JSON(500, gin.H{"error": "Could not load leaderboard rank"})
							return
						}
						response["current_user"] = gin.H{
							"id":       currentUserID,
							"username": me.Username,
							"win_rate": me.WinRate,
							"rank":     rank + 1,
						}
					}
				} else {
					if err := config.DB.Model(&models.User{}).Where("is_active = true").Where("points > ? OR (points = ? AND id < ?)", me.Points, me.Points, me.ID).Count(&rank).Error; err != nil {
						c.JSON(500, gin.H{"error": "Could not load leaderboard rank"})
						return
					}
					response["current_user"] = gin.H{
						"id":       currentUserID,
						"username": me.Username,
						"points":   me.Points,
						"rank":     rank + 1,
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}
