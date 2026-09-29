package controllers

import (
	"math"
	"net/http"
	"strconv"

	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

// GetWalletHistory handles GET /api/wallet/history with pagination contract
func GetWalletHistory(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	txType := c.Query("type")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "20")))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := config.DB.Model(&models.WalletLedger{}).Where("user_id = ?", userID)
	if txType != "" {
		query = query.Where("type = ?", txType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count wallet transactions"})
		return
	}

	var txns []models.WalletLedger
	offset := (page - 1) * pageSize
	if err := query.Order("created_at desc, id desc").Limit(pageSize).Offset(offset).Find(&txns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wallet history"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"items":       txns,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
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
