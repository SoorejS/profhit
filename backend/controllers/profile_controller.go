package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"regexp"
	"strings"
	"time"
)

func UpdateProfile(c *gin.Context) {
	var req struct {
		FullName  string `json:"full_name" binding:"required,max=100"`
		Phone     string `json:"phone" binding:"required,max=20"`
		City      string `json:"city" binding:"required,max=100"`
		Country   string `json:"country" binding:"required,max=100"`
		Interests string `json:"interests" binding:"required,max=500"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Name, phone, city, country and interests are required within their length limits"})
		return
	}
	for _, s := range []string{req.FullName, req.Phone, req.City, req.Country, req.Interests} {
		if strings.TrimSpace(s) == "" {
			c.JSON(400, gin.H{"error": "Profile fields cannot be blank"})
			return
		}
	}
	id := c.MustGet("userID").(uint)
	if !regexp.MustCompile(`^\+?[0-9]{7,15}$`).MatchString(strings.TrimSpace(req.Phone)) {
		c.JSON(400, gin.H{"error": "Enter a phone number with 7–15 digits and an optional country-code +"})
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := services.LockWalletTx(tx, id); err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{"full_name": strings.TrimSpace(req.FullName), "phone": strings.TrimSpace(req.Phone), "city": strings.TrimSpace(req.City), "country": strings.TrimSpace(req.Country), "interests": strings.TrimSpace(req.Interests)}).Error; err != nil {
			return err
		}
		var user models.User
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		if !services.ProfileComplete(user) {
			return gorm.ErrInvalidData
		}
		existing := models.Achievement{Code: "PROFILE_100", Title: "100% Profile Completed", Description: "Completed name, email, phone, city, country and interests", Reward: 30}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&existing).Error; err != nil {
			return err
		}
		if err := tx.Where("code = ?", "PROFILE_100").First(&existing).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.UserAchievement{}).Where("user_id = ? AND achievement_id = ?", id, existing.ID).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return nil
		}
		if err := tx.Create(&models.UserAchievement{UserID: id, AchievementID: existing.ID, UnlockedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		return services.CreditWalletTx(tx, id, 30, models.TxTypeAdminAdjustment, existing.ID, "Profile completion", nil)
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not update profile"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Profile saved. Completion reward is 30 coins, once per account."})
}
