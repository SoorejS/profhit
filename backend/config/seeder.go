package config

import (
	"log"
	"profhit-backend/models"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func SeedDatabase() {
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
			DB.Create(&u)
		}
		log.Println("Seeded admin and regular users.")
	}

	DB.Model(&models.Market{}).Count(&count)

	if count > 0 {
		return // Already seeded
	}

	log.Println("Database is empty. Seeding initial fixed-odds markets...")

	// Resolution dates
	soon := time.Now().AddDate(0, 0, 1)

	markets := []models.Market{
		{
			Title:            "Will it rain today in Mumbai?",
			Description:      "Predict if Mumbai will experience any rainfall today.",
			Category:         "Weather",
			Difficulty:       "Easy",
			Payout:           20,
			Options:          `["Yes", "No"]`,
			ResolutionStatus: "Open",
			EndDate:          soon,
		},
		{
			Title:            "Which team will win the IPL Finals?",
			Description:      "Predict the winner of the upcoming IPL Finals.",
			Category:         "Sports",
			Difficulty:       "Medium",
			Payout:           60,
			Options:          `["CSK", "MI", "RCB", "KKR"]`,
			ResolutionStatus: "Open",
			EndDate:          soon,
		},
		{
			Title:            "Will Nifty 50 close up or down today?",
			Description:      "Predict the closing direction of Nifty 50.",
			Category:         "Markets",
			Difficulty:       "Easy",
			Payout:           20,
			Options:          `["Up", "Down"]`,
			ResolutionStatus: "Open",
			EndDate:          soon,
		},
		{
			Title:            "Predict exact seat count for NDA alliance.",
			Description:      "Enter the exact number of seats NDA will win.",
			Category:         "Politics",
			Difficulty:       "Hard",
			Payout:           250,
			Options:          `[]`,
			ResolutionStatus: "Open",
			EndDate:          time.Now().AddDate(0, 1, 0),
		},
	}

	for _, m := range markets {
		DB.Create(&m)
	}
	log.Println("Seeding complete! Fixed-odds markets loaded.")
}
