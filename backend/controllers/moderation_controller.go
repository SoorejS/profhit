package controllers

import (
	"fmt"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"time"

	"github.com/gin-gonic/gin"
)

// GetReports fetches all reports (admin/super_admin)
func GetReports(c *gin.Context) {
	var reports []models.Report
	status := c.Query("status")

	query := config.DB.Order("created_at desc")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reports"})
		return
	}

	c.JSON(http.StatusOK, reports)
}

// ResolveReport handles a report and applies moderation action if required
func ResolveReport(c *gin.Context) {
	reportID := c.Param("id")

	var req struct {
		Action       string `json:"action" binding:"required"` // "Dismiss", "Mute", "Suspend", "Ban", "DeleteComment"
		DurationDays int    `json:"duration_days"`             // Optional: for Mute/Suspend
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var report models.Report
	if err := config.DB.First(&report, reportID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	if report.Status != "Pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Report already resolved or dismissed"})
		return
	}

	adminID := c.MustGet("userID").(uint)

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	switch req.Action {
	case "Dismiss":
		report.Status = "Dismissed"
	case "DeleteComment":
		if report.TargetType != "Comment" {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Action DeleteComment only applies to Comment target"})
			return
		}
		if err := tx.Delete(&models.Comment{}, report.TargetID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete comment"})
			return
		}
		report.Status = "Resolved"
	case "Mute":
		if report.TargetType != "User" {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Action Mute only applies to User target"})
			return
		}
		if err := tx.Model(&models.User{}).Where("id = ?", report.TargetID).Update("is_muted", true).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mute user"})
			return
		}
		report.Status = "Resolved"
	case "Suspend":
		if report.TargetType != "User" {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Action Suspend only applies to User target"})
			return
		}
		if req.DurationDays <= 0 {
			req.DurationDays = 7 // default to 7 days
		}
		until := time.Now().AddDate(0, 0, req.DurationDays)
		if err := tx.Model(&models.User{}).Where("id = ?", report.TargetID).Updates(map[string]interface{}{"suspended_until": until}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to suspend user"})
			return
		}
		report.Status = "Resolved"
	case "Ban":
		if report.TargetType != "User" {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Action Ban only applies to User target"})
			return
		}
		if err := tx.Model(&models.User{}).Where("id = ?", report.TargetID).Update("is_active", false).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ban user"})
			return
		}
		report.Status = "Resolved"
	default:
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown action"})
		return
	}

	report.ResolvedByID = &adminID
	if err := tx.Save(&report).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save report status"})
		return
	}

	// Audit log
	_ = services.LogAction(tx, adminID, "RESOLVE_REPORT", fmt.Sprintf("report_%d", report.ID), fmt.Sprintf("Action taken: %s", req.Action), c.ClientIP())

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Report handled successfully"})
}
