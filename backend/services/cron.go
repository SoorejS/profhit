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
