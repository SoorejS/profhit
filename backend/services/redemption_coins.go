package services

import (
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"profhit-backend/models"
	"time"
)

func DebitRedemptionTx(tx *gorm.DB, userID uint, amount int, kind string, ref uint) error {
	if amount <= 0 || ref == 0 {
		return errors.New("invalid redemption debit")
	}
	if err := LockWalletTx(tx, userID); err != nil {
		return err
	}
	var batches []models.CoinBatch
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND balance > 0 AND expires_at > ?", userID, time.Now().UTC()).Order("created_at, id").Find(&batches).Error; err != nil {
		return err
	}
	remaining := amount
	for _, b := range batches {
		if remaining == 0 {
			break
		}
		used := b.Balance
		if used > remaining {
			used = remaining
		}
		remaining -= used
		if err := tx.Model(&b).Update("balance", b.Balance-used).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.CoinConsumption{UserID: userID, BatchID: b.ID, Kind: kind, ReferenceID: ref, Amount: used}).Error; err != nil {
			return err
		}
	}
	if remaining != 0 {
		return errors.New("insufficient unexpired balance")
	}
	return addLedgerEntry(tx, userID, models.TxTypeRedemption, 0, amount, ref, "Voucher redemption: "+kind, nil)
}

// Restores only the original, still-valid batches. Never restarts their six-month clock.
func RefundRedemptionTx(tx *gorm.DB, userID uint, kind string, ref uint, adminID *uint) error {
	if err := LockWalletTx(tx, userID); err != nil {
		return err
	}
	var rows []models.CoinConsumption
	if err := tx.Where("user_id = ? AND kind = ? AND reference_id = ?", userID, kind, ref).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("historical redemption needs allocation reconciliation before refund")
	}
	total := 0
	for _, r := range rows {
		if r.Refunded {
			continue
		}
		var b models.CoinBatch
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&b, r.BatchID).Error; err != nil {
			return err
		}
		if b.ExpiresAt.After(time.Now().UTC()) {
			total += r.Amount
			if err := tx.Model(&b).Update("balance", b.Balance+r.Amount).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&r).Update("refunded", true).Error; err != nil {
			return err
		}
	}
	if total == 0 {
		return nil
	}
	return recordLedger(tx, userID, models.TxTypeRefund, total, 0, ref, "Redemption refund to original unexpired batches", adminID, false)
}
