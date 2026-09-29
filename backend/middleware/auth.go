package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	TokenVersion uint   `json:"token_version"`
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	Tier         string `json:"tier"`
	Role         string `json:"role"` // RBAC role embedded in token
	jwt.RegisteredClaims
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func InvalidateToken(token string) error {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) { return []byte(os.Getenv("JWT_SECRET")), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return err
	}
	return config.DB.Create(&models.RevokedToken{Hash: tokenHash(token), ExpiresAt: claims.ExpiresAt.Time}).Error
}
func ValidateToken(tokenString string) (*Claims, models.User, error) {
	var user models.User
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, user, errors.New("authentication unavailable")
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) { return []byte(secret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !token.Valid || claims.UserID == 0 {
		return nil, user, errors.New("invalid or expired token")
	}
	var count int64
	if err := config.DB.Model(&models.RevokedToken{}).Where("hash = ?", tokenHash(tokenString)).Count(&count).Error; err != nil {
		return nil, user, err
	}
	if count > 0 {
		return nil, user, errors.New("token revoked")
	}
	if err := config.DB.First(&user, claims.UserID).Error; err != nil {
		return nil, user, err
	}
	if !user.IsActive || (user.SuspendedUntil != nil && user.SuspendedUntil.After(time.Now())) || claims.TokenVersion != user.TokenVersion {
		return nil, user, errors.New("session no longer valid")
	}
	return claims, user, nil
}

// AuthRequired validates JWT and sets user context values
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		claims, user, err := ValidateToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("username", user.Username)
		c.Set("tier", user.Tier)
		c.Set("role", user.Role)

		c.Next()
	}
}

// RoleRequired restricts a route to users with one of the specified roles.
// Must be used after AuthRequired().
func RoleRequired(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: no role found"})
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: invalid role"})
			c.Abort()
			return
		}

		for _, allowed := range roles {
			if roleStr == allowed {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":         "Access denied: insufficient permissions",
			"your_role":     roleStr,
			"required_role": roles,
		})
		c.Abort()
	}
}
