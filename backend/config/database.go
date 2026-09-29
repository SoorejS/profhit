package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

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
			log.Printf("PostgreSQL DB not ready (attempt %d/10), retrying in 2s...", i+1)
			time.Sleep(2 * time.Second)
		}
	} else if os.Getenv("USE_SQLITE") == "true" {
		log.Println("Using SQLite for local development")
		db, err = gorm.Open(sqlite.Open("profhit.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			log.Fatal("Failed to connect to SQLite: ", err)
		}
	} else {
		sslMode := os.Getenv("DB_SSLMODE")
		if sslMode == "" {
			sslMode = "require"
		}
		dsnURL := url.URL{Scheme: "postgres", User: url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD")), Host: fmt.Sprintf("%s:%s", os.Getenv("DB_HOST"), os.Getenv("DB_PORT")), Path: "/" + os.Getenv("DB_NAME")}
		query := url.Values{"sslmode": {sslMode}, "TimeZone": {"UTC"}}
		dsnURL.RawQuery = query.Encode()
		dsn := dsnURL.String()
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
		log.Fatal("Failed to connect to database after 10 attempts; check database settings and availability")
	}

	// Connection pool tuning
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	if db.Dialector.Name() == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Successfully connected to the database!")
	DB = db
}
