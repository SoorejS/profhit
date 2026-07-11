package tests

import (
	"testing"
	"profhit-backend/models"
	"profhit-backend/services"
	"profhit-backend/config"
	"github.com/stretchr/testify/assert"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	config.DB = db
	config.DB.AutoMigrate(&models.User{}, &models.UserAchievement{}, &models.WalletLedger{}, &models.Achievement{})
}

func TestProfileCompletionAchievement(t *testing.T) {
	setupTestDB()

	// Create user
	user := models.User{
		Username: "achieve_test",
		Email: "achieve@test.com",
		Points: 0,
		KycStatus: true,
		TwoFactorSecret: "SECRET",
	}
	config.DB.Create(&user)

	// Trigger logic
	services.CheckProfileCompletion(user.ID)

	// Verify achievement unlocked
	var ach models.UserAchievement
	err := config.DB.Where("user_id = ?", user.ID).First(&ach).Error
	assert.NoError(t, err)

	// Verify points awarded (default 100 + 250)
	var u models.User
	config.DB.First(&u, user.ID)
	assert.Equal(t, 350, u.Points)

	// Trigger again, should not double award
	services.CheckProfileCompletion(user.ID)
	config.DB.First(&u, user.ID)
	assert.Equal(t, 350, u.Points) // Still 350
}
