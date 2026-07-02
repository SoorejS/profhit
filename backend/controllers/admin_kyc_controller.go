package controllers

import (
	"net/http"

	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

// GetAdminKycRequests returns all KYC verification records.
func GetAdminKycRequests(c *gin.Context) {
	var records []models.HyperVergeKYC
	// Preload the User to get their basic details
	if err := config.DB.Preload("User").Order("created_at desc").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch KYC records"})
		return
	}
	
	// Create a safe response that masks user fields just in case
	var safeRecords []map[string]interface{}
	for _, req := range records {
		safeRecords = append(safeRecords, map[string]interface{}{
			"id": req.ID,
			"user_id": req.UserID,
			"username": req.User.Username,
			"status": req.Status,
			"provider": req.Provider,
			"provider_reference": req.ProviderReference,
			"verification_result": req.VerificationResult,
			"failure_reason": req.FailureReason,
			"created_at": req.CreatedAt,
			"verified_at": req.VerifiedAt,
		})
	}
	
	c.JSON(http.StatusOK, safeRecords)
}

// GetAdminKycRequestByID returns a specific KYC verification record with detailed but masked data.
func GetAdminKycRequestByID(c *gin.Context) {
	id := c.Param("id")
	
	var kyc models.HyperVergeKYC
	if err := config.DB.Preload("User").First(&kyc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "KYC record not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"id": kyc.ID,
		"user_id": kyc.UserID,
		"username": kyc.User.Username,
		"status": kyc.Status,
		"provider": kyc.Provider,
		"provider_reference": kyc.ProviderReference,
		"pan_masked": kyc.PANNumberMasked,
		"aadhaar_last4": kyc.AadhaarLast4,
		"full_name": kyc.FullName,
		"dob": kyc.DOB,
		"face_match_score": kyc.FaceMatchScore,
		"liveness_score": kyc.LivenessScore,
		"verification_result": kyc.VerificationResult,
		"failure_reason": kyc.FailureReason,
		"created_at": kyc.CreatedAt,
		"verified_at": kyc.VerifiedAt,
	})
}
