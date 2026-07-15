package models

import (
	"gorm.io/gorm"
	"time"
)

// CoinBatch represents a specific credit of coins that is subject to rolling expiry.
type CoinBatch struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	UserID         uint           `gorm:"not null;index" json:"user_id"`
	Amount         int            `gorm:"not null;check:amount >= 0" json:"amount"`
	Balance        int            `gorm:"not null;check:balance >= 0" json:"balance"` // Unspent amount
	ExpiresAt      time.Time      `gorm:"index" json:"expires_at"`
	ReminderSentAt *time.Time     `json:"reminder_sent_at"`
	Source         string         `json:"source"` // e.g. "prediction_win", "daily_login"
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}
