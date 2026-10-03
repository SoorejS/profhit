package services

import (
	"gorm.io/gorm"
	"profhit-backend/models"
	"time"
)

// A legacy boolean or document-only verification is insufficient for redemption.
// Contact attestations must come from a verified provider adapter, never client flags.
func RedemptionVerified(tx *gorm.DB, userID uint, now time.Time) bool {
	var k models.HyperVergeKYC
	if tx.Where("user_id = ?", userID).Order("created_at desc, id desc").First(&k).Error != nil {
		return false
	}
	return k.Status == "Verified" && k.VerifiedAt != nil && !k.VerifiedAt.After(now) && k.VerifiedAt.AddDate(1, 0, 0).After(now) && k.PhoneVerifiedAt != nil && k.EmailConfirmedAt != nil && !k.PhoneVerifiedAt.After(now) && !k.EmailConfirmedAt.After(now) && k.PhoneVerifiedAt.AddDate(1, 0, 0).After(now) && k.EmailConfirmedAt.AddDate(1, 0, 0).After(now)
}
