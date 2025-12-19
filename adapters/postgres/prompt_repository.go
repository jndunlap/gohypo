package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"gohypo/ports"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// PromptRepositoryImpl implements PromptRepository for PostgreSQL
type PromptRepositoryImpl struct {
	db *sqlx.DB
}

// NewPromptRepository creates a new PostgreSQL prompt repository
func NewPromptRepository(db *sqlx.DB) ports.PromptRepository {
	return &PromptRepositoryImpl{db: db}
}

// SavePrompt saves a research prompt for a user and session
func (r *PromptRepositoryImpl) SavePrompt(ctx context.Context, userID, sessionID uuid.UUID, promptContent string, promptType string, metadata map[string]interface{}) error {
	metadataJSON, _ := json.Marshal(metadata)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO research_prompts (session_id, user_id, prompt_content, prompt_type, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`, sessionID, userID, promptContent, promptType, metadataJSON)

	return err
}

// GetPrompt retrieves a prompt by its ID
func (r *PromptRepositoryImpl) GetPrompt(ctx context.Context, userID uuid.UUID, promptID uuid.UUID) (string, error) {
	var promptContent string
	err := r.db.GetContext(ctx, &promptContent, `
		SELECT prompt_content
		FROM research_prompts
		WHERE user_id = $1 AND id = $2
	`, userID, promptID)

	return promptContent, err
}

// ListSessionPrompts returns all prompts for a specific session
func (r *PromptRepositoryImpl) ListSessionPrompts(ctx context.Context, userID, sessionID uuid.UUID) ([]*ports.PromptRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, session_id, user_id, prompt_content, prompt_type, metadata, created_at
		FROM research_prompts
		WHERE user_id = $1 AND session_id = $2
		ORDER BY created_at ASC
	`, userID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prompts []*ports.PromptRecord
	for rows.Next() {
		var record ports.PromptRecord
		var metadataJSON []byte

		err := rows.Scan(
			&record.ID,
			&record.SessionID,
			&record.UserID,
			&record.PromptContent,
			&record.PromptType,
			&metadataJSON,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal metadata
		json.Unmarshal(metadataJSON, &record.Metadata)

		prompts = append(prompts, &record)
	}

	return prompts, rows.Err()
}

// ListUserPrompts returns prompts for a user, optionally limited
func (r *PromptRepositoryImpl) ListUserPrompts(ctx context.Context, userID uuid.UUID, limit int) ([]*ports.PromptRecord, error) {
	query := `
		SELECT id, session_id, user_id, prompt_content, prompt_type, metadata, created_at
		FROM research_prompts
		WHERE user_id = $1
		ORDER BY created_at DESC
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

	var prompts []*ports.PromptRecord
	for rows.Next() {
		var record ports.PromptRecord
		var metadataJSON []byte

		err := rows.Scan(
			&record.ID,
			&record.SessionID,
			&record.UserID,
			&record.PromptContent,
			&record.PromptType,
			&metadataJSON,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal metadata
		json.Unmarshal(metadataJSON, &record.Metadata)

		prompts = append(prompts, &record)
	}

	return prompts, rows.Err()
}

// GetLatestPrompt returns the most recent prompt for a session
func (r *PromptRepositoryImpl) GetLatestPrompt(ctx context.Context, userID, sessionID uuid.UUID) (string, error) {
	var promptContent string
	err := r.db.GetContext(ctx, &promptContent, `
		SELECT prompt_content
		FROM research_prompts
		WHERE user_id = $1 AND session_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, sessionID)

	if err == sql.ErrNoRows {
		return "", nil // No prompt found
	}

	return promptContent, err
}
