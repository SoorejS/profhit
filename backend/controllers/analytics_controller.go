package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

// GetAuditLogs returns a paginated list of system audit logs (super_admin only)
func GetAuditLogs(c *gin.Context) {
	var logs []models.AuditLog

	query := config.DB.Order("created_at desc")

	// Optional filtering
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if adminID := c.Query("admin_id"); adminID != "" {
		query = query.Where("admin_id = ?", adminID)
	}

	if err := query.Limit(100).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}
