package controllers

import (
	"math"
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

	// --- CPMM AMM Math ---
	// Formula: k = LiquidityYes * LiquidityNo
	// When buying YES, the pool gains P points. It mints P YES and P NO shares.
	// The pool keeps the P NO shares, increasing LiquidityNo.
	// We calculate new LiquidityYes to maintain k, and give the excess YES shares to the user.
	k := market.LiquidityYes * market.LiquidityNo
	var shares float64

	if req.Outcome == "Yes" {
		newLiquidityNo := market.LiquidityNo + req.Points
		newLiquidityYes := k / newLiquidityNo
		shares = (market.LiquidityYes + req.Points) - newLiquidityYes

		market.LiquidityYes = newLiquidityYes
		market.LiquidityNo = newLiquidityNo
	} else {
		newLiquidityYes := market.LiquidityYes + req.Points
		newLiquidityNo := k / newLiquidityYes
		shares = (market.LiquidityNo + req.Points) - newLiquidityNo

		market.LiquidityYes = newLiquidityYes
		market.LiquidityNo = newLiquidityNo
	}

	// Calculate exact average price paid per share
	pricePaidPerShare := (req.Points / shares) * 100.0

	// Update the marginal display prices (PriceYes = PoolNo / TotalPool)
	totalLiquidity := market.LiquidityYes + market.LiquidityNo
	market.YesPrice = int((market.LiquidityNo / totalLiquidity) * 100)
	market.NoPrice = int((market.LiquidityYes / totalLiquidity) * 100)
	market.Volume += req.Points

	// Clamp visually for UI cleanliness
	market.YesPrice = clamp(market.YesPrice, 1, 99)
	market.NoPrice = clamp(market.NoPrice, 1, 99)

	trade := models.Trade{
		UserID:   user.ID,
		MarketID: market.ID,
		Outcome:  req.Outcome,
		Points:   req.Points,
		Shares:   shares,
		Price:    int(pricePaidPerShare),
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

	// --- CPMM Sell Math (Quadratic Equation for exact slippage) ---
	// Equation: P = ((Y+N+S) - sqrt((Y+N+S)^2 - 4*S*Other)) / 2
	S := trade.Shares
	Y := market.LiquidityYes
	N := market.LiquidityNo
	var payoutFloat float64
	
	if trade.Outcome == "Yes" {
		Other := N
		sum := Y + N + S
		payoutFloat = (sum - math.Sqrt(sum*sum - 4*S*Other)) / 2.0
		
		market.LiquidityYes += (S - payoutFloat)
		market.LiquidityNo -= payoutFloat
	} else {
		Other := Y
		sum := Y + N + S
		payoutFloat = (sum - math.Sqrt(sum*sum - 4*S*Other)) / 2.0
		
		market.LiquidityNo += (S - payoutFloat)
		market.LiquidityYes -= payoutFloat
	}

	payout := int(payoutFloat)

	// Update marginal display prices
	totalLiquidity := market.LiquidityYes + market.LiquidityNo
	market.YesPrice = int((market.LiquidityNo / totalLiquidity) * 100)
	market.NoPrice = int((market.LiquidityYes / totalLiquidity) * 100)
	market.Volume += float64(payout) // Treat selling as active volume

	market.YesPrice = clamp(market.YesPrice, 1, 99)
	market.NoPrice = clamp(market.NoPrice, 1, 99)

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

	// Update the market AMM state
	if err := tx.Save(&market).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update market AMM pool"})
		return
	}

	tx.Commit()

	// Broadcast the update to all connected clients so charts update in real-time
	BroadcastUpdate("trade_placed", gin.H{
		"market_id": market.ID,
		"yes_price": market.YesPrice,
		"no_price":  market.NoPrice,
		"volume":    market.Volume,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Shares sold successfully",
		"payout":  int(payout),
		"balance": user.Points,
	})
}
