package services

import (
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
)

// AdvanceMarketLifecycle is safe across stateless replicas. Requests reconcile
// cutoffs even when a scaled-to-zero instance has no running background ticker.
func AdvanceMarketLifecycle() error {
	now := time.Now().UTC()
	for _, step := range []struct {
		From          []string
		Condition, To string
	}{
		{[]string{"Scheduled"}, "start_time <= ?", "Live"},
		{[]string{"Live", "Open"}, "COALESCE(lock_time, end_date) <= ?", "Locked"},
		{[]string{"Locked"}, "resolution_time <= ?", "Awaiting Resolution"},
	} {
		if err := config.DB.Model(&models.Market{}).Where("resolution_status IN ?", step.From).Where(step.Condition, now).Update("resolution_status", step.To).Error; err != nil {
			return err
		}
	}
	return nil
}
