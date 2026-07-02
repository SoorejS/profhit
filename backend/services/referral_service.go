package services

import (
	"errors"
	"math/rand"
	"time"
	"fmt"

	"profhit-backend/config"
	"profhit-backend/models"

	"gorm.io/gorm"
)

// GenerateReferralCode creates a unique 8-character alphanumeric string
func GenerateReferralCode() string {
	var charset = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// ProcessReferral is called during user registration.
// It finds the referrer, links the new user, and records the initial SignedUp event.
func ProcessReferral(tx *gorm.DB, newUserID uint, referralCode string) error {
	if referralCode == "" {
		return nil // No referral code provided
	}

	var referrer models.User
	if err := tx.Where("referral_code = ?", referralCode).First(&referrer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("invalid referral code")
		}
		return err
	}

	if referrer.ID == newUserID {
		return errors.New("cannot refer yourself")
	}

	// Link user to referrer
	if err := tx.Model(&models.User{}).Where("id = ?", newUserID).Update("referred_by", referrer.ID).Error; err != nil {
		return err
	}

	// Create ReferralEvent for "signed_up" (e.g. 50 coins reward)
	signupReward := 50
	event := models.ReferralEvent{
		ReferrerID: referrer.ID,
		ReferredID: newUserID,
		Status:     models.ReferralStatusSignedUp,
		Earnings:   signupReward,
	}

	if err := tx.Create(&event).Error; err != nil {
		return err
	}

	// Reward referrer
	if err := CreditWalletTx(tx, referrer.ID, signupReward, models.TxTypeReferralBonus, newUserID, fmt.Sprintf("Referral signup bonus (User %d)", newUserID), nil); err != nil {
		return err
	}
	
	// Reward new user
	if err := CreditWalletTx(tx, newUserID, signupReward, models.TxTypeReferralBonus, referrer.ID, "Welcome bonus from referral", nil); err != nil {
		return err
	}

	return nil
}

// TriggerReferralEvent checks if a referred user has reached a milestone (KYC, FirstDeposit, etc)
// and rewards the referrer if the milestone hasn't been rewarded yet.
func TriggerReferralEvent(userID uint, status models.ReferralStatus, rewardAmount int) error {
	return config.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		if user.ReferredBy == 0 {
			return nil // User wasn't referred
		}

		// Check if event already exists to prevent duplicate abuse
		var existing models.ReferralEvent
		if err := tx.Where("referred_id = ? AND status = ?", userID, status).First(&existing).Error; err == nil {
			return nil // Already rewarded for this milestone
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Create event
		event := models.ReferralEvent{
			ReferrerID: user.ReferredBy,
			ReferredID: userID,
			Status:     status,
			Earnings:   rewardAmount,
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}

		// Reward referrer
		if err := CreditWalletTx(tx, user.ReferredBy, rewardAmount, models.TxTypeReferralBonus, userID, fmt.Sprintf("Referral milestone bonus: %s (User %d)", status, userID), nil); err != nil {
			return err
		}

		return nil
	})
}
