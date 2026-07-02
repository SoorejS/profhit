package controllers

import (
	"fmt"
	"net/http"
	"time"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// GetAllMarkets fetches all open markets, optionally filtered by category
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

// CreateMarket allows an admin/content creator to publish a new prediction market
func CreateMarket(c *gin.Context) {
	var market models.Market
	if err := c.ShouldBindJSON(&market); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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

// ResolveMarket closes a market, declares the winning option, and pays out all
// correct predictors via the immutable CoinTransaction ledger.
// Admin/SuperAdmin only (enforced by route middleware).
func ResolveMarket(c *gin.Context) {
	id := c.Param("id")

	var market models.Market
	if err := config.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus != "Open" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Market is already resolved or not open"})
		return
	}

	var req struct {
		// Accept both 'winner' (internal API) and 'outcome' (frontend shorthand)
		Winner  string `json:"winner"`
		Outcome string `json:"outcome"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Normalise: prefer 'winner', fallback to 'outcome'
	correctOption := req.Winner
	if correctOption == "" {
		correctOption = req.Outcome
	}
	if correctOption == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'winner' or 'outcome' field is required"})
		return
	}

	// Validate that correct option is one of the market's actual options
	if !isValidOption(correctOption, market.Options) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid winner option — not listed for this market"})
		return
	}

	// Capture the resolving admin's ID
	adminIDVal, _ := c.Get("userID")
	adminID, _ := adminIDVal.(uint)

	// ── Load all predictions for this market ────────────────────────────────
	var predictions []models.PredictionSubmission
	if err := config.DB.Where("market_id = ?", market.ID).Find(&predictions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load predictions"})
		return
	}

	// ── Process everything inside ONE atomic transaction ─────────────────────
	// CRITICAL FIX: CreditCoinsTx is called with the same tx object, ensuring
	// that if any payout fails, ALL changes (predictions + coins + market) roll back.
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	winnerCount := 0
	loserCount := 0

	for i := range predictions {
		pred := &predictions[i]
		isCorrect := pred.Choice == correctOption
		pred.IsCorrect = &isCorrect

		if err := tx.Save(pred).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update prediction record"})
			return
		}

		if isCorrect {
			winnerCount++
			// CreditWalletTx runs INSIDE the same tx — rolls back if market save fails
			if err := services.CreditWalletTx(
				tx,
				pred.UserID,
				pred.Potential,
				models.TxTypePredictionWin,
				market.ID,
				"Won prediction on: "+market.Title,
				&adminID,
			); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Payout failed for user " + itoa(int(pred.UserID)) + ": " + err.Error(),
				})
				return
			}
		} else {
			loserCount++
		}
	}

	// ── Finalise the market ─────────────────────────────────────────────────
	now := time.Now()
	market.ResolutionStatus = "Resolved"
	market.CorrectOption = correctOption
	market.ResolvedAt = &now
	market.ResolvedByID = adminID

	if err := tx.Save(&market).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save resolved market"})
		return
	}

	_ = services.LogAction(tx, adminID, "RESOLVE_MARKET", fmt.Sprintf("market_%d", market.ID), "Resolved market with winner: "+correctOption, c.ClientIP())

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Market resolved successfully!",
		"market_id":    market.ID,
		"winner":       correctOption,
		"resolved_at":  now,
		"total_preds":  len(predictions),
		"winners_paid": winnerCount,
		"losers":       loserCount,
	})
}

// ProposeMarket allows a regular user to suggest a new market topic
func ProposeMarket(c *gin.Context) {
	var market models.Market
	if err := c.ShouldBindJSON(&market); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.MustGet("userID").(uint)
	market.CreatorID = userID
	market.ResolutionStatus = "Proposed"

	if err := config.DB.Create(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to propose market"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Market proposed successfully and is awaiting admin approval.",
		"market":  market,
	})
}

// ApproveMarket moves a proposed market to Open status (admin only)
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

	callerID := c.MustGet("userID").(uint)
	_ = services.LogAction(nil, callerID, "APPROVE_MARKET", fmt.Sprintf("market_%d", market.ID), "Approved proposed market: "+market.Title, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "Market approved and is now live!", "market": market})
}

// GetProposedMarkets returns all markets awaiting approval (admin only)
func GetProposedMarkets(c *gin.Context) {
	var markets []models.Market
	if err := config.DB.Where("resolution_status = ?", "Proposed").Find(&markets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch proposed markets"})
		return
	}
	c.JSON(http.StatusOK, markets)
}

// GetPortfolio returns the authenticated user's full prediction history,
// enriched with market title and resolution state via a single JOIN query.
func GetPortfolio(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	type PortfolioEntry struct {
		ID              uint       `json:"id"`
		MarketID        uint       `json:"market_id"`
		MarketTitle     string     `json:"market"`
		MarketStatus    string     `json:"market_status"`
		CorrectOption   string     `json:"correct_option"`
		Choice          string     `json:"choice"`
		PotentialPayout int        `json:"potential_payout"`
		IsCorrect       *bool      `json:"is_correct"`
		CreatedAt       time.Time  `json:"created_at"`
	}

	var portfolio []PortfolioEntry

	// Single JOIN query — eliminates N+1
	err := config.DB.Raw(`
		SELECT
			ps.id,
			ps.market_id,
			m.title         AS market_title,
			m.resolution_status AS market_status,
			m.correct_option,
			ps.choice,
			ps.potential    AS potential_payout,
			ps.is_correct,
			ps.created_at
		FROM prediction_submissions ps
		INNER JOIN markets m ON m.id = ps.market_id
		WHERE ps.user_id = ?
		  AND ps.deleted_at IS NULL
		  AND m.deleted_at IS NULL
		ORDER BY ps.created_at DESC
	`, userID).Scan(&portfolio).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load portfolio"})
		return
	}

	if portfolio == nil {
		portfolio = []PortfolioEntry{}
	}

	c.JSON(http.StatusOK, portfolio)
}

