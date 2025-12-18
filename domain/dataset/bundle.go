package dataset

import (
	"fmt"

	"gohypo/domain/core"
)

// MatrixBundle is the canonical data object for all statistical computation
// This is the single input to StatsPort and BatteryPort, and output of MatrixResolverPort
type MatrixBundle struct {
	// Core data
	Matrix     Matrix
	ColumnMeta []ColumnMeta

	// Resolution audits
	Audits []ResolutionAudit

	// Context references for determinism
	SnapshotID core.SnapshotID
	ViewID     core.ID
	CohortHash core.CohortHash

	// Metadata
	CutoffAt  core.CutoffAt
	Lag       core.Lag
	CreatedAt core.Timestamp

	// Fingerprint for replayability
	Fingerprint core.Hash
}

// Matrix represents dense numerical data ready for statistical analysis
type Matrix struct {
	Data         [][]float64        // rows=entities, cols=variables
	EntityIDs    []core.ID          // entity identifiers
	VariableKeys []core.VariableKey // column variable keys
}

// ColumnMeta contains metadata for each matrix column
type ColumnMeta struct {
	VariableKey     core.VariableKey
	StatisticalType StatisticalType
	DerivedColumns  []DerivedColumn // missing indicators, etc.
	ResolutionAudit ResolutionAudit
}

// DerivedColumn represents computed columns (e.g., missing indicators)
type DerivedColumn struct {
	Name  string // e.g., "missing_indicator"
	Index int    // column position in matrix
	Type  string // "binary", "numeric", etc.
}

// ResolutionAudit tracks how each variable was resolved
type ResolutionAudit struct {
	VariableKey       core.VariableKey
	MaxTimestamp      core.Timestamp
	RowCount          int
	ImputationApplied string
	ScalarGuarantee   bool
	AsOfMode          AsOfMode
	WindowDays        *int
	ResolutionErrors  []string
}

// AsOfMode defines how variables are resolved
type AsOfMode string

const (
	AsOfLatestValue AsOfMode = "latest_value_as_of"
	AsOfCountWindow AsOfMode = "count_over_window"
	AsOfSumWindow   AsOfMode = "sum_over_window"
	AsOfExists      AsOfMode = "exists_as_of"
)

// VariableContract represents a variable's resolution rules
type VariableContract struct {
	VarKey           core.VariableKey `json:"var_key"`
	AsOfMode         AsOfMode         `json:"as_of_mode"`
	StatisticalType  StatisticalType  `json:"statistical_type"`
	WindowDays       *int             `json:"window_days,omitempty"`
	ImputationPolicy ImputationPolicy `json:"imputation_policy"`
	ScalarGuarantee  bool             `json:"scalar_guarantee"`
}

// StatisticalType defines variable types for analysis
type StatisticalType string

const (
	TypeNumeric     StatisticalType = "numeric"
	TypeCategorical StatisticalType = "categorical"
	TypeBinary      StatisticalType = "binary"
	TypeTimestamp   StatisticalType = "timestamp"
)

// ImputationPolicy defines how to handle missing values
type ImputationPolicy string

// Constructors
func NewMatrixBundle(snapshotID core.SnapshotID, viewID core.ID, cohortHash core.CohortHash, cutoff core.CutoffAt, lag core.Lag) *MatrixBundle {
	return &MatrixBundle{
		SnapshotID: snapshotID,
		ViewID:     viewID,
		CohortHash: cohortHash,
		CutoffAt:   cutoff,
		Lag:        lag,
		CreatedAt:  core.Now(),
	}
}

// AddColumn adds a resolved column to the matrix
func (b *MatrixBundle) AddColumn(varKey core.VariableKey, values []float64, meta ColumnMeta, audit ResolutionAudit) {
	// Add column to matrix
	if b.Matrix.Data == nil {
		b.Matrix.Data = make([][]float64, len(values))
		b.Matrix.EntityIDs = make([]core.ID, len(values))
		for i := range b.Matrix.Data {
			b.Matrix.Data[i] = make([]float64, 0, 1) // start with capacity for this column
		}
	}

	// Extend each row with this column's values
	for i, value := range values {
		b.Matrix.Data[i] = append(b.Matrix.Data[i], value)
	}

	// Add metadata
	b.Matrix.VariableKeys = append(b.Matrix.VariableKeys, varKey)
	b.ColumnMeta = append(b.ColumnMeta, meta)
	b.Audits = append(b.Audits, audit)
}

// Validate ensures the bundle is internally consistent
func (b *MatrixBundle) Validate() error {
	if len(b.Matrix.Data) == 0 {
		return core.ErrInsufficientData
	}

	rowCount := len(b.Matrix.Data)
	if len(b.Matrix.EntityIDs) != rowCount {
		return core.NewValidationError("entity_ids", "length mismatch with data rows")
	}

	colCount := len(b.Matrix.VariableKeys)
	if len(b.ColumnMeta) != colCount {
		return core.NewValidationError("column_meta", "length mismatch with variable keys")
	}

	// Check each row has the right number of columns
	for i, row := range b.Matrix.Data {
		if len(row) != colCount {
			return core.NewValidationError("matrix_data",
				fmt.Sprintf("row %d has %d columns, expected %d", i, len(row), colCount))
		}
	}

	return nil
}

// GetColumn returns the column index for a variable key
func (b *MatrixBundle) GetColumn(varKey core.VariableKey) (int, bool) {
	for i, key := range b.Matrix.VariableKeys {
		if key == varKey {
			return i, true
		}
	}
	return -1, false
}

// GetColumnData returns the data for a specific column
func (b *MatrixBundle) GetColumnData(varKey core.VariableKey) ([]float64, bool) {
	colIdx, found := b.GetColumn(varKey)
	if !found {
		return nil, false
	}

	data := make([]float64, len(b.Matrix.Data))
	for i, row := range b.Matrix.Data {
		data[i] = row[colIdx]
	}

	return data, true
}

// RowCount returns the number of entities (rows)
func (b *MatrixBundle) RowCount() int {
	return len(b.Matrix.Data)
}

// ColumnCount returns the number of variables (columns)
func (b *MatrixBundle) ColumnCount() int {
	return len(b.Matrix.VariableKeys)
}
