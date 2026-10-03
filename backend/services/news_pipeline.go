package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"html"
	"net/url"
	"os"
	"profhit-backend/config"
	"profhit-backend/models"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

var markup = regexp.MustCompile(`<[^>]*>`)

func cleanNewsText(raw string, max int) string {
	r := []rune(strings.Join(strings.Fields(html.UnescapeString(markup.ReplaceAllString(raw, " "))), " "))
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}
func canonicalNewsURL(raw string) (string, error) {
	if !PublicProviderURL(raw) {
		return "", errors.New("article requires public HTTPS source")
	}
	u, _ := url.Parse(raw)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	q := u.Query()
	for key := range q {
		if strings.HasPrefix(strings.ToLower(key), "utm_") || key == "fbclid" || key == "gclid" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func titleTokens(raw string) map[string]bool {
	tokens := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(raw), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(word)) >= 3 && !strings.Contains("|the|and|for|with|will|today|who|what|how|this|that|", "|"+word+"|") {
			tokens[word] = true
		}
	}
	return tokens
}
func SimilarEventTitles(a, b string) bool {
	left, right := titleTokens(a), titleTokens(b)
	if len(left) < 3 || len(right) < 3 {
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	overlap := 0
	for token := range left {
		if right[token] {
			overlap++
		}
	}
	union := len(left) + len(right) - overlap
	return float64(overlap)/float64(union) >= 0.72
}
func NormalizeNewsEvent(article Article, now time.Time) (models.NewsEvent, error) {
	var e models.NewsEvent
	published, err := parseNewsDate(article.PublishedAt)
	if err != nil || published.After(now) || published.Before(now.Add(-24*time.Hour)) {
		return e, errors.New("article is stale, undated or from the future")
	}
	canonical, err := canonicalNewsURL(article.URL)
	if err != nil {
		return e, err
	}
	title := cleanNewsText(article.Title, 300)
	if len([]rune(title)) < 10 {
		return e, errors.New("article title too short")
	}
	name := cleanNewsText(article.Source.Name, 120)
	if name == "" {
		u, _ := url.Parse(canonical)
		name = u.Hostname()
	}
	category := newsCategory(article.Category, title+" "+article.Description)
	entities := []string{}
	for word := range titleTokens(title) {
		entities = append(entities, word)
	}
	sort.Strings(entities)
	encoded, _ := json.Marshal(entities)
	e = models.NewsEvent{Title: title, Summary: cleanNewsText(article.Description, 2000), Category: category, Entities: string(encoded), Language: article.Language, SourceURL: canonical, SourceName: name, PublishedAt: published, DiscoveredAt: now.UTC(), Status: "Needs review"}
	e.Fingerprint = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(title)+"|"+published.Format("2006-01-02"))))
	return e, nil
}
func IngestNewsArticle(article Article, now time.Time) (*models.NewsEvent, bool, error) {
	normalized, err := NormalizeNewsEvent(article, now)
	if err != nil {
		return nil, false, err
	}
	provider := article.Provider
	if provider == "" {
		provider = "editorial"
	}
	identityRaw := article.ID
	if identityRaw == "" {
		identityRaw = normalized.SourceURL
	}
	identity := fmt.Sprintf("%x", sha256.Sum256([]byte(provider+"|"+identityRaw)))
	sourcePublishedAt, sourceURL, sourceName := normalized.PublishedAt, normalized.SourceURL, normalized.SourceName
	duplicate := false
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.NewsSource
		sourceErr := tx.Where("identity = ?", identity).First(&existing).Error
		if sourceErr == nil {
			duplicate = true
			return tx.First(&normalized, existing.EventID).Error
		}
		if !errors.Is(sourceErr, gorm.ErrRecordNotFound) {
			return sourceErr
		}
		var candidates []models.NewsEvent
		if err := tx.Where("published_at BETWEEN ? AND ?", normalized.PublishedAt.Add(-6*time.Hour), normalized.PublishedAt.Add(6*time.Hour)).Limit(500).Find(&candidates).Error; err != nil {
			return err
		}
		for _, candidate := range candidates {
			if candidate.SourceURL == normalized.SourceURL || candidate.Fingerprint == normalized.Fingerprint || SimilarEventTitles(candidate.Title, normalized.Title) {
				normalized = candidate
				duplicate = true
				break
			}
		}
		if !duplicate {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&normalized)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				duplicate = true
				if err := tx.Where("fingerprint = ?", normalized.Fingerprint).First(&normalized).Error; err != nil {
					return err
				}
			}
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.NewsSource{EventID: normalized.ID, Identity: identity, Provider: provider, ExternalID: article.ID, Name: sourceName, URL: sourceURL, PublishedAt: sourcePublishedAt}).Error
	})
	return &normalized, duplicate, err
}
func RefreshLiveNews(ctx context.Context, providers []NewsProvider, now time.Time) error {
	if config.DB == nil {
		return errors.New("news database unavailable")
	}
	initial := models.NewsIngestionState{ID: 1, Status: "Not configured"}
	if err := config.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; err != nil {
		return err
	}
	// New deployment configuration should not wait through an old missing-key backoff.
	// Failed configured providers still retain their quota and retry schedule.
	if len(providers) > 0 {
		if err := config.DB.Model(&models.NewsIngestionState{}).Where("id = 1 AND status = ? AND lease_until <= ?", "Not configured", now).Update("next_fetch_at", now).Error; err != nil {
			return err
		}
	}
	lease := now.Add(2 * time.Minute)
	claim := config.DB.Model(&models.NewsIngestionState{}).Where("id = 1 AND lease_until <= ? AND next_fetch_at <= ?", now, now).Updates(map[string]interface{}{"lease_until": lease, "last_attempt_at": now, "status": "Fetching"})
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected != 1 {
		return nil
	}
	var state models.NewsIngestionState
	if err := config.DB.First(&state, 1).Error; err != nil {
		return err
	}
	articles, provider, fetchErr := fetchProviders(ctx, providers)
	changes := map[string]interface{}{"lease_until": now, "next_fetch_at": now.Add(NewsRefreshInterval()), "provider": provider, "articles_fetched": len(articles), "duplicates_removed": 0, "events_detected": 0, "rejected": 0, "error": ""}
	if fetchErr != nil {
		failures := state.Failures + 1
		backoff := NewsRefreshInterval() * time.Duration(1<<min(failures-1, 4))
		changes["failures"] = failures
		changes["status"] = "Unavailable"
		if errors.Is(fetchErr, ErrNewsNotConfigured) {
			changes["status"] = "Not configured"
		}
		changes["error"] = fetchErr.Error()
		changes["next_fetch_at"] = now.Add(backoff)
		if err := config.DB.Model(&state).Updates(changes).Error; err != nil {
			return err
		}
		return fetchErr
	}
	duplicates, rejected, detected := 0, 0, 0
	generated := 0
	generator := ConfiguredQuestionGenerator()
	valid := []Article{}
	for _, article := range articles {
		event, duplicate, err := IngestNewsArticle(article, now)
		if err != nil {
			rejected++
			continue
		}
		valid = append(valid, article)
		if duplicate {
			duplicates++
		} else {
			detected++
		}
		if _, err := prepareEventDraft(*event, now); err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
			changes["status"] = "Failed"
			changes["error"] = "Could not persist editorial draft"
			_ = config.DB.Model(&state).Updates(changes).Error
			return err
		}
		if !duplicate && generator != nil && os.Getenv("NEWS_AI_ENABLED") == "true" && generated < envInt("NEWS_GENERATION_LIMIT", 5, 1, 50) && ctx.Err() == nil {
			generated++
			if err := GenerateEventPrediction(ctx, event.ID, generator, now); err != nil {
				_ = config.DB.Model(&models.NewsEvent{}).Where("id = ?", event.ID).Update("rejection_reason", err.Error()).Error
			}
		}
	}
	changes["status"] = "Healthy"
	changes["failures"] = 0
	changes["last_success_at"] = now
	changes["duplicates_removed"] = duplicates
	changes["events_detected"] = detected
	changes["rejected"] = rejected
	if err := config.DB.Model(&state).Updates(changes).Error; err != nil {
		return err
	}
	newsCacheMu.Lock()
	newsCache = valid
	newsCacheTime = now
	newsCacheMu.Unlock()
	BroadcastToAll("news_event_updated", map[string]interface{}{"events_detected": detected})
	return nil
}
func prepareEventDraft(event models.NewsEvent, now time.Time) (*models.Market, error) {
	key := fmt.Sprintf("news-event:%d", event.ID)
	published, discovered := event.PublishedAt, event.DiscoveredAt
	title := cleanNewsText("News candidate: "+event.Title, 200)
	cutoff := now.Add(24 * time.Hour)
	m := models.Market{Title: title, Description: event.Summary, Category: event.Category, Difficulty: "Easy", Payout: CategoryPayouts[event.Category][0], Options: `["Yes","No"]`, PredictionType: CategoryTypes[event.Category][0], ResolutionStatus: "Draft", Visibility: "Public", DailyKey: &key, NewsEventID: &event.ID, NewsURL: event.SourceURL, NewsSourceName: event.SourceName, NewsEventTitle: event.Title, NewsPublishedAt: &published, NewsDiscoveredAt: &discovered, LockTime: &cutoff, EndDate: cutoff}
	result := config.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&m)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrDuplicatedKey
	}
	return &m, nil
}

// Generator input is structured, source-linked and always subject to editorial review.
// Neither a news headline nor AI text is an outcome authority.
func ValidateNewsPrediction(event models.NewsEvent, m *models.Market, now time.Time) error {
	if event.PublishedAt.Before(now.Add(-24*time.Hour)) || event.PublishedAt.After(now) {
		return errors.New("news event is no longer fresh")
	}
	if m.LockTime == nil || !m.LockTime.After(now) || m.LockTime.After(now.Add(7*24*time.Hour)) {
		return errors.New("news prediction cutoff must be within the next seven days")
	}
	if len(strings.TrimSpace(m.Title)) < 15 || !strings.Contains(m.Title, "?") {
		return errors.New("provide a clear prediction question")
	}
	overlap := 0
	for token := range titleTokens(m.Title) {
		if titleTokens(event.Title + " " + event.Summary)[token] {
			overlap++
		}
	}
	if overlap < 2 {
		return errors.New("question must refer to the source event and its entities")
	}
	if m.Category != event.Category {
		return errors.New("question category must match reviewed event category")
	}
	return ConfigurePrediction(m)
}
