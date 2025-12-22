package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// HypothesisRepositoryImpl implements HypothesisRepository for PostgreSQL
type HypothesisRepositoryImpl struct {
	db *sqlx.DB
}

// NewHypothesisRepository creates a new PostgreSQL hypothesis repository
func NewHypothesisRepository(db *sqlx.DB) ports.HypothesisRepository {
	return &HypothesisRepositoryImpl{db: db}
}

// SaveHypothesis saves a hypothesis result for a user and session
func (r *HypothesisRepositoryImpl) SaveHypothesis(ctx context.Context, userID, sessionID uuid.UUID, result *models.HypothesisResult) error {
	refereeResultsJSON, _ := json.Marshal(result.RefereeResults)
	executionMetadataJSON, _ := json.Marshal(result.ExecutionMetadata)
	dataTopologyJSON, _ := json.Marshal(result.DataTopology)

	// PhaseEValues is now stored as JSONB
	var phaseEValuesJSON []byte
	if result.PhaseEValues != nil {
		phaseEValuesJSON, _ = json.Marshal(result.PhaseEValues)
	} else {
		phaseEValuesJSON, _ = json.Marshal([]float64{})
	}

	// Parse workspace ID if provided
	var workspaceID *uuid.UUID
	if result.WorkspaceID != "" {
		if parsed, err := uuid.Parse(result.WorkspaceID); err == nil {
			workspaceID = &parsed
		}
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO hypothesis_results (
			id, session_id, user_id, workspace_id, business_hypothesis, science_hypothesis, null_case,
			referee_results, passed, validation_timestamp,
			standards_version, execution_metadata, created_at,
			phase_e_values, feasibility_score, risk_level, data_topology,
			current_e_value, normalized_e_value, confidence, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (id) DO UPDATE SET
			workspace_id = EXCLUDED.workspace_id,
			referee_results = EXCLUDED.referee_results,
			passed = EXCLUDED.passed,
			validation_timestamp = EXCLUDED.validation_timestamp,
			standards_version = EXCLUDED.standards_version,
			execution_metadata = EXCLUDED.execution_metadata,
			phase_e_values = EXCLUDED.phase_e_values,
			feasibility_score = EXCLUDED.feasibility_score,
			risk_level = EXCLUDED.risk_level,
			data_topology = EXCLUDED.data_topology,
			current_e_value = EXCLUDED.current_e_value,
			normalized_e_value = EXCLUDED.normalized_e_value,
			confidence = EXCLUDED.confidence,
			status = EXCLUDED.status`, result.ID, sessionID, userID, workspaceID, result.BusinessHypothesis, result.ScienceHypothesis,
		result.NullCase, refereeResultsJSON, result.Passed,
		result.ValidationTimestamp, result.StandardsVersion, executionMetadataJSON,
		phaseEValuesJSON, result.FeasibilityScore, result.RiskLevel, dataTopologyJSON,
		result.CurrentEValue, result.NormalizedEValue, result.Confidence, result.Status)

	return err
}

// GetHypothesis retrieves a hypothesis by user ID and hypothesis ID
func (r *HypothesisRepositoryImpl) GetHypothesis(ctx context.Context, userID uuid.UUID, hypothesisID string) (*models.HypothesisResult, error) {
	var result models.HypothesisResult
	var refereeResultsJSON, executionMetadataJSON, dataTopologyJSON, phaseEValuesJSON []byte
	var workspaceID *uuid.UUID

	err := r.db.QueryRowContext(ctx, `
		SELECT id, session_id, workspace_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at,
			   phase_e_values, feasibility_score, risk_level, data_topology,
			   current_e_value, normalized_e_value, confidence, status
		FROM hypothesis_results
		WHERE user_id = $1 AND id = $2
	`, userID, hypothesisID).Scan(
		&result.ID, &result.SessionID, &workspaceID, &result.BusinessHypothesis, &result.ScienceHypothesis,
		&result.NullCase, &refereeResultsJSON, &result.Passed,
		&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
		&phaseEValuesJSON, &result.FeasibilityScore, &result.RiskLevel, &dataTopologyJSON,
		&result.CurrentEValue, &result.NormalizedEValue, &result.Confidence, &result.Status,
	)

	if err != nil {
		return nil, err
	}

	// Set workspace ID if available
	if workspaceID != nil {
		result.WorkspaceID = workspaceID.String()
	}

	// Unmarshal phase_e_values JSONB
	if len(phaseEValuesJSON) > 0 {
		if err := json.Unmarshal(phaseEValuesJSON, &result.PhaseEValues); err != nil {
			return nil, fmt.Errorf("failed to unmarshal phase_e_values: %w", err)
		}
	}

	// Unmarshal JSON fields
	json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
	json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)
	json.Unmarshal(dataTopologyJSON, &result.DataTopology)

	// Unmarshal phase_e_values JSONB
	if len(phaseEValuesJSON) > 0 {
		json.Unmarshal(phaseEValuesJSON, &result.PhaseEValues)
	}

	return &result, nil
}

// ListUserHypotheses returns hypotheses for a user, optionally limited
func (r *HypothesisRepositoryImpl) ListUserHypotheses(ctx context.Context, userID uuid.UUID, limit int) ([]*models.HypothesisResult, error) {
	query := `
		SELECT id, session_id, workspace_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at,
			   phase_e_values, feasibility_score, risk_level, data_topology,
			   current_e_value, normalized_e_value, confidence, status
		FROM hypothesis_results
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

	var results []*models.HypothesisResult
	for rows.Next() {
		var result models.HypothesisResult
		var refereeResultsJSON, executionMetadataJSON, dataTopologyJSON, phaseEValuesJSON []byte
		var workspaceID *uuid.UUID

		err := rows.Scan(
			&result.ID, &result.SessionID, &workspaceID, &result.BusinessHypothesis, &result.ScienceHypothesis,
			&result.NullCase, &refereeResultsJSON, &result.Passed,
			&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
			&phaseEValuesJSON, &result.FeasibilityScore, &result.RiskLevel, &dataTopologyJSON,
			&result.CurrentEValue, &result.NormalizedEValue, &result.Confidence, &result.Status,
		)
		if err != nil {
			return nil, err
		}

		// Set workspace ID if available
		if workspaceID != nil {
			result.WorkspaceID = workspaceID.String()
		}

		// Unmarshal JSON fields
		json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
		json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)
		json.Unmarshal(dataTopologyJSON, &result.DataTopology)

		// Unmarshal phase_e_values JSONB
		if len(phaseEValuesJSON) > 0 {
			json.Unmarshal(phaseEValuesJSON, &result.PhaseEValues)
		}

		results = append(results, &result)
	}

	return results, rows.Err()
}

// ListSessionHypotheses returns all hypotheses for a specific session
func (r *HypothesisRepositoryImpl) ListSessionHypotheses(ctx context.Context, userID, sessionID uuid.UUID) ([]*models.HypothesisResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, session_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at,
			   phase_e_values, feasibility_score, risk_level, data_topology,
			   current_e_value, normalized_e_value, confidence, status
		FROM hypothesis_results
		WHERE user_id = $1 AND session_id = $2
		ORDER BY created_at ASC
	`, userID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.HypothesisResult
	for rows.Next() {
		var result models.HypothesisResult
		var refereeResultsJSON, executionMetadataJSON, dataTopologyJSON []byte
		var phaseEValues []float64

		err := rows.Scan(
			&result.ID, &result.SessionID, &result.BusinessHypothesis, &result.ScienceHypothesis,
			&result.NullCase, &refereeResultsJSON, &result.Passed,
			&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
			&phaseEValues, &result.FeasibilityScore, &result.RiskLevel, &dataTopologyJSON,
			&result.CurrentEValue, &result.NormalizedEValue, &result.Confidence, &result.Status,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
		json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)
		json.Unmarshal(dataTopologyJSON, &result.DataTopology)

		// Set phase E-values
		result.PhaseEValues = phaseEValues

		results = append(results, &result)
	}

	return results, rows.Err()
}

// GetUserStats returns statistics about a user's hypotheses
func (r *HypothesisRepositoryImpl) GetUserStats(ctx context.Context, userID uuid.UUID) (*models.UserHypothesisStats, error) {
	var stats models.UserHypothesisStats
	var earliest, latest sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total_hypotheses,
			COUNT(CASE WHEN passed THEN 1 END) as validated_count,
			COUNT(CASE WHEN NOT passed THEN 1 END) as rejected_count,
			MIN(created_at) as earliest_hypothesis,
			MAX(created_at) as latest_hypothesis
		FROM hypothesis_results
		WHERE user_id = $1
	`, userID).Scan(&stats.TotalHypotheses, &stats.ValidatedCount, &stats.RejectedCount, &earliest, &latest)

	if err != nil {
		return nil, err
	}

	if stats.TotalHypotheses > 0 {
		stats.ValidationRate = float64(stats.ValidatedCount) / float64(stats.TotalHypotheses) * 100
	}

	if earliest.Valid {
		stats.EarliestHypothesis = &earliest.Time
	}
	if latest.Valid {
		stats.LatestHypothesis = &latest.Time
	}

	return &stats, nil
}

// ListByValidationState returns hypotheses filtered by validation state
func (r *HypothesisRepositoryImpl) ListByValidationState(ctx context.Context, userID uuid.UUID, validated bool, limit int) ([]*models.HypothesisResult, error) {
	query := `
		SELECT id, session_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at,
			   phase_e_values, feasibility_score, risk_level, data_topology,
			   current_e_value, normalized_e_value, confidence, status
		FROM hypothesis_results
		WHERE user_id = $1 AND passed = $2
		ORDER BY created_at DESC
	`

	args := []interface{}{userID, validated}
	if limit > 0 {
		query += " LIMIT $3"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.HypothesisResult
	for rows.Next() {
		var result models.HypothesisResult
		var refereeResultsJSON, executionMetadataJSON, dataTopologyJSON []byte
		var phaseEValues []float64

		err := rows.Scan(
			&result.ID, &result.SessionID, &result.BusinessHypothesis, &result.ScienceHypothesis,
			&result.NullCase, &refereeResultsJSON, &result.Passed,
			&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
			&phaseEValues, &result.FeasibilityScore, &result.RiskLevel, &dataTopologyJSON,
			&result.CurrentEValue, &result.NormalizedEValue, &result.Confidence, &result.Status,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
		json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)
		json.Unmarshal(dataTopologyJSON, &result.DataTopology)

		// Set phase E-values
		result.PhaseEValues = phaseEValues

		results = append(results, &result)
	}

	return results, rows.Err()
}

// ListByWorkspace returns hypotheses for a specific workspace
func (r *HypothesisRepositoryImpl) ListByWorkspace(ctx context.Context, userID uuid.UUID, workspaceID string, limit int) ([]*models.HypothesisResult, error) {
	query := `
		SELECT id, session_id, workspace_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at,
			   phase_e_values, feasibility_score, risk_level, data_topology,
			   current_e_value, normalized_e_value, confidence, status
		FROM hypothesis_results
		WHERE user_id = $1 AND workspace_id::text = $2
		ORDER BY created_at DESC
	`

	args := []interface{}{userID, workspaceID}
	if limit > 0 {
		query += " LIMIT $3"
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.HypothesisResult
	for rows.Next() {
		var result models.HypothesisResult
		var refereeResultsJSON, executionMetadataJSON, dataTopologyJSON []byte
		var dbWorkspaceID *uuid.UUID
		var phaseEValues []float64

		err := rows.Scan(
			&result.ID, &result.SessionID, &dbWorkspaceID, &result.BusinessHypothesis, &result.ScienceHypothesis,
			&result.NullCase, &refereeResultsJSON, &result.Passed,
			&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
			&phaseEValues, &result.FeasibilityScore, &result.RiskLevel, &dataTopologyJSON,
			&result.CurrentEValue, &result.NormalizedEValue, &result.Confidence, &result.Status,
		)
		if err != nil {
			return nil, err
		}

		// Set workspace ID if available
		if dbWorkspaceID != nil {
			result.WorkspaceID = dbWorkspaceID.String()
		}

		// Unmarshal JSON fields
		json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
		json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)
		json.Unmarshal(dataTopologyJSON, &result.DataTopology)

		// Set phase E-values
		result.PhaseEValues = phaseEValues

		results = append(results, &result)
	}

	return results, rows.Err()
}
