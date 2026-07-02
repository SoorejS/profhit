package main

import (
	"log"
	"os"

	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/routes"
)

func main() {
	// Initialize Database Connection
	config.ConnectDB()

	// ── Security: refuse to start without a JWT_SECRET ───────────────────────
	// Loading .env before the check ensures local dev still works.
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable is not set. " +
			"Set a strong random secret (≥32 chars) in your environment or .env file. " +
			"The application cannot start without it.")
	}

	// Auto-Migrate the database models
	log.Println("Running Auto-Migration...")
	err := config.DB.AutoMigrate(
		&models.User{},
		&models.Market{},
		&models.PredictionSubmission{},
		&models.Comment{},
		&models.HyperVergeKYC{},
		&models.WithdrawalRequest{},
		&models.WalletLedger{},       // Double-entry wallet ledger
		&models.UserStreak{},         // Daily login streak tracker
		&models.PasswordResetToken{}, // Password reset tokens
		&models.ReferralEvent{},      // Referral events
		&models.AuditLog{},           // Audit logs
		&models.Report{},             // Reports
	)
	if err != nil {
		log.Fatal("Failed to migrate database: \n", err)
	}

	// Seed dummy data if empty
	config.SeedDatabase()

	// Setup Router
	r := routes.SetupRouter()

	// Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s", port)
	r.Run(":" + port)
}
