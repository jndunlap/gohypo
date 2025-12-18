package research

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionManager manages multiple research sessions
type SessionManager struct {
	sessions map[string]*ResearchSession
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ResearchSession),
	}
}

// CreateSession creates a new research session with the given metadata
func (sm *SessionManager) CreateSession(metadata map[string]interface{}) *ResearchSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := uuid.New().String()
	session := NewResearchSession(sessionID, metadata)
	sm.sessions[sessionID] = session

	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*ResearchSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// UpdateSessionProgress updates progress for a session
func (sm *SessionManager) UpdateSessionProgress(sessionID string, progress float64, currentHypothesis string) error {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.UpdateProgress(progress, currentHypothesis)
	return nil
}

// AddHypothesisToSession adds a completed hypothesis to a session
func (sm *SessionManager) AddHypothesisToSession(sessionID string, result HypothesisResult) error {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.AddHypothesis(result)
	return nil
}

// SetSessionState updates the state of a session
func (sm *SessionManager) SetSessionState(sessionID string, state SessionState) error {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.SetState(state)
	return nil
}

// SetSessionError sets an error state for a session
func (sm *SessionManager) SetSessionError(sessionID string, errMsg string) error {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.SetError(errMsg)
	return nil
}

// GetActiveSessions returns all sessions that are not complete or errored
func (sm *SessionManager) GetActiveSessions() []*ResearchSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var active []*ResearchSession
	for _, session := range sm.sessions {
		if session.State != SessionStateComplete && session.State != SessionStateError {
			active = append(active, session)
		}
	}

	return active
}

// GetSessionStatus returns the status of a session
func (sm *SessionManager) GetSessionStatus(sessionID string) (map[string]interface{}, error) {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.GetStatus(), nil
}

// ListSessions returns all sessions (optionally filtered by state)
func (sm *SessionManager) ListSessions(state *SessionState) []*ResearchSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sessions []*ResearchSession
	for _, session := range sm.sessions {
		if state == nil || session.State == *state {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// CleanupOldSessions removes sessions older than the specified duration
func (sm *SessionManager) CleanupOldSessions(maxAge time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, session := range sm.sessions {
		if session.StartedAt.Before(cutoff) &&
			(session.State == SessionStateComplete || session.State == SessionStateError) {
			delete(sm.sessions, id)
			removed++
		}
	}

	return removed
}
