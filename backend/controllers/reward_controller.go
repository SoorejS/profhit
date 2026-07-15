package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// GetRewardCatalog fetches all active reward items
func GetRewardCatalog(c *gin.Context) {
	var items []models.RewardItem
	if err := config.DB.Where("is_active = ?", true).Order("cost asc").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reward catalog"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// SubmitRedemption allows a user to redeem coins for an item
func SubmitRedemption(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var req struct {
		RewardItemID uint `json:"reward_item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var item models.RewardItem
	if err := config.DB.First(&item, req.RewardItemID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reward item not found"})
		return
	}

	if !item.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reward item is not currently active"})
		return
	}

	if item.Inventory == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reward item is out of stock"})
		return
	}

	// Verify points
	var user models.User
	config.DB.First(&user, userID)
	if user.Points < item.Cost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient coins"})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Deduct points
	if err := services.DebitWalletTx(tx, userID, item.Cost, models.TxTypeRedemption, 0, "Redemption: "+item.Name, nil); err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to deduct coins. " + err.Error()})
		return
	}

	// Create Redemption Record
	redemption := models.Redemption{
		UserID:       userID,
		RewardItemID: item.ID,
		CostPaid:     item.Cost,
		Status:       "Pending",
	}

	if err := tx.Create(&redemption).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create redemption request"})
		return
	}

	// Update inventory if not infinite
	if item.Inventory > 0 {
		item.Inventory -= 1
		tx.Save(&item)
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Redemption request submitted successfully", "redemption": redemption})
}

// GetUserRedemptions fetches redemptions for a user
func GetUserRedemptions(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var redemptions []models.Redemption

	if err := config.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&redemptions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch redemptions"})
		return
	}

	c.JSON(http.StatusOK, redemptions)
}

// AdminGetRedemptions fetches all redemptions for admin
func AdminGetRedemptions(c *gin.Context) {
	status := c.Query("status")
	var redemptions []models.Redemption
	query := config.DB.Order("created_at desc")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Find(&redemptions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch redemptions"})
		return
	}
	c.JSON(http.StatusOK, redemptions)
}

// AdminProcessRedemption allows admin to approve, reject, or complete a redemption
func AdminProcessRedemption(c *gin.Context) {
	id := c.Param("id")
	adminID := c.MustGet("userID").(uint)

	var req struct {
		Status       string `json:"status" binding:"required"` // Approved, Rejected, Completed
		VoucherCode  string `json:"voucher_code"`
		AdminRemarks string `json:"admin_remarks"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var redemption models.Redemption
	if err := config.DB.First(&redemption, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Redemption not found"})
		return
	}

	if redemption.Status == "Completed" || redemption.Status == "Rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Redemption is already finalized"})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	redemption.Status = req.Status
	redemption.AdminRemarks = req.AdminRemarks
	redemption.AdminID = &adminID

	if req.Status == "Completed" {
		redemption.VoucherCode = req.VoucherCode
		services.BroadcastToUser(redemption.UserID, "reward_approved", "Your redemption has been processed and your voucher code is available!")
	} else if req.Status == "Rejected" {
		// Refund coins
		if err := services.CreditWalletTx(tx, redemption.UserID, redemption.CostPaid, models.TxTypeRefund, 0, "Redemption Rejected Refund", &adminID); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refund coins"})
			return
		}

		// Restore inventory if applicable
		var item models.RewardItem
		if err := tx.First(&item, redemption.RewardItemID).Error; err == nil && item.Inventory >= 0 {
			item.Inventory += 1
			tx.Save(&item)
		}

		services.BroadcastToUser(redemption.UserID, "reward_rejected", "Your redemption was rejected. Coins have been refunded.")
	}

	if err := tx.Save(&redemption).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update redemption"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Redemption processed", "redemption": redemption})
}
