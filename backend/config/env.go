package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
)

// ValidateEnv fails closed; optional integrations remain unavailable without credentials.
func ValidateEnv() {
	_ = godotenv.Load()
	if err := CheckEnv(); err != nil {
		log.Fatal(err)
	}
}
func CheckEnv() error {
	if os.Getenv("GIN_MODE") == "release" && os.Getenv("USE_SQLITE") == "true" && os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("production requires PostgreSQL; SQLite is development-only")
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 || strings.Contains(strings.ToLower(secret), "replace_this") || strings.Contains(strings.ToLower(secret), "change-before") || secret == "super_secret_jwt_key_for_production" {
		return fmt.Errorf("JWT_SECRET must be a unique random secret of at least 32 characters")
	}
	for _, key := range []string{"RAZORPAY_KEY_SECRET", "RAZORPAY_WEBHOOK_SECRET", "HYPERVERGE_API_SECRET", "HYPERVERGE_WEBHOOK_SECRET"} {
		value := strings.ToLower(os.Getenv(key))
		if strings.HasPrefix(value, "dummy") || strings.HasPrefix(value, "your_") {
			return fmt.Errorf("%s contains placeholder credentials", key)
		}
	}
	return nil
}
