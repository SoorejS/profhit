package models

import (
	"time"
)

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MarketID  uint      `gorm:"not null;index" json:"market_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Username  string    `gorm:"not null" json:"username"` // Denormalized for fast reads
	Content   string    `gorm:"not null;type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
