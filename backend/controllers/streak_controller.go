package controllers

import (
	"errors"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"time"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// DailyLoginResponse is what the client receives after a successful check-in.
type DailyLoginResponse struct {
	AlreadyCheckedIn bool   `json:"already_checked_in"`
	CoinsEarned      int    `json:"coins_earned"`
	CurrentStreak    int    `json:"current_streak"`
	NextMilestone    int    `json:"next_milestone"`
	NewBalance       int    `json:"new_balance"`
	Message          string `json:"message"`
}

// DailyLogin handles POST /api/me/daily-login
// It is idempotent — calling it multiple times on the same calendar day is safe.
func DailyLogin(c *gin.Context) {
	id := c.MustGet("userID").(uint)
	claimed, err := services.ClaimDailyLogin(id, time.Now().UTC())
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not award daily login"})
		return
	}
	var u models.User
	if config.DB.First(&u, id).Error != nil {
		c.JSON(500, gin.H{"error": "Could not load balance"})
		return
	}
	earned := 0
	message := "Daily login already rewarded today"
	if claimed {
		earned = 10
		message = "Daily login: +10 coins. Prediction streaks are separate."
	}
	c.JSON(200, DailyLoginResponse{AlreadyCheckedIn: !claimed, CoinsEarned: earned, NewBalance: u.Points, Message: message})
}

// GetStreakInfo handles GET /api/me/streak
func GetStreakInfo(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(uint)

	var streak models.PredictionStreak
	if err := config.DB.Where("user_id = ?", userID).First(&streak).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(500, gin.H{"error": "Could not load streak"})
			return
		}
		// No streak record yet
		c.JSON(http.StatusOK, gin.H{
			"current_streak":        0,
			"longest_streak":        0,
			"total_prediction_days": 0,
			"next_milestone":        models.StreakRewardTable[0].Milestone,
			"last_prediction_date":  nil,
		})
		return
	}
	current := streak.CurrentStreak
	if streak.LastPredictionDate.Before(time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -1)) {
		current = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"current_streak":        current,
		"longest_streak":        streak.LongestStreak,
		"total_prediction_days": streak.TotalDays,
		"next_milestone":        nextMilestone(streak.CurrentStreak),
		"last_prediction_date":  streak.LastPredictionDate,
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func sameDay(a, b time.Time) bool {
	a = a.UTC()
	b = b.UTC()
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func nextMilestone(current int) int {
	for _, r := range models.StreakRewardTable {
		if r.Milestone > current {
			return r.Milestone
		}
	}
	return 0
}

func buildMessage(streak, total, bonus int) string {
	if bonus > 0 {
		return "🔥 " + itoa(streak) + "-day streak! +" + itoa(total) + " coins earned!"
	}
	return "✅ Daily check-in complete! +" + itoa(total) + " coins"
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
