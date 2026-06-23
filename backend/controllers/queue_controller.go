package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

// GetKycRequests returns all pending KYC requests
func GetKycRequests(c *gin.Context) {
	var reqs []models.KycRequest
	if err := config.DB.Where("status = ?", "Pending").Order("created_at asc").Find(&reqs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch KYC requests"})
		return
	}
	c.JSON(http.StatusOK, reqs)
}

// ApproveKycRequest approves a KYC request and upgrades the user
func ApproveKycRequest(c *gin.Context) {
	reqID := c.Param("id")
	adminID := c.MustGet("userID").(uint)

	var kycReq models.KycRequest
	if err := config.DB.First(&kycReq, reqID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	if kycReq.Status != "Pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is not pending"})
		return
	}

	tx := config.DB.Begin()

	kycReq.Status = "Approved"
	kycReq.AdminID = &adminID
	if err := tx.Save(&kycReq).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
		return
	}

	// Upgrade User
	var user models.User
	if err := tx.First(&user, kycReq.UserID).Error; err == nil {
		user.KycStatus = true
		user.Tier = "Gold"
		tx.Save(&user)
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "KYC Approved"})
}

// RejectKycRequest rejects a KYC request
func RejectKycRequest(c *gin.Context) {
	reqID := c.Param("id")
	adminID := c.MustGet("userID").(uint)

	var kycReq models.KycRequest
	if err := config.DB.First(&kycReq, reqID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	kycReq.Status = "Rejected"
	kycReq.AdminID = &adminID
	config.DB.Save(&kycReq)

	c.JSON(http.StatusOK, gin.H{"message": "KYC Rejected"})
}

// GetWithdrawals returns all pending withdrawal requests
func GetWithdrawals(c *gin.Context) {
	var reqs []models.WithdrawalRequest
	if err := config.DB.Where("status = ?", "Pending").Order("created_at asc").Find(&reqs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch withdrawals"})
		return
	}
	c.JSON(http.StatusOK, reqs)
}

// ApproveWithdrawal approves the withdrawal
func ApproveWithdrawal(c *gin.Context) {
	reqID := c.Param("id")
	adminID := c.MustGet("userID").(uint)

	var wReq models.WithdrawalRequest
	if err := config.DB.First(&wReq, reqID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	if wReq.Status != "Pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is not pending"})
		return
	}

	wReq.Status = "Approved"
	wReq.AdminID = &adminID
	config.DB.Save(&wReq)

	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal Approved"})
}

// RejectWithdrawal rejects the withdrawal and refunds the points
func RejectWithdrawal(c *gin.Context) {
	reqID := c.Param("id")
	adminID := c.MustGet("userID").(uint)

	var wReq models.WithdrawalRequest
	if err := config.DB.First(&wReq, reqID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	if wReq.Status != "Pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is not pending"})
		return
	}

	tx := config.DB.Begin()

	wReq.Status = "Rejected"
	wReq.AdminID = &adminID
	if err := tx.Save(&wReq).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
		return
	}

	// Refund User
	var user models.User
	if err := tx.First(&user, wReq.UserID).Error; err == nil {
		user.Points += wReq.Amount
		tx.Save(&user)
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal Rejected and points refunded"})
}
