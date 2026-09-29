package controllers

import (
	"math"
	"net/http"
	"strconv"

	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

// GetAdminKycRequests returns KYC verification records with pagination contract.
func GetAdminKycRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "20")))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := config.DB.Model(&models.HyperVergeKYC{})

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count KYC records"})
		return
	}

	var records []models.HyperVergeKYC
	if err := query.Preload("User").Order("created_at desc, id desc").Limit(pageSize).Offset(offset).Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch KYC records"})
		return
	}

	var safeRecords []map[string]interface{}
	for _, req := range records {
		safeRecords = append(safeRecords, map[string]interface{}{
			"id":                  req.ID,
			"user_id":             req.UserID,
			"username":            req.User.Username,
			"status":              req.Status,
			"provider":            req.Provider,
			"provider_reference":  req.ProviderReference,
			"verification_result": req.VerificationResult,
			"failure_reason":      req.FailureReason,
			"created_at":          req.CreatedAt,
			"verified_at":         req.VerifiedAt,
		})
	}

	if safeRecords == nil {
		safeRecords = []map[string]interface{}{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"items":       safeRecords,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetAdminKycRequestByID returns a specific KYC verification record with detailed but masked data.
func GetAdminKycRequestByID(c *gin.Context) {
	id := c.Param("id")

	var kyc models.HyperVergeKYC
	if err := config.DB.Preload("User").Where("id = ?", id).First(&kyc).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "KYC record not found"})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"id":                  kyc.ID,
		"user_id":             kyc.UserID,
		"username":            kyc.User.Username,
		"status":              kyc.Status,
		"provider":            kyc.Provider,
		"provider_reference":  kyc.ProviderReference,
		"verification_result": kyc.VerificationResult,
		"failure_reason":      kyc.FailureReason,
		"created_at":          kyc.CreatedAt,
		"verified_at":         kyc.VerifiedAt,
	})
}
