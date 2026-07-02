package models

import (
	"time"
	"gorm.io/gorm"
)

// PasswordResetToken stores a one-time, expiring token for password resets.
// The token is a cryptographically random hex string.
type PasswordResetToken struct {
	ID        uint           `gorm:"primaryKey" json:"-"`
	UserID    uint           `gorm:"not null;index" json:"-"`
	User      User           `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Token     string         `gorm:"not null;uniqueIndex" json:"-"` // hex token
	ExpiresAt time.Time      `gorm:"not null" json:"-"`
	CreatedAt time.Time      `json:"-"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
