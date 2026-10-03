package models

import "time"

// NewsEvent is provenance for an existing Market, never a second prediction system.
type NewsEvent struct {
	ID              uint         `gorm:"primaryKey" json:"id"`
	Fingerprint     string       `gorm:"uniqueIndex" json:"-"`
	Title           string       `json:"title"`
	Summary         string       `json:"summary"`
	Category        string       `gorm:"index" json:"category"`
	Entities        string       `json:"entities"`
	Language        string       `json:"language"`
	SourceURL       string       `json:"source_url"`
	SourceName      string       `json:"source_name"`
	PublishedAt     time.Time    `gorm:"index" json:"published_at"`
	DiscoveredAt    time.Time    `json:"discovered_at"`
	Status          string       `gorm:"index" json:"status"`
	RejectionReason string       `json:"rejection_reason"`
	Sources         []NewsSource `gorm:"foreignKey:EventID" json:"sources"`
}

type NewsSource struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	EventID     uint      `gorm:"index;not null" json:"event_id"`
	Identity    string    `gorm:"uniqueIndex" json:"-"`
	Provider    string    `json:"provider"`
	ExternalID  string    `json:"external_id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

// A database lease avoids simultaneous ingestion across API replicas.
type NewsIngestionState struct {
	ID                uint       `gorm:"primaryKey" json:"-"`
	LeaseUntil        time.Time  `json:"-"`
	NextFetchAt       time.Time  `json:"next_fetch_at"`
	LastAttemptAt     *time.Time `json:"last_attempt_at"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider"`
	Error             string     `json:"error"`
	Failures          int        `json:"consecutive_failures"`
	ArticlesFetched   int        `json:"articles_fetched"`
	DuplicatesRemoved int        `json:"duplicates_removed"`
	EventsDetected    int        `json:"events_detected"`
	Rejected          int        `json:"articles_rejected"`
}
