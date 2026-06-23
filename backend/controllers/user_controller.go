package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"profhit-backend/config"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/services"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func generateToken(user models.User) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "prophit-super-secret-key-2026"
	}

	claims := middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Tier:     user.Tier,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// RegisterUser creates a new user with hashed password
func RegisterUser(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
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

	user := models.User{
		Username:  input.Username,
		Email:     input.Email,
		Password:  string(hashedPwd),
		Points:    100,
		Tier:      "Standard",
		Role:      models.RoleUser,
		IsActive:  true,
		KycStatus: false,
	}

	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user"})
		return
	}

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
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
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

	token, err := generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Block banned users
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been suspended. Contact support."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful!",
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

// GetMe returns the logged-in user's profile from JWT context
func GetMe(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var user models.User

	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"tier":       user.Tier,
		"role":       user.Role,
		"is_active":  user.IsActive,
		"points":     user.Points,
		"kyc_status": user.KycStatus,
		"created_at": user.CreatedAt,
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

// UpdateKycStatus handles 3rd party KYC document verification with image upload
func UpdateKycStatus(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var user models.User

	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Handle Image Upload
	file, err := c.FormFile("document_image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document image is required"})
		return
	}

	// Create uploads directory if not exists
	os.Mkdir("uploads", 0755)
	
	// Save file
	filename := fmt.Sprintf("%d_%d_%s", userID, time.Now().Unix(), file.Filename)
	savePath := filepath.Join("uploads", filename)
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save document image"})
		return
	}

	docId := c.PostForm("document_id")
	if docId == "" {
		docId = "MOCK_DOC"
	}

	// Create a Pending KYC Request
	kycReq := models.KycRequest{
		UserID:     user.ID,
		DocumentID: docId,
		Status:     "Pending",
	}

	if err := config.DB.Create(&kycReq).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create KYC request"})
		return
	}

	// Return a message stating it's under review.
	c.JSON(http.StatusOK, gin.H{
		"message": "KYC Document submitted successfully! It is now pending Admin approval.",
		"user": gin.H{
			"id":     user.ID,
			"tier":   user.Tier,
			"points": user.Points,
		},
	})
}

// ForgotPassword handles sending a password reset email via SMTP
func ForgotPassword(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", req["email"]).First(&user).Error; err != nil {
		// Don't leak user existence
		c.JSON(http.StatusOK, gin.H{"message": "If that email exists, a reset link has been sent."})
		return
	}

	// In a real app, generate a secure token and save to DB
	resetToken := "mock-reset-token-123"
	resetLink := "http://localhost:3000/reset?token=" + resetToken

	body := "Hello " + user.Username + ",\n\nClick the link below to reset your password:\n" + resetLink + "\n\nThanks,\nThe PROPHIT Team"
	
	go services.SendEmail(user.Email, "Reset Your PROPHIT Password", body)

	c.JSON(http.StatusOK, gin.H{"message": "If that email exists, a reset link has been sent."})
}
