package testkit

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"gohypo/adapters/datareadiness"
	"gohypo/adapters/datareadiness/coercer"
	"gohypo/adapters/datareadiness/synthesizer"
	"gohypo/adapters/excel"
	"gohypo/app"
	"gohypo/domain/core"
	"gohypo/domain/datareadiness/resolution"
	"gohypo/domain/dataset"
	"gohypo/domain/run"
	"gohypo/domain/stage"
	"gohypo/ports"
)

// TestKit provides testing utilities and fixtures
type TestKit struct {
	ledger      *InMemoryLedgerAdapter // Shared ledger instance
	excelConfig *excel.ExcelConfig     // Excel data source configuration
	excelData   *excel.ExcelData       // Pre-loaded Excel data for fast access
	excelLoaded bool                   // Whether Excel data has been loaded
}

// NewTestKit creates a new test kit instance with synthetic data
func NewTestKit() (*TestKit, error) {
	ledger := NewInMemoryLedgerAdapter()
	return &TestKit{ledger: ledger}, nil
}

// NewTestKitWithExcel creates a new test kit instance with Excel data configuration
func NewTestKitWithExcel(excelConfig *excel.ExcelConfig) (*TestKit, error) {
	ledger := NewInMemoryLedgerAdapter()

	// Log Excel file information and read immediately for fast metadata access
	log.Printf("Initializing test kit with Excel file: %s", excelConfig.FilePath)

	// Read Excel data immediately to cache metadata and ensure fast access
	excelStart := time.Now()
	reader := excel.NewDataReader(excelConfig.FilePath)
	data, err := reader.ReadData()
	if err != nil {
		log.Printf("Warning: Could not read Excel file during initialization: %v", err)
		// Continue without Excel data - matrix resolver will handle errors gracefully
		return &TestKit{
			ledger:      ledger,
			excelConfig: excelConfig,
			excelLoaded: false,
		}, nil
	}

	excelTime := time.Since(excelStart)
	log.Printf("[Performance] Excel file pre-loaded in %.2fms (%d columns, %d rows)",
		float64(excelTime.Nanoseconds())/1e6, len(data.Headers), len(data.Rows))

	return &TestKit{
		ledger:      ledger,
		excelConfig: excelConfig,
		excelData:   data,
		excelLoaded: true,
	}, nil
}

// MatrixResolverAdapter returns a matrix resolver adapter
func (t *TestKit) MatrixResolverAdapter() ports.MatrixResolverPort {
	// Return Excel adapter if configured, otherwise synthetic data
	if t.excelConfig != nil && t.excelConfig.Enabled {
		if t.excelLoaded && t.excelData != nil {
			log.Printf("[Performance] Using pre-loaded Excel data for matrix resolver")
			return excel.NewExcelMatrixResolverAdapterWithData(*t.excelConfig, t.excelData)
		} else {
			log.Printf("[Performance] Excel data not pre-loaded, will read on demand")
			return excel.NewExcelMatrixResolverAdapter(*t.excelConfig)
		}
	}
	return NewFakeMatrixResolverAdapter()
}

// RegistryAdapter returns a registry adapter
func (t *TestKit) RegistryAdapter() ports.RegistryPort {
	// Return a fake data implementation
	return &FakeRegistryAdapter{}
}

// RNGAdapter returns an RNG adapter
func (t *TestKit) RNGAdapter() ports.RNGPort {
	// Return a stub implementation
	return &RNGAdapter{}
}

// LedgerReaderAdapter returns a ledger reader adapter for UI
func (t *TestKit) LedgerReaderAdapter() ports.LedgerReaderPort {
	// Share the same storage as LedgerAdapter
	return t.ledger
}

// CreateTestMatrixBundle creates a test matrix bundle with realistic fake data
func (t *TestKit) CreateTestMatrixBundle(ctx context.Context, matrixBundleID string) (*dataset.MatrixBundle, error) {
	// Use the fake resolver to generate realistic data
	resolver := NewFakeMatrixResolverAdapter()

	variableKeys := []core.VariableKey{"inspection_count", "severity_score", "region", "has_violation"}
	entityCount := 1000
	entityIDs := make([]core.ID, entityCount)
	for i := range entityIDs {
		entityIDs[i] = core.ID(fmt.Sprintf("entity_%d", i+1))
	}

	req := ports.MatrixResolutionRequest{
		VarKeys:    variableKeys,
		EntityIDs:  entityIDs,
		SnapshotID: core.SnapshotID(matrixBundleID),
		ViewID:     core.ID("test_view"),
	}

	bundle, err := resolver.ResolveMatrix(ctx, req)
	if err != nil {
		return nil, err
	}

	bundle.SnapshotID = core.SnapshotID(matrixBundleID)
	return bundle, nil
}

// CreateTestSnapshot creates a test snapshot
func (t *TestKit) CreateTestSnapshot(ctx context.Context, datasetName string, entityCount int, seed int64) (core.SnapshotID, error) {
	// Return a stub snapshot ID
	return core.SnapshotID("test-snapshot-" + datasetName), nil
}

// RegisterTestContract registers a test contract
func (t *TestKit) RegisterTestContract(ctx context.Context, varKey core.VariableKey, contract *TestContract) error {
	// Stub implementation
	return nil
}

// GenerateTestEvents generates test events
func (t *TestKit) GenerateTestEvents(ctx context.Context, snapshotID core.SnapshotID, count int) error {
	// Stub implementation
	return nil
}

// ResolveTestMatrix resolves a test matrix
func (t *TestKit) ResolveTestMatrix(ctx context.Context, snapshotID core.SnapshotID, varKeys []string) (*dataset.MatrixBundle, error) {
	// Return a stub implementation
	return &dataset.MatrixBundle{
		SnapshotID: snapshotID,
	}, nil
}

// RunTestStatsSweep runs a test stats sweep
func (t *TestKit) RunTestStatsSweep(ctx context.Context, snapshotID core.SnapshotID) (*stage.PipelineResult, error) {
	// Return a stub implementation
	return &stage.PipelineResult{}, nil
}

// GetTestRun gets a test run
func (t *TestKit) GetTestRun(ctx context.Context, runID core.RunID) (*TestRun, error) {
	// Return a stub implementation
	return &TestRun{
		Fingerprint: core.Hash("test-fingerprint"),
	}, nil
}

// ReplayRun replays a run
func (t *TestKit) ReplayRun(ctx context.Context, fingerprint core.Hash) (*TestRun, error) {
	// Return a stub implementation
	return &TestRun{
		Fingerprint: fingerprint,
	}, nil
}

// LedgerAdapter returns a ledger adapter
func (t *TestKit) LedgerAdapter() ports.LedgerPort {
	// Return shared ledger instance so UI and pipeline use same storage
	return t.ledger
}

// ProfilerAdapter returns a profiler adapter
func (t *TestKit) ProfilerAdapter() ports.ProfilerPort {
	// Create coercer with default config
	coercerInstance := coercer.NewTypeCoercer(coercer.DefaultCoercionConfig())
	// Return a real profiler adapter with coercer
	return datareadiness.NewProfilerAdapter(coercerInstance)
}

// ReadinessOrchestrator creates a readiness orchestrator with all dependencies
func (t *TestKit) ReadinessOrchestrator() (*resolution.DataReadinessOrchestrator, error) {
	// Create dependencies
	config := resolution.DefaultOrchestratorConfig()

	deps := resolution.ReadinessOrchestratorDeps{
		Profiler:    t.ProfilerAdapter(),
		Coercer:     coercer.NewTypeCoercer(config.CoercionConfig),
		Synthesizer: synthesizer.NewContractSynthesizer(config.SynthesisConfig),
		Gate:        resolution.NewReadinessGate(config.GateConfig),
	}

	return resolution.NewDataReadinessOrchestrator(deps)
}

// StageRunner returns a stage runner instance
func (t *TestKit) StageRunner() *app.StageRunner {
	return app.NewStageRunner(t.LedgerAdapter(), t.RNGAdapter())
}

// TestContract represents a test contract
type TestContract struct {
	AsOfMode        dataset.AsOfMode
	StatisticalType dataset.StatisticalType
	WindowDays      *int
}

// RNGAdapter implements the RNGPort interface for testing
type RNGAdapter struct{}

// SeededStream creates a deterministic random number generator for a named operation
func (r *RNGAdapter) SeededStream(ctx context.Context, name string, seed int64) (*rand.Rand, error) {
	return rand.New(rand.NewSource(seed)), nil
}

// Stream creates a deterministic RNG stream for a specific stage/relationship
func (r *RNGAdapter) Stream(ctx context.Context, runID, stageName, relationshipKey string, baseSeed int64) (*rand.Rand, error) {
	// Create deterministic seed by hashing runID + stageName + relationshipKey + baseSeed
	// This ensures identical results for the same run/stage/relationship combination
	seed := baseSeed
	if runID != "" {
		seed = int64(hashString(runID)) + seed
	}
	if stageName != "" {
		seed = int64(hashString(stageName)) + seed
	}
	if relationshipKey != "" {
		seed = int64(hashString(relationshipKey)) + seed
	}
	return rand.New(rand.NewSource(seed)), nil
}

// ValidateSeed ensures the seed produces expected deterministic results
func (r *RNGAdapter) ValidateSeed(ctx context.Context, name string, seed int64, expected []float64) error {
	// Stub implementation - always returns nil
	return nil
}

// hashString creates a simple hash for deterministic seeding
func hashString(s string) uint32 {
	var hash uint32 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + uint32(c) // djb2 algorithm
	}
	return hash
}

// TestRun represents a test run
type TestRun struct {
	Fingerprint core.Hash
	Artifacts   []core.Artifact
	// Add other fields as needed
}

// Stub implementations for testing

// FakeMatrixResolverAdapter implements MatrixResolverPort with realistic fake data
type FakeMatrixResolverAdapter struct {
	rng *rand.Rand
}

func NewFakeMatrixResolverAdapter() *FakeMatrixResolverAdapter {
	return &FakeMatrixResolverAdapter{
		rng: rand.New(rand.NewSource(42)), // Fixed seed for reproducibility
	}
}

func (s *FakeMatrixResolverAdapter) ResolveMatrix(ctx context.Context, req ports.MatrixResolutionRequest) (*dataset.MatrixBundle, error) {
	variableKeys := make([]core.VariableKey, len(req.VarKeys))
	for i, key := range req.VarKeys {
		variableKeys[i] = core.VariableKey(key)
	}

	// Use entity count from request or default to 1000
	entityCount := len(req.EntityIDs)
	if entityCount == 0 {
		entityCount = 1000
		req.EntityIDs = make([]core.ID, entityCount)
		for i := range req.EntityIDs {
			req.EntityIDs[i] = core.ID(fmt.Sprintf("entity_%d", i+1))
		}
	}

	data := make([][]float64, entityCount)
	columnMeta := make([]dataset.ColumnMeta, len(variableKeys))
	audits := make([]dataset.ResolutionAudit, len(variableKeys))

	// Generate realistic correlated data
	for i := range data {
		data[i] = make([]float64, len(variableKeys))

		for j, varKey := range variableKeys {
			keyStr := string(varKey)

			// Generate data based on variable name patterns
			switch {
			case keyStr == "inspection_count":
				// Inspection count: positive integers with some variance
				data[i][j] = float64(5 + s.rng.Intn(20) + i%10)

			case keyStr == "severity_score":
				// Severity score: correlated with inspection_count (r ~ 0.7)
				base := data[i][0] * 0.5 // Correlate with first column
				noise := s.rng.Float64() * 3.0
				data[i][j] = base + noise
				if data[i][j] < 0 {
					data[i][j] = 0
				}

			case keyStr == "region" || keyStr == "region_northwest" || keyStr == "region_southeast":
				// Categorical: regions encoded as 0/1
				data[i][j] = float64(s.rng.Intn(2))

			case keyStr == "has_violation":
				// Binary: violation flag, correlated with severity
				if len(data[i]) > 1 && data[i][1] > 5.0 {
					data[i][j] = 1.0
				} else {
					data[i][j] = float64(s.rng.Intn(2))
				}

			case keyStr == "facility_size":
				// Facility size: log-normal distribution
				data[i][j] = math.Exp(s.rng.NormFloat64()*0.5 + 3.0)

			case keyStr == "age_years" || keyStr == "age":
				// Age: normal distribution
				data[i][j] = 30 + s.rng.NormFloat64()*10
				if data[i][j] < 0 {
					data[i][j] = 0
				}

			default:
				// Default: random normal distribution
				data[i][j] = s.rng.NormFloat64() * 10.0
			}
		}
	}

	// Determine statistical types and create metadata
	for i, varKey := range variableKeys {
		keyStr := string(varKey)
		var statType dataset.StatisticalType

		switch {
		case keyStr == "has_violation" || keyStr == "region" || keyStr == "region_northwest" || keyStr == "region_southeast":
			statType = dataset.TypeBinary
		case keyStr == "region" && i < len(variableKeys):
			statType = dataset.TypeCategorical
		default:
			statType = dataset.TypeNumeric
		}

		columnMeta[i] = dataset.ColumnMeta{
			VariableKey:     varKey,
			StatisticalType: statType,
			ResolutionAudit: dataset.ResolutionAudit{
				VariableKey:       varKey,
				RowCount:          entityCount,
				ScalarGuarantee:   true,
				MaxTimestamp:      core.Now(),
				ImputationApplied: "none",
				AsOfMode:          dataset.AsOfLatestValue,
			},
		}
		audits[i] = columnMeta[i].ResolutionAudit
	}

	return &dataset.MatrixBundle{
		Matrix: dataset.Matrix{
			Data:         data,
			EntityIDs:    req.EntityIDs,
			VariableKeys: variableKeys,
		},
		ColumnMeta:  columnMeta,
		Audits:      audits,
		SnapshotID:  req.SnapshotID,
		ViewID:      req.ViewID,
		CohortHash:  core.CohortHash("test-cohort-hash"),
		CutoffAt:    core.CutoffAt(core.Now()),
		Lag:         core.Lag(0),
		CreatedAt:   core.Now(),
		Fingerprint: core.Hash("test-fingerprint"),
	}, nil
}

// FakeRegistryAdapter implements RegistryPort with realistic contracts
type FakeRegistryAdapter struct{}

func (s *FakeRegistryAdapter) GetContract(ctx context.Context, varKey string) (*dataset.VariableContract, error) {
	key := core.VariableKey(varKey)
	keyStr := string(key)

	var contract dataset.VariableContract
	contract.VarKey = key
	contract.ScalarGuarantee = true

	// Determine contract based on variable name patterns
	switch {
	case keyStr == "inspection_count" || keyStr == "count" || keyStr == "frequency":
		contract.AsOfMode = dataset.AsOfCountWindow
		contract.StatisticalType = dataset.TypeNumeric
		window := 30
		contract.WindowDays = &window
		contract.ImputationPolicy = "zero_fill"

	case keyStr == "severity_score" || keyStr == "score" || keyStr == "rating":
		contract.AsOfMode = dataset.AsOfLatestValue
		contract.StatisticalType = dataset.TypeNumeric
		contract.WindowDays = nil
		contract.ImputationPolicy = "mean_fill"

	case keyStr == "region" || keyStr == "category" || keyStr == "type":
		contract.AsOfMode = dataset.AsOfLatestValue
		contract.StatisticalType = dataset.TypeCategorical
		contract.WindowDays = nil
		contract.ImputationPolicy = "mode_fill"

	case keyStr == "has_violation" || keyStr == "is_active" || keyStr == "flag":
		contract.AsOfMode = dataset.AsOfExists
		contract.StatisticalType = dataset.TypeBinary
		contract.WindowDays = nil
		contract.ImputationPolicy = "false_fill"

	case keyStr == "facility_size" || keyStr == "revenue" || keyStr == "amount":
		contract.AsOfMode = dataset.AsOfLatestValue
		contract.StatisticalType = dataset.TypeNumeric
		contract.WindowDays = nil
		contract.ImputationPolicy = "median_fill"

	case keyStr == "age_years" || keyStr == "age":
		contract.AsOfMode = dataset.AsOfLatestValue
		contract.StatisticalType = dataset.TypeNumeric
		contract.WindowDays = nil
		contract.ImputationPolicy = "mean_fill"

	default:
		// Default contract
		contract.AsOfMode = dataset.AsOfLatestValue
		contract.StatisticalType = dataset.TypeNumeric
		contract.WindowDays = nil
		contract.ImputationPolicy = "zero_fill"
	}

	return &contract, nil
}

// InMemoryLedgerAdapter implements LedgerPort with in-memory storage
type InMemoryLedgerAdapter struct {
	artifacts    map[core.ArtifactID]core.Artifact
	runArtifacts map[core.RunID][]core.ArtifactID
	mu           sync.RWMutex
}

func NewInMemoryLedgerAdapter() *InMemoryLedgerAdapter {
	adapter := &InMemoryLedgerAdapter{
		artifacts:    make(map[core.ArtifactID]core.Artifact),
		runArtifacts: make(map[core.RunID][]core.ArtifactID),
	}
	return adapter
}

func (s *InMemoryLedgerAdapter) StoreArtifact(ctx context.Context, runID string, artifact core.Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	artifactID := core.ArtifactID(artifact.ID)
	s.artifacts[artifactID] = artifact

	// Track artifacts by run
	runIDTyped := core.RunID(runID)
	if s.runArtifacts[runIDTyped] == nil {
		s.runArtifacts[runIDTyped] = []core.ArtifactID{}
	}
	s.runArtifacts[runIDTyped] = append(s.runArtifacts[runIDTyped], artifactID)

	return nil
}

func (s *InMemoryLedgerAdapter) ListArtifacts(ctx context.Context, filters ports.ArtifactFilters) ([]core.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []core.Artifact
	count := 0

	for _, artifact := range s.artifacts {
		// Apply filters
		if filters.Kind != nil && artifact.Kind != *filters.Kind {
			continue
		}

		if filters.RunID != nil {
			runArtifacts, exists := s.runArtifacts[*filters.RunID]
			if !exists {
				continue
			}
			found := false
			for _, aid := range runArtifacts {
				if aid == core.ArtifactID(artifact.ID) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, artifact)
		count++
		if filters.Limit > 0 && count >= filters.Limit {
			break
		}
	}

	return results, nil
}

func (s *InMemoryLedgerAdapter) GetArtifact(ctx context.Context, artifactID core.ArtifactID) (*core.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	artifact, exists := s.artifacts[artifactID]
	if !exists {
		return nil, fmt.Errorf("artifact not found: %s", artifactID)
	}

	return &artifact, nil
}

func (s *InMemoryLedgerAdapter) GetArtifactsByRun(ctx context.Context, runID core.RunID) ([]core.Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	artifactIDs, exists := s.runArtifacts[runID]
	if !exists {
		return []core.Artifact{}, nil
	}

	artifacts := make([]core.Artifact, 0, len(artifactIDs))
	for _, aid := range artifactIDs {
		if artifact, ok := s.artifacts[aid]; ok {
			artifacts = append(artifacts, artifact)
		}
	}

	return artifacts, nil
}

func (s *InMemoryLedgerAdapter) GetArtifactsByKind(ctx context.Context, kind core.ArtifactKind, limit int) ([]core.Artifact, error) {
	return s.ListArtifacts(ctx, ports.ArtifactFilters{Kind: &kind, Limit: limit})
}

func (s *InMemoryLedgerAdapter) GetRunManifest(ctx context.Context, runID core.RunID) (*run.RunManifestArtifact, error) {
	// Return a fake run manifest
	snapshotID := core.SnapshotID("test-snapshot")
	registryHash := core.RegistryHash("test-registry-hash")
	cohortHash := core.CohortHash("test-cohort-hash")
	stagePlanHash := core.Hash("test-stage-plan-hash")
	seed := int64(42)
	codeVersion := "1.0.0-test"

	fingerprint := run.NewRunFingerprint(snapshotID, registryHash, cohortHash, stagePlanHash, seed, codeVersion)

	return &run.RunManifestArtifact{
		RunID:         runID,
		SnapshotID:    snapshotID,
		SnapshotAt:    core.SnapshotAt(core.Now()),
		Lag:           core.Lag(24 * 60 * 60 * 1000000000), // 24 hours
		CutoffAt:      core.CutoffAt(core.Now()),
		RegistryHash:  registryHash,
		CohortHash:    cohortHash,
		StagePlanHash: core.StageListHash(stagePlanHash),
		Seed:          seed,
		CodeVersion:   codeVersion,
		Fingerprint:   fingerprint,
		CreatedAt:     core.Now(),
	}, nil
}
