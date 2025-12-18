package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/ports"
)

// MatrixResolverAdapter implements MatrixResolverPort for PostgreSQL
type MatrixResolverAdapter struct {
	db *sql.DB
}

// NewMatrixResolverAdapter creates a new matrix resolver adapter
func NewMatrixResolverAdapter(db *sql.DB) *MatrixResolverAdapter {
	return &MatrixResolverAdapter{db: db}
}

// ResolveMatrix produces a MatrixBundle for the given snapshot and variables
func (a *MatrixResolverAdapter) ResolveMatrix(ctx context.Context, req ports.MatrixResolutionRequest) (*dataset.MatrixBundle, error) {
	// Get snapshot details to calculate cutoff
	snapshot, err := a.getSnapshot(ctx, req.SnapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}
	cutoffAt := snapshot.SnapshotAt.ApplyLag(snapshot.Lag)

	// Create the matrix bundle
	bundle := dataset.NewMatrixBundle(req.SnapshotID, req.ViewID, "", cutoffAt, snapshot.Lag)

	// Resolve each variable using cohort-driven approach
	for _, varKey := range req.VarKeys {
		values, audit, err := a.resolveVariableCohortDriven(ctx, req.EntityIDs, varKey, snapshot, cutoffAt)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve variable %s: %w", varKey, err)
		}

		// Create column metadata
		meta := dataset.ColumnMeta{
			VariableKey:     varKey,
			StatisticalType: dataset.TypeNumeric, // TODO: get from contract
			DerivedColumns:  []dataset.DerivedColumn{},
			ResolutionAudit: *audit,
		}

		// Add column to bundle
		bundle.AddColumn(varKey, values, meta, *audit)
	}

	return bundle, nil
}

// resolveVariableCohortDriven uses cohort CTE + LEFT JOIN pattern for deterministic resolution
func (a *MatrixResolverAdapter) resolveVariableCohortDriven(ctx context.Context, entityIDs []core.ID, varKey core.VariableKey, snapshot *Snapshot, cutoffAt core.CutoffAt) ([]float64, *dataset.ResolutionAudit, error) {
	// Get variable contract
	contract, err := a.getVariableContract(ctx, varKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get contract for %s: %w", varKey, err)
	}

	// Build cohort CTE safely using array literal
	entityIDStrings := make([]string, len(entityIDs))
	for i, id := range entityIDs {
		// Basic escaping for array literal (assumes IDs don't contain } or ,)
		// In production, use lib/pq.Array or a proper escaping function
		entityIDStrings[i] = string(id)
	}
	arrayLiteral := "{" + strings.Join(entityIDStrings, ",") + "}"

	cohortCTE := "SELECT unnest($1::text[]) AS entity_id"

	// Build resolution subquery (inherently scalar per entity)
	resolutionQuery := a.buildScalarResolutionQuery(varKey, contract, cutoffAt)

	// Combine with LEFT JOIN
	query := fmt.Sprintf(`
		WITH cohort AS (%s),
		     resolved AS (%s)
		SELECT
			cohort.entity_id,
			COALESCE(resolved.value, %s) as final_value,
			resolved.observed_at
		FROM cohort
		LEFT JOIN resolved USING (entity_id)
		ORDER BY cohort.entity_id
	`, cohortCTE, resolutionQuery, a.getImputationSQL(contract.ImputationPolicy))

	// Execute query with parameters
	rows, err := a.db.QueryContext(ctx, query, arrayLiteral)
	if err != nil {
		return nil, nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Collect results (one row per entity in cohort order)
	values := make([]float64, len(entityIDs))
	maxTimestamp := time.Time{}

	for i := 0; rows.Next(); i++ {
		var entityID string
		var value float64
		var observedAt sql.NullTime

		if err := rows.Scan(&entityID, &value, &observedAt); err != nil {
			return nil, nil, fmt.Errorf("row scan failed: %w", err)
		}

		values[i] = value
		if observedAt.Valid && observedAt.Time.After(maxTimestamp) {
			maxTimestamp = observedAt.Time
		}
	}

	if err = rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("row iteration failed: %w", err)
	}

	// Create audit
	audit := &dataset.ResolutionAudit{
		VariableKey:       varKey,
		MaxTimestamp:      core.NewTimestamp(maxTimestamp),
		RowCount:          len(entityIDs),
		ImputationApplied: a.determineImputationApplied(values, contract.ImputationPolicy),
		ScalarGuarantee:   true, // Guaranteed by SQL structure
		AsOfMode:          dataset.AsOfMode(contract.AsOfMode),
		WindowDays:        contract.WindowDays,
	}

	return values, audit, nil
}

// buildScalarResolutionQuery creates SQL that guarantees one row per entity
func (a *MatrixResolverAdapter) buildScalarResolutionQuery(varKey core.VariableKey, contract *VariableContract, cutoffAt core.CutoffAt) string {
	switch contract.AsOfMode {
	case "latest_value_as_of":
		return a.buildLatestValueQuery(varKey, cutoffAt)

	case "count_over_window":
		return a.buildCountWindowQuery(varKey, contract.WindowDays, cutoffAt)

	case "sum_over_window":
		return a.buildSumWindowQuery(varKey, contract.WindowDays, cutoffAt)

	case "exists_as_of":
		return a.buildExistsQuery(varKey, cutoffAt)

	default:
		panic(fmt.Sprintf("unsupported as-of mode: %s", contract.AsOfMode))
	}
}

// buildLatestValueQuery - DISTINCT ON guarantees scalar per entity
func (a *MatrixResolverAdapter) buildLatestValueQuery(varKey core.VariableKey, cutoffAt core.CutoffAt) string {
	return fmt.Sprintf(`
		SELECT DISTINCT ON (entity_id)
			entity_id,
			(payload->>'%s')::float8 as value,
			observed_at
		FROM raw_events
		WHERE payload ? '%s'
		  AND observed_at <= '%s'
		ORDER BY entity_id, observed_at DESC
	`, varKey, varKey, cutoffAt.Time().Format("2006-01-02 15:04:05"))
}

// buildCountWindowQuery - GROUP BY guarantees scalar per entity
func (a *MatrixResolverAdapter) buildCountWindowQuery(varKey core.VariableKey, windowDays *int, cutoffAt core.CutoffAt) string {
	windowStart := cutoffAt
	if windowDays != nil {
		windowStart = core.NewCutoffAt(cutoffAt.Time().AddDate(0, 0, -(*windowDays)))
	}

	return fmt.Sprintf(`
		SELECT
			entity_id,
			COUNT(*)::float8 as value,
			MAX(observed_at) as observed_at
		FROM raw_events
		WHERE payload ? '%s'
		  AND observed_at <= '%s'
		  AND observed_at >= '%s'
		GROUP BY entity_id
	`, varKey, cutoffAt.Time().Format("2006-01-02 15:04:05"),
		windowStart.Time().Format("2006-01-02 15:04:05"))
}

// buildSumWindowQuery - GROUP BY guarantees scalar per entity
func (a *MatrixResolverAdapter) buildSumWindowQuery(varKey core.VariableKey, windowDays *int, cutoffAt core.CutoffAt) string {
	windowStart := cutoffAt
	if windowDays != nil {
		windowStart = core.NewCutoffAt(cutoffAt.Time().AddDate(0, 0, -(*windowDays)))
	}

	return fmt.Sprintf(`
		SELECT
			entity_id,
			SUM((payload->>'%s')::float8) as value,
			MAX(observed_at) as observed_at
		FROM raw_events
		WHERE payload ? '%s'
		  AND observed_at <= '%s'
		  AND observed_at >= '%s'
		GROUP BY entity_id
	`, varKey, varKey, cutoffAt.Time().Format("2006-01-02 15:04:05"),
		windowStart.Time().Format("2006-01-02 15:04:05"))
}

// buildExistsQuery - GROUP BY guarantees scalar per entity
func (a *MatrixResolverAdapter) buildExistsQuery(varKey core.VariableKey, cutoffAt core.CutoffAt) string {
	return fmt.Sprintf(`
		SELECT
			entity_id,
			CASE WHEN COUNT(*) > 0 THEN 1.0 ELSE 0.0 END as value,
			MAX(observed_at) as observed_at
		FROM raw_events
		WHERE payload ? '%s'
		  AND observed_at <= '%s'
		GROUP BY entity_id
	`, varKey, cutoffAt.Time().Format("2006-01-02 15:04:05"))
}

// getImputationSQL returns the SQL for default imputation
func (a *MatrixResolverAdapter) getImputationSQL(policy string) string {
	switch policy {
	case "zero_fill":
		return "0.0"
	case "mean_fill":
		return "0.0" // TODO: calculate actual mean
	default:
		return "0.0" // contract_default
	}
}

// determineImputationApplied checks if imputation was actually applied
func (a *MatrixResolverAdapter) determineImputationApplied(values []float64, policy string) string {
	// Check if any values match the imputation default
	// This is a simplified check - in practice you'd track which entities got imputed
	hasImputed := false
	for _, v := range values {
		if v == 0.0 { // Assuming 0.0 is the imputation value
			hasImputed = true
			break
		}
	}

	if hasImputed {
		return policy
	}
	return "none"
}

// getSnapshot retrieves snapshot details
func (a *MatrixResolverAdapter) getSnapshot(ctx context.Context, snapshotID core.SnapshotID) (*Snapshot, error) {
	query := `
		SELECT id, dataset, snapshot_at, lag_buffer, registry_hash
		FROM snapshots WHERE id = $1`

	var s Snapshot
	var lagSeconds int
	err := a.db.QueryRowContext(ctx, query, snapshotID).Scan(
		&s.ID, &s.Dataset, &s.SnapshotAt, &lagSeconds, &s.RegistryHash)
	if err != nil {
		return nil, err
	}

	s.Lag = core.NewLag(time.Duration(lagSeconds) * time.Second)
	return &s, nil
}

// getVariableContract retrieves the contract for a variable
func (a *MatrixResolverAdapter) getVariableContract(ctx context.Context, varKey core.VariableKey) (*VariableContract, error) {
	query := `
		SELECT var_key, as_of_mode, statistical_type, window_days,
		       imputation_policy, scalar_guarantee
		FROM variable_contracts
		WHERE var_key = $1`

	var contract VariableContract
	var windowDays sql.NullInt32

	err := a.db.QueryRowContext(ctx, query, varKey).Scan(
		&contract.VarKey,
		&contract.AsOfMode,
		&contract.StatisticalType,
		&windowDays,
		&contract.ImputationPolicy,
		&contract.ScalarGuarantee,
	)

	if err != nil {
		return nil, err
	}

	if windowDays.Valid {
		days := int(windowDays.Int32)
		contract.WindowDays = &days
	}

	return &contract, nil
}

// Snapshot represents a snapshot record
type Snapshot struct {
	ID           core.SnapshotID
	Dataset      string
	SnapshotAt   core.SnapshotAt
	Lag          core.Lag
	RegistryHash core.RegistryHash
}

// VariableContract represents a variable contract (internal to adapter)
type VariableContract struct {
	VarKey           core.VariableKey
	AsOfMode         string
	StatisticalType  string
	WindowDays       *int
	ImputationPolicy string
	ScalarGuarantee  bool
}
