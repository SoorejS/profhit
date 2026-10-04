package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strconv"
	"strings"
	"time"
)

func AdminMarkets(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}
	items := []models.Market{}
	query := config.DB.Where("resolution_status NOT IN ?", []string{"Draft", "Proposed"})
	if status := c.Query("status"); status != "" {
		query = config.DB.Where("resolution_status = ?", status)
	}
	if query.Order("updated_at desc, id desc").Limit(100).Offset(offset).Find(&items).Error != nil {
		c.JSON(500, gin.H{"error": "Markets unavailable"})
		return
	}
	c.JSON(200, items)
}
func PreviewMarket(c *gin.Context) {
	var m models.Market
	if config.DB.Where("id = ?", c.Param("id")).First(&m).Error != nil {
		c.JSON(404, gin.H{"error": "Market not found"})
		return
	}
	c.JSON(200, m)
}
func FeatureMarket(c *gin.Context) {
	var req struct {
		Featured bool `json:"featured"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "Invalid feature setting"})
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var m models.Market
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", c.Param("id")).First(&m).Error; err != nil {
			return err
		}
		if req.Featured && (m.EditorialReviewedBy == 0 || !m.IsCurated && m.NewsEventID == nil || m.IsDemo || m.ResolutionStatus != "Live" || m.LockTime == nil || !m.LockTime.After(time.Now().UTC())) {
			return gorm.ErrInvalidData
		}
		if err := tx.Model(&m).Update("is_featured", req.Featured).Error; err != nil {
			return err
		}
		return services.LogAction(tx, c.MustGet("userID").(uint), "FEATURE_MARKET", fmt.Sprint(m.ID), fmt.Sprintf("Featured: %t", req.Featured), c.ClientIP())
	})
	if err != nil {
		c.JSON(400, gin.H{"error": "Only reviewed open genuine predictions can be featured"})
		return
	}
	services.BroadcastToAll("market_state_changed", gin.H{"market_id": c.Param("id")})
	c.JSON(200, gin.H{"message": "Feature setting saved"})
}
func VoidMarket(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || len(strings.TrimSpace(req.Reason)) < 10 || len(req.Reason) > 2000 {
		c.JSON(400, gin.H{"error": "Explain why this market must be cancelled (10–2000 characters)"})
		return
	}
	users, err := services.VoidMarket(c.Param("id"), req.Reason, c.MustGet("userID").(uint), c.ClientIP())
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	for _, id := range users {
		services.BroadcastToUser(id, "wallet_updated", gin.H{"reason": "Entry refunded"})
		services.BroadcastToUser(id, "notification_created", nil)
	}
	services.BroadcastToAll("market_state_changed", gin.H{"market_id": c.Param("id")})
	c.JSON(200, gin.H{"message": "Market cancelled; entry Coins refunded", "refunded_players": len(users)})
}
