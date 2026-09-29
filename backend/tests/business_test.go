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
	db, err := gorm.Open(sqlite.Open(":memory:?_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	config.DB = db

	if err := config.Migrate(db); err != nil {
		panic(err)
	}
}

func TestProfileCompletionAchievement(t *testing.T) {
	setupTestDB()

	// Create user
	user := models.User{
		Username:         "achieve_test",
		Email:            "achieve@test.com",
		Points:           0,
		KycStatus:        true,
		TwoFactorSecret:  "SECRET",
		TwoFactorEnabled: true,
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
