package models

import "time"

// Only hashes are persisted, never bearer credentials.
type RevokedToken struct {
	Hash      string    `gorm:"primaryKey;size:64"`
	ExpiresAt time.Time `gorm:"index;not null"`
}
