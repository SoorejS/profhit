package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"profhit-backend/config"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/services"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

func GenerateToken(user models.User) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", errors.New("JWT_SECRET not configured")
	}

	expiryMinutes := 7 * 24 * 60 // default 7 days
	if expStr := os.Getenv("JWT_EXPIRY_MINUTES"); expStr != "" {
		if parsed, err := strconv.Atoi(expStr); err == nil && parsed > 0 {
			expiryMinutes = parsed
		}
	}

	claims := middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Tier:     user.Tier,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiryMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func generateToken(user models.User) (string, error) {
	return GenerateToken(user)
}

// RegisterUser creates a new user with hashed password
func RegisterUser(c *gin.Context) {
	var input struct {
		Username     string `json:"username" binding:"required"`
		Email        string `json:"email" binding:"required,email"`
		Password     string `json:"password" binding:"required,min=8"`
		ReferralCode string `json:"referral_code"`
		// PDF §2.1 — Demographic fields (all optional at registration)
		Phone       string `json:"phone"`
		DateOfBirth string `json:"date_of_birth"` // Expected format: YYYY-MM-DD
		City        string `json:"city"`
		Country     string `json:"country"`
		Location    string `json:"location"`
		Interests   string `json:"interests"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if username or email already exists
	var existing models.User
	if err := config.DB.Where("username = ? OR email = ?", input.Username, input.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username or email already taken"})
		return
	}

	// Hash password
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	newReferralCode := services.GenerateReferralCode()

	// Parse optional DateOfBirth
	var dob *time.Time
	if input.DateOfBirth != "" {
		parsed, err := time.Parse("2006-01-02", input.DateOfBirth)
		if err == nil {
			dob = &parsed
		}
	}

	user := models.User{
		Username:     input.Username,
		Email:        input.Email,
		Password:     string(hashedPwd),
		Points:       0, // Points are now added via WalletLedger below
		Tier:         "Standard",
		Role:         models.RoleUser,
		IsActive:     true,
		KycStatus:    false,
		ReferralCode: newReferralCode,
		ReferredBy:   0,
		// Demographics
		Phone:       input.Phone,
		DateOfBirth: dob,
		City:        input.City,
		Country:     input.Country,
		Location:    input.Location,
		Interests:   input.Interests,
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user"})
		return
	}

	// Base sign-up bonus via ledger
	if err := services.CreditWalletTx(tx, user.ID, 100, models.TxTypeAdminAdjustment, 0, "Welcome Bonus", nil); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not issue welcome bonus"})
		return
	}

	// Process referral if provided
	if input.ReferralCode != "" {
		if err := services.ProcessReferral(tx, user.ID, input.ReferralCode); err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	// Refresh user to get latest points from DB after ledger txns
	config.DB.First(&user, user.ID)

	token, err := generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Account created successfully!",
		"token":   token,
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"tier":      user.Tier,
			"role":      user.Role,
			"is_active": user.IsActive,
			"points":    user.Points,
		},
	})
}

// LoginUser authenticates a user and returns a JWT
func LoginUser(c *gin.Context) {
	var input struct {
		Email         string `json:"email" binding:"required"`
		Password      string `json:"password" binding:"required"`
		TwoFactorCode string `json:"two_factor_code"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if user.TwoFactorEnabled {
		if input.TwoFactorCode == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "2fa_required"})
			return
		}

		valid := totp.Validate(input.TwoFactorCode, user.TwoFactorSecret)
		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid 2FA code"})
			return
		}
	}

	// Block banned users BEFORE generating the token
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been suspended. Contact support."})
		return
	}

	token, err := generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful!",
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

// LogoutUser invalidates the current JWT token
func LogoutUser(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
		return
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		middleware.InvalidateToken(parts[1])
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetUserStats returns calculated statistics for a specific user
func GetUserStats(c *gin.Context) {
	id := c.Param("id")

	var totalPredictions int64
	var wonPredictions int64
	var coinsEarned int64
	var coinsSpent int64

	config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ?", id).Count(&totalPredictions)
	config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ? AND is_correct = ?", id, true).Count(&wonPredictions)

	// Coins earned via predictions, referral bonuses, or admin adjustments
	config.DB.Model(&models.WalletLedger{}).Where("user_id = ? AND tx_type IN ?", id, []string{string(models.TxTypePredictionWin), string(models.TxTypeAdminAdjustment), string(models.TxTypeReferralBonus)}).Select("COALESCE(SUM(credit), 0)").Scan(&coinsEarned)

	// Coins spent via predictions and withdrawals
	config.DB.Model(&models.WalletLedger{}).Where("user_id = ? AND tx_type IN ?", id, []string{string(models.TxTypePredictionStake), string(models.TxTypeRedemption)}).Select("COALESCE(SUM(debit), 0)").Scan(&coinsSpent)

	var longestStreak int
	var current models.UserStreak
	if err := config.DB.Where("user_id = ?", id).First(&current).Error; err == nil {
		longestStreak = current.LongestStreak
	}

	winRate := float64(0)
	if totalPredictions > 0 {
		winRate = float64(wonPredictions) / float64(totalPredictions) * 100
	}

	var achievementCount int64
	config.DB.Model(&models.UserAchievement{}).Where("user_id = ?", id).Count(&achievementCount)

	c.JSON(http.StatusOK, gin.H{
		"total_predictions": totalPredictions,
		"win_rate":          fmt.Sprintf("%.1f%%", winRate),
		"coins_earned":      coinsEarned,
		"coins_spent":       coinsSpent,
		"longest_streak":    longestStreak,
		"achievement_count": achievementCount,
	})
}

// GetMe returns the logged-in user's profile from JWT context
func GetMe(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var user models.User

	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Trigger async profile completion check
	go services.CheckProfileCompletion(userID)

	c.JSON(http.StatusOK, gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"tier":          user.Tier,
		"role":          user.Role,
		"is_active":     user.IsActive,
		"points":        user.Points,
		"kyc_status":    user.KycStatus,
		"referral_code": user.ReferralCode,
		"created_at":    user.CreatedAt,
		// PDF §2.1 demographics
		"phone":         user.Phone,
		"date_of_birth": user.DateOfBirth,
		"city":          user.City,
		"country":       user.Country,
		"location":      user.Location,
		"interests":     user.Interests,
	})
}

// GetUser fetches a user by ID (public profile)
func GetUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User

	if err := config.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"tier":     user.Tier,
		"points":   user.Points,
	})
}

// ForgotPassword generates a secure reset token, persists it, and emails the user.
func ForgotPassword(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil || req["email"] == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", req["email"]).First(&user).Error; err != nil {
		// Don't leak user existence — always return the same message
		c.JSON(http.StatusOK, gin.H{"message": "If that email exists, a reset link has been sent."})
		return
	}

	// Generate a cryptographically random 32-byte hex token
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}
	token := hex.EncodeToString(rawBytes)

	// Invalidate any existing tokens for this user
	config.DB.Where("user_id = ?", user.ID).Delete(&models.PasswordResetToken{})

	resetRecord := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := config.DB.Create(&resetRecord).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to persist reset token"})
		return
	}

	// Send formatted HTML reset email asynchronously
	go services.SendPasswordResetEmail(user.Email, user.Username, token)

	c.JSON(http.StatusOK, gin.H{"message": "If that email exists, a reset link has been sent."})
}

// ResetPassword validates the token and updates the user's password.
func ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var record models.PasswordResetToken
	if err := config.DB.Where("token = ?", req.Token).First(&record).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	if time.Now().After(record.ExpiresAt) {
		config.DB.Delete(&record) // Clean up expired token
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reset token has expired. Please request a new one."})
		return
	}

	// Hash new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Update password and consume the token atomically
	tx := config.DB.Begin()

	if err := tx.Model(&models.User{}).Where("id = ?", record.UserID).
		Update("password", string(hashed)).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Delete the token so it cannot be reused
	if err := tx.Delete(&record).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to invalidate reset token"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully. Please log in with your new password."})
}
