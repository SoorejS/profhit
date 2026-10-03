package models

import (
	"gorm.io/gorm"
	"time"
)

type PredictionSubmission struct {
	User      User           `gorm:"foreignKey:UserID;constraint:OnDelete:RESTRICT" json:"-"`
	Market    Market         `gorm:"foreignKey:MarketID;constraint:OnDelete:RESTRICT" json:"-"`
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index:idx_user_market,unique" json:"user_id"`
	MarketID  uint           `gorm:"not null;index:idx_user_market,unique" json:"market_id"`
	Choice    string         `gorm:"not null" json:"choice"`                               // What they predicted
	Amount    int            `gorm:"not null" json:"amount"`                               // Historical stake; new free-play submissions use zero.
	Potential int            `gorm:"not null;check:potential > 0" json:"potential_payout"` // Fixed coin payout if correct
	IsCorrect *bool          `json:"is_correct"`                                           // null=pending, true/false when resolved
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
