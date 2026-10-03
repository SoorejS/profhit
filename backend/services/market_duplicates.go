package services

import (
	"crypto/sha256"
	"encoding/binary"
	"gorm.io/gorm"
	"profhit-backend/models"
	"time"
)

type DuplicateMarketError struct{ Existing models.Market }

func (e *DuplicateMarketError) Error() string { return "A similar prediction already exists" }
func CreateUniqueMarket(db *gorm.DB, market *models.Market) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			hash := sha256.Sum256([]byte("prophit-market:" + market.Category))
			key := int64(binary.BigEndian.Uint64(hash[:8]) & 0x7fffffffffffffff)
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", key).Error; err != nil {
				return err
			}
		}
		existing, err := FindMarketDuplicate(tx, *market)
		if err != nil {
			return err
		}
		if existing != nil {
			return &DuplicateMarketError{Existing: *existing}
		}
		return tx.Create(market).Error
	})
}

func FindMarketDuplicate(db *gorm.DB, market models.Market) (*models.Market, error) {
	if market.LockTime == nil {
		return nil, nil
	}
	var candidates []models.Market
	err := db.Where("id <> ? AND category = ? AND resolution_status NOT IN ? AND lock_time BETWEEN ? AND ?", market.ID, market.Category, []string{"Archived", "Resolved"}, market.LockTime.Add(-24*time.Hour), market.LockTime.Add(24*time.Hour)).Order("id desc").Limit(500).Find(&candidates).Error
	if err != nil {
		return nil, err
	}
	for _, existing := range candidates {
		if market.NewsEventID != nil && existing.NewsEventID != nil && *market.NewsEventID == *existing.NewsEventID || SimilarEventTitles(market.Title, existing.Title) {
			return &existing, nil
		}
	}
	return nil, nil
}
