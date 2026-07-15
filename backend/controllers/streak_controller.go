package controllers

import (
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
	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(uint)

	today := truncateToDay(time.Now())

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Upsert the streak row with FOR UPDATE lock
	var streak models.UserStreak
	result := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ?", userID).First(&streak)

	if result.Error != nil {
		// First-ever login — create a fresh streak record
		streak = models.UserStreak{
			UserID:        userID,
			CurrentStreak: 0,
			LongestStreak: 0,
			LastLoginDate: time.Time{}, // zero value
			TotalLogins:   0,
		}
	}

	// Already checked in today → return early
	if sameDay(streak.LastLoginDate, today) {
		tx.Rollback() // Rollback transaction as no writes are needed
		var user models.User
		config.DB.First(&user, userID)
		c.JSON(http.StatusOK, DailyLoginResponse{
			AlreadyCheckedIn: true,
			CoinsEarned:      0,
			CurrentStreak:    streak.CurrentStreak,
			NextMilestone:    nextMilestone(streak.CurrentStreak),
			NewBalance:       user.Points,
			Message:          "Already checked in today!",
		})
		return
	}

	// Determine new streak value
	yesterday := today.AddDate(0, 0, -1)
	if sameDay(streak.LastLoginDate, yesterday) {
		streak.CurrentStreak++
	} else {
		streak.CurrentStreak = 1 // Streak broken — reset
	}
	if streak.CurrentStreak > streak.LongestStreak {
		streak.LongestStreak = streak.CurrentStreak
	}
	streak.LastLoginDate = today
	streak.TotalLogins++

	// Save streak
	if result.Error != nil {
		if err := tx.Create(&streak).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create streak"})
			return
		}
	} else {
		if err := tx.Save(&streak).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update streak"})
			return
		}
	}

	// --- Award coins ---
	totalCoins := models.DailyLoginBaseCoins
	streakBonus := 0

	for _, reward := range models.StreakRewardTable {
		if streak.CurrentStreak == reward.Milestone {
			streakBonus = reward.Coins
			break
		}
	}

	// Base daily login coins
	_ = services.CreditWalletTx(tx, userID, models.DailyLoginBaseCoins,
		models.TxTypeDailyLogin, 0,
		"Daily login reward", nil)

	// Streak bonus coins
	if streakBonus > 0 {
		totalCoins += streakBonus
		_ = services.CreditWalletTx(tx, userID, streakBonus,
			models.TxTypeStreakBonus, 0,
			"Streak bonus – day "+itoa(streak.CurrentStreak), nil)
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Fetch updated balance
	var user models.User
	config.DB.First(&user, userID)

	c.JSON(http.StatusOK, DailyLoginResponse{
		AlreadyCheckedIn: false,
		CoinsEarned:      totalCoins,
		CurrentStreak:    streak.CurrentStreak,
		NextMilestone:    nextMilestone(streak.CurrentStreak),
		NewBalance:       user.Points,
		Message:          buildMessage(streak.CurrentStreak, totalCoins, streakBonus),
	})
}

// GetStreakInfo handles GET /api/me/streak
func GetStreakInfo(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	userID, _ := userIDVal.(uint)

	var streak models.UserStreak
	if err := config.DB.Where("user_id = ?", userID).First(&streak).Error; err != nil {
		// No streak record yet
		c.JSON(http.StatusOK, gin.H{
			"current_streak":  0,
			"longest_streak":  0,
			"total_logins":    0,
			"next_milestone":  models.StreakRewardTable[0].Milestone,
			"last_login_date": nil,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"current_streak":  streak.CurrentStreak,
		"longest_streak":  streak.LongestStreak,
		"total_logins":    streak.TotalLogins,
		"next_milestone":  nextMilestone(streak.CurrentStreak),
		"last_login_date": streak.LastLoginDate,
	})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func nextMilestone(current int) int {
	for _, r := range models.StreakRewardTable {
		if r.Milestone > current {
			return r.Milestone
		}
	}
	return models.StreakRewardTable[len(models.StreakRewardTable)-1].Milestone + 30
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
