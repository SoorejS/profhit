package tests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"
)

// These tests require a dedicated disposable database, never application data.
type pgNewsFixture struct {
	calls   atomic.Int32
	article services.Article
}

func (p *pgNewsFixture) Name() string { return "isolated-test-fixture" }
func (p *pgNewsFixture) Fetch(context.Context) ([]services.Article, error) {
	p.calls.Add(1)
	return []services.Article{p.article}, nil
}

func TestPostgresNewsLeaseAndProposalDeduplication(t *testing.T) {
	getPostgresDB(t)
	now := time.Now().UTC()
	require.NoError(t, config.DB.Where("id = ?", 1).Delete(&models.NewsIngestionState{}).Error)
	provider := &pgNewsFixture{article: services.Article{
		ID: fmt.Sprintf("isolated-%d", now.UnixNano()), Provider: "isolated-test-fixture",
		Title:       fmt.Sprintf("Disposable PostgreSQL rainfall audit event %d", now.UnixNano()),
		URL:         fmt.Sprintf("https://fixture.example.test/%d", now.UnixNano()),
		PublishedAt: now.Add(-time.Hour).Format(time.RFC3339), Category: "Weather",
		Source: services.ArticleSource{Name: "Isolated test fixture"},
	}}
	var workers sync.WaitGroup
	errorsOut := make(chan error, 20)
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			errorsOut <- services.RefreshLiveNews(context.Background(), []services.NewsProvider{provider}, now)
		}()
	}
	workers.Wait()
	close(errorsOut)
	for err := range errorsOut {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, provider.calls.Load())
	var event models.NewsEvent
	require.NoError(t, config.DB.Where("title = ?", provider.article.Title).First(&event).Error)
	var linked int64
	require.NoError(t, config.DB.Model(&models.Market{}).Where("news_event_id = ?", event.ID).Count(&linked).Error)
	require.EqualValues(t, 1, linked)

	cutoff := now.Add(2 * time.Hour)
	title := fmt.Sprintf("Will disposable audit event %d finish above threshold?", now.UnixNano())
	var accepted, duplicates, failed atomic.Int32
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			market := compliantTestMarket("Weather", "Easy", cutoff)
			market.Title = title
			err := services.CreateUniqueMarket(config.DB, &market)
			var duplicate *services.DuplicateMarketError
			if err == nil {
				accepted.Add(1)
			} else if errors.As(err, &duplicate) {
				duplicates.Add(1)
			} else {
				failed.Add(1)
			}
		}()
	}
	workers.Wait()
	require.EqualValues(t, 0, failed.Load())
	require.EqualValues(t, 1, accepted.Load())
	require.EqualValues(t, 19, duplicates.Load())
}
