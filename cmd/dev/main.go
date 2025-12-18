package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"gohypo/adapters/db/postgres/migrations"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/internal/testkit"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gohypo-dev",
		Short: "GoHypo development tools",
	}

	rootCmd.AddCommand(
		newSeedCmd(),
		newSmokeTestCmd(),
		newDeterminismTestCmd(),
		newMigrateCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newSeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Generate seed data for development",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateSeedData(cmd.Context())
		},
	}
	return cmd
}

func newSmokeTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "smoke",
		Short: "Run smoke tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSmokeTests(cmd.Context())
		},
	}
	return cmd
}

func newDeterminismTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "determinism [run-id]",
		Short: "Test determinism of a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID, err := core.ParseRunID(args[0])
			if err != nil {
				return err
			}
			return testDeterminism(cmd.Context(), runID)
		},
	}
	return cmd
}

func generateSeedData(ctx context.Context) error {
	fmt.Println("Generating seed data...")

	// Initialize test kit
	kit, err := testkit.NewTestKit()
	if err != nil {
		return fmt.Errorf("failed to initialize test kit: %w", err)
	}

	// Generate deterministic test data
	snapshotID, err := kit.CreateTestSnapshot(ctx, "test_dataset", 100, 1000)
	if err != nil {
		return fmt.Errorf("failed to create test snapshot: %w", err)
	}

	fmt.Printf("Created test snapshot: %s\n", snapshotID)

	// Generate test contracts
	contracts := map[string]*testkit.TestContract{
		"inspection_count": {
			AsOfMode:        dataset.AsOfCountWindow,
			StatisticalType: dataset.TypeNumeric,
			WindowDays:      &[]int{30}[0],
		},
		"severity_score": {
			AsOfMode:        dataset.AsOfLatestValue,
			StatisticalType: dataset.TypeNumeric,
		},
		"has_violation": {
			AsOfMode:        dataset.AsOfExists,
			StatisticalType: dataset.TypeBinary,
		},
	}

	for varKey, contract := range contracts {
		err := kit.RegisterTestContract(ctx, core.VariableKey(varKey), contract)
		if err != nil {
			return fmt.Errorf("failed to register contract %s: %w", varKey, err)
		}
		fmt.Printf("Registered contract: %s\n", varKey)
	}

	// Generate test events
	err = kit.GenerateTestEvents(ctx, snapshotID, 1000)
	if err != nil {
		return fmt.Errorf("failed to generate test events: %w", err)
	}

	fmt.Println("Seed data generation completed successfully")
	return nil
}

func runSmokeTests(ctx context.Context) error {
	fmt.Println("Running smoke tests...")

	kit, err := testkit.NewTestKit()
	if err != nil {
		return fmt.Errorf("failed to initialize test kit: %w", err)
	}

	// Test basic functionality
	tests := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"snapshot_creation", func(ctx context.Context) error {
			_, err := kit.CreateTestSnapshot(ctx, "smoke_test", 10, 100)
			return err
		}},
		{"matrix_resolution", func(ctx context.Context) error {
			snapshotID, _ := kit.CreateTestSnapshot(ctx, "smoke_matrix", 10, 50)
			bundle, err := kit.ResolveTestMatrix(ctx, snapshotID, []string{"test_var"})
			if err != nil {
				return err
			}
			if bundle.RowCount() == 0 {
				return fmt.Errorf("no data resolved")
			}
			return nil
		}},
		{"stats_sweep", func(ctx context.Context) error {
			snapshotID, _ := kit.CreateTestSnapshot(ctx, "smoke_stats", 10, 50)
			_, err := kit.RunTestStatsSweep(ctx, snapshotID)
			return err
		}},
	}

	passed := 0
	for _, test := range tests {
		fmt.Printf("  Running %s...", test.name)
		if err := test.fn(ctx); err != nil {
			fmt.Printf(" FAILED: %v\n", err)
		} else {
			fmt.Println(" PASSED")
			passed++
		}
	}

	fmt.Printf("\nSmoke tests: %d/%d passed\n", passed, len(tests))
	if passed < len(tests) {
		return fmt.Errorf("some smoke tests failed")
	}

	return nil
}

func testDeterminism(ctx context.Context, runID core.RunID) error {
	fmt.Printf("Testing determinism for run %s...\n", runID)

	kit, err := testkit.NewTestKit()
	if err != nil {
		return fmt.Errorf("failed to initialize test kit: %w", err)
	}

	// Get original run details
	originalRun, err := kit.GetTestRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to get original run: %w", err)
	}

	// Re-run with same fingerprint
	fmt.Println("Re-running with same fingerprint...")
	newRun, err := kit.ReplayRun(ctx, originalRun.Fingerprint)
	if err != nil {
		return fmt.Errorf("failed to replay run: %w", err)
	}

	// Compare results
	if err := compareRuns(originalRun, newRun); err != nil {
		return fmt.Errorf("determinism test failed: %w", err)
	}

	fmt.Println("✓ Determinism test passed - results identical")
	return nil
}

func compareRuns(original, replay *testkit.TestRun) error {
	// Compare fingerprints
	if original.Fingerprint != replay.Fingerprint {
		return fmt.Errorf("fingerprints differ")
	}

	// Compare artifacts count
	if len(original.Artifacts) != len(replay.Artifacts) {
		return fmt.Errorf("artifact counts differ: %d vs %d",
			len(original.Artifacts), len(replay.Artifacts))
	}

	// Compare key metrics (simplified)
	for i, origArt := range original.Artifacts {
		replayArt := replay.Artifacts[i]

		// Compare types
		if origArt.Kind != replayArt.Kind {
			return fmt.Errorf("artifact %d type differs: %s vs %s",
				i, origArt.Kind, replayArt.Kind)
		}

		// For relationship artifacts, compare effect sizes
		if origArt.Kind == core.ArtifactRelationship {
			origPayload, ok := origArt.Payload.(map[string]interface{})
			if !ok {
				return fmt.Errorf("artifact %d original payload is not a map", i)
			}
			replayPayload, ok := replayArt.Payload.(map[string]interface{})
			if !ok {
				return fmt.Errorf("artifact %d replay payload is not a map", i)
			}

			origEffect := origPayload["effect_size"]
			replayEffect := replayPayload["effect_size"]
			if origEffect != replayEffect {
				return fmt.Errorf("artifact %d effect size differs: %v vs %v",
					i, origEffect, replayEffect)
			}
		}
	}

	return nil
}

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [up|down|status]",
		Short: "Run database migrations",
		Long: `Run database schema migrations.

Commands:
  up      Apply all pending migrations
  down    Rollback the last migration
  status  Show migration status`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			action := args[0]
			return runMigrations(cmd.Context(), action)
		},
	}
	return cmd
}

func runMigrations(ctx context.Context, action string) error {
	fmt.Printf("Running migrations: %s\n", action)

	// For development, use a file-based SQLite database for persistence
	// In production, this would be a real Postgres connection
	db, err := sql.Open("sqlite3", "./dev_migrations.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create migrator
	migrator := migrations.NewMigrator(db)

	// Execute action
	switch action {
	case "up":
		return migrator.Up(ctx)
	case "down":
		return migrator.Down(ctx)
	case "status":
		return migrator.Status(ctx)
	default:
		return fmt.Errorf("unknown migration action: %s", action)
	}
}
