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

	// Auto-Migrate the database models
	log.Println("Running Auto-Migration...")
	err := config.DB.AutoMigrate(&models.User{}, &models.Market{}, &models.Trade{}, &models.LimitOrder{}, &models.Comment{})
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
