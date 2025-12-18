package senses

import (
	"context"
	"fmt"
	"math"

	"gohypo/domain/core"
)

// WelchTTestSense detects significant differences between group means
type WelchTTestSense struct{}

// NewWelchTTestSense creates a new Welch's t-test sense
func NewWelchTTestSense() *WelchTTestSense {
	return &WelchTTestSense{}
}

// Name returns the sense name
func (s *WelchTTestSense) Name() string {
	return "welch_ttest"
}

// Description returns a human-readable description
func (s *WelchTTestSense) Description() string {
	return "Detects significant differences between group means with unequal variances"
}

// RequiresGroups indicates this sense can benefit from group segmentation
func (s *WelchTTestSense) RequiresGroups() bool {
	return true
}

// Analyze performs Welch's t-test between groups
func (s *WelchTTestSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) SenseResult {
	if len(x) != len(y) || len(x) < 4 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for Welch's t-test analysis",
		}
	}

	// For Welch's t-test, we need to identify groups
	// Strategy: if one variable looks binary/categorical, use it to split the other
	group1, group2 := s.identifyGroups(x, y)

	if len(group1) < 2 || len(group2) < 2 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Could not identify suitable groups for t-test comparison",
		}
	}

	// Compute Welch's t-test
	tStat, pValue, effectSize := s.computeWelchTTest(group1, group2)

	// Calculate confidence
	confidence := calculateConfidence(pValue)

	// Classify signal strength (using absolute t-statistic)
	signal := classifySignal(math.Abs(tStat), s.Name())

	// Generate description
	description := s.generateDescription(tStat, pValue, effectSize, len(group1), len(group2), string(varX), string(varY))

	return SenseResult{
		SenseName:   s.Name(),
		EffectSize:  effectSize, // Cohen's d effect size
		PValue:      pValue,
		Confidence:  confidence,
		Signal:      signal,
		Description: description,
		Metadata: map[string]interface{}{
			"t_statistic": tStat,
			"group1_size": len(group1),
			"group2_size": len(group2),
			"group1_mean": s.mean(group1),
			"group2_mean": s.mean(group2),
			"variable_x":  string(varX),
			"variable_y":  string(varY),
		},
	}
}

// identifyGroups attempts to split data into two groups for comparison
func (s *WelchTTestSense) identifyGroups(x, y []float64) ([]float64, []float64) {
	// Strategy 1: Check if x looks like a binary/categorical variable
	if s.isBinaryVariable(x) {
		return s.splitByBinaryVariable(x, y)
	}

	// Strategy 2: Check if y looks like a binary/categorical variable
	if s.isBinaryVariable(y) {
		return s.splitByBinaryVariable(y, x)
	}

	// Strategy 3: Use median split on the variable with more variance
	if s.variance(x) > s.variance(y) {
		return s.medianSplit(x, y)
	} else {
		return s.medianSplit(y, x)
	}
}

// isBinaryVariable checks if a variable appears to be binary/categorical
func (s *WelchTTestSense) isBinaryVariable(data []float64) bool {
	uniqueValues := make(map[float64]bool)
	for _, val := range data {
		if !math.IsNaN(val) {
			uniqueValues[val] = true
		}
	}

	// Consider it binary if it has exactly 2 unique values
	return len(uniqueValues) == 2
}

// splitByBinaryVariable splits numeric data by a binary grouping variable
func (s *WelchTTestSense) splitByBinaryVariable(groupVar, numericVar []float64) ([]float64, []float64) {
	// Find the two unique values in groupVar
	var val1, val2 float64
	foundFirst := false

	for _, val := range groupVar {
		if !math.IsNaN(val) {
			if !foundFirst {
				val1 = val
				foundFirst = true
			} else if val != val1 {
				val2 = val
				break
			}
		}
	}

	group1 := []float64{}
	group2 := []float64{}

	for i, g := range groupVar {
		if math.IsNaN(g) || math.IsNaN(numericVar[i]) {
			continue
		}

		if g == val1 {
			group1 = append(group1, numericVar[i])
		} else if g == val2 {
			group2 = append(group2, numericVar[i])
		}
	}

	return group1, group2
}

// medianSplit splits data around the median of the grouping variable
func (s *WelchTTestSense) medianSplit(groupVar, numericVar []float64) ([]float64, []float64) {
	// Calculate median of groupVar
	sorted := make([]float64, 0, len(groupVar))
	for _, val := range groupVar {
		if !math.IsNaN(val) {
			sorted = append(sorted, val)
		}
	}

	if len(sorted) < 2 {
		return []float64{}, []float64{}
	}

	// Simple median calculation
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	median := sorted[len(sorted)/2]

	group1 := []float64{}
	group2 := []float64{}

	for i, g := range groupVar {
		if math.IsNaN(g) || math.IsNaN(numericVar[i]) {
			continue
		}

		if g <= median {
			group1 = append(group1, numericVar[i])
		} else {
			group2 = append(group2, numericVar[i])
		}
	}

	return group1, group2
}

// computeWelchTTest performs Welch's t-test
func (s *WelchTTestSense) computeWelchTTest(group1, group2 []float64) (float64, float64, float64) {
	n1 := float64(len(group1))
	n2 := float64(len(group2))

	if n1 < 2 || n2 < 2 {
		return 0, 1.0, 0
	}

	// Calculate means
	mean1 := s.mean(group1)
	mean2 := s.mean(group2)

	// Calculate variances
	var1 := s.variance(group1)
	var2 := s.variance(group2)

	// Welch's t-statistic: t = (mean1 - mean2) / sqrt(var1/n1 + var2/n2)
	se := math.Sqrt(var1/n1 + var2/n2)
	tStat := (mean1 - mean2) / se

	// Degrees of freedom using Welch-Satterthwaite equation
	df := math.Pow(var1/n1+var2/n2, 2) / (math.Pow(var1/n1, 2)/(n1-1) + math.Pow(var2/n2, 2)/(n2-1))

	// p-value using t-distribution approximation
	pValue := 2 * (1 - s.tCDFApproximation(math.Abs(tStat), df))

	// Effect size (Cohen's d with pooled standard deviation)
	pooledSD := math.Sqrt(((n1-1)*var1 + (n2-1)*var2) / (n1 + n2 - 2))
	effectSize := (mean1 - mean2) / pooledSD

	return tStat, pValue, effectSize
}

// Helper statistical functions
func (s *WelchTTestSense) mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sum := 0.0
	for _, val := range data {
		sum += val
	}
	return sum / float64(len(data))
}

func (s *WelchTTestSense) variance(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}

	mean := s.mean(data)
	sumSq := 0.0

	for _, val := range data {
		diff := val - mean
		sumSq += diff * diff
	}

	return sumSq / float64(len(data)-1)
}

// tCDFApproximation approximates the cumulative distribution function of the t-distribution
func (s *WelchTTestSense) tCDFApproximation(t, df float64) float64 {
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

// generateDescription creates a human-readable description of the t-test result
func (s *WelchTTestSense) generateDescription(tStat, pValue, effectSize float64, n1, n2 int, varX, varY string) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant difference between groups (t=%.3f, p=%.3f, d=%.3f, n1=%d, n2=%d)", tStat, pValue, effectSize, n1, n2)
	}

	direction := "higher"
	if tStat < 0 {
		direction = "lower"
	}

	strength := ""
	absD := math.Abs(effectSize)
	if absD < 0.2 {
		strength = "small"
	} else if absD < 0.5 {
		strength = "medium"
	} else if absD < 0.8 {
		strength = "large"
	} else {
		strength = "very large"
	}

	return fmt.Sprintf("Significant group difference: Group 1 has %s %s mean than Group 2 (t=%.3f, p=%.3f, d=%.3f, n1=%d, n2=%d)", strength, direction, tStat, pValue, effectSize, n1, n2)
}
