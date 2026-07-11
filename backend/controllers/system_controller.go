package controllers

import (
	"net/http"
	"profhit-backend/config"

	"github.com/gin-gonic/gin"
)

// HealthCheck verifies the API is running and the database is accessible
func HealthCheck(c *gin.Context) {
	// Ping database
	sqlDB, err := config.DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": "Database not reachable"})
		return
	}
	
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": "Database ping failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"message": "PROPHIT Backend is running",
	})
}
