package tests

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/routes"
	"profhit-backend/services"
)

func TestProductionMigrationAndFinancialConstraints(t *testing.T) {
	setupTestDB()
	require.True(t, config.DB.Migrator().HasTable(&models.PaymentTransaction{}))
	require.NoError(t, config.Migrate(config.DB)) // Restart/repeated migration.
	u := testUser(t, "constraints", 50)
	require.Error(t, config.DB.Create(&models.CoinBatch{UserID: 99999, Amount: 1, Balance: 1}).Error)
	require.Error(t, config.DB.Create(&models.CoinBatch{UserID: u.ID, Amount: 1, Balance: 2}).Error)
	require.Error(t, config.DB.Create(&models.WalletLedger{UserID: u.ID, Credit: 10, BalanceBefore: 50, BalanceAfter: 99}).Error)
	require.Error(t, config.DB.Model(&models.User{}).Where("id = ?", u.ID).Update("points", -1).Error)
	invariant(t, u.ID, 50)
}

func TestWinRateWorksOnSQLiteAndExcludesUnresolved(t *testing.T) {
	setupTestDB()
	u := testUser(t, "winrate", 0)
	for i := 0; i < 12; i++ {
		m := models.Market{Title: fmt.Sprint("Market ", i), Category: "Sports", Options: `["Yes","No"]`}
		require.NoError(t, config.DB.Create(&m).Error)
		var correct *bool
		if i < 10 {
			v := i < 8
			correct = &v
		}
		require.NoError(t, config.DB.Create(&models.PredictionSubmission{UserID: u.ID, MarketID: m.ID, Choice: "Yes", Amount: 10, Potential: 20, IsCorrect: correct}).Error)
	}
	w := request(t, controllers.GetTopWinRate, 0, "", nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	var rows []struct {
		WinRate float64 `json:"win_rate"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	require.Equal(t, float64(80), rows[0].WinRate)
}

func TestSuspendedLoginDoesNotIssueSession(t *testing.T) {
	setupTestDB()
	u := testUser(t, "suspended", 0)
	password, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, config.DB.Model(&u).Updates(map[string]interface{}{"password": string(password), "suspended_until": time.Now().Add(time.Hour)}).Error)
	w := request(t, controllers.LoginUser, 0, `{"email":"suspended@test.invalid","password":"test-password"}`, nil)
	require.Equal(t, 403, w.Code)
	require.NotContains(t, w.Body.String(), `"token"`)
}

func TestRedemptionRejectRefundsAndRestocksExactlyOnce(t *testing.T) {
	setupTestDB()
	u := testUser(t, "redeemer", 100)
	require.NoError(t, config.DB.Model(&u).Update("kyc_status", true).Error)
	item := models.RewardItem{Name: "Voucher", Cost: 50, Inventory: 1, IsActive: true}
	require.NoError(t, config.DB.Create(&item).Error)
	body := fmt.Sprintf(`{"reward_item_id":%d}`, item.ID)
	require.Equal(t, 201, request(t, controllers.SubmitRedemption, u.ID, body, nil).Code)
	require.Equal(t, 400, request(t, controllers.SubmitRedemption, u.ID, body, nil).Code)
	invariant(t, u.ID, 50)
	var redemption models.Redemption
	require.NoError(t, config.DB.First(&redemption).Error)
	params := gin.Params{{Key: "id", Value: fmt.Sprint(redemption.ID)}}
	require.Equal(t, 200, request(t, controllers.AdminProcessRedemption, u.ID, `{"status":"Rejected"}`, params).Code)
	require.Equal(t, 400, request(t, controllers.AdminProcessRedemption, u.ID, `{"status":"Rejected"}`, params).Code)
	invariant(t, u.ID, 100)
	require.NoError(t, config.DB.First(&item, item.ID).Error)
	require.Equal(t, 1, item.Inventory)
}

func TestEveryProtectedRouteRejectsAnonymousRequests(t *testing.T) {
	setupTestDB()
	t.Setenv("GIN_MODE", "release")
	t.Setenv("JWT_SECRET", strings.Repeat("test", 12))
	router := routes.SetupRouter()
	public := map[string]bool{
		"POST /api/auth/register": true, "POST /api/auth/login": true, "POST /api/auth/google": true,
		"POST /api/auth/forgot-password": true, "POST /api/auth/reset-password": true, "GET /api/auth/config": true,
		"GET /api/health": true, "POST /api/webhooks/hyperverge": true, "POST /api/webhooks/razorpay": true,
		"GET /api/news": true, "GET /api/markets": true, "GET /api/markets/:id": true, "GET /api/markets/:id/comments": true,
		"GET /api/leaderboard": true, "GET /api/leaderboard/legacy": true, "GET /api/leaderboard/streak": true,
		"GET /api/leaderboard/winrate": true, "GET /api/activity": true, "GET /api/users/:id": true,
	}
	checked := 0
	for _, route := range router.Routes() {
		if public[route.Method+" "+route.Path] {
			continue
		}
		path := strings.ReplaceAll(route.Path, ":id", "1")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(route.Method, path, nil))
		require.Equal(t, 401, w.Code, route.Method+" "+path+" "+w.Body.String())
		checked++
	}
	require.Greater(t, checked, 35)
	// The production simulator must not be registered.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/api/simulator/sign-webhook", nil))
	require.Equal(t, 404, w.Code)
}

func TestUnenabledTwoFactorDoesNotAwardProfileBonus(t *testing.T) {
	setupTestDB()
	u := testUser(t, "unfinished2fa", 0)
	require.NoError(t, config.DB.Model(&u).Updates(map[string]interface{}{"kyc_status": true, "two_factor_secret": "provisioned-not-enabled"}).Error)
	services.CheckProfileCompletion(u.ID)
	invariant(t, u.ID, 0)
}

func TestWebSocketPrivateIsolationAndRevocation(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", strings.Repeat("socket", 8))
	alice := testUser(t, "socketalice", 0)
	bob := testUser(t, "socketbob", 0)
	aToken, err := controllers.GenerateToken(alice)
	require.NoError(t, err)
	bToken, err := controllers.GenerateToken(bob)
	require.NoError(t, err)
	services.StartWebSocketHub()
	router := gin.New()
	router.GET("/ws", controllers.WsHandler)
	server := httptest.NewServer(router)
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token="
	a, _, err := websocket.DefaultDialer.Dial(url+aToken, nil)
	require.NoError(t, err)
	defer a.Close()
	b, _, err := websocket.DefaultDialer.Dial(url+bToken, nil)
	require.NoError(t, err)
	defer b.Close()
	// Server acknowledgement confirms registration before events are queued.
	for _, conn := range []*websocket.Conn{a, b} {
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		var ready services.WSMessage
		require.NoError(t, conn.ReadJSON(&ready))
		require.Equal(t, "connected", ready.Event)
	}
	services.BroadcastToUser(alice.ID, "private", "only alice")
	services.BroadcastToAll("public", "everyone")
	var msg services.WSMessage
	a.SetReadDeadline(time.Now().Add(3 * time.Second))
	require.NoError(t, a.ReadJSON(&msg))
	require.Equal(t, "private", msg.Event)
	require.NoError(t, a.ReadJSON(&msg))
	require.Equal(t, "public", msg.Event)
	b.SetReadDeadline(time.Now().Add(3 * time.Second))
	require.NoError(t, b.ReadJSON(&msg))
	require.Equal(t, "public", msg.Event)
	require.NoError(t, middleware.InvalidateToken(bToken))
	services.BroadcastToUser(bob.ID, "private", "must not arrive")
	require.Error(t, b.ReadJSON(&msg))
}

func TestPasswordResetUnavailableIsExplicit(t *testing.T) {
	setupTestDB()
	t.Setenv("SMTP_HOST", "")
	w := request(t, controllers.ForgotPassword, 0, `{"email":"any@example.invalid"}`, nil)
	require.Equal(t, 503, w.Code)
}

func TestNewsUnavailableWithoutProviderConfiguration(t *testing.T) {
	t.Setenv("NEWS_API_KEY", "")
	w := request(t, controllers.GetTrendingNews, 0, "", nil)
	require.Equal(t, 503, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"News is not configured"}`, w.Body.String())
}

func TestLeaderboardRankUsesSameTiesAndEligibility(t *testing.T) {
	setupTestDB()
	first := testUser(t, "first_rank", 100)
	second := testUser(t, "second_rank", 100)
	banned := testUser(t, "banned_rank", 200)
	require.NoError(t, config.DB.Model(&banned).Update("is_active", false).Error)
	w := request(t, controllers.GetUnifiedLeaderboard, second.ID, "", nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	var response struct {
		Data []struct {
			ID   uint
			Rank int
		}
		CurrentUser struct {
			Rank     int
			Username string
		} `json:"current_user"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data, 2)
	require.Equal(t, first.ID, response.Data[0].ID)
	require.Equal(t, 2, response.CurrentUser.Rank)
	require.Equal(t, second.Username, response.CurrentUser.Username)
}

func TestDraftCommentsAreNotPublic(t *testing.T) {
	setupTestDB()
	u := testUser(t, "draft_comment", 0)
	m := models.Market{Title: "Draft", Category: "Sports", ResolutionStatus: "Draft", Options: `["Yes","No"]`}
	require.NoError(t, config.DB.Create(&m).Error)
	require.NoError(t, config.DB.Create(&models.Comment{UserID: u.ID, MarketID: m.ID, Username: u.Username, Content: "Internal draft"}).Error)
	w := request(t, controllers.GetComments, 0, "", gin.Params{{Key: "id", Value: fmt.Sprint(m.ID)}})
	require.Equal(t, 404, w.Code)
	require.NotContains(t, w.Body.String(), "Internal draft")
}

func TestPredictionCategoryDayBoundaryUTC(t *testing.T) {
	for _, previousDay := range []bool{true, false} {
		t.Run(fmt.Sprint(previousDay), func(t *testing.T) {
			setupTestDB()
			u := testUser(t, "midnight", 100)
			now := time.Now().UTC()
			start := now.Truncate(24 * time.Hour)
			prior := start
			if previousDay {
				prior = start.Add(-time.Nanosecond)
			}
			first := models.Market{Title: "Prior", Category: "Sports", Options: `["Yes","No"]`, ResolutionStatus: "Open", Payout: 20, EndDate: now.Add(time.Hour)}
			next := first
			next.Title = "Next"
			require.NoError(t, config.DB.Create(&first).Error)
			require.NoError(t, config.DB.Create(&next).Error)
			require.NoError(t, config.DB.Create(&models.PredictionSubmission{UserID: u.ID, MarketID: first.ID, Choice: "Yes", Amount: 10, Potential: 20, CreatedAt: prior}).Error)
			w := request(t, controllers.SubmitPrediction, u.ID, fmt.Sprintf(`{"market_id":%d,"choice":"Yes","amount":10}`, next.ID), nil)
			if previousDay {
				require.Equal(t, 200, w.Code, w.Body.String())
			} else {
				require.Equal(t, 429, w.Code, w.Body.String())
			}
		})
	}
}
