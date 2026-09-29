package controllers

import (
	"log"
	"net/http"
	"os"
	"strings"

	"profhit-backend/middleware"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		for _, allowed := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
			if strings.TrimSpace(allowed) == origin {
				return true
			}
		}
		return origin == os.Getenv("APP_URL")

	},
}

func WsHandler(c *gin.Context) {
	// Authentication via query parameter
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
		return
	}

	claims, _, err := middleware.ValidateToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session"})
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	// Register with centralized service
	services.UpgradeAndRegister(ws, claims.UserID, tokenStr)
}
