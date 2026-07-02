package models

import (
	"time"
	"gorm.io/gorm"
)

type WithdrawalRequest struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	User          User           `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Tier          string         `gorm:"not null" json:"tier"`                        // Bronze, Silver, Gold, etc.
	CoinsDeducted int            `gorm:"not null" json:"coins_deducted"`
	Amount        int            `gorm:"not null" json:"amount"`                      // The INR value of the voucher
	Status        string         `gorm:"default:'Pending';index" json:"status"`       // Pending, Approved, Rejected
	AdminID       *uint          `json:"admin_id"`                                    // Who processed it
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
