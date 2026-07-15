package models

import (
	"gorm.io/gorm"
	"time"
)

// MaxReferralRewards is the maximum number of referral milestone rewards
// a single referrer can earn, per the PDF business rules.
const MaxReferralRewards = 20

// ReferralStatus tracks the lifecycle of a referral.
type ReferralStatus string

const (
	ReferralStatusSignedUp     ReferralStatus = "signed_up"
	ReferralStatusKYCCompleted ReferralStatus = "kyc_completed"
	ReferralStatusFirstDeposit ReferralStatus = "first_deposit"
	ReferralStatusFirstBet     ReferralStatus = "first_prediction"
)

// ReferralEvent tracks the timeline of a referred user completing actions.
// PendingUntil enforces the mandatory 48-hour fraud-check delay before coins
// are credited. IsPaid is flipped to true by the hourly cron job once the
// delay has elapsed. This ensures rewards survive server restarts.
type ReferralEvent struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ReferrerID   uint           `gorm:"not null;index" json:"referrer_id"`
	ReferredID   uint           `gorm:"not null;index" json:"referred_id"`
	Referrer     User           `gorm:"foreignKey:ReferrerID;constraint:OnDelete:CASCADE" json:"-"`
	Referred     User           `gorm:"foreignKey:ReferredID;constraint:OnDelete:CASCADE" json:"-"`
	Status       ReferralStatus `gorm:"not null;index" json:"status"`
	Earnings     int            `gorm:"default:0" json:"earnings"`          // Points to be awarded
	PendingUntil time.Time      `gorm:"index" json:"pending_until"`         // Award not before this time (48h delay)
	IsPaid       bool           `gorm:"default:false;index" json:"is_paid"` // Flipped by cron after delay elapses
	CreatedAt    time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
