package resolution

import (
	"context"
	"fmt"
	"time"

	"gohypo/adapters/datareadiness/coercer"
	"gohypo/adapters/datareadiness/synthesizer"
	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
	"gohypo/domain/datareadiness/profiling"
	"gohypo/domain/dataset"
	"gohypo/ports"
)

// ReadinessOrchestratorDeps contains all dependencies for the orchestrator
type ReadinessOrchestratorDeps struct {
	Profiler    ports.ProfilerPort
	Coercer     *coercer.TypeCoercer
	Synthesizer *synthesizer.ContractSynthesizer
	Gate        *ReadinessGate
}

// DataReadinessOrchestrator coordinates the entire data readiness pipeline
type DataReadinessOrchestrator struct {
	deps ReadinessOrchestratorDeps
}

// OrchestratorConfig defines the configuration for the orchestrator
type OrchestratorConfig struct {
	CoercionConfig  coercer.CoercionConfig      `json:"coercion_config"`
	SynthesisConfig synthesizer.SynthesisConfig `json:"synthesis_config"`
	GateConfig      GateConfig                  `json:"gate_config"`
	ProfilingConfig profiling.ProfilingConfig   `json:"profiling_config"`
}

// DefaultOrchestratorConfig returns sensible defaults
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		CoercionConfig:  coercer.DefaultCoercionConfig(),
		SynthesisConfig: synthesizer.DefaultSynthesisConfig(),
		GateConfig:      DefaultGateConfig(),
		ProfilingConfig: profiling.DefaultProfilingConfig(),
	}
}

// NewDataReadinessOrchestrator creates a fully configured orchestrator
func NewDataReadinessOrchestrator(deps ReadinessOrchestratorDeps) (*DataReadinessOrchestrator, error) {
	if deps.Profiler == nil {
		return nil, fmt.Errorf("profiler dependency is required")
	}
	if deps.Coercer == nil {
		return nil, fmt.Errorf("coercer dependency is required")
	}
	if deps.Synthesizer == nil {
		return nil, fmt.Errorf("synthesizer dependency is required")
	}
	if deps.Gate == nil {
		return nil, fmt.Errorf("gate dependency is required")
	}

	fmt.Printf("Data readiness orchestrator initialized with profiler, coercer, synthesizer, and gate\n")

	return &DataReadinessOrchestrator{
		deps: deps,
	}, nil
}

// ProcessSource processes a complete source through the readiness pipeline
func (o *DataReadinessOrchestrator) ProcessSource(ctx context.Context, sourceName string, rawData interface{}) (ReadinessResult, error) {
	startTime := time.Now()

	// Step 1: Ingest and normalize to canonical events
	ingestionResult, events, err := o.ingestSource(sourceName, rawData)
	if err != nil {
		return ReadinessResult{}, fmt.Errorf("ingestion failed: %w", err)
	}

	// Log ingestion results
	if len(ingestionResult.Errors) > 0 {
		fmt.Printf("Warning: %d ingestion errors for source %s\n", len(ingestionResult.Errors), sourceName)
	}

	// Early return if no events were ingested
	if ingestionResult.EventsIngested == 0 {
		return ReadinessResult{}, fmt.Errorf("no events could be ingested from source %s", sourceName)
	}

	// Step 2: Profile all field_keys
	profilingResult, err := o.deps.Profiler.ProfileSource(ctx, sourceName, events, profiling.DefaultProfilingConfig())
	if err != nil {
		return ReadinessResult{}, fmt.Errorf("profiling failed: %w", err)
	}

	// Step 3: Synthesize contract drafts (if synthesizer is available)
	var contractDrafts []synthesizer.ContractDraft
	if o.deps.Synthesizer != nil && len(profilingResult.Profiles) > 0 {
		contractDrafts, err = o.deps.Synthesizer.SynthesizeContracts(profilingResult.Profiles)
		if err != nil {
			// Log warning but continue - contract synthesis is optional
			fmt.Printf("Warning: Contract synthesis failed for source %s: %v\n", sourceName, err)
		} else {
			fmt.Printf("Synthesized %d contract drafts for source %s\n",
				len(contractDrafts), sourceName)
		}
	}

	// Step 4: Apply readiness gates
	readinessResult := o.deps.Gate.EvaluateReadiness(profilingResult.Profiles)

	// Step 5: Apply remediation suggestions
	for i, evaluation := range readinessResult.ReadyVariables {
		readinessResult.ReadyVariables[i] = o.deps.Gate.ApplyRemediation(evaluation)
	}

	// Log final results
	fmt.Printf("Data readiness completed for %s: %d events ingested, %d variables profiled, %d ready, %d rejected (%.2fs)\n",
		sourceName, ingestionResult.EventsIngested, len(profilingResult.Profiles), readinessResult.ReadyCount,
		readinessResult.RejectedCount, time.Since(startTime).Seconds())

	return readinessResult, nil
}

// ingestSource converts raw data to canonical events
func (o *DataReadinessOrchestrator) ingestSource(sourceName string, rawData interface{}) (ingestion.IngestionResult, []ingestion.CanonicalEvent, error) {
	startTime := time.Now()
	events := []ingestion.CanonicalEvent{}
	errors := []ingestion.IngestionError{}

	if rawData == nil {
		result := ingestion.IngestionResult{
			SourceName:     sourceName,
			EventsIngested: 0,
			Errors:         errors,
			DurationMs:     time.Since(startTime).Milliseconds(),
		}
		return result, events, fmt.Errorf("raw data cannot be nil")
	}

	// Handle different data types
	switch data := rawData.(type) {
	case map[string]interface{}:
		// Single object - create one event
		event, err := o.createEventFromObject(sourceName, data)
		if err != nil {
			ingestionErr := ingestion.IngestionError{
				RowIndex:  0,
				Field:     "unknown",
				Value:     fmt.Sprintf("%v", data),
				ErrorType: "event_creation_failed",
				Message:   fmt.Sprintf("Failed to create event from object: %v", err),
			}
			errors = append(errors, ingestionErr)
		} else {
			events = append(events, *event)
		}

	case []interface{}:
		// Array of objects - create one event per object
		for i, item := range data {
			if itemMap, ok := item.(map[string]interface{}); ok {
				event, err := o.createEventFromObject(sourceName, itemMap)
				if err != nil {
					ingestionErr := ingestion.IngestionError{
						RowIndex:  i,
						Field:     fmt.Sprintf("item_%d", i),
						Value:     fmt.Sprintf("%v", item),
						ErrorType: "event_creation_failed",
						Message:   fmt.Sprintf("Failed to create event from array item %d: %v", i, err),
					}
					errors = append(errors, ingestionErr)
				} else {
					events = append(events, *event)
				}
			} else {
				ingestionErr := ingestion.IngestionError{
					RowIndex:  i,
					Field:     fmt.Sprintf("item_%d", i),
					Value:     fmt.Sprintf("%v", item),
					ErrorType: "invalid_data_type",
					Message:   "Array item is not a map/object",
				}
				errors = append(errors, ingestionErr)
			}
		}

	case []map[string]interface{}:
		// Array of maps - create one event per map
		for i, itemMap := range data {
			event, err := o.createEventFromObject(sourceName, itemMap)
			if err != nil {
				ingestionErr := ingestion.IngestionError{
					RowIndex:  i,
					Field:     fmt.Sprintf("item_%d", i),
					Value:     fmt.Sprintf("%v", itemMap),
					ErrorType: "event_creation_failed",
					Message:   fmt.Sprintf("Failed to create event from map item %d: %v", i, err),
				}
				errors = append(errors, ingestionErr)
			} else {
				events = append(events, *event)
			}
		}

	default:
		result := ingestion.IngestionResult{
			SourceName:     sourceName,
			EventsIngested: 0,
			Errors:         errors,
			DurationMs:     time.Since(startTime).Milliseconds(),
		}
		return result, events, fmt.Errorf("unsupported data type: %T", rawData)
	}

	result := ingestion.IngestionResult{
		SourceName:     sourceName,
		EventsIngested: len(events),
		Errors:         errors,
		DurationMs:     time.Since(startTime).Milliseconds(),
	}
	return result, events, nil
}

// createEventFromObject creates a canonical event from a single data object
func (o *DataReadinessOrchestrator) createEventFromObject(sourceName string, data map[string]interface{}) (*ingestion.CanonicalEvent, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data object")
	}

	// Use a default field key for the entire object
	fieldKey := "raw_data"

	// Try to find an ID field for entity identification
	entityID := core.NewID()
	if idValue, exists := data["id"]; exists {
		if idStr, ok := idValue.(string); ok && idStr != "" {
			entityID = core.ID(idStr)
		}
	}

	// Try to find a timestamp field
	observedAt := core.Now()
	if timestampValue, exists := data["timestamp"]; exists {
		if tsStr, ok := timestampValue.(string); ok && tsStr != "" {
			if parsed, err := time.Parse(time.RFC3339, tsStr); err == nil {
				observedAt = core.Timestamp(parsed)
			}
		}
	}

	// Coerce the entire data object as the value
	coercedValue := o.deps.Coercer.CoerceValue(data)

	event := &ingestion.CanonicalEvent{
		EntityID:   entityID,
		ObservedAt: observedAt,
		Source:     sourceName,
		FieldKey:   fieldKey,
		Value:      coercedValue,
		RawPayload: data,
	}

	return event, nil
}

// GetReadyContracts returns the contracts for variables that passed readiness gates
func (o *DataReadinessOrchestrator) GetReadyContracts(result *ReadinessResult, contractDrafts []synthesizer.ContractDraft) ([]*dataset.VariableContract, error) {
	if result == nil {
		return nil, fmt.Errorf("readiness result cannot be nil")
	}

	contracts := make([]*dataset.VariableContract, 0, result.ReadyCount)

	for _, evaluation := range result.ReadyVariables {
		// Find the corresponding contract draft
		var draft *synthesizer.ContractDraft
		for i, d := range contractDrafts {
			if d.VariableKey == evaluation.VariableKey {
				draft = &contractDrafts[i]
				break
			}
		}

		if draft != nil {
			// Only include high-confidence contracts
			if draft.Confidence >= 0.7 {
				contract := draft.ToVariableContract()
				if contract != nil {
					contracts = append(contracts, contract)
				}
			}
		} else {
			// Create a basic contract if no draft is available
			basicContract := &dataset.VariableContract{
				VarKey:           core.VariableKey(evaluation.VariableKey),
				AsOfMode:         dataset.AsOfMode("latest"),         // Default mode
				StatisticalType:  dataset.StatisticalType("numeric"), // Default to numeric
				ImputationPolicy: dataset.ImputationPolicy("drop"),   // Default policy
				ScalarGuarantee:  true,
			}
			contracts = append(contracts, basicContract)
		}
	}

	fmt.Printf("Generated %d variable contracts from %d ready variables\n", len(contracts), result.ReadyCount)
	return contracts, nil
}

// GetAdmissibleVariables returns the list of variables ready for statistical analysis
func (o *DataReadinessOrchestrator) GetAdmissibleVariables(result *ReadinessResult) []AdmissibleVariable {
	if result == nil || len(result.ReadyVariables) == 0 {
		return []AdmissibleVariable{}
	}

	variables := make([]AdmissibleVariable, 0, result.ReadyCount)

	for _, evaluation := range result.ReadyVariables {
		variable := AdmissibleVariable{
			Key:             evaluation.VariableKey,
			Source:          evaluation.Source,
			StatisticalType: string(evaluation.Profile.InferredType),
			Description:     o.generateDescription(evaluation.Profile),
			QualityScore:    evaluation.Profile.QualityScore,
			MissingRate:     evaluation.Profile.MissingStats.MissingRate,
		}
		variables = append(variables, variable)
	}

	fmt.Printf("Prepared %d admissible variables for statistical analysis\n", len(variables))
	return variables
}

// generateDescription creates a human-readable description of a variable
func (o *DataReadinessOrchestrator) generateDescription(profile profiling.FieldProfile) string {
	switch profile.InferredType {
	case profiling.TypeNumeric:
		if profile.TypeSpecific.NumericStats != nil {
			stats := profile.TypeSpecific.NumericStats
			return fmt.Sprintf("Numeric variable (range: %.2f to %.2f, mean: %.2f)",
				stats.Min, stats.Max, stats.Mean)
		}
		return "Numeric variable"

	case profiling.TypeCategorical:
		return fmt.Sprintf("Categorical variable (%d unique values)", profile.Cardinality.UniqueCount)

	case profiling.TypeBoolean:
		return "Boolean/binary variable"

	case profiling.TypeTimestamp:
		return "Timestamp variable"

	default:
		return fmt.Sprintf("Variable of type %s", profile.InferredType)
	}
}

// DataReadinessResult contains the complete outcome of processing a source
type DataReadinessResult struct {
	SourceName       string                      `json:"source_name"`
	IngestionResult  ingestion.IngestionResult   `json:"ingestion_result"`
	ProfilingResult  profiling.ProfilingResult   `json:"profiling_result"`
	ContractDrafts   []synthesizer.ContractDraft `json:"contract_drafts"`
	ReadinessResult  ReadinessResult             `json:"readiness_result"`
	ProcessingTimeMs int64                       `json:"processing_time_ms"`
}

// AdmissibleVariable represents a variable ready for statistical analysis
type AdmissibleVariable struct {
	Key             string  `json:"key"`
	Source          string  `json:"source"`
	StatisticalType string  `json:"statistical_type"`
	Description     string  `json:"description"`
	QualityScore    float64 `json:"quality_score"`
	MissingRate     float64 `json:"missing_rate"`
}

// Summary provides a human-readable summary of the readiness pipeline
func (r *DataReadinessResult) Summary() string {
	return fmt.Sprintf(`Data Readiness Summary for %s:
- Ingested: %d events (%d errors)
- Profiled: %d variables
- Ready: %d variables
- Rejected: %d variables
- Processing time: %d ms`,
		r.SourceName,
		r.IngestionResult.EventsIngested,
		len(r.IngestionResult.Errors),
		r.ProfilingResult.TotalFields,
		r.ReadinessResult.ReadyCount,
		r.ReadinessResult.RejectedCount,
		r.ProcessingTimeMs)
}
