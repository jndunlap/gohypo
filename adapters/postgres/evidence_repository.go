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

// EvidencePoint represents a single evidence accumulation data point
type EvidencePoint struct {
	ID                  int64     `json:"id"`
	HypothesisID        string    `json:"hypothesis_id"`
	Timestamp           time.Time `json:"timestamp"`
	EValue             float64   `json:"e_value"`
	NormalizedEValue   float64   `json:"normalized_e_value"`
	Confidence         float64   `json:"confidence"`
	ActiveTestCount    int       `json:"active_test_count"`
	CompletedTestCount int       `json:"completed_test_count"`
	Phase              int       `json:"phase"`
	UISnapshot         map[string]interface{} `json:"ui_snapshot"`
	MemoryUsageMB      int       `json:"memory_usage_mb"`
	CPUUsagePercent    float64   `json:"cpu_usage_percent"`
}

// EvidenceRepository handles time-series evidence data for live UI updates
type EvidenceRepository struct {
	db *sqlx.DB
}

// NewEvidenceRepository creates a new evidence repository
func NewEvidenceRepository(db *sqlx.DB) *EvidenceRepository {
	return &EvidenceRepository{db: db}
}

// InsertEvidencePoint adds a new evidence accumulation data point
func (r *EvidenceRepository) InsertEvidencePoint(ctx context.Context, point *EvidencePoint) error {
	query := `
		INSERT INTO evidence_accumulation (
			hypothesis_id, e_value, normalized_e_value, confidence,
			active_test_count, completed_test_count, phase, ui_snapshot,
			memory_usage_mb, cpu_usage_percent, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	uiSnapshotJSON, err := json.Marshal(point.UISnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal UI snapshot: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		point.HypothesisID,
		point.EValue,
		point.NormalizedEValue,
		point.Confidence,
		point.ActiveTestCount,
		point.CompletedTestCount,
		point.Phase,
		uiSnapshotJSON,
		point.MemoryUsageMB,
		point.CPUUsagePercent,
		point.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("failed to insert evidence point: %w", err)
	}

	return nil
}

// GetLatestEvidenceForHypothesis gets the most recent evidence point for a hypothesis
func (r *EvidenceRepository) GetLatestEvidenceForHypothesis(ctx context.Context, hypothesisID string) (*EvidencePoint, error) {
	query := `
		SELECT id, hypothesis_id, timestamp, e_value, normalized_e_value, confidence,
			   active_test_count, completed_test_count, phase, ui_snapshot,
			   memory_usage_mb, cpu_usage_percent
		FROM evidence_accumulation
		WHERE hypothesis_id = $1
		ORDER BY timestamp DESC
		LIMIT 1`

	var point EvidencePoint
	var uiSnapshotJSON []byte

	err := r.db.QueryRowContext(ctx, query, hypothesisID).Scan(
		&point.ID,
		&point.HypothesisID,
		&point.Timestamp,
		&point.EValue,
		&point.NormalizedEValue,
		&point.Confidence,
		&point.ActiveTestCount,
		&point.CompletedTestCount,
		&point.Phase,
		&uiSnapshotJSON,
		&point.MemoryUsageMB,
		&point.CPUUsagePercent,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No evidence points yet
		}
		return nil, fmt.Errorf("failed to get latest evidence: %w", err)
	}

	// Unmarshal UI snapshot
	if err := json.Unmarshal(uiSnapshotJSON, &point.UISnapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal UI snapshot: %w", err)
	}

	return &point, nil
}

// GetEvidenceHistoryForHypothesis gets evidence history for UI replay
func (r *EvidenceRepository) GetEvidenceHistoryForHypothesis(ctx context.Context, hypothesisID string, since time.Time) ([]*EvidencePoint, error) {
	query := `
		SELECT id, hypothesis_id, timestamp, e_value, normalized_e_value, confidence,
			   active_test_count, completed_test_count, phase, ui_snapshot,
			   memory_usage_mb, cpu_usage_percent
		FROM evidence_accumulation
		WHERE hypothesis_id = $1 AND timestamp >= $2
		ORDER BY timestamp ASC`

	rows, err := r.db.QueryContext(ctx, query, hypothesisID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence history: %w", err)
	}
	defer rows.Close()

	var points []*EvidencePoint
	for rows.Next() {
		var point EvidencePoint
		var uiSnapshotJSON []byte

		err := rows.Scan(
			&point.ID,
			&point.HypothesisID,
			&point.Timestamp,
			&point.EValue,
			&point.NormalizedEValue,
			&point.Confidence,
			&point.ActiveTestCount,
			&point.CompletedTestCount,
			&point.Phase,
			&uiSnapshotJSON,
			&point.MemoryUsageMB,
			&point.CPUUsagePercent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan evidence point: %w", err)
		}

		// Unmarshal UI snapshot
		if err := json.Unmarshal(uiSnapshotJSON, &point.UISnapshot); err != nil {
			return nil, fmt.Errorf("failed to unmarshal UI snapshot: %w", err)
		}

		points = append(points, &point)
	}

	return points, rows.Err()
}

// GetEvidenceSummaryForSession gets aggregated evidence data for dashboard
func (r *EvidenceRepository) GetEvidenceSummaryForSession(ctx context.Context, sessionID uuid.UUID) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_points,
			AVG(e_value) as avg_e_value,
			MAX(e_value) as max_e_value,
			MIN(e_value) as min_e_value,
			AVG(normalized_e_value) as avg_normalized,
			MAX(normalized_e_value) as max_normalized,
			AVG(confidence) as avg_confidence,
			SUM(CASE WHEN phase = 0 THEN 1 ELSE 0 END) as integrity_points,
			SUM(CASE WHEN phase = 1 THEN 1 ELSE 0 END) as causality_points,
			SUM(CASE WHEN phase = 2 THEN 1 ELSE 0 END) as complexity_points
		FROM evidence_accumulation ea
		JOIN hypotheses h ON ea.hypothesis_id = h.id
		WHERE h.session_id = $1`

	var summary struct {
		TotalPoints      int     `json:"total_points"`
		AvgEValue        float64 `json:"avg_e_value"`
		MaxEValue        float64 `json:"max_e_value"`
		MinEValue        float64 `json:"min_e_value"`
		AvgNormalized    float64 `json:"avg_normalized"`
		MaxNormalized    float64 `json:"max_normalized"`
		AvgConfidence    float64 `json:"avg_confidence"`
		IntegrityPoints  int     `json:"integrity_points"`
		CausalityPoints  int     `json:"causality_points"`
		ComplexityPoints int     `json:"complexity_points"`
	}

	row := r.db.QueryRowContext(ctx, query, sessionID.String())
	err := row.Scan(
		&summary.TotalPoints,
		&summary.AvgEValue,
		&summary.MaxEValue,
		&summary.MinEValue,
		&summary.AvgNormalized,
		&summary.MaxNormalized,
		&summary.AvgConfidence,
		&summary.IntegrityPoints,
		&summary.CausalityPoints,
		&summary.ComplexityPoints,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// No evidence data yet
			return map[string]interface{}{
				"total_points":       0,
				"avg_e_value":        0.0,
				"max_e_value":        0.0,
				"min_e_value":        0.0,
				"avg_normalized":     0.0,
				"max_normalized":     0.0,
				"avg_confidence":     0.0,
				"integrity_points":   0,
				"causality_points":   0,
				"complexity_points":  0,
			}, nil
		}
		return nil, fmt.Errorf("failed to get evidence summary: %w", err)
	}

	// Convert to map for JSON response
	return map[string]interface{}{
		"total_points":       summary.TotalPoints,
		"avg_e_value":        summary.AvgEValue,
		"max_e_value":        summary.MaxEValue,
		"min_e_value":        summary.MinEValue,
		"avg_normalized":     summary.AvgNormalized,
		"max_normalized":     summary.MaxNormalized,
		"avg_confidence":     summary.AvgConfidence,
		"integrity_points":   summary.IntegrityPoints,
		"causality_points":   summary.CausalityPoints,
		"complexity_points":  summary.ComplexityPoints,
	}, nil
}

// DeleteEvidenceForHypothesis removes all evidence points for a hypothesis
func (r *EvidenceRepository) DeleteEvidenceForHypothesis(ctx context.Context, hypothesisID string) error {
	query := `DELETE FROM evidence_accumulation WHERE hypothesis_id = $1`

	result, err := r.db.ExecContext(ctx, query, hypothesisID)
	if err != nil {
		return fmt.Errorf("failed to delete evidence: %w", err)
	}

	deletedCount, _ := result.RowsAffected()
	fmt.Printf("Deleted %d evidence points for hypothesis %s\n", deletedCount, hypothesisID)

	return nil
}

// CleanupOldEvidence removes evidence points older than the specified retention period
func (r *EvidenceRepository) CleanupOldEvidence(ctx context.Context, retentionPeriod time.Duration) error {
	cutoff := time.Now().Add(-retentionPeriod)

	query := `DELETE FROM evidence_accumulation WHERE timestamp < $1`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup old evidence: %w", err)
	}

	deletedCount, _ := result.RowsAffected()
	fmt.Printf("Cleaned up %d old evidence points older than %v\n", deletedCount, cutoff)

	return nil
}

// GetEvidenceVelocity calculates the rate of evidence accumulation
func (r *EvidenceRepository) GetEvidenceVelocity(ctx context.Context, hypothesisID string, window time.Duration) (float64, error) {
	query := `
		SELECT COUNT(*) as points_in_window
		FROM evidence_accumulation
		WHERE hypothesis_id = $1 AND timestamp >= $2`

	cutoff := time.Now().Add(-window)

	var pointsInWindow int
	err := r.db.QueryRowContext(ctx, query, hypothesisID, cutoff).Scan(&pointsInWindow)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate evidence velocity: %w", err)
	}

	// Velocity as points per minute
	windowMinutes := window.Minutes()
	if windowMinutes == 0 {
		return 0, nil
	}

	velocity := float64(pointsInWindow) / windowMinutes
	return velocity, nil
}
