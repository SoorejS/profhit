package services

import (
	"errors"
	"profhit-backend/config"
	"profhit-backend/models"
	"sync"
	"time"
)

var ErrNewsNotConfigured = errors.New("news provider is not configured")

// Article represents a news article from GNews
type Article struct {
	ID          string        `json:"id"`
	Source      ArticleSource `json:"source"`
	Provider    string        `json:"provider"`
	Category    string        `json:"category"`
	Language    string        `json:"lang"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Content     string        `json:"content"`
	URL         string        `json:"url"`
	Image       string        `json:"image"`
	PublishedAt string        `json:"publishedAt"`
}

type ArticleSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type GNewsResponse struct {
	TotalArticles int       `json:"totalArticles"`
	Articles      []Article `json:"articles"`
}

var (
	newsCache     []Article
	newsCacheTime time.Time
	newsCacheMu   sync.Mutex
)

// GetTrendingNews fetches the top breaking news. It uses an in-memory cache
// valid for 1 hour to prevent exhausting the 100 requests/day GNews limit.
func GetTrendingNews() ([]Article, error) {
	newsCacheMu.Lock()
	defer newsCacheMu.Unlock()

	if len(ConfiguredNewsProviders()) == 0 {
		return nil, ErrNewsNotConfigured
	}
	// Persisted articles remain visible across stateless Vercel instances.
	var events []models.NewsEvent
	now := time.Now().UTC()
	if err := config.DB.Where("published_at BETWEEN ? AND ?", now.Add(-24*time.Hour), now).Order("published_at DESC, id DESC").Limit(40).Find(&events).Error; err != nil {
		return nil, errors.New("news storage unavailable")
	}
	articles := make([]Article, 0, len(events))
	for _, event := range events {
		articles = append(articles, Article{Title: event.Title, Description: event.Summary, URL: event.SourceURL, PublishedAt: event.PublishedAt.Format(time.RFC3339), Source: ArticleSource{Name: event.SourceName}})
	}
	return articles, nil
}
