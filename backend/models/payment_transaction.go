package models

import (
	"time"
)

// PaymentTransaction stores Razorpay transaction details ensuring exact-once processing
type PaymentTransaction struct {
	User              User      `gorm:"foreignKey:UserID;constraint:OnDelete:RESTRICT" json:"-"`
	ID                uint      `gorm:"primaryKey" json:"id"`
	UserID            uint      `gorm:"not null;index" json:"user_id"`
	ProviderOrderID   string    `gorm:"uniqueIndex;not null" json:"provider_order_id"`
	ProviderPaymentID string    `gorm:"not null;uniqueIndex:idx_payment_provider_id,where:provider_payment_id <> ''" json:"provider_payment_id"`
	AmountPaise       int       `gorm:"not null;default:0" json:"amount_paise"`
	Amount            float64   `gorm:"not null" json:"amount"`
	Status            string    `gorm:"not null;default:'Pending'" json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}
