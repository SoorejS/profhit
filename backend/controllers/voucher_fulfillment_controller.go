package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"html"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strconv"
	"strings"
	"time"
)

// Admin supplies a purchased official voucher and its invoice/order reference.
// The application never generates a purported gift-card code.
func FulfillVoucher(c *gin.Context) {
	var input struct {
		Code   string `json:"voucher_code" binding:"required,max=300"`
		Source string `json:"source_reference" binding:"required,max=500"`
	}
	if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Source) == "" {
		c.JSON(400, gin.H{"error": "Actual voucher code and official supplier invoice/order reference are required"})
		return
	}
	admin := c.MustGet("userID").(uint)
	encrypted, err := services.EncryptVoucher(strings.TrimSpace(input.Code))
	if err != nil {
		c.JSON(503, gin.H{"error": "Voucher storage encryption is not configured"})
		return
	}
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var w models.WithdrawalRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&w, c.Param("id")).Error; err != nil {
			return err
		}
		if w.Status != "Processing" {
			return gorm.ErrInvalidData
		}
		now := time.Now().UTC()
		w.VoucherCode = encrypted
		w.SourceReference = strings.TrimSpace(input.Source)
		w.Status = "Fulfilled"
		w.AdminID = &admin
		w.FulfilledAt = &now
		if err := tx.Save(&w).Error; err != nil {
			return err
		}
		if w.Tier != "Bronze" {
			b := models.Badge{Code: "TIER_" + strings.ToUpper(w.Tier), Name: w.Tier + " badge", Description: "Earned by voucher fulfilment"}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&b).Error; err != nil {
				return err
			}
			if err := tx.Where("code = ?", b.Code).First(&b).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserBadge{UserID: w.UserID, BadgeID: b.ID, EarnedAt: now}).Error; err != nil {
				return err
			}
			// Never downgrade an already-earned higher tier.
			tiers := []string{"Bronze", "Silver", "Gold", "Platinum", "Diamond"}
			allowed := []string{}
			for _, tier := range tiers {
				allowed = append(allowed, tier)
				if tier == w.Tier {
					break
				}
			}
			if err := tx.Model(&models.User{}).Where("id = ? AND tier IN ?", w.UserID, allowed).Update("tier", w.Tier).Error; err != nil {
				return err
			}
		}
		return services.LogAction(tx, admin, "FULFILL_VOUCHER", fmt.Sprint(w.ID), "Official supplier reference: "+w.SourceReference, c.ClientIP())
	})
	if err != nil {
		c.JSON(409, gin.H{"error": "Voucher must be Processing before fulfilment"})
		return
	}
	c.JSON(200, gin.H{"message": "Voucher fulfilled. Email delivery is a separate tracked operation."})
}

func DeliverVoucher(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid voucher ID"})
		return
	}
	// A persistent claim prevents parallel sends. If the process dies after SMTP
	// acceptance, the operator reconciles the dispatch instead of automatically resending.
	now := time.Now().UTC()
	claimed := config.DB.Model(&models.WithdrawalRequest{}).Where("id = ? AND status IN ? AND dispatch_started_at IS NULL", id, []string{"Fulfilled", "Failed"}).Update("dispatch_started_at", now)
	if claimed.Error != nil || claimed.RowsAffected != 1 {
		c.JSON(409, gin.H{"error": "Voucher is not ready or delivery has already been attempted; reconcile delivery before retrying"})
		return
	}
	var w models.WithdrawalRequest
	var u models.User
	err = config.DB.First(&w, id).Error
	if err == nil {
		err = config.DB.First(&u, w.UserID).Error
	}
	if err == nil && (w.VoucherCode == "" || w.SourceReference == "") {
		err = fmt.Errorf("sourced voucher missing")
	}
	if err == nil {
		var code string
		code, err = services.DecryptVoucher(w.VoucherCode)
		if err == nil {
			err = services.SendEmail(u.Email, "Your PROPHIT voucher", fmt.Sprintf("<p>Your %s voucher (₹%d) is ready.</p><p>Code: <strong>%s</strong></p><p>Request #%d. Keep this code private.</p>", html.EscapeString(w.Tier), w.Amount, html.EscapeString(code), w.ID))
		}
	}
	if err != nil {
		config.DB.Model(&models.WithdrawalRequest{}).Where("id = ?", id).Updates(map[string]interface{}{"status": "Failed", "delivery_error": "Email dispatch failed or has uncertain outcome. Reconcile SMTP logs before retrying."})
		c.JSON(503, gin.H{"error": "Voucher delivery failed; status recorded for reconciliation"})
		return
	}
	if config.DB.Transaction(func(tx *gorm.DB) error {
		delivered := time.Now().UTC()
		if err := tx.Model(&models.WithdrawalRequest{}).Where("id = ?", id).Updates(map[string]interface{}{"status": "Delivered", "delivered_at": delivered, "delivery_error": ""}).Error; err != nil {
			return err
		}
		return tx.Create(&models.Notification{UserID: w.UserID, Key: fmt.Sprintf("voucher:%d", w.ID), Message: fmt.Sprintf("Your %s voucher was sent by email", w.Tier)}).Error
	}) != nil {
		c.JSON(500, gin.H{"error": "Email accepted but delivery state could not be persisted; reconcile before retrying"})
		return
	}
	services.BroadcastToUser(w.UserID, "notification_created", gin.H{"voucher_id": w.ID})
	c.JSON(200, gin.H{"message": "Voucher email accepted by SMTP; delivery recorded"})
}
