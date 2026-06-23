package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

// GetAllMarkets fetches all markets, optionally filtered by category
func GetAllMarkets(c *gin.Context) {
	var markets []models.Market
	category := c.Query("category")

	query := config.DB.Where("resolution_status = ?", "Open")
	if category != "" {
		query = query.Where("category = ?", category)
	}

	if err := query.Order("volume desc").Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch markets"})
		return
	}

	c.JSON(http.StatusOK, markets)
}

// CreateMarket allows an admin to create a new prediction market
func CreateMarket(c *gin.Context) {
	var market models.Market
	if err := c.ShouldBindJSON(&market); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default prices
	if market.YesPrice == 0 {
		market.YesPrice = 50
	}
	if market.NoPrice == 0 {
		market.NoPrice = 50
	}
	market.ResolutionStatus = "Open"

	if err := config.DB.Create(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create market"})
		return
	}

	c.JSON(http.StatusCreated, market)
}

// GetMarketByID fetches a single market with full detail
func GetMarketByID(c *gin.Context) {
	id := c.Param("id")
	var market models.Market

	if err := config.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	c.JSON(http.StatusOK, market)
}

// ResolveMarket closes a market and declares a winner
func ResolveMarket(c *gin.Context) {
	id := c.Param("id")
	var market models.Market

	if err := config.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus != "Open" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Market is already resolved"})
		return
	}

	var req struct {
		Winner string `json:"winner" binding:"required"` // "Yes" or "No"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Winner != "Yes" && req.Winner != "No" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Winner must be 'Yes' or 'No'"})
		return
	}

	// --- Payout Logic ---
	tx := config.DB.Begin()

	resolutionStatus := "Resolved_Yes"
	if req.Winner == "No" {
		resolutionStatus = "Resolved_No"
	}

	// Fetch all winning trades for this market
	var winningTrades []models.Trade
	tx.Where("market_id = ? AND outcome = ?", market.ID, req.Winner).Find(&winningTrades)

	// Pay out winning traders: each share is worth 1 point
	for _, trade := range winningTrades {
		var user models.User
		if err := tx.First(&user, trade.UserID).Error; err != nil {
			continue
		}
		payout := int(trade.Shares)
		user.Points += payout
		tx.Save(&user)

		// Record the payout
		trade.Payout = payout
		tx.Save(&trade)
	}

	market.ResolutionStatus = resolutionStatus
	if err := tx.Save(&market).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve market"})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message":          "Market resolved successfully!",
		"market":           market,
		"winning_trades":   len(winningTrades),
		"resolution":       resolutionStatus,
	})
}

// ProposeMarket allows a regular user to suggest a new market
func ProposeMarket(c *gin.Context) {
	var market models.Market
	if err := c.ShouldBindJSON(&market); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.MustGet("userID").(uint)
	market.CreatorID = userID
	market.ResolutionStatus = "Proposed"
	market.YesPrice = 50
	market.NoPrice = 50

	if err := config.DB.Create(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to propose market"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Market proposed successfully and is awaiting approval.", "market": market})
}

// ApproveMarket allows an admin to move a proposed market to Open status
func ApproveMarket(c *gin.Context) {
	id := c.Param("id")
	var market models.Market

	if err := config.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus != "Proposed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only proposed markets can be approved"})
		return
	}

	market.ResolutionStatus = "Open"
	if err := config.DB.Save(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve market"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Market approved and is now live!", "market": market})
}

// GetProposedMarkets fetches all markets awaiting approval (Admin only)
func GetProposedMarkets(c *gin.Context) {
	var markets []models.Market
	if err := config.DB.Where("resolution_status = ?", "Proposed").Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch proposed markets"})
		return
	}
	c.JSON(http.StatusOK, markets)
}

