package main

import (
	"log"
	"os"
	"profhit-backend/config"
)

func main() {
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("Set the target PostgreSQL environment explicitly")
	}
	config.ValidateEnv()
	config.ConnectDB()
	if err := config.Migrate(config.DB); err != nil {
		log.Fatal("Schema migration failed")
	}
	log.Println("Schema migration complete")
}
