package controllers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
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
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		if len(search) > 200 {
			c.JSON(400, gin.H{"error": "Search is too long"})
			return
		}
		term := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(news_event_title) LIKE ? OR LOWER(category) LIKE ? OR LOWER(news_source_name) LIKE ?", term, term, term, term)
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
		query = query.Where("lock_time > ?", time.Now().UTC())
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

	isAudit := market.IsDemo
	sourceURL, sourceName, published, sourceKind := market.NewsURL, market.NewsSourceName, market.NewsPublishedAt, market.SourceKind
	if sourceKind == "" {
		sourceKind = "article"
	}
	if sourceURL != "" && (!services.PublicProviderURL(sourceURL) || (published != nil && published.After(time.Now().UTC())) || len(sourceName) == 0 || len(sourceName) > 100 || (sourceKind != "article" && sourceKind != "official_event")) {
		c.JSON(400, gin.H{"error": "Curated events need a public HTTPS source, source name and actual publication date"})
		return
	}
	if err := validateNewMarket(&market); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	market.CreatorID = c.MustGet("userID").(uint)
	// Only an authenticated operator of the virtual demo can exclude an audit
	// market from the genuine-opportunity count. Proposals cannot set this flag.
	market.IsDemo = isAudit && os.Getenv("VIRTUAL_COIN_DEMO") == "true"
	if sourceURL != "" {
		if (sourceKind == "article" && (published == nil || published.Before(time.Now().UTC().Add(-24*time.Hour)))) || (sourceKind == "official_event" && !services.ApprovedResultURL(market.Category, sourceURL)) {
			c.JSON(400, gin.H{"error": "Articles need actual publication dates; official events need an approved official source"})
			return
		}
		now := time.Now().UTC()
		market.IsCurated = true
		market.SourceKind = sourceKind
		market.NewsURL, market.NewsSourceName, market.NewsPublishedAt, market.NewsDiscoveredAt = sourceURL, sourceName, published, &now
	}
	if !createUniqueMarket(c, &market) {
		return
	}

	c.JSON(http.StatusCreated, market)
}

// GetMarketByID fetches a single market with full detail
func GetMarketByID(c *gin.Context) {
	id := c.Param("id")
	var market models.Market

	if err := config.DB.Where("id = ? AND visibility = ? AND resolution_status NOT IN ?", id, "Public", []string{"Draft", "Proposed"}).First(&market).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Market not found"})
		return
	}

	c.JSON(http.StatusOK, market)
}

// ResolveMarket closes a market, declares the winning option, and pays out all
// correct predictors via the immutable CoinTransaction ledger.
// Admin/SuperAdmin only (enforced by route middleware).
func ResolveMarket(c *gin.Context) {
	var input services.ResolutionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	input.AdminID = c.MustGet("userID").(uint)
	input.ClientIP = c.ClientIP()
	result, err := services.SettleMarket(c.Param("id"), input)
	if err != nil {
		status := 500
		if typed, ok := err.(*services.SettlementError); ok {
			status = typed.Status
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, result)
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
	if !createUniqueMarket(c, &market) {
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

	if market.ResolutionStatus != "Proposed" && market.ResolutionStatus != "Draft" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only unpublished markets can be approved"})
		return
	}

	if market.LockTime == nil || !market.LockTime.After(time.Now().UTC()) {
		c.JSON(400, gin.H{"error": "Proposal lock time has passed"})
		return
	}
	checked := market
	if market.NewsEventID != nil {
		var event models.NewsEvent
		if config.DB.First(&event, *market.NewsEventID).Error != nil || services.ValidateNewsPrediction(event, &checked, time.Now().UTC()) != nil {
			c.JSON(400, gin.H{"error": "News event is stale or rules need source-linked editorial review"})
			return
		}
	}
	if services.ConfigurePrediction(&checked) != nil || checked.Payout != market.Payout || market.PredictionType == "" {
		c.JSON(400, gin.H{"error": "Review the typed rules and approved result source before publishing"})
		return
	}
	previous := market.ResolutionStatus
	market.ResolutionStatus = "Live"
	if market.StartTime != nil && market.StartTime.After(time.Now().UTC()) {
		market.ResolutionStatus = "Scheduled"
	}
	result := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status = ?", market.ID, previous).Update("resolution_status", market.ResolutionStatus)
	if result.Error != nil || result.RowsAffected != 1 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve market"})
		return
	}

	callerID := c.MustGet("userID").(uint)
	_ = services.LogAction(nil, callerID, "APPROVE_MARKET", fmt.Sprintf("market_%d", market.ID), "Approved proposed market: "+market.Title, c.ClientIP())
	services.BroadcastToAll("market_live", gin.H{"market_id": market.ID})

	c.JSON(http.StatusOK, gin.H{"message": "Market published", "market": market})
}

// GetProposedMarkets returns all markets awaiting approval (admin only)
func GetProposedMarkets(c *gin.Context) {
	var markets []models.Market
	if err := config.DB.Where("resolution_status IN ?", []string{"Draft", "Proposed"}).Find(&markets).Error; err != nil {
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
	if (req.Status == "Scheduled" || req.Status == "Live") && (market.LockTime == nil || !market.LockTime.After(time.Now().UTC())) {
		c.JSON(400, gin.H{"error": "Market requires a future lock time"})
		return
	}
	if req.Status == "Scheduled" || req.Status == "Live" {
		checked := market
		if services.ConfigurePrediction(&checked) != nil || checked.Payout != market.Payout || market.PredictionType == "" {
			c.JSON(400, gin.H{"error": "Review typed rules before publishing"})
			return
		}
	}
	if req.Status == "Scheduled" && (market.StartTime == nil || !market.StartTime.After(time.Now().UTC())) {
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
	categories := map[string]string{"weather": "Weather", "sports": "Sports", "politics": "Politics", "entertainment": "Entertainment", "markets": "Financial Markets", "financial markets": "Financial Markets", "finance": "Financial Markets", "wild card": "Wild Card", "technology": "Technology", "geopolitics": "Geopolitics"}
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
	if m.LockTime == nil || !m.LockTime.After(time.Now().UTC()) {
		return fmt.Errorf("A future lock time is required")
	}
	if m.StartTime != nil && !m.StartTime.Before(*m.LockTime) {
		return fmt.Errorf("Start time must precede lock time")
	}
	if m.ResolutionTime != nil && m.ResolutionTime.Before(*m.LockTime) {
		return fmt.Errorf("Resolution time must follow lock time")
	}
	lockUTC := m.LockTime.UTC()
	m.LockTime = &lockUTC
	if m.StartTime != nil {
		v := m.StartTime.UTC()
		m.StartTime = &v
	}
	if m.ResolutionTime != nil {
		v := m.ResolutionTime.UTC()
		m.ResolutionTime = &v
	}
	m.EndDate = *m.LockTime
	if m.ResolutionStatus == "" {
		m.ResolutionStatus = "Draft"
	}
	if m.ResolutionStatus != "Draft" && m.ResolutionStatus != "Scheduled" && m.ResolutionStatus != "Live" {
		return fmt.Errorf("Invalid initial status")
	}
	if m.ResolutionStatus == "Scheduled" && (m.StartTime == nil || !m.StartTime.After(time.Now().UTC())) {
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
	m.WeeklyChallengeID = nil
	m.NewsEventID = nil
	m.NewsURL = ""
	m.NewsSourceName = ""
	m.NewsEventTitle = ""
	m.NewsPublishedAt = nil
	m.NewsDiscoveredAt = nil
	m.IsDemo = false
	m.IsCurated = false
	m.SourceKind = "article"
	m.ResultSpec = ""
	m.ResultApprovedBy = 0
	m.ResultEvidence = ""
	m.ResolutionFailure = ""
	m.NextResolutionAttemptAt = nil
	return services.ConfigurePrediction(m)
}

func createUniqueMarket(c *gin.Context, market *models.Market) bool {
	err := services.CreateUniqueMarket(config.DB, market)
	if duplicate, ok := err.(*services.DuplicateMarketError); ok {
		visible := duplicate.Existing.Visibility == "Public" && duplicate.Existing.ResolutionStatus != "Draft" && duplicate.Existing.ResolutionStatus != "Proposed"
		c.JSON(409, gin.H{"error": duplicate.Error(), "existing_market_id": duplicate.Existing.ID, "existing_title": duplicate.Existing.Title, "existing_can_view": visible})
		return false
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not create or check prediction"})
		return false
	}
	return true
}
