package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

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
	triGateResultJSON, _ := json.Marshal(result.TriGateResult)
	executionMetadataJSON, _ := json.Marshal(result.ExecutionMetadata)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO hypothesis_results (
			id, session_id, user_id, business_hypothesis, science_hypothesis, null_case,
			referee_results, tri_gate_result, passed, validation_timestamp,
			standards_version, execution_metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
		ON CONFLICT (id) DO UPDATE SET
			referee_results = EXCLUDED.referee_results,
			tri_gate_result = EXCLUDED.tri_gate_result,
			passed = EXCLUDED.passed,
			validation_timestamp = EXCLUDED.validation_timestamp,
			standards_version = EXCLUDED.standards_version,
			execution_metadata = EXCLUDED.execution_metadata
	`, result.ID, sessionID, userID, result.BusinessHypothesis, result.ScienceHypothesis,
		result.NullCase, refereeResultsJSON, triGateResultJSON, result.Passed,
		result.ValidationTimestamp, result.StandardsVersion, executionMetadataJSON)

	return err
}

// GetHypothesis retrieves a hypothesis by user ID and hypothesis ID
func (r *HypothesisRepositoryImpl) GetHypothesis(ctx context.Context, userID uuid.UUID, hypothesisID string) (*models.HypothesisResult, error) {
	var result models.HypothesisResult
	var refereeResultsJSON, triGateResultJSON, executionMetadataJSON []byte

	err := r.db.QueryRowContext(ctx, `
		SELECT id, session_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, tri_gate_result, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at
		FROM hypothesis_results
		WHERE user_id = $1 AND id = $2
	`, userID, hypothesisID).Scan(
		&result.ID, &result.SessionID, &result.BusinessHypothesis, &result.ScienceHypothesis,
		&result.NullCase, &refereeResultsJSON, &triGateResultJSON, &result.Passed,
		&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Unmarshal JSON fields
	json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
	json.Unmarshal(triGateResultJSON, &result.TriGateResult)
	json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)

	return &result, nil
}

// ListUserHypotheses returns hypotheses for a user, optionally limited
func (r *HypothesisRepositoryImpl) ListUserHypotheses(ctx context.Context, userID uuid.UUID, limit int) ([]*models.HypothesisResult, error) {
	query := `
		SELECT id, session_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, tri_gate_result, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at
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
		var refereeResultsJSON, triGateResultJSON, executionMetadataJSON []byte

		err := rows.Scan(
			&result.ID, &result.SessionID, &result.BusinessHypothesis, &result.ScienceHypothesis,
			&result.NullCase, &refereeResultsJSON, &triGateResultJSON, &result.Passed,
			&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
		json.Unmarshal(triGateResultJSON, &result.TriGateResult)
		json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)

		results = append(results, &result)
	}

	return results, rows.Err()
}

// ListSessionHypotheses returns all hypotheses for a specific session
func (r *HypothesisRepositoryImpl) ListSessionHypotheses(ctx context.Context, userID, sessionID uuid.UUID) ([]*models.HypothesisResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, session_id, business_hypothesis, science_hypothesis, null_case,
			   referee_results, tri_gate_result, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at
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
		var refereeResultsJSON, triGateResultJSON, executionMetadataJSON []byte

		err := rows.Scan(
			&result.ID, &result.SessionID, &result.BusinessHypothesis, &result.ScienceHypothesis,
			&result.NullCase, &refereeResultsJSON, &triGateResultJSON, &result.Passed,
			&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
		json.Unmarshal(triGateResultJSON, &result.TriGateResult)
		json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)

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
			   referee_results, tri_gate_result, passed, validation_timestamp,
			   standards_version, execution_metadata, created_at
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
		var refereeResultsJSON, triGateResultJSON, executionMetadataJSON []byte

		err := rows.Scan(
			&result.ID, &result.SessionID, &result.BusinessHypothesis, &result.ScienceHypothesis,
			&result.NullCase, &refereeResultsJSON, &triGateResultJSON, &result.Passed,
			&result.ValidationTimestamp, &result.StandardsVersion, &executionMetadataJSON, &result.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON fields
		json.Unmarshal(refereeResultsJSON, &result.RefereeResults)
		json.Unmarshal(triGateResultJSON, &result.TriGateResult)
		json.Unmarshal(executionMetadataJSON, &result.ExecutionMetadata)

		results = append(results, &result)
	}

	return results, rows.Err()
}
