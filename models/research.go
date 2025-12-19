package models

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JSONBMap is a custom type for PostgreSQL JSONB columns that maps to map[string]interface{}
type JSONBMap map[string]interface{}

// Value implements driver.Valuer interface
func (j JSONBMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner interface
func (j *JSONBMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONBMap)
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*j = make(JSONBMap)
		return nil
	}

	if len(bytes) == 0 {
		*j = make(JSONBMap)
		return nil
	}

	result := make(JSONBMap)
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	*j = result
	return nil
}

// ResearchSession manages the lifecycle of a research generation session
type ResearchSession struct {
	ID                  uuid.UUID          `json:"id" db:"id"`
	UserID              uuid.UUID          `json:"user_id" db:"user_id"`
	State               SessionState       `json:"state" db:"state"`
	Progress            float64            `json:"progress" db:"progress"`
	CurrentHypothesis   string             `json:"current_hypothesis" db:"current_hypothesis"`
	CompletedHypotheses []HypothesisResult `json:"completed_hypotheses"` // Not stored in DB
	StartedAt           time.Time          `json:"started_at" db:"started_at"`
	CompletedAt         *time.Time         `json:"completed_at,omitempty" db:"completed_at"`
	Error               sql.NullString     `json:"error,omitempty" db:"error_message"`
	Metadata            JSONBMap           `json:"metadata" db:"metadata"`
	CreatedAt           time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at" db:"updated_at"`
	mu                  sync.RWMutex
}

// NewResearchSession creates a new research session
func NewResearchSession(id uuid.UUID, userID uuid.UUID, metadata map[string]interface{}) *ResearchSession {
	now := time.Now()
	jsonbMetadata := JSONBMap(metadata)
	if jsonbMetadata == nil {
		jsonbMetadata = make(JSONBMap)
	}
	return &ResearchSession{
		ID:                  id,
		UserID:              userID,
		State:               SessionStateIdle,
		Progress:            0.0,
		CurrentHypothesis:   "",
		CompletedHypotheses: make([]HypothesisResult, 0),
		StartedAt:           now,
		CreatedAt:           now,
		UpdatedAt:           now,
		Metadata:            jsonbMetadata,
	}
}

// UpdateProgress updates the session progress and current hypothesis
func (s *ResearchSession) UpdateProgress(progress float64, currentHypothesis string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Progress = progress
	s.CurrentHypothesis = currentHypothesis
}

// AddHypothesis adds a completed hypothesis to the session
func (s *ResearchSession) AddHypothesis(result HypothesisResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.CompletedHypotheses = append(s.CompletedHypotheses, result)
}

// SetState updates the session state
func (s *ResearchSession) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.State = state
	if state == SessionStateComplete || state == SessionStateError {
		now := time.Now()
		s.CompletedAt = &now
	}
}

// SetError sets an error state with message
func (s *ResearchSession) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.State = SessionStateError
	s.Error = sql.NullString{String: err, Valid: err != ""}
	now := time.Now()
	s.CompletedAt = &now
}

// GetStatus returns a snapshot of the current session status
func (s *ResearchSession) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	errorMsg := ""
	if s.Error.Valid {
		errorMsg = s.Error.String
	}
	return map[string]interface{}{
		"id":                 s.ID,
		"state":              s.State,
		"progress":           s.Progress,
		"current_hypothesis": s.CurrentHypothesis,
		"completed_count":    len(s.CompletedHypotheses),
		"started_at":         s.StartedAt,
		"completed_at":       s.CompletedAt,
		"error":              errorMsg,
	}
}
