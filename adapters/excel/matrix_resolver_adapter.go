package excel

import (
	"context"
	"fmt"
	"strconv"

	"gohypo/adapters/datareadiness"
	"gohypo/adapters/datareadiness/coercer"
	"gohypo/adapters/datareadiness/synthesizer"
	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
	"gohypo/domain/dataset"
	"gohypo/ports"
)

// ExcelMatrixResolverAdapter implements MatrixResolverPort for Excel files
type ExcelMatrixResolverAdapter struct {
	config       ExcelConfig
	reader       *ExcelReader
	entityColumn string // Auto-detected entity column
	coercer      *coercer.TypeCoercer
	profiler     *datareadiness.ProfilerAdapter
	synthesizer  *synthesizer.ContractSynthesizer
}

// NewExcelMatrixResolverAdapter creates a new Excel-based matrix resolver
func NewExcelMatrixResolverAdapter(config ExcelConfig) *ExcelMatrixResolverAdapter {
	coercerInstance := coercer.NewTypeCoercer(config.CoercionConfig)
	return &ExcelMatrixResolverAdapter{
		config:      config,
		reader:      NewExcelReader(config.FilePath),
		coercer:     coercerInstance,
		profiler:    datareadiness.NewProfilerAdapter(coercerInstance),
		synthesizer: synthesizer.NewContractSynthesizer(config.SynthesisConfig),
	}
}

// ResolveMatrix reads Excel data and processes through data readiness pipeline
func (a *ExcelMatrixResolverAdapter) ResolveMatrix(ctx context.Context, req ports.MatrixResolutionRequest) (*dataset.MatrixBundle, error) {
	// Step 1: Read raw Excel data from Sheet1
	rawData, err := a.reader.ReadData()
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel data: %w", err)
	}

	// Step 2: Auto-detect entity column
	entityColumn, err := a.reader.DetectEntityColumn(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to detect entity column: %w", err)
	}
	a.entityColumn = entityColumn

	// Step 3: Convert to canonical events for profiling
	events, err := a.convertToCanonicalEvents(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to canonical events: %w", err)
	}

	// Step 4: Profile data using standardized profiler
	profilingResult, err := a.profiler.ProfileSource(ctx, "excel_source", events, a.config.ProfilingConfig)
	if err != nil {
		return nil, fmt.Errorf("profiling failed: %w", err)
	}

	// Step 5: Synthesize contracts using standardized synthesizer
	contractDrafts, err := a.synthesizer.SynthesizeContracts(profilingResult.Profiles)
	if err != nil {
		return nil, fmt.Errorf("contract synthesis failed: %w", err)
	}

	// Step 6: Filter to requested variables
	availableDrafts := a.filterRequestedVariables(contractDrafts, req.VarKeys)

	// Step 7: Create MatrixBundle using standardized contracts
	return a.buildMatrixBundle(rawData, availableDrafts, req)
}

// convertToCanonicalEvents creates events for profiling using standardized coercer
func (a *ExcelMatrixResolverAdapter) convertToCanonicalEvents(rawData *ExcelData) ([]ingestion.CanonicalEvent, error) {
	var events []ingestion.CanonicalEvent

	for _, row := range rawData.Rows {
		entityID := row[a.entityColumn]
		if entityID == "" {
			continue // Skip rows without entity ID
		}

		// Create events for each column using standardized type coercion
		for colName, cellValue := range row {
			if colName == a.entityColumn {
				continue // Skip entity column
			}

			// Convert RawRowData to map[string]interface{} for RawPayload
			rawPayload := make(map[string]interface{})
			for k, v := range row {
				rawPayload[k] = v
			}

			event := ingestion.CanonicalEvent{
				EntityID:   core.ID(entityID),
				ObservedAt: core.Now(), // Excel data is point-in-time
				Source:     "excel",
				FieldKey:   colName,
				Value:      a.coercer.CoerceValue(cellValue), // Standardized coercion
				RawPayload: rawPayload,
			}
			events = append(events, event)
		}
	}

	return events, nil
}

// buildMatrixBundle creates the final MatrixBundle using synthesized contracts
func (a *ExcelMatrixResolverAdapter) buildMatrixBundle(
	rawData *ExcelData,
	drafts []synthesizer.ContractDraft,
	req ports.MatrixResolutionRequest,
) (*dataset.MatrixBundle, error) {

	bundle := dataset.NewMatrixBundle(req.SnapshotID, req.ViewID, "", core.NewCutoffAt(core.Now().Time()), core.Lag(0))

	// Build entity index and filter entities
	entityIndex := make(map[string]int)
	var entityIDs []core.ID

	for _, row := range rawData.Rows {
		entityID := row[a.entityColumn]
		if entityID == "" || entityIndex[entityID] > 0 {
			continue
		}

		if a.shouldIncludeEntity(entityID, req.EntityIDs) {
			entityIndex[entityID] = len(entityIDs)
			entityIDs = append(entityIDs, core.ID(entityID))
		}
	}

	if len(entityIDs) == 0 {
		return nil, fmt.Errorf("no valid entities found after filtering")
	}

	bundle.Matrix.EntityIDs = entityIDs
	bundle.Matrix.Data = make([][]float64, len(entityIDs))
	for i := range bundle.Matrix.Data {
		bundle.Matrix.Data[i] = make([]float64, len(drafts))
	}

	// Populate matrix using contract-based resolution
	for colIdx, draft := range drafts {
		contract := draft.ToVariableContract()

		for rowIdx, entityID := range entityIDs {
			// Find the raw data for this entity
			var entityData RawRowData
			for _, row := range rawData.Rows {
				if row[a.entityColumn] == string(entityID) {
					entityData = row
					break
				}
			}

			if entityData == nil {
				continue
			}

			// Get the value for this variable
			rawValue := entityData[draft.VariableKey]

			// Apply standardized type coercion based on contract
			coercedValue := a.coercer.CoerceValue(rawValue)

			// Convert to float64 based on contract type
			floatValue := a.contractValueToFloat64(coercedValue, contract)
			bundle.Matrix.Data[rowIdx][colIdx] = floatValue
		}

		// Add metadata
		bundle.Matrix.VariableKeys = append(bundle.Matrix.VariableKeys, core.VariableKey(draft.VariableKey))

		meta := dataset.ColumnMeta{
			VariableKey:     core.VariableKey(draft.VariableKey),
			StatisticalType: dataset.StatisticalType(draft.StatisticalType),
			DerivedColumns:  []dataset.DerivedColumn{},
			ResolutionAudit: dataset.ResolutionAudit{
				VariableKey:       core.VariableKey(draft.VariableKey),
				MaxTimestamp:      core.Now(),
				RowCount:          len(entityIDs),
				ImputationApplied: "none", // Excel data is complete
				ScalarGuarantee:   true,
				AsOfMode:          dataset.AsOfMode(draft.AsOfMode),
				WindowDays:        draft.WindowDays,
			},
		}
		bundle.ColumnMeta = append(bundle.ColumnMeta, meta)
		bundle.Audits = append(bundle.Audits, meta.ResolutionAudit)
	}

	// Compute fingerprint
	bundle.Fingerprint = core.Hash(fmt.Sprintf("excel-%s-%d-%d", a.config.FilePath, len(entityIDs), len(drafts)))
	bundle.CreatedAt = core.Now()

	return bundle, nil
}

// contractValueToFloat64 converts coerced values to float64 based on contract
func (a *ExcelMatrixResolverAdapter) contractValueToFloat64(value ingestion.Value, contract *dataset.VariableContract) float64 {
	switch contract.StatisticalType {
	case dataset.TypeBinary:
		if value.Type == ingestion.ValueTypeBoolean && value.BooleanVal != nil {
			if *value.BooleanVal {
				return 1.0
			}
			return 0.0
		}
	case dataset.TypeCategorical:
		if value.Type == ingestion.ValueTypeString && value.StringVal != nil {
			// Hash string for categorical encoding
			hash := 0
			for _, r := range *value.StringVal {
				hash = hash*31 + int(r)
			}
			return float64(hash % 1000)
		}
	case dataset.TypeNumeric:
		if value.Type == ingestion.ValueTypeNumeric && value.NumericVal != nil {
			return *value.NumericVal
		}
	}

	// Fallback: try to parse as float
	if value.Type == ingestion.ValueTypeString && value.StringVal != nil {
		if f, err := strconv.ParseFloat(*value.StringVal, 64); err == nil {
			return f
		}
	}

	return 0.0 // Default imputation
}

// Helper methods

func (a *ExcelMatrixResolverAdapter) filterRequestedVariables(drafts []synthesizer.ContractDraft, requested []core.VariableKey) []synthesizer.ContractDraft {
	if len(requested) == 0 {
		return drafts // Return all if no filter specified
	}

	reqSet := make(map[string]bool)
	for _, key := range requested {
		reqSet[string(key)] = true
	}

	var filtered []synthesizer.ContractDraft
	for _, draft := range drafts {
		if reqSet[draft.VariableKey] {
			filtered = append(filtered, draft)
		}
	}
	return filtered
}

func (a *ExcelMatrixResolverAdapter) shouldIncludeEntity(entityID string, requested []core.ID) bool {
	if len(requested) == 0 {
		return true // Include all if no filter
	}
	for _, reqID := range requested {
		if string(reqID) == entityID {
			return true
		}
	}
	return false
}
