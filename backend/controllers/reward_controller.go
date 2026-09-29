package controllers

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strconv"

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

	tx := config.DB.Begin()
	defer tx.Rollback()
	var item models.RewardItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, req.RewardItemID).Error; err != nil {
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
	tx.First(&user, userID)
	if !user.KycStatus {
		c.JSON(403, gin.H{"error": "KYC verification is required for redemption"})
		return
	}
	if user.Points < item.Cost {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient coins"})
		return
	}

	// Deduct points
	if err := services.DebitWalletTx(tx, userID, item.Cost, models.TxTypeRedemption, 0, "Redemption: "+item.Name, nil); err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient unexpired balance or wallet unavailable"})
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
		if err := tx.Save(&item).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not update inventory"})
			return
		}
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "20")))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := config.DB.Model(&models.Redemption{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count redemptions"})
		return
	}

	var redemptions []models.Redemption
	if err := query.Order("created_at desc, id desc").Limit(pageSize).Offset(offset).Find(&redemptions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch redemptions"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"items":       redemptions,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// AdminGetRedemptions fetches all redemptions for admin
func AdminGetRedemptions(c *gin.Context) {
	status := c.Query("status")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "20")))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := config.DB.Model(&models.Redemption{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count redemptions"})
		return
	}

	var redemptions []models.Redemption
	if err := query.Order("created_at desc, id desc").Limit(pageSize).Offset(offset).Find(&redemptions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch redemptions"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"items":       redemptions,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
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

	tx := config.DB.Begin()
	defer tx.Rollback()
	var redemption models.Redemption
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&redemption).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Redemption not found"})
		return
	}

	if redemption.Status == "Completed" || redemption.Status == "Rejected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Redemption is already finalized"})
		return
	}

	legal := (redemption.Status == "Pending" && (req.Status == "Approved" || req.Status == "Rejected")) || (redemption.Status == "Approved" && (req.Status == "Completed" || req.Status == "Rejected"))
	if !legal || (req.Status == "Completed" && req.VoucherCode == "") {
		c.JSON(400, gin.H{"error": "Invalid redemption transition or missing voucher code"})
		return
	}
	redemption.Status = req.Status
	redemption.AdminRemarks = req.AdminRemarks
	redemption.AdminID = &adminID

	if req.Status == "Completed" {
		redemption.VoucherCode = req.VoucherCode

	} else if req.Status == "Rejected" {
		// Match submission lock order: reward item, then wallet.
		var item models.RewardItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, redemption.RewardItemID).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not lock reward inventory"})
			return
		}
		// Refund coins
		if err := services.CreditWalletTx(tx, redemption.UserID, redemption.CostPaid, models.TxTypeRefund, 0, "Redemption Rejected Refund", &adminID); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refund coins"})
			return
		}

		if err := tx.Model(&models.RewardItem{}).Where("id = ? AND inventory >= 0", redemption.RewardItemID).UpdateColumn("inventory", gorm.Expr("inventory + 1")).Error; err != nil {
			c.JSON(500, gin.H{"error": "Could not restore inventory"})
			return
		}

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
