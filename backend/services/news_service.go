package services

import (
	"errors"
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
	if time.Since(newsCacheTime) < NewsRefreshInterval() && len(newsCache) > 0 {
		return append([]Article(nil), newsCache...), nil
	}
	// Only the leased background worker consumes provider quota.
	return nil, errors.New("news refresh pending or provider cache expired")
}
