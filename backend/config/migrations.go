package config

import (
	"gorm.io/gorm"
	"profhit-backend/models"
)

// Migrate is shared by startup and integration tests so fresh deployments use
// exactly the schema exercised by the tests. Never discard migration errors.
func Migrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(72831004)").Error; err != nil {
				return err
			}
			return migrateModels(tx)
		})
	}
	return migrateModels(db)
}
func migrateModels(db *gorm.DB) error {
	// Removing the old positive-stake constraint permits free submissions. Rows remain intact.
	if db.Migrator().HasConstraint(&models.PredictionSubmission{}, "chk_prediction_submissions_amount") {
		if err := db.Migrator().DropConstraint(&models.PredictionSubmission{}, "chk_prediction_submissions_amount"); err != nil {
			return err
		}
	}
	return db.AutoMigrate(
		&models.User{}, &models.Market{}, &models.PredictionSubmission{},
		&models.Comment{}, &models.HyperVergeKYC{}, &models.WithdrawalRequest{},
		&models.WalletLedger{}, &models.UserStreak{}, &models.PasswordResetToken{},
		&models.ReferralEvent{}, &models.AuditLog{}, &models.Report{},
		&models.WeeklyChallenge{}, &models.ChallengeParticipant{},
		&models.Achievement{}, &models.UserAchievement{}, &models.Badge{}, &models.UserBadge{},
		&models.RewardItem{}, &models.Redemption{}, &models.CoinBatch{},
		&models.PaymentTransaction{}, &models.RevokedToken{}, &models.Notification{},
		&models.PredictionStreak{}, &models.EconomyMigration{},
		&models.CoinConsumption{},
		&models.NewsEvent{}, &models.NewsSource{}, &models.NewsIngestionState{},
		&models.RateLimitBucket{},
	)
}
