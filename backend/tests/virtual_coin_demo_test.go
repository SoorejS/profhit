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
	"testing"
	"time"
)

func TestCuratedFutureOpportunityExcludesDemoAndExpiredMarkets(t *testing.T) {
	setupTestDB()
	now := time.Now().UTC()
	m := compliantTestMarket("Weather", "Easy", now.Add(time.Hour))
	m.IsCurated = true
	m.SourceKind = "official_event"
	m.NewsURL = "https://api.weather.gov/gridpoints/LWX/96,72/forecast"
	m.NewsSourceName = "National Weather Service"
	m.NewsDiscoveredAt = &now
	require.NoError(t, config.DB.Create(&m).Error)
	fixture := m
	fixture.ID = 0
	fixture.IsDemo = true
	require.NoError(t, config.DB.Create(&fixture).Error)
	closed := m
	closed.ID = 0
	past := now.Add(-time.Minute)
	closed.LockTime = &past
	closed.EndDate = past
	require.NoError(t, config.DB.Create(&closed).Error)
	stale := m
	stale.ID = 0
	stale.SourceKind = "article"
	old := now.Add(-25 * time.Hour)
	stale.NewsPublishedAt = &old
	require.NoError(t, config.DB.Create(&stale).Error)
	w := request(t, controllers.LiveNewsFeed, 0, "", nil)
	var feed struct {
		Count int             `json:"playable_count"`
		Items []models.Market `json:"items"`
	}
	require.Equal(t, 200, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &feed))
	require.Equal(t, 1, feed.Count)
	require.Len(t, feed.Items, 1)
	require.Nil(t, feed.Items[0].NewsPublishedAt)
}

func TestVirtualCoinEntryAtomicDebitAndSettlement(t *testing.T) {
	setupTestDB()
	u := testUser(t, "virtual_coin_player", 100)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.EntryCoins = 10
	require.NoError(t, config.DB.Create(&m).Error)
	body := fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, m.ID)
	require.Equal(t, 400, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":0}`, m.ID), nil).Code)
	invariant(t, u.ID, 100)
	require.Equal(t, 200, request(t, controllers.SubmitPrediction, u.ID, body, nil).Code)
	invariant(t, u.ID, 90)
	require.NotEqual(t, 200, request(t, controllers.SubmitPrediction, u.ID, body, nil).Code)
	invariant(t, u.ID, 90)
	now := time.Now().UTC()
	cutoff := now.Add(-time.Minute)
	require.NoError(t, config.DB.Model(&m).Updates(map[string]interface{}{"resolution_status": "Locked", "lock_time": cutoff, "end_date": cutoff}).Error)
	_, err := services.SettleMarket(fmt.Sprint(m.ID), services.ResolutionInput{Winner: "Yes", EvidenceURL: m.ResolutionSource, ObservedAt: &now, AdminID: u.ID})
	require.NoError(t, err)
	invariant(t, u.ID, 110)
	_, err = services.SettleMarket(fmt.Sprint(m.ID), services.ResolutionInput{Winner: "Yes", EvidenceURL: m.ResolutionSource, ObservedAt: &now, AdminID: u.ID})
	require.Error(t, err)
	invariant(t, u.ID, 110)
}

func TestInsufficientVirtualCoinsRollsBackSubmission(t *testing.T) {
	setupTestDB()
	u := testUser(t, "empty_virtual_wallet", 5)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.EntryCoins = 10
	require.NoError(t, config.DB.Create(&m).Error)
	require.Equal(t, 400, request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, m.ID), nil).Code)
	invariant(t, u.ID, 5)
	var count int64
	require.NoError(t, config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ?", u.ID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, config.DB.First(&m, m.ID).Error)
	require.Zero(t, m.Volume)
}

func TestPostgresVirtualCoinEntryAndSettlement(t *testing.T) {
	getPostgresDB(t)
	u := pgTestUser(t, "pg_virtual_entry", 100)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.EntryCoins = 10
	require.NoError(t, config.DB.Create(&m).Error)
	body := fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, m.ID)
	require.Equal(t, 200, request(t, controllers.SubmitPrediction, u.ID, body, nil).Code)
	verifyPostgresWalletInvariant(t, u.ID)
	require.NoError(t, config.DB.First(&u, u.ID).Error)
	require.Equal(t, 90, u.Points)
	require.NotEqual(t, 200, request(t, controllers.SubmitPrediction, u.ID, body, nil).Code)
	cutoff := time.Now().UTC().Add(-time.Minute)
	require.NoError(t, config.DB.Model(&m).Updates(map[string]interface{}{"resolution_status": "Locked", "lock_time": cutoff, "end_date": cutoff}).Error)
	now := time.Now().UTC()
	_, err := services.SettleMarket(m.ID, services.ResolutionInput{Winner: "Yes", EvidenceURL: m.ResolutionSource, ObservedAt: &now, AdminID: u.ID})
	require.NoError(t, err)
	verifyPostgresWalletInvariant(t, u.ID)
	require.NoError(t, config.DB.First(&u, u.ID).Error)
	require.Equal(t, 110, u.Points)
}

func TestStaleCuratedArticleCannotBePublished(t *testing.T) {
	setupTestDB()
	u := testUser(t, "curated_editor", 0)
	m := compliantTestMarket("Weather", "Easy", time.Now().UTC().Add(time.Hour))
	m.ResolutionStatus = "Draft"
	m.IsCurated = true
	m.SourceKind = "article"
	old := time.Now().UTC().Add(-25 * time.Hour)
	m.NewsPublishedAt = &old
	require.NoError(t, config.DB.Create(&m).Error)
	w := request(t, controllers.ApproveMarket, u.ID, "", gin.Params{{Key: "id", Value: fmt.Sprint(m.ID)}})
	require.Equal(t, 400, w.Code)
	require.Contains(t, w.Body.String(), "stale")
	require.NoError(t, config.DB.First(&m, m.ID).Error)
	require.Equal(t, "Draft", m.ResolutionStatus)
}
