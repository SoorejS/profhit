package services

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"profhit-backend/config"
	"profhit-backend/models"
	"testing"
	"time"
)

func TestMarketCutoffComparisonUsesUTC(t *testing.T) {
	previousDB := config.DB
	previousLocal := time.Local
	time.Local = time.FixedZone("Asia/Calcutta", 19800)
	defer func() { config.DB = previousDB; time.Local = previousLocal }()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	defer sql.Close()
	config.DB = db
	if err := config.Migrate(db); err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(2 * time.Hour)
	m := models.Market{Title: "Future UTC cutoff", Category: "Weather", ResolutionStatus: "Live", LockTime: &future, EndDate: future}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	transitionMarkets()
	if err := db.First(&m, m.ID).Error; err != nil {
		t.Fatal(err)
	}
	if m.ResolutionStatus != "Live" {
		t.Fatalf("future UTC cutoff locked early: %s", m.ResolutionStatus)
	}
	past := time.Now().UTC().Add(-time.Second)
	if err := db.Model(&m).Updates(map[string]interface{}{"lock_time": past, "end_date": past}).Error; err != nil {
		t.Fatal(err)
	}
	transitionMarkets()
	if err := db.First(&m, m.ID).Error; err != nil {
		t.Fatal(err)
	}
	if m.ResolutionStatus != "Locked" {
		t.Fatalf("elapsed cutoff did not lock: %s", m.ResolutionStatus)
	}
}
