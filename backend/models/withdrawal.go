package models

import (
	"time"
	"gorm.io/gorm"
)

type WithdrawalRequest struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	Amount    int            `gorm:"not null" json:"amount"`
	Status    string         `gorm:"default:'Pending'" json:"status"` // Pending, Approved, Rejected
	AdminID   *uint          `json:"admin_id"` // Who processed it
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
