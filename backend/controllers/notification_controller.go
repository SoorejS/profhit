package controllers

import (
	"github.com/gin-gonic/gin"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
)

func GetNotifications(c *gin.Context) {
	limit, offset := getPagination(c)
	items := []models.Notification{}
	if err := config.DB.Where("user_id = ?", c.MustGet("userID")).Order("created_at desc").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		c.JSON(500, gin.H{"error": "Could not load notifications"})
		return
	}
	c.JSON(200, items)
}
func ReadNotifications(c *gin.Context) {
	if err := config.DB.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", c.MustGet("userID")).Update("read_at", time.Now()).Error; err != nil {
		c.JSON(500, gin.H{"error": "Could not mark notifications read"})
		return
	}
	c.JSON(200, gin.H{"message": "Notifications marked read"})
}
