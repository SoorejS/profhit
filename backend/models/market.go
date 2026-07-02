package models

import (
	"time"
	"gorm.io/gorm"
)

type Market struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Title            string         `gorm:"not null" json:"title"`
	Description      string         `json:"description"`
	Category         string         `gorm:"index;not null" json:"category"` // Weather, Sports, Politics, etc.
	Difficulty       string         `gorm:"not null" json:"difficulty"`     // Easy, Medium, Hard
	Payout           int            `gorm:"not null" json:"payout"`         // Fixed coin payout (e.g. 20)
	Options          string         `gorm:"not null" json:"options"`        // JSON array e.g. ["Yes","No"]
	Volume           int            `gorm:"default:0" json:"volume"`        // Total predictions submitted
	ResolutionStatus string         `gorm:"default:'Open'" json:"resolution_status"` // 'Proposed','Open','Resolved'
	ResolutionSource string         `json:"resolution_source"`
	CorrectOption    string         `json:"correct_option"` // Populated on resolution
	ResolvedAt       *time.Time     `json:"resolved_at"`    // Timestamp of resolution
	ResolvedByID     uint           `gorm:"default:0" json:"resolved_by_id"` // Admin who resolved
	CreatorID        uint           `gorm:"default:0" json:"creator_id"`
	EndDate          time.Time      `json:"end_date"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

