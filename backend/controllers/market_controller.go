package controllers

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm/clause"
	"math"
	"net/http"
	"strconv"
	"strings"
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
	query := config.DB.Where("visibility = ? AND resolution_status NOT IN ?", "Public", []string{"Draft", "Proposed"})

	if status != "" {
		query = query.Where("resolution_status = ?", status)
	} else {
		// Default to active-like statuses for general browsing
		query = query.Where("resolution_status IN ?", []string{"Open", "Scheduled", "Live", "Locked", "Awaiting Resolution"})
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

	if err := validateNewMarket(&market); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	market.CreatorID = c.MustGet("userID").(uint)

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

	if err := config.DB.Where("id = ? AND resolution_status NOT IN ?", id, []string{"Draft", "Proposed"}).First(&market).Error; err != nil {
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

	tx := config.DB.Begin()
	defer tx.Rollback()
	var market models.Market
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	// Allow resolution from Locked or Awaiting Resolution states.
	// "Open" was a legacy value that never existed in the real lifecycle.
	resolvableStatuses := map[string]bool{"Locked": true, "Awaiting Resolution": true}
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
	if err := tx.Where("market_id = ?", market.ID).Order("user_id ASC").Find(&predictions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load predictions"})
		return
	}

	// ── Process everything inside ONE atomic transaction ─────────────────────
	// CRITICAL FIX: CreditCoinsTx is called with the same tx object, ensuring
	// that if any payout fails, ALL changes (predictions + coins + market) roll back.

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
					"error": "Payout failed; no changes were committed",
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
				SELECT (SUM(CASE WHEN is_correct = true THEN 1 ELSE 0 END) * 100.0) / NULLIF(COUNT(is_correct), 0)
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

	if err := validateNewMarket(&market); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
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

	if err := config.DB.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus != "Proposed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only proposed markets can be approved"})
		return
	}

	if market.LockTime == nil || !market.LockTime.After(time.Now()) {
		c.JSON(400, gin.H{"error": "Proposal lock time has passed"})
		return
	}
	market.ResolutionStatus = "Live"
	result := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status = ?", market.ID, "Proposed").Update("resolution_status", "Live")
	if result.Error != nil || result.RowsAffected != 1 {
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("limit", "20")))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

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

	var total int64
	countErr := config.DB.Raw(`
		SELECT COUNT(*)
		FROM prediction_submissions ps
		INNER JOIN markets m ON m.id = ps.market_id
		WHERE ps.user_id = ?
		  AND ps.deleted_at IS NULL
		  AND m.deleted_at IS NULL
	`, userID).Scan(&total).Error

	if countErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count portfolio items"})
		return
	}

	var portfolio []PortfolioEntry

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
		ORDER BY ps.created_at DESC, ps.id DESC
		LIMIT ? OFFSET ?
	`, userID, pageSize, offset).Scan(&portfolio).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load portfolio"})
		return
	}

	if portfolio == nil {
		portfolio = []PortfolioEntry{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	c.JSON(http.StatusOK, gin.H{
		"items":       portfolio,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
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
	if err := config.DB.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	allowed := map[string][]string{"Draft": {"Scheduled", "Live"}, "Scheduled": {"Live"}, "Open": {"Locked"}, "Live": {"Locked"}, "Locked": {"Awaiting Resolution"}, "Resolved": {"Archived"}}
	legal := false
	for _, state := range allowed[market.ResolutionStatus] {
		if state == req.Status {
			legal = true
		}
	}
	if !legal {
		c.JSON(400, gin.H{"error": "Illegal market state transition"})
		return
	}
	if (req.Status == "Scheduled" || req.Status == "Live") && (market.LockTime == nil || !market.LockTime.After(time.Now())) {
		c.JSON(400, gin.H{"error": "Market requires a future lock time"})
		return
	}
	if req.Status == "Scheduled" && (market.StartTime == nil || !market.StartTime.After(time.Now())) {
		c.JSON(400, gin.H{"error": "Scheduled market requires a future start time"})
		return
	}
	result := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status = ?", market.ID, market.ResolutionStatus).Update("resolution_status", req.Status)
	market.ResolutionStatus = req.Status
	if result.Error != nil || result.RowsAffected != 1 {
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
	if err := config.DB.Where("id = ?", id).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	if market.ResolutionStatus != "Draft" && market.ResolutionStatus != "Resolved" {
		c.JSON(400, gin.H{"error": "Only drafts and resolved markets may be archived"})
		return
	}
	result := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status = ?", market.ID, market.ResolutionStatus).Update("resolution_status", "Archived")
	if result.Error != nil || result.RowsAffected != 1 {
		c.JSON(409, gin.H{"error": "Market changed; refresh and retry"})
		return
	}
	c.JSON(200, gin.H{"message": "Market archived"})
}
func validateNewMarket(m *models.Market) error {
	m.Title = strings.TrimSpace(m.Title)
	if len(m.Title) < 5 || len(m.Title) > 200 || len(m.Description) > 5000 {
		return fmt.Errorf("Title must be 5-200 characters and description at most 5000 characters")
	}
	categories := map[string]string{"weather": "Weather", "sports": "Sports", "politics": "Politics", "entertainment": "Entertainment", "markets": "Financial Markets", "financial markets": "Financial Markets", "finance": "Financial Markets", "wild card": "Wild Card"}
	category, ok := categories[strings.ToLower(strings.TrimSpace(m.Category))]
	if !ok {
		return fmt.Errorf("Invalid category")
	}
	m.Category = category
	if m.Difficulty == "" {
		m.Difficulty = "Medium"
	}
	bounds := map[string][2]int{"Easy": {20, 40}, "Medium": {50, 100}, "Hard": {120, 400}}
	bound, ok := bounds[m.Difficulty]
	if !ok {
		return fmt.Errorf("Invalid difficulty")
	}
	if m.Payout == 0 {
		m.Payout = bound[0]
	}
	if m.Payout < bound[0] || m.Payout > bound[1] {
		return fmt.Errorf("Payout outside difficulty bounds")
	}
	var options []string
	if m.Options == "" {
		m.Options = `["Yes","No"]`
	}
	if json.Unmarshal([]byte(m.Options), &options) != nil || len(options) < 2 || len(options) > 10 {
		return fmt.Errorf("Provide 2-10 options")
	}
	seen := map[string]bool{}
	for _, option := range options {
		if strings.TrimSpace(option) == "" || len(option) > 100 || seen[option] {
			return fmt.Errorf("Options must be nonempty and unique")
		}
		seen[option] = true
	}
	if m.LockTime == nil && !m.EndDate.IsZero() {
		m.LockTime = &m.EndDate
	}
	if m.LockTime == nil || !m.LockTime.After(time.Now()) {
		return fmt.Errorf("A future lock time is required")
	}
	if m.StartTime != nil && !m.StartTime.Before(*m.LockTime) {
		return fmt.Errorf("Start time must precede lock time")
	}
	if m.ResolutionTime != nil && m.ResolutionTime.Before(*m.LockTime) {
		return fmt.Errorf("Resolution time must follow lock time")
	}
	m.EndDate = *m.LockTime
	if m.ResolutionStatus == "" {
		m.ResolutionStatus = "Draft"
	}
	if m.ResolutionStatus != "Draft" && m.ResolutionStatus != "Scheduled" && m.ResolutionStatus != "Live" {
		return fmt.Errorf("Invalid initial status")
	}
	if m.ResolutionStatus == "Scheduled" && (m.StartTime == nil || !m.StartTime.After(time.Now())) {
		return fmt.Errorf("Scheduled markets need a future start time")
	}
	if m.Visibility == "" {
		m.Visibility = "Public"
	}
	if m.Visibility != "Public" && m.Visibility != "Unlisted" {
		return fmt.Errorf("Invalid visibility")
	}
	m.DailyKey = nil
	m.ID = 0
	m.Volume = 0
	m.CorrectOption = ""
	m.ResolvedAt = nil
	m.ResolvedByID = 0
	m.CoinReward = 0
	m.CreatedAt = time.Time{}
	m.UpdatedAt = time.Time{}
	m.DeletedAt.Valid = false
	return nil
}
