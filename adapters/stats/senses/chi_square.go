package senses

import (
	"context"
	"fmt"
	"math"

	"gohypo/domain/core"
)

// ChiSquareSense detects associations between categorical variables
type ChiSquareSense struct{}

// NewChiSquareSense creates a new Chi-Square sense
func NewChiSquareSense() *ChiSquareSense {
	return &ChiSquareSense{}
}

// Name returns the sense name
func (s *ChiSquareSense) Name() string {
	return "chi_square"
}

// Description returns a human-readable description
func (s *ChiSquareSense) Description() string {
	return "Detects associations between categorical variables and distribution anomalies"
}

// RequiresGroups indicates this sense works with categorical data
func (s *ChiSquareSense) RequiresGroups() bool {
	return false
}

// Analyze performs Chi-Square test of independence
func (s *ChiSquareSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) SenseResult {
	if len(x) != len(y) || len(x) < 10 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for Chi-Square analysis",
		}
	}

	// Discretize variables into categories
	xCats := s.discretizeForChiSquare(x, 3) // Use 3 categories as default
	yCats := s.discretizeForChiSquare(y, 3)

	// Build contingency table
	table := s.buildContingencyTable(xCats, yCats)

	if len(table) < 2 || len(table[0]) < 2 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Could not build suitable contingency table for Chi-Square test",
		}
	}

	// Compute Chi-Square statistic and p-value
	chiSq, pValue, effectSize := s.computeChiSquare(table)

	// Calculate confidence
	confidence := calculateConfidence(pValue)

	// Classify signal strength
	signal := classifySignal(chiSq, s.Name())

	// Generate description
	description := s.generateDescription(chiSq, pValue, effectSize, table, string(varX), string(varY))

	return SenseResult{
		SenseName:   s.Name(),
		EffectSize:  effectSize, // Cramer's V effect size
		PValue:      pValue,
		Confidence:  confidence,
		Signal:      signal,
		Description: description,
		Metadata: map[string]interface{}{
			"chi_square_stat": chiSq,
			"degrees_freedom": (len(table) - 1) * (len(table[0]) - 1),
			"table_rows":      len(table),
			"table_cols":      len(table[0]),
			"variable_x":      string(varX),
			"variable_y":      string(varY),
		},
	}
}

// discretizeForChiSquare converts continuous variables to categorical bins for Chi-Square
func (s *ChiSquareSense) discretizeForChiSquare(data []float64, maxCategories int) []int {
	if len(data) == 0 {
		return []int{}
	}

	// For Chi-Square, we want balanced categories
	// Use quantiles to ensure roughly equal distribution
	sorted := make([]float64, 0, len(data))
	for _, val := range data {
		if !math.IsNaN(val) {
			sorted = append(sorted, val)
		}
	}

	if len(sorted) < maxCategories {
		maxCategories = len(sorted)
	}

	// Simple sort
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Create quantile-based bins
	bins := make([]int, len(data))
	for i, val := range data {
		if math.IsNaN(val) {
			bins[i] = -1 // Missing value indicator
			continue
		}

		// Find which quantile bin this value belongs to
		bin := 0
		for b := 1; b < maxCategories; b++ {
			thresholdIdx := (len(sorted) * b) / maxCategories
			if thresholdIdx < len(sorted) && val >= sorted[thresholdIdx] {
				bin = b
			} else {
				break
			}
		}
		bins[i] = bin
	}

	return bins
}

// buildContingencyTable creates a contingency table from two categorical variables
func (s *ChiSquareSense) buildContingencyTable(xCats, yCats []int) [][]int {
	if len(xCats) != len(yCats) {
		return [][]int{}
	}

	// Find the range of categories
	maxX, maxY := 0, 0
	for i := range xCats {
		if xCats[i] > maxX {
			maxX = xCats[i]
		}
		if yCats[i] > maxY {
			maxY = yCats[i]
		}
	}

	// Initialize contingency table
	table := make([][]int, maxX+1)
	for i := range table {
		table[i] = make([]int, maxY+1)
	}

	// Fill contingency table
	for i := range xCats {
		if xCats[i] >= 0 && yCats[i] >= 0 {
			table[xCats[i]][yCats[i]]++
		}
	}

	return table
}

// computeChiSquare calculates the Chi-Square statistic and associated metrics
func (s *ChiSquareSense) computeChiSquare(table [][]int) (float64, float64, float64) {
	rows := len(table)
	if rows == 0 {
		return 0, 1.0, 0
	}
	cols := len(table[0])

	// Calculate total sample size
	total := 0
	for i := range table {
		for j := range table[i] {
			total += table[i][j]
		}
	}

	if total < 5 {
		return 0, 1.0, 0 // Too small sample
	}

	// Calculate expected frequencies and Chi-Square statistic
	chiSq := 0.0
	rowTotals := make([]int, rows)
	colTotals := make([]int, cols)

	// Calculate marginal totals
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			rowTotals[i] += table[i][j]
			colTotals[j] += table[i][j]
		}
	}

	// Calculate Chi-Square statistic
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			expected := float64(rowTotals[i]*colTotals[j]) / float64(total)
			if expected > 0 {
				observed := float64(table[i][j])
				chiSq += math.Pow(observed-expected, 2) / expected
			}
		}
	}

	// Degrees of freedom
	df := float64((rows - 1) * (cols - 1))

	// p-value approximation using Chi-Square distribution
	pValue := s.chiSquareCDF(chiSq, df)

	// Effect size: Cramer's V = sqrt(χ² / (n * min(r-1, c-1)))
	minDim := math.Min(float64(rows-1), float64(cols-1))
	cramerV := math.Sqrt(chiSq / (float64(total) * minDim))

	return chiSq, pValue, cramerV
}

// chiSquareCDF approximates the cumulative distribution function of Chi-Square
func (s *ChiSquareSense) chiSquareCDF(chiSq, df float64) float64 {
	// Use Wilson-Hilferty transformation for approximation
	// χ² ≈ df * (1 - 2/(9*df) + z*sqrt(2/(9*df)))^3
	// where z is from standard normal

	if chiSq <= 0 {
		return 0
	}

	// For large df, use normal approximation
	if df > 30 {
		mean := df
		std := math.Sqrt(2 * df)
		z := (chiSq - mean) / std
		return 0.5 * (1 + math.Erf(z/math.Sqrt(2)))
	}

	// Wilson-Hilferty transformation
	z := math.Cbrt((chiSq/df - 1 + 2/(9*df)) / math.Sqrt(2/(9*df)))
	cdf := 0.5 * (1 + math.Erf(z/math.Sqrt(2)))

	// For very small p-values, the approximation might be poor
	// Clamp to reasonable range
	if cdf < 0 {
		cdf = 0
	}
	if cdf > 1 {
		cdf = 1
	}

	return 1 - cdf // We want p-value, so 1 - CDF
}

// generateDescription creates a human-readable description of the Chi-Square result
func (s *ChiSquareSense) generateDescription(chiSq, pValue, cramerV float64, table [][]int, varX, varY string) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant association between %s and %s (χ²=%.3f, p=%.3f, V=%.3f)", varX, varY, chiSq, pValue, cramerV)
	}

	strength := ""
	if cramerV < 0.1 {
		strength = "weak"
	} else if cramerV < 0.3 {
		strength = "moderate"
	} else if cramerV < 0.5 {
		strength = "strong"
	} else {
		strength = "very strong"
	}

	rows := len(table)
	cols := len(table[0])

	return fmt.Sprintf("%s association between %s and %s (χ²=%.3f, p=%.3f, V=%.3f, %dx%d table)", strength, varX, varY, chiSq, pValue, cramerV, rows, cols)
}
