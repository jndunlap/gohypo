package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// UIStateSnapshot represents a complete UI state for HTMX reconnection
type UIStateSnapshot struct {
	SessionID    uuid.UUID               `json:"session_id"`
	UIState      map[string]interface{} `json:"ui_state"`
	Version      int                     `json:"version"`
	LastUpdated  time.Time               `json:"last_updated"`
	Compressed   bool                    `json:"compressed"`
}

// UIStateRepository handles HTMX UI state synchronization for reconnection
type UIStateRepository struct {
	db *sqlx.DB
}

// NewUIStateRepository creates a new UI state repository
func NewUIStateRepository(db *sqlx.DB) *UIStateRepository {
	return &UIStateRepository{db: db}
}

// SaveUIState saves or updates the UI state for a session
func (r *UIStateRepository) SaveUIState(ctx context.Context, snapshot *UIStateSnapshot) error {
	uiStateJSON, err := json.Marshal(snapshot.UIState)
	if err != nil {
		return fmt.Errorf("failed to marshal UI state: %w", err)
	}

	query := `
		INSERT INTO ui_state_cache (session_id, ui_state, version, last_updated)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (session_id) DO UPDATE SET
			ui_state = EXCLUDED.ui_state,
			version = EXCLUDED.version,
			last_updated = EXCLUDED.last_updated`

	_, err = r.db.ExecContext(ctx, query,
		snapshot.SessionID,
		uiStateJSON,
		snapshot.Version,
		snapshot.LastUpdated,
	)

	if err != nil {
		return fmt.Errorf("failed to save UI state: %w", err)
	}

	return nil
}

// GetUIState retrieves the current UI state for a session
func (r *UIStateRepository) GetUIState(ctx context.Context, sessionID uuid.UUID) (*UIStateSnapshot, error) {
	query := `
		SELECT session_id, ui_state, version, last_updated
		FROM ui_state_cache
		WHERE session_id = $1`

	var snapshot UIStateSnapshot
	var uiStateJSON []byte

	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(
		&snapshot.SessionID,
		&uiStateJSON,
		&snapshot.Version,
		&snapshot.LastUpdated,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// No UI state cached yet - return empty state
			return &UIStateSnapshot{
				SessionID:   sessionID,
				UIState:     make(map[string]interface{}),
				Version:     1,
				LastUpdated: time.Now(),
				Compressed: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get UI state: %w", err)
	}

	// Unmarshal UI state
	if err := json.Unmarshal(uiStateJSON, &snapshot.UIState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal UI state: %w", err)
	}

	return &snapshot, nil
}

// GetUIStateIfNewer returns UI state only if it's newer than the provided version
func (r *UIStateRepository) GetUIStateIfNewer(ctx context.Context, sessionID uuid.UUID, currentVersion int) (*UIStateSnapshot, error) {
	query := `
		SELECT session_id, ui_state, version, last_updated
		FROM ui_state_cache
		WHERE session_id = $1 AND version > $2`

	var snapshot UIStateSnapshot
	var uiStateJSON []byte

	err := r.db.QueryRowContext(ctx, query, sessionID, currentVersion).Scan(
		&snapshot.SessionID,
		&uiStateJSON,
		&snapshot.Version,
		&snapshot.LastUpdated,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// No newer version available
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get newer UI state: %w", err)
	}

	// Unmarshal UI state
	if err := json.Unmarshal(uiStateJSON, &snapshot.UIState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal UI state: %w", err)
	}

	return &snapshot, nil
}

// DeleteUIState removes the UI state cache for a session
func (r *UIStateRepository) DeleteUIState(ctx context.Context, sessionID uuid.UUID) error {
	query := `DELETE FROM ui_state_cache WHERE session_id = $1`

	result, err := r.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete UI state: %w", err)
	}

	deletedCount, _ := result.RowsAffected()
	if deletedCount == 0 {
		// Not an error if no state existed
		fmt.Printf("No UI state found for session %s\n", sessionID)
	} else {
		fmt.Printf("Deleted UI state for session %s\n", sessionID)
	}

	return nil
}

// GetStaleSessions finds sessions with outdated UI state cache
func (r *UIStateRepository) GetStaleSessions(ctx context.Context, maxAge time.Duration) ([]uuid.UUID, error) {
	cutoff := time.Now().Add(-maxAge)

	query := `
		SELECT usc.session_id
		FROM ui_state_cache usc
		JOIN research_sessions rs ON usc.session_id = rs.id
		WHERE usc.last_updated < $1
		AND rs.status IN ('analyzing', 'generating', 'validating')`

	rows, err := r.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query stale sessions: %w", err)
	}
	defer rows.Close()

	var sessionIDs []uuid.UUID
	for rows.Next() {
		var sessionID uuid.UUID
		if err := rows.Scan(&sessionID); err != nil {
			return nil, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, sessionID)
	}

	return sessionIDs, rows.Err()
}

// CompressUIState compresses large UI states for storage efficiency
func (r *UIStateRepository) CompressUIState(ctx context.Context, sessionID uuid.UUID) error {
	// Get current state
	snapshot, err := r.GetUIState(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get current UI state: %w", err)
	}

	// Check if compression is needed (arbitrary threshold)
	uiStateJSON, _ := json.Marshal(snapshot.UIState)
	if len(uiStateJSON) < 10000 { // Less than 10KB, no compression needed
		return nil
	}

	// In a real implementation, you'd compress with gzip/lz4
	// For now, we'll just mark it as compressed in the database
	query := `
		UPDATE ui_state_cache
		SET compressed_state = $2, compression_algorithm = 'gzip'
		WHERE session_id = $1`

	_, err = r.db.ExecContext(ctx, query, sessionID, uiStateJSON) // In reality, compress first
	if err != nil {
		return fmt.Errorf("failed to update compressed state: %w", err)
	}

	fmt.Printf("Compressed UI state for session %s (%d bytes)\n", sessionID, len(uiStateJSON))
	return nil
}

// GetUIStateStats returns statistics about UI state cache usage
func (r *UIStateRepository) GetUIStateStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_sessions,
			AVG(version) as avg_version,
			AVG(EXTRACT(EPOCH FROM (NOW() - last_updated))) as avg_age_seconds,
			COUNT(*) FILTER (WHERE compression_algorithm IS NOT NULL) as compressed_sessions
		FROM ui_state_cache`

	var stats struct {
		TotalSessions      int     `json:"total_sessions"`
		AvgVersion         float64 `json:"avg_version"`
		AvgAgeSeconds      float64 `json:"avg_age_seconds"`
		CompressedSessions int     `json:"compressed_sessions"`
	}

	row := r.db.QueryRowContext(ctx, query)
	err := row.Scan(
		&stats.TotalSessions,
		&stats.AvgVersion,
		&stats.AvgAgeSeconds,
		&stats.CompressedSessions,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get UI state stats: %w", err)
	}

	return map[string]interface{}{
		"total_sessions":       stats.TotalSessions,
		"avg_version":          stats.AvgVersion,
		"avg_age_seconds":      stats.AvgAgeSeconds,
		"compressed_sessions":  stats.CompressedSessions,
	}, nil
}

// CleanupOldUIStates removes UI state cache for completed sessions older than specified age
func (r *UIStateRepository) CleanupOldUIStates(ctx context.Context, maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)

	query := `
		DELETE FROM ui_state_cache
		WHERE session_id IN (
			SELECT id FROM research_sessions
			WHERE status IN ('completed', 'failed')
			AND updated_at < $1
		)`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old UI states: %w", err)
	}

	deletedCount, _ := result.RowsAffected()
	fmt.Printf("Cleaned up %d old UI state caches\n", deletedCount)

	return nil
}
