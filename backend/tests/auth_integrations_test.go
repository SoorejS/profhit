package tests

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestTwoFactorEnrollmentLoginAndDisableLifecycle(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", strings.Repeat("two-factor-test-secret-", 2))
	user := testUser(t, "two_factor_flow", 0)
	otherUser := testUser(t, "two_factor_other", 0)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, config.DB.Model(&user).Update("password", string(passwordHash)).Error)
	require.NoError(t, config.DB.First(&user, user.ID).Error)
	oldToken, err := controllers.GenerateToken(user)
	require.NoError(t, err)

	require.Equal(t, http.StatusUnprocessableEntity,
		request(t, controllers.SetupTwoFactor, user.ID, `{"current_password":"wrong-password"}`, nil).Code)

	setup := request(t, controllers.SetupTwoFactor, user.ID, `{"current_password":"correct-password"}`, nil)
	require.Equal(t, http.StatusOK, setup.Code, setup.Body.String())
	var enrollment struct {
		Secret           string `json:"secret"`
		ProvisioningURI  string `json:"provisioning_uri"`
		QRCodeDataURL    string `json:"qr_code_data_url"`
		RecoveryCodes    any    `json:"recovery_codes"`
		RecoveryCodeNote string `json:"recovery_codes_note"`
	}
	require.NoError(t, json.Unmarshal(setup.Body.Bytes(), &enrollment))
	require.NotEmpty(t, enrollment.Secret)
	require.Contains(t, enrollment.ProvisioningURI, "otpauth://totp/PROPHIT:")
	require.True(t, strings.HasPrefix(enrollment.QRCodeDataURL, "data:image/png;base64,"))
	require.Nil(t, enrollment.RecoveryCodes)
	require.Contains(t, enrollment.RecoveryCodeNote, "not implemented")

	var stored models.User
	require.NoError(t, config.DB.First(&stored, user.ID).Error)
	require.False(t, stored.TwoFactorEnabled)
	require.NotEmpty(t, stored.TwoFactorSecret)
	var untouched models.User
	require.NoError(t, config.DB.First(&untouched, otherUser.ID).Error)
	require.Empty(t, untouched.TwoFactorSecret)
	require.False(t, untouched.TwoFactorEnabled)
	require.Equal(t, http.StatusBadRequest,
		request(t, controllers.EnableTwoFactor, user.ID, `{"code":"12ab"}`, nil).Code)

	expiredCode, err := totp.GenerateCode(stored.TwoFactorSecret, time.Now().UTC().Add(-3*time.Minute))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity,
		request(t, controllers.EnableTwoFactor, user.ID, fmt.Sprintf(`{"code":%q}`, expiredCode), nil).Code)

	validCode, err := totp.GenerateCode(stored.TwoFactorSecret, time.Now().UTC())
	require.NoError(t, err)
	enabled := request(t, controllers.EnableTwoFactor, user.ID, fmt.Sprintf(`{"code":%q}`, validCode), nil)
	require.Equal(t, http.StatusOK, enabled.Code, enabled.Body.String())
	var rotated struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(enabled.Body.Bytes(), &rotated))
	require.NotEmpty(t, rotated.Token)
	_, _, err = middleware.ValidateToken(oldToken)
	require.Error(t, err)
	_, _, err = middleware.ValidateToken(rotated.Token)
	require.NoError(t, err)

	missing := request(t, controllers.LoginUser, 0, `{"email":"two_factor_flow@test.invalid","password":"correct-password"}`, nil)
	require.Equal(t, http.StatusUnauthorized, missing.Code)
	require.NotContains(t, missing.Body.String(), `"token"`)
	require.Contains(t, missing.Body.String(), "2fa_required")
	malformed := request(t, controllers.LoginUser, 0, `{"email":"two_factor_flow@test.invalid","password":"correct-password","two_factor_code":"12345x"}`, nil)
	require.Equal(t, http.StatusUnauthorized, malformed.Code)
	require.NotContains(t, malformed.Body.String(), `"token"`)
	expired := request(t, controllers.LoginUser, 0, fmt.Sprintf(`{"email":"two_factor_flow@test.invalid","password":"correct-password","two_factor_code":%q}`, expiredCode), nil)
	require.Equal(t, http.StatusUnauthorized, expired.Code)
	require.NotContains(t, expired.Body.String(), `"token"`)
	validCode, err = totp.GenerateCode(stored.TwoFactorSecret, time.Now().UTC())
	require.NoError(t, err)
	login := request(t, controllers.LoginUser, 0, fmt.Sprintf(`{"email":"two_factor_flow@test.invalid","password":"correct-password","two_factor_code":%q}`, validCode), nil)
	require.Equal(t, http.StatusOK, login.Code, login.Body.String())
	require.Contains(t, login.Body.String(), `"token"`)

	wrongCode := "000000"
	if validCode == wrongCode {
		wrongCode = "000001"
	}
	require.Equal(t, http.StatusUnprocessableEntity,
		request(t, controllers.DisableTwoFactor, user.ID, fmt.Sprintf(`{"code":%q}`, wrongCode), nil).Code)
	disabled := request(t, controllers.DisableTwoFactor, user.ID, fmt.Sprintf(`{"code":%q}`, validCode), nil)
	require.Equal(t, http.StatusOK, disabled.Code, disabled.Body.String())
	require.NoError(t, config.DB.First(&stored, user.ID).Error)
	require.False(t, stored.TwoFactorEnabled)
	require.Empty(t, stored.TwoFactorSecret)

	status := request(t, controllers.GetTwoFactorStatus, user.ID, "", nil)
	require.Equal(t, http.StatusOK, status.Code)
	require.Contains(t, status.Body.String(), `"recovery_codes_enabled":false`)
}

func TestPasswordResetTokenLifecycle(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", strings.Repeat("password-reset-test-", 2))
	user := testUser(t, "reset_lifecycle", 0)
	oldPassword, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, config.DB.Model(&user).Update("password", string(oldPassword)).Error)

	firstRaw := "first-reset-token"
	firstHash := fmt.Sprintf("%x", sha256.Sum256([]byte(firstRaw)))
	require.NoError(t, services.ReplacePasswordResetToken(user.ID, firstHash, time.Now().UTC().Add(time.Hour)))
	secondRaw := "replacement-reset-token"
	secondHash := fmt.Sprintf("%x", sha256.Sum256([]byte(secondRaw)))
	require.NoError(t, services.ReplacePasswordResetToken(user.ID, secondHash, time.Now().UTC().Add(time.Hour)))
	var count int64
	require.NoError(t, config.DB.Model(&models.PasswordResetToken{}).Where("user_id = ?", user.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.Equal(t, http.StatusBadRequest,
		request(t, controllers.ResetPassword, 0, fmt.Sprintf(`{"token":%q,"new_password":"new-password"}`, firstRaw), nil).Code)

	valid := request(t, controllers.ResetPassword, 0, fmt.Sprintf(`{"token":%q,"new_password":"new-password"}`, secondRaw), nil)
	require.Equal(t, http.StatusOK, valid.Code, valid.Body.String())
	require.Equal(t, http.StatusBadRequest,
		request(t, controllers.ResetPassword, 0, fmt.Sprintf(`{"token":%q,"new_password":"another-password"}`, secondRaw), nil).Code)
	require.Equal(t, http.StatusBadRequest,
		request(t, controllers.ResetPassword, 0, `{"token":"malformed","new_password":"another-password"}`, nil).Code)

	expiredRaw := "expired-reset-token"
	expiredHash := fmt.Sprintf("%x", sha256.Sum256([]byte(expiredRaw)))
	require.NoError(t, services.ReplacePasswordResetToken(user.ID, expiredHash, time.Now().UTC().Add(-time.Minute)))
	require.Equal(t, http.StatusBadRequest,
		request(t, controllers.ResetPassword, 0, fmt.Sprintf(`{"token":%q,"new_password":"another-password"}`, expiredRaw), nil).Code)

	oldLogin := request(t, controllers.LoginUser, 0, `{"email":"reset_lifecycle@test.invalid","password":"old-password"}`, nil)
	require.Equal(t, http.StatusUnauthorized, oldLogin.Code)
	newLogin := request(t, controllers.LoginUser, 0, `{"email":"reset_lifecycle@test.invalid","password":"new-password"}`, nil)
	require.Equal(t, http.StatusOK, newLogin.Code, newLogin.Body.String())
}

func TestGoogleTokenValidationAndAccountReuse(t *testing.T) {
	setupTestDB()
	t.Setenv("JWT_SECRET", strings.Repeat("google-auth-test-secret-", 2))
	t.Setenv("GOOGLE_CLIENT_ID", "")
	require.Equal(t, http.StatusServiceUnavailable,
		request(t, controllers.GoogleLogin, 0, `{"credential":"credential"}`, nil).Code)

	t.Setenv("GOOGLE_CLIENT_ID", "staging-client-id")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	require.Equal(t, http.StatusUnauthorized,
		request(t, controllers.GoogleLogin, 0, `{"credential":"expired-or-invalid"}`, nil).Code)

	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"iss":"https://accounts.google.com","sub":"google-subject","email":"oauth@test.invalid","email_verified":"true","name":"OAuth User","aud":"another-client"}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	require.Equal(t, http.StatusUnauthorized,
		request(t, controllers.GoogleLogin, 0, `{"credential":"wrong-audience"}`, nil).Code)

	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"iss":"https://accounts.google.com","sub":"google-subject","email":"oauth@test.invalid","email_verified":"true","name":"OAuth User","aud":"staging-client-id"}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	first := request(t, controllers.GoogleLogin, 0, `{"credential":"valid-token"}`, nil)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := request(t, controllers.GoogleLogin, 0, `{"credential":"valid-token"}`, nil)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	var count int64
	require.NoError(t, config.DB.Model(&models.User{}).Where("email = ?", "oauth@test.invalid").Count(&count).Error)
	require.Equal(t, int64(1), count)

	var googleUser models.User
	require.NoError(t, config.DB.Where("email = ?", "oauth@test.invalid").First(&googleUser).Error)
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "PROPHIT", AccountName: googleUser.Email})
	require.NoError(t, err)
	require.NoError(t, config.DB.Model(&googleUser).Updates(map[string]interface{}{
		"two_factor_secret": key.Secret(), "two_factor_enabled": true,
	}).Error)
	require.Equal(t, http.StatusUnauthorized,
		request(t, controllers.GoogleLogin, 0, `{"credential":"valid-token"}`, nil).Code)
	require.Equal(t, http.StatusUnauthorized,
		request(t, controllers.GoogleLogin, 0, `{"credential":"valid-token","two_factor_code":"abcdef"}`, nil).Code)
	code, err := totp.GenerateCode(key.Secret(), time.Now().UTC())
	require.NoError(t, err)
	withTwoFactor := request(t, controllers.GoogleLogin, 0, fmt.Sprintf(`{"credential":"valid-token","two_factor_code":%q}`, code), nil)
	require.Equal(t, http.StatusOK, withTwoFactor.Code, withTwoFactor.Body.String())
}

func TestSMTPFailureDoesNotExposePassword(t *testing.T) {
	t.Setenv("SMTP_HOST", "127.0.0.1")
	t.Setenv("SMTP_PORT", "1")
	t.Setenv("SMTP_USERNAME", "staging-user@test.invalid")
	t.Setenv("SMTP_PASSWORD", "do-not-leak-this-password")
	err := services.SendEmail("recipient@test.invalid", "Staging test", "<p>test</p>")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "do-not-leak-this-password")

	t.Setenv("SMTP_PORT", "not-a-port")
	err = services.SendEmail("recipient@test.invalid", "Staging test", "<p>test</p>")
	require.EqualError(t, err, "SMTP_PORT must be a valid TCP port")
}
