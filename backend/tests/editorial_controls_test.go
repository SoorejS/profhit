package tests

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/models"
	"profhit-backend/services"
	"sync"
	"testing"
	"time"
)

func TestEditorialGateCannotBeBypassedByTransitionOrForgedCreate(t *testing.T) {
	setupTestDB()
	u := testUser(t, "editorial_admin", 0)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.Title = "Will Chennai's next weather bulletin predict heavy rain?"
	m.Description = "An upcoming official weather bulletin determines whether heavy rain warnings are issued."
	m.ResolutionStatus = "Draft"
	m.IsCurated = true
	m.SourceKind = "official_event"
	m.NewsURL = m.ResolutionSource
	m.NewsSourceName = "IMD"
	now := time.Now().UTC()
	m.NewsDiscoveredAt = &now
	result := now.Add(2 * time.Hour)
	m.ResolutionTime = &result
	require.NoError(t, config.DB.Create(&m).Error)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(m.ID)}}
	require.Equal(t, 400, request(t, controllers.TransitionMarketState, u.ID, `{"status":"Live"}`, params).Code)
	require.Equal(t, 400, request(t, controllers.ApproveMarket, u.ID, `{}`, params).Code)
	review := `{"current":true,"future_outcome":true,"objective":true,"trusted_source":true,"sensible_cutoff":true,"interesting":true,"rationale":"An official weather warning can affect upcoming travel and outdoor plans."}`
	require.Equal(t, 200, request(t, controllers.ApproveMarket, u.ID, review, params).Code)
	require.NoError(t, config.DB.First(&m, m.ID).Error)
	require.Equal(t, u.ID, m.EditorialReviewedBy)
	m.ID = 0
	m.Title = "Different future bulletin for Chennai?"
	m.ResolutionStatus = "Live"
	m.EditorialReviewedBy = 999
	m.IsFeatured = true
	body, _ := json.Marshal(m)
	w := request(t, controllers.CreateMarket, u.ID, string(body), nil)
	require.Equal(t, 201, w.Code, w.Body.String())
	var created models.Market
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "Draft", created.ResolutionStatus)
	require.Zero(t, created.EditorialReviewedBy)
	require.False(t, created.IsFeatured)
}
func TestVoidRefundsExactlyOnceAndCannotThenSettle(t *testing.T) {
	setupTestDB()
	verifyVoidRefund(t, false)
}
func TestPostgresVoidRefundInvariant(t *testing.T) {
	getPostgresDB(t)
	verifyVoidRefund(t, true)
}
func verifyVoidRefund(t *testing.T, pg bool) {
	var u models.User
	if pg {
		u = pgTestUser(t, "pg_void_refund", 100)
	} else {
		u = testUser(t, "void_refund", 100)
	}
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.EntryCoins = 10
	require.NoError(t, config.DB.Create(&m).Error)
	require.Equal(t, 200, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, m.ID), nil).Code)
	_, err := services.VoidMarket(m.ID, "Source withdrew the event; cancellation is required", u.ID, "127.0.0.1")
	require.NoError(t, err)
	if pg {
		verifyPostgresWalletInvariant(t, u.ID)
	} else {
		invariant(t, u.ID, 100)
	}
	require.NoError(t, config.DB.First(&u, u.ID).Error)
	require.Equal(t, 100, u.Points)
	_, err = services.VoidMarket(m.ID, "Repeated cancellation must not refund again", u.ID, "")
	require.Error(t, err)
	var refunds int64
	require.NoError(t, config.DB.Model(&models.WalletLedger{}).Where("user_id = ? AND type = ?", u.ID, models.TxTypeRefund).Count(&refunds).Error)
	require.EqualValues(t, 1, refunds)
	now := time.Now().UTC()
	_, err = services.SettleMarket(m.ID, services.ResolutionInput{Winner: "Yes", EvidenceURL: m.ResolutionSource, ObservedAt: &now, AdminID: u.ID})
	require.Error(t, err)
	w := request(t, controllers.GetMyStats, u.ID, "", nil)
	require.Contains(t, w.Body.String(), `"pending_potential":0`)
	next := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	next.Title = "Independent replacement observation"
	next.EntryCoins = 10
	require.NoError(t, config.DB.Create(&next).Error)
	require.Equal(t, 200, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, next.ID), nil).Code)
}

func TestPostgresConcurrentCancellationCreditsOnlyOnce(t *testing.T) {
	getPostgresDB(t)
	u := pgTestUser(t, "concurrent_void", 100)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.EntryCoins = 10
	require.NoError(t, config.DB.Create(&m).Error)
	require.Equal(t, 200, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, m.ID), nil).Code)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := services.VoidMarket(m.ID, "Official source cancelled this event", u.ID, "")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	require.Equal(t, 1, success)
	verifyPostgresWalletInvariant(t, u.ID)
	require.NoError(t, config.DB.First(&u, u.ID).Error)
	require.Equal(t, 100, u.Points)
}

func TestDraftEditSavesNewSourceAndPausedMarketRejectsEntries(t *testing.T) {
	setupTestDB()
	u := testUser(t, "draft_source_editor", 100)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.ResolutionStatus = "Draft"
	m.IsCurated = true
	m.SourceKind = "official_event"
	m.NewsURL = "https://mausam.imd.gov.in/"
	m.NewsSourceName = "IMD"
	require.NoError(t, config.DB.Create(&m).Error)
	input := m
	input.NewsURL = "https://mausam.imd.gov.in/chennai/"
	input.NewsSourceName = "IMD Chennai"
	body, _ := json.Marshal(input)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(m.ID)}}
	require.Equal(t, 200, request(t, controllers.UpdateMarketRules, u.ID, string(body), params).Code)
	require.NoError(t, config.DB.First(&m, m.ID).Error)
	require.Equal(t, input.NewsURL, m.NewsURL)
	require.Equal(t, input.NewsSourceName, m.NewsSourceName)
	require.NoError(t, config.DB.Model(&m).Update("resolution_status", "Paused").Error)
	require.Equal(t, 400, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":0}`, m.ID), nil).Code)
	cutoff := time.Now().UTC().Add(-time.Minute)
	require.NoError(t, config.DB.Model(&m).Update("lock_time", cutoff).Error)
	require.Equal(t, 400, request(t, controllers.TransitionMarketState, u.ID, `{"status":"Live"}`, params).Code)
}
func TestGenericFixtureFailsQualityGate(t *testing.T) {
	m := compliantTestMarket("Sports", "Easy", time.Now().UTC().Add(time.Hour))
	result := time.Now().UTC().Add(2 * time.Hour)
	m.ResolutionTime = &result
	m.Title = "Team A at Team B: who wins on 2026-10-06?"
	m.Description = "Curated from the linked live official source. No result has been declared."
	r := services.EditorialReview{Current: true, FutureOutcome: true, Objective: true, TrustedSource: true, SensibleCutoff: true, Interesting: true, Rationale: "This has been reviewed but remains a generic automatically generated fixture."}
	require.Error(t, services.ValidateEditorialReview(m, r, time.Now().UTC()))
}
