package research

import (
	"sync"
	"time"

	"gohypo/domain/discovery"
	"gohypo/models"
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

// ResearchSession manages the lifecycle of a research generation session
type ResearchSession struct {
	ID                  string                 `json:"id"`
	State               SessionState           `json:"state"`
	Progress            float64                `json:"progress"` // 0.0 - 100.0
	CurrentHypothesis   string                 `json:"current_hypothesis"`
	CompletedHypotheses []HypothesisResult     `json:"completed_hypotheses"`
	StartedAt           time.Time              `json:"started_at"`
	CompletedAt         *time.Time             `json:"completed_at,omitempty"`
	Error               string                 `json:"error,omitempty"`
	Metadata            map[string]interface{} `json:"metadata"`
	mu                  sync.RWMutex
}

// HypothesisResult represents a completed hypothesis with validation results
type HypothesisResult struct {
	ID                 string                     `json:"id"`
	BusinessHypothesis string                     `json:"business_hypothesis"`
	ScienceHypothesis  string                     `json:"science_hypothesis"`
	NullCase           string                     `json:"null_case"`
	ValidationMethods  []models.ValidationMethod  `json:"validation_methods"`
	RefereeGates       models.RefereeGates        `json:"referee_gates"`
	Validated          bool                       `json:"validated"`
	Rejected           bool                       `json:"rejected"`
	EffectSize         float64                    `json:"effect_size"`
	PValue             float64                    `json:"p_value"`
	SampleSize         int                        `json:"sample_size"`
	CreatedAt          time.Time                  `json:"created_at"`
	ProcessingTime     time.Duration              `json:"processing_time"`
	Metadata           map[string]interface{}     `json:"metadata"`
	DiscoveryBriefs    []discovery.DiscoveryBrief `json:"discovery_briefs,omitempty"`
	// Legacy fields for backward compatibility
	Claim              string                    `json:"claim,omitempty"`
	LogicType          string                    `json:"logic_type,omitempty"`
	ValidationStrategy models.ValidationStrategy `json:"validation_strategy,omitempty"`
}

// NewResearchSession creates a new research session
func NewResearchSession(id string, metadata map[string]interface{}) *ResearchSession {
	return &ResearchSession{
		ID:                  id,
		State:               SessionStateIdle,
		Progress:            0.0,
		CurrentHypothesis:   "",
		CompletedHypotheses: make([]HypothesisResult, 0),
		StartedAt:           time.Now(),
		Metadata:            metadata,
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
	s.Error = err
	now := time.Now()
	s.CompletedAt = &now
}

// GetStatus returns a snapshot of the current session status
func (s *ResearchSession) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"id":                 s.ID,
		"state":              s.State,
		"progress":           s.Progress,
		"current_hypothesis": s.CurrentHypothesis,
		"completed_count":    len(s.CompletedHypotheses),
		"started_at":         s.StartedAt,
		"completed_at":       s.CompletedAt,
		"error":              s.Error,
	}
}
