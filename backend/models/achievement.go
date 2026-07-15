package models

import (
	"gorm.io/gorm"
	"time"
)

// Achievement represents an unlockable milestone.
type Achievement struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Code        string         `gorm:"uniqueIndex;not null" json:"code"` // e.g., FIRST_PREDICTION, STREAK_7
	Title       string         `gorm:"not null" json:"title"`
	Description string         `json:"description"`
	Reward      int            `gorm:"default:0" json:"reward"` // Coin reward
	Icon        string         `json:"icon"`                    // URL or class
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserAchievement tracks a user unlocking an achievement.
type UserAchievement struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"not null;uniqueIndex:idx_user_achievement" json:"user_id"`
	AchievementID uint      `gorm:"not null;uniqueIndex:idx_user_achievement" json:"achievement_id"`
	UnlockedAt    time.Time `json:"unlocked_at"`
}

// Badge represents a visual status earned by a user.
type Badge struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Code        string         `gorm:"uniqueIndex;not null" json:"code"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserBadge tracks a user earning a badge.
type UserBadge struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	UserID   uint      `gorm:"not null;uniqueIndex:idx_user_badge" json:"user_id"`
	BadgeID  uint      `gorm:"not null;uniqueIndex:idx_user_badge" json:"badge_id"`
	EarnedAt time.Time `json:"earned_at"`
}
