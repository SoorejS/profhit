package services

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"strings"
	"testing"
	"time"
)

func TestProviderWeatherEvidenceUsesSharedSettlementExactlyOnce(t *testing.T) {
	newsTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	cutoff := now.Add(-20 * time.Minute)
	spec := WeatherResultSpec{Provider: "openweather", Latitude: 13.08, Longitude: 80.27, Metric: "temperature_c", Threshold: 30, ObservationFrom: now.Add(-5 * time.Minute), ObservationUntil: now.Add(10 * time.Minute)}
	admin := models.User{Username: "fixture_operator", Email: "operator@example.test", Role: models.RoleAdmin, ReferralCode: "TEST_OPERATOR"}
	user := models.User{Username: "fixture_player", Email: "player@example.test", ReferralCode: "TEST_PLAYER"}
	require.NoError(t, config.DB.Create(&admin).Error)
	require.NoError(t, config.DB.Create(&user).Error)
	raw, _ := json.Marshal(spec)
	market := models.Market{Title: "Fixture only: Chennai temperature observation?", Category: "Weather", Difficulty: "Easy", Payout: 20, Options: `["Yes","No"]`, PredictionType: "binary", ResolutionStatus: "Locked", Visibility: "Public", LockTime: &cutoff, EndDate: cutoff, ResolutionTime: &spec.ObservationFrom, ResolutionRule: spec.Rule(), ResolutionSource: spec.EvidenceURL(), ResultSpec: string(raw), ResultApprovedBy: admin.ID}
	require.NoError(t, ConfigurePrediction(&market))
	require.NoError(t, config.DB.Create(&market).Error)
	require.NoError(t, config.DB.Create(&models.PredictionSubmission{UserID: user.ID, MarketID: market.ID, Choice: "Yes", Potential: 20}).Error)
	client := &http.Client{Transport: newsRoundTrip(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "test-only-weather-key", req.URL.Query().Get("appid"))
		body, _ := json.Marshal(map[string]interface{}{"dt": now.Unix(), "coord": map[string]float64{"lat": 13.08, "lon": 80.27}, "main": map[string]float64{"temp": 35}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}
	outcome, err := FetchWeatherResult(context.Background(), client, spec, "test-only-weather-key", now)
	require.NoError(t, err)
	require.Equal(t, "Yes", outcome.Outcome)
	require.NotContains(t, outcome.EvidenceURL, "key")
	input := ResolutionInput{Outcome: outcome.Outcome, EvidenceURL: outcome.EvidenceURL, ObservedAt: &outcome.ObservedAt, AdminID: admin.ID, ProviderEvidence: outcome.Evidence, ExpectedResultSpec: market.ResultSpec}
	_, err = SettleMarket(market.ID, input)
	require.NoError(t, err)
	_, err = SettleMarket(market.ID, input)
	require.Error(t, err)
	var ledger int64
	config.DB.Model(&models.WalletLedger{}).Where("user_id = ? AND credit = ?", user.ID, 20).Count(&ledger)
	require.EqualValues(t, 1, ledger)
	require.NoError(t, config.DB.First(&market, market.ID).Error)
	require.NotEmpty(t, market.ResultEvidence)
	require.Equal(t, "Resolved", market.ResolutionStatus)
	spec.ObservationFrom = now.Add(time.Hour)
	spec.ObservationUntil = now.Add(75 * time.Minute)
	_, err = FetchWeatherResult(context.Background(), client, spec, "test-only-weather-key", now)
	require.Error(t, err)
	_, err = FetchWeatherResult(context.Background(), client, spec, "", now)
	require.Error(t, err)
}
