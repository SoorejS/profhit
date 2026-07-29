package controllers

import (
	"net/http"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// GetTrendingNews returns the top cached news articles
func GetTrendingNews(c *gin.Context) {
	articles, err := services.GetTrendingNews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch trending news"})
		return
	}

	c.JSON(http.StatusOK, articles)
}
