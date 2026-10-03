package controllers

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strconv"
	"strings"
	"time"
)

func playableNewsQuery(now time.Time) *gorm.DB {
	return config.DB.Model(&models.Market{}).Where("visibility = ? AND is_demo = ? AND ((news_event_id IS NOT NULL AND news_published_at BETWEEN ? AND ?) OR (is_curated = ? AND (source_kind = 'official_event' OR news_published_at BETWEEN ? AND ?))) AND news_url <> '' AND news_discovered_at IS NOT NULL AND prediction_type <> '' AND resolution_rule <> '' AND resolution_source <> '' AND resolution_status = ? AND lock_time > ?", "Public", false, now.Add(-24*time.Hour), now, true, now.Add(-24*time.Hour), now, "Live", now).Where("start_time IS NULL OR start_time <= ?", now)
}
func LiveNewsFeed(c *gin.Context) {
	now := time.Now().UTC()
	query := playableNewsQuery(now)
	category := c.Query("category")
	if category != "" {
		if _, ok := services.CategoryPayouts[category]; !ok {
			c.JSON(400, gin.H{"error": "Invalid category"})
			return
		}
		query = query.Where("category = ?", category)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		if len(search) > 200 {
			c.JSON(400, gin.H{"error": "Search is too long"})
			return
		}
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(news_event_title) LIKE ? OR LOWER(category) LIKE ? OR LOWER(news_source_name) LIKE ?", pattern, pattern, pattern, pattern)
	}
	section := c.DefaultQuery("section", "latest")
	order := "news_published_at desc, id desc"
	switch section {
	case "breaking":
		query = query.Where("news_published_at >= ?", now.Add(-time.Hour))
	case "latest", "all":
	case "trending":
		order = "volume desc, news_published_at desc, id desc"
	case "closing_soon":
		query = query.Where("lock_time <= ?", now.Add(6*time.Hour))
		order = "lock_time asc, id asc"
	case "for_you":
		userID, ok := c.Get("userID")
		if !ok {
			c.JSON(401, gin.H{"error": "Sign in to personalize your feed"})
			return
		}
		var user models.User
		if config.DB.First(&user, userID).Error != nil {
			c.JSON(401, gin.H{"error": "Session unavailable"})
			return
		}
		interests := []string{}
		for cat := range services.CategoryPayouts {
			if strings.Contains(strings.ToLower(user.Interests), strings.ToLower(cat)) {
				interests = append(interests, cat)
			}
		}
		if len(interests) > 0 {
			query = query.Where("category IN ?", interests)
		}
	default:
		c.JSON(400, gin.H{"error": "Invalid feed section"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "60"))
	if limit < 1 || limit > 100 {
		limit = 60
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 || offset > 10000 {
		offset = 0
	}
	var total, playable, events int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"error": "Could not count live predictions"})
		return
	}
	if playableNewsQuery(now).Count(&playable).Error != nil || config.DB.Model(&models.NewsEvent{}).Where("published_at BETWEEN ? AND ?", now.Add(-24*time.Hour), now).Count(&events).Error != nil {
		c.JSON(500, gin.H{"error": "Could not count current events"})
		return
	}
	items := []models.Market{}
	if query.Order(order).Offset(offset).Limit(limit).Find(&items).Error != nil {
		c.JSON(500, gin.H{"error": "Could not fetch live predictions"})
		return
	}
	var state models.NewsIngestionState
	err := config.DB.First(&state, 1).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(500, gin.H{"error": "Provider status unavailable"})
		return
	}
	status := state.Status
	if status == "" {
		status = "Not configured"
	}
	if status == "Healthy" && (state.LastSuccessAt == nil || now.Sub(*state.LastSuccessAt) > 2*services.NewsRefreshInterval()) {
		status = "Stale"
	}
	c.JSON(200, gin.H{"items": items, "total": total, "playable_count": playable, "current_event_count": events, "target": 50, "target_met": playable >= 50, "offset": offset, "limit": limit, "as_of": now, "provider": gin.H{"status": status, "last_success_at": state.LastSuccessAt, "next_fetch_at": state.NextFetchAt}, "mode": "LIVE DATA", "currency": "virtual coins", "paid_entry": false})
}
func AdminNewsStatus(c *gin.Context) {
	var state models.NewsIngestionState
	err := config.DB.First(&state, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		state.Status = "Not configured"
	} else if err != nil {
		c.JSON(500, gin.H{"error": "News status unavailable"})
		return
	}
	counts := map[string]int64{}
	for _, status := range []string{"Draft", "Proposed", "Live", "Resolved"} {
		var count int64
		if config.DB.Model(&models.Market{}).Where("news_event_id IS NOT NULL AND resolution_status = ?", status).Count(&count).Error != nil {
			c.JSON(500, gin.H{"error": "News market counts unavailable"})
			return
		}
		counts[status] = count
	}
	var events []models.NewsEvent
	if config.DB.Preload("Sources").Order("published_at desc").Limit(100).Find(&events).Error != nil {
		c.JSON(500, gin.H{"error": "News events unavailable"})
		return
	}
	c.JSON(200, gin.H{"ingestion": state, "markets": counts, "events": events, "generation": "Editorial structured candidates; automatic publication disabled", "resolution": "Approved evidence through the existing settlement endpoint"})
}
func RefreshNews(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	err := services.RefreshLiveNews(ctx, services.ConfiguredNewsProviders(), time.Now().UTC())
	if err != nil {
		c.JSON(503, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Refresh checked. Database schedule and provider quota remain enforced."})
}

func GenerateNewsQuestion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid event ID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 35*time.Second)
	defer cancel()
	if err := services.GenerateEventPrediction(ctx, uint(id), services.ConfiguredQuestionGenerator(), time.Now().UTC()); err != nil {
		c.JSON(422, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Validated question saved to existing draft queue. Editorial approval is required."})
}
