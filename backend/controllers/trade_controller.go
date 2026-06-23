package controllers

import (
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

type TradeRequest struct {
	MarketID uint    `json:"market_id" binding:"required"`
	Outcome  string  `json:"outcome" binding:"required"`
	Points   float64 `json:"points" binding:"required"`
}

// PlaceTrade handles a user buying Yes/No shares — userID from JWT
func PlaceTrade(c *gin.Context) {
	var req TradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user from JWT middleware
	userID := c.MustGet("userID").(uint)

	var user models.User
	var market models.Market

	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := config.DB.First(&market, req.MarketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus != "Open" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "This market is already closed"})
		return
	}

	if float64(user.Points) < req.Points {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient points balance"})
		return
	}

	price := market.YesPrice
	if req.Outcome == "No" {
		price = market.NoPrice
	}
	priceInPts := float64(price) / 100.0
	shares := req.Points / priceInPts

	trade := models.Trade{
		UserID:   user.ID,
		MarketID: market.ID,
		Outcome:  req.Outcome,
		Points:   req.Points,
		Shares:   shares,
		Price:    price,
	}

	// Advanced AMM Math (Dynamic Liquidity-based Price Shift)
	// Base liquidity prevents massive swings early on, behaving like a Constant Product Market Maker
	baseLiquidity := 5000.0
	totalLiquidity := float64(market.Volume) + baseLiquidity
	
	// Shift is proportional to trade size vs total liquidity depth
	shiftFloat := (req.Points / totalLiquidity) * 100.0
	shift := int(shiftFloat)
	if shift < 1 && req.Points >= 10 { // Ensure some minimum movement for decent trades
		shift = 1
	}

	if req.Outcome == "Yes" {
		market.YesPrice = clamp(market.YesPrice+shift, 2, 98)
		market.NoPrice = 100 - market.YesPrice
	} else {
		market.NoPrice = clamp(market.NoPrice+shift, 2, 98)
		market.YesPrice = 100 - market.NoPrice
	}

	tx := config.DB.Begin()

	if err := tx.Create(&trade).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record trade"})
		return
	}

	user.Points -= int(req.Points)
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct points"})
		return
	}

	market.Volume += req.Points
	if err := tx.Save(&market).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update market"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	// Trigger limit orders that match the new price
	go checkAndExecuteLimitOrders(market.ID, market.YesPrice, market.NoPrice)

	// Broadcast the update to all connected clients
	BroadcastUpdate("trade_placed", gin.H{
		"market_id": market.ID,
		"yes_price": market.YesPrice,
		"no_price":  market.NoPrice,
		"volume":    market.Volume,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Trade successful!",
		"trade":   trade,
		"balance": user.Points,
	})
}

// checkAndExecuteLimitOrders looks for pending orders that hit their target price
func checkAndExecuteLimitOrders(marketID uint, yesPrice, noPrice int) {
	var orders []models.LimitOrder
	config.DB.Where("market_id = ? AND status = ?", marketID, "Pending").Find(&orders)

	for _, order := range orders {
		shouldExecute := false
		currentPrice := 0
		if order.Outcome == "Yes" && yesPrice <= order.TargetPrice {
			shouldExecute = true
			currentPrice = yesPrice
		} else if order.Outcome == "No" && noPrice <= order.TargetPrice {
			shouldExecute = true
			currentPrice = noPrice
		}

		if shouldExecute {
			executeOrder(order, currentPrice)
		}
	}
}

func executeOrder(order models.LimitOrder, price int) {
	tx := config.DB.Begin()

	priceInPts := float64(price) / 100.0
	shares := order.Points / priceInPts
	trade := models.Trade{
		UserID:   order.UserID,
		MarketID: order.MarketID,
		Outcome:  order.Outcome,
		Points:   order.Points,
		Shares:   shares,
		Price:    price,
	}

	if err := tx.Create(&trade).Error; err != nil {
		tx.Rollback()
		return
	}

	order.Status = "Executed"
	tx.Save(&order)

	tx.Commit()

	// Notify via WebSocket
	BroadcastUpdate("limit_order_executed", gin.H{
		"user_id":  order.UserID,
		"order_id": order.ID,
		"message":  "Your limit order was executed!",
	})
}

// GetUserTrades returns portfolio for the logged-in user
func GetUserTrades(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var trades []models.Trade

	if err := config.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&trades).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch portfolio"})
		return
	}

	// Enrich with market titles
	type TradeWithMarket struct {
		models.Trade
		MarketTitle string `json:"market_title"`
	}

	result := make([]TradeWithMarket, 0, len(trades))
	for _, t := range trades {
		var market models.Market
		config.DB.Select("title").First(&market, t.MarketID)
		result = append(result, TradeWithMarket{Trade: t, MarketTitle: market.Title})
	}

	c.JSON(http.StatusOK, result)
}

func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// SellTrade handles a user cashing out a position before resolution
func SellTrade(c *gin.Context) {
	tradeID := c.Param("id")
	userID := c.MustGet("userID").(uint)

	var trade models.Trade
	if err := config.DB.First(&trade, tradeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	if trade.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not your trade"})
		return
	}

	if trade.Shares <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Already sold or no shares"})
		return
	}

	var market models.Market
	if err := config.DB.First(&market, trade.MarketID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus != "Open" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Market already resolved/closed"})
		return
	}

	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	// Calculate payout based on CURRENT market price
	currentPrice := market.YesPrice
	if trade.Outcome == "No" {
		currentPrice = market.NoPrice
	}

	payout := trade.Shares * (float64(currentPrice) / 100.0)

	tx := config.DB.Begin()

	// Update user points
	user.Points += int(payout)
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
		return
	}

	// Mark trade as sold (set shares to 0, record payout)
	trade.Shares = 0
	trade.Payout = int(payout)
	if err := tx.Save(&trade).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update trade"})
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message": "Shares sold successfully",
		"payout":  int(payout),
		"balance": user.Points,
	})
}
