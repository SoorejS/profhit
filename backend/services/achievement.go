package services

import (
	"log"
	"fmt"
	"profhit-backend/config"
	"profhit-backend/models"
)

// CheckProfileCompletion checks if a user has 100% completed their profile.
// If yes, it unlocks the achievement and awards a bonus via WalletLedger.
func CheckProfileCompletion(userID uint) {
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		return
	}

	// Definition of 100% Profile:
	// - KYC Status is true
	// - 2FA is enabled (TwoFactorSecret is not empty)
	if !user.KycStatus || user.TwoFactorSecret == "" {
		return
	}

	// Check if achievement already unlocked
	var ach models.Achievement
	if err := config.DB.Where("code = ?", "PROFILE_100").First(&ach).Error; err != nil {
		// If achievement doesn't exist in DB yet, create it
		ach = models.Achievement{
			Code:        "PROFILE_100",
			Title:       "100% Profile Completed",
			Description: "Completed KYC and enabled 2FA.",
			Reward:      250,
			Icon:        "fa-solid fa-id-card",
		}
		config.DB.Create(&ach)
	}

	var userAch models.UserAchievement
	if err := config.DB.Where("user_id = ? AND achievement_id = ?", userID, ach.ID).First(&userAch).Error; err == nil {
		// Already unlocked
		return
	}

	// Unlock it
	config.DB.Create(&models.UserAchievement{
		UserID:        userID,
		AchievementID: ach.ID,
	})

	// Award coins
	tx := config.DB.Begin()
	if err := CreditWalletTx(tx, userID, ach.Reward, models.TxTypeAdminAdjustment, 0, "Achievement Unlocked: 100% Profile", nil); err == nil {
		tx.Commit()
		BroadcastToUser(userID, "achievement_unlocked", "You unlocked: 100% Profile Completed! +250 Coins")
	} else {
		tx.Rollback()
		log.Println("Failed to award achievement coins:", err)
	}
}

// CheckPredictionAchievements checks and unlocks achievements related to predictions
func CheckPredictionAchievements(userID uint) {
	// Count user predictions
	var count int64
	config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ?", userID).Count(&count)

	if count == 1 {
		UnlockAchievement(userID, "FIRST_PREDICTION", "First Prediction", "Make your first prediction", 50, "fa-solid fa-seedling")
	} else if count == 10 {
		UnlockAchievement(userID, "PREDICTIONS_10", "10 Predictions", "Make 10 predictions", 100, "fa-solid fa-tree")
	} else if count == 100 {
		UnlockAchievement(userID, "PREDICTIONS_100", "Centurion", "Make 100 predictions", 500, "fa-solid fa-crown")
	}
}

// UnlockAchievement is a generic helper
func UnlockAchievement(userID uint, code, title, desc string, reward int, icon string) {
	var ach models.Achievement
	if err := config.DB.Where("code = ?", code).First(&ach).Error; err != nil {
		ach = models.Achievement{
			Code:        code,
			Title:       title,
			Description: desc,
			Reward:      reward,
			Icon:        icon,
		}
		config.DB.Create(&ach)
	}

	var userAch models.UserAchievement
	if err := config.DB.Where("user_id = ? AND achievement_id = ?", userID, ach.ID).First(&userAch).Error; err == nil {
		return // Already unlocked
	}

	config.DB.Create(&models.UserAchievement{
		UserID:        userID,
		AchievementID: ach.ID,
	})

	if reward > 0 {
		tx := config.DB.Begin()
		if err := CreditWalletTx(tx, userID, reward, models.TxTypeAdminAdjustment, 0, "Achievement Unlocked: "+title, nil); err == nil {
			tx.Commit()
			BroadcastToUser(userID, "achievement_unlocked", fmt.Sprintf("You unlocked: %s! +%d Coins", title, reward))
		} else {
			tx.Rollback()
		}
	} else {
		BroadcastToUser(userID, "achievement_unlocked", "You unlocked: "+title+"!")
	}
}
