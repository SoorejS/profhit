package config

import (
	"fmt"
	"gorm.io/gorm"
	"log"
	"os"
	"profhit-backend/models"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func SeedDatabase() {
	if os.Getenv("SEED_DEMO_DATA") != "true" || os.Getenv("GIN_MODE") == "release" {
		return
	}
	var count int64
	DB.Model(&models.User{}).Count(&count)

	if count == 0 {
		log.Println("Seeding mock users with roles...")
		hashedPasswordBytes, _ := bcrypt.GenerateFromPassword([]byte("password"), 12)
		hashedPassword := string(hashedPasswordBytes)

		hashedPBytes, _ := bcrypt.GenerateFromPassword([]byte("p"), 12)
		hashedP := string(hashedPBytes)

		users := []models.User{
			// ---- Admin Roles ----
			{Username: "SuperAdmin", Email: "superadmin@prophit.com", Password: hashedPassword, Tier: "Diamond", Role: models.RoleSuperAdmin, IsActive: true, Points: 9999, KycStatus: true},
			{Username: "AdminUser", Email: "admin@prophit.com", Password: hashedPassword, Tier: "Gold", Role: models.RoleAdmin, IsActive: true, Points: 5000, KycStatus: true},
			// ---- Regular Users ----
			{Username: "You", Email: "test@example.com", Password: hashedPassword, Tier: "Gold", Role: models.RoleUser, IsActive: true, Points: 100, KycStatus: true},
			{Username: "WhaleTrader99", Email: "w@w.com", Password: hashedP, Tier: "Diamond", Role: models.RoleUser, IsActive: true, Points: 8540, KycStatus: true},
		}
		for _, u := range users {
			u.ReferralCode = fmt.Sprintf("DEMO%d", len(u.Username))
			amount := u.Points
			if err := DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Create(&u).Error; err != nil {
					return err
				}
				if err := tx.Create(&models.WalletLedger{UserID: u.ID, Type: models.TxTypeAdminAdjustment, Credit: amount, BalanceAfter: amount, Description: "Development seed balance", Status: "completed"}).Error; err != nil {
					return err
				}
				earned := time.Now().UTC()
				return tx.Create(&models.CoinBatch{UserID: u.ID, Amount: amount, Balance: amount, CreatedAt: earned, ExpiresAt: models.CoinExpiry(earned), Source: "development_seed"}).Error
			}); err != nil {
				log.Printf("Demo seed failed: %v", err)
			}
		}
		log.Println("Seeded admin and regular users.")
	}

	DB.Model(&models.Market{}).Count(&count)

	if count > 0 {
		return // Already seeded
	}

	log.Println("Database is empty. Seeding initial fixed-odds markets...")

	// Resolution dates
	soon := time.Now().UTC().AddDate(0, 0, 1)

	markets := []models.Market{
		{Title: "Development demo: rain observation", Category: "Weather", Difficulty: "Easy", Payout: 20, PredictionType: "binary", Options: `["Yes","No"]`, ResolutionSource: "https://openweathermap.org/"},
		{Title: "Development demo: sports winner", Category: "Sports", Difficulty: "Easy", Payout: 25, PredictionType: "winner", Options: `["Team A","Team B"]`, ResolutionSource: "https://www.espn.com/"},
		{Title: "Development demo: neutral election result", Category: "Politics", Difficulty: "Easy", Payout: 30, PredictionType: "winner", Options: `["Candidate A","Candidate B"]`, ResolutionSource: "https://eci.gov.in/"},
		{Title: "Development demo: entertainment winner", Category: "Entertainment", Difficulty: "Easy", Payout: 25, PredictionType: "winner", Options: `["Nominee A","Nominee B"]`, ResolutionSource: "https://oscars.org/"},
		{Title: "Development demo: market direction", Category: "Financial Markets", Difficulty: "Easy", Payout: 20, PredictionType: "direction", Options: `["Up","Down"]`, ResolutionSource: "https://nseindia.com/"},
		{Title: "Development demo: two-option event", Category: "Wild Card", Difficulty: "Easy", Payout: 40, PredictionType: "binary", Options: `["Yes","No"]`, ResolutionSource: "https://nasa.gov/"},
	}

	for _, m := range markets {
		m.Description = "Disposable development fixture, not a current event or real forecast."
		m.ResolutionRule = "Development fixture: select the sourced result at the published cutoff. Editorial review is required for actual events."
		m.LockTime = &soon
		m.EndDate = soon
		m.ResolutionStatus = "Live"
		m.Visibility = "Public"
		DB.Create(&m)
	}
	log.Println("Seeding complete! Fixed-odds markets loaded.")
}
