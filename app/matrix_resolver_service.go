package app

import (
	"context"
	"fmt"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/ports"
)

// MatrixResolverService handles deterministic matrix resolution with full audit trails
type MatrixResolverService struct {
	matrixResolverPort ports.MatrixResolverPort
	registryPort       ports.RegistryPort
	rngPort            ports.RNGPort
}

// AuditableResolutionRequest defines the inputs for deterministic matrix resolution
type AuditableResolutionRequest struct {
	DatasetViewID  core.ID
	CohortSelector map[string]interface{} // filters for entity selection
	SnapshotAt     core.SnapshotAt
	Lag            core.Lag
	VariableKeys   []core.VariableKey
	Seed           int64 // for deterministic randomness if needed
}

// NewMatrixResolverService creates a matrix resolver service
func NewMatrixResolverService(matrixResolverPort ports.MatrixResolverPort, registryPort ports.RegistryPort, rngPort ports.RNGPort) *MatrixResolverService {
	return &MatrixResolverService{
		matrixResolverPort: matrixResolverPort,
		registryPort:       registryPort,
		rngPort:            rngPort,
	}
}

// ResolveMatrixAuditable produces a deterministic MatrixBundle with complete audit trail
func (s *MatrixResolverService) ResolveMatrixAuditable(ctx context.Context, req AuditableResolutionRequest) (*dataset.AuditableResolutionResult, error) {
	// Step 1: Validate all variable contracts exist and are admissible
	contracts, err := s.validateVariableContracts(ctx, req.VariableKeys)
	if err != nil {
		return nil, fmt.Errorf("contract validation failed: %w", err)
	}

	// Step 2: Create snapshot specification
	snapshotID := core.NewID()

	// Step 3: Build cohort (entity selection)
	cohortEntities, cohortHash, err := s.buildCohort(ctx, req.CohortSelector, req.Seed)
	if err != nil {
		return nil, fmt.Errorf("cohort building failed: %w", err)
	}

	// Step 4: Create snapshot manifest
	manifest := dataset.NewSnapshotManifest(
		core.SnapshotID(snapshotID),
		req.SnapshotAt,
		req.Lag,
		cohortEntities,
		req.DatasetViewID,
		cohortHash,
	)

	// Step 5: Resolve matrix with audit trail
	matrixReq := ports.MatrixResolutionRequest{
		ViewID:     req.DatasetViewID,
		SnapshotID: core.SnapshotID(snapshotID),
		EntityIDs:  cohortEntities,
		VarKeys:    req.VariableKeys,
	}

	matrixBundle, err := s.matrixResolverPort.ResolveMatrix(ctx, matrixReq)
	if err != nil {
		return nil, fmt.Errorf("matrix resolution failed: %w", err)
	}

	// Step 6: Generate resolution audits for each variable
	audits, err := s.generateResolutionAudits(matrixBundle, contracts, manifest.CutoffAt)
	if err != nil {
		return nil, fmt.Errorf("audit generation failed: %w", err)
	}

	// Step 7: Compute registry hash and create fingerprint
	registryHash, err := s.computeRegistryHash(ctx, contracts)
	if err != nil {
		return nil, fmt.Errorf("registry hash computation failed: %w", err)
	}

	resolverVersion := "1.0.0" // TODO: make this configurable
	fingerprint := manifest.ComputeFingerprint(registryHash, resolverVersion, req.Seed)

	// Step 8: Create auditable result
	result := &dataset.AuditableResolutionResult{
		Manifest:     manifest,
		MatrixBundle: matrixBundle,
		Audits:       audits,
		Fingerprint:  fingerprint,
	}

	// Step 9: Validate result meets all requirements
	if err := result.ValidateResult(); err != nil {
		return nil, fmt.Errorf("result validation failed: %w", err)
	}

	return result, nil
}

// validateVariableContracts ensures all variables are admissible
func (s *MatrixResolverService) validateVariableContracts(ctx context.Context, varKeys []core.VariableKey) (map[string]*dataset.VariableContract, error) {
	contracts := make(map[string]*dataset.VariableContract)

	for _, varKey := range varKeys {
		contract, err := s.registryPort.GetContract(ctx, string(varKey))
		if err != nil {
			return nil, core.NewResolutionError(string(varKey),
				fmt.Errorf("contract not found: %w", err))
		}

		// Check scalar guarantee
		if !contract.ScalarGuarantee {
			return nil, core.NewResolutionError(string(varKey),
				fmt.Errorf("scalar guarantee not satisfied"))
		}

		contracts[string(varKey)] = &dataset.VariableContract{
			VarKey:           contract.VarKey,
			AsOfMode:         dataset.AsOfMode(contract.AsOfMode),
			StatisticalType:  dataset.StatisticalType(contract.StatisticalType),
			WindowDays:       contract.WindowDays,
			ImputationPolicy: contract.ImputationPolicy,
			ScalarGuarantee:  contract.ScalarGuarantee,
		}
	}

	return contracts, nil
}

// buildCohort creates the entity cohort based on selectors
func (s *MatrixResolverService) buildCohort(ctx context.Context, selector map[string]interface{}, seed int64) ([]core.ID, core.CohortHash, error) {
	// TODO: Implement proper cohort selection based on filters
	// For now, return a deterministic set based on seed

	rng, err := s.rngPort.SeededStream(ctx, "cohort_selection", seed)
	if err != nil {
		return nil, "", err
	}

	// Generate deterministic entity IDs
	entityCount := 100 // TODO: make configurable
	entityIDs := make([]core.ID, entityCount)
	for i := 0; i < entityCount; i++ {
		entityIDs[i] = core.ID(fmt.Sprintf("entity_%d", rng.Intn(1000)+1))
	}

	// Convert []core.ID to []string for ComputeCohortHash
	entityIDStrings := make([]string, len(entityIDs))
	for i, id := range entityIDs {
		entityIDStrings[i] = string(id)
	}

	cohortHash := core.ComputeCohortHash(entityIDStrings, selector)
	return entityIDs, cohortHash, nil
}

// generateResolutionAudits creates audit trail for each variable resolution
func (s *MatrixResolverService) generateResolutionAudits(bundle *dataset.MatrixBundle, contracts map[string]*dataset.VariableContract, cutoffAt core.CutoffAt) ([]dataset.ResolverAudit, error) {
	audits := make([]dataset.ResolverAudit, len(bundle.Matrix.VariableKeys))

	for i, varKey := range bundle.Matrix.VariableKeys {
		contract := contracts[string(varKey)]
		meta := bundle.ColumnMeta[i]

		// Check for resolution errors
		var errors []string
		if len(meta.ResolutionAudit.ResolutionErrors) > 0 {
			// Use the first resolution error as the failure reason
			errors = append(errors, meta.ResolutionAudit.ResolutionErrors[0])
		}

		audit := dataset.ResolverAudit{
			VariableKey:       varKey,
			MaxTimestampUsed:  meta.ResolutionAudit.MaxTimestamp,
			RowCount:          bundle.RowCount(),
			ImputationApplied: s.determineImputation(meta),
			ScalarGuarantee:   len(meta.ResolutionAudit.ResolutionErrors) == 0 && len(errors) == 0,
			AsOfMode:          contract.AsOfMode,
			WindowDays:        contract.WindowDays,
			ResolutionErrors:  errors,
		}

		audits[i] = audit
	}

	return audits, nil
}

// determineImputation analyzes the matrix column to determine what imputation was applied
func (s *MatrixResolverService) determineImputation(meta dataset.ColumnMeta) string {
	// Analyze the resolution audit and column data to determine imputation strategy
	// This is a simplified implementation
	if len(meta.ResolutionAudit.ResolutionErrors) == 0 {
		return "none"
	}
	return "contract_default" // TODO: implement proper imputation detection
}

// computeRegistryHash creates hash of all variable contracts
func (s *MatrixResolverService) computeRegistryHash(ctx context.Context, contracts map[string]*dataset.VariableContract) (core.RegistryHash, error) {
	// Convert contracts to the format expected by core.ComputeRegistryHash
	contractMap := make(map[string]interface{})
	for key, contract := range contracts {
		contractMap[key] = map[string]interface{}{
			"as_of_mode":        contract.AsOfMode,
			"statistical_type":  contract.StatisticalType,
			"window_days":       contract.WindowDays,
			"imputation_policy": contract.ImputationPolicy,
			"scalar_guarantee":  contract.ScalarGuarantee,
		}
	}

	return core.ComputeRegistryHash(contractMap), nil
}
