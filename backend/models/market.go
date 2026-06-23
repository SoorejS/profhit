package models

import (
	"time"
	"gorm.io/gorm"
)

type Market struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Title            string         `gorm:"not null" json:"title"`
	Description      string         `json:"description"`
	Category         string         `gorm:"index;not null" json:"category"` // e.g., "Politics", "Finance"
	YesPrice         int            `gorm:"default:50" json:"yes_price"` // Between 1 and 99
	NoPrice          int            `gorm:"default:50" json:"no_price"`
	Volume           float64        `gorm:"default:0" json:"volume"`
	ResolutionStatus string         `gorm:"default:'Open'" json:"resolution_status"` // 'Proposed', 'Open', 'Resolved_Yes', 'Resolved_No'
	ResolutionSource string         `json:"resolution_source"`
	CreatorID        uint           `gorm:"default:0" json:"creator_id"` // 0 if created by admin
	EndDate          time.Time      `json:"end_date"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}
