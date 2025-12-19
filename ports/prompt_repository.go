package ports

import (
	"context"

	"github.com/google/uuid"
)

// PromptRepository defines the interface for prompt data operations
type PromptRepository interface {
	// SavePrompt saves a research prompt for a user and session
	SavePrompt(ctx context.Context, userID, sessionID uuid.UUID, promptContent string, promptType string, metadata map[string]interface{}) error

	// GetPrompt retrieves a prompt by its ID
	GetPrompt(ctx context.Context, userID uuid.UUID, promptID uuid.UUID) (string, error)

	// ListSessionPrompts returns all prompts for a specific session
	ListSessionPrompts(ctx context.Context, userID, sessionID uuid.UUID) ([]*PromptRecord, error)

	// ListUserPrompts returns prompts for a user, optionally limited
	ListUserPrompts(ctx context.Context, userID uuid.UUID, limit int) ([]*PromptRecord, error)

	// GetLatestPrompt returns the most recent prompt for a session
	GetLatestPrompt(ctx context.Context, userID, sessionID uuid.UUID) (string, error)
}

// PromptRecord represents a stored prompt with metadata
type PromptRecord struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	SessionID     uuid.UUID              `json:"session_id" db:"session_id"`
	UserID        uuid.UUID              `json:"user_id" db:"user_id"`
	PromptContent string                 `json:"prompt_content" db:"prompt_content"`
	PromptType    string                 `json:"prompt_type" db:"prompt_type"`
	Metadata      map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt     string                 `json:"created_at" db:"created_at"`
}
