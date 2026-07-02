package models

import (
	"time"

	"gorm.io/gorm"
)

// AuditLog tracks critical admin and system actions
type AuditLog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	AdminID   uint           `gorm:"index" json:"admin_id"`   // The user who performed the action
	Action    string         `gorm:"index" json:"action"`     // e.g. "BAN_USER", "RESOLVE_MARKET"
	TargetID  string         `gorm:"index" json:"target_id"`  // e.g. "user_42" or "market_5"
	Details   string         `gorm:"type:text" json:"details"` // JSON string or text describing the change
	IPAddress string         `json:"ip_address"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
