package usage

import (
	"context"
	"log"
	"time"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
)

// Service handles LLM usage tracking and persistence
type Service struct {
	repo ports.LLMUsageRepository
}

// NewService creates a new usage service
func NewService(repo ports.LLMUsageRepository) *Service {
	return &Service{repo: repo}
}

// RecordUsage asynchronously records LLM usage for a user operation
func (s *Service) RecordUsage(ctx context.Context, userID uuid.UUID, sessionID *uuid.UUID, operationType string, usage *models.UsageData) error {
	// Validate usage data
	if usage == nil {
		log.Printf("[UsageService] ERROR: nil usage data provided")
		return nil // Don't fail the caller for tracking issues
	}

	if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 {
		log.Printf("[UsageService] ERROR: invalid token counts: %+v", usage)
		return nil
	}

	// Create usage record
	llmUsage := &models.LLMUsage{
		UserID:           userID,
		SessionID:        sessionID,
		Provider:         usage.Provider,
		Model:            usage.Model,
		OperationType:    operationType,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CreatedAt:        time.Now(),
	}

	// Async persistence to avoid blocking LLM calls
	go func() {
		if err := s.persistWithRetry(llmUsage); err != nil {
			log.Printf("[UsageService] ERROR: failed to persist usage after retries: %v", err)
			// TODO: Send to dead letter queue for manual review
		}
	}()

	return nil
}

// persistWithRetry attempts to persist usage with exponential backoff
func (s *Service) persistWithRetry(usage *models.LLMUsage) error {
	const maxRetries = 3
	const baseDelay = 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := s.repo.RecordUsage(context.Background(), usage); err == nil {
			return nil // Success
		}

		if attempt < maxRetries-1 {
			delay := time.Duration(attempt+1) * baseDelay
			time.Sleep(delay)
		}
	}

	// Final attempt
	return s.repo.RecordUsage(context.Background(), usage)
}

// GetUserUsageSummary returns aggregated usage for a user in a time period
func (s *Service) GetUserUsageSummary(ctx context.Context, userID uuid.UUID, start, end time.Time) (*models.UserUsageSummary, error) {
	return s.repo.GetUserUsageSummary(ctx, userID, start, end)
}

// GetUserUsage returns detailed usage records for a user
func (s *Service) GetUserUsage(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*models.LLMUsage, error) {
	return s.repo.GetUserUsage(ctx, userID, start, end)
}

// GetTotalTokens returns total token usage for a user in a time period
func (s *Service) GetTotalTokens(ctx context.Context, userID uuid.UUID, start, end time.Time) (int, error) {
	return s.repo.GetTotalTokens(ctx, userID, start, end)
}
