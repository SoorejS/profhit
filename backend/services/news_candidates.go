package services

import (
	"crypto/sha256"
	"fmt"
	"gorm.io/gorm/clause"
	"net/url"
	"profhit-backend/config"
	"profhit-backend/models"
	"strings"
	"time"
)

// News suggests an editorial draft; it never invents a measurable outcome or settles coins.
func PrepareNewsCandidate(article Article, now time.Time) (*models.Market, error) {
	published, err := time.Parse(time.RFC3339, article.PublishedAt)
	u, urlErr := url.Parse(article.URL)
	if err != nil || urlErr != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || published.After(now) || published.Before(now.Add(-24*time.Hour)) || strings.TrimSpace(article.Title) == "" {
		return nil, fmt.Errorf("candidate requires a fresh dated HTTPS article")
	}
	key := fmt.Sprintf("news:%x", sha256.Sum256([]byte(article.URL)))
	title := []rune("News candidate: " + article.Title)
	if len(title) > 200 {
		title = title[:200]
	}
	lock := now.Add(24 * time.Hour)
	m := models.Market{Title: string(title), Description: article.Description, Category: "Wild Card", Difficulty: "Easy", Payout: 40, Options: `["Yes","No"]`, PredictionType: "binary", ResolutionStatus: "Draft", DailyKey: &key, Visibility: "Public", NewsURL: article.URL, NewsPublishedAt: &published, LockTime: &lock, EndDate: lock}
	result := config.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&m)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("article already has an editorial candidate")
	}
	return &m, nil
}
