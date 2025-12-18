package app

import (
	"context"
	"fmt"
	"time"

	"gohypo/domain/artifacts"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/stage"
	"gohypo/domain/verdict"
	"gohypo/ports"
)

// HypothesisService orchestrates hypothesis generation and referee validation
type HypothesisService struct {
	generatorPort ports.GeneratorPort
	batteryPort   ports.BatteryPort
	stageRunner   *StageRunner
	ledgerPort    ports.LedgerPort
	rngPort       ports.RNGPort
}

// AuditableHypothesisRequest defines inputs for hypothesis generation
type AuditableHypothesisRequest struct {
	RunID            core.RunID
	MatrixBundleID   core.ID
	RelationshipArts []core.Artifact
	MaxHypotheses    int
	RigorProfile     stage.RigorProfile
	Seed             int64
}

// HypothesisGenerationResult contains generated hypotheses with audit trail
type HypothesisGenerationResult struct {
	RunID       core.RunID
	Hypotheses  []core.Artifact
	Manifest    core.Artifact
	Fingerprint core.Hash
	RuntimeMs   int64
	Success     bool
}

// NewHypothesisService creates a hypothesis service
func NewHypothesisService(generatorPort ports.GeneratorPort, batteryPort ports.BatteryPort, stageRunner *StageRunner, ledgerPort ports.LedgerPort, rngPort ports.RNGPort) *HypothesisService {
	return &HypothesisService{
		generatorPort: generatorPort,
		batteryPort:   batteryPort,
		stageRunner:   stageRunner,
		ledgerPort:    ledgerPort,
		rngPort:       rngPort,
	}
}

// ProposeHypotheses generates hypothesis candidates from relationship artifacts
func (s *HypothesisService) ProposeHypotheses(ctx context.Context, req AuditableHypothesisRequest) (*HypothesisGenerationResult, error) {
	startTime := time.Now()

	// Build generation context
	genContext := ports.HypothesisContext{
		RelationshipArts: req.RelationshipArts,
		// TODO: Add dead zones, mechanism taxonomy, variable contracts
	}

	// Generate hypotheses
	hypotheses, err := s.generatorPort.GenerateHypotheses(ctx, ports.HypothesisRequest{
		Context:       genContext,
		MaxHypotheses: req.MaxHypotheses,
		RigorProfile:  ports.RigorProfile(req.RigorProfile),
	})
	if err != nil {
		return nil, fmt.Errorf("hypothesis generation failed: %w", err)
	}

	// Convert to artifacts and persist
	hypothesisArtifacts := s.convertHypothesesToArtifacts(hypotheses.Candidates, req.RunID)
	for _, artifact := range hypothesisArtifacts {
		// Keep ledger clean: validate via artifact registry before persisting
		if err := artifacts.ValidateArtifact(artifact); err != nil {
			return nil, fmt.Errorf("hypothesis artifact validation failed: %w", err)
		}
		if err := s.ledgerPort.StoreArtifact(ctx, string(req.RunID), artifact); err != nil {
			return nil, fmt.Errorf("failed to store hypothesis artifact: %w", err)
		}
	}

	// Persist generation audit artifact (prompt/response hashes, model, drops)
	manifest := s.createGenerationAudit(req, hypotheses, len(hypothesisArtifacts))

	// Persist manifest
	if err := artifacts.ValidateArtifact(manifest); err != nil {
		return nil, fmt.Errorf("generation audit artifact validation failed: %w", err)
	}
	if err := s.ledgerPort.StoreArtifact(ctx, string(req.RunID), manifest); err != nil {
		return nil, fmt.Errorf("failed to store manifest: %w", err)
	}

	// Compute fingerprint
	fingerprint := s.computeGenerationFingerprint(req, hypothesisArtifacts, manifest)

	runtimeMs := time.Since(startTime).Milliseconds()

	result := &HypothesisGenerationResult{
		RunID:       req.RunID,
		Hypotheses:  hypothesisArtifacts,
		Manifest:    manifest,
		Fingerprint: fingerprint,
		RuntimeMs:   runtimeMs,
		Success:     true,
	}

	return result, nil
}

// ExecuteRun validates hypotheses through the referee pipeline
func (s *HypothesisService) ExecuteRun(ctx context.Context, req ExecuteRunRequest) (*CausalRunResult, error) {
	startTime := time.Now()

	// Load hypothesis artifact
	hypothesisArtifact, err := s.ledgerPort.GetArtifact(ctx, core.ArtifactID(req.HypothesisID))
	if err != nil {
		return nil, fmt.Errorf("failed to load hypothesis: %w", err)
	}

	// Extract hypothesis from artifact
	hypothesis, err := s.extractHypothesisFromArtifact(hypothesisArtifact)
	if err != nil {
		return nil, fmt.Errorf("failed to extract hypothesis: %w", err)
	}

	// Load matrix bundle for validation
	matrixBundle, err := s.loadMatrixBundle(ctx, req.MatrixBundleID)
	if err != nil {
		return nil, fmt.Errorf("failed to load matrix bundle: %w", err)
	}

	// Execute referee validation using BatteryPort (PermutationReferee)
	validationResult, err := s.batteryPort.ValidateHypothesis(ctx, hypothesis.ID, matrixBundle)
	if err != nil {
		return nil, fmt.Errorf("referee validation failed: %w", err)
	}

	// Map validation result to verdict and checklist
	verdictStatus := s.mapValidationStatus(validationResult.Status)
	checklist := s.createChecklistFromValidation(validationResult)

	// Create causal run artifact
	causalRun := s.createCausalRunArtifact(hypothesis, req, verdictStatus, checklist, validationResult)

	// Persist artifacts
	if err := s.ledgerPort.StoreArtifact(ctx, string(req.RunID), causalRun); err != nil {
		return nil, fmt.Errorf("failed to store causal run: %w", err)
	}

	// Persist falsification log if present
	if validationResult.FalsificationLog != nil {
		falsificationArtifact := s.createFalsificationArtifact(req, validationResult)
		if err := s.ledgerPort.StoreArtifact(ctx, string(req.RunID), falsificationArtifact); err != nil {
			return nil, fmt.Errorf("failed to store falsification log: %w", err)
		}
	}

	fingerprint := s.computeRunFingerprint(req, causalRun)
	runtimeMs := time.Since(startTime).Milliseconds()

	result := &CausalRunResult{
		RunID:       req.RunID,
		CausalRun:   causalRun,
		Checklist:   checklist,
		Verdict:     verdictStatus,
		Fingerprint: fingerprint,
		RuntimeMs:   runtimeMs,
		Success:     true,
	}

	return result, nil
}

// convertHypothesesToArtifacts transforms hypothesis candidates into domain artifacts
func (s *HypothesisService) convertHypothesesToArtifacts(candidates []ports.HypothesisCandidate, runID core.RunID) []core.Artifact {
	artifacts := make([]core.Artifact, len(candidates))

	for i, candidate := range candidates {
		artifacts[i] = core.Artifact{
			ID:   core.NewID(),
			Kind: core.ArtifactHypothesis,
			Payload: map[string]interface{}{
				"run_id":               runID,
				"cause_key":            candidate.CauseKey,
				"effect_key":           candidate.EffectKey,
				"mechanism_category":   candidate.MechanismCategory,
				"confounder_keys":      candidate.ConfounderKeys,
				"rationale":            candidate.Rationale,
				"suggested_rigor":      candidate.SuggestedRigor,
				"generator_type":       candidate.GeneratorType,
				"supporting_artifacts": candidate.SupportingArtifacts,
			},
			CreatedAt: core.Now(),
		}
	}

	return artifacts
}

// createGenerationAudit creates an audit trail for hypothesis generation.
// This is persisted as an artifact so changes are explainable (prompt hash + response hash).
func (s *HypothesisService) createGenerationAudit(req AuditableHypothesisRequest, gen *ports.HypothesisGeneration, generatedCount int) core.Artifact {
	return core.Artifact{
		ID:   core.NewID(),
		Kind: core.ArtifactVariableHealth, // TODO: Add dedicated audit artifact kind
		Payload: map[string]interface{}{
			"operation":            "hypothesis_generation_audit",
			"run_id":               req.RunID,
			"matrix_bundle_id":     req.MatrixBundleID,
			"relationships_used":   len(req.RelationshipArts),
			"hypotheses_generated": generatedCount,
			"max_requested":        req.MaxHypotheses,
			"rigor_profile":        req.RigorProfile,
			"generator_type":       gen.Audit.GeneratorType,
			"model":                gen.Audit.Model,
			"temperature":          gen.Audit.Temperature,
			"max_tokens":           gen.Audit.MaxTokens,
			"prompt_hash":          gen.Audit.PromptHash,
			"response_hash":        gen.Audit.ResponseHash,
			"dropped":              gen.Audit.Dropped,
		},
		CreatedAt: core.Now(),
	}
}

// computeGenerationFingerprint creates deterministic fingerprint
func (s *HypothesisService) computeGenerationFingerprint(req AuditableHypothesisRequest, hypotheses []core.Artifact, manifest core.Artifact) core.Hash {
	data := fmt.Sprintf("%s|%s|%d|%d", req.RunID, req.MatrixBundleID, len(hypotheses), req.Seed)
	return core.NewHash([]byte(data))
}

// extractHypothesisFromArtifact extracts hypothesis data from an artifact
func (s *HypothesisService) extractHypothesisFromArtifact(artifact *core.Artifact) (*Hypothesis, error) {
	if artifact.Kind != core.ArtifactHypothesis {
		return nil, fmt.Errorf("artifact is not a hypothesis: %s", artifact.Kind)
	}

	payload, ok := artifact.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid hypothesis payload type")
	}

	causeKeyStr, _ := payload["cause_key"].(string)
	effectKeyStr, _ := payload["effect_key"].(string)
	mechanismCategory, _ := payload["mechanism_category"].(string)
	rationale, _ := payload["rationale"].(string)

	causeKey, err := core.ParseVariableKey(causeKeyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid cause_key: %w", err)
	}

	effectKey, err := core.ParseVariableKey(effectKeyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid effect_key: %w", err)
	}

	// Extract confounder keys
	var confounderKeys []core.VariableKey
	if confounders, ok := payload["confounder_keys"].([]interface{}); ok {
		for _, c := range confounders {
			if cStr, ok := c.(string); ok {
				if cKey, err := core.ParseVariableKey(cStr); err == nil {
					confounderKeys = append(confounderKeys, cKey)
				}
			}
		}
	}

	return &Hypothesis{
		ID:                core.HypothesisID(artifact.ID),
		CauseKey:          causeKey,
		EffectKey:         effectKey,
		MechanismCategory: mechanismCategory,
		ConfounderKeys:    confounderKeys,
		Rationale:         rationale,
	}, nil
}

// mapValidationStatus maps verdict.VerdictStatus to ValidationVerdict
func (s *HypothesisService) mapValidationStatus(status verdict.VerdictStatus) ValidationVerdict {
	switch status {
	case verdict.StatusValidated:
		return VerdictSignal
	case verdict.StatusRejected:
		return VerdictNoise
	case verdict.StatusMarginal:
		return VerdictUnstable
	default:
		return VerdictInadmissible
	}
}

// createChecklistFromValidation creates a RunChecklist from ValidationResult
func (s *HypothesisService) createChecklistFromValidation(result *ports.ValidationResult) *RunChecklist {
	checklist := &RunChecklist{
		PhantomGateResult:       result.Status == verdict.StatusValidated,
		ConfounderStressResult:  result.PValue < 0.05,
		ConditionalIndependence: result.Status == verdict.StatusValidated,
		NestedModelResult:       result.EffectSize,
		StabilityScore:          result.Confidence,
	}
	return checklist
}

// createCausalRunArtifact creates the validation result artifact
func (s *HypothesisService) createCausalRunArtifact(hypothesis *Hypothesis, req ExecuteRunRequest, verdictStatus ValidationVerdict, checklist *RunChecklist, validationResult *ports.ValidationResult) core.Artifact {
	return core.Artifact{
		ID:   core.NewID(),
		Kind: core.ArtifactRun,
		Payload: map[string]interface{}{
			"hypothesis_id": hypothesis.ID,
			"run_id":        req.RunID,
			"verdict":       verdictStatus,
			"rigor_profile": req.RigorProfile,
			"dataset_spec":  req.DatasetSpec,
			"validation": map[string]interface{}{
				"status":           validationResult.Status,
				"reason":           validationResult.Reason,
				"p_value":          validationResult.PValue,
				"confidence":       validationResult.Confidence,
				"effect_size":      validationResult.EffectSize,
				"null_percentile":  validationResult.NullPercentile,
				"num_permutations": validationResult.NumPermutations,
			},
			"checklist": map[string]interface{}{
				"phantom_gate":             checklist.PhantomGateResult,
				"confounder_stress":        checklist.ConfounderStressResult,
				"conditional_independence": checklist.ConditionalIndependence,
				"nested_model_result":      checklist.NestedModelResult,
				"stability_score":          checklist.StabilityScore,
			},
		},
		CreatedAt: core.Now(),
	}
}

// createFalsificationArtifact creates an artifact for falsification logs
func (s *HypothesisService) createFalsificationArtifact(req ExecuteRunRequest, validationResult *ports.ValidationResult) core.Artifact {
	return core.Artifact{
		ID:   core.NewID(),
		Kind: core.ArtifactVariableHealth, // TODO: Add dedicated falsification artifact kind
		Payload: map[string]interface{}{
			"operation":            "falsification_log",
			"hypothesis_id":        validationResult.HypothesisID,
			"run_id":               req.RunID,
			"reason":               validationResult.Reason,
			"permutation_p_value":  validationResult.FalsificationLog.PermutationPValue,
			"observed_effect_size": validationResult.FalsificationLog.ObservedEffectSize,
			"null_distribution": map[string]interface{}{
				"mean":          validationResult.FalsificationLog.NullDistribution.Mean,
				"std_dev":       validationResult.FalsificationLog.NullDistribution.StdDev,
				"min":           validationResult.FalsificationLog.NullDistribution.Min,
				"max":           validationResult.FalsificationLog.NullDistribution.Max,
				"percentile_95": validationResult.FalsificationLog.NullDistribution.Percentile95,
				"percentile_99": validationResult.FalsificationLog.NullDistribution.Percentile99,
			},
			"sample_size": validationResult.FalsificationLog.SampleSize,
			"test_used":   validationResult.FalsificationLog.TestUsed,
			"variable_x":  string(validationResult.FalsificationLog.VariableX),
			"variable_y":  string(validationResult.FalsificationLog.VariableY),
			"rejected_at": validationResult.FalsificationLog.RejectedAt,
		},
		CreatedAt: core.Now(),
	}
}

// computeRunFingerprint creates validation fingerprint
func (s *HypothesisService) computeRunFingerprint(req ExecuteRunRequest, causalRun core.Artifact) core.Hash {
	data := fmt.Sprintf("%s|%s|%s", req.RunID, req.HypothesisID, req.DatasetSpec)
	return core.NewHash([]byte(data))
}

// Helper types and methods
type Hypothesis struct {
	ID                core.HypothesisID
	CauseKey          core.VariableKey
	EffectKey         core.VariableKey
	MechanismCategory string
	ConfounderKeys    []core.VariableKey
	Rationale         string
}

type ExecuteRunRequest struct {
	RunID          core.RunID
	HypothesisID   core.HypothesisID
	MatrixBundleID core.ID
	RigorProfile   stage.RigorProfile
	DatasetSpec    string
}

type CausalRunResult struct {
	RunID       core.RunID
	CausalRun   core.Artifact
	Checklist   *RunChecklist
	Verdict     ValidationVerdict
	Fingerprint core.Hash
	RuntimeMs   int64
	Success     bool
}

type ValidationVerdict string

const (
	VerdictSignal       ValidationVerdict = "provisional_signal"
	VerdictNoise        ValidationVerdict = "noise"
	VerdictConfounded   ValidationVerdict = "confounded"
	VerdictUnstable     ValidationVerdict = "unstable"
	VerdictInadmissible ValidationVerdict = "inadmissible"
)

type RunChecklist struct {
	PhantomGateResult       bool
	ConfounderStressResult  bool
	ConditionalIndependence bool
	NestedModelResult       float64
	StabilityScore          float64
}

// loadMatrixBundle loads a matrix bundle from storage
// For MVP, this is a placeholder that needs to be implemented based on your storage layer
func (s *HypothesisService) loadMatrixBundle(ctx context.Context, bundleID core.ID) (*dataset.MatrixBundle, error) {
	// TODO: Implement loading from storage based on your storage implementation
	// This should load the MatrixBundle that was created during the stats sweep
	// For now, return an error indicating this needs implementation
	return nil, fmt.Errorf("loadMatrixBundle not yet implemented - needs storage layer integration")
}
