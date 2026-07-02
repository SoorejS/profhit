package middleware

import (
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
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Tier     string `json:"tier"`
	Role     string `json:"role"` // RBAC role embedded in token
	jwt.RegisteredClaims
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

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server misconfiguration: JWT_SECRET not set"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(parts[1], &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("tier", claims.Tier)
		c.Set("role", claims.Role)

		// DB Check for active/suspended status to ensure real-time bans apply
		var user models.User
		if err := config.DB.First(&user, claims.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User no longer exists"})
			c.Abort()
			return
		}

		if !user.IsActive {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is banned"})
			c.Abort()
			return
		}

		if user.SuspendedUntil != nil && user.SuspendedUntil.After(time.Now()) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is temporarily suspended until " + user.SuspendedUntil.Format(time.RFC3339)})
			c.Abort()
			return
		}

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
