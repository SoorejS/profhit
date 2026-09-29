package models

import "time"

type Notification struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"-"`
	Key       string     `gorm:"uniqueIndex;not null" json:"-"`
	Message   string     `gorm:"not null" json:"message"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}
