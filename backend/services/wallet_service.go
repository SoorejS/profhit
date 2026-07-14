package services

import (
	"errors"
	"time"

	"profhit-backend/config"
	"profhit-backend/models"

	"gorm.io/gorm"
)

// ── Internal helpers ─────────────────────────────────────────────────────────

func addLedgerEntry(db *gorm.DB, userID uint, txType models.TransactionType, credit int, debit int, sourceRef uint, note string, adminID *uint) error {
	if credit < 0 || debit < 0 {
		return errors.New("credit and debit amounts must be non-negative")
	}
	if credit == 0 && debit == 0 {
		return errors.New("either credit or debit must be greater than zero")
	}

	// Lock the user row to ensure strict sequential ledger balances
	var user models.User
	if err := db.Set("gorm:query_option", "FOR UPDATE").First(&user, userID).Error; err != nil {
		return err
	}

	balanceBefore := user.Points
	balanceAfter := balanceBefore + credit - debit

	if balanceAfter < 0 {
		return errors.New("insufficient balance")
	}

	// Create Immutable Ledger Entry
	entry := models.WalletLedger{
		UserID:        userID,
		Type:          txType,
		Credit:        credit,
		Debit:         debit,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		ReferenceID:   sourceRef,
		Description:   note,
		Status:        "completed",
		AdminID:       adminID,
	}

	if err := db.Create(&entry).Error; err != nil {
		return err
	}

	// For credits, always mint a new 1-year CoinBatch
	if credit > 0 {
		batch := models.CoinBatch{
			UserID:    userID,
			Amount:    credit,
			Balance:   credit,
			ExpiresAt: time.Now().AddDate(1, 0, 0),
			Source:    string(txType),
		}
		if err := db.Create(&batch).Error; err != nil {
			return err
		}
	}

	// Update cached user balance
	return db.Model(&user).UpdateColumn("points", balanceAfter).Error
}

// ── Transactional variants (use when inside an existing tx) ──────────────────

func CreditWalletTx(tx *gorm.DB, userID uint, amount int, txType models.TransactionType, sourceRef uint, note string, adminID *uint) error {
	return addLedgerEntry(tx, userID, txType, amount, 0, sourceRef, note, adminID)
}

func DebitWalletTx(tx *gorm.DB, userID uint, amount int, txType models.TransactionType, sourceRef uint, note string, adminID *uint) error {
	// Execute FIFO CoinBatch deduction
	if err := ConsumeCoinBatchesTx(tx, userID, amount); err != nil {
		return err
	}

	return addLedgerEntry(tx, userID, txType, 0, amount, sourceRef, note, adminID)
}

// ConsumeCoinBatchesTx deducts 'amount' from unexpired CoinBatches (FIFO)
func ConsumeCoinBatchesTx(tx *gorm.DB, userID uint, debitAmount int) error {
	if debitAmount <= 0 {
		return nil
	}

	var batches []models.CoinBatch
	// Lock the rows to prevent race conditions during FIFO consumption
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("user_id = ? AND balance > 0 AND expires_at > ?", userID, time.Now()).
		Order("created_at ASC").
		Find(&batches).Error; err != nil {
		return err
	}

	remainingDebit := debitAmount
	for i := 0; i < len(batches) && remainingDebit > 0; i++ {
		batch := &batches[i]
		if batch.Balance <= remainingDebit {
			// Consume entire batch
			remainingDebit -= batch.Balance
			batch.Balance = 0
		} else {
			// Consume partial batch
			batch.Balance -= remainingDebit
			remainingDebit = 0
		}
		if err := tx.Save(batch).Error; err != nil {
			return err
		}
	}

	if remainingDebit > 0 {
		// Due to aggregate WalletLedger vs CoinBatch drift from earlier missing logic,
		// some users might have points but no unexpired coin batches. We tolerate this
		// to maintain backward compatibility by allowing the debit to proceed if the aggregate
		// check in addLedgerEntry passes, rather than returning an error here.
	}

	return nil
}

// ── Standalone variants (own transaction, for simple single-op callers) ──────

func CreditWallet(userID uint, amount int, txType models.TransactionType, sourceRef uint, note string, adminID *uint) error {
	return config.DB.Transaction(func(tx *gorm.DB) error {
		return CreditWalletTx(tx, userID, amount, txType, sourceRef, note, adminID)
	})
}

func DebitWallet(userID uint, amount int, txType models.TransactionType, sourceRef uint, note string, adminID *uint) error {
	return config.DB.Transaction(func(tx *gorm.DB) error {
		return DebitWalletTx(tx, userID, amount, txType, sourceRef, note, adminID)
	})
}

// GetLedger returns all ledger transactions for a user, newest first.
func GetLedger(userID uint) ([]models.WalletLedger, error) {
	var txns []models.WalletLedger
	err := config.DB.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&txns).Error
	return txns, err
}
