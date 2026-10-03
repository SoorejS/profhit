package controllers

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"time"
)

func ConfigureWeatherResolution(c *gin.Context) {
	var spec services.WeatherResultSpec
	if c.ShouldBindJSON(&spec) != nil {
		c.JSON(400, gin.H{"error": "Invalid result specification"})
		return
	}
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var m models.Market
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, c.Param("id")).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.PredictionSubmission{}).Where("market_id = ?", m.ID).Count(&count).Error; err != nil {
			return err
		}
		if m.ResolutionStatus != "Draft" || count != 0 {
			return errors.New("only unpublished drafts without participation can configure results")
		}
		if err := spec.Validate(m); err != nil {
			return err
		}
		now := time.Now().UTC()
		if !spec.ObservationFrom.After(now) || spec.ObservationUntil.After(now.Add(7*24*time.Hour)) {
			return errors.New("provider observation must be within the next seven days")
		}
		m.ResolutionRule = spec.Rule()
		m.ResolutionSource = spec.EvidenceURL()
		m.Options = `["Yes","No"]`
		m.ResolutionTime = &spec.ObservationFrom
		encoded, _ := json.Marshal(spec)
		m.ResultSpec = string(encoded)
		m.ResultApprovedBy = c.MustGet("userID").(uint)
		return tx.Save(&m).Error
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Deterministic weather result configured. Review the exact coordinates, metric, threshold, time and question before publishing."})
}
