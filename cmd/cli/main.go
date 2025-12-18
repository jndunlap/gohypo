package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gohypo/adapters/llm"
	"gohypo/adapters/llm/heuristic"
	"gohypo/app"
	"gohypo/domain/core"
	"gohypo/domain/stage"
	"gohypo/domain/stats"
	"gohypo/internal/testkit"
	"gohypo/ports"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gohypo-cli",
		Short: "GoHypo CLI for testing matrix resolution and stats sweeps",
	}

	rootCmd.AddCommand(
		newResolveCmd(),
		newSweepCmd(),
		newHypothesesCmd(),
		newReadinessCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newResolveCmd() *cobra.Command {
	var seed int64
	var datasetView string
	var lagHours int

	cmd := &cobra.Command{
		Use:   "resolve [snapshot-at] [var-keys...]",
		Short: "Resolve variables to matrix with audit trail",
		Long: `Resolve variables for a snapshot with complete audit trail.

Example: gohypo-cli resolve "2024-01-01T12:00:00Z" inspection_count severity_score --seed 12345 --lag-hours 24`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshotAtStr := args[0]
			varKeys := args[1:]

			snapshotAt, err := time.Parse(time.RFC3339, snapshotAtStr)
			if err != nil {
				return fmt.Errorf("invalid snapshot-at format (use RFC3339): %w", err)
			}

			// Convert lag hours to duration
			lag := time.Duration(lagHours) * time.Hour

			return runResolve(cmd.Context(), datasetView, core.NewSnapshotAt(snapshotAt), core.NewLag(lag), varKeys, seed)
		},
	}

	cmd.Flags().Int64Var(&seed, "seed", 42, "Random seed for deterministic operations")
	cmd.Flags().StringVar(&datasetView, "dataset-view", "test_view", "Dataset view identifier")
	cmd.Flags().IntVar(&lagHours, "lag-hours", 24, "Lag buffer in hours")

	return cmd
}

func newSweepCmd() *cobra.Command {
	var seed int64

	cmd := &cobra.Command{
		Use:   "sweep [matrix-bundle-id]",
		Short: "Run Layer 0 stats sweep on resolved matrix",
		Long: `Run statistical relationship discovery (Layer 0) on a resolved matrix bundle.

This produces relationship artifacts with complete statistical validation including
permutation tests, stability analysis, and phantom benchmarks.

Example: gohypo-cli sweep bundle-123 --seed 12345`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			matrixBundleID := args[0]
			return runSweep(cmd.Context(), matrixBundleID, seed)
		},
	}

	cmd.Flags().Int64Var(&seed, "seed", 42, "Random seed for deterministic operations")

	return cmd
}

func newHypothesesCmd() *cobra.Command {
	var seed int64
	var maxHypotheses int
	var rigor string

	cmd := &cobra.Command{
		Use:   "hypotheses [matrix-bundle-id]",
		Short: "Run Layer 1 hypothesis generation from Layer 0 relationships",
		Long: `Generate hypothesis candidates from Layer 0 relationship artifacts.

Generator selection is controlled by:
- GENERATOR_MODE=llm|heuristic (default: heuristic)

If GENERATOR_MODE=llm, OpenAI configuration is read from:
- LLM_API_KEY
- LLM_MODEL (default: gpt-4o-mini)
- LLM_BASE_URL (optional; default: https://api.openai.com/v1)
- LLM_TEMPERATURE (optional; default: 0.2)
- LLM_MAX_TOKENS (optional; default: 2000)

Example:
  GENERATOR_MODE=llm LLM_API_KEY=... gohypo-cli hypotheses bundle-123 --seed 42 --max-hypotheses 10 --rigor standard`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			matrixBundleID := args[0]
			return runHypotheses(cmd.Context(), matrixBundleID, seed, maxHypotheses, rigor)
		},
	}

	cmd.Flags().Int64Var(&seed, "seed", 42, "Random seed for deterministic operations")
	cmd.Flags().IntVar(&maxHypotheses, "max-hypotheses", 10, "Maximum hypotheses to generate")
	cmd.Flags().StringVar(&rigor, "rigor", "standard", "Rigor profile: basic|standard|decision")
	return cmd
}

func newReadinessCmd() *cobra.Command {
	var detailedOutput bool

	cmd := &cobra.Command{
		Use:   "readiness [source-name] [data-file]",
		Short: "Process a data source through the readiness pipeline",
		Long: `Process an arbitrary data source through the complete data readiness pipeline.

This demonstrates the deterministic transformation from unknown data to
statistically ready variables with complete audit trails.

Example: gohypo-cli readiness my_source data.json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceName := args[0]
			dataFile := args[1]

			return runReadiness(cmd.Context(), sourceName, dataFile, detailedOutput)
		},
	}

	cmd.Flags().BoolVar(&detailedOutput, "detailed", false, "Save detailed results to JSON file")

	return cmd
}

func runReadiness(ctx context.Context, sourceName, dataFile string, detailedOutput bool) error {
	fmt.Printf("🔬 Processing source '%s' through data readiness pipeline...\n", sourceName)

	// Load sample data (in production, this would be from various sources)
	rawData, err := loadSampleData(dataFile)
	if err != nil {
		return fmt.Errorf("failed to load data: %w", err)
	}

	// Initialize test kit and create orchestrator
	kit, err := testkit.NewTestKit()
	if err != nil {
		return fmt.Errorf("failed to initialize test kit: %w", err)
	}

	orchestrator, err := kit.ReadinessOrchestrator()
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Process the source
	startTime := time.Now()
	readinessResult, err := orchestrator.ProcessSource(ctx, sourceName, rawData)
	if err != nil {
		return fmt.Errorf("readiness processing failed: %w", err)
	}

	processingTime := time.Since(startTime)

	// Display results
	fmt.Printf("\n📊 READINESS PIPELINE RESULTS\n")
	fmt.Printf("Source: %s\n", sourceName)
	fmt.Printf("Processing Time: %v\n", processingTime)
	fmt.Printf("Total Variables: %d\n", readinessResult.TotalVariables)
	fmt.Printf("Ready Variables: %d\n", readinessResult.ReadyCount)
	fmt.Printf("Rejected Variables: %d\n", readinessResult.RejectedCount)

	// Show ready variables
	if readinessResult.ReadyCount > 0 {
		fmt.Printf("\n✅ READY VARIABLES:\n")
		for i, variable := range readinessResult.ReadyVariables {
			fmt.Printf("%d. %s (%s)\n", i+1, variable.VariableKey, variable.Profile.InferredType)
			fmt.Printf("   Quality: %.2f, Missing: %.1f%%\n",
				variable.Profile.QualityScore, variable.Profile.MissingStats.MissingRate*100)

			// Show rejections if any
			for _, rejection := range variable.Rejections {
				if rejection.Severity == "warning" {
					fmt.Printf("   ⚠️  %s: %s\n", rejection.Rule, rejection.Message)
				}
			}
		}
	}

	// Show rejected variables with reasons
	if readinessResult.RejectedCount > 0 {
		fmt.Printf("\n❌ REJECTED VARIABLES:\n")
		rejectionReasons := readinessResult.GetRejectedReasons()

		for reason, count := range rejectionReasons {
			fmt.Printf("• %s: %d variables\n", reason, count)
		}

		fmt.Printf("\nDetailed rejections:\n")
		for i, variable := range readinessResult.RejectedVariables[:min(5, len(readinessResult.RejectedVariables))] {
			fmt.Printf("%d. %s\n", i+1, variable.VariableKey)
			for _, rejection := range variable.Rejections {
				fmt.Printf("   🚫 %s (%s): %s\n", rejection.Rule, rejection.Severity, rejection.Message)
			}
		}

		if len(readinessResult.RejectedVariables) > 5 {
			fmt.Printf("   ... and %d more\n", len(readinessResult.RejectedVariables)-5)
		}
	}

	// Show admissible variables for statistical analysis
	admissible := orchestrator.GetAdmissibleVariables(readinessResult)
	if len(admissible) > 0 {
		fmt.Printf("\n🎯 ADMISSIBLE VARIABLES FOR STATISTICS:\n")
		for _, variable := range admissible {
			fmt.Printf("• %s: %s (quality: %.2f)\n",
				variable.Key, variable.Description, variable.QualityScore)
		}
	}

	// Show summary statistics
	fmt.Printf("\n📈 SUMMARY STATISTICS:\n")
	totalMissing := 0.0
	totalQuality := 0.0
	for _, variable := range readinessResult.ReadyVariables {
		totalMissing += variable.Profile.MissingStats.MissingRate
		totalQuality += variable.Profile.QualityScore
	}

	if readinessResult.ReadyCount > 0 {
		avgMissing := totalMissing / float64(readinessResult.ReadyCount)
		avgQuality := totalQuality / float64(readinessResult.ReadyCount)
		fmt.Printf("Average Missing Rate: %.1f%%\n", avgMissing*100)
		fmt.Printf("Average Quality Score: %.2f\n", avgQuality)
	}

	fmt.Printf("Readiness Rate: %.1f%%\n",
		float64(readinessResult.ReadyCount)/float64(readinessResult.TotalVariables)*100)

	// Optional: Save detailed results to JSON
	if detailedOutput {
		result := map[string]interface{}{
			"source":           sourceName,
			"processing_time":  processingTime.String(),
			"readiness_result": readinessResult,
		}

		jsonData, _ := json.MarshalIndent(result, "", "  ")
		outputFile := fmt.Sprintf("%s_readiness.json", sourceName)
		if err := os.WriteFile(outputFile, jsonData, 0644); err == nil {
			fmt.Printf("\n💾 Detailed results saved to: %s\n", outputFile)
		}
	}

	fmt.Printf("\n✅ DATA READINESS PIPELINE COMPLETED\n")
	fmt.Printf("Source '%s' is now ready for statistical analysis!\n", sourceName)

	return nil
}

// loadSampleData loads sample data for demonstration (in production, this would parse actual files)
func loadSampleData(filename string) (interface{}, error) {
	// Mock data for demonstration - in production, this would parse actual files
	sampleData := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"entity_id":        "user_123",
				"observed_at":      "2024-01-15T10:30:00Z",
				"inspection_count": 5,
				"severity_score":   7.2,
				"has_violation":    true,
				"region":           "northwest",
			},
			{
				"entity_id":        "user_456",
				"observed_at":      "2024-01-15T11:15:00Z",
				"inspection_count": 0,
				"severity_score":   2.1,
				"has_violation":    false,
				"region":           "southeast",
			},
			{
				"entity_id":        "user_789",
				"observed_at":      "2024-01-15T12:00:00Z",
				"inspection_count": 12,
				"severity_score":   8.9,
				"has_violation":    true,
				"region":           "northwest",
			},
		},
	}

	return sampleData, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runResolve(ctx context.Context, datasetView string, snapshotAt core.SnapshotAt, lag core.Lag, varKeyStrings []string, seed int64) error {
	fmt.Printf("Resolving variables for snapshot at %s with %s lag...\n", snapshotAt, lag)

	// Initialize test kit (in production, this would use real adapters)
	kit, err := testkit.NewTestKit()
	if err != nil {
		return fmt.Errorf("failed to initialize test kit: %w", err)
	}

	// Parse variable keys
	varKeys := make([]core.VariableKey, len(varKeyStrings))
	for i, keyStr := range varKeyStrings {
		varKeys[i], _ = core.ParseVariableKey(keyStr)
	}

	// Create matrix resolver service
	matrixSvc := app.NewMatrixResolverService(
		kit.MatrixResolverAdapter(),
		kit.RegistryAdapter(),
		kit.RNGAdapter(),
	)

	// Prepare resolution request
	req := app.AuditableResolutionRequest{
		DatasetViewID:  core.ID(datasetView),
		CohortSelector: map[string]interface{}{"active": true}, // Example selector
		SnapshotAt:     snapshotAt,
		Lag:            lag,
		VariableKeys:   varKeys,
		Seed:           seed,
	}

	// Execute resolution
	result, err := matrixSvc.ResolveMatrixAuditable(ctx, req)
	if err != nil {
		return fmt.Errorf("matrix resolution failed: %w", err)
	}

	// Display results
	fmt.Printf("\n=== RESOLUTION RESULTS ===\n")
	fmt.Printf("Snapshot ID: %s\n", result.Manifest.SnapshotID)
	fmt.Printf("Snapshot At: %s\n", result.Manifest.SnapshotAt)
	fmt.Printf("Cutoff At: %s\n", result.Manifest.CutoffAt)
	fmt.Printf("Lag Applied: %s\n", result.Manifest.Lag)
	fmt.Printf("Cohort Size: %d\n", result.Manifest.CohortSize)
	fmt.Printf("Matrix Dimensions: %dx%d\n", result.MatrixBundle.RowCount(), result.MatrixBundle.ColumnCount())

	fmt.Printf("\n=== VARIABLE AUDITS ===\n")
	for i, audit := range result.Audits {
		fmt.Printf("%d. %s:\n", i+1, audit.VariableKey)
		fmt.Printf("   Max Timestamp: %s\n", audit.MaxTimestampUsed.Time().Format(time.RFC3339))
		fmt.Printf("   Row Count: %d\n", audit.RowCount)
		fmt.Printf("   Imputation: %s\n", audit.ImputationApplied)
		fmt.Printf("   Scalar Guarantee: %t\n", audit.ScalarGuarantee)
		fmt.Printf("   As-Of Mode: %s\n", audit.AsOfMode)
		if audit.WindowDays != nil {
			fmt.Printf("   Window Days: %d\n", *audit.WindowDays)
		}
		if len(audit.ResolutionErrors) > 0 {
			fmt.Printf("   Errors: %v\n", audit.ResolutionErrors)
		}
		fmt.Println()
	}

	fmt.Printf("\n=== FINGERPRINT ===\n")
	fmt.Printf("Manifest Hash: %s\n", result.Fingerprint.ManifestHash)
	fmt.Printf("Registry Hash: %s\n", result.Fingerprint.RegistryHash)
	fmt.Printf("Resolver Version: %s\n", result.Fingerprint.ResolverVersion)
	fmt.Printf("Seed: %d\n", result.Fingerprint.Seed)
	fmt.Printf("Complete Fingerprint: %s\n", result.Fingerprint.Fingerprint)

	// Validate result
	if err := result.ValidateResult(); err != nil {
		fmt.Printf("\n❌ VALIDATION FAILED: %v\n", err)
		return err
	}

	fmt.Printf("\n✅ RESOLUTION COMPLETED SUCCESSFULLY\n")
	fmt.Printf("This result is completely deterministic and replayable using the fingerprint.\n")

	return nil
}

func runSweep(ctx context.Context, matrixBundleID string, seed int64) error {
	fmt.Printf("Running Layer 0 stats sweep on matrix bundle %s...\n", matrixBundleID)

	// Initialize test kit (in production, this would use real adapters)
	kit, err := testkit.NewTestKit()
	if err != nil {
		return fmt.Errorf("failed to initialize test kit: %w", err)
	}

	// For demo purposes, create a test matrix bundle
	// In production, this would load from storage
	matrixBundle, err := kit.CreateTestMatrixBundle(ctx, matrixBundleID)
	if err != nil {
		return fmt.Errorf("failed to create/load matrix bundle: %w", err)
	}

	// Create stats sweep service
	statsSvc := app.NewStatsSweepService(
		kit.StageRunner(),
		kit.LedgerAdapter(),
		kit.RNGAdapter(),
	)

	// Run the sweep
	req := app.AuditableSweepRequest{
		MatrixBundle: matrixBundle,
		RigorProfile: stage.RigorDecision, // Full statistical validation
		Seed:         seed,
	}

	result, err := statsSvc.RunAuditableSweep(ctx, req)
	if err != nil {
		return fmt.Errorf("stats sweep failed: %w", err)
	}

	// Display results
	fmt.Printf("\n=== LAYER 0 STATS SWEEP RESULTS ===\n")
	fmt.Printf("Sweep ID: %s\n", result.SweepID)
	fmt.Printf("Runtime: %d ms\n", result.RuntimeMs)
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("Relationships Found: %d\n", len(result.Relationships))
	fmt.Printf("Fingerprint: %s\n", result.Fingerprint)

	fmt.Printf("\n=== RELATIONSHIP ARTIFACTS ===\n")
	for i, rel := range result.Relationships {
		var (
			varX, varY, testTypeStr string
			effectSize, pValue      float64
			sampleSize, comparisons int
			qValue                  float64
			warnings                []stats.WarningCode
		)

		// Handle payload whether it's a struct or map
		switch payload := rel.Payload.(type) {
		case stats.RelationshipPayload:
			varX = string(payload.VariableX)
			varY = string(payload.VariableY)
			testTypeStr = string(payload.TestType)
			effectSize = payload.EffectSize
			pValue = payload.PValue
			sampleSize = payload.SampleSize
			comparisons = payload.TotalComparisons
			qValue = payload.QValue
			warnings = payload.Warnings
		case map[string]interface{}:
			// Fallback for map
			varX, _ = payload["variable_x"].(string)
			varY, _ = payload["variable_y"].(string)
			testTypeStr, _ = payload["test_used"].(string)
			effectSize, _ = payload["effect_size"].(float64)
			pValue, _ = payload["p_value"].(float64)
			sampleSizeVal, _ := payload["sample_size"].(float64)
			sampleSize = int(sampleSizeVal)
			comparisonsVal, _ := payload["total_comparisons"].(float64)
			comparisons = int(comparisonsVal)
			qValue, _ = payload["q_value"].(float64)
			// Warnings handling omitted for simplicity in map case
		default:
			fmt.Printf("%d. <invalid relationship format>\n", i+1)
			continue
		}

		fmt.Printf("%d. %s ↔ %s\n", i+1, varX, varY)

		// Handle FDR correction
		fdrStr := "N/A"
		if qValue > 0 {
			fdrStr = fmt.Sprintf("%.4f", qValue)
		}

		fmt.Printf("   Test: %s | Effect: %.3f | P: %.4f | Q: %s\n",
			testTypeStr, effectSize, pValue, fdrStr)
		fmt.Printf("   N: %d | Comparisons: %d\n",
			sampleSize, comparisons)

		// Show warnings if any
		if len(warnings) > 0 {
			fmt.Printf("   Warnings: %v\n", warnings)
		}
		fmt.Println()
	}

	// Display manifest
	fmt.Printf("\n=== SWEEP MANIFEST ===\n")
	manifestPayload := result.Manifest.Payload.(map[string]interface{})
	fmt.Printf("Tests Executed: %v\n", manifestPayload["tests_executed"])
	fmt.Printf("Total Comparisons: %v\n", manifestPayload["total_comparisons"])
	fmt.Printf("Successful Tests: %v\n", manifestPayload["successful_tests"])
	fmt.Printf("Rejection Counts: %v\n", manifestPayload["rejection_counts"])

	fmt.Printf("\n✅ LAYER 0 SWEEP COMPLETED SUCCESSFULLY\n")
	fmt.Printf("All relationships include permutation tests, stability analysis, and phantom benchmarks.\n")
	fmt.Printf("Results are completely deterministic and replayable using the fingerprint.\n")

	return nil
}

func runHypotheses(ctx context.Context, matrixBundleID string, seed int64, maxHypotheses int, rigor string) error {
	fmt.Printf("Running Layer 1 hypothesis generation on matrix bundle %s...\n", matrixBundleID)

	kit, err := testkit.NewTestKit()
	if err != nil {
		return fmt.Errorf("failed to initialize test kit: %w", err)
	}

	// Create a shared ledger + stage runner (so sweep + hypotheses see same artifacts)
	ledger := testkit.NewInMemoryLedgerAdapter()
	rng := kit.RNGAdapter()
	stageRunner := app.NewStageRunner(ledger, rng)

	matrixBundle, err := kit.CreateTestMatrixBundle(ctx, matrixBundleID)
	if err != nil {
		return fmt.Errorf("failed to create/load matrix bundle: %w", err)
	}

	// Run sweep (Layer 0) to produce relationship artifacts
	statsSvc := app.NewStatsSweepService(stageRunner, ledger, rng)
	sweepReq := app.AuditableSweepRequest{
		MatrixBundle: matrixBundle,
		RigorProfile: stage.RigorDecision, // full Layer 0 evidence for best hypotheses
		Seed:         seed,
		SweepID:      core.ID(matrixBundleID),
	}
	sweepResult, err := statsSvc.RunAuditableSweep(ctx, sweepReq)
	if err != nil {
		return fmt.Errorf("stats sweep failed: %w", err)
	}

	var rigorProfile stage.RigorProfile
	switch rigor {
	case "basic":
		rigorProfile = stage.RigorBasic
	case "standard":
		rigorProfile = stage.RigorStandard
	case "decision":
		rigorProfile = stage.RigorDecision
	default:
		return fmt.Errorf("invalid rigor: %s (expected basic|standard|decision)", rigor)
	}

	// Select generator based on env
	var generator ports.GeneratorPort
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GENERATOR_MODE")))
	if mode == "llm" {
		model := strings.TrimSpace(os.Getenv("LLM_MODEL"))
		if model == "" {
			model = "gpt-4o-mini"
		}
		temp := 0.2
		if v := strings.TrimSpace(os.Getenv("LLM_TEMPERATURE")); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				temp = f
			}
		}
		maxTokens := 2000
		if v := strings.TrimSpace(os.Getenv("LLM_MAX_TOKENS")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				maxTokens = n
			}
		}

		fallback := heuristic.NewGenerator()
		llmGen, err := llm.NewGeneratorAdapter(llm.Config{
			Model:               model,
			APIKey:              os.Getenv("LLM_API_KEY"),
			BaseURL:             os.Getenv("LLM_BASE_URL"),
			Temperature:         temp,
			MaxTokens:           maxTokens,
			Timeout:             30 * time.Second,
			FallbackToHeuristic: true,
		}, fallback)
		if err != nil {
			fmt.Printf("LLM generator init failed (%v); falling back to heuristic\n", err)
			generator = fallback
		} else {
			generator = llmGen
		}
	} else {
		generator = heuristic.NewGenerator()
	}

	hypothesisSvc := app.NewHypothesisService(generator, nil, stageRunner, ledger, rng)

	runID := core.RunID(sweepResult.SweepID)
	genResult, err := hypothesisSvc.ProposeHypotheses(ctx, app.AuditableHypothesisRequest{
		RunID:            runID,
		MatrixBundleID:   core.ID(matrixBundleID),
		RelationshipArts: sweepResult.Relationships,
		MaxHypotheses:    maxHypotheses,
		RigorProfile:     rigorProfile,
		Seed:             seed,
	})
	if err != nil {
		return fmt.Errorf("hypothesis generation failed: %w", err)
	}

	fmt.Printf("\n=== HYPOTHESIS GENERATION RESULTS ===\n")
	fmt.Printf("Run ID: %s\n", genResult.RunID)
	fmt.Printf("Hypotheses stored: %d\n", len(genResult.Hypotheses))
	fmt.Printf("Audit artifact ID: %s\n", genResult.Manifest.ID)
	fmt.Printf("Fingerprint: %s\n", genResult.Fingerprint)
	fmt.Printf("Runtime: %d ms\n", genResult.RuntimeMs)

	return nil
}
