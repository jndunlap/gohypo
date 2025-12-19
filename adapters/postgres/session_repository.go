package postgres

import (
	"context"
	"time"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// SessionRepositoryImpl implements SessionRepository for PostgreSQL
type SessionRepositoryImpl struct {
	db *sqlx.DB
}

// NewSessionRepository creates a new PostgreSQL session repository
func NewSessionRepository(db *sqlx.DB) ports.SessionRepository {
	return &SessionRepositoryImpl{db: db}
}

// CreateSession creates a new research session for a user
func (r *SessionRepositoryImpl) CreateSession(ctx context.Context, userID uuid.UUID, metadata map[string]interface{}) (*models.ResearchSession, error) {
	sessionID := uuid.New()
	session := models.NewResearchSession(sessionID, userID, metadata)

	// JSONBMap implements driver.Valuer, so it will be automatically converted
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO research_sessions (id, user_id, state, progress, current_hypothesis, started_at, metadata, created_at, updated_at, title)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULL)
	`, session.ID, session.UserID, session.State, session.Progress, session.CurrentHypothesis, session.StartedAt, session.Metadata, session.CreatedAt, session.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetSession retrieves a session by user ID and session ID
func (r *SessionRepositoryImpl) GetSession(ctx context.Context, userID, sessionID uuid.UUID) (*models.ResearchSession, error) {
	var session models.ResearchSession
	err := r.db.GetContext(ctx, &session, `
		SELECT id, user_id, state, progress, current_hypothesis, started_at, completed_at, error_message, metadata, created_at, updated_at
		FROM research_sessions
		WHERE user_id = $1 AND id = $2
	`, userID, sessionID)

	if err != nil {
		return nil, err
	}

	// Initialize the mutex for thread safety
	session.CompletedHypotheses = make([]models.HypothesisResult, 0) // Not stored in DB

	return &session, nil
}

// UpdateSessionProgress updates the progress of a session
func (r *SessionRepositoryImpl) UpdateSessionProgress(ctx context.Context, userID, sessionID uuid.UUID, progress float64, currentHypothesis string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE research_sessions
		SET progress = $3, current_hypothesis = $4, updated_at = NOW()
		WHERE user_id = $1 AND id = $2
	`, userID, sessionID, progress, currentHypothesis)
	return err
}

// UpdateSessionState updates the state of a session
func (r *SessionRepositoryImpl) UpdateSessionState(ctx context.Context, userID, sessionID uuid.UUID, state models.SessionState) error {
	var completedAt interface{}
	if state == models.SessionStateComplete || state == models.SessionStateError {
		completedAt = time.Now()
	} else {
		completedAt = nil
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE research_sessions
		SET state = $3, completed_at = $4, updated_at = NOW()
		WHERE user_id = $1 AND id = $2
	`, userID, sessionID, state, completedAt)
	return err
}

// ListUserSessions returns sessions for a user, optionally limited
func (r *SessionRepositoryImpl) ListUserSessions(ctx context.Context, userID uuid.UUID, limit int) ([]*models.ResearchSession, error) {
	query := `
		SELECT id, user_id, state, progress, current_hypothesis, started_at, completed_at, error_message, metadata, created_at, updated_at
		FROM research_sessions
		WHERE user_id = $1
		ORDER BY started_at DESC
	`

	args := []interface{}{userID}
	if limit > 0 {
		query += " LIMIT $2"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.ResearchSession
	for rows.Next() {
		var session models.ResearchSession
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.State,
			&session.Progress,
			&session.CurrentHypothesis,
			&session.StartedAt,
			&session.CompletedAt,
			&session.Error,
			&session.Metadata,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		session.CompletedHypotheses = make([]models.HypothesisResult, 0)
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// GetActiveUserSessions returns only active (non-complete/error) sessions for a user
func (r *SessionRepositoryImpl) GetActiveUserSessions(ctx context.Context, userID uuid.UUID) ([]*models.ResearchSession, error) {
	// Use GetContext for each session to ensure proper scanning, or use rows manually
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, state, progress, current_hypothesis, started_at, completed_at, error_message, metadata, created_at, updated_at
		FROM research_sessions
		WHERE user_id = $1 AND state NOT IN ('complete', 'error')
		ORDER BY started_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.ResearchSession
	for rows.Next() {
		var session models.ResearchSession
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.State,
			&session.Progress,
			&session.CurrentHypothesis,
			&session.StartedAt,
			&session.CompletedAt,
			&session.Error,
			&session.Metadata,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		session.CompletedHypotheses = make([]models.HypothesisResult, 0)
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// SetSessionError sets an error state for a session
func (r *SessionRepositoryImpl) SetSessionError(ctx context.Context, userID, sessionID uuid.UUID, errorMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE research_sessions
		SET state = 'error', error_message = $3, completed_at = NOW(), updated_at = NOW()
		WHERE user_id = $1 AND id = $2
	`, userID, sessionID, errorMsg)
	return err
}
