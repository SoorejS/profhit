package services

import (
	"errors"
	"gorm.io/gorm"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
)

func ClaimDailyLogin(userID uint, at time.Time) (bool, error) {
	claimed := false
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := LockWalletTx(tx, userID); err != nil {
			return err
		}
		var row models.UserStreak
		err := tx.Where("user_id = ?", userID).First(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		day := at.UTC().Truncate(24 * time.Hour)
		if !row.LastLoginDate.IsZero() && !day.After(row.LastLoginDate.UTC().Truncate(24*time.Hour)) {
			return nil
		}
		row.UserID = userID
		row.LastLoginDate = day
		row.TotalLogins++
		row.CurrentStreak = 0
		row.LongestStreak = 0
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if err := CreditWalletTx(tx, userID, 10, models.TxTypeDailyLogin, row.ID, "Daily login reward", nil); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err == nil && claimed {
		BroadcastToUser(userID, "wallet_updated", "Daily login reward credited")
	}
	return claimed, err
}
