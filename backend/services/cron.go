package services

import (
	"log"
	"time"
	"fmt"
	"profhit-backend/config"
	"profhit-backend/models"
)

// StartCronJobs initializes background workers for the application
func StartCronJobs() {
	log.Println("Starting Cron Jobs...")

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
}

func runEveryHour() {
	processCoinExpiries()
	sendExpiryReminders()
	processPendingReferrals()
	publishDailyWildCard()
}

// transitionMarkets moves markets between states based on their lifecycle timestamps
func transitionMarkets() {
	now := time.Now()

	// 1. Scheduled -> Live
	var scheduledMarkets []models.Market
	config.DB.Where("resolution_status = ? AND start_time <= ?", "Scheduled", now).Find(&scheduledMarkets)
	for _, m := range scheduledMarkets {
		m.ResolutionStatus = "Live"
		config.DB.Save(&m)
		BroadcastToAll("market_live", fmt.Sprintf("Market '%s' is now LIVE!", m.Title))
	}

	// 2. Live -> Locked
	var liveMarkets []models.Market
	config.DB.Where("resolution_status = ? AND lock_time <= ?", "Live", now).Find(&liveMarkets)
	for _, m := range liveMarkets {
		m.ResolutionStatus = "Locked"
		config.DB.Save(&m)
		BroadcastToAll("market_locked", fmt.Sprintf("Market '%s' is now LOCKED. No more predictions accepted.", m.Title))
	}

	// 3. Locked -> Awaiting Resolution
	var lockedMarkets []models.Market
	config.DB.Where("resolution_status = ? AND resolution_time <= ?", "Locked", now).Find(&lockedMarkets)
	for _, m := range lockedMarkets {
		m.ResolutionStatus = "Awaiting Resolution"
		config.DB.Save(&m)
		// Optionally notify admins
	}
}

// processCoinExpiries finds expired coin batches and deducts them from the ledger
func processCoinExpiries() {
	now := time.Now()
	var expiredBatches []models.CoinBatch

	// Find batches that have expired but still have a balance > 0
	config.DB.Where("expires_at <= ? AND balance > 0", now).Find(&expiredBatches)

	for _, batch := range expiredBatches {
		// Deduct via ledger
		tx := config.DB.Begin()
		
		err := DebitWalletTx(tx, batch.UserID, batch.Balance, models.TxTypeExpired, 0, "Coin Expiry", nil)
		if err == nil {
			batch.Balance = 0
			tx.Save(&batch)
			tx.Commit()
		} else {
			tx.Rollback()
			log.Println("Failed to process coin expiry for batch:", batch.ID, err)
		}
	}
}

// processPendingReferrals finds all ReferralEvent records whose 48-hour pending
// window has elapsed and credits the earned coins to the referrer via WalletLedger.
// This cron fulfills PDF §4.3 — delayed referral payouts survive server restarts
// because the pending state is persisted in the database.
func processPendingReferrals() {
	now := time.Now()
	var pendingEvents []models.ReferralEvent

	config.DB.Where("is_paid = ? AND pending_until <= ? AND deleted_at IS NULL", false, now).
		Find(&pendingEvents)

	for _, event := range pendingEvents {
		tx := config.DB.Begin()

		// Credit coins to referrer via immutable ledger
		err := CreditWalletTx(tx, event.ReferrerID, event.Earnings,
			models.TxTypeReferralBonus, event.ReferredID,
			fmt.Sprintf("Referral bonus: milestone '%s' (User %d)", event.Status, event.ReferredID),
			nil,
		)
		if err != nil {
			tx.Rollback()
			log.Printf("[Cron] Failed to pay referral event %d: %v", event.ID, err)
			continue
		}

		// Mark as paid so it is never processed again
		if err := tx.Model(&event).Update("is_paid", true).Error; err != nil {
			tx.Rollback()
			log.Printf("[Cron] Failed to mark referral event %d as paid: %v", event.ID, err)
			continue
		}

		tx.Commit()
		log.Printf("[Cron] Paid referral event %d: %d coins to user %d", event.ID, event.Earnings, event.ReferrerID)
	}
}

// publishDailyWildCard generates a Daily Wild Card market if one doesn't exist for the day
func publishDailyWildCard() {
	// Simple check: Is there a Wild Card market created in the last 24h?
	var count int64
	yesterday := time.Now().Add(-24 * time.Hour)
	config.DB.Model(&models.Market{}).Where("category = ? AND created_at > ?", "Wild Card", yesterday).Count(&count)

	if count == 0 {
		// Create a mock wild card market
		start := time.Now()
		lock := start.Add(12 * time.Hour)
		res := lock.Add(12 * time.Hour)

		m := models.Market{
			Title: "Will it rain in London tomorrow?",
			Description: "Daily Wild Card. Predict the weather in London.",
			Category: "Wild Card",
			Difficulty: "Easy",
			Payout: 50,
			Options: `["Yes", "No"]`,
			ResolutionStatus: "Live",
			Visibility: "Public",
			StartTime: &start,
			LockTime: &lock,
			ResolutionTime: &res,
			EndDate: lock,
		}

		if err := config.DB.Create(&m).Error; err == nil {
			BroadcastToAll("market_live", "The Daily Wild Card market is now LIVE!")
		}
	}
}

// sendExpiryReminders finds coin batches expiring within 30 days that haven't received a reminder
func sendExpiryReminders() {
	now := time.Now()
	thirtyDaysFromNow := now.AddDate(0, 0, 30)

	var expiringBatches []models.CoinBatch
	config.DB.Where("expires_at <= ? AND balance > 0 AND reminder_sent_at IS NULL", thirtyDaysFromNow).Find(&expiringBatches)

	for _, batch := range expiringBatches {
		// In a real application, we would use an email service or push notification service here.
		// For now, we simulate sending the reminder.
		log.Printf("[Cron] Sending 30-day expiry reminder to UserID: %d for %d coins (Batch %d)", batch.UserID, batch.Balance, batch.ID)
		
		// Mark as sent
		batch.ReminderSentAt = &now
		config.DB.Save(&batch)
	}
}
