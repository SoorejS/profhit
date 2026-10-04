package tests

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/services"
	"sync"
	"testing"
	"time"
)

func compliantTestMarket(category, difficulty string, cutoff time.Time) models.Market {
	sources := map[string]string{"Weather": "https://openweathermap.org/results", "Sports": "https://www.espn.com/results", "Politics": "https://eci.gov.in/results", "Entertainment": "https://oscars.org/results", "Financial Markets": "https://nseindia.com/results", "Wild Card": "https://nasa.gov/results", "Technology": "https://apple.com/results", "Geopolitics": "https://un.org/results"}
	m := models.Market{Title: "Measurable audit prediction", Category: category, Difficulty: difficulty, Options: `["Yes","No"]`, ResolutionStatus: "Live", Visibility: "Public", LockTime: &cutoff, EndDate: cutoff, ResolutionSource: sources[category], ResolutionRule: "Published observation in the declared units at cutoff", RangeWidth: 2}
	if (category == "Wild Card" || category == "Technology" || category == "Geopolitics") && difficulty == "Medium" {
		m.Options = `["A","B","C","D"]`
	}
	if category == "Entertainment" && difficulty == "Medium" {
		m.Options = `["A","B","C","D"]`
	}
	if services.ConfigurePrediction(&m) != nil {
		panic("invalid test market")
	}
	return m
}

func TestMyStatsCountsOnlyOwnedSettledPredictions(t *testing.T) {
	setupTestDB()
	owner := testUser(t, "summary_owner", 0)
	other := testUser(t, "summary_other", 0)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	require.NoError(t, config.DB.Create(&m).Error)
	yes, no := true, false
	for i, correct := range []*bool{&yes, &no, nil} {
		next := compliantTestMarket("Sports", "Easy", time.Now().UTC().Add(time.Hour))
		require.NoError(t, config.DB.Create(&next).Error)
		require.NoError(t, config.DB.Create(&models.PredictionSubmission{UserID: owner.ID, MarketID: next.ID, Choice: "Yes", Potential: 25, IsCorrect: correct, CreatedAt: time.Now().UTC().AddDate(0, 0, -i)}).Error)
	}
	require.NoError(t, config.DB.Create(&models.PredictionSubmission{UserID: other.ID, MarketID: m.ID, Choice: "Yes", Potential: 20, IsCorrect: &yes}).Error)
	response := request(t, controllers.GetMyStats, owner.ID, "", nil)
	require.Equal(t, 200, response.Code)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, float64(3), result["total_predictions"])
	require.Equal(t, "50.0%", result["win_rate"])
	require.Equal(t, float64(25), result["pending_potential"])
}

func TestUnbanDoesNotRestorePreviousSessions(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", "disposable-session-revocation-regression-secret")
	admin := testUser(t, "ban_operator", 0)
	target := testUser(t, "ban_target", 0)
	old, err := controllers.GenerateToken(target)
	require.NoError(t, err)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(target.ID)}}
	require.Equal(t, 200, request(t, controllers.BanUser, admin.ID, "", params).Code)
	require.Equal(t, 200, request(t, controllers.UnbanUser, admin.ID, "", params).Code)
	_, _, err = middleware.ValidateToken(old)
	require.Error(t, err)
	require.NoError(t, config.DB.First(&target, target.ID).Error)
	fresh, err := controllers.GenerateToken(target)
	require.NoError(t, err)
	_, _, err = middleware.ValidateToken(fresh)
	require.NoError(t, err)
}

// Test-only evidence fixture: never exposed through an application verification route.
func grantTestVerification(t *testing.T, id uint) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, config.DB.Create(&models.HyperVergeKYC{UserID: id, SessionID: fmt.Sprint("verified_", id), Status: "Verified", VerifiedAt: &now, PhoneVerifiedAt: &now, EmailConfirmedAt: &now}).Error)
}

func TestPDFExpiryCalendarMigrationAndRefund(t *testing.T) {
	require.Equal(t, "2027-02-28T12:30:00Z", services.CoinExpiry(time.Date(2026, 8, 31, 12, 30, 0, 0, time.UTC)).Format(time.RFC3339))
	require.Equal(t, "2028-02-29T12:30:00Z", services.CoinExpiry(time.Date(2027, 8, 31, 12, 30, 0, 0, time.UTC)).Format(time.RFC3339))
	setupTestDB()
	u := testUser(t, "expiry_pdf", 100)
	var b models.CoinBatch
	require.NoError(t, config.DB.First(&b).Error)
	require.Equal(t, services.CoinExpiry(b.CreatedAt), b.ExpiresAt)
	require.NoError(t, config.DB.Model(&b).Update("expires_at", b.CreatedAt.AddDate(1, 0, 0)).Error)
	rows, err := services.PreviewPDFExpiryMigration()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Error(t, services.ApplyPDFExpiryMigration(""))
	require.NoError(t, services.ApplyPDFExpiryMigration("local audit"))
	require.NoError(t, services.ApplyPDFExpiryMigration("local audit"))
	invariant(t, u.ID, 100)
	require.NoError(t, config.DB.First(&b, b.ID).Error)
	originalExpiry := b.ExpiresAt
	require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.DebitRedemptionTx(tx, u.ID, 50, "tier", 1) }))
	require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.RefundRedemptionTx(tx, u.ID, "tier", 1, nil) }))
	require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.RefundRedemptionTx(tx, u.ID, "tier", 1, nil) }))
	invariant(t, u.ID, 100)
	require.NoError(t, config.DB.First(&b, b.ID).Error)
	require.Equal(t, originalExpiry, b.ExpiresAt)
	require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.DebitRedemptionTx(tx, u.ID, 50, "tier", 2) }))
	require.NoError(t, config.DB.Model(&b).Update("expires_at", time.Now().UTC().Add(-time.Hour)).Error)
	require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.RefundRedemptionTx(tx, u.ID, "tier", 2, nil) }))
	require.NoError(t, services.ExpireCoinBatch(b.ID))
	invariant(t, u.ID, 0)
}

func TestPDFFreePlayAndConcurrentSubmission(t *testing.T) {
	setupTestDB()
	u := testUser(t, "zero_balance", 0)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	require.NoError(t, config.DB.Create(&m).Error)
	paid := request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, m.ID), nil)
	require.Equal(t, 400, paid.Code)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes"}`, m.ID), nil)
		}()
	}
	wg.Wait()
	var count int64
	require.NoError(t, config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ?", u.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	invariant(t, u.ID, 0)
	require.Equal(t, 410, request(t, controllers.CreateRazorpayOrder, u.ID, `{"amount":100}`, nil).Code)
	require.Equal(t, 410, request(t, controllers.VerifyPayment, u.ID, `{}`, nil).Code)
	legacy := m
	legacy.ID = 0
	legacy.Category = "Sports"
	legacy.PredictionType = ""
	require.NoError(t, config.DB.Create(&legacy).Error)
	require.Equal(t, 409, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes"}`, legacy.ID), nil).Code)
}

func TestPDFPredictionStreakAndDailyLoginAreSeparate(t *testing.T) {
	setupTestDB()
	u := testUser(t, "streak_pdf", 0)
	start := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -8)
	for d := 0; d < 8; d++ {
		at := start.AddDate(0, 0, d)
		var wg sync.WaitGroup
		for j := 0; j < 4; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.RecordPredictionDayTx(tx, u.ID, at) }))
			}()
		}
		wg.Wait()
	}
	invariant(t, u.ID, 100)
	var row models.PredictionStreak
	require.NoError(t, config.DB.First(&row).Error)
	require.Equal(t, 8, row.CurrentStreak)
	require.Equal(t, 8, row.TotalDays)
	for d := 0; d < 8; d++ {
		claimed, err := services.ClaimDailyLogin(u.ID, start.AddDate(0, 0, d))
		require.NoError(t, err)
		require.True(t, claimed)
	}
	invariant(t, u.ID, 180)
	require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.RecordPredictionDayTx(tx, u.ID, start.AddDate(0, 0, 10)) }))
	require.NoError(t, config.DB.First(&row, row.ID).Error)
	require.Equal(t, 1, row.CurrentStreak)
}

func TestPDFProfileThirtyOnceUnderConcurrency(t *testing.T) {
	setupTestDB()
	u := testUser(t, "profile_pdf", 0)
	require.Equal(t, 400, request(t, controllers.UpdateProfile, u.ID, `{"full_name":" "}`, nil).Code)
	body := `{"full_name":"Audit Person","phone":"+919000000000","city":"Pune","country":"India","interests":"Sports"}`
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(t, controllers.UpdateProfile, u.ID, body, nil)
			require.Equal(t, 200, w.Code, w.Body.String())
		}()
	}
	wg.Wait()
	services.CheckProfileCompletion(u.ID)
	invariant(t, u.ID, 30)
	var stored models.User
	require.NoError(t, config.DB.First(&stored, u.ID).Error)
	require.False(t, stored.KycStatus)
	require.False(t, stored.TwoFactorEnabled)
}

func testPDFReferral(t *testing.T) {
	setupTestDB()
	ref := testUser(t, "referrer", 0)
	for i := 0; i < 25; i++ {
		child := testUser(t, fmt.Sprintf("child%d", i), 0)
		require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.ProcessReferral(tx, child.ID, ref.ReferralCode) }))
		require.Error(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.ProcessReferral(tx, child.ID, ref.ReferralCode) }))
		var before int64
		config.DB.Model(&models.ReferralEvent{}).Where("referred_id = ?", child.ID).Count(&before)
		require.Zero(t, before)
		m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
		require.NoError(t, config.DB.Create(&m).Error)
		require.Equal(t, 200, request(t, controllers.SubmitPrediction, child.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes"}`, m.ID), nil).Code)
		require.NoError(t, config.DB.Transaction(func(tx *gorm.DB) error {
			return services.TriggerReferralEventTx(tx, child.ID, models.ReferralStatusKYCCompleted, 200)
		}))
	}
	require.Error(t, config.DB.Transaction(func(tx *gorm.DB) error { return services.ProcessReferral(tx, ref.ID, ref.ReferralCode) }))
	var events []models.ReferralEvent
	require.NoError(t, config.DB.Where("referrer_id = ?", ref.ID).Find(&events).Error)
	require.Len(t, events, 20)
	for _, e := range events {
		require.Equal(t, 50, e.Earnings)
		require.InDelta(t, 48, e.PendingUntil.Sub(e.CreatedAt).Hours(), 0.01)
		require.NoError(t, services.PayReferralEvent(e.ID))
	}
	invariant(t, ref.ID, 0)
	require.NoError(t, config.DB.Model(&models.ReferralEvent{}).Where("referrer_id = ?", ref.ID).Update("pending_until", time.Now().UTC().Add(-time.Hour)).Error)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, e := range events {
				require.NoError(t, services.PayReferralEvent(e.ID))
			}
		}()
	}
	wg.Wait()
	invariant(t, ref.ID, 1000)
}

func TestPDFAllCategoryPayoutsAndTypedSettlement(t *testing.T) {
	expected := map[string][3]int{"Weather": {20, 50, 120}, "Sports": {25, 60, 200}, "Politics": {30, 80, 250}, "Entertainment": {25, 70, 150}, "Financial Markets": {20, 80, 300}, "Wild Card": {40, 100, 400}}
	for category, payouts := range expected {
		for i, difficulty := range []string{"Easy", "Medium", "Hard"} {
			m := compliantTestMarket(category, difficulty, time.Now().UTC().Add(time.Hour))
			require.Equal(t, payouts[i], m.Payout)
		}
	}
	cases := []struct {
		category, difficulty, guess, outcome string
		want                                 bool
	}{
		{"Weather", "Easy", "Yes", "Yes", true}, {"Weather", "Medium", "[10,12]", "11", true}, {"Weather", "Hard", "30.5", "30.5", true},
		{"Sports", "Hard", "[2,1]", "[2,1]", true}, {"Sports", "Hard", "215", "216", false}, {"Politics", "Medium", "10", "15", true}, {"Politics", "Medium", "10", "15.1", false}, {"Politics", "Hard", "250", "250", true},
		{"Entertainment", "Medium", `["A","C","B"]`, `["A","B","C"]`, true}, {"Entertainment", "Hard", "[10,12]", "12", true}, {"Financial Markets", "Hard", "100", "101", true}, {"Wild Card", "Hard", "100", "101", true}}
	for _, c := range cases {
		m := compliantTestMarket(c.category, c.difficulty, time.Now().UTC().Add(time.Hour))
		guess, err := services.ValidatePredictionValue(m, c.guess, false)
		require.NoError(t, err)
		wins, err := services.PredictionWinners(m, c.outcome, []models.PredictionSubmission{{ID: 1, Choice: guess}})
		require.NoError(t, err)
		require.Equal(t, c.want, wins[1], fmt.Sprint(c))
	}
	for _, raw := range []string{"NaN", "Infinity", "subjective opinion"} {
		_, err := services.ValidatePredictionValue(compliantTestMarket("Wild Card", "Hard", time.Now().UTC().Add(time.Hour)), raw, false)
		require.Error(t, err)
	}
	_, err := services.ValidatePredictionValue(compliantTestMarket("Entertainment", "Medium", time.Now().UTC().Add(time.Hour)), `["A","A","B"]`, false)
	require.Error(t, err)
	m := compliantTestMarket("Financial Markets", "Hard", time.Now().UTC().Add(time.Hour))
	wins, err := services.PredictionWinners(m, "100", []models.PredictionSubmission{{ID: 1, Choice: "99"}, {ID: 2, Choice: "101"}, {ID: 3, Choice: "105"}})
	require.NoError(t, err)
	require.True(t, wins[1])
	require.True(t, wins[2])
	require.False(t, wins[3])
	require.False(t, services.ApprovedResultURL("Weather", "https://openweathermap.org.evil.invalid/results"))
}

func TestPDFWeeklyChallengeConcurrentSettlement(t *testing.T) {
	setupTestDB()
	u := testUser(t, "challenge_pdf", 0)
	admin := testUser(t, "challenge_admin", 0)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	require.NoError(t, config.DB.Create(&m).Error)
	require.Equal(t, 201, request(t, controllers.CreateWeeklyChallenge, admin.ID, fmt.Sprintf(`{"market_id":%d}`, m.ID), nil).Code)
	require.Equal(t, 400, request(t, controllers.CreateWeeklyChallenge, admin.ID, fmt.Sprintf(`{"market_id":%d}`, m.ID), nil).Code)
	require.Equal(t, 200, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes"}`, m.ID), nil).Code)
	require.Equal(t, 429, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes"}`, m.ID), nil).Code)
	past := time.Now().UTC().Add(-time.Second)
	require.NoError(t, config.DB.Model(&m).Updates(map[string]interface{}{"resolution_status": "Locked", "lock_time": past}).Error)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(m.ID)}}
	require.Equal(t, 400, request(t, controllers.ResolveMarket, admin.ID, `{"winner":"Yes"}`, params).Code)
	body := fmt.Sprintf(`{"winner":"Yes","evidence_url":"https://openweathermap.org/results","observed_at":"%s"}`, time.Now().UTC().Format(time.RFC3339Nano))
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); request(t, controllers.ResolveMarket, admin.ID, body, params) }()
	}
	wg.Wait()
	invariant(t, u.ID, 40)
	var c models.WeeklyChallenge
	require.NoError(t, config.DB.First(&c).Error)
	require.Equal(t, "Completed", c.Status)
	var p models.ChallengeParticipant
	require.NoError(t, config.DB.First(&p).Error)
	require.Equal(t, 40, p.RewardWon)
	require.Equal(t, 1, p.Score)
}

func TestPDFRedemptionGateTiersAndVoucherLifecycle(t *testing.T) {
	setupTestDB()
	t.Setenv("VOUCHER_ENCRYPTION_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("SMTP_HOST", "")
	u := testUser(t, "voucher_pdf", 20000)
	require.Equal(t, 403, request(t, controllers.RedeemVoucher, u.ID, `{"tier":"Bronze"}`, nil).Code)
	grantTestVerification(t, u.ID)
	tiers := []string{"Bronze", "Silver", "Gold", "Platinum", "Diamond"}
	costs := []int{500, 1200, 2500, 5000, 10000}
	values := []int{50, 150, 350, 800, 2000}
	for i, tier := range tiers {
		w := request(t, controllers.RedeemVoucher, u.ID, fmt.Sprintf(`{"tier":%q}`, tier), nil)
		require.Equal(t, 200, w.Code, w.Body.String())
		var row models.WithdrawalRequest
		require.NoError(t, config.DB.Where("tier = ?", tier).First(&row).Error)
		require.Equal(t, costs[i], row.CoinsDeducted)
		require.Equal(t, values[i], row.Amount)
	}
	var row models.WithdrawalRequest
	require.NoError(t, config.DB.Where("tier = ?", "Silver").First(&row).Error)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(row.ID)}}
	require.Equal(t, 409, request(t, controllers.FulfillVoucher, u.ID, `{"voucher_code":"LOCAL-TEST-NOT-VALID","source_reference":"Disposable test supplier invoice"}`, params).Code)
	require.Equal(t, 200, request(t, controllers.ApproveWithdrawal, u.ID, `{}`, params).Code)
	require.Equal(t, 200, request(t, controllers.FulfillVoucher, u.ID, `{"voucher_code":"LOCAL-TEST-NOT-VALID","source_reference":"Disposable test supplier invoice"}`, params).Code)
	require.Equal(t, 503, request(t, controllers.DeliverVoucher, u.ID, `{}`, params).Code)
	require.Equal(t, 409, request(t, controllers.DeliverVoucher, u.ID, `{}`, params).Code)
	require.NoError(t, config.DB.First(&row, row.ID).Error)
	require.Equal(t, "Failed", row.Status)
	require.Nil(t, row.DeliveredAt)
	var badges int64
	require.NoError(t, config.DB.Model(&models.UserBadge{}).Where("user_id = ?", u.ID).Count(&badges).Error)
	require.Equal(t, int64(1), badges)
	old := time.Now().UTC().AddDate(-1, 0, -1)
	require.NoError(t, config.DB.Model(&models.HyperVergeKYC{}).Where("user_id = ?", u.ID).Update("verified_at", old).Error)
	require.False(t, services.RedemptionVerified(config.DB, u.ID, time.Now().UTC()))
}

func TestPDFNewsFreshnessDuplicateAndEditorialGate(t *testing.T) {
	setupTestDB()
	now := time.Now().UTC()
	a := services.Article{Title: "Dated actual provider headline", URL: "https://news.example.invalid/item", PublishedAt: now.Add(-time.Hour).Format(time.RFC3339)}
	m, err := services.PrepareNewsCandidate(a, now)
	require.NoError(t, err)
	require.Equal(t, "Draft", m.ResolutionStatus)
	require.Equal(t, a.URL, m.NewsURL)
	require.Empty(t, m.ResolutionRule)
	require.Empty(t, m.ResolutionSource)
	_, err = services.PrepareNewsCandidate(a, now)
	require.Error(t, err)
	a.PublishedAt = now.Add(-25 * time.Hour).Format(time.RFC3339)
	_, err = services.PrepareNewsCandidate(a, now)
	require.Error(t, err)
	admin := testUser(t, "news_editor", 0)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(m.ID)}}
	require.Equal(t, 400, request(t, controllers.ApproveMarket, admin.ID, `{}`, params).Code)
	reviewed := compliantTestMarket("Wild Card", "Easy", now.Add(time.Hour))
	reviewed.Description = "An isolated upcoming observation tests reviewed publication of measurable rules."
	resolution := now.Add(2 * time.Hour)
	reviewed.ResolutionTime = &resolution
	body, _ := json.Marshal(reviewed)
	require.Equal(t, 200, request(t, controllers.UpdateMarketRules, admin.ID, string(body), params).Code)
	require.Equal(t, 200, request(t, controllers.ApproveMarket, admin.ID, `{"current":true,"future_outcome":true,"objective":true,"trusted_source":true,"sensible_cutoff":true,"interesting":true,"rationale":"Isolated regression verifies a complete and accountable editorial publication review."}`, params).Code)
}
