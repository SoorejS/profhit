package controllers

import (
	"github.com/gin-gonic/gin"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
)

func GetWalletBatches(c *gin.Context) {
	id := c.MustGet("userID").(uint)
	var batches []models.CoinBatch
	if config.DB.Where("user_id = ? AND balance > 0", id).Order("created_at,id").Find(&batches).Error != nil {
		c.JSON(500, gin.H{"error": "Could not load coin batches"})
		return
	}
	spendable := 0
	expired := 0
	for _, b := range batches {
		if b.ExpiresAt.After(time.Now().UTC()) {
			spendable += b.Balance
		} else {
			expired += b.Balance
		}
	}
	c.JSON(200, gin.H{"batches": batches, "spendable": spendable, "awaiting_expiry_processing": expired, "expiry_months": 6})
}
func GetVoucherRequests(c *gin.Context) {
	var rows []models.WithdrawalRequest
	if config.DB.Where("user_id = ?", c.MustGet("userID").(uint)).Order("id desc").Limit(100).Find(&rows).Error != nil {
		c.JSON(500, gin.H{"error": "Could not load vouchers"})
		return
	}
	c.JSON(200, rows)
}
