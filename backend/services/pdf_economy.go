package services

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"profhit-backend/config"
	"profhit-backend/models"
	"strings"
	"time"
)

// CoinExpiry clamps month ends (August 31 -> February 28/29), in UTC.
func CoinExpiry(earned time.Time) time.Time {
	return models.CoinExpiry(earned)
}

func ProfileComplete(u models.User) bool {
	for _, value := range []string{u.FullName, u.Email, u.Phone, u.City, u.Country, u.Interests} {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

// RecordPredictionDayTx shares the submission wallet lock and commits with the prediction.
func RecordPredictionDayTx(tx *gorm.DB, userID uint, at time.Time) error {
	if err := LockWalletTx(tx, userID); err != nil {
		return err
	}
	day := at.UTC().Truncate(24 * time.Hour)
	var streak models.PredictionStreak
	err := tx.Where("user_id = ?", userID).First(&streak).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil {
		streak.UserID = userID
	}
	if !streak.LastPredictionDate.IsZero() && !day.After(streak.LastPredictionDate) {
		return nil
	}
	if streak.LastPredictionDate.Equal(day.AddDate(0, 0, -1)) {
		streak.CurrentStreak++
	} else {
		streak.CurrentStreak = 1
	}
	streak.LastPredictionDate = day
	streak.TotalDays++
	if streak.CurrentStreak > streak.LongestStreak {
		streak.LongestStreak = streak.CurrentStreak
	}
	if err := tx.Save(&streak).Error; err != nil {
		return err
	}
	reward := 0
	if streak.CurrentStreak == 3 {
		reward = 25
	}
	if streak.CurrentStreak == 7 {
		reward = 75
	}
	if reward != 0 {
		return CreditWalletTx(tx, userID, reward, models.TxTypeStreakBonus, streak.ID, "Prediction streak milestone", nil)
	}
	return nil
}

type ExpiryTransition struct {
	BatchID   uint      `json:"batch_id"`
	OldExpiry time.Time `json:"old_expiry"`
	NewExpiry time.Time `json:"new_expiry"`
	Remaining int       `json:"remaining"`
}

// PreviewPDFExpiryMigration is read-only; operators review balances that will expire.
func PreviewPDFExpiryMigration() ([]ExpiryTransition, error) {
	var batches []models.CoinBatch
	if err := config.DB.Order("id").Find(&batches).Error; err != nil {
		return nil, err
	}
	changes := []ExpiryTransition{}
	for _, batch := range batches {
		expiry := CoinExpiry(batch.CreatedAt)
		if expiry.Before(batch.ExpiresAt) {
			changes = append(changes, ExpiryTransition{batch.ID, batch.ExpiresAt, expiry, batch.Balance})
		}
	}
	return changes, nil
}

// ApplyPDFExpiryMigration is deliberately not called from startup. It shortens only,
// leaves all ledger entries intact, and logs a durable operator migration record.
func ApplyPDFExpiryMigration(actor string) error {
	if strings.TrimSpace(actor) == "" {
		return errors.New("operator identity required")
	}
	return config.DB.Transaction(func(tx *gorm.DB) error {
		claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.EconomyMigration{ID: "pdf-six-month-v1", AppliedAt: time.Now().UTC(), Actor: actor})
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return nil
		}
		var batches []models.CoinBatch
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Find(&batches).Error; err != nil {
			return err
		}
		changes := []ExpiryTransition{}
		for _, batch := range batches {
			expiry := CoinExpiry(batch.CreatedAt)
			if expiry.Before(batch.ExpiresAt) {
				changes = append(changes, ExpiryTransition{batch.ID, batch.ExpiresAt, expiry, batch.Balance})
				if err := tx.Model(&batch).Update("expires_at", expiry).Error; err != nil {
					return err
				}
			}
		}
		data, err := json.Marshal(changes)
		if err != nil {
			return err
		}
		return tx.Model(&models.EconomyMigration{}).Where("id = ?", "pdf-six-month-v1").Update("changes_json", string(data)).Error
	})
}
