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
// It finds the referrer, links the new user, and records the initial SignedUp event
// with the mandatory 48-hour pending period before coins are credited.
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

	// ── PDF §4.3: Enforce 20-referral cap ─────────────────────────────────────
	var paidCount int64
	tx.Model(&models.ReferralEvent{}).
		Where("referrer_id = ? AND is_paid = ?", referrer.ID, true).
		Count(&paidCount)
	var pendingCount int64
	tx.Model(&models.ReferralEvent{}).
		Where("referrer_id = ? AND is_paid = ? AND deleted_at IS NULL", referrer.ID, false).
		Count(&pendingCount)

	if paidCount+pendingCount >= int64(models.MaxReferralRewards) {
		// Silently succeed — new user is still registered, referrer just gets no more bonus
		if err := tx.Model(&models.User{}).Where("id = ?", newUserID).Update("referred_by", referrer.ID).Error; err != nil {
			return err
		}
		return nil
	}

	// Link user to referrer
	if err := tx.Model(&models.User{}).Where("id = ?", newUserID).Update("referred_by", referrer.ID).Error; err != nil {
		return err
	}

	// ── PDF §4.3: 48-hour pending delay ────────────────────────────────────────
	signupReward := 50
	pendingUntil := time.Now().Add(48 * time.Hour)

	event := models.ReferralEvent{
		ReferrerID:   referrer.ID,
		ReferredID:   newUserID,
		Status:       models.ReferralStatusSignedUp,
		Earnings:     signupReward,
		PendingUntil: pendingUntil,
		IsPaid:       false, // Will be credited by cron after 48h
	}

	if err := tx.Create(&event).Error; err != nil {
		return err
	}

	// Also create a pending event for the new user's welcome bonus
	welcomeEvent := models.ReferralEvent{
		ReferrerID:   newUserID,  // new user is "referrer" of their own welcome bonus
		ReferredID:   referrer.ID,
		Status:       models.ReferralStatusSignedUp,
		Earnings:     signupReward,
		PendingUntil: pendingUntil,
		IsPaid:       false,
	}
	if err := tx.Create(&welcomeEvent).Error; err != nil {
		return err
	}

	return nil
}

// TriggerReferralEvent checks if a referred user has reached a milestone
// and creates a PENDING referral event if eligible.
// Actual coin crediting is deferred to the cron job after 48 hours.
func TriggerReferralEvent(userID uint, status models.ReferralStatus, rewardAmount int) error {
	return config.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		if user.ReferredBy == 0 {
			return nil // User wasn't referred
		}

		// ── PDF §4.3: Check duplicate milestone (prevent re-award) ───────────
		var existing models.ReferralEvent
		if err := tx.Where("referred_id = ? AND status = ?", userID, status).First(&existing).Error; err == nil {
			return nil // Already created an event for this milestone
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// ── PDF §4.3: Enforce 20-referral cap ─────────────────────────────────
		var totalEvents int64
		tx.Model(&models.ReferralEvent{}).
			Where("referrer_id = ? AND deleted_at IS NULL", user.ReferredBy).
			Count(&totalEvents)

		if totalEvents >= int64(models.MaxReferralRewards) {
			return nil // Cap reached — no more rewards for this referrer
		}

		// ── PDF §4.3: 48-hour pending delay ─────────────────────────────────
		event := models.ReferralEvent{
			ReferrerID:   user.ReferredBy,
			ReferredID:   userID,
			Status:       status,
			Earnings:     rewardAmount,
			PendingUntil: time.Now().Add(48 * time.Hour),
			IsPaid:       false,
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}

		_ = fmt.Sprintf("Pending referral event created for referrer %d, will pay %d coins after %s",
			user.ReferredBy, rewardAmount, event.PendingUntil.Format(time.RFC3339))

		return nil
	})
}
