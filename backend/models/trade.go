package models

import (
	"time"
	"gorm.io/gorm"
)

type Trade struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	MarketID  uint           `gorm:"not null;index" json:"market_id"`
	Outcome   string         `gorm:"not null" json:"outcome"` // "Yes" or "No"
	Points    float64        `gorm:"not null" json:"points"`  // How much they spent
	Shares    float64        `gorm:"not null" json:"shares"`  // How many shares they got
	Price     int            `gorm:"not null" json:"price"`   // The price at time of purchase
	Payout    int            `gorm:"default:0" json:"payout"` // Payout on resolution
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
