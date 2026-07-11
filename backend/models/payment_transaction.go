package models

import (
	"time"
)

// PaymentTransaction stores Razorpay transaction details ensuring exact-once processing
type PaymentTransaction struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	UserID            uint      `gorm:"not null;index" json:"user_id"`
	ProviderOrderID   string    `gorm:"uniqueIndex;not null" json:"provider_order_id"`
	ProviderPaymentID string    `gorm:"not null" json:"provider_payment_id"`
	Amount            float64   `gorm:"not null" json:"amount"`
	Status            string    `gorm:"not null;default:'Completed'" json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}
