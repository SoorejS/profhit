package controllers

import (
	"errors"
	"net/http"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// GetTrendingNews returns the top cached news articles
func GetTrendingNews(c *gin.Context) {
	articles, err := services.GetTrendingNews()
	if err != nil {
		if errors.Is(err, services.ErrNewsNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "News is not configured"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "News provider is unavailable"})
		return
	}

	c.JSON(http.StatusOK, articles)
}
