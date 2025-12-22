//go:build ignore
// +build ignore

package main

import (
	"fmt"

	"gohypo/domain/stats/brief"
	analysisbrief "gohypo/internal/analysis/brief"
	"gohypo/internal/policy"
)

func main() {
	fmt.Println("🔬 GoHypo Statistical Infrastructure Demo")
	fmt.Println("=========================================")

	// Example dataset (simulated sports performance data)
	dataset := map[string][]float64{
		"HTFormPtsStr":    {0.67, 0.45, 0.89, 0.23, 0.78, 0.12, 0.91, 0.34, 0.56, 0.78},           // Sparse performance metric
		"conversion_rate": {0.023, 0.031, 0.028, 0.035, 0.029, 0.026, 0.033, 0.030, 0.027, 0.032}, // Normal distribution
		"spend_trend":     {1000, 1200, 1100, 1300, 1250, 1150, 1350, 1280, 1180, 1380},           // Trending time series
	}

	// Initialize statistical infrastructure
	computer := analysisbrief.NewComputer()
	policyGen := policy.NewPolicyGenerator()

	fmt.Println("\n📊 Profiling Dataset Columns:")
	fmt.Println("------------------------------")

	for columnName, data := range dataset {
		fmt.Printf("\n🔍 Analyzing column: %s\n", columnName)

		// Generate statistical brief using unified computation
		request := brief.ComputationRequest{
			ForValidation: true,
			ForHypothesis: true,
		}
		statBrief, err := computer.ComputeBrief(data, columnName, "", request)
		if err != nil {
			fmt.Printf("Error computing brief: %v\n", err)
			continue
		}

		// Generate adaptive policy
		config := policyGen.GenerateAdaptivePolicy(*statBrief)

		// Display results
		fmt.Printf("  Distribution: %.3f skewness, %.3f kurtosis (%s)\n",
			statBrief.Distribution.Skewness, statBrief.Distribution.Kurtosis,
			map[bool]string{true: "normal", false: "non-normal"}[statBrief.Distribution.IsNormal])

		fmt.Printf("  Quality: %.1f%% sparsity, %.2f noise coefficient\n",
			statBrief.Quality.SparsityRatio*100, statBrief.Quality.NoiseCoefficient)

		stationaryStr := "not analyzed"
		if statBrief.Temporal != nil {
			stationaryStr = map[bool]string{true: "stationary", false: "non-stationary"}[statBrief.Temporal.IsStationary]
		}
		fmt.Printf("  Stationarity: %s\n", stationaryStr)

		fmt.Printf("  LLM Summary: %s\n", statBrief.ToLLMFormat())

		fmt.Printf("  Adaptive Policy: α=%.4f, iterations=%d, nonparametric=%v\n",
			config.Shredder.Alpha, config.Shredder.Iterations, config.General.UseNonParametric)
	}

	fmt.Println("\n✅ Statistical Infrastructure Successfully Implemented!")
	fmt.Println("   - Replaced heuristic approximations with proper CDFs")
	fmt.Println("   - Added comprehensive data profiling")
	fmt.Println("   - Implemented adaptive policy generation")
	fmt.Println("   - Created LLM-optimized metadata formatting")
}



