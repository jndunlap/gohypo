package referee

import (
	"fmt"
	"math"
	"math/rand"
)

// Shredder implements permutation shuffling for statistical integrity
type Shredder struct {
	Iterations int     // Number of permutation iterations
	Alpha      float64 // Significance threshold
}

// Execute runs permutation testing to detect statistical flukes
func (s *Shredder) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Permutation_Shredder",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if s.Iterations == 0 {
		s.Iterations = SHREDDER_ITERATIONS
	}
	if s.Alpha == 0 {
		s.Alpha = SHREDDER_P_ALPHA
	}

	// Compute observed effect size
	observedEffect := s.computeEffectSize(x, y)

	// Generate null distribution through permutation
	nullDistribution := s.generateNullDistribution(x, y, s.Iterations)

	// Calculate empirical p-value
	extremeCount := 0
	for _, nullEffect := range nullDistribution {
		if math.Abs(nullEffect) >= math.Abs(observedEffect) {
			extremeCount++
		}
	}
	pValue := float64(extremeCount) / float64(s.Iterations)

	// Apply centralized standard
	passed := pValue <= s.Alpha

	failureReason := ""
	if !passed {
		if pValue >= 0.5 {
			failureReason = fmt.Sprintf("CRITICAL: No statistical relationship detected (p=%.6f). The data shows completely random behavior - hypothesis is not supported by evidence. Expected strong causal signal for valid hypothesis.", pValue)
		} else if pValue >= 0.05 {
			failureReason = fmt.Sprintf("WEAK SIGNAL: Hypothesis shows some relationship but lacks statistical rigor (p=%.6f). May be due to noise, small sample, or weak effect. Need p<0.001 for causal confidence.", pValue)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT PRECISION: Statistical test passed but p=%.6f doesn't meet PhD standard of p<0.001. Hypothesis may be true but requires more data or stronger effect size.", pValue)
		}
	}

	return RefereeResult{
		GateName:  "Permutation_Shredder",
		Passed:    passed,
		Statistic: observedEffect,
		PValue:    pValue,
		StandardUsed: fmt.Sprintf("Two-tailed permutation (N=%d) with p ≤ %.3f (%.1f%% confidence)",
			s.Iterations, s.Alpha, (1-s.Alpha)*100),
		FailureReason: failureReason,
	}
}

func (s *Shredder) computeEffectSize(x, y []float64) float64 {
	// Use Pearson correlation as default effect size
	return s.pearsonCorrelation(x, y)
}

func (s *Shredder) generateNullDistribution(x, y []float64, iterations int) []float64 {
	nullDist := make([]float64, iterations)

	for i := 0; i < iterations; i++ {
		// Shuffle x variable (driver)
		shuffledX := make([]float64, len(x))
		copy(shuffledX, x)

		// Fisher-Yates shuffle
		for j := len(shuffledX) - 1; j > 0; j-- {
			k := rand.Intn(j + 1)
			shuffledX[j], shuffledX[k] = shuffledX[j], shuffledX[k]
		}

		// Compute effect size with shuffled data
		nullDist[i] = s.computeEffectSize(shuffledX, y)
	}

	return nullDist
}

func (s *Shredder) pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		// Handle NaN and Inf values
		if math.IsNaN(x[i]) || math.IsInf(x[i], 0) || math.IsNaN(y[i]) || math.IsInf(y[i], 0) {
			continue
		}
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if denominator == 0 || math.IsNaN(denominator) {
		return 0
	}

	result := numerator / denominator
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}

	return result
}
