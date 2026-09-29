package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/services"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testUser(t *testing.T, name string, coins int) models.User {
	t.Helper()
	u := models.User{Username: name, Email: name + "@test.invalid", Password: "unused", ReferralCode: name, IsActive: true}
	require.NoError(t, config.DB.Create(&u).Error)
	if coins > 0 {
		require.NoError(t, services.CreditWallet(u.ID, coins, models.TxTypePurchase, 0, "test credit", nil))
	}
	return u
}
func request(t *testing.T, handler gin.HandlerFunc, userID uint, body string, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Set("role", models.RoleSuperAdmin)
	c.Params = params
	c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)
	return w
}
func invariant(t *testing.T, id uint, want int) {
	t.Helper()
	var u models.User
	require.NoError(t, config.DB.First(&u, id).Error)
	var ledger, batches int
	require.NoError(t, config.DB.Model(&models.WalletLedger{}).Where("user_id = ?", id).Select("COALESCE(SUM(credit-debit),0)").Scan(&ledger).Error)
	require.NoError(t, config.DB.Model(&models.CoinBatch{}).Where("user_id = ?", id).Select("COALESCE(SUM(balance),0)").Scan(&batches).Error)
	require.Equal(t, want, u.Points)
	require.Equal(t, want, ledger)
	require.Equal(t, want, batches)
}
func TestWalletFIFOExpiryAndRollback(t *testing.T) {
	setupTestDB()
	u := testUser(t, "fifo", 100)
	var old models.CoinBatch
	config.DB.First(&old)
	require.NoError(t, services.CreditWallet(u.ID, 80, models.TxTypePurchase, 0, "new", nil))
	require.NoError(t, services.DebitWallet(u.ID, 120, models.TxTypePredictionStake, 0, "stake", nil))
	config.DB.First(&old, old.ID)
	require.Zero(t, old.Balance)
	invariant(t, u.ID, 60)
	require.Error(t, services.DebitWallet(u.ID, 61, models.TxTypePredictionStake, 0, "overdraw", nil))
	invariant(t, u.ID, 60)
	var batch models.CoinBatch
	config.DB.Where("balance > 0").First(&batch)
	require.NoError(t, config.DB.Model(&batch).Update("expires_at", time.Now().Add(-time.Hour)).Error)
	require.NoError(t, services.CreditWallet(u.ID, 40, models.TxTypePurchase, 0, "fresh", nil))
	require.Error(t, services.DebitWallet(u.ID, 50, models.TxTypePredictionStake, 0, "expired spend", nil))
	invariant(t, u.ID, 100)
	require.NoError(t, services.ExpireCoinBatch(batch.ID))
	require.NoError(t, services.ExpireCoinBatch(batch.ID))
	invariant(t, u.ID, 40)
	require.Error(t, services.DebitWallet(u.ID, -1, models.TxTypePredictionStake, 0, "invalid", nil))
	invariant(t, u.ID, 40)
}
func TestWalletConcurrentDebits(t *testing.T) {
	setupTestDB()
	u := testUser(t, "concurrent", 100)
	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if services.DebitWallet(u.ID, 10, models.TxTypePredictionStake, 0, "concurrent", nil) == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(10), success.Load())
	invariant(t, u.ID, 0)
}
func TestReminderIsPersistentAndIdempotent(t *testing.T) {
	setupTestDB()
	u := testUser(t, "reminder", 100)
	var b models.CoinBatch
	config.DB.First(&b)
	config.DB.Model(&b).Update("expires_at", time.Now().Add(24*time.Hour))
	require.NoError(t, services.SendExpiryReminder(b.ID))
	require.NoError(t, services.SendExpiryReminder(b.ID))
	var n int64
	config.DB.Model(&models.Notification{}).Where("user_id = ?", u.ID).Count(&n)
	require.Equal(t, int64(1), n)
}
func TestReferralDelayDuplicateAndCap(t *testing.T) {
	setupTestDB()
	ref := testUser(t, "referrer", 0)
	child := testUser(t, "child", 0)
	require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.ProcessReferral(tx, child.ID, ref.ReferralCode) }))
	require.Error(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.ProcessReferral(tx, child.ID, ref.ReferralCode) }))
	require.Error(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.ProcessReferral(tx, ref.ID, ref.ReferralCode) }))
	var e models.ReferralEvent
	config.DB.Where("referrer_id = ?", ref.ID).First(&e)
	require.NoError(t, services.PayReferralEvent(e.ID))
	invariant(t, ref.ID, 0)
	config.DB.Model(&e).Update("pending_until", time.Now().Add(-time.Hour))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = services.PayReferralEvent(e.ID) }()
	}
	wg.Wait()
	invariant(t, ref.ID, 50)
	for i := 0; i < 25; i++ {
		u := testUser(t, fmt.Sprintf("child%d", i), 0)
		require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.ProcessReferral(tx, u.ID, ref.ReferralCode) }))
	}
	var count int64
	config.DB.Model(&models.ReferralEvent{}).Where("referrer_id = ?", ref.ID).Count(&count)
	require.Equal(t, int64(20), count)
}
func TestPredictionCutoffDailyLimitAndResolution(t *testing.T) {
	setupTestDB()
	u := testUser(t, "predictor", 100)
	admin := testUser(t, "admin", 0)
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Second)
	m := models.Market{Title: "Cutoff", Category: "Sports", Options: `["Yes","No"]`, Payout: 20, ResolutionStatus: "Live", LockTime: &past, EndDate: future}
	require.NoError(t, config.DB.Create(&m).Error)
	body := fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, m.ID)
	require.Equal(t, 400, request(t, controllers.SubmitPrediction, u.ID, body, nil).Code)
	invariant(t, u.ID, 100)
	config.DB.Model(&m).Update("lock_time", future)
	require.Equal(t, 200, request(t, controllers.SubmitPrediction, u.ID, body, nil).Code)
	other := m
	other.ID = 0
	require.NoError(t, config.DB.Create(&other).Error)
	require.Equal(t, 429, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, other.ID), nil).Code)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(m.ID)}}
	require.Equal(t, 400, request(t, controllers.ResolveMarket, admin.ID, `{"winner":"Yes"}`, params).Code)
	config.DB.Model(&m).Update("resolution_status", "Locked")
	require.Equal(t, 200, request(t, controllers.ResolveMarket, admin.ID, `{"winner":"Yes"}`, params).Code)
	require.Equal(t, 400, request(t, controllers.ResolveMarket, admin.ID, `{"winner":"Yes"}`, params).Code)
	invariant(t, u.ID, 160) // 100 - 10 stake + 50 achievement + 20 payout.
	require.Equal(t, 400, request(t, controllers.TransitionMarketState, admin.ID, `{"status":"Live"}`, params).Code)
}
func TestDailyLoginConcurrentExactlyOnce(t *testing.T) {
	setupTestDB()
	u := testUser(t, "daily", 0)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); request(t, controllers.DailyLogin, u.ID, `{}`, nil) }()
	}
	wg.Wait()
	invariant(t, u.ID, 10)
}
func TestAchievementClaimIsAtomic(t *testing.T) {
	setupTestDB()
	u := testUser(t, "achievement", 0)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); services.UnlockAchievement(u.ID, "test", "Test", "Test", 50, "") }()
	}
	wg.Wait()
	invariant(t, u.ID, 50)
}
func TestAuthUsesCurrentRoleAndPersistentRevocation(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	u := testUser(t, "session", 0)
	u.Role = models.RoleAdmin
	config.DB.Model(&u).Update("role", u.Role)
	token, err := controllers.GenerateToken(u)
	require.NoError(t, err)
	config.DB.Model(&u).Update("role", models.RoleUser)
	router := gin.New()
	router.GET("/", middleware.AuthRequired(), middleware.RoleRequired(models.RoleAdmin), func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	require.NoError(t, middleware.InvalidateToken(token))
	_, _, err = middleware.ValidateToken(token)
	require.Error(t, err)
	var n int64
	config.DB.Model(&models.RevokedToken{}).Count(&n)
	require.Equal(t, int64(1), n)
}
func TestWalletOwnershipAndStringIDInjection(t *testing.T) {
	setupTestDB()
	alice := testUser(t, "alice", 100)
	bob := testUser(t, "bob", 50)
	var entry models.WalletLedger
	config.DB.Where("user_id = ?", alice.ID).First(&entry)
	require.Equal(t, 404, request(t, controllers.GetWalletTransaction, bob.ID, "", gin.Params{{Key: "id", Value: fmt.Sprint(entry.ID)}}).Code)
	require.Equal(t, 404, request(t, controllers.GetUser, bob.ID, "", gin.Params{{Key: "id", Value: "1 OR 1=1"}}).Code)
}
func TestEmptyWebhookSecretsFailClosed(t *testing.T) {
	setupTestDB()
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "")
	t.Setenv("HYPERVERGE_WEBHOOK_SECRET", "")
	require.Equal(t, 503, request(t, controllers.RazorpayWebhook, 0, `{}`, nil).Code)
	require.Equal(t, 503, request(t, controllers.HypervergeWebhook, 0, `{}`, nil).Code)
}
func TestCapturedWebhookExactlyOnceAndAmountChecked(t *testing.T) {
	setupTestDB()
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "testwebhook")
	u := testUser(t, "paid", 0)
	require.NoError(t, config.DB.Create(&models.PaymentTransaction{UserID: u.ID, ProviderOrderID: "order_webhook", Status: "Pending", AmountPaise: 5000, Amount: 50}).Error)
	call := func(amount int) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_webhook","order_id":"order_webhook","amount":%d,"currency":"INR","status":"captured"}}}}`, amount)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
		h := hmac.New(sha256.New, []byte("testwebhook"))
		h.Write([]byte(body))
		c.Request.Header.Set("X-Razorpay-Signature", hex.EncodeToString(h.Sum(nil)))
		controllers.RazorpayWebhook(c)
		return w
	}
	require.Equal(t, 500, call(100).Code)
	invariant(t, u.ID, 0)
	require.Equal(t, 200, call(5000).Code)
	require.Equal(t, 200, call(5000).Code)
	invariant(t, u.ID, 50)
}
