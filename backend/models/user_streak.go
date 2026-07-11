package models

import (
	"time"
	"gorm.io/gorm"
)

// UserStreak tracks consecutive daily login activity for each user.
// It is updated atomically whenever /api/me/daily-login is called.
type UserStreak struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	User            User      `json:"-"`
	CurrentStreak   int       `gorm:"default:0" json:"current_streak"`     // Consecutive days logged in
	LongestStreak   int       `gorm:"default:0" json:"longest_streak"`     // All-time best
	LastLoginDate   time.Time `json:"last_login_date"`                     // Midnight-normalised date of last login
	TotalLogins     int       `gorm:"default:0" json:"total_logins"`       // Lifetime login count
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// StreakReward maps a streak milestone to the bonus coins awarded.
type StreakReward struct {
	Milestone int // e.g. 3, 7, 30
	Coins     int // Extra coins on top of the daily base
}

// StreakRewardTable is the business-rule lookup from the game plan PDF.
var StreakRewardTable = []StreakReward{
	{Milestone: 3,  Coins: 25},  // 3-day streak bonus
	{Milestone: 7,  Coins: 75},  // 7-day streak bonus
	{Milestone: 30, Coins: 500}, // Monthly loyalty bonus
}

// DailyLoginBaseCoins is the base award for any valid daily login.
const DailyLoginBaseCoins = 10
