package models

import (
	"time"
	"gorm.io/gorm"
)

// ReferralStatus tracks the lifecycle of a referral.
type ReferralStatus string

const (
	ReferralStatusSignedUp     ReferralStatus = "signed_up"
	ReferralStatusKYCCompleted ReferralStatus = "kyc_completed"
	ReferralStatusFirstDeposit ReferralStatus = "first_deposit"
	ReferralStatusFirstBet     ReferralStatus = "first_prediction"
)

// ReferralEvent tracks the timeline of a referred user completing actions.
type ReferralEvent struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ReferrerID uint           `gorm:"not null;index" json:"referrer_id"`
	ReferredID uint           `gorm:"not null;index" json:"referred_id"`
	Referrer   User           `gorm:"foreignKey:ReferrerID;constraint:OnDelete:CASCADE" json:"-"`
	Referred   User           `gorm:"foreignKey:ReferredID;constraint:OnDelete:CASCADE" json:"-"`
	Status     ReferralStatus `gorm:"not null;index" json:"status"`
	Earnings   int            `gorm:"default:0" json:"earnings"` // Points awarded for this event
	CreatedAt  time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
