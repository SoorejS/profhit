package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

func AddComment(c *gin.Context) {
	marketID := c.Param("id")
	userID := c.MustGet("userID").(uint)

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var market models.Market
	if err := config.DB.First(&market, marketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	comment := models.Comment{
		MarketID: market.ID,
		UserID:   user.ID,
		Username: user.Username,
		Content:  req.Content,
	}

	if err := config.DB.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add comment"})
		return
	}

	// Broadcast the comment
	BroadcastUpdate("new_comment", gin.H{
		"market_id": market.ID,
		"comment":   comment,
	})

	c.JSON(http.StatusOK, comment)
}

func GetComments(c *gin.Context) {
	marketID := c.Param("id")
	var comments []models.Comment

	if err := config.DB.Where("market_id = ?", marketID).Order("created_at asc").Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}

	c.JSON(http.StatusOK, comments)
}
