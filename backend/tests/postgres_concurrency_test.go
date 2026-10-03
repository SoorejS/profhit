package tests

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
)

func getPostgresDB(t *testing.T) *gorm.Config {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://profhit:test_db_password_12345@127.0.0.1:15432/profhit?sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping PostgreSQL test: could not connect to PostgreSQL at %s: %v", dsn, err)
		return nil
	}
	config.DB = db
	require.NoError(t, config.Migrate(db))
	return &gorm.Config{}
}

func pgTestUser(t *testing.T, baseName string, coins int) models.User {
	t.Helper()
	uniqueID := fmt.Sprintf("%s_%d_%d", baseName, time.Now().UTC().UnixNano(), rand.Intn(100000))
	u := models.User{
		Username:     uniqueID,
		Email:        uniqueID + "@test.invalid",
		Password:     "unused",
		ReferralCode: uniqueID,
		IsActive:     true,
	}
	require.NoError(t, config.DB.Create(&u).Error)
	if coins > 0 {
		require.NoError(t, services.CreditWallet(u.ID, coins, models.TxTypePurchase, 0, "test credit", nil))
	}
	return u
}

func verifyPostgresWalletInvariant(t *testing.T, userID uint) {
	t.Helper()
	var u models.User
	require.NoError(t, config.DB.First(&u, userID).Error)

	var ledgerSum int
	require.NoError(t, config.DB.Model(&models.WalletLedger{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(credit - debit), 0)").
		Scan(&ledgerSum).Error)

	var batchSum int
	require.NoError(t, config.DB.Model(&models.CoinBatch{}).
		Where("user_id = ? AND expires_at > ?", userID, time.Now().UTC()).
		Select("COALESCE(SUM(balance), 0)").
		Scan(&batchSum).Error)

	require.GreaterOrEqual(t, u.Points, 0, "User points must not be negative")
	require.GreaterOrEqual(t, ledgerSum, 0, "Ledger balance must not be negative")
	require.GreaterOrEqual(t, batchSum, 0, "Batch balance must not be negative")
	require.Equal(t, u.Points, ledgerSum, "User.Points must match ledger sum")
}

func TestPostgresWalletConcurrency(t *testing.T) {
	getPostgresDB(t)

	u := pgTestUser(t, "pg_wallet_conc", 1000)
	verifyPostgresWalletInvariant(t, u.ID)

	var wg sync.WaitGroup
	var successCount int64

	// Concurrent debits: 20 goroutines attempting to debit 60 points each (total 1200 attempted, only 16 can succeed on 1000 balance)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := services.DebitWallet(u.ID, 60, models.TxTypePredictionStake, 0, fmt.Sprintf("debit %d", idx), nil)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}
	wg.Wait()

	require.Equal(t, int64(16), successCount, "Exactly 16 debits of 60 points should succeed on 1000 balance")
	verifyPostgresWalletInvariant(t, u.ID)

	// Verify balance is exactly 40
	var finalUser models.User
	require.NoError(t, config.DB.First(&finalUser, u.ID).Error)
	require.Equal(t, 40, finalUser.Points)

	// Concurrent credits: 10 goroutines adding 25 points each
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := services.CreditWallet(u.ID, 25, models.TxTypeDailyLogin, 0, fmt.Sprintf("credit %d", idx), nil)
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()

	verifyPostgresWalletInvariant(t, u.ID)

	require.NoError(t, config.DB.First(&finalUser, u.ID).Error)
	require.Equal(t, 290, finalUser.Points, "40 + (10 * 25) = 290")
}

func TestPostgresPaymentSettlementConcurrency(t *testing.T) {
	getPostgresDB(t)

	u := pgTestUser(t, "pg_pay_conc", 100)
	orderID := fmt.Sprintf("order_pg_conc_%d_%d", time.Now().UTC().UnixNano(), rand.Intn(10000))
	paymentID := fmt.Sprintf("pay_pg_conc_%d_%d", time.Now().UTC().UnixNano(), rand.Intn(10000))

	tx := models.PaymentTransaction{
		ProviderOrderID: orderID,
		UserID:          u.ID,
		AmountPaise:     5000,
		Amount:          50.0,
		Status:          "Pending",
	}
	require.NoError(t, config.DB.Create(&tx).Error)

	var wg sync.WaitGroup
	var successCount int64

	// 10 concurrent settlement attempts for the exact same payment transaction
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := config.DB.Transaction(func(dbTx *gorm.DB) error {
				var p models.PaymentTransaction
				if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", tx.ID).First(&p).Error; err != nil {
					return err
				}
				if p.Status == "Completed" {
					return nil // Already completed
				}
				p.Status = "Completed"
				p.ProviderPaymentID = paymentID
				if err := dbTx.Save(&p).Error; err != nil {
					return err
				}
				if err := services.CreditWalletTx(dbTx, u.ID, 500, models.TxTypePurchase, 0, "PG test payment", nil); err != nil {
					return err
				}
				atomic.AddInt64(&successCount, 1)
				return nil
			})
			_ = err
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), successCount, "Payment must be settled exactly once")
	verifyPostgresWalletInvariant(t, u.ID)

	var updatedUser models.User
	require.NoError(t, config.DB.First(&updatedUser, u.ID).Error)
	require.Equal(t, 600, updatedUser.Points, "100 initial + 500 settled = 600 points")
}

func TestPostgresRedemptionStockConcurrency(t *testing.T) {
	getPostgresDB(t)

	item := models.RewardItem{
		Name:      fmt.Sprintf("PG Item %d", time.Now().UTC().UnixNano()),
		Cost:      100,
		Inventory: 3,
		IsActive:  true,
	}
	require.NoError(t, config.DB.Create(&item).Error)

	var wg sync.WaitGroup
	var successCount int64

	// 10 concurrent redemption attempts on an item with inventory = 3
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			buyer := pgTestUser(t, fmt.Sprintf("pg_redeemer_%d", idx), 200)

			err := config.DB.Transaction(func(dbTx *gorm.DB) error {
				var rItem models.RewardItem
				if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rItem, item.ID).Error; err != nil {
					return err
				}
				if rItem.Inventory <= 0 {
					return fmt.Errorf("out of stock")
				}
				rItem.Inventory--
				if err := dbTx.Save(&rItem).Error; err != nil {
					return err
				}

				if err := services.DebitWalletTx(dbTx, buyer.ID, rItem.Cost, models.TxTypeRedemption, 0, "Redemption", nil); err != nil {
					return err
				}

				redemption := models.Redemption{
					UserID:       buyer.ID,
					RewardItemID: rItem.ID,
					CostPaid:     rItem.Cost,
					Status:       "Approved",
				}
				return dbTx.Create(&redemption).Error
			})

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}
	wg.Wait()

	require.Equal(t, int64(3), successCount, "Exactly 3 redemptions must succeed for item with inventory = 3")

	var finalItem models.RewardItem
	require.NoError(t, config.DB.First(&finalItem, item.ID).Error)
	require.Equal(t, 0, finalItem.Inventory)
}

func TestPostgresMarketResolutionConcurrency(t *testing.T) {
	getPostgresDB(t)

	market := models.Market{
		Title:            fmt.Sprintf("PG Resolution Test %d", time.Now().UTC().UnixNano()),
		Category:         "Tech",
		ResolutionStatus: "Live",
		Options:          `["Yes","No"]`,
	}
	require.NoError(t, config.DB.Create(&market).Error)

	u1 := pgTestUser(t, "pg_mkt_u1", 100)
	u2 := pgTestUser(t, "pg_mkt_u2", 100)

	sub1 := models.PredictionSubmission{UserID: u1.ID, MarketID: market.ID, Choice: "Yes", Amount: 50, Potential: 100}
	sub2 := models.PredictionSubmission{UserID: u2.ID, MarketID: market.ID, Choice: "No", Amount: 50, Potential: 100}
	require.NoError(t, config.DB.Create(&sub1).Error)
	require.NoError(t, config.DB.Create(&sub2).Error)

	var wg sync.WaitGroup
	var resolveCount int64

	// 5 concurrent resolution attempts for the same market
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := config.DB.Transaction(func(dbTx *gorm.DB) error {
				var m models.Market
				if err := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, market.ID).Error; err != nil {
					return err
				}
				if m.ResolutionStatus == "Resolved" {
					return nil // Already resolved
				}
				m.ResolutionStatus = "Resolved"
				m.CorrectOption = "Yes"
				if err := dbTx.Save(&m).Error; err != nil {
					return err
				}

				// Payout logic
				var subs []models.PredictionSubmission
				if err := dbTx.Where("market_id = ?", m.ID).Find(&subs).Error; err != nil {
					return err
				}
				for _, s := range subs {
					isCorrect := (s.Choice == m.CorrectOption)
					s.IsCorrect = &isCorrect
					if err := dbTx.Save(&s).Error; err != nil {
						return err
					}
					if isCorrect {
						if err := services.CreditWalletTx(dbTx, s.UserID, s.Potential, models.TxTypePredictionWin, m.ID, "Market payout", nil); err != nil {
							return err
						}
					}
				}
				atomic.AddInt64(&resolveCount, 1)
				return nil
			})
			_ = err
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), resolveCount, "Market must be resolved exactly once")
	verifyPostgresWalletInvariant(t, u1.ID)
	verifyPostgresWalletInvariant(t, u2.ID)
}
