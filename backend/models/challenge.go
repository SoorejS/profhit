package models

import (
	"time"
	"gorm.io/gorm"
)

// WeeklyChallenge represents a time-bound challenge with a reward pool.
type WeeklyChallenge struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"not null" json:"title"`
	Description string         `json:"description"`
	RewardPool  int            `gorm:"not null" json:"reward_pool"`
	StartDate   time.Time      `gorm:"not null;index" json:"start_date"`
	EndDate     time.Time      `gorm:"not null;index" json:"end_date"`
	Status      string         `gorm:"default:'Active';index" json:"status"` // Active, Completed
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// ChallengeParticipant tracks a user's progress in a specific challenge.
type ChallengeParticipant struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ChallengeID uint           `gorm:"not null;index" json:"challenge_id"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	Score       int            `gorm:"default:0" json:"score"` // e.g., points earned or wins
	Rank        int            `json:"rank"`
	RewardWon   int            `json:"reward_won"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
