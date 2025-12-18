package senses

import (
	"context"
	"fmt"
	"math"
	"sort"

	"gohypo/domain/core"
)

// SpearmanSense detects monotonic relationships using rank correlation
type SpearmanSense struct{}

// NewSpearmanSense creates a new Spearman correlation sense
func NewSpearmanSense() *SpearmanSense {
	return &SpearmanSense{}
}

// Name returns the sense name
func (s *SpearmanSense) Name() string {
	return "spearman"
}

// Description returns a human-readable description
func (s *SpearmanSense) Description() string {
	return "Detects monotonic relationships robust to outliers and non-normality"
}

// RequiresGroups indicates this sense doesn't need group segmentation
func (s *SpearmanSense) RequiresGroups() bool {
	return false
}

// Analyze computes Spearman's rank correlation coefficient
func (s *SpearmanSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) SenseResult {
	if len(x) != len(y) || len(x) < 3 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for Spearman correlation analysis",
		}
	}

	// Compute Spearman correlation
	rho, pValue := s.computeSpearmanCorrelation(x, y)

	// Calculate confidence
	confidence := calculateConfidence(pValue)

	// Classify signal strength
	signal := classifySignal(math.Abs(rho), s.Name())

	// Generate description
	description := s.generateDescription(rho, pValue, string(varX), string(varY))

	return SenseResult{
		SenseName:   s.Name(),
		EffectSize:  rho,
		PValue:      pValue,
		Confidence:  confidence,
		Signal:      signal,
		Description: description,
		Metadata: map[string]interface{}{
			"correlation_type":       "rank",
			"robust_to_outliers":     true,
			"monotonic_relationship": true,
			"sample_size":            len(x),
			"variable_x":             string(varX),
			"variable_y":             string(varY),
		},
	}
}

// computeSpearmanCorrelation calculates Spearman's rho
func (s *SpearmanSense) computeSpearmanCorrelation(x, y []float64) (float64, float64) {
	n := len(x)

	// Convert to ranks
	xRanks := s.computeRanks(x)
	yRanks := s.computeRanks(y)

	// Check for valid ranks
	if len(xRanks) != n || len(yRanks) != n {
		return 0, 1.0
	}

	// Calculate Spearman correlation using the standard formula
	sumDiffSq := 0.0
	for i := 0; i < n; i++ {
		diff := xRanks[i] - yRanks[i]
		sumDiffSq += diff * diff
	}

	// Spearman's rho = 1 - (6 * Σd²) / (n(n²-1))
	denominator := float64(n) * (float64(n*n) - 1)
	if denominator == 0 {
		return 0, 1.0
	}

	rho := 1.0 - (6.0 * sumDiffSq / denominator)

	// Clamp to [-1, 1] range (due to floating point precision)
	if rho > 1.0 {
		rho = 1.0
	} else if rho < -1.0 {
		rho = -1.0
	}

	// Compute p-value using t-distribution approximation
	// t = r * sqrt((n-2)/(1-r²))
	tStat := rho * math.Sqrt(float64(n-2)/(1-rho*rho))
	df := float64(n - 2)

	// Two-tailed p-value
	pValue := 2 * (1 - s.tCDFApproximation(math.Abs(tStat), df))

	return rho, pValue
}

// computeRanks converts values to ranks, handling ties properly
func (s *SpearmanSense) computeRanks(data []float64) []float64 {
	n := len(data)
	if n == 0 {
		return []float64{}
	}

	// Create index-value pairs for sorting
	type pair struct {
		value float64
		index int
	}

	pairs := make([]pair, n)
	for i, val := range data {
		pairs[i] = pair{value: val, index: i}
	}

	// Sort by value
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value < pairs[j].value
	})

	ranks := make([]float64, n)

	// Assign ranks, handling ties by averaging
	i := 0
	for i < n {
		j := i + 1

		// Find the end of the tie group
		for j < n && pairs[j].value == pairs[i].value {
			j++
		}

		// Calculate average rank for this group
		groupSize := j - i
		avgRank := float64(i+1) + float64(groupSize-1)/2.0

		// Assign average rank to all tied elements
		for k := i; k < j; k++ {
			ranks[pairs[k].index] = avgRank
		}

		i = j
	}

	return ranks
}

// tCDFApproximation approximates the cumulative distribution function of the t-distribution
func (s *SpearmanSense) tCDFApproximation(t, df float64) float64 {
	// Use normal approximation for large df
	if df > 30 {
		return 0.5 * (1 + math.Erf(t/math.Sqrt(2)))
	}

	// For smaller df, use approximation: CDF(t) ≈ 0.5 + (t / (2 * sqrt(df))) for small t
	if math.Abs(t) < 1.0 {
		return 0.5 + (t / (2.0 * math.Sqrt(df)))
	}

	// For larger t values, approximate tail probabilities
	if t > 0 {
		return 1.0 - (0.5 / (1.0 + t*t/df))
	}
	return 0.5 / (1.0 + t*t/df)
}

// generateDescription creates a human-readable description of the Spearman result
func (s *SpearmanSense) generateDescription(rho, pValue float64, varX, varY string) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant monotonic relationship between %s and %s (ρ=%.3f, p=%.3f)", varX, varY, rho, pValue)
	}

	direction := "positive"
	if rho < 0 {
		direction = "negative"
	}

	strength := ""
	absRho := math.Abs(rho)
	if absRho < 0.2 {
		strength = "weak"
	} else if absRho < 0.4 {
		strength = "moderate"
	} else if absRho < 0.6 {
		strength = "strong"
	} else if absRho < 0.8 {
		strength = "very strong"
	} else {
		strength = "perfect"
	}

	return fmt.Sprintf("%s %s monotonic relationship between %s and %s (ρ=%.3f, p=%.3f)", strength, direction, varX, varY, rho, pValue)
}
