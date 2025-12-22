package ports

import (
	"context"

	"gohypo/models"

	"github.com/google/uuid"
)

// HypothesisRepository defines the interface for hypothesis data operations
type HypothesisRepository interface {
	// SaveHypothesis saves a hypothesis result for a user and session
	SaveHypothesis(ctx context.Context, userID, sessionID uuid.UUID, result *models.HypothesisResult) error

	// GetHypothesis retrieves a hypothesis by user ID and hypothesis ID
	GetHypothesis(ctx context.Context, userID uuid.UUID, hypothesisID string) (*models.HypothesisResult, error)

	// ListUserHypotheses returns hypotheses for a user, optionally limited
	ListUserHypotheses(ctx context.Context, userID uuid.UUID, limit int) ([]*models.HypothesisResult, error)

	// ListSessionHypotheses returns all hypotheses for a specific session
	ListSessionHypotheses(ctx context.Context, userID, sessionID uuid.UUID) ([]*models.HypothesisResult, error)

	// GetUserStats returns statistics about a user's hypotheses
	GetUserStats(ctx context.Context, userID uuid.UUID) (*models.UserHypothesisStats, error)

	// ListByValidationState returns hypotheses filtered by validation state
	ListByValidationState(ctx context.Context, userID uuid.UUID, validated bool, limit int) ([]*models.HypothesisResult, error)

	// ListByWorkspace returns hypotheses for a specific workspace
	ListByWorkspace(ctx context.Context, userID uuid.UUID, workspaceID string, limit int) ([]*models.HypothesisResult, error)
}
