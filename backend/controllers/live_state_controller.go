package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"profhit-backend/config"
	"profhit-backend/models"
)

// LiveState is a database-backed change signal, shared across function instances.
// No business data is manufactured and private versions require app auth.
func LiveState(c *gin.Context) {
	var markets []struct {
		Status  string
		Count   int64
		Volume  int64
		Updated string
	}
	err := config.DB.Model(&models.Market{}).Select("resolution_status AS status, COUNT(*) AS count, COALESCE(SUM(volume),0) AS volume, CAST(MAX(updated_at) AS TEXT) AS updated").Where("visibility = ? AND resolution_status NOT IN ?", "Public", []string{"Draft", "Proposed"}).Group("resolution_status").Order("resolution_status").Scan(&markets).Error
	if err != nil {
		c.JSON(500, gin.H{"error": "Live state unavailable"})
		return
	}
	versions := gin.H{"public": stateDigest(markets)}
	lifecycle := make([]interface{}, 0, len(markets))
	for _, market := range markets {
		lifecycle = append(lifecycle, []interface{}{market.Status, market.Count, market.Updated})
	}
	versions["lifecycle"] = stateDigest(lifecycle)
	if id, ok := c.Get("userID"); ok {
		var ledgerID, notificationID int64
		var readCount int64
		if config.DB.Model(&models.WalletLedger{}).Where("user_id = ?", id).Select("COALESCE(MAX(id),0)").Scan(&ledgerID).Error != nil || config.DB.Model(&models.Notification{}).Where("user_id = ?", id).Select("COALESCE(MAX(id),0)").Scan(&notificationID).Error != nil || config.DB.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NOT NULL", id).Count(&readCount).Error != nil {
			c.JSON(500, gin.H{"error": "Account live state unavailable"})
			return
		}
		versions["wallet"] = ledgerID
		versions["notifications"] = stateDigest([]int64{notificationID, readCount})
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, versions)
}
func stateDigest(value interface{}) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
