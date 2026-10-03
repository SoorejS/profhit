package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"profhit-backend/config"
	"profhit-backend/models"
	"strings"
	"time"
)

func GenerateReferralCode() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic("referral entropy unavailable")
	}
	return strings.ToUpper(hex.EncodeToString(raw[:]))
}

// Signup only records the relationship. No economic event is created here.
func ProcessReferral(tx *gorm.DB, childID uint, code string) error {
	if code == "" {
		return nil
	}
	var parent models.User
	if err := tx.Where("referral_code = ?", code).First(&parent).Error; err != nil {
		return errors.New("invalid referral code")
	}
	if parent.ID == childID {
		return errors.New("cannot refer yourself")
	}
	if err := LockWalletTx(tx, childID); err != nil {
		return err
	}
	result := tx.Model(&models.User{}).Where("id = ? AND referred_by = 0", childID).Update("referred_by", parent.ID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("referral already recorded")
	}
	return nil
}

func TriggerReferralEvent(userID uint, status models.ReferralStatus, amount int) error {
	return config.DB.Transaction(func(tx *gorm.DB) error { return TriggerReferralEventTx(tx, userID, status, amount) })
}

// Legacy call sites cannot mint rewards for signup, KYC or deposits.
func TriggerReferralEventTx(tx *gorm.DB, userID uint, status models.ReferralStatus, amount int) error {
	if status != models.ReferralStatusFirstBet || amount != 50 {
		return nil
	}
	var user models.User
	if err := tx.First(&user, userID).Error; err != nil {
		return err
	}
	if user.ReferredBy == 0 {
		return nil
	}
	if user.ReferredBy == userID {
		return errors.New("invalid self referral")
	}
	var firstCount int64
	if err := tx.Model(&models.PredictionSubmission{}).Where("user_id = ?", userID).Count(&firstCount).Error; err != nil {
		return err
	}
	if firstCount != 1 {
		return nil
	}
	if err := LockWalletTx(tx, user.ReferredBy); err != nil {
		return err
	}
	var existing int64
	if err := tx.Model(&models.ReferralEvent{}).Where("referred_id = ? AND status = ?", userID, models.ReferralStatusFirstBet).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	var reserved int64
	if err := tx.Model(&models.ReferralEvent{}).Where("referrer_id = ? AND (is_paid = ? OR status = ?)", user.ReferredBy, true, models.ReferralStatusFirstBet).Count(&reserved).Error; err != nil {
		return err
	}
	if reserved >= models.MaxReferralRewards {
		return nil
	}
	return tx.Create(&models.ReferralEvent{ReferrerID: user.ReferredBy, ReferredID: userID, Status: models.ReferralStatusFirstBet, Earnings: 50, PendingUntil: time.Now().UTC().Add(48 * time.Hour)}).Error
}
