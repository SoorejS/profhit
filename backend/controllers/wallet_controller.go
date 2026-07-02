package controllers

import (
	"net/http"

	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

// GetWalletHistory handles GET /api/wallet/history
func GetWalletHistory(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	txType := c.Query("type")

	var txns []models.WalletLedger
	query := config.DB.Where("user_id = ?", userID)

	if txType != "" {
		query = query.Where("type = ?", txType)
	}

	if err := query.Order("created_at desc").Find(&txns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wallet history"})
		return
	}

	c.JSON(http.StatusOK, txns)
}

// GetWalletTransaction handles GET /api/wallet/transaction/:id
func GetWalletTransaction(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id := c.Param("id")

	var txn models.WalletLedger
	if err := config.DB.Where("id = ? AND user_id = ?", id, userID).First(&txn).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	c.JSON(http.StatusOK, txn)
}
