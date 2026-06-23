package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

type LimitOrderRequest struct {
	MarketID    uint    `json:"market_id" binding:"required"`
	Outcome     string  `json:"outcome" binding:"required"`
	TargetPrice int     `json:"target_price" binding:"required"`
	Points      float64 `json:"points" binding:"required"`
}

func CreateLimitOrder(c *gin.Context) {
	var req LimitOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.MustGet("userID").(uint)

	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if float64(user.Points) < req.Points {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient points balance"})
		return
	}

	order := models.LimitOrder{
		UserID:      userID,
		MarketID:    req.MarketID,
		Outcome:     req.Outcome,
		TargetPrice: req.TargetPrice,
		Points:      req.Points,
		Status:      "Pending",
	}

	tx := config.DB.Begin()
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create limit order"})
		return
	}

	// Lock the points
	user.Points -= int(req.Points)
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lock points"})
		return
	}
	tx.Commit()

	c.JSON(http.StatusOK, order)
}

func GetUserLimitOrders(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var orders []models.LimitOrder

	if err := config.DB.Where("user_id = ? AND status = ?", userID, "Pending").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func CancelLimitOrder(c *gin.Context) {
	orderID := c.Param("id")
	userID := c.MustGet("userID").(uint)

	var order models.LimitOrder
	if err := config.DB.First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized"})
		return
	}

	if order.Status != "Pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order cannot be cancelled"})
		return
	}

	tx := config.DB.Begin()
	order.Status = "Cancelled"
	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
		return
	}

	// Refund the points
	var user models.User
	config.DB.First(&user, userID)
	user.Points += int(order.Points)
	tx.Save(&user)
	
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{"message": "Order cancelled and points refunded"})
}
