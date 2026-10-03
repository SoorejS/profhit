package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/routes"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type authResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Points   int    `json:"points"`
	} `json:"user"`
}

func TestPlanBEmailPasswordRegistrationAndLogin(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", strings.Repeat("plan-b-auth-secret-", 3))
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("GOOGLE_CLIENT_ID", "")

	registration := request(t, controllers.RegisterUser, 0, `{
		"username":"  PlanB_User  ",
		"email":"  PlanB.User@Example.Test  ",
		"password":"correct-horse-battery",
		"role":"super_admin",
		"points":999999,
		"kyc_status":true
	}`, nil)
	require.Equal(t, http.StatusCreated, registration.Code, registration.Body.String())

	var registered authResponse
	require.NoError(t, json.Unmarshal(registration.Body.Bytes(), &registered))
	require.NotEmpty(t, registered.Token)
	require.Equal(t, "planb_user", registered.User.Username)
	require.Equal(t, "planb.user@example.test", registered.User.Email)
	require.Equal(t, models.RoleUser, registered.User.Role)
	require.Equal(t, 100, registered.User.Points)

	var stored models.User
	require.NoError(t, config.DB.First(&stored, registered.User.ID).Error)
	require.Equal(t, models.RoleUser, stored.Role)
	require.False(t, stored.KycStatus)
	require.NotEqual(t, "correct-horse-battery", stored.Password)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("correct-horse-battery")))

	claims, tokenUser, err := middleware.ValidateToken(registered.Token)
	require.NoError(t, err)
	require.Equal(t, stored.ID, claims.UserID)
	require.Equal(t, models.RoleUser, claims.Role)
	require.Equal(t, stored.ID, tokenUser.ID)

	tests := []struct {
		name string
		body string
		code int
	}{
		{"duplicate email is case insensitive", `{"username":"another_user","email":"PLANB.USER@EXAMPLE.TEST","password":"valid-password"}`, http.StatusConflict},
		{"duplicate username is case insensitive", `{"username":"PLANB_USER","email":"another@example.test","password":"valid-password"}`, http.StatusConflict},
		{"invalid email", `{"username":"valid_user","email":"not-an-email","password":"valid-password"}`, http.StatusBadRequest},
		{"short password", `{"username":"valid_user","email":"valid@example.test","password":"short"}`, http.StatusBadRequest},
		{"blank password", `{"username":"valid_user","email":"valid@example.test","password":"        "}`, http.StatusBadRequest},
		{"invalid username", `{"username":"<script>","email":"valid@example.test","password":"valid-password"}`, http.StatusBadRequest},
		{"missing fields", `{}`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response := request(t, controllers.RegisterUser, 0, tc.body, nil)
			require.Equal(t, tc.code, response.Code, response.Body.String())
		})
	}

	wrongPassword := request(t, controllers.LoginUser, 0, `{"email":"planb.user@example.test","password":"wrong-password"}`, nil)
	require.Equal(t, http.StatusUnauthorized, wrongPassword.Code)
	require.NotContains(t, wrongPassword.Body.String(), `"token"`)
	unknownEmail := request(t, controllers.LoginUser, 0, `{"email":"unknown@example.test","password":"correct-horse-battery"}`, nil)
	require.Equal(t, http.StatusUnauthorized, unknownEmail.Code)
	require.Equal(t, wrongPassword.Body.String(), unknownEmail.Body.String())
	require.Equal(t, http.StatusBadRequest,
		request(t, controllers.LoginUser, 0, `{"email":"malformed","password":"correct-horse-battery"}`, nil).Code)
	require.Equal(t, http.StatusBadRequest, request(t, controllers.LoginUser, 0, `{}`, nil).Code)

	login := request(t, controllers.LoginUser, 0, `{"email":" PLANB.USER@EXAMPLE.TEST ","password":"correct-horse-battery"}`, nil)
	require.Equal(t, http.StatusOK, login.Code, login.Body.String())
	var loggedIn authResponse
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &loggedIn))
	require.NotEmpty(t, loggedIn.Token)
	require.Equal(t, stored.ID, loggedIn.User.ID)
}

func TestPlanBPasswordResetUnavailableWithoutSMTP(t *testing.T) {
	setupTestDB()
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	response := request(t, controllers.ForgotPassword, 0, `{"email":" user@example.test "}`, nil)
	require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "temporarily unavailable")
	require.Equal(t, http.StatusBadRequest,
		request(t, controllers.ForgotPassword, 0, `{"email":"invalid"}`, nil).Code)
}

func TestPlanBProtectedRouteLogoutAndAccountStatus(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", strings.Repeat("plan-b-session-secret-", 3))
	user := testUser(t, "plan_b_session", 0)
	hash, err := bcrypt.GenerateFromPassword([]byte("session-password"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, config.DB.Model(&user).Update("password", string(hash)).Error)
	require.NoError(t, config.DB.First(&user, user.ID).Error)
	token, err := controllers.GenerateToken(user)
	require.NoError(t, err)

	router := routes.SetupRouter()
	me := performJSONRequest(router, http.MethodGet, "/api/me", "", token, "198.51.100.10:12000")
	require.Equal(t, http.StatusOK, me.Code, me.Body.String())
	require.Contains(t, me.Body.String(), `"id":`)

	logout := performJSONRequest(router, http.MethodPost, "/api/auth/logout", `{}`, token, "198.51.100.10:12001")
	require.Equal(t, http.StatusOK, logout.Code, logout.Body.String())
	require.Equal(t, http.StatusUnauthorized,
		performJSONRequest(router, http.MethodGet, "/api/me", "", token, "198.51.100.10:12002").Code)

	activeToken, err := controllers.GenerateToken(user)
	require.NoError(t, err)
	require.NoError(t, config.DB.Model(&user).Update("is_active", false).Error)
	require.Equal(t, http.StatusUnauthorized,
		performJSONRequest(router, http.MethodGet, "/api/me", "", activeToken, "198.51.100.10:12003").Code)
}

func TestPlanBLoginRateLimit(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", strings.Repeat("plan-b-limit-secret-", 3))
	router := routes.SetupRouter()
	for attempt := 1; attempt <= 6; attempt++ {
		response := performJSONRequest(router, http.MethodPost, "/api/auth/login",
			`{"email":"nobody@example.test","password":"wrong-password"}`, "", "203.0.113.20:30000")
		if attempt <= 5 {
			require.Equal(t, http.StatusUnauthorized, response.Code, "attempt %d: %s", attempt, response.Body.String())
		} else {
			require.Equal(t, http.StatusTooManyRequests, response.Code, response.Body.String())
			require.Contains(t, response.Body.String(), "Too many requests")
		}
	}
}

func performJSONRequest(handler http.Handler, method, path, body, token, remoteAddr string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.RemoteAddr = remoteAddr
	handler.ServeHTTP(recorder, req)
	return recorder
}
