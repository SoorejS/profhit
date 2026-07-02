package models

import (
	"time"

	"gorm.io/gorm"
)

// Report tracking user-submitted reports for markets or comments
type Report struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ReporterID   uint           `gorm:"index" json:"reporter_id"`
	TargetType   string         `gorm:"index" json:"target_type"` // e.g. "User", "Market", "Comment"
	TargetID     uint           `gorm:"index" json:"target_id"`
	Reason       string         `json:"reason"`
	Description  string         `gorm:"type:text" json:"description"`
	Status       string         `gorm:"default:'Pending'" json:"status"` // "Pending", "Reviewed", "Resolved", "Dismissed"
	ResolvedByID *uint          `json:"resolved_by_id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
