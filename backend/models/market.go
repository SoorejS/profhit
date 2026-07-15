package models

import (
	"gorm.io/gorm"
	"time"
)

type Market struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Title            string         `gorm:"not null" json:"title"`
	Description      string         `json:"description"`
	Category         string         `gorm:"index;not null" json:"category"`                 // Weather, Sports, Politics, Entertainment, Financial Markets, Wild Card
	Difficulty       string         `gorm:"default:'Medium'" json:"difficulty"`             // Easy, Medium, Hard
	Payout           int            `gorm:"default:0" json:"payout"`                        // Fixed coin payout (e.g. 20)
	Options          string         `gorm:"default:'[\"Yes\",\"No\"]'" json:"options"`      // JSON array e.g. ["Yes","No"]
	Volume           int            `gorm:"default:0" json:"volume"`                        // Total predictions submitted
	ResolutionStatus string         `gorm:"default:'Draft';index" json:"resolution_status"` // Draft, Scheduled, Live, Locked, Awaiting Resolution, Resolved, Archived
	ResolutionSource string         `json:"resolution_source"`
	CorrectOption    string         `json:"correct_option"`                  // Populated on resolution
	ResolvedAt       *time.Time     `json:"resolved_at"`                     // Timestamp of resolution
	ResolvedByID     uint           `gorm:"default:0" json:"resolved_by_id"` // Admin who resolved
	CreatorID        uint           `gorm:"default:0" json:"creator_id"`
	Visibility       string         `gorm:"default:'Public'" json:"visibility"` // Public, Unlisted
	CoinReward       int            `gorm:"default:0" json:"coin_reward"`       // Reward to the creator
	StartTime        *time.Time     `json:"start_time"`                         // When to transition from Scheduled -> Live
	LockTime         *time.Time     `json:"lock_time"`                          // When to stop accepting predictions (Live -> Locked)
	ResolutionTime   *time.Time     `json:"resolution_time"`                    // Expected time of resolution (Locked -> Awaiting Resolution)
	EndDate          time.Time      `json:"end_date"`                           // Legacy/general end date
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}
