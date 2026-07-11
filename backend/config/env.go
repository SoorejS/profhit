package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// ValidateEnv loads .env and ensures required variables are present
func ValidateEnv() {
	godotenv.Load() // silently ignore if no .env

	requiredVars := []string{
		"JWT_SECRET",
		"RAZORPAY_KEY_ID",
		"RAZORPAY_KEY_SECRET",
		"RAZORPAY_WEBHOOK_SECRET",
		"HYPERVERGE_API_KEY",
		"HYPERVERGE_API_SECRET",
		"HYPERVERGE_WORKFLOW_ID",
		"HYPERVERGE_WEBHOOK_SECRET",
	}

	missing := false
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			log.Printf("CRITICAL: Environment variable %s is not set!", v)
			missing = true
		}
	}

	if missing {
		log.Fatal("Halting startup due to missing required environment variables.")
	}
}
