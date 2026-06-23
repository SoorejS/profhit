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
			{Username: "ContentCreator1", Email: "creator@prophit.com", Password: hashedPassword, Tier: "Gold", Role: models.RoleContentCreator, IsActive: true, Points: 2000, KycStatus: true},
			{Username: "ITSupport1", Email: "itsupport@prophit.com", Password: hashedPassword, Tier: "Silver", Role: models.RoleITSupport, IsActive: true, Points: 500, KycStatus: false},
			// ---- Regular Users ----
			{Username: "You", Email: "test@example.com", Password: hashedPassword, Tier: "Gold", Role: models.RoleUser, IsActive: true, Points: 100, KycStatus: true},
			{Username: "WhaleTrader99", Email: "w@w.com", Password: hashedP, Tier: "Diamond", Role: models.RoleUser, IsActive: true, Points: 8540, KycStatus: true},
			{Username: "CryptoKing", Email: "c@c.com", Password: hashedP, Tier: "Gold", Role: models.RoleUser, IsActive: true, Points: 3120, KycStatus: true},
			{Username: "PolymarketPro", Email: "p@p.com", Password: hashedP, Tier: "Silver", Role: models.RoleUser, IsActive: true, Points: 950, KycStatus: true},
			{Username: "NoobBettor", Email: "n@n.com", Password: hashedP, Tier: "Bronze", Role: models.RoleUser, IsActive: true, Points: 45, KycStatus: true},
			{Username: "QuantGuru", Email: "q@q.com", Password: hashedP, Tier: "Diamond", Role: models.RoleUser, IsActive: true, Points: 12400, KycStatus: true},
			{Username: "NewsJunkie", Email: "j@j.com", Password: hashedP, Tier: "Gold", Role: models.RoleUser, IsActive: true, Points: 2200, KycStatus: true},
			{Username: "DataNerd", Email: "d@d.com", Password: hashedP, Tier: "Silver", Role: models.RoleUser, IsActive: true, Points: 780, KycStatus: true},
		}
		for _, u := range users {
			DB.Create(&u)
		}
		log.Println("Seeded 4 admin/staff accounts and 8 regular users.")
	}


	DB.Model(&models.Market{}).Count(&count)

	if count > 0 {
		return // Already seeded
	}

	log.Println("Database is empty. Seeding initial markets...")

	// Resolution dates
	soon := time.Now().AddDate(0, 1, 0)
	midterm := time.Now().AddDate(0, 6, 0)
	longterm := time.Now().AddDate(1, 0, 0)

	markets := []models.Market{
		// --- POLITICS ---
		{
			Title:            "Will TVK win more than 15 seats in upcoming assembly elections?",
			Description:      "TVK (Tamilaga Vettri Kazhagam), led by Vijay, contests in the TN state assembly elections. Resolution based on official ECI results.",
			Category:         "Politics",
			YesPrice:         65, NoPrice: 35,
			Volume:           142500,
			ResolutionStatus: "Open",
			ResolutionSource: "Election Commission of India (eci.gov.in)",
			EndDate:          soon,
		},
		{
			Title:            "Will NDA win more than 350 seats in 2029 Lok Sabha?",
			Description:      "The National Democratic Alliance's seat count in the 2029 Indian General Elections. Resolution based on official ECI results.",
			Category:         "Politics",
			YesPrice:         40, NoPrice: 60,
			Volume:           500000,
			ResolutionStatus: "Open",
			ResolutionSource: "Election Commission of India (eci.gov.in)",
			EndDate:          longterm,
		},
		{
			Title:            "Will Donald Trump run for US President in 2028?",
			Description:      "Will Donald Trump officially file as a candidate for the 2028 US Presidential election?",
			Category:         "Politics",
			YesPrice:         15, NoPrice: 85,
			Volume:           1200000,
			ResolutionStatus: "Open",
			ResolutionSource: "FEC (fec.gov) official filing records",
			EndDate:          midterm,
		},
		{
			Title:            "Will Arvind Kejriwal become Delhi CM again?",
			Description:      "Will AAP's Arvind Kejriwal be sworn in as Chief Minister of Delhi in the next state election?",
			Category:         "Politics",
			YesPrice:         55, NoPrice: 45,
			Volume:           320000,
			ResolutionStatus: "Open",
			ResolutionSource: "Official Delhi Government announcement",
			EndDate:          soon,
		},
		{
			Title:            "Will UK call a snap general election before 2027?",
			Description:      "Will the UK Prime Minister dissolve Parliament and call an early general election before the scheduled 2027 date?",
			Category:         "Politics",
			YesPrice:         30, NoPrice: 70,
			Volume:           180000,
			ResolutionStatus: "Open",
			ResolutionSource: "UK Parliament official announcement",
			EndDate:          midterm,
		},
		// --- FINANCE ---
		{
			Title:            "Will RBI hike repo rate by 25bps in next MPC meeting?",
			Description:      "Will the Reserve Bank of India's Monetary Policy Committee vote to increase the repo rate by 25 basis points?",
			Category:         "Finance",
			YesPrice:         20, NoPrice: 80,
			Volume:           89200,
			ResolutionStatus: "Open",
			ResolutionSource: "RBI official MPC press release (rbi.org.in)",
			EndDate:          soon,
		},
		{
			Title:            "Will Bitcoin cross $100k by December 2026?",
			Description:      "Will Bitcoin (BTC) price reach or exceed $100,000 USD on any major exchange before December 31, 2026?",
			Category:         "Finance",
			YesPrice:         75, NoPrice: 25,
			Volume:           2100000,
			ResolutionStatus: "Open",
			ResolutionSource: "CoinGecko 24h average price",
			EndDate:          longterm,
		},
		{
			Title:            "Will Reliance Industries stock hit ₹3,500?",
			Description:      "Will RELIANCE.NSE close above ₹3,500 per share on the NSE at any point before the resolution date?",
			Category:         "Finance",
			YesPrice:         30, NoPrice: 70,
			Volume:           450000,
			ResolutionStatus: "Open",
			ResolutionSource: "NSE India official closing price (nseindia.com)",
			EndDate:          midterm,
		},
		{
			Title:            "Will US Fed cut interest rates by 50bps in next FOMC?",
			Description:      "Will the Federal Reserve cut its federal funds rate by 50 basis points at the next FOMC meeting?",
			Category:         "Finance",
			YesPrice:         45, NoPrice: 55,
			Volume:           890000,
			ResolutionStatus: "Open",
			ResolutionSource: "Federal Reserve official FOMC statement (federalreserve.gov)",
			EndDate:          soon,
		},
		{
			Title:            "Will Nifty 50 cross 30,000 by end of 2026?",
			Description:      "Will the BSE Nifty 50 index close above 30,000 points on any trading day before December 31, 2026?",
			Category:         "Finance",
			YesPrice:         60, NoPrice: 40,
			Volume:           650000,
			ResolutionStatus: "Open",
			ResolutionSource: "NSE India official index data",
			EndDate:          longterm,
		},
		// --- TECHNOLOGY ---
		{
			Title:            "Will ISRO successfully launch Gaganyaan in 2026?",
			Description:      "Will the Indian Space Research Organisation complete the crewed Gaganyaan mission before December 31, 2026?",
			Category:         "Technology",
			YesPrice:         88, NoPrice: 12,
			Volume:           45100,
			ResolutionStatus: "Open",
			ResolutionSource: "ISRO official press release (isro.gov.in)",
			EndDate:          longterm,
		},
		{
			Title:            "Will OpenAI release GPT-5 before Q4 2026?",
			Description:      "Will OpenAI officially release a model publicly branded as GPT-5 before October 1, 2026?",
			Category:         "Technology",
			YesPrice:         80, NoPrice: 20,
			Volume:           1500000,
			ResolutionStatus: "Open",
			ResolutionSource: "OpenAI official blog (openai.com/blog)",
			EndDate:          midterm,
		},
		{
			Title:            "Will Tesla achieve full self-driving approval in the US?",
			Description:      "Will Tesla's FSD (Full Self-Driving) system receive regulatory approval for fully driverless operation from NHTSA?",
			Category:         "Technology",
			YesPrice:         25, NoPrice: 75,
			Volume:           780000,
			ResolutionStatus: "Open",
			ResolutionSource: "NHTSA official announcement (nhtsa.gov)",
			EndDate:          longterm,
		},
		{
			Title:            "Will Apple announce a foldable iPhone in 2026?",
			Description:      "Will Apple officially announce a commercially available foldable iPhone product at any event before December 31, 2026?",
			Category:         "Technology",
			YesPrice:         10, NoPrice: 90,
			Volume:           520000,
			ResolutionStatus: "Open",
			ResolutionSource: "Apple official press release (apple.com/newsroom)",
			EndDate:          midterm,
		},
		{
			Title:            "Will Jio 6G launch in India before 2028?",
			Description:      "Will Reliance Jio commercially launch a 6G network service for consumers in India before January 1, 2028?",
			Category:         "Technology",
			YesPrice:         35, NoPrice: 65,
			Volume:           290000,
			ResolutionStatus: "Open",
			ResolutionSource: "TRAI and Reliance Jio official announcements",
			EndDate:          longterm,
		},
		// --- GLOBAL NEWS ---
		{
			Title:            "Will the UN broker a ceasefire in Eastern Europe by July 2026?",
			Description:      "Will a formal UN-brokered ceasefire agreement be signed and publicly announced for the Eastern European conflict before July 31, 2026?",
			Category:         "Global News",
			YesPrice:         20, NoPrice: 80,
			Volume:           950000,
			ResolutionStatus: "Open",
			ResolutionSource: "UN official press release (un.org)",
			EndDate:          soon,
		},
		{
			Title:            "Will WHO declare a new global health emergency in 2026?",
			Description:      "Will the World Health Organization declare a new Public Health Emergency of International Concern (PHEIC) in 2026?",
			Category:         "Global News",
			YesPrice:         15, NoPrice: 85,
			Volume:           410000,
			ResolutionStatus: "Open",
			ResolutionSource: "WHO official declaration (who.int)",
			EndDate:          longterm,
		},
		{
			Title:            "Will the Paris Agreement 2030 targets be officially delayed?",
			Description:      "Will a majority of G20 nations formally vote to extend Paris Agreement 2030 carbon reduction targets beyond 2035?",
			Category:         "Global News",
			YesPrice:         65, NoPrice: 35,
			Volume:           630000,
			ResolutionStatus: "Open",
			ResolutionSource: "UNFCCC official announcement (unfccc.int)",
			EndDate:          longterm,
		},
		{
			Title:            "Will India and China restore full diplomatic relations by 2027?",
			Description:      "Will India and China fully restore ambassador-level diplomatic relations and fully reopen the LAC to normal trade before 2027?",
			Category:         "Global News",
			YesPrice:         45, NoPrice: 55,
			Volume:           320000,
			ResolutionStatus: "Open",
			ResolutionSource: "MEA India official press release (mea.gov.in)",
			EndDate:          longterm,
		},
		{
			Title:            "Will Saudi Arabia join the BRICS bloc in 2026?",
			Description:      "Will Saudi Arabia formally complete the accession process and be admitted as a full member of BRICS in 2026?",
			Category:         "Global News",
			YesPrice:         55, NoPrice: 45,
			Volume:           480000,
			ResolutionStatus: "Open",
			ResolutionSource: "BRICS official summit declaration",
			EndDate:          midterm,
		},
	}

	for _, m := range markets {
		DB.Create(&m)
	}
	log.Println("Seeding complete! 20 markets across 4 categories loaded.")
}
