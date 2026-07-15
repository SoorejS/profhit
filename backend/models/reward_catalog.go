package models

import (
	"gorm.io/gorm"
	"time"
)

// RewardItem represents an item available in the reward center.
type RewardItem struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description"`
	Cost        int            `gorm:"not null;check:cost > 0" json:"cost"` // Coin cost
	Inventory   int            `gorm:"default:-1" json:"inventory"`         // -1 means infinite
	ImageURL    string         `json:"image_url"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Redemption tracks a user redeeming coins for a reward item.
type Redemption struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"not null;index" json:"user_id"`
	RewardItemID uint           `gorm:"not null;index" json:"reward_item_id"`
	CostPaid     int            `gorm:"not null" json:"cost_paid"`
	Status       string         `gorm:"default:'Pending';index" json:"status"` // Pending, Approved, Rejected, Completed
	VoucherCode  string         `json:"voucher_code"`                          // Populated upon completion
	AdminRemarks string         `json:"admin_remarks"`
	AdminID      *uint          `json:"admin_id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
