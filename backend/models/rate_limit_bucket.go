package models

import "time"

// RateLimitBucket preserves limits across stateless function replicas.
type RateLimitBucket struct {
	Key       string    `gorm:"primaryKey;size:64"`
	Requests  int       `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
}
