package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

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

// RejectWithdrawal rejects the withdrawal and refunds the coins via the ledger
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
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	wReq.Status = "Rejected"
	wReq.AdminID = &adminID
	if err := tx.Save(&wReq).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
		return
	}

	// Refund via the immutable ledger — no direct User.Points mutation
	if err := services.CreditWalletTx(
		tx,
		wReq.UserID,
		wReq.CoinsDeducted,
		models.TxTypeRefund,
		0,
		"Refund: rejected voucher redemption",
		&adminID,
	); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refund coins: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal rejected and coins refunded"})
}

