package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strings"

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
		Credential string `json:"credential" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing Google credential"})
		return
	}

	// Verify token with Google's tokeninfo endpoint
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", input.Credential))
	if err != nil || resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired Google token"})
		return
	}
	defer resp.Body.Close()

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
	expectedClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if expectedClientID != "" && gInfo.Aud != expectedClientID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Google client ID mismatch"})
		return
	}

	// Find or create user by Email
	var user models.User
	err = config.DB.Where("email = ?", gInfo.Email).First(&user).Error

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

		dummyPwd, _ := bcrypt.GenerateFromPassword([]byte(gInfo.Sub+"_google_oauth"), 12)
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

		tx.Commit()
		config.DB.First(&user, user.ID)
	}

	if !user.IsActive {
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
