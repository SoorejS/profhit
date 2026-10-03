package services

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
)

// CheckProfileCompletion checks if a user has 100% completed their profile.
// If yes, it unlocks the achievement and awards a bonus via WalletLedger.
func CheckProfileCompletion(userID uint) {
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		return
	}

	if !ProfileComplete(user) {
		return
	}

	UnlockAchievement(userID, "PROFILE_100", "100% Profile Completed", "Completed name, email, phone, city, country and interests.", 30, "fa-solid fa-id-card")
}

// CheckPredictionAchievements checks and unlocks achievements related to predictions
func CheckPredictionAchievements(userID uint) {
	// Count user predictions
	var count int64
	config.DB.Model(&models.PredictionSubmission{}).Where("user_id = ?", userID).Count(&count)

	if count >= 1 {
		UnlockAchievement(userID, "FIRST_PREDICTION", "First Prediction", "Make your first prediction", 0, "fa-solid fa-seedling")
	}
	if count >= 10 {
		UnlockAchievement(userID, "PREDICTIONS_10", "10 Predictions", "Make 10 predictions", 0, "fa-solid fa-tree")
	}
	if count >= 100 {
		UnlockAchievement(userID, "PREDICTIONS_100", "Centurion", "Make 100 predictions", 0, "fa-solid fa-crown")
	}
}

// UnlockAchievement is a generic helper
func UnlockAchievement(userID uint, code, title, desc string, reward int, icon string) {
	awarded := false
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := LockWalletTx(tx, userID); err != nil {
			return err
		}
		ach := models.Achievement{Code: code, Title: title, Description: desc, Reward: reward, Icon: icon}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ach).Error; err != nil {
			return err
		}
		if err := tx.Where("code = ?", code).First(&ach).Error; err != nil {
			return err
		}
		// Correct current definitions without rewriting historical wallet awards.
		if err := tx.Model(&ach).Updates(map[string]interface{}{"title": title, "description": desc, "reward": reward, "icon": icon}).Error; err != nil {
			return err
		}
		claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserAchievement{UserID: userID, AchievementID: ach.ID, UnlockedAt: time.Now().UTC()})
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return nil
		}
		if reward > 0 {
			if err := CreditWalletTx(tx, userID, reward, models.TxTypeAdminAdjustment, ach.ID, "Achievement unlocked: "+ach.Title, nil); err != nil {
				return err
			}
		}
		awarded = true
		return nil
	})
	if err != nil {
		log.Printf("Achievement award failed: %v", err)
		return
	}
	if awarded {
		BroadcastToUser(userID, "achievement_unlocked", fmt.Sprintf("You unlocked: %s!", title))
	}
}
