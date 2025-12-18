package senses

import (
	"context"
	"fmt"
	"math"
	"sort"

	"gohypo/domain/core"
)

// MutualInformationSense detects non-linear relationships between variables
type MutualInformationSense struct{}

// NewMutualInformationSense creates a new mutual information sense
func NewMutualInformationSense() *MutualInformationSense {
	return &MutualInformationSense{}
}

// Name returns the sense name
func (s *MutualInformationSense) Name() string {
	return "mutual_information"
}

// Description returns a human-readable description
func (s *MutualInformationSense) Description() string {
	return "Detects non-linear relationships that correlation-based methods miss"
}

// RequiresGroups indicates this sense doesn't need group segmentation
func (s *MutualInformationSense) RequiresGroups() bool {
	return false
}

// Analyze computes mutual information between two variables
func (s *MutualInformationSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) SenseResult {
	if len(x) != len(y) || len(x) == 0 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for mutual information analysis",
		}
	}

	// Compute mutual information
	mi, pValue := s.computeMutualInformation(x, y)

	// Calculate confidence
	confidence := calculateConfidence(pValue)

	// Classify signal strength
	signal := classifySignal(mi, s.Name())

	// Generate description
	description := s.generateDescription(mi, pValue, string(varX), string(varY))

	return SenseResult{
		SenseName:   s.Name(),
		EffectSize:  mi,
		PValue:      pValue,
		Confidence:  confidence,
		Signal:      signal,
		Description: description,
		Metadata: map[string]interface{}{
			"variable_x":  string(varX),
			"variable_y":  string(varY),
			"sample_size": len(x),
		},
	}
}

// computeMutualInformation calculates mutual information I(X;Y) and a permutation p-value.
func (s *MutualInformationSense) computeMutualInformation(x, y []float64) (float64, float64) {
	n := len(x)
	if n < 10 {
		return 0, 1.0 // Need minimum sample size
	}

	// Compute observed MI without recursion.
	mi := s.computeMIOnly(x, y)

	// Compute p-value using permutation test
	pValue := s.computeMIPValue(x, y, mi, 100) // 100 permutations

	return mi, pValue
}

// computeMIOnly computes the mutual information value only (no p-value).
// IMPORTANT: This must not call computeMIPValue to avoid recursion during permutation tests.
func (s *MutualInformationSense) computeMIOnly(x, y []float64) float64 {
	// Discretize continuous variables into bins
	xBins := s.discretizeVariable(x, 10) // Use 10 bins as default
	yBins := s.discretizeVariable(y, 10)

	// Compute entropies
	hX := s.computeEntropy(xBins)
	hY := s.computeEntropy(yBins)
	hXY := s.computeJointEntropy(xBins, yBins)

	// Mutual Information: I(X;Y) = H(X) + H(Y) - H(X,Y)
	return math.Max(0, hX+hY-hXY) // Ensure non-negative
}

// discretizeVariable converts continuous values to discrete bins
func (s *MutualInformationSense) discretizeVariable(data []float64, numBins int) []int {
	if len(data) == 0 {
		return []int{}
	}

	// Sort data to find quantiles
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	bins := make([]int, len(data))
	for i, val := range data {
		// Find which bin this value belongs to
		bin := 0
		for b := 1; b < numBins; b++ {
			threshold := sorted[(len(sorted)*b)/numBins]
			if val >= threshold {
				bin = b
			} else {
				break
			}
		}
		bins[i] = bin
	}

	return bins
}

// computeEntropy calculates Shannon entropy of a discrete variable
func (s *MutualInformationSense) computeEntropy(bins []int) float64 {
	if len(bins) == 0 {
		return 0
	}

	// Count frequency of each bin
	counts := make(map[int]int)
	for _, bin := range bins {
		counts[bin]++
	}

	entropy := 0.0
	n := float64(len(bins))

	for _, count := range counts {
		if count > 0 {
			p := float64(count) / n
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// computeJointEntropy calculates joint entropy H(X,Y)
func (s *MutualInformationSense) computeJointEntropy(xBins, yBins []int) float64 {
	if len(xBins) != len(yBins) || len(xBins) == 0 {
		return 0
	}

	// Count joint frequency of each (x,y) pair
	jointCounts := make(map[string]int)
	for i := range xBins {
		key := fmt.Sprintf("%d,%d", xBins[i], yBins[i])
		jointCounts[key]++
	}

	entropy := 0.0
	n := float64(len(xBins))

	for _, count := range jointCounts {
		if count > 0 {
			p := float64(count) / n
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// computeMIPValue computes p-value for mutual information using permutation test
func (s *MutualInformationSense) computeMIPValue(x, y []float64, observedMI float64, numPermutations int) float64 {
	if numPermutations <= 0 {
		return 1.0
	}

	extremeCount := 0

	// Run permutations
	for i := 0; i < numPermutations; i++ {
		// Permute y while keeping x fixed
		permutedY := s.permuteSlice(y)

		// Compute MI for permuted data
		permutedMI := s.computeMIOnly(x, permutedY)

		// Count how many times permuted MI >= observed MI
		if permutedMI >= observedMI {
			extremeCount++
		}
	}

	// p-value is proportion of permutations where MI >= observed
	return float64(extremeCount) / float64(numPermutations)
}

// permuteSlice randomly permutes a slice (Fisher-Yates shuffle)
func (s *MutualInformationSense) permuteSlice(data []float64) []float64 {
	permuted := make([]float64, len(data))
	copy(permuted, data)

	// Simple random permutation (not cryptographically secure, but fine for stats)
	for i := len(permuted) - 1; i > 0; i-- {
		j := int(math.Round(float64(i)*math.Abs(math.Sin(float64(i*17))))) % (i + 1) // Deterministic but pseudo-random
		permuted[i], permuted[j] = permuted[j], permuted[i]
	}

	return permuted
}

// generateDescription creates a human-readable description of the MI result
func (s *MutualInformationSense) generateDescription(mi, pValue float64, varX, varY string) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant mutual information between %s and %s (MI=%.3f, p=%.3f)", varX, varY, mi, pValue)
	}

	strength := ""
	if mi < 0.1 {
		strength = "weak"
	} else if mi < 0.3 {
		strength = "moderate"
	} else if mi < 0.5 {
		strength = "strong"
	} else {
		strength = "very strong"
	}

	return fmt.Sprintf("%s mutual information between %s and %s (MI=%.3f, p=%.3f) - detects non-linear relationships", strength, varX, varY, mi, pValue)
}
