package models

import (
	"time"
	"gorm.io/gorm"
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
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;not null" json:"username"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	Password  string         `gorm:"not null" json:"-"` // Omitted from JSON responses
	Tier      string         `gorm:"default:'Standard'" json:"tier"`
	Role      string         `gorm:"default:'user'" json:"role"` // RBAC role
	IsActive  bool           `gorm:"default:true" json:"is_active"` // false = banned
	Points    int            `gorm:"default:100" json:"points"`
	KycStatus        bool           `gorm:"default:false" json:"kyc_status"`
	ReferralCode     string         `gorm:"uniqueIndex" json:"referral_code"`
	ReferredBy       uint           `gorm:"default:0" json:"referred_by"`
	TwoFactorSecret  string         `json:"-"`
	TwoFactorEnabled bool           `gorm:"default:false" json:"two_factor_enabled"`
	IsMuted          bool           `gorm:"default:false" json:"is_muted"` // Cannot comment if true
	SuspendedUntil   *time.Time     `json:"suspended_until"`               // Temporary ban
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
