package models

import (
	"time"
	"gorm.io/gorm"
)

type PredictionSubmission struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index:idx_user_market,unique" json:"user_id"`
	MarketID  uint           `gorm:"not null;index:idx_user_market,unique" json:"market_id"`
	Choice    string         `gorm:"not null" json:"choice"`             // What they predicted
	Potential int            `gorm:"not null" json:"potential_payout"`   // Fixed coin payout if correct
	IsCorrect *bool          `json:"is_correct"`                          // null=pending, true/false when resolved
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
