package research

import (
	"context"
	"fmt"
	"time"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
)

// ResearchStorage handles persistence of research hypotheses
type ResearchStorage struct {
	hypothesisRepo ports.HypothesisRepository
	userRepo       ports.UserRepository
	sessionRepo    ports.SessionRepository
}

// NewResearchStorage creates a new research storage instance with database repositories
func NewResearchStorage(hypothesisRepo ports.HypothesisRepository, userRepo ports.UserRepository, sessionRepo ports.SessionRepository) *ResearchStorage {
	return &ResearchStorage{
		hypothesisRepo: hypothesisRepo,
		userRepo:       userRepo,
		sessionRepo:    sessionRepo,
	}
}

// SaveHypothesis saves a hypothesis result for the default user
func (rs *ResearchStorage) SaveHypothesis(ctx context.Context, result *models.HypothesisResult) error {
	user, err := rs.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	sessionUUID, err := uuid.Parse(result.SessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Get the session to extract workspace ID
	session, err := rs.sessionRepo.GetSession(ctx, user.ID, sessionUUID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Set workspace ID on the result
	if session.WorkspaceID != uuid.Nil {
		result.WorkspaceID = session.WorkspaceID.String()
	}

	return rs.hypothesisRepo.SaveHypothesis(ctx, user.ID, sessionUUID, result)
}

// GetByID retrieves a hypothesis by its ID for the default user
func (rs *ResearchStorage) GetByID(ctx context.Context, id string) (*models.HypothesisResult, error) {
	user, err := rs.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	return rs.hypothesisRepo.GetHypothesis(ctx, user.ID, id)
}

// GetDefaultUser returns the default user
func (rs *ResearchStorage) GetDefaultUser(ctx context.Context) (*models.User, error) {
	return rs.userRepo.GetOrCreateDefaultUser(ctx)
}

// ListRecent returns the most recent hypotheses, limited by count
func (rs *ResearchStorage) ListRecent(ctx context.Context, limit int) ([]*models.HypothesisResult, error) {
	user, err := rs.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	return rs.hypothesisRepo.ListUserHypotheses(ctx, user.ID, limit)
}

// ListByValidationState returns hypotheses filtered by validation state
func (rs *ResearchStorage) ListByValidationState(ctx context.Context, validated bool, limit int) ([]*models.HypothesisResult, error) {
	user, err := rs.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	return rs.hypothesisRepo.ListByValidationState(ctx, user.ID, validated, limit)
}

// ListAll returns all hypotheses for the default user sorted by creation time (newest first)
func (rs *ResearchStorage) ListAll(ctx context.Context) ([]*models.HypothesisResult, error) {
	user, err := rs.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	return rs.hypothesisRepo.ListUserHypotheses(ctx, user.ID, 0) // 0 = no limit
}

// GetStats returns statistics about stored hypotheses for the default user
func (rs *ResearchStorage) GetStats(ctx context.Context) (map[string]interface{}, error) {
	user, err := rs.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	stats, err := rs.hypothesisRepo.GetUserStats(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_hypotheses":    stats.TotalHypotheses,
		"validated_count":     stats.ValidatedCount,
		"rejected_count":      stats.RejectedCount,
		"validation_rate":     stats.ValidationRate,
		"earliest_hypothesis": stats.EarliestHypothesis,
		"latest_hypothesis":   stats.LatestHypothesis,
	}, nil
}

// ListByWorkspace returns hypotheses for a specific workspace
func (rs *ResearchStorage) ListByWorkspace(ctx context.Context, workspaceID string, limit int) ([]*models.HypothesisResult, error) {
	user, err := rs.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user: %w", err)
	}

	return rs.hypothesisRepo.ListByWorkspace(ctx, user.ID, workspaceID, limit)
}

// CleanupOldFiles removes hypothesis files older than the specified duration
// Note: Database cleanup can be handled separately if needed
func (rs *ResearchStorage) CleanupOldFiles(maxAge time.Duration) (int, error) {
	// Database-backed storage doesn't need file cleanup
	return 0, nil
}
