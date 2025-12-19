package ports

import (
	"context"

	"gohypo/models"

	"github.com/google/uuid"
)

// SessionRepository defines the interface for session data operations
type SessionRepository interface {
	// CreateSession creates a new research session for a user
	CreateSession(ctx context.Context, userID uuid.UUID, metadata map[string]interface{}) (*models.ResearchSession, error)

	// GetSession retrieves a session by user ID and session ID
	GetSession(ctx context.Context, userID, sessionID uuid.UUID) (*models.ResearchSession, error)

	// UpdateSessionProgress updates the progress of a session
	UpdateSessionProgress(ctx context.Context, userID, sessionID uuid.UUID, progress float64, currentHypothesis string) error

	// UpdateSessionState updates the state of a session
	UpdateSessionState(ctx context.Context, userID, sessionID uuid.UUID, state models.SessionState) error

	// ListUserSessions returns sessions for a user, optionally limited
	ListUserSessions(ctx context.Context, userID uuid.UUID, limit int) ([]*models.ResearchSession, error)

	// GetActiveUserSessions returns only active (non-complete/error) sessions for a user
	GetActiveUserSessions(ctx context.Context, userID uuid.UUID) ([]*models.ResearchSession, error)

	// SetSessionError sets an error state for a session
	SetSessionError(ctx context.Context, userID, sessionID uuid.UUID, errorMsg string) error
}
