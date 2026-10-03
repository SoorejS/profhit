package controllers

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"net/http"
	"regexp"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm/clause"
)

var sixDigitCode = regexp.MustCompile(`^[0-9]{6}$`)

// GetTwoFactorStatus returns enrollment state without exposing the TOTP secret.
func GetTwoFactorStatus(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"enabled":                user.TwoFactorEnabled,
		"setup_pending":          !user.TwoFactorEnabled && user.TwoFactorSecret != "",
		"recovery_codes_enabled": false,
	})
}

// SetupTwoFactor creates a pending authenticator enrollment. It does not enable
// 2FA until a valid code from the new secret is verified.
func SetupTwoFactor(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var input struct {
		CurrentPassword string `json:"current_password" binding:"required,max=72"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is required"})
		return
	}

	tx := config.DB.Begin()
	defer tx.Rollback()
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.TwoFactorEnabled {
		c.JSON(http.StatusConflict, gin.H{"error": "Two-factor authentication is already enabled"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.CurrentPassword)) != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Current password is incorrect"})
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "PROPHIT",
		AccountName: user.Email,
		Period:      30,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create authenticator setup"})
		return
	}
	image, err := key.Image(256, 256)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not render authenticator QR code"})
		return
	}
	var qr bytes.Buffer
	if err := png.Encode(&qr, image); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not render authenticator QR code"})
		return
	}
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"two_factor_secret":  key.Secret(),
		"two_factor_enabled": false,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save authenticator setup"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save authenticator setup"})
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"secret":              key.Secret(),
		"provisioning_uri":    key.URL(),
		"qr_code_data_url":    "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr.Bytes()),
		"recovery_codes":      nil,
		"recovery_codes_note": "Recovery codes are not implemented.",
	})
}

// EnableTwoFactor verifies the pending secret and rotates existing sessions.
func EnableTwoFactor(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	code, ok := bindTwoFactorCode(c)
	if !ok {
		return
	}

	tx := config.DB.Begin()
	defer tx.Rollback()
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.TwoFactorEnabled {
		c.JSON(http.StatusConflict, gin.H{"error": "Two-factor authentication is already enabled"})
		return
	}
	if user.TwoFactorSecret == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Start authenticator setup before enabling 2FA"})
		return
	}
	if !totp.Validate(code, user.TwoFactorSecret) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid authenticator code"})
		return
	}

	user.TwoFactorEnabled = true
	user.TokenVersion++
	newToken, err := GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not rotate authenticated session"})
		return
	}
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"two_factor_enabled": true,
		"token_version":      user.TokenVersion,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not enable two-factor authentication"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not enable two-factor authentication"})
		return
	}
	services.CheckProfileCompletion(user.ID)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"message": "Two-factor authentication enabled", "token": newToken})
}

// DisableTwoFactor requires a current authenticator code and rotates all sessions.
func DisableTwoFactor(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	code, ok := bindTwoFactorCode(c)
	if !ok {
		return
	}

	tx := config.DB.Begin()
	defer tx.Rollback()
	var user models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if !user.TwoFactorEnabled || user.TwoFactorSecret == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Two-factor authentication is not enabled"})
		return
	}
	if !totp.Validate(code, user.TwoFactorSecret) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid authenticator code"})
		return
	}

	user.TokenVersion++
	newToken, err := GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not rotate authenticated session"})
		return
	}
	if err := tx.Model(&user).Updates(map[string]interface{}{
		"two_factor_enabled": false,
		"two_factor_secret":  "",
		"token_version":      user.TokenVersion,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not disable two-factor authentication"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not disable two-factor authentication"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"message": "Two-factor authentication disabled", "token": newToken})
}

func bindTwoFactorCode(c *gin.Context) (string, bool) {
	var input struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || !sixDigitCode.MatchString(input.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A 6-digit authenticator code is required"})
		return "", false
	}
	return input.Code, true
}
