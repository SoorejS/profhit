package tests

import (
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"testing"
)

func setupTestDB() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	config.DB = db

	// Drop tables first to prevent shared memory cache contamination between tests
	config.DB.Migrator().DropTable(
		&models.User{},
		&models.Market{},
		&models.PredictionSubmission{},
		&models.Comment{},
		&models.HyperVergeKYC{},
		&models.WithdrawalRequest{},
		&models.WalletLedger{},
		&models.UserStreak{},
		&models.PasswordResetToken{},
		&models.ReferralEvent{},
		&models.AuditLog{},
		&models.Report{},
		&models.WeeklyChallenge{},
		&models.ChallengeParticipant{},
		&models.Achievement{},
		&models.UserAchievement{},
		&models.Badge{},
		&models.UserBadge{},
		&models.RewardItem{},
		&models.Redemption{},
		&models.CoinBatch{},
		&models.PaymentTransaction{},
	)

	config.DB.AutoMigrate(
		&models.User{},
		&models.Market{},
		&models.PredictionSubmission{},
		&models.Comment{},
		&models.HyperVergeKYC{},
		&models.WithdrawalRequest{},
		&models.WalletLedger{},
		&models.UserStreak{},
		&models.PasswordResetToken{},
		&models.ReferralEvent{},
		&models.AuditLog{},
		&models.Report{},
		&models.WeeklyChallenge{},
		&models.ChallengeParticipant{},
		&models.Achievement{},
		&models.UserAchievement{},
		&models.Badge{},
		&models.UserBadge{},
		&models.RewardItem{},
		&models.Redemption{},
		&models.CoinBatch{},
		&models.PaymentTransaction{},
	)
}

func TestProfileCompletionAchievement(t *testing.T) {
	setupTestDB()

	// Create user
	user := models.User{
		Username:        "achieve_test",
		Email:           "achieve@test.com",
		Points:          0,
		KycStatus:       true,
		TwoFactorSecret: "SECRET",
	}
	config.DB.Create(&user)

	// Trigger logic
	services.CheckProfileCompletion(user.ID)

	// Verify achievement unlocked
	var ach models.UserAchievement
	err := config.DB.Where("user_id = ?", user.ID).First(&ach).Error
	assert.NoError(t, err)

	// Verify points awarded (default 0 + 250)
	var u models.User
	config.DB.First(&u, user.ID)
	assert.Equal(t, 250, u.Points)

	// Trigger again, should not double award
	services.CheckProfileCompletion(user.ID)
	config.DB.First(&u, user.ID)
	assert.Equal(t, 250, u.Points) // Still 250
}
