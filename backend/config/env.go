package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// ValidateEnv loads .env and provides fallback defaults for required variables
func ValidateEnv() {
	godotenv.Load() // silently ignore if no .env

	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "eda29bf0185763c4")
		log.Println("[Env] JWT_SECRET not set in environment, using default secret.")
	}

	defaults := map[string]string{
		"RAZORPAY_KEY_ID":           "dummy_rzp_key",
		"RAZORPAY_KEY_SECRET":       "dummy_rzp_secret",
		"RAZORPAY_WEBHOOK_SECRET":   "dummy_rzp_webhook_secret",
		"HYPERVERGE_API_KEY":        "dummy_key_for_testing",
		"HYPERVERGE_API_SECRET":     "dummy_secret_for_testing",
		"HYPERVERGE_WORKFLOW_ID":    "dummy_workflow_id",
		"HYPERVERGE_WEBHOOK_SECRET": "dummy_webhook_secret",
	}

	for k, v := range defaults {
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
			log.Printf("[Env] %s not set, using fallback dummy testing value.\n", k)
		}
	}
}
