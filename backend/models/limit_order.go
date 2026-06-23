package models

import (
	"time"
	"gorm.io/gorm"
)

type LimitOrder struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	MarketID    uint           `gorm:"not null;index" json:"market_id"`
	Outcome     string         `gorm:"not null" json:"outcome"` // "Yes" or "No"
	TargetPrice int            `gorm:"not null" json:"target_price"`
	Points      float64        `gorm:"not null" json:"points"`
	Status      string         `gorm:"default:'Pending'" json:"status"` // Pending, Executed, Cancelled
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}
