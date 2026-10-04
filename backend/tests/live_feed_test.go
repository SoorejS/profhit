package tests

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"profhit-backend/config"
	"profhit-backend/controllers"
	"profhit-backend/models"
	"profhit-backend/services"
	"testing"
	"time"
)

func TestLiveFeedCountsPlayableMarketsNotArticlesOrFixtures(t *testing.T) {
	setupTestDB()
	now := time.Now().UTC()
	event := models.NewsEvent{Fingerprint: "isolated-fixture", Title: "Isolated test event", Category: "Weather", SourceURL: "https://news.example.test/article", SourceName: "Fixture only", PublishedAt: now.Add(-time.Hour), DiscoveredAt: now, Status: "Needs review"}
	for i := 0; i < 55; i++ {
		event.ID = 0
		event.Fingerprint = fmt.Sprintf("isolated-fixture-%d", i)
		require.NoError(t, config.DB.Create(&event).Error)
		m := compliantTestMarket("Weather", "Easy", now.Add(time.Hour))
		m.Title = fmt.Sprintf("Isolated test prediction %d?", i)
		m.NewsEventID = &event.ID
		m.NewsURL = event.SourceURL
		m.NewsSourceName = event.SourceName
		m.NewsEventTitle = event.Title
		m.NewsPublishedAt = &event.PublishedAt
		m.EditorialReviewedBy = 1
		m.NewsDiscoveredAt = &event.DiscoveredAt
		switch i {
		case 50:
			m.IsDemo = true
		case 51:
			old := now.Add(-25 * time.Hour)
			m.NewsPublishedAt = &old
		case 52:
			m.ResolutionStatus = "Draft"
		case 53:
			m.ResolutionStatus = "Locked"
		case 54:
			m.ResolutionRule = ""
		}
		require.NoError(t, config.DB.Create(&m).Error)
	}
	response := request(t, controllers.LiveNewsFeed, 0, "", nil)
	require.Equal(t, 200, response.Code, response.Body.String())
	var data struct {
		Playable  int             `json:"playable_count"`
		Events    int             `json:"current_event_count"`
		Items     []models.Market `json:"items"`
		TargetMet bool            `json:"target_met"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &data))
	require.Equal(t, 50, data.Playable)
	require.Equal(t, 55, data.Events)
	require.Len(t, data.Items, 50)
	require.True(t, data.TargetMet)
}
func TestDuplicateProposalShowsExistingMarket(t *testing.T) {
	setupTestDB()
	user := testUser(t, "news_creator", 0)
	cutoff := time.Now().UTC().Add(time.Hour)
	m := compliantTestMarket("Sports", "Easy", cutoff)
	m.Title = "Will India win the Australia match?"
	require.NoError(t, config.DB.Create(&m).Error)
	proposed := compliantTestMarket("Sports", "Easy", cutoff)
	proposed.Title = "Who will win India Australia match?"
	body, err := json.Marshal(proposed)
	require.NoError(t, err)
	response := request(t, controllers.ProposeMarket, user.ID, string(body), nil)
	require.Equal(t, 409, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "existing_market_id")
	require.NoError(t, services.ConfigurePrediction(&proposed))
}
