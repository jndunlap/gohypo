package app

import (
	"context"
	"fmt"
	"time"

	"gohypo/domain/artifacts"
	"gohypo/domain/core"
	"gohypo/domain/stage"
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

	// Load hypothesis
	hypothesis, err := s.loadHypothesis(ctx, req.HypothesisID)
	if err != nil {
		return nil, fmt.Errorf("failed to load hypothesis: %w", err)
	}

	// Load matrix bundle for validation
	matrixBundle, err := s.loadMatrixBundle(ctx, req.MatrixBundleID)
	if err != nil {
		return nil, fmt.Errorf("failed to load matrix bundle: %w", err)
	}

	// Execute referee validation
	checklist, verdict, err := s.executeRefereeValidation(ctx, hypothesis, matrixBundle, req.RigorProfile)
	if err != nil {
		return nil, fmt.Errorf("referee validation failed: %w", err)
	}

	// Create causal run artifact
	causalRun := s.createCausalRunArtifact(hypothesis, req, verdict, checklist)

	// Persist artifacts
	if err := s.ledgerPort.StoreArtifact(ctx, string(req.RunID), causalRun); err != nil {
		return nil, fmt.Errorf("failed to store causal run: %w", err)
	}

	fingerprint := s.computeRunFingerprint(req, causalRun)
	runtimeMs := time.Since(startTime).Milliseconds()

	result := &CausalRunResult{
		RunID:       req.RunID,
		CausalRun:   causalRun,
		Checklist:   checklist,
		Verdict:     verdict,
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

// executeRefereeValidation runs the complete validation battery
func (s *HypothesisService) executeRefereeValidation(ctx context.Context, hypothesis *Hypothesis, matrixBundle *MatrixBundle, rigor stage.RigorProfile) (*RunChecklist, ValidationVerdict, error) {
	checklist := &RunChecklist{}

	// Triage battery (always run)
	triageResults, err := s.runTriageBattery(ctx, hypothesis, matrixBundle)
	if err != nil {
		return nil, VerdictInadmissible, err
	}
	checklist.PhantomGateResult = triageResults.PhantomGate
	checklist.ConfounderStressResult = triageResults.ConfounderStress

	// Check if triage passes
	if !triageResults.PhantomGate || !triageResults.ConfounderStress {
		verdict := VerdictNoise
		if !triageResults.ConfounderStress {
			verdict = VerdictConfounded
		}
		return checklist, verdict, nil
	}

	// Decision battery (if rigor requires and triage passes)
	if rigor == stage.RigorDecision {
		decisionResults, err := s.runDecisionBattery(ctx, hypothesis, matrixBundle)
		if err != nil {
			return nil, VerdictUnstable, err
		}
		checklist.ConditionalIndependence = decisionResults.ConditionalIndependence
		checklist.NestedModelResult = decisionResults.NestedModelResult
		checklist.StabilityScore = decisionResults.StabilityScore

		if !decisionResults.ConditionalIndependence || decisionResults.NestedModelResult < 0.01 {
			return checklist, VerdictConfounded, nil
		}

		if decisionResults.StabilityScore < 0.7 {
			return checklist, VerdictUnstable, nil
		}
	}

	return checklist, VerdictSignal, nil
}

// createCausalRunArtifact creates the validation result artifact
func (s *HypothesisService) createCausalRunArtifact(hypothesis *Hypothesis, req ExecuteRunRequest, verdict ValidationVerdict, checklist *RunChecklist) core.Artifact {
	return core.Artifact{
		ID:   core.NewID(),
		Kind: core.ArtifactRun,
		Payload: map[string]interface{}{
			"hypothesis_id": hypothesis.ID,
			"run_id":        req.RunID,
			"verdict":       verdict,
			"rigor_profile": req.RigorProfile,
			"dataset_spec":  req.DatasetSpec,
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

// Placeholder implementations for battery tests
func (s *HypothesisService) runTriageBattery(ctx context.Context, hypothesis *Hypothesis, matrixBundle *MatrixBundle) (*TriageResults, error) {
	// TODO: Implement actual battery tests
	return &TriageResults{
		PhantomGate:      true, // Assume passes for demo
		ConfounderStress: true, // Assume passes for demo
	}, nil
}

func (s *HypothesisService) runDecisionBattery(ctx context.Context, hypothesis *Hypothesis, matrixBundle *MatrixBundle) (*DecisionResults, error) {
	// TODO: Implement actual decision tests
	return &DecisionResults{
		ConditionalIndependence: true, // Assume passes for demo
		NestedModelResult:       0.15, // Assume significant improvement
		StabilityScore:          0.85, // Assume stable
	}, nil
}

type TriageResults struct {
	PhantomGate      bool
	ConfounderStress bool
}

type DecisionResults struct {
	ConditionalIndependence bool
	NestedModelResult       float64
	StabilityScore          float64
}

// Placeholder implementations for loading data
func (s *HypothesisService) loadHypothesis(ctx context.Context, hypothesisID core.HypothesisID) (*Hypothesis, error) {
	// TODO: Implement loading from ledger
	return &Hypothesis{
		ID:                hypothesisID,
		CauseKey:          "inspection_count",
		EffectKey:         "severity_score",
		MechanismCategory: "direct_causal",
	}, nil
}

func (s *HypothesisService) loadMatrixBundle(ctx context.Context, bundleID core.ID) (*MatrixBundle, error) {
	// TODO: Implement loading from storage
	return &MatrixBundle{}, nil
}

// MatrixBundle placeholder
type MatrixBundle struct {
	// TODO: Import from dataset package
}
