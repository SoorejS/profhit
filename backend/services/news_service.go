package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Article represents a news article from GNews
type Article struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
	URL         string `json:"url"`
	Image       string `json:"image"`
	PublishedAt string `json:"publishedAt"`
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

	// Return cache if it's less than 1 hour old
	if time.Since(newsCacheTime) < 1*time.Hour && len(newsCache) > 0 {
		return newsCache, nil
	}

	apiKey := os.Getenv("NEWS_API_KEY")
	if apiKey == "" {
		// Fallback mock data if API key isn't provided yet
		return []Article{
			{
				Title:       "Setup Required: Add NEWS_API_KEY",
				Description: "To view real live news, add your GNews API key to the environment variables.",
				URL:         "#",
			},
		}, nil
	}

	// Fetch top headlines (general/world)
	url := fmt.Sprintf("https://gnews.io/api/v4/top-headlines?category=general&lang=en&max=5&apikey=%s", apiKey)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("news API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result GNewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse news JSON: %v", err)
	}

	// Update cache
	newsCache = result.Articles
	newsCacheTime = time.Now()

	log.Printf("[News] Fetched %d new articles from API", len(newsCache))
	return newsCache, nil
}
