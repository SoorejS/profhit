package services

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newsTestDB(t *testing.T) {
	t.Helper()
	old := config.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NowFunc: func() time.Time { return time.Now().UTC() }})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(1)
	require.NoError(t, config.Migrate(db))
	config.DB = db
	t.Cleanup(func() {
		config.DB = old
		sql.Close()
		newsCacheMu.Lock()
		newsCache = nil
		newsCacheTime = time.Time{}
		newsCacheMu.Unlock()
	})
}

type fixtureProvider struct {
	calls    atomic.Int32
	articles []Article
	err      error
}

func (p *fixtureProvider) Name() string { return "test-fixture" }
func (p *fixtureProvider) Fetch(context.Context) ([]Article, error) {
	p.calls.Add(1)
	return p.articles, p.err
}
func fixtureArticle(now time.Time) Article {
	return Article{ID: "fixture-1", Title: "Chennai rainfall forecast for tomorrow", Description: "Test fixture only: rain measurement at Chennai tomorrow.", URL: "https://news.example.test/chennai?utm_source=test", PublishedAt: now.Add(-time.Hour).Format(time.RFC3339), Source: ArticleSource{Name: "Test source"}, Provider: "test-fixture", Category: "Weather", Language: "en"}
}
func TestLiveNewsNormalizationAndMultipleSources(t *testing.T) {
	newsTestDB(t)
	now := time.Now().UTC()
	article := fixtureArticle(now)
	event, duplicate, err := IngestNewsArticle(article, now)
	require.NoError(t, err)
	require.False(t, duplicate)
	require.Equal(t, "https://news.example.test/chennai", event.SourceURL)
	other := article
	other.ID = "alternate"
	other.Provider = "test-other"
	other.URL = "https://another.example.test/story"
	other.PublishedAt = now.Add(-30 * time.Minute).Format(time.RFC3339)
	same, duplicate, err := IngestNewsArticle(other, now)
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, event.ID, same.ID)
	require.Equal(t, event.PublishedAt, same.PublishedAt)
	var sources int64
	require.NoError(t, config.DB.Model(&models.NewsSource{}).Count(&sources).Error)
	require.EqualValues(t, 2, sources)
	var alternate models.NewsSource
	require.NoError(t, config.DB.Where("external_id = ?", "alternate").First(&alternate).Error)
	published, err := parseNewsDate(other.PublishedAt)
	require.NoError(t, err)
	require.True(t, alternate.PublishedAt.Equal(published))
	article.PublishedAt = now.Add(-25 * time.Hour).Format(time.RFC3339)
	_, err = NormalizeNewsEvent(article, now)
	require.Error(t, err)
	article.PublishedAt = now.Add(time.Hour).Format(time.RFC3339)
	_, err = NormalizeNewsEvent(article, now)
	require.Error(t, err)
}
func TestNewsProviderSafety(t *testing.T) {
	for _, raw := range []string{"http://news.example.test/feed", "https://127.0.0.1/feed", "https://10.0.0.1/feed", "https://[::1]/feed", "https://localhost/feed", "https://user:secret@example.test/feed", "https://news.example.test:8443/feed"} {
		require.False(t, PublicProviderURL(raw), raw)
	}
	require.True(t, PublicProviderURL("https://gnews.io/api/v4/top-headlines"))
	require.True(t, SimilarEventTitles("Will India win today's Australia match?", "Who will win India Australia match?"))
	require.False(t, SimilarEventTitles("Mumbai rainfall forecast", "Chennai temperature forecast"))
}
func TestLeasedNewsRefreshDoesNotPublishOrRepeatProviderCalls(t *testing.T) {
	newsTestDB(t)
	now := time.Now().UTC()
	provider := &fixtureProvider{articles: []Article{fixtureArticle(now), fixtureArticle(now)}}
	var workers sync.WaitGroup
	errorsOut := make(chan error, 20)
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			errorsOut <- RefreshLiveNews(context.Background(), []NewsProvider{provider}, now)
		}()
	}
	workers.Wait()
	close(errorsOut)
	for err := range errorsOut {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, provider.calls.Load())
	var state models.NewsIngestionState
	require.NoError(t, config.DB.First(&state, 1).Error)
	require.Equal(t, "Healthy", state.Status)
	require.Equal(t, 2, state.ArticlesFetched)
	require.Equal(t, 1, state.DuplicatesRemoved)
	var drafts, live int64
	config.DB.Model(&models.Market{}).Where("resolution_status = ?", "Draft").Count(&drafts)
	config.DB.Model(&models.Market{}).Where("resolution_status = ?", "Live").Count(&live)
	require.EqualValues(t, 1, drafts)
	require.Zero(t, live)
}
func TestNewsFailureBackoffAndFallback(t *testing.T) {
	newsTestDB(t)
	now := time.Now().UTC()
	broken := &fixtureProvider{err: errors.New("fixture provider failed")}
	good := &fixtureProvider{articles: []Article{fixtureArticle(now)}}
	articles, name, err := fetchProviders(context.Background(), []NewsProvider{broken, good})
	require.NoError(t, err)
	require.Len(t, articles, 1)
	require.Equal(t, "test-fixture", name)
	require.Error(t, RefreshLiveNews(context.Background(), []NewsProvider{broken}, now))
	var state models.NewsIngestionState
	require.NoError(t, config.DB.First(&state, 1).Error)
	require.Equal(t, "Unavailable", state.Status)
	require.True(t, state.NextFetchAt.After(now))
	require.NoError(t, RefreshLiveNews(context.Background(), []NewsProvider{broken}, now.Add(time.Minute)))
	require.EqualValues(t, 2, broken.calls.Load())
}

type newsRoundTrip func(*http.Request) (*http.Response, error)

func (f newsRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGNewsBatchAndRSSNormalization(t *testing.T) {
	now := time.Now().UTC()
	var calls int
	client := &http.Client{Transport: newsRoundTrip(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "50", req.URL.Query().Get("max"))
		require.Equal(t, "test-only-key", req.URL.Query().Get("apikey"))
		data, _ := json.Marshal(GNewsResponse{Articles: []Article{fixtureArticle(now)}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})}
	articles, err := (GNewsProvider{Client: client, BaseURL: "https://gnews.io/api/v4/top-headlines", Key: "test-only-key", Categories: []string{"general", "sports", "business", "entertainment"}, PageSize: 50}).Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, articles, 4)
	require.Equal(t, 4, calls)
	rssClient := &http.Client{Transport: newsRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`<rss><channel><title>Fixture feed</title><item><guid>one</guid><title>Future Chennai rainfall forecast</title><description>Fixture only</description><link>https://news.example.test/rain</link><pubDate>` + now.Format(time.RFC1123Z) + `</pubDate></item><item><title>Undated article</title></item></channel></rss>`))}, nil
	})}
	rss, err := (RSSProvider{Client: rssClient, Feeds: []string{"https://news.example.test/rss"}}).Fetch(context.Background())
	require.NoError(t, err)
	require.Len(t, rss, 1)
	require.Equal(t, "Weather", rss[0].Category)
}
func TestStructuredCandidateValidationRejectsUnrelatedAndStaleQuestions(t *testing.T) {
	now := time.Now().UTC()
	event, err := NormalizeNewsEvent(fixtureArticle(now), now)
	require.NoError(t, err)
	candidate := PredictionCandidate{Eligible: true, Question: "Will rainfall be recorded in Chennai tomorrow?", Category: "Weather", Difficulty: "Easy", AnswerType: "binary", ClosingTime: now.Add(time.Hour).Format(time.RFC3339), ResolutionRule: "Measured rain in the published Chennai observation window; yes if greater than zero mm. Cancelled observation is not resolvable.", ResolutionSource: "https://openweathermap.org/", SourceURL: event.SourceURL, Options: []string{"Yes", "No"}, Confidence: 0.95}
	market, err := CandidateMarket(event, candidate, now)
	require.NoError(t, err)
	require.Equal(t, 20, market.Payout)
	require.Equal(t, "Draft", market.ResolutionStatus)
	candidate.SourceURL = "https://different.example.test/article"
	_, err = CandidateMarket(event, candidate, now)
	require.Error(t, err)
	candidate.SourceURL = event.SourceURL
	candidate.Confidence = 0.4
	_, err = CandidateMarket(event, candidate, now)
	require.Error(t, err)
	candidate.Confidence = 0.95
	candidate.Question = "Will celebrity popularity improve significantly?"
	_, err = CandidateMarket(event, candidate, now)
	require.Error(t, err)
	candidate.Question = market.Title
	event.PublishedAt = now.Add(-25 * time.Hour)
	_, err = CandidateMarket(event, candidate, now)
	require.Error(t, err)
}
