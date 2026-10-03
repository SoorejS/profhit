package services

import (
	"time"

	"profhit-backend/config"
	"profhit-backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReplacePasswordResetToken atomically replaces every prior reset token for a
// user. tokenHash must be a one-way hash; raw reset tokens are never persisted.
func ReplacePasswordResetToken(userID uint, tokenHash string, expiresAt time.Time) error {
	return config.DB.Transaction(func(tx *gorm.DB) error {
		var locked models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, userID).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&models.PasswordResetToken{}).Error; err != nil {
			return err
		}
		return tx.Create(&models.PasswordResetToken{
			UserID:    userID,
			Token:     tokenHash,
			ExpiresAt: expiresAt,
		}).Error
	})
}
