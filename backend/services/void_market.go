package services

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"profhit-backend/config"
	"profhit-backend/models"
	"strings"
	"time"
)

// Serialize with entries and settlement using the market lock, then ascending wallet locks.
// Preserving submissions and the immutable ledger makes cancellation accountable and idempotent.
func VoidMarket(id interface{}, reason string, adminID uint, ip string) ([]uint, error) {
	users := []uint{}
	if len(strings.TrimSpace(reason)) < 10 || len(reason) > 2000 {
		return users, errors.New("a cancellation reason of 10–2000 characters is required")
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var m models.Market
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&m).Error; err != nil {
			return err
		}
		if m.ResolutionStatus == "Voided" || m.ResolutionStatus == "Resolved" || m.ResolutionStatus == "Archived" {
			return errors.New("settled or cancelled markets cannot be cancelled again")
		}
		var preds []models.PredictionSubmission
		if err := tx.Where("market_id = ?", m.ID).Order("user_id asc").Find(&preds).Error; err != nil {
			return err
		}
		for _, p := range preds {
			if p.IsCorrect != nil {
				return errors.New("settled predictions cannot be refunded")
			}
			if err := LockWalletTx(tx, p.UserID); err != nil {
				return err
			}
			if p.Amount > 0 {
				if err := CreditWalletTx(tx, p.UserID, p.Amount, models.TxTypeRefund, p.ID, "Entry refunded: "+m.Title, &adminID); err != nil {
					return err
				}
			}
			key := fmt.Sprintf("market_void_%d_user_%d", m.ID, p.UserID)
			if err := tx.Create(&models.Notification{UserID: p.UserID, Key: key, Message: fmt.Sprintf("%s was cancelled. %d Coins refunded. %s", m.Title, p.Amount, reason)}).Error; err != nil {
				return err
			}
			users = append(users, p.UserID)
		}
		now := time.Now().UTC()
		if err := tx.Model(&m).Updates(map[string]interface{}{"resolution_status": "Voided", "void_reason": strings.TrimSpace(reason), "is_featured": false, "resolved_at": now, "resolved_by_id": adminID}).Error; err != nil {
			return err
		}
		if len(users) > 0 {
			if err := tx.Exec(`UPDATE users SET total_predictions = (SELECT COUNT(ps.id) FROM prediction_submissions ps INNER JOIN markets m ON m.id = ps.market_id WHERE ps.user_id = users.id AND ps.deleted_at IS NULL AND m.deleted_at IS NULL AND m.resolution_status <> 'Voided') WHERE id IN ?`, users).Error; err != nil {
				return err
			}
		}
		return LogAction(tx, adminID, "VOID_MARKET", fmt.Sprint(m.ID), reason, ip)
	})
	return users, err
}
