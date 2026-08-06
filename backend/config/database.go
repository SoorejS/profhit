package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"profhit-backend/models"

	"github.com/glebarez/sqlite"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	godotenv.Load() // silently ignore if no .env

	var db *gorm.DB
	var err error

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		log.Println("Connecting to PostgreSQL using DATABASE_URL...")
		for i := 0; i < 10; i++ {
			db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			if err == nil {
				break
			}
			log.Printf("PostgreSQL DB not ready (attempt %d/10), retrying in 2s... Error: %v", i+1, err)
			time.Sleep(2 * time.Second)
		}
	} else if os.Getenv("USE_SQLITE") == "true" {
		log.Println("Using SQLite for local development")
		db, err = gorm.Open(sqlite.Open("profhit.db"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			log.Fatal("Failed to connect to SQLite: ", err)
		}
	} else {
		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
			os.Getenv("DB_PORT"),
		)
		for i := 0; i < 10; i++ {
			db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			if err == nil {
				break
			}
			log.Printf("DB not ready (attempt %d/10), retrying in 2s...", i+1)
			time.Sleep(2 * time.Second)
		}
	}

	if err != nil {
		log.Fatal("Failed to connect to database after 10 attempts:\n", err)
	}

	if err := db.AutoMigrate(
		&models.WalletLedger{},
		&models.HyperVergeKYC{},
		&models.UserStreak{},
		&models.CoinBatch{},
		&models.RewardItem{},
		&models.Redemption{},
		&models.WithdrawalRequest{},
		&models.PaymentTransaction{},
	); err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}

	// Connection pool tuning
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Successfully connected to the database!")
	DB = db
}
