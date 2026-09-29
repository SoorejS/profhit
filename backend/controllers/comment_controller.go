package controllers

import (
	"math"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func AddComment(c *gin.Context) {
	marketID := c.Param("id")
	userID := c.MustGet("userID").(uint)

	var req struct {
		Content string `json:"content" binding:"required,max=2000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		c.JSON(400, gin.H{"error": "Comment cannot be empty"})
		return
	}
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.IsMuted {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are currently muted and cannot comment."})
		return
	}

	var market models.Market
	if err := config.DB.Where("id = ? AND resolution_status NOT IN ?", marketID, []string{"Draft", "Proposed"}).First(&market).Error; err != nil {
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
	if market.Visibility == "Public" {
		services.BroadcastToAll("new_comment", gin.H{
			"market_id": marketID,
			"comment": gin.H{
				"id":         comment.ID,
				"user_id":    user.ID,
				"username":   user.Username,
				"content":    comment.Content,
				"created_at": comment.CreatedAt,
			},
		})
	}

	c.JSON(http.StatusOK, comment)
}

func GetComments(c *gin.Context) {
	marketID := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "50")))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var market models.Market
	if err := config.DB.Where("id = ? AND resolution_status NOT IN ?", marketID, []string{"Draft", "Proposed"}).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	query := config.DB.Model(&models.Comment{}).Where("market_id = ?", marketID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count comments"})
		return
	}

	var comments = []models.Comment{}
	if err := query.Order("created_at asc, id asc").Limit(pageSize).Offset(offset).Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"items":       comments,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}
