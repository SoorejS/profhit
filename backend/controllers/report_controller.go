package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SubmitReport handles user reports for comments/markets/users
func SubmitReport(c *gin.Context) {
	var req struct {
		TargetType  string `json:"target_type" binding:"required"`
		TargetID    uint   `json:"target_id" binding:"required"`
		Reason      string `json:"reason" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate target type
	validTypes := map[string]bool{"User": true, "Market": true, "Comment": true}
	if !validTypes[req.TargetType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target_type. Must be User, Market, or Comment."})
		return
	}

	reporterID := c.MustGet("userID").(uint)

	report := models.Report{
		ReporterID:  reporterID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Reason:      req.Reason,
		Description: req.Description,
		Status:      "Pending",
	}

	if err := config.DB.Create(&report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Report submitted successfully."})
}

// GetUserReports returns all reports submitted by the authenticated user
func GetUserReports(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")
	limit := 50
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	var reports []models.Report
	if err := config.DB.Where("reporter_id = ?", userID).Order("created_at desc").Limit(limit).Offset(offset).Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reports"})
		return
	}

	c.JSON(http.StatusOK, reports)
}
