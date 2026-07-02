package services

import (
	"profhit-backend/config"
	"profhit-backend/models"

	"gorm.io/gorm"
)

// LogAction records an administrative or system action.
// It accepts a tx object to participate in larger transactions. If tx is nil, it uses config.DB.
func LogAction(tx *gorm.DB, adminID uint, action, targetID, details, ip string) error {
	db := config.DB
	if tx != nil {
		db = tx
	}

	auditLog := models.AuditLog{
		AdminID:   adminID,
		Action:    action,
		TargetID:  targetID,
		Details:   details,
		IPAddress: ip,
	}

	return db.Create(&auditLog).Error
}
