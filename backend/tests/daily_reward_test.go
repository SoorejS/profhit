package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/models"
	"profhit-backend/services"
)

func TestDailyRewardEligibilityCreditAndDuplicate(t *testing.T) {
	setupTestDB()
	user := testUser(t, "daily_reward_status", 90)
	readStatus := func() controllers.DailyLoginResponse {
		response := request(t, controllers.GetDailyLoginInfo, user.ID, "", nil)
		require.Equal(t, 200, response.Code, response.Body.String())
		var status controllers.DailyLoginResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &status))
		return status
	}
	before := readStatus()
	require.False(t, before.AlreadyCheckedIn)
	require.Equal(t, 90, before.NewBalance)
	var count int64
	require.NoError(t, config.DB.Model(&models.UserStreak{}).Where("user_id = ?", user.ID).Count(&count).Error)
	require.Zero(t, count, "reading eligibility must not claim a reward")
	for i := 0; i < 2; i++ {
		response := request(t, controllers.DailyLogin, user.ID, "", nil)
		require.Equal(t, 200, response.Code, response.Body.String())
		var reward controllers.DailyLoginResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &reward))
		require.Equal(t, 100, reward.NewBalance)
		require.Equal(t, i == 1, reward.AlreadyCheckedIn)
		if i == 0 {
			require.Equal(t, 10, reward.CoinsEarned)
		} else {
			require.Zero(t, reward.CoinsEarned)
		}
	}
	after := readStatus()
	require.True(t, after.AlreadyCheckedIn)
	require.Equal(t, 100, after.NewBalance)
	require.Equal(t, time.Now().UTC().Truncate(24*time.Hour).Add(24*time.Hour), after.NextClaimAt)
	invariant(t, user.ID, 100)
	require.NoError(t, config.DB.Model(&models.WalletLedger{}).Where("user_id = ? AND type = ?", user.ID, models.TxTypeDailyLogin).Count(&count).Error)
	require.EqualValues(t, 1, count)
	// An expired prior-day claim must become eligible without altering the wallet.
	require.NoError(t, config.DB.Model(&models.UserStreak{}).Where("user_id = ?", user.ID).Update("last_login_date", time.Now().UTC().Add(-24*time.Hour).Truncate(24*time.Hour)).Error)
	require.False(t, readStatus().AlreadyCheckedIn)
}

func TestPostgresDailyRewardConcurrentClaims(t *testing.T) {
	getPostgresDB(t)
	user := pgTestUser(t, "daily_reward_concurrent", 90)
	results := make(chan bool, 8)
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			claimed, err := services.ClaimDailyLogin(user.ID, time.Now().UTC())
			results <- claimed
			errors <- err
		}()
	}
	credited := 0
	for i := 0; i < 8; i++ {
		if <-results {
			credited++
		}
		require.NoError(t, <-errors)
	}
	require.Equal(t, 1, credited)
	verifyPostgresWalletInvariant(t, user.ID)
	var stored models.User
	require.NoError(t, config.DB.First(&stored, user.ID).Error)
	require.Equal(t, 100, stored.Points)
}
