package models

import (
	"gorm.io/gorm"
	"time"
)

// TransactionType enumerates all sources of wallet balance changes.
type TransactionType string

const (
	TxTypeDailyLogin      TransactionType = "daily_login"
	TxTypeStreakBonus     TransactionType = "streak_bonus"
	TxTypePredictionWin   TransactionType = "prediction_win"
	TxTypePredictionStake TransactionType = "prediction_stake"
	TxTypeRefund          TransactionType = "refund"
	TxTypePurchase        TransactionType = "purchase"
	TxTypeReferralBonus   TransactionType = "referral_bonus"
	TxTypeRedemption      TransactionType = "redemption"
	TxTypeAdminAdjustment TransactionType = "admin_adjustment"
	TxTypeExpired         TransactionType = "expired"
)

// WalletLedger is a strict double-entry ledger.
// No balance should change without an immutable entry here.
type WalletLedger struct {
	ID            uint            `gorm:"primaryKey" json:"id"`
	UserID        uint            `gorm:"not null;index" json:"user_id"`
	User          User            `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Type          TransactionType `gorm:"not null;index" json:"type"`
	Credit        int             `gorm:"not null;default:0;check:credit >= 0" json:"credit"` // Amount added
	Debit         int             `gorm:"not null;default:0;check:debit >= 0" json:"debit"`   // Amount subtracted
	BalanceBefore int             `gorm:"not null" json:"balance_before"`
	BalanceAfter  int             `gorm:"not null" json:"balance_after"`
	ReferenceID   uint            `gorm:"default:0" json:"reference_id"` // e.g., Market ID, Redemption ID
	Description   string          `json:"description"`
	Status        string          `gorm:"default:'completed';index" json:"status"` // pending, completed, failed
	AdminID       *uint           `json:"admin_id"`                                // If adjusted by an admin
	CreatedAt     time.Time       `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
}
