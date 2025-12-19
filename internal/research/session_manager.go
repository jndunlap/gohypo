package research

import (
	"context"
	"fmt"
	"time"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
)

// SessionManager manages research sessions using database persistence
type SessionManager struct {
	sessionRepo ports.SessionRepository
	userRepo    ports.UserRepository
}

// NewSessionManager creates a new session manager with database repositories
func NewSessionManager(sessionRepo ports.SessionRepository, userRepo ports.UserRepository) *SessionManager {
	return &SessionManager{
		sessionRepo: sessionRepo,
		userRepo:    userRepo,
	}
}

// CreateSession creates a new research session for the default user
func (sm *SessionManager) CreateSession(ctx context.Context, metadata map[string]interface{}) (*models.ResearchSession, error) {
	user, err := sm.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	return sm.sessionRepo.CreateSession(ctx, user.ID, metadata)
}

// GetSession retrieves a session by ID for the default user
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*models.ResearchSession, error) {
	user, err := sm.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	return sm.sessionRepo.GetSession(ctx, user.ID, sessionUUID)
}

// UpdateSessionProgress updates progress for a session
func (sm *SessionManager) UpdateSessionProgress(ctx context.Context, sessionID string, progress float64, currentHypothesis string) error {
	user, err := sm.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	return sm.sessionRepo.UpdateSessionProgress(ctx, user.ID, sessionUUID, progress, currentHypothesis)
}

// AddHypothesisToSession adds a completed hypothesis to a session (deprecated - use ResearchStorage instead)
func (sm *SessionManager) AddHypothesisToSession(ctx context.Context, sessionID string, result models.HypothesisResult) error {
	// This method is deprecated. Hypothesis persistence should be handled by ResearchStorage
	return fmt.Errorf("AddHypothesisToSession is deprecated - use ResearchStorage.SaveHypothesis instead")
}

// SetSessionState updates the state of a session
func (sm *SessionManager) SetSessionState(ctx context.Context, sessionID string, state models.SessionState) error {
	user, err := sm.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	return sm.sessionRepo.UpdateSessionState(ctx, user.ID, sessionUUID, state)
}

// SetSessionError sets an error state for a session
func (sm *SessionManager) SetSessionError(ctx context.Context, sessionID string, errMsg string) error {
	user, err := sm.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	return sm.sessionRepo.SetSessionError(ctx, user.ID, sessionUUID, errMsg)
}

// GetActiveSessions returns all sessions that are not complete or errored
func (sm *SessionManager) GetActiveSessions(ctx context.Context) ([]*models.ResearchSession, error) {
	user, err := sm.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	return sm.sessionRepo.GetActiveUserSessions(ctx, user.ID)
}

// GetSessionStatus returns the status of a session
func (sm *SessionManager) GetSessionStatus(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return session.GetStatus(), nil
}

// ListSessions returns all sessions for the default user (optionally filtered by state)
func (sm *SessionManager) ListSessions(ctx context.Context, state *models.SessionState) ([]*models.ResearchSession, error) {
	user, err := sm.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	// For now, get all sessions and filter by state in memory
	// This could be optimized with a database query if needed
	sessions, err := sm.sessionRepo.ListUserSessions(ctx, user.ID, 0) // 0 = no limit
	if err != nil {
		return nil, err
	}

	if state == nil {
		return sessions, nil
	}

	var filtered []*models.ResearchSession
	for _, session := range sessions {
		if session.State == *state {
			filtered = append(filtered, session)
		}
	}

	return filtered, nil
}

// CleanupOldSessions removes sessions older than the specified duration
// Note: In database-backed implementation, this could be implemented as a database cleanup task
func (sm *SessionManager) CleanupOldSessions(maxAge time.Duration) int {
	// For now, return 0 as database cleanup can be handled separately
	// TODO: Implement database-based cleanup if needed
	return 0
}


