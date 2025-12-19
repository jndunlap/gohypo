package stats

import (
	"fmt"

	"gohypo/domain/core"
	"gohypo/domain/stats/brief"
)

// ============================================================================
// STABLE PRIMITIVES (Canonical, never change)
// ============================================================================

// RelationshipKey uniquely identifies a variable pair relationship
type RelationshipKey struct {
	VariableX  core.VariableKey       `json:"variable_x"`
	VariableY  core.VariableKey       `json:"variable_y"`
	TestType   TestType               `json:"test_type"`
	TestParams map[string]interface{} `json:"test_params,omitempty"` // Alpha, tails, corrections, etc.
	FamilyID   core.Hash              `json:"family_id"`             // Family key hash for FDR grouping
}

// CanonicalMetrics contains statistical results that are always comparable
// INVARIANTS:
// - SampleSize (N) always present and > 0
// - PValue always present (0.0 to 1.0)
// - EffectSize always standardized (unitless) or EffectUnit declared
type CanonicalMetrics struct {
	EffectSize       float64 `json:"effect_size"`           // Standardized effect (correlation, difference, etc.)
	EffectUnit       string  `json:"effect_unit,omitempty"` // Unit if not standardized (e.g., "r", "d", "phi")
	PValue           float64 `json:"p_value"`               // Uncorrected p-value (0.0 to 1.0)
	QValue           float64 `json:"q_value,omitempty"`     // FDR-corrected q-value (false discovery rate)
	SampleSize       int     `json:"sample_size"`           // N used in test (> 0)
	TotalComparisons int     `json:"total_comparisons"`     // Total tests in family for FDR
	FDRMethod        string  `json:"fdr_method,omitempty"`  // FDR correction method (e.g., "BH", "BY")
}

// DataQuality captures data characteristics that affect interpretation
// DEPRECATED: Use domain/stats/brief.StatisticalBrief.Quality instead
type DataQuality struct {
	MissingRateX float64 `json:"missing_rate_x"` // Missing data rate for variable X
	MissingRateY float64 `json:"missing_rate_y"` // Missing data rate for variable Y
	UniqueCountX int     `json:"unique_count_x"` // Unique values in X
	UniqueCountY int     `json:"unique_count_y"` // Unique values in Y
	VarianceX    float64 `json:"variance_x"`     // Variance of X (if numeric)
	VarianceY    float64 `json:"variance_y"`     // Variance of Y (if numeric)
	CardinalityX int     `json:"cardinality_x"`  // Cardinality bucket for X
	CardinalityY int     `json:"cardinality_y"`  // Cardinality bucket for Y
}

// NewDataQualityFromBrief creates DataQuality from StatisticalBrief for backward compatibility
func NewDataQualityFromBrief(brief *brief.StatisticalBrief) DataQuality {
	return DataQuality{
		MissingRateX: brief.Quality.MissingRatio,
		MissingRateY: brief.Quality.MissingRatio,                    // Single variable context
		UniqueCountX: brief.SampleSize - brief.Quality.OutlierCount, // Approximation
		UniqueCountY: brief.SampleSize - brief.Quality.OutlierCount, // Approximation
		VarianceX:    brief.Summary.StdDev * brief.Summary.StdDev,
		VarianceY:    brief.Summary.StdDev * brief.Summary.StdDev, // Approximation
		CardinalityX: brief.SampleSize,
		CardinalityY: brief.SampleSize,
	}
}

// DirectionalHints provides Layer 1 hypothesis generation hints
type DirectionalHints struct {
	AssociationDirection string `json:"association_direction,omitempty"` // "positive", "negative", "none"
	SupportsMonotonic    bool   `json:"supports_monotonic"`              // Spearman agrees with Pearson
	EffectUnit           string `json:"effect_unit,omitempty"`           // Unit of effect size (e.g., "r", "d", "phi")
	Standardized         bool   `json:"standardized"`                    // Effect is standardized (unitless)
}

// WarningCode represents structured warning types
type WarningCode string

const (
	WarningPerfectCorrelation WarningCode = "PERFECT_CORRELATION" // r = ±1.0 (likely data leak/transform)
	WarningLowVariance        WarningCode = "LOW_VARIANCE"        // Near-zero variance detected
	WarningLikelyDerived      WarningCode = "LIKELY_DERIVED"      // Variables appear mathematically related
	WarningLowN               WarningCode = "LOW_N"               // Sample size < 30
	WarningHighMissing        WarningCode = "HIGH_MISSING"        // >30% missing in either variable
	WarningSparseData         WarningCode = "SPARSE_DATA"         // Very few non-zero values
)

// ============================================================================
// STAGE-SPECIFIC EVIDENCE (Extensible per stage)
// ============================================================================

// EvidenceBlock contains stage-specific statistical evidence
type EvidenceBlock struct {
	StageName     string                 `json:"stage_name"`              // e.g., "pairwise", "permutation"
	SchemaVersion string                 `json:"schema_version"`          // Version for evidence format evolution
	Method        string                 `json:"method"`                  // e.g., "pearson", "spearman"
	Parameters    map[string]interface{} `json:"parameters,omitempty"`    // Test parameters (alpha, tails, etc.)
	RawResults    map[string]interface{} `json:"raw_results,omitempty"`   // Raw statistical outputs
	SenseResults  []SenseResult          `json:"sense_results,omitempty"` // Five statistical senses results
	Warnings      []WarningCode          `json:"warnings,omitempty"`      // Stage-specific warnings
	ComputedAt    core.Timestamp         `json:"computed_at"`
}

// SenseResult represents output from a statistical sense (matches adapters/stats/senses)
type SenseResult struct {
	SenseName   string                 `json:"sense_name"`
	EffectSize  float64                `json:"effect_size"`
	PValue      float64                `json:"p_value"`
	Confidence  float64                `json:"confidence"`  // 0-1 confidence score
	Signal      string                 `json:"signal"`      // "weak", "moderate", "strong", "very_strong"
	Description string                 `json:"description"` // Human-readable explanation
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ============================================================================
// DOMAIN ARTIFACTS (Layer 0 outputs)
// ============================================================================

// RelationshipArtifact represents a discovered statistical relationship
type RelationshipArtifact struct {
	Key             RelationshipKey  `json:"key"`
	Metrics         CanonicalMetrics `json:"metrics"`
	DataQuality     DataQuality      `json:"data_quality"`
	Directional     DirectionalHints `json:"directional,omitempty"`
	Evidence        []EvidenceBlock  `json:"evidence"`                   // Per-stage evidence
	OverallWarnings []WarningCode    `json:"overall_warnings,omitempty"` // Cross-stage warnings
	Fingerprint     core.Hash        `json:"fingerprint"`                // Deterministic fingerprint
	DiscoveredAt    core.Timestamp   `json:"discovered_at"`
}

// RelationshipPayload defines the flat JSON structure for relationship artifacts
// This ensures compatibility between producers (StatsSweep) and consumers (HypothesisGenerator)
type RelationshipPayload struct {
	// Flattened Key fields
	VariableX  core.VariableKey       `json:"variable_x"`
	VariableY  core.VariableKey       `json:"variable_y"`
	TestType   TestType               `json:"test_type"`
	TestParams map[string]interface{} `json:"test_params,omitempty"`
	FamilyID   core.Hash              `json:"family_id"`

	// Flattened Metrics fields
	EffectSize       float64 `json:"effect_size"`
	EffectUnit       string  `json:"effect_unit,omitempty"`
	PValue           float64 `json:"p_value"`
	QValue           float64 `json:"q_value,omitempty"`
	SampleSize       int     `json:"sample_size"`
	TotalComparisons int     `json:"total_comparisons"`
	FDRMethod        string  `json:"fdr_method,omitempty"`

	// Additional context
	DiscoveredAt core.Timestamp `json:"discovered_at"`
	Fingerprint  core.Hash      `json:"fingerprint"`
	Warnings     []WarningCode  `json:"warnings,omitempty"`
}

// ToPayload converts the artifact to a flat payload
func (r *RelationshipArtifact) ToPayload() RelationshipPayload {
	return RelationshipPayload{
		VariableX:        r.Key.VariableX,
		VariableY:        r.Key.VariableY,
		TestType:         r.Key.TestType,
		TestParams:       r.Key.TestParams,
		FamilyID:         r.Key.FamilyID,
		EffectSize:       r.Metrics.EffectSize,
		EffectUnit:       r.Metrics.EffectUnit,
		PValue:           r.Metrics.PValue,
		QValue:           r.Metrics.QValue,
		SampleSize:       r.Metrics.SampleSize,
		TotalComparisons: r.Metrics.TotalComparisons,
		FDRMethod:        r.Metrics.FDRMethod,
		DiscoveredAt:     r.DiscoveredAt,
		Fingerprint:      r.Fingerprint,
		Warnings:         r.OverallWarnings,
	}
}

// SkippedRelationshipArtifact records why a variable pair was not tested
type SkippedRelationshipArtifact struct {
	Key         RelationshipKey `json:"key"`
	ReasonCode  WarningCode     `json:"reason_code"`  // Why it was skipped
	Counts      map[string]int  `json:"counts"`       // Supporting counts (missing, zeros, etc.)
	DataQuality DataQuality     `json:"data_quality"` // Basic data quality info
	FirstSeenAt core.Timestamp  `json:"first_seen_at"`
}

// VariableEligibilityArtifact records why a variable can/can't participate in specific tests
type VariableEligibilityArtifact struct {
	VariableKey core.VariableKey       `json:"variable_key"`
	TestType    TestType               `json:"test_type"`
	Eligible    bool                   `json:"eligible"`
	ReasonCodes []WarningCode          `json:"reason_codes,omitempty"` // Why ineligible
	DataQuality DataQuality            `json:"data_quality"`           // Variable's data characteristics
	Thresholds  map[string]interface{} `json:"thresholds"`             // Thresholds that were checked
	AssessedAt  core.Timestamp         `json:"assessed_at"`
}

// FDRFamilyArtifact defines a statistical family for FDR correction
type FDRFamilyArtifact struct {
	FamilyID      core.Hash         `json:"family_id"`       // Unique family identifier
	FamilyKey     FamilyKey         `json:"family_key"`      // The key used to compute FamilyID
	NumTests      int               `json:"num_tests"`       // Total tests in this family
	FDRMethod     string            `json:"fdr_method"`      // FDR correction method used
	StagePlanHash core.Hash         `json:"stage_plan_hash"` // Stage plan that defined this family
	SnapshotID    core.SnapshotID   `json:"snapshot_id"`     // Source snapshot
	CohortHash    core.CohortHash   `json:"cohort_hash"`     // Entity cohort
	RegistryHash  core.RegistryHash `json:"registry_hash"`   // Variable registry state
	CreatedAt     core.Timestamp    `json:"created_at"`
}

// FamilyKey defines the grouping criteria for FDR families
type FamilyKey struct {
	SnapshotID    core.SnapshotID   `json:"snapshot_id"`
	CohortHash    core.CohortHash   `json:"cohort_hash"`
	StageName     string            `json:"stage_name"` // e.g., "pairwise"
	TestType      TestType          `json:"test_type"`  // e.g., "pearson"
	RegistryHash  core.RegistryHash `json:"registry_hash"`
	StagePlanHash core.Hash         `json:"stage_plan_hash"`
}

// ============================================================================
// SWEEP METADATA (Complete audit trail)
// ============================================================================

// SweepManifest captures the complete specification and results of a stats sweep
type SweepManifest struct {
	SweepID       core.ID           `json:"sweep_id"`
	SnapshotID    core.SnapshotID   `json:"snapshot_id"`     // Source snapshot
	RegistryHash  core.RegistryHash `json:"registry_hash"`   // Variable registry state
	CohortHash    core.CohortHash   `json:"cohort_hash"`     // Entity cohort specification
	StagePlanHash core.Hash         `json:"stage_plan_hash"` // Stage execution plan
	Seed          int64             `json:"seed"`            // Random seed for determinism

	TestsExecuted    []string `json:"tests_executed"`    // Test types run
	RuntimeMs        int64    `json:"runtime_ms"`        // Total execution time
	TotalComparisons int      `json:"total_comparisons"` // Total pairs evaluated
	SuccessfulTests  int      `json:"successful_tests"`  // Tests that produced results
	SkippedTests     int      `json:"skipped_tests"`     // Tests that were skipped

	RejectionCounts map[WarningCode]int `json:"rejection_counts"` // Structured rejection codes
	ArtifactCounts  map[string]int      `json:"artifact_counts"`  // Count by artifact type

	Fingerprint core.Hash      `json:"fingerprint"` // Complete sweep fingerprint
	CreatedAt   core.Timestamp `json:"created_at"`
}

// ============================================================================
// TYPE DEFINITIONS
// ============================================================================

// TestType defines the statistical test performed
type TestType string

const (
	TestPearson       TestType = "pearson"        // Pearson correlation
	TestSpearman      TestType = "spearman"       // Spearman rank correlation
	TestKendall       TestType = "kendall"        // Kendall tau correlation
	TestChiSquare     TestType = "chisquare"      // Chi-square test
	TestTTest         TestType = "ttest"          // Student's t-test
	TestANOVA         TestType = "anova"          // Analysis of variance
	TestMannWhitney   TestType = "mann_whitney"   // Mann-Whitney U test
	TestKruskalWallis TestType = "kruskal_wallis" // Kruskal-Wallis test
)

// StatisticalType defines variable types for analysis (moved from dataset for DRY)
type StatisticalType string

const (
	TypeNumeric     StatisticalType = "numeric"
	TypeCategorical StatisticalType = "categorical"
	TypeBinary      StatisticalType = "binary"
	TypeTimestamp   StatisticalType = "timestamp"
	TypeText        StatisticalType = "text"
	TypeUnknown     StatisticalType = "unknown"
)

// ============================================================================
// CONSTRUCTORS
// ============================================================================

// NewRelationshipArtifact creates a new relationship artifact with validation
func NewRelationshipArtifact(key RelationshipKey, metrics CanonicalMetrics) (*RelationshipArtifact, error) {
	// Validate invariants
	if err := validateRelationshipArtifact(key, metrics); err != nil {
		return nil, err
	}

	return &RelationshipArtifact{
		Key:             key,
		Metrics:         metrics,
		DataQuality:     DataQuality{},
		Directional:     DirectionalHints{},
		Evidence:        []EvidenceBlock{},
		OverallWarnings: []WarningCode{},
		Fingerprint:     core.Hash("relationship-fingerprint"), // TODO: compute real fingerprint
		DiscoveredAt:    core.Now(),
	}, nil
}

// MustNewRelationshipArtifact creates a relationship artifact (panics on invalid input)
// Use only in tests and development - production code should handle validation errors
func MustNewRelationshipArtifact(key RelationshipKey, metrics CanonicalMetrics) *RelationshipArtifact {
	artifact, err := NewRelationshipArtifact(key, metrics)
	if err != nil {
		panic(err)
	}
	return artifact
}

// validateRelationshipArtifact checks invariants for relationship artifacts
func validateRelationshipArtifact(key RelationshipKey, metrics CanonicalMetrics) error {
	if metrics.SampleSize <= 0 {
		return fmt.Errorf("SampleSize must be > 0, got %d", metrics.SampleSize)
	}
	if metrics.PValue < 0.0 || metrics.PValue > 1.0 {
		return fmt.Errorf("PValue must be in [0.0, 1.0], got %f", metrics.PValue)
	}
	if key.FamilyID == "" {
		return fmt.Errorf("FamilyID must be set")
	}
	if key.VariableX == "" || key.VariableY == "" {
		return fmt.Errorf("VariableX and VariableY must be set")
	}
	return nil
}

// NewSkippedRelationshipArtifact creates a skipped relationship artifact
func NewSkippedRelationshipArtifact(key RelationshipKey, reason WarningCode) *SkippedRelationshipArtifact {
	return &SkippedRelationshipArtifact{
		Key:         key,
		ReasonCode:  reason,
		Counts:      make(map[string]int),
		DataQuality: DataQuality{},
		FirstSeenAt: core.Now(),
	}
}

// NewVariableEligibilityArtifact creates a variable eligibility assessment
func NewVariableEligibilityArtifact(varKey core.VariableKey, testType TestType, eligible bool,
	reasonCodes []WarningCode, dataQuality DataQuality, thresholds map[string]interface{}) *VariableEligibilityArtifact {

	return &VariableEligibilityArtifact{
		VariableKey: varKey,
		TestType:    testType,
		Eligible:    eligible,
		ReasonCodes: reasonCodes,
		DataQuality: dataQuality,
		Thresholds:  thresholds,
		AssessedAt:  core.Now(),
	}
}

// NewFDRFamilyArtifact creates an FDR family definition
func NewFDRFamilyArtifact(familyKey FamilyKey, numTests int, fdrMethod string) *FDRFamilyArtifact {
	familyID := computeFamilyID(familyKey)

	return &FDRFamilyArtifact{
		FamilyID:      familyID,
		FamilyKey:     familyKey,
		NumTests:      numTests,
		FDRMethod:     fdrMethod,
		StagePlanHash: familyKey.StagePlanHash,
		SnapshotID:    familyKey.SnapshotID,
		CohortHash:    familyKey.CohortHash,
		RegistryHash:  familyKey.RegistryHash,
		CreatedAt:     core.Now(),
	}
}

// computeFamilyID generates a deterministic hash for a family key
func computeFamilyID(key FamilyKey) core.Hash {
	// Simple deterministic hash - in production, use proper crypto hash
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		key.SnapshotID, key.CohortHash, key.StageName,
		key.TestType, key.RegistryHash, key.StagePlanHash)
	return core.Hash(fmt.Sprintf("family_%s", data)) // TODO: Use proper hash
}

// ComputeFamilyID is the centralized function for deterministic family assignment
// This should be called by stage runners to ensure consistent family grouping
func ComputeFamilyID(snapshotID core.SnapshotID, cohortHash core.CohortHash,
	stageName string, testType TestType, registryHash core.RegistryHash,
	stagePlanHash core.Hash) core.Hash {

	familyKey := FamilyKey{
		SnapshotID:    snapshotID,
		CohortHash:    cohortHash,
		StageName:     stageName,
		TestType:      testType,
		RegistryHash:  registryHash,
		StagePlanHash: stagePlanHash,
	}

	return computeFamilyID(familyKey)
}

// NewSweepManifest creates a new sweep manifest with complete determinism metadata
func NewSweepManifest(sweepID core.ID, snapshotID core.SnapshotID, registryHash core.RegistryHash,
	cohortHash core.CohortHash, stagePlanHash core.Hash, seed int64) *SweepManifest {

	return &SweepManifest{
		SweepID:          sweepID,
		SnapshotID:       snapshotID,
		RegistryHash:     registryHash,
		CohortHash:       cohortHash,
		StagePlanHash:    stagePlanHash,
		Seed:             seed,
		TestsExecuted:    []string{},
		RuntimeMs:        0,
		TotalComparisons: 0,
		SuccessfulTests:  0,
		SkippedTests:     0,
		RejectionCounts:  make(map[WarningCode]int),
		ArtifactCounts:   make(map[string]int),
		Fingerprint:      core.Hash("sweep-fingerprint"), // TODO: compute real fingerprint
		CreatedAt:        core.Now(),
	}
}
