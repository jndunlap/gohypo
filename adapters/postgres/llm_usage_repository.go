package postgres

import (
	"context"
	"database/sql"
	"time"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// LLMUsageRepositoryImpl implements LLMUsageRepository for PostgreSQL
type LLMUsageRepositoryImpl struct {
	db *sqlx.DB
}

// NewLLMUsageRepository creates a new PostgreSQL LLM usage repository
func NewLLMUsageRepository(db *sqlx.DB) ports.LLMUsageRepository {
	return &LLMUsageRepositoryImpl{db: db}
}

// RecordUsage records LLM usage for an API call
func (r *LLMUsageRepositoryImpl) RecordUsage(ctx context.Context, usage *models.LLMUsage) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO llm_usage (
			user_id, session_id, provider, model, operation_type,
			prompt_tokens, completion_tokens, total_tokens, created_at
		) VALUES (
			:user_id, :session_id, :provider, :model, :operation_type,
			:prompt_tokens, :completion_tokens, :total_tokens, :created_at
		)
	`, usage)
	return err
}

// GetUserUsage retrieves usage records for a user within a date range
func (r *LLMUsageRepositoryImpl) GetUserUsage(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*models.LLMUsage, error) {
	var usages []*models.LLMUsage
	err := r.db.SelectContext(ctx, &usages, `
		SELECT id, user_id, session_id, provider, model, operation_type,
		       prompt_tokens, completion_tokens, total_tokens, created_at
		FROM llm_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at DESC
	`, userID, start, end)
	return usages, err
}

// GetUserUsageSummary returns aggregated usage statistics for a user
func (r *LLMUsageRepositoryImpl) GetUserUsageSummary(ctx context.Context, userID uuid.UUID, start, end time.Time) (*models.UserUsageSummary, error) {
	summary := &models.UserUsageSummary{
		UserID:      userID,
		PeriodStart: start,
		PeriodEnd:   end,
		ByProvider:  make(map[string]models.ProviderUsage),
		ByModel:     make(map[string]models.ModelUsage),
	}

	// Get basic aggregates
	err := r.db.GetContext(ctx, &summary, `
		SELECT
			COUNT(*) as request_count,
			SUM(total_tokens) as total_tokens,
			SUM(prompt_tokens) as total_prompt_tokens,
			SUM(completion_tokens) as total_completion_tokens
		FROM llm_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
	`, userID, start, end)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get provider breakdown
	providerRows, err := r.db.QueryContext(ctx, `
		SELECT provider, SUM(total_tokens) as total_tokens, COUNT(*) as request_count
		FROM llm_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY provider
	`, userID, start, end)

	if err != nil {
		return nil, err
	}
	defer providerRows.Close()

	for providerRows.Next() {
		var provider models.ProviderUsage
		err := providerRows.Scan(&provider.Provider, &provider.TotalTokens, &provider.RequestCount)
		if err != nil {
			return nil, err
		}
		summary.ByProvider[provider.Provider] = provider
	}

	// Get model breakdown
	modelRows, err := r.db.QueryContext(ctx, `
		SELECT model, provider, SUM(total_tokens) as total_tokens, COUNT(*) as request_count
		FROM llm_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY model, provider
	`, userID, start, end)

	if err != nil {
		return nil, err
	}
	defer modelRows.Close()

	for modelRows.Next() {
		var model models.ModelUsage
		err := modelRows.Scan(&model.Model, &model.Provider, &model.TotalTokens, &model.RequestCount)
		if err != nil {
			return nil, err
		}
		summary.ByModel[model.Model] = model
	}

	return summary, nil
}

// GetUsageByProvider returns usage aggregated by provider
func (r *LLMUsageRepositoryImpl) GetUsageByProvider(ctx context.Context, userID uuid.UUID, start, end time.Time) (map[string]*models.ProviderUsage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT provider, SUM(total_tokens) as total_tokens, COUNT(*) as request_count
		FROM llm_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY provider
	`, userID, start, end)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*models.ProviderUsage)
	for rows.Next() {
		var provider models.ProviderUsage
		err := rows.Scan(&provider.Provider, &provider.TotalTokens, &provider.RequestCount)
		if err != nil {
			return nil, err
		}
		result[provider.Provider] = &provider
	}

	return result, nil
}

// GetUsageByModel returns usage aggregated by model
func (r *LLMUsageRepositoryImpl) GetUsageByModel(ctx context.Context, userID uuid.UUID, start, end time.Time) (map[string]*models.ModelUsage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT model, provider, SUM(total_tokens) as total_tokens, COUNT(*) as request_count
		FROM llm_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY model, provider
	`, userID, start, end)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*models.ModelUsage)
	for rows.Next() {
		var model models.ModelUsage
		err := rows.Scan(&model.Model, &model.Provider, &model.TotalTokens, &model.RequestCount)
		if err != nil {
			return nil, err
		}
		result[model.Model] = &model
	}

	return result, nil
}

// GetTotalTokens returns the total token count for a user in a time period
func (r *LLMUsageRepositoryImpl) GetTotalTokens(ctx context.Context, userID uuid.UUID, start, end time.Time) (int, error) {
	var total int
	err := r.db.GetContext(ctx, &total, `
		SELECT COALESCE(SUM(total_tokens), 0)
		FROM llm_usage
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
	`, userID, start, end)
	return total, err
}
