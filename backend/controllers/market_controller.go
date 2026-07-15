package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// GetAllMarkets fetches markets with discovery and lifecycle filtering
func GetAllMarkets(c *gin.Context) {
	var markets []models.Market
	category := c.Query("category")
	status := c.Query("status")
	sort := c.Query("sort")

	// By default, only show Public markets to users. We assume Admin uses a different endpoint or passes a flag if needed.
	// But let's allow all if admin, else Public. To keep it simple, just filter Public unless status is explicitly Draft.
	query := config.DB.Where("visibility = ?", "Public")

	if status != "" {
		query = query.Where("resolution_status = ?", status)
	} else {
		// Default to active-like statuses for general browsing
		query = query.Where("resolution_status IN ?", []string{"Scheduled", "Live", "Locked", "Awaiting Resolution"})
	}

	if category != "" {
		query = query.Where("category = ?", category)
	}

	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")
	limit := 50
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	// Sorting logic
	orderClause := "created_at desc"
	if sort == "trending" {
		orderClause = "volume desc"
	} else if sort == "newest" {
		orderClause = "created_at desc"
	} else if sort == "ending_soon" {
		orderClause = "lock_time asc"
		query = query.Where("lock_time > ?", time.Now())
	}

	if err := query.Order(orderClause).Limit(limit).Offset(offset).Find(&markets).Error; err != nil {
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

	// ── PDF §3.1 / §4.1: Validate difficulty enum ─────────────────────────────
	validDifficulties := map[string]bool{"Easy": true, "Medium": true, "Hard": true}
	if market.Difficulty != "" && !validDifficulties[market.Difficulty] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid difficulty. Must be one of: Easy, Medium, Hard",
		})
		return
	}

	// ── PDF §4.1: Enforce difficulty-based payout bounds ─────────────────────
	// Easy: 20–40 coins | Medium: 50–100 coins | Hard: 120–400 coins
	if market.Payout > 0 {
		type payoutRange struct{ min, max int }
		payoutBounds := map[string]payoutRange{
			"Easy":   {min: 20, max: 40},
			"Medium": {min: 50, max: 100},
			"Hard":   {min: 120, max: 400},
		}
		if bounds, ok := payoutBounds[market.Difficulty]; ok {
			if market.Payout < bounds.min || market.Payout > bounds.max {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf(
						"Payout %d is outside the allowed range for %s difficulty (%d–%d coins)",
						market.Payout, market.Difficulty, bounds.min, bounds.max,
					),
				})
				return
			}
		}
	}

	if market.ResolutionStatus == "" {
		market.ResolutionStatus = "Draft" // Draft, Scheduled, Live
	}

	// Automatically calculate legacy EndDate based on LockTime if missing
	if market.EndDate.IsZero() && market.LockTime != nil {
		market.EndDate = *market.LockTime
	}

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

	// Allow resolution from Locked or Awaiting Resolution states.
	// "Open" was a legacy value that never existed in the real lifecycle.
	resolvableStatuses := map[string]bool{"Locked": true, "Awaiting Resolution": true, "Live": true}
	if !resolvableStatuses[market.ResolutionStatus] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Market must be Locked or Awaiting Resolution before it can be resolved. Current status: " + market.ResolutionStatus})
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

	// Recalculate WinRate and TotalPredictions for all users involved in this market
	if len(predictions) > 0 {
		var userIDs []uint
		for _, p := range predictions {
			userIDs = append(userIDs, p.UserID)
		}

		if err := tx.Exec(`
			UPDATE users
			SET total_predictions = (
				SELECT COUNT(id) FROM prediction_submissions WHERE user_id = users.id AND deleted_at IS NULL
			),
			win_rate = COALESCE((
				SELECT (SUM(CASE WHEN is_correct = true THEN 1 ELSE 0 END) * 100.0) / NULLIF(COUNT(id), 0)
				FROM prediction_submissions 
				WHERE user_id = users.id AND deleted_at IS NULL
			), 0)
			WHERE id IN ?
		`, userIDs).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user stats"})
			return
		}
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
		ID              uint      `json:"id"`
		MarketID        uint      `json:"market_id"`
		MarketTitle     string    `json:"market"`
		MarketStatus    string    `json:"market_status"`
		CorrectOption   string    `json:"correct_option"`
		Choice          string    `json:"choice"`
		PotentialPayout int       `json:"potential_payout"`
		IsCorrect       *bool     `json:"is_correct"`
		CreatedAt       time.Time `json:"created_at"`
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

// TransitionMarketState allows admins to manually move market through its lifecycle
func TransitionMarketState(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"` // Draft, Scheduled, Live, Locked, Awaiting Resolution, Archived
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validStatuses := map[string]bool{
		"Draft": true, "Scheduled": true, "Live": true, "Locked": true, "Awaiting Resolution": true, "Archived": true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	var market models.Market
	if err := config.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	market.ResolutionStatus = req.Status
	if err := config.DB.Save(&market).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transition market state"})
		return
	}

	// Trigger WebSocket notification for certain transitions
	if req.Status == "Live" {
		services.BroadcastToAll("market_live", fmt.Sprintf("Market '%s' is now LIVE!", market.Title))
	} else if req.Status == "Locked" {
		services.BroadcastToAll("market_locked", fmt.Sprintf("Market '%s' is now LOCKED. No more predictions accepted.", market.Title))
	}

	c.JSON(http.StatusOK, market)
}

// DeleteMarket allows an admin to archive or hard delete a market (Drafts only).
func DeleteMarket(c *gin.Context) {
	id := c.Param("id")

	var market models.Market
	if err := config.DB.First(&market, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus == "Draft" {
		config.DB.Unscoped().Delete(&market)
		c.JSON(http.StatusOK, gin.H{"message": "Draft market deleted successfully"})
	} else {
		// Just archive
		market.ResolutionStatus = "Archived"
		config.DB.Save(&market)
		c.JSON(http.StatusOK, gin.H{"message": "Market archived successfully"})
	}
}
