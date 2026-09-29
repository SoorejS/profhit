package config

import (
	"gorm.io/gorm"
	"profhit-backend/models"
)

// Migrate is shared by startup and integration tests so fresh deployments use
// exactly the schema exercised by the tests. Never discard migration errors.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{}, &models.Market{}, &models.PredictionSubmission{},
		&models.Comment{}, &models.HyperVergeKYC{}, &models.WithdrawalRequest{},
		&models.WalletLedger{}, &models.UserStreak{}, &models.PasswordResetToken{},
		&models.ReferralEvent{}, &models.AuditLog{}, &models.Report{},
		&models.WeeklyChallenge{}, &models.ChallengeParticipant{},
		&models.Achievement{}, &models.UserAchievement{}, &models.Badge{}, &models.UserBadge{},
		&models.RewardItem{}, &models.Redemption{}, &models.CoinBatch{},
		&models.PaymentTransaction{}, &models.RevokedToken{}, &models.Notification{},
	)
}
