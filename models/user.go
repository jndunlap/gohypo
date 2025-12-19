package models

import (
	"time"

	"github.com/google/uuid"
)

// SessionState represents the current state of a research session
type SessionState string

const (
	SessionStateIdle       SessionState = "idle"
	SessionStateAnalyzing  SessionState = "analyzing"
	SessionStateValidating SessionState = "validating"
	SessionStateComplete   SessionState = "complete"
	SessionStateError      SessionState = "error"
)

// User represents a system user
type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Username  string    `json:"username" db:"username"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// UserHypothesisStats represents statistics for a user's hypotheses
type UserHypothesisStats struct {
	TotalHypotheses    int        `json:"total_hypotheses"`
	ValidatedCount     int        `json:"validated_count"`
	RejectedCount      int        `json:"rejected_count"`
	ValidationRate     float64    `json:"validation_rate"`
	EarliestHypothesis *time.Time `json:"earliest_hypothesis,omitempty"`
	LatestHypothesis   *time.Time `json:"latest_hypothesis,omitempty"`
}
