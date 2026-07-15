package models

import (
	"gorm.io/gorm"
	"time"
)

// Role constants
const (
	RoleSuperAdmin     = "super_admin"
	RoleAdmin          = "admin"
	RoleContentCreator = "content_creator"
	RoleITSupport      = "it_support"
	RoleUser           = "user"
)

type User struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Username string `gorm:"uniqueIndex;not null" json:"username"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `gorm:"not null" json:"-"` // Omitted from JSON responses
	Tier     string `gorm:"default:'Standard'" json:"tier"`
	Role     string `gorm:"default:'user'" json:"role"`    // RBAC role
	IsActive bool   `gorm:"default:true" json:"is_active"` // false = banned

	// ── Core Platform Fields ──────────────────────────────────────────────────
	Points           int        `gorm:"index;default:0;check:points >= 0" json:"points"`
	WinRate          float64    `gorm:"index;default:0" json:"win_rate"`
	TotalPredictions int        `gorm:"default:0" json:"total_predictions"`
	KycStatus        bool       `gorm:"default:false" json:"kyc_status"`
	ReferralCode     string     `gorm:"uniqueIndex" json:"referral_code"`
	ReferredBy       uint       `gorm:"default:0" json:"referred_by"`
	TwoFactorSecret  string     `json:"-"`
	TwoFactorEnabled bool       `gorm:"default:false" json:"two_factor_enabled"`
	IsMuted          bool       `gorm:"default:false" json:"is_muted"` // Cannot comment if true
	SuspendedUntil   *time.Time `json:"suspended_until"`               // Temporary ban

	// ── Demographic Fields (PDF §2.1 — Commercial Asset for Advertiser Platform) ─
	Phone       string     `gorm:"default:''" json:"phone"`
	DateOfBirth *time.Time `json:"date_of_birth"`
	City        string     `gorm:"default:''" json:"city"`
	Country     string     `gorm:"default:''" json:"country"`
	Location    string     `gorm:"default:''" json:"location"`  // Free-text region/state
	Interests   string     `gorm:"default:''" json:"interests"` // Comma-separated tags, e.g. "Sports,Finance"

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
