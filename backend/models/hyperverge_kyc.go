package models

import (
	"time"

	"gorm.io/gorm"
)

// HyperVergeKYC stores the entire lifecycle of a real KYC verification attempt.
type HyperVergeKYC struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"not null;index" json:"user_id"`
	User   User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`

	// e.g. "Started", "Pending", "DocumentsUploaded", "Verified", "Rejected", "Expired"
	Status string `gorm:"default:'Started';index" json:"status"`

	// Real-world auditing fields
	Provider          string `gorm:"default:'HyperVerge'" json:"provider"`
	ProviderReference string `gorm:"index" json:"provider_reference"` // HyperVerge transaction ID
	SessionID         string `gorm:"uniqueIndex;not null" json:"session_id"`
	WorkflowID        string `json:"workflow_id"`
	VerificationID    string `json:"verification_id"`

	// Extracted data (MASKED FOR COMPLIANCE)
	PANNumberMasked string `json:"pan_masked"`    // e.g., "ABCDE****F"
	AadhaarLast4    string `json:"aadhaar_last4"` // e.g., "1234"
	FullName        string `json:"full_name"`
	DOB             string `json:"dob"`
	Gender          string `json:"gender"`
	Address         string `json:"address"`

	// Document proofs
	DocumentFrontURL string `json:"document_front_url"`
	DocumentBackURL  string `json:"document_back_url"`
	SelfieURL        string `json:"selfie_url"`

	// Fraud / Verification metrics
	FaceMatchScore float64 `json:"face_match_score"`
	LivenessScore  float64 `json:"liveness_score"`

	VerificationResult string `json:"verification_result"` // e.g. "auto_approved", "needs_review", "rejected"
	FailureReason      string `json:"failure_reason"`      // Human-readable reason if rejected

	// For auditing webhooks
	WebhookPayload string `gorm:"type:text" json:"-"`

	VerifiedAt *time.Time     `json:"verified_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
