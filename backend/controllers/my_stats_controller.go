package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"profhit-backend/config"
	"profhit-backend/models"
)

// GetMyStats uses the authenticated identity and includes the full history.
func GetMyStats(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	var stats struct {
		Total            int64
		Settled          int64
		Won              int64
		PendingPotential int64
	}
	err := config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ?", userID).
		Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN is_correct IS NOT NULL THEN 1 ELSE 0 END),0) AS settled, COALESCE(SUM(CASE WHEN is_correct = ? THEN 1 ELSE 0 END),0) AS won, COALESCE(SUM(CASE WHEN is_correct IS NULL THEN potential ELSE 0 END),0) AS pending_potential", true).Scan(&stats).Error
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not load prediction summary"})
		return
	}
	rate := "—"
	if stats.Settled > 0 {
		rate = fmt.Sprintf("%.1f%%", 100*float64(stats.Won)/float64(stats.Settled))
	}
	c.JSON(200, gin.H{"total_predictions": stats.Total, "settled_predictions": stats.Settled, "won_predictions": stats.Won, "win_rate": rate, "pending_potential": stats.PendingPotential})
}
