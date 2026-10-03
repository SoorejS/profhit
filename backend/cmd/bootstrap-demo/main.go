// bootstrap-demo provisions an administrator only through an operator's database
// connection. It creates no public bootstrap endpoint and never resets accounts.
package main

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"log"
	"net/mail"
	"os"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
	"strings"
)

func main() {
	if os.Getenv("VIRTUAL_COIN_DEMO") != "true" || os.Getenv("DATABASE_URL") == "" {
		log.Fatal("Requires the operator's demo PostgreSQL environment")
	}
	email := strings.ToLower(strings.TrimSpace(os.Getenv("DEMO_ADMIN_EMAIL")))
	password := os.Getenv("DEMO_ADMIN_PASSWORD")
	if _, err := mail.ParseAddress(email); err != nil || len(password) < 32 || len(password) > 72 {
		log.Fatal("Set DEMO_ADMIN_EMAIL and a unique DEMO_ADMIN_PASSWORD of 32-72 bytes")
	}
	config.ValidateEnv()
	config.ConnectDB()
	if err := config.Migrate(config.DB); err != nil {
		log.Fatal("Schema migration failed")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatal("Password hashing failed")
	}
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(72831005)").Error; err != nil {
			return err
		}
		var existing models.User
		if err := tx.Where("email = ?", email).First(&existing).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("Administrator provisioning requires a new email; existing accounts are never overwritten")
		}
		u := models.User{Username: os.Getenv("DEMO_ADMIN_USERNAME"), Email: email, Password: string(hash), Role: models.RoleSuperAdmin, IsActive: true, ReferralCode: services.GenerateReferralCode()}
		if len(u.Username) < 3 || len(u.Username) > 64 {
			return errors.New("Set DEMO_ADMIN_USERNAME")
		}
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		return services.LogAction(tx, u.ID, "BOOTSTRAP_DEMO_ADMIN", "demo", "Operator provisioned administrator; no seeded credentials", "operator")
	})
	if err != nil {
		log.Fatal("Demo administrator provisioning failed")
	}
	log.Println("Demo administrator created")
}
