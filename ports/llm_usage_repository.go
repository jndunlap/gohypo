package ports

import (
	"context"
	"time"

	"gohypo/models"

	"github.com/google/uuid"
)

// LLMUsageRepository defines the interface for LLM usage data operations
type LLMUsageRepository interface {
	// Record usage for an LLM call
	RecordUsage(ctx context.Context, usage *models.LLMUsage) error

	// Get usage for a user within date range
	GetUserUsage(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*models.LLMUsage, error)

	// Get aggregated usage summary for a user
	GetUserUsageSummary(ctx context.Context, userID uuid.UUID, start, end time.Time) (*models.UserUsageSummary, error)

	// Get usage by provider for analytics
	GetUsageByProvider(ctx context.Context, userID uuid.UUID, start, end time.Time) (map[string]*models.ProviderUsage, error)

	// Get usage by model for analytics
	GetUsageByModel(ctx context.Context, userID uuid.UUID, start, end time.Time) (map[string]*models.ModelUsage, error)

	// Get total token counts for a user in a period
	GetTotalTokens(ctx context.Context, userID uuid.UUID, start, end time.Time) (int, error)
}
