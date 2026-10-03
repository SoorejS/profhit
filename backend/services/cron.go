package services

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
)

// StartCronJobs initializes background workers for the application
func StartCronJobs() {
	log.Println("Starting Cron Jobs...")
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			if err := RefreshLiveNews(ctx, ConfiguredNewsProviders(), time.Now().UTC()); err != nil {
				log.Printf("News refresh: %v", err)
			}
			cancel()
			time.Sleep(time.Minute)
		}
	}()

	// Tick every minute
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			<-ticker.C
			runEveryMinute()
		}
	}()

	// Tick every hour (for larger batch jobs like expiry and wild card)
	hourTicker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			<-hourTicker.C
			runEveryHour()
		}
	}()
}

func runEveryMinute() {
	transitionMarkets()
	resolveProviderMarkets()
}

func runEveryHour() {
	if err := config.DB.Where("expires_at < ?", time.Now().UTC().Add(-24*time.Hour)).Delete(&models.RateLimitBucket{}).Error; err != nil {
		log.Printf("Request protection cleanup failed")
	}
	if err := config.DB.Where("expires_at < ?", time.Now().UTC()).Delete(&models.RevokedToken{}).Error; err != nil {
		log.Printf("Session cleanup failed: %v", err)
	}
	processCoinExpiries()
	sendExpiryReminders()
	processPendingReferrals()
}

// transitionMarkets moves markets between states based on their lifecycle timestamps
func transitionMarkets() {
	now := time.Now().UTC()
	if err := config.DB.Model(&models.WeeklyChallenge{}).Where("status = ? AND end_date <= ?", "Active", now).Update("status", "Closed").Error; err != nil {
		log.Printf("Challenge cutoff transition failed: %v", err)
	}

	// 1. Scheduled -> Live
	var scheduledMarkets []models.Market
	if err := config.DB.Where("resolution_status = ? AND start_time <= ?", "Scheduled", now).Find(&scheduledMarkets).Error; err != nil {
		log.Printf("Market transition scan failed: %v", err)
		return
	}
	for _, m := range scheduledMarkets {
		result := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status = ?", m.ID, "Scheduled").Update("resolution_status", "Live")
		if result.Error != nil || result.RowsAffected != 1 {
			continue
		}
		if m.Visibility == "Public" {
			BroadcastToAll("market_live", fmt.Sprintf("Market '%s' is now LIVE!", m.Title))
		}
	}

	// 2. Live -> Locked
	var liveMarkets []models.Market
	if err := config.DB.Where("resolution_status IN ? AND (lock_time <= ? OR end_date <= ?)", []string{"Live", "Open"}, now, now).Find(&liveMarkets).Error; err != nil {
		log.Printf("Market transition scan failed: %v", err)
		return
	}
	for _, m := range liveMarkets {
		result := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status IN ?", m.ID, []string{"Live", "Open"}).Update("resolution_status", "Locked")
		if result.Error != nil || result.RowsAffected != 1 {
			continue
		}
		if m.Visibility == "Public" {
			BroadcastToAll("market_locked", fmt.Sprintf("Market '%s' is now LOCKED. No more predictions accepted.", m.Title))
		}
	}

	// 3. Locked -> Awaiting Resolution
	var lockedMarkets []models.Market
	if err := config.DB.Where("resolution_status = ? AND resolution_time <= ?", "Locked", now).Find(&lockedMarkets).Error; err != nil {
		log.Printf("Market transition scan failed: %v", err)
		return
	}
	for _, m := range lockedMarkets {
		if err := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status = ?", m.ID, "Locked").Update("resolution_status", "Awaiting Resolution").Error; err != nil {
			log.Printf("Market transition failed: %v", err)
		}
		// Optionally notify admins
	}
}

// processCoinExpiries finds expired coin batches and deducts them from the ledger
func processCoinExpiries() {
	now := time.Now().UTC()
	var expiredBatches []models.CoinBatch

	// Find batches that have expired but still have a balance > 0
	if err := config.DB.Where("expires_at <= ? AND balance > 0", now).Find(&expiredBatches).Error; err != nil {
		log.Printf("Expiry scan failed: %v", err)
		return
	}

	for _, batch := range expiredBatches {
		if err := ExpireCoinBatch(batch.ID); err != nil {
			log.Printf("[Cron] Expiry failed for batch %d: %v", batch.ID, err)
		}
	}

}

// processPendingReferrals finds all ReferralEvent records whose 48-hour pending
// window has elapsed and credits the earned coins to the referrer via WalletLedger.
// This cron fulfills PDF §4.3 — delayed referral payouts survive server restarts
// because the pending state is persisted in the database.
func processPendingReferrals() {
	now := time.Now().UTC()
	var pendingEvents []models.ReferralEvent

	if err := config.DB.Where("is_paid = ? AND status = ? AND earnings = 50 AND pending_until <= ?", false, models.ReferralStatusFirstBet, now).Find(&pendingEvents).Error; err != nil {
		log.Printf("Referral scan failed: %v", err)
		return
	}

	for _, event := range pendingEvents {
		if err := PayReferralEvent(event.ID); err != nil {
			log.Printf("[Cron] Referral %d failed: %v", event.ID, err)
		}
	}

}

// publishDailyWildCard generates a Daily Wild Card market if one doesn't exist for the day
func publishDailyWildCard() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_ = RefreshLiveNews(ctx, ConfiguredNewsProviders(), time.Now().UTC())
}

// sendExpiryReminders finds coin batches expiring within 30 days that haven't received a reminder
func sendExpiryReminders() {
	now := time.Now().UTC()
	var batches []models.CoinBatch
	if err := config.DB.Where("expires_at > ? AND expires_at <= ? AND balance > 0 AND reminder_sent_at IS NULL", now, now.AddDate(0, 0, 30)).Find(&batches).Error; err != nil {
		log.Printf("Reminder query failed: %v", err)
		return
	}
	for _, batch := range batches {
		if err := SendExpiryReminder(batch.ID); err != nil {
			log.Printf("Reminder failed: %v", err)
		}
	}
}
func SendExpiryReminder(batchID uint) error {
	var notifiedUser uint
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var batch models.CoinBatch
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&batch, batchID).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if batch.ReminderSentAt != nil || batch.Balance <= 0 || !batch.ExpiresAt.After(now) || batch.ExpiresAt.After(now.AddDate(0, 0, 30)) {
			return nil
		}
		notification := models.Notification{UserID: batch.UserID, Key: fmt.Sprintf("expiry:%d", batch.ID), Message: fmt.Sprintf("%d coins expire on %s. Use them before that date.", batch.Balance, batch.ExpiresAt.UTC().Format("2006-01-02"))}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification).Error; err != nil {
			return err
		}
		notifiedUser = batch.UserID
		return tx.Model(&batch).Update("reminder_sent_at", now).Error
	})
	if err == nil && notifiedUser != 0 {
		BroadcastToUser(notifiedUser, "notification_created", "Coin expiry reminder")
	}
	return err
}

func PayReferralEvent(eventID uint) error {
	var paidUser uint
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var event models.ReferralEvent
		if err := tx.First(&event, eventID).Error; err != nil {
			return err
		}
		if event.Status != models.ReferralStatusFirstBet || event.Earnings != 50 || event.ReferrerID == event.ReferredID {
			return nil
		}
		if err := LockWalletTx(tx, event.ReferrerID); err != nil {
			return err
		}
		var parent, child models.User
		if err := tx.First(&parent, event.ReferrerID).Error; err != nil {
			return err
		}
		if err := tx.First(&child, event.ReferredID).Error; err != nil {
			return err
		}
		if child.ReferredBy != parent.ID {
			return nil
		}
		if !parent.IsActive || !child.IsActive || (parent.SuspendedUntil != nil && parent.SuspendedUntil.After(time.Now().UTC())) || (child.SuspendedUntil != nil && child.SuspendedUntil.After(time.Now().UTC())) {
			return nil
		}
		var paid int64
		if err := tx.Model(&models.ReferralEvent{}).Where("referrer_id = ? AND is_paid = ?", event.ReferrerID, true).Count(&paid).Error; err != nil {
			return err
		}
		if paid >= models.MaxReferralRewards {
			return nil
		}
		// Conditional claim makes repeated and concurrent cron executions idempotent.
		claimed := tx.Model(&models.ReferralEvent{}).Where("id = ? AND is_paid = ? AND pending_until <= ?", eventID, false, time.Now().UTC()).Update("is_paid", true)
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected == 0 {
			return nil
		}
		if err := CreditWalletTx(tx, event.ReferrerID, event.Earnings, models.TxTypeReferralBonus, event.ID, "Referral bonus: "+string(event.Status), nil); err != nil {
			return err
		}
		paidUser = event.ReferrerID
		return nil
	})
	if err == nil && paidUser != 0 {
		BroadcastToUser(paidUser, "wallet_updated", "Referral reward credited")
	}
	return err
}
