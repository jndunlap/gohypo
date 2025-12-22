package validation

import (
	"context"
	"fmt"
	"math/rand"

	"gohypo/domain/core"
	"gohypo/internal/referee"
)

type StabilitySelectionConfig struct {
	SubsampleCount     int     // Number of random subsamples (default: 10)
	SubsampleFraction  float64 // Fraction of data per subsample (default: 0.8)
	StabilityThreshold float64 // Minimum fraction of subsamples that must pass (default: 0.8)
	RandomSeed         int64   // For reproducible results
}

type StabilitySelector struct {
	config   StabilitySelectionConfig
	executor *ConcurrentExecutor
}

func NewStabilitySelector(config StabilitySelectionConfig) *StabilitySelector {
	if config.SubsampleCount == 0 {
		config.SubsampleCount = 10
	}
	if config.SubsampleFraction == 0 {
		config.SubsampleFraction = 0.8
	}
	if config.StabilityThreshold == 0 {
		config.StabilityThreshold = 0.8
	}

	return &StabilitySelector{
		config:   config,
		executor: NewConcurrentExecutor(50), // Default capacity
	}
}

// ValidateWithStability performs stability selection validation
func (ss *StabilitySelector) ValidateWithStability(
	ctx context.Context,
	refereeNames []string,
	fullXData, fullYData []float64,
	variableX, variableY core.VariableKey,
) (*StabilityResult, error) {

	rng := rand.New(rand.NewSource(ss.config.RandomSeed))

	type subsampleResult struct {
		subsampleIndex int
		refereeResults []referee.RefereeResult
		error          error
	}

	resultsChan := make(chan subsampleResult, ss.config.SubsampleCount)

	// Run validation on multiple subsamples concurrently
	for i := 0; i < ss.config.SubsampleCount; i++ {
		go func(subsampleIndex int) {
			// Create subsample with replacement
			xSub, ySub := ss.createSubsample(fullXData, fullYData, rng)

			// Run all referees on this subsample
			results, err := ss.executor.ExecuteReferees(ctx, refereeNames, xSub, ySub)

			resultsChan <- subsampleResult{
				subsampleIndex: subsampleIndex,
				refereeResults: results,
				error:          err,
			}
		}(i)
	}

	// Collect results from all subsamples
	subsampleResults := make([][]referee.RefereeResult, ss.config.SubsampleCount)
	var errors []error

	for i := 0; i < ss.config.SubsampleCount; i++ {
		result := <-resultsChan
		if result.error != nil {
			errors = append(errors, result.error)
			continue
		}
		subsampleResults[result.subsampleIndex] = result.refereeResults
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("stability selection failed: %v", errors)
	}

	return ss.analyzeStability(subsampleResults, refereeNames), nil
}

// createSubsample creates a random subsample with replacement
func (ss *StabilitySelector) createSubsample(xData, yData []float64, rng *rand.Rand) ([]float64, []float64) {
	n := len(xData)
	subsampleSize := int(float64(n) * ss.config.SubsampleFraction)

	xSub := make([]float64, subsampleSize)
	ySub := make([]float64, subsampleSize)

	for i := 0; i < subsampleSize; i++ {
		idx := rng.Intn(n)
		xSub[i] = xData[idx]
		ySub[i] = yData[idx]
	}

	return xSub, ySub
}

// analyzeStability determines which hypotheses are stable across subsamples
func (ss *StabilitySelector) analyzeStability(
	subsampleResults [][]referee.RefereeResult,
	refereeNames []string,
) *StabilityResult {

	result := &StabilityResult{
		SubsampleCount:     ss.config.SubsampleCount,
		RefereeStability:   make(map[string]RefereeStability),
		OverallStability:   0.0,
		StableHypotheses:   []string{},
		UnstableHypotheses: []string{},
		RefereeNames:       refereeNames,
		SubsampleResults:   make([]SubsampleResult, len(subsampleResults)),
		StabilityThreshold: ss.config.StabilityThreshold,
	}

	// Store detailed subsample results for heatmap
	for i, subsampleResult := range subsampleResults {
		result.SubsampleResults[i] = SubsampleResult{
			SubsampleIndex: i,
			RefereeResults: subsampleResult,
		}
	}

	// Analyze stability for each referee
	for refereeIndex, refereeName := range refereeNames {
		passCount := 0

		for _, subsampleResult := range subsampleResults {
			if refereeIndex < len(subsampleResult) && subsampleResult[refereeIndex].Passed {
				passCount++
			}
		}

		stabilityScore := float64(passCount) / float64(ss.config.SubsampleCount)
		isStable := stabilityScore >= ss.config.StabilityThreshold

		result.RefereeStability[refereeName] = RefereeStability{
			RefereeName:    refereeName,
			StabilityScore: stabilityScore,
			PassCount:      passCount,
			IsStable:       isStable,
		}

		if isStable {
			result.StableHypotheses = append(result.StableHypotheses, refereeName)
		} else {
			result.UnstableHypotheses = append(result.UnstableHypotheses, refereeName)
		}
	}

	// Calculate overall stability (fraction of referees that are stable)
	stableCount := 0
	for _, stability := range result.RefereeStability {
		if stability.IsStable {
			stableCount++
		}
	}
	result.OverallStability = float64(stableCount) / float64(len(refereeNames))

	// Calculate minimum subsamples needed for stability
	result.MinStableSubs = int(float64(ss.config.SubsampleCount) * ss.config.StabilityThreshold)

	return result
}

type StabilityResult struct {
	SubsampleCount     int
	RefereeStability   map[string]RefereeStability
	OverallStability   float64
	StableHypotheses   []string
	UnstableHypotheses []string
	RefereeNames       []string                    // For UI display order
	SubsampleResults   []SubsampleResult          // Detailed per-subsample data
	StabilityThreshold float64                    // Threshold used for stability
	MinStableSubs      int                        // Minimum subsamples needed for stability
}

type SubsampleResult struct {
	SubsampleIndex   int
	RefereeResults   []referee.RefereeResult
}

type RefereeStability struct {
	RefereeName    string
	StabilityScore float64 // 0.0 to 1.0
	PassCount      int
	IsStable       bool
}
