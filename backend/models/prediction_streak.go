package models

import "time"

// PredictionStreak is separate from the historical login/check-in record.
type PredictionStreak struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	UserID             uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	User               User      `json:"-"`
	CurrentStreak      int       `json:"current_streak"`
	LongestStreak      int       `json:"longest_streak"`
	LastPredictionDate time.Time `json:"last_prediction_date"`
	TotalDays          int       `json:"total_days"`
}

// EconomyMigration records explicit, reviewed transitions; never deletes a ledger.
type EconomyMigration struct {
	ID          string `gorm:"primaryKey"`
	AppliedAt   time.Time
	Actor       string
	ChangesJSON string
}
