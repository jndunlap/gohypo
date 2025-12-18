package senses

import (
	"context"
	"fmt"
	"math"

	"gohypo/domain/core"
)

// CrossCorrelationSense detects temporal dependencies and lagged relationships
type CrossCorrelationSense struct{}

// NewCrossCorrelationSense creates a new cross-correlation sense
func NewCrossCorrelationSense() *CrossCorrelationSense {
	return &CrossCorrelationSense{}
}

// Name returns the sense name
func (s *CrossCorrelationSense) Name() string {
	return "cross_correlation"
}

// Description returns a human-readable description
func (s *CrossCorrelationSense) Description() string {
	return "Detects temporal dependencies and time-lagged relationships"
}

// RequiresGroups indicates this sense doesn't need group segmentation
func (s *CrossCorrelationSense) RequiresGroups() bool {
	return false
}

// Analyze computes cross-correlation with lag detection
func (s *CrossCorrelationSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) SenseResult {
	if len(x) != len(y) || len(x) < 10 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for cross-correlation analysis",
		}
	}

	// Compute cross-correlations for different lags
	maxLag := len(x) / 4 // Up to 25% of series length
	if maxLag > 20 {
		maxLag = 20 // Cap at 20 lags
	}
	if maxLag < 1 {
		maxLag = 1
	}

	correlations := s.computeCrossCorrelations(x, y, maxLag)

	// Find the maximum absolute correlation and its lag
	maxCorr, bestLag := s.findBestCorrelation(correlations)

	// Assess statistical significance
	pValue := s.assessSignificance(maxCorr, len(x), maxLag)

	// Calculate confidence
	confidence := calculateConfidence(pValue)

	// Classify signal strength
	signal := classifySignal(math.Abs(maxCorr), s.Name())

	// Generate description
	description := s.generateDescription(maxCorr, bestLag, pValue, string(varX), string(varY))

	return SenseResult{
		SenseName:   s.Name(),
		EffectSize:  maxCorr,
		PValue:      pValue,
		Confidence:  confidence,
		Signal:      signal,
		Description: description,
		Metadata: map[string]interface{}{
			"best_lag":          bestLag,
			"max_absolute_corr": math.Abs(maxCorr),
			"lag_range_tested":  fmt.Sprintf("-%d to +%d", maxLag, maxLag),
			"direction":         s.getDirectionDescription(maxCorr, bestLag),
			"sample_size":       len(x),
			"variable_x":        string(varX),
			"variable_y":        string(varY),
		},
	}
}

// computeCrossCorrelations calculates cross-correlation for multiple lags
func (s *CrossCorrelationSense) computeCrossCorrelations(x, y []float64, maxLag int) map[int]float64 {
	correlations := make(map[int]float64)

	// Test both positive and negative lags
	for lag := -maxLag; lag <= maxLag; lag++ {
		corr := s.computeCrossCorrelationAtLag(x, y, lag)
		correlations[lag] = corr
	}

	return correlations
}

// computeCrossCorrelationAtLag calculates correlation between x and y shifted by lag
func (s *CrossCorrelationSense) computeCrossCorrelationAtLag(x, y []float64, lag int) float64 {
	n := len(x)

	// For positive lag: correlate x[t] with y[t+lag]
	// For negative lag: correlate x[t] with y[t-lag] (which is y[t] with x[t+lag])
	if lag >= 0 {
		return s.computePearsonCorrelation(safeSlice(x, 0, n-lag), safeSlice(y, lag, n))
	} else {
		absLag := -lag
		return s.computePearsonCorrelation(safeSlice(x, absLag, n), safeSlice(y, 0, n-absLag))
	}
}

// safeSlice creates a safe slice from start to end index
func safeSlice(data []float64, start, end int) []float64 {
	if start < 0 {
		start = 0
	}
	if end > len(data) {
		end = len(data)
	}
	if start >= end {
		return []float64{}
	}
	return data[start:end]
}

// computePearsonCorrelation calculates Pearson correlation coefficient
func (s *CrossCorrelationSense) computePearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	n := float64(len(x))

	// Calculate means
	sumX, sumY := 0.0, 0.0
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX, meanY := sumX/n, sumY/n

	// Calculate correlation components
	numerator := 0.0
	sumXX, sumYY := 0.0, 0.0

	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	denominator := math.Sqrt(sumXX * sumYY)
	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

// findBestCorrelation finds the lag with maximum absolute correlation
func (s *CrossCorrelationSense) findBestCorrelation(correlations map[int]float64) (float64, int) {
	maxCorr := 0.0
	bestLag := 0

	for lag, corr := range correlations {
		absCorr := math.Abs(corr)
		if absCorr > math.Abs(maxCorr) {
			maxCorr = corr
			bestLag = lag
		}
	}

	return maxCorr, bestLag
}

// assessSignificance assesses statistical significance of cross-correlation
func (s *CrossCorrelationSense) assessSignificance(corr float64, sampleSize, maxLag int) float64 {
	absCorr := math.Abs(corr)
	if absCorr == 0 {
		return 1.0
	}

	// Effective sample size is reduced by lag
	effectiveN := sampleSize - maxLag*2
	if effectiveN < 5 {
		effectiveN = sampleSize
	}

	// Use Fisher's z-transformation for significance testing
	// z = 0.5 * ln((1+r)/(1-r))
	z := 0.5 * math.Log((1+absCorr)/(1-absCorr))

	// Standard error of z: 1/sqrt(n-3)
	se := 1.0 / math.Sqrt(float64(effectiveN-3))

	// Test statistic: z / se
	testStat := z / se

	// p-value using normal distribution approximation
	pValue := 2 * (1 - 0.5*(1+math.Erf(testStat/math.Sqrt(2))))

	// Clamp to reasonable range
	if pValue < 1e-10 {
		pValue = 1e-10
	}

	return pValue
}

// getDirectionDescription describes the relationship direction and lag
func (s *CrossCorrelationSense) getDirectionDescription(corr float64, lag int) string {
	direction := "positive"
	if corr < 0 {
		direction = "negative"
	}

	if lag == 0 {
		return fmt.Sprintf("%s contemporaneous", direction)
	} else if lag > 0 {
		return fmt.Sprintf("%s (X leads Y by %d periods)", direction, lag)
	} else {
		return fmt.Sprintf("%s (Y leads X by %d periods)", direction, -lag)
	}
}

// generateDescription creates a human-readable description of the cross-correlation result
func (s *CrossCorrelationSense) generateDescription(corr float64, lag int, pValue float64, varX, varY string) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant temporal relationship between %s and %s (r=%.3f at lag %d, p=%.3f)", varX, varY, corr, lag, pValue)
	}

	absCorr := math.Abs(corr)
	strength := ""
	if absCorr < 0.3 {
		strength = "weak"
	} else if absCorr < 0.6 {
		strength = "moderate"
	} else if absCorr < 0.8 {
		strength = "strong"
	} else {
		strength = "very strong"
	}

	relationship := ""
	if lag == 0 {
		relationship = "contemporaneous"
	} else if lag > 0 {
		relationship = fmt.Sprintf("%s leads %s by %d periods", varX, varY, lag)
	} else {
		relationship = fmt.Sprintf("%s leads %s by %d periods", varY, varX, -lag)
	}

	sign := "positive"
	if corr < 0 {
		sign = "negative"
	}

	return fmt.Sprintf("%s %s temporal relationship: %s (r=%.3f, lag=%d, p=%.3f)", strength, sign, relationship, corr, lag, pValue)
}
