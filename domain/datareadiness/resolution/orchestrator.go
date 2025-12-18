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

	return &DataReadinessOrchestrator{
		deps: deps,
	}, nil
}

// ProcessSource processes a complete source through the readiness pipeline
func (o *DataReadinessOrchestrator) ProcessSource(ctx context.Context, sourceName string, rawData interface{}) (*ReadinessResult, error) {
	startTime := time.Now()

	// Step 1: Ingest and normalize to canonical events
	events, ingestErrors, err := o.ingestSource(rawData)
	if err != nil {
		return nil, fmt.Errorf("ingestion failed: %w", err)
	}

	ingestResult := ingestion.IngestionResult{
		SourceName:     sourceName,
		EventsIngested: len(events),
		Errors:         ingestErrors,
		DurationMs:     time.Since(startTime).Milliseconds(),
	}

	// Step 2: Profile all field_keys
	profilingResult, err := o.deps.Profiler.ProfileSource(ctx, sourceName, events, profiling.DefaultProfilingConfig())
	if err != nil {
		return nil, fmt.Errorf("profiling failed: %w", err)
	}

	// Step 3: Synthesize contract drafts
	contractDrafts, err := o.deps.Synthesizer.SynthesizeContracts(profilingResult.Profiles)
	if err != nil {
		return nil, fmt.Errorf("contract synthesis failed: %w", err)
	}

	// Step 4: Apply readiness gates
	readinessResult := o.deps.Gate.EvaluateReadiness(profilingResult.Profiles)

	// Step 5: Apply remediation suggestions
	for i, evaluation := range readinessResult.ReadyVariables {
		readinessResult.ReadyVariables[i] = o.deps.Gate.ApplyRemediation(evaluation)
	}

	// Step 6: Create final result (for future use)
	_ = &DataReadinessResult{
		SourceName:       sourceName,
		IngestionResult:  ingestResult,
		ProfilingResult:  *profilingResult,
		ContractDrafts:   contractDrafts,
		ReadinessResult:  readinessResult,
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
	}

	return &readinessResult, nil
}

// ingestSource converts raw data to canonical events (mock implementation)
func (o *DataReadinessOrchestrator) ingestSource(rawData interface{}) ([]ingestion.CanonicalEvent, []ingestion.IngestionError, error) {
	// This is a placeholder - in reality, you'd have source-specific normalizers
	// that convert CSV, JSON, database tables, etc. to canonical events

	events := []ingestion.CanonicalEvent{}
	errors := []ingestion.IngestionError{}

	// Mock implementation for demonstration - handle both single object and array
	if data, ok := rawData.(map[string]interface{}); ok {
		// Single object - create one event with raw payload
		event := ingestion.CanonicalEvent{
			EntityID:   core.NewID(),
			ObservedAt: core.Now(),
			Source:     "test_source",
			FieldKey:   "test_data",
			Value:      o.deps.Coercer.CoerceValue(data["test"]),
			RawPayload: data, // Include raw data for profiling
		}
		events = append(events, event)
	} else if dataArray, ok := rawData.([]interface{}); ok {
		// Array of objects
		for _, item := range dataArray {
			if itemData, ok := item.(map[string]interface{}); ok {
				event := ingestion.CanonicalEvent{
					EntityID:   core.NewID(),
					ObservedAt: core.Now(),
					Source:     "test_source",
					FieldKey:   "test_data",
					Value:      o.deps.Coercer.CoerceValue(itemData["test"]),
					RawPayload: itemData,
				}
				events = append(events, event)
			}
		}
	}

	return events, errors, nil
}

// GetReadyContracts returns the contracts for variables that passed readiness gates
func (o *DataReadinessOrchestrator) GetReadyContracts(result *ReadinessResult, contractDrafts []synthesizer.ContractDraft) ([]*dataset.VariableContract, error) {
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

		if draft != nil && draft.Confidence >= 0.7 { // Only high-confidence contracts
			contract := draft.ToVariableContract()
			contracts = append(contracts, contract)
		}
	}

	return contracts, nil
}

// GetAdmissibleVariables returns the list of variables ready for statistical analysis
func (o *DataReadinessOrchestrator) GetAdmissibleVariables(result *ReadinessResult) []AdmissibleVariable {
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
