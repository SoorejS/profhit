package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func setupProviderTestDB() {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	config.DB = db

	// Drop tables first to prevent shared memory cache contamination between tests
	config.DB.Migrator().DropTable(
		&models.User{},
		&models.Market{},
		&models.PredictionSubmission{},
		&models.Comment{},
		&models.HyperVergeKYC{},
		&models.WithdrawalRequest{},
		&models.WalletLedger{},
		&models.UserStreak{},
		&models.PasswordResetToken{},
		&models.ReferralEvent{},
		&models.AuditLog{},
		&models.Report{},
		&models.WeeklyChallenge{},
		&models.ChallengeParticipant{},
		&models.Achievement{},
		&models.UserAchievement{},
		&models.Badge{},
		&models.UserBadge{},
		&models.RewardItem{},
		&models.Redemption{},
		&models.CoinBatch{},
		&models.PaymentTransaction{},
	)

	config.DB.AutoMigrate(
		&models.User{},
		&models.Market{},
		&models.PredictionSubmission{},
		&models.Comment{},
		&models.HyperVergeKYC{},
		&models.WithdrawalRequest{},
		&models.WalletLedger{},
		&models.UserStreak{},
		&models.PasswordResetToken{},
		&models.ReferralEvent{},
		&models.AuditLog{},
		&models.Report{},
		&models.WeeklyChallenge{},
		&models.ChallengeParticipant{},
		&models.Achievement{},
		&models.UserAchievement{},
		&models.Badge{},
		&models.UserBadge{},
		&models.RewardItem{},
		&models.Redemption{},
		&models.CoinBatch{},
		&models.PaymentTransaction{},
	)
}

func TestPaymentVerificationIdempotency(t *testing.T) {
	setupProviderTestDB()
	gin.SetMode(gin.TestMode)

	user := models.User{Username: "tester", Points: 0}
	config.DB.Create(&user)

	os.Setenv("RAZORPAY_KEY_SECRET", "testsecret")

	// Payload
	payload := controllers.PaymentVerification{
		RazorpayOrderID:   "order_123",
		RazorpayPaymentID: "pay_123",
		Points:            500,
	}

	// Generate valid signature
	data := payload.RazorpayOrderID + "|" + payload.RazorpayPaymentID
	h := hmac.New(sha256.New, []byte("testsecret"))
	h.Write([]byte(data))
	payload.RazorpaySignature = hex.EncodeToString(h.Sum(nil))

	body, _ := json.Marshal(payload)

	router := gin.Default()
	// Mock authentication middleware by injecting UserID
	router.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	router.POST("/verify", controllers.VerifyPayment)

	// First request - should succeed
	req1, _ := http.NewRequest("POST", "/verify", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)

	// Wait for async referral bonus to trigger
	time.Sleep(100 * time.Millisecond)

	var u1 models.User
	config.DB.First(&u1, user.ID)
	assert.Equal(t, 500, u1.Points) // 500 deposit (referral bonus is pending for 48h)

	// Second request (Duplicate/Replay) - should return OK but NOT credit funds
	req2, _ := http.NewRequest("POST", "/verify", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code) // Idempotent success response

	var u2 models.User
	config.DB.First(&u2, user.ID)
	assert.Equal(t, 500, u2.Points) // Balance should STILL be 500
}

func TestHyperVergeRetryBackoff(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup a mock server that fails twice then succeeds
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"result": map[string]string{
				"redirectUrl": "https://kyc.mock.com/verify",
			},
		})
	}))
	defer server.Close()

	os.Setenv("HYPERVERGE_API_URL", server.URL)
	os.Setenv("HYPERVERGE_API_KEY", "appID")
	os.Setenv("HYPERVERGE_API_SECRET", "appKey")
	os.Setenv("HYPERVERGE_WORKFLOW_ID", "wf_123")

	setupProviderTestDB()
	user := models.User{Username: "kyctester", Points: 0}
	config.DB.Create(&user)

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	router.POST("/kyc/start", controllers.StartKYCSession)

	req, _ := http.NewRequest("POST", "/kyc/start", nil)
	w := httptest.NewRecorder()

	startTime := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(startTime)

	assert.Equal(t, http.StatusOK, w.Code)
	// Retry loop does 500ms + 1000ms = 1500ms min delay
	assert.GreaterOrEqual(t, duration.Milliseconds(), int64(1500))
	assert.Equal(t, 3, attempts)
}

func TestWebhookSignatureValidation(t *testing.T) {
	setupProviderTestDB()
	gin.SetMode(gin.TestMode)

	os.Setenv("RAZORPAY_WEBHOOK_SECRET", "webhooksecret")

	router := gin.Default()
	router.POST("/webhook", controllers.RazorpayWebhook)

	payload := `{"event": "payment.authorized"}`

	// Request with missing signature
	req1, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer([]byte(payload)))
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusBadRequest, w1.Code)

	// Request with invalid signature
	req2, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer([]byte(payload)))
	req2.Header.Set("X-Razorpay-Signature", "wrong_signature")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	// Request with valid signature
	h := hmac.New(sha256.New, []byte("webhooksecret"))
	h.Write([]byte(payload))
	validSig := hex.EncodeToString(h.Sum(nil))

	req3, _ := http.NewRequest("POST", "/webhook", bytes.NewBuffer([]byte(payload)))
	req3.Header.Set("X-Razorpay-Signature", validSig)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}
