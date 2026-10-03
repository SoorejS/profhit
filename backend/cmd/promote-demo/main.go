// promote-demo is an operator-only role assignment for an already registered
// demo account. The user must log in normally again after promotion.
package main

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"os"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
)

func main() {
	if os.Getenv("VIRTUAL_COIN_DEMO") != "true" || os.Getenv("DATABASE_URL") == "" || os.Getenv("DEMO_ADMIN_EMAIL") == "" || os.Getenv("DEMO_ADMIN_USERNAME") == "" {
		log.Fatal("Requires the operator's demo database and exact registered account identity")
	}
	config.ValidateEnv()
	config.ConnectDB()
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ? AND username = ? AND is_active = ?", os.Getenv("DEMO_ADMIN_EMAIL"), os.Getenv("DEMO_ADMIN_USERNAME"), true).First(&user).Error; err != nil {
			return err
		}
		if user.Role == models.RoleSuperAdmin {
			return nil
		}
		if err := tx.Model(&user).Updates(map[string]interface{}{"role": models.RoleSuperAdmin, "token_version": gorm.Expr("token_version + 1")}).Error; err != nil {
			return err
		}
		return services.LogAction(tx, user.ID, "PROMOTE_DEMO_ADMIN", fmt.Sprint(user.ID), "Operator assigned demo administrator role; previous sessions revoked", "operator")
	})
	if err != nil {
		log.Fatal("Demo administrator promotion failed")
	}
	log.Println("Demo account promoted; sign in again")
}
