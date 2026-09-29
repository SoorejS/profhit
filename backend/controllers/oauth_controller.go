package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"os"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type GoogleTokenInfo struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"` // Unique Google User ID
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
}

// GoogleLogin handles Google OAuth ID token verification and user auto-provisioning
func GoogleLogin(c *gin.Context) {
	var input struct {
		Credential    string `json:"credential" binding:"required"`
		TwoFactorCode string `json:"two_factor_code"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing Google credential"})
		return
	}

	// Verify token with Google's tokeninfo endpoint
	expectedClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if expectedClientID == "" {
		c.JSON(503, gin.H{"error": "Google Sign-In is not configured"})
		return
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(input.Credential))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired Google token"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(401, gin.H{"error": "Invalid Google token"})
		return
	}

	var gInfo GoogleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&gInfo); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to parse Google user info"})
		return
	}

	if gInfo.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google account has no email associated"})
		return
	}

	// Optional aud check if GOOGLE_CLIENT_ID is set
	if gInfo.Aud != expectedClientID || gInfo.EmailVerified != "true" || (gInfo.Iss != "accounts.google.com" && gInfo.Iss != "https://accounts.google.com") || len(gInfo.Sub) < 4 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Google client ID mismatch"})
		return
	}

	// Find or create user by Email
	var user models.User
	err = config.DB.Where("email = ?", gInfo.Email).First(&user).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(500, gin.H{"error": "Could not look up account"})
		return
	}
	if err != nil {
		// Create new user account via Google Sign-In
		username := strings.Split(gInfo.Email, "@")[0]
		// Sanitize username
		username = strings.ReplaceAll(username, ".", "_")

		// Check username collision
		var count int64
		config.DB.Model(&models.User{}).Where("username = ?", username).Count(&count)
		if count > 0 {
			username = fmt.Sprintf("%s_%s", username, gInfo.Sub[:4])
		}

		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			c.JSON(500, gin.H{"error": "Could not create account"})
			return
		}
		dummyPwd, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(randomBytes)), 12)
		if err != nil {
			c.JSON(500, gin.H{"error": "Could not create account"})
			return
		}
		newReferralCode := services.GenerateReferralCode()

		user = models.User{
			Username:     username,
			Email:        gInfo.Email,
			Password:     string(dummyPwd),
			Points:       0,
			Tier:         "Standard",
			Role:         models.RoleUser,
			IsActive:     true,
			KycStatus:    false,
			ReferralCode: newReferralCode,
		}

		tx := config.DB.Begin()
		if err := tx.Create(&user).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user account from Google"})
			return
		}

		// Welcome bonus via ledger
		if err := services.CreditWalletTx(tx, user.ID, 100, models.TxTypeAdminAdjustment, 0, "Welcome Bonus (Google Sign-In)", nil); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not issue welcome bonus"})
			return
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not create account"})
			return
		}
		config.DB.First(&user, user.ID)
	}

	if user.TwoFactorEnabled && !totp.Validate(input.TwoFactorCode, user.TwoFactorSecret) {
		c.JSON(401, gin.H{"error": "2fa_required"})
		return
	}
	if !user.IsActive || (user.SuspendedUntil != nil && user.SuspendedUntil.After(time.Now())) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been suspended."})
		return
	}

	token, err := GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Google Login successful!",
		"token":   token,
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"tier":       user.Tier,
			"role":       user.Role,
			"is_active":  user.IsActive,
			"points":     user.Points,
			"kyc_status": user.KycStatus,
		},
	})
}
