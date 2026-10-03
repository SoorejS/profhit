package controllers

import (
	"gorm.io/gorm/clause"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetWithdrawals returns all pending withdrawal requests
func GetWithdrawals(c *gin.Context) {
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

	var reqs []models.WithdrawalRequest
	if err := config.DB.Order("created_at asc").Limit(limit).Offset(offset).Find(&reqs).Error; err != nil {
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
	if err := config.DB.Where("id = ?", reqID).First(&wReq).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	if wReq.Status != "Pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is not pending"})
		return
	}

	wReq.Status = "Processing"
	wReq.AdminID = &adminID
	result := config.DB.Model(&models.WithdrawalRequest{}).Where("id = ? AND status = ?", wReq.ID, "Pending").Updates(map[string]interface{}{"status": "Processing", "admin_id": adminID})
	if result.Error != nil || result.RowsAffected != 1 {
		c.JSON(409, gin.H{"error": "Request changed; refresh and retry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal Approved"})
}

// RejectWithdrawal rejects the withdrawal and refunds the coins via the ledger
func RejectWithdrawal(c *gin.Context) {
	reqID := c.Param("id")
	adminID := c.MustGet("userID").(uint)

	tx := config.DB.Begin()
	defer tx.Rollback()
	var wReq models.WithdrawalRequest
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", reqID).First(&wReq).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	if wReq.Status != "Pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is not pending"})
		return
	}

	wReq.Status = "Rejected"
	wReq.AdminID = &adminID
	if err := tx.Save(&wReq).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
		return
	}

	// Refund via the immutable ledger — no direct User.Points mutation
	if err := services.RefundRedemptionTx(tx, wReq.UserID, "tier", wReq.ID, &adminID); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refund coins"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal rejected and coins refunded"})
}
