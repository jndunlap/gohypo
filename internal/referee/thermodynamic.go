package referee

import (
	"fmt"
	"math"
	"strconv"
)

// LempelZivComplexity implements Lempel-Ziv complexity analysis
type LempelZivComplexity struct {
	AlphabetSize int     // Number of bins for symbolization
	WindowSize   int     // Size of sliding window for local complexity
	Overlap      float64 // Overlap fraction between windows
}

// Execute measures algorithmic complexity using Lempel-Ziv compression
func (lzc *LempelZivComplexity) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Lempel_Ziv_Complexity",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if lzc.AlphabetSize == 0 {
		lzc.AlphabetSize = 8 // 8-symbol alphabet
	}
	if lzc.WindowSize == 0 {
		lzc.WindowSize = 50 // Window size for local complexity
	}
	if lzc.Overlap == 0 {
		lzc.Overlap = 0.5 // 50% overlap
	}

	// Convert data to symbolic sequences
	xSymbols := lzc.symbolizeData(x, lzc.AlphabetSize)
	ySymbols := lzc.symbolizeData(y, lzc.AlphabetSize)

	// Compute Lempel-Ziv complexity for individual sequences
	xComplexity := lzc.lempelZivComplexity(xSymbols)
	yComplexity := lzc.lempelZivComplexity(ySymbols)

	// Compute joint complexity (combined sequence)
	jointSymbols := lzc.combineSequences(xSymbols, ySymbols)
	jointComplexity := lzc.lempelZivComplexity(jointSymbols)

	// Compute synergy/antagonism score
	expectedComplexity := (xComplexity + yComplexity) / 2
	synergyScore := jointComplexity - expectedComplexity

	// Normalize synergy score
	complexityScore := synergyScore / math.Max(xComplexity, yComplexity)
	complexityScore = math.Max(-1, math.Min(1, complexityScore)) // Clamp to [-1, 1]

	// Bootstrap for significance
	complexityScores := lzc.bootstrapComplexity(x, y, lzc.AlphabetSize, 1000)
	pValue := lzc.computeComplexityPValue(complexityScores, complexityScore)

	// Apply hardcoded standard: significant complexity structure (|score| > 0.3) with p < 0.05
	passed := math.Abs(complexityScore) > 0.3 && pValue < 0.05

	failureReason := ""
	if !passed {
		if math.Abs(complexityScore) <= 0.1 {
			failureReason = fmt.Sprintf("RANDOM/NOISY DATA: No algorithmic structure detected (|score|=%.3f). Data appears completely random or dominated by noise. Hypothesis lacks any information-theoretic foundation.", math.Abs(complexityScore))
		} else if math.Abs(complexityScore) <= 0.3 {
			failureReason = fmt.Sprintf("WEAK STRUCTURE: Some complexity detected but insufficient for reliable inference (|score|=%.3f). Data may be noisy or relationship too weak to detect computationally.", math.Abs(complexityScore))
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT COMPLEXITY CONFIDENCE: Structure detected but statistical uncertainty too high (p=%.4f). May have genuine algorithmic patterns but needs larger sample.", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Lempel_Ziv_Complexity",
		Passed:        passed,
		Statistic:     complexityScore,
		PValue:        pValue,
		StandardUsed:  "Significant complexity structure (|score| > 0.3) with p < 0.05",
		FailureReason: failureReason,
	}
}

// symbolizeData converts numeric data to symbolic sequence
func (lzc *LempelZivComplexity) symbolizeData(data []float64, alphabetSize int) []int {
	if len(data) == 0 {
		return []int{}
	}

	// Compute quantiles for symbol boundaries
	symbols := make([]int, len(data))
	quantiles := lzc.computeQuantiles(data, alphabetSize-1)

	for i, value := range data {
		// Assign symbol based on quantile
		symbol := 0
		for q := range quantiles {
			if value >= quantiles[q] {
				symbol = q + 1
			}
		}
		symbols[i] = symbol
	}

	return symbols
}

// computeQuantiles computes quantile boundaries for symbolization
func (lzc *LempelZivComplexity) computeQuantiles(data []float64, nQuantiles int) []float64 {
	if len(data) == 0 || nQuantiles <= 0 {
		return []float64{}
	}

	// Sort data for quantile computation
	sorted := make([]float64, len(data))
	copy(sorted, data)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	quantiles := make([]float64, nQuantiles)
	for i := 0; i < nQuantiles; i++ {
		pos := float64(i+1) * float64(len(sorted)-1) / float64(nQuantiles+1)
		idx := int(pos)
		if idx >= len(sorted)-1 {
			quantiles[i] = sorted[len(sorted)-1]
		} else {
			fraction := pos - float64(idx)
			quantiles[i] = sorted[idx] + fraction*(sorted[idx+1]-sorted[idx])
		}
	}

	return quantiles
}

// lempelZivComplexity computes Lempel-Ziv complexity of a symbolic sequence
func (lzc *LempelZivComplexity) lempelZivComplexity(sequence []int) float64 {
	if len(sequence) == 0 {
		return 0
	}

	// Lempel-Ziv factorization
	factors := lzc.lempelZivFactorization(sequence)
	c := float64(len(factors))

	// Normalize by sequence length and alphabet size
	n := float64(len(sequence))
	alphabetSize := float64(lzc.countUniqueSymbols(sequence))

	if alphabetSize <= 1 {
		return 0
	}

	// Normalized complexity measure
	normalized := c * math.Log2(alphabetSize) / n

	return math.Min(1.0, normalized) // Cap at 1.0
}

// lempelZivFactorization performs Lempel-Ziv factorization
func (lzc *LempelZivComplexity) lempelZivFactorization(sequence []int) []string {
	factors := []string{}
	i := 0

	for i < len(sequence) {
		// Find longest prefix that hasn't been seen before
		longestPrefix := ""
		j := i

		for j < len(sequence) {
			candidate := lzc.sequenceToString(sequence[i : j+1])

			// Check if this prefix has been seen before
			if lzc.containsPrefix(factors, candidate) {
				j++
			} else {
				longestPrefix = candidate
				break
			}
		}

		if longestPrefix == "" {
			// Last element
			longestPrefix = lzc.sequenceToString(sequence[i : i+1])
		}

		factors = append(factors, longestPrefix)
		i += len(longestPrefix)
	}

	return factors
}

// containsPrefix checks if any factor starts with the given prefix
func (lzc *LempelZivComplexity) containsPrefix(factors []string, prefix string) bool {
	for _, factor := range factors {
		if len(factor) >= len(prefix) && factor[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// sequenceToString converts int slice to string representation
func (lzc *LempelZivComplexity) sequenceToString(seq []int) string {
	result := ""
	for _, s := range seq {
		result += strconv.Itoa(s) + ","
	}
	return result
}

// countUniqueSymbols counts unique symbols in sequence
func (lzc *LempelZivComplexity) countUniqueSymbols(sequence []int) int {
	seen := make(map[int]bool)
	for _, symbol := range sequence {
		seen[symbol] = true
	}
	return len(seen)
}

// combineSequences interleaves two symbolic sequences
func (lzc *LempelZivComplexity) combineSequences(seq1, seq2 []int) []int {
	combined := []int{}
	maxLen := int(math.Max(float64(len(seq1)), float64(len(seq2))))

	for i := 0; i < maxLen; i++ {
		if i < len(seq1) {
			combined = append(combined, seq1[i])
		}
		if i < len(seq2) {
			combined = append(combined, seq2[i])
		}
	}

	return combined
}

// bootstrapComplexity performs bootstrap sampling for complexity analysis
func (lzc *LempelZivComplexity) bootstrapComplexity(x, y []float64, alphabetSize, nBootstrap int) []float64 {
	scores := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample
		xBoot, yBoot := make([]float64, n), make([]float64, n)
		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
		}

		// Compute complexity score for bootstrap sample
		xSymbols := lzc.symbolizeData(xBoot, alphabetSize)
		ySymbols := lzc.symbolizeData(yBoot, alphabetSize)
		jointSymbols := lzc.combineSequences(xSymbols, ySymbols)

		xComplexity := lzc.lempelZivComplexity(xSymbols)
		yComplexity := lzc.lempelZivComplexity(ySymbols)
		jointComplexity := lzc.lempelZivComplexity(jointSymbols)

		expectedComplexity := (xComplexity + yComplexity) / 2
		synergyScore := jointComplexity - expectedComplexity
		normalizedScore := synergyScore / math.Max(xComplexity, yComplexity)
		scores[i] = math.Max(-1, math.Min(1, normalizedScore))
	}

	return scores
}

// computeComplexityPValue computes p-value from bootstrap distribution
func (lzc *LempelZivComplexity) computeComplexityPValue(bootstrapScores []float64, observedScore float64) float64 {
	count := 0
	for _, score := range bootstrapScores {
		if math.Abs(score) >= math.Abs(observedScore) {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapScores))
}

// AlgorithmicComplexity implements broader algorithmic complexity analysis
type AlgorithmicComplexity struct {
	CompressionMethods []string // Different compression algorithms to try
	WindowSize         int      // Size of sliding window for local complexity
	TemporalResolution int      // Number of time windows to analyze
}

// Execute performs comprehensive algorithmic complexity analysis
func (ac *AlgorithmicComplexity) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Algorithmic_Complexity",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if ac.CompressionMethods == nil {
		ac.CompressionMethods = []string{"lz77", "run_length", "delta"}
	}
	if ac.WindowSize == 0 {
		ac.WindowSize = 30
	}
	if ac.TemporalResolution == 0 {
		ac.TemporalResolution = 10
	}

	// Create sliding windows for temporal complexity analysis
	windows := ac.createComplexityWindows(x, y, ac.WindowSize, ac.TemporalResolution)

	// Compute complexity for each window using multiple methods
	windowComplexities := make([][]float64, len(windows))
	for i, window := range windows {
		windowComplexities[i] = ac.computeWindowComplexities(window, ac.CompressionMethods)
	}

	// Analyze complexity stability across time
	stabilityScore := ac.analyzeComplexityStability(windowComplexities)

	// Bootstrap for significance
	stabilityScores := ac.bootstrapComplexityStability(x, y, ac.WindowSize, ac.TemporalResolution, ac.CompressionMethods, 100)
	pValue := ac.computeStabilityPValue(stabilityScores, stabilityScore)

	// Apply hardcoded standard: stable complexity structure (score > 0.6) with p < 0.05
	passed := stabilityScore > 0.6 && pValue < 0.05

	failureReason := ""
	if !passed {
		if stabilityScore <= 0.3 {
			failureReason = fmt.Sprintf("HIGHLY UNSTABLE COMPLEXITY: Algorithmic patterns change dramatically across time/conditions (stability=%.3f). Data structure is context-dependent or chaotic. Findings may not generalize.", stabilityScore)
		} else if stabilityScore <= 0.6 {
			failureReason = fmt.Sprintf("MODERATE COMPLEXITY VARIABILITY: Algorithmic structure shows some instability (stability=%.3f). Information patterns may vary across different data segments or conditions.", stabilityScore)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT STABILITY CONFIDENCE: Complexity pattern detected but statistical uncertainty too high (p=%.4f). May be truly stable but requires larger sample.", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Algorithmic_Complexity",
		Passed:        passed,
		Statistic:     stabilityScore,
		PValue:        pValue,
		StandardUsed:  "Stable complexity structure (score > 0.6) with p < 0.05",
		FailureReason: failureReason,
	}
}

// createComplexityWindows creates sliding windows for complexity analysis
func (ac *AlgorithmicComplexity) createComplexityWindows(x, y []float64, windowSize, numWindows int) [][][]float64 {
	windows := [][][]float64{}
	totalPoints := len(x)
	windowStep := totalPoints / numWindows

	for i := 0; i < numWindows; i++ {
		start := i * windowStep
		end := start + windowSize
		if end > totalPoints {
			end = totalPoints
		}
		if start >= end {
			break
		}

		window := make([][]float64, end-start)
		for j := start; j < end; j++ {
			window[j-start] = []float64{x[j], y[j]}
		}
		windows = append(windows, window)
	}

	return windows
}

// computeWindowComplexities computes multiple complexity measures for a window
func (ac *AlgorithmicComplexity) computeWindowComplexities(window [][]float64, methods []string) []float64 {
	complexities := make([]float64, len(methods))

	// Flatten window data for sequence analysis
	sequence := []float64{}
	for _, point := range window {
		sequence = append(sequence, point...)
	}

	for i, method := range methods {
		switch method {
		case "lz77":
			complexities[i] = ac.lz77Complexity(sequence)
		case "run_length":
			complexities[i] = ac.runLengthComplexity(sequence)
		case "delta":
			complexities[i] = ac.deltaComplexity(sequence)
		default:
			complexities[i] = 0.5 // Default moderate complexity
		}
	}

	return complexities
}

// lz77Complexity computes LZ77-style compression complexity
func (ac *AlgorithmicComplexity) lz77Complexity(sequence []float64) float64 {
	if len(sequence) < 3 {
		return 0
	}

	compressed := ac.lz77Compress(sequence)
	return float64(len(compressed)) / float64(len(sequence))
}

// lz77Compress performs simplified LZ77 compression
func (ac *AlgorithmicComplexity) lz77Compress(sequence []float64) []float64 {
	compressed := []float64{}
	i := 0

	for i < len(sequence) {
		// Look for longest match in previous data
		longestMatch := 1
		matchStart := i

		for j := 0; j < i; j++ {
			matchLen := 0
			for k := 0; k < 10 && i+k < len(sequence) && j+k < i; k++ { // Max match length 10
				if sequence[j+k] == sequence[i+k] {
					matchLen++
				} else {
					break
				}
			}
			if matchLen > longestMatch {
				longestMatch = matchLen
				matchStart = j
			}
		}

		if longestMatch > 1 {
			// Encode as (distance, length) - use negative values to indicate compression
			distance := i - matchStart
			compressed = append(compressed, -float64(distance), -float64(longestMatch))
			i += longestMatch
		} else {
			// Literal value
			compressed = append(compressed, sequence[i])
			i++
		}
	}

	return compressed
}

// runLengthComplexity computes run-length encoding complexity
func (ac *AlgorithmicComplexity) runLengthComplexity(sequence []float64) float64 {
	if len(sequence) == 0 {
		return 0
	}

	runs := 1
	currentValue := sequence[0]

	for i := 1; i < len(sequence); i++ {
		if sequence[i] != currentValue {
			runs++
			currentValue = sequence[i]
		}
	}

	// Complexity is inverse of compression ratio
	compressionRatio := float64(runs*2) / float64(len(sequence)) // 2 values per run (value + count)
	return math.Min(1.0, compressionRatio)
}

// deltaComplexity computes complexity based on delta encoding
func (ac *AlgorithmicComplexity) deltaComplexity(sequence []float64) float64 {
	if len(sequence) < 2 {
		return 0
	}

	deltas := make([]float64, len(sequence)-1)
	for i := 1; i < len(sequence); i++ {
		deltas[i-1] = sequence[i] - sequence[i-1]
	}

	// Complexity based on variance of deltas
	mean := 0.0
	for _, delta := range deltas {
		mean += delta
	}
	mean /= float64(len(deltas))

	variance := 0.0
	for _, delta := range deltas {
		diff := delta - mean
		variance += diff * diff
	}
	variance /= float64(len(deltas))

	// Normalize by data range
	dataRange := ac.dataRange(sequence)
	if dataRange == 0 {
		return 0.5
	}

	normalizedVariance := variance / (dataRange * dataRange)
	return math.Min(1.0, math.Sqrt(normalizedVariance))
}

// dataRange computes the range of values in data
func (ac *AlgorithmicComplexity) dataRange(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	minVal, maxVal := data[0], data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	return maxVal - minVal
}

// analyzeComplexityStability measures consistency of complexity measures across windows
func (ac *AlgorithmicComplexity) analyzeComplexityStability(windowComplexities [][]float64) float64 {
	if len(windowComplexities) <= 1 {
		return 1.0
	}

	// Average complexity across methods for each window
	windowAverages := make([]float64, len(windowComplexities))
	for i, complexities := range windowComplexities {
		sum := 0.0
		for _, complexity := range complexities {
			sum += complexity
		}
		windowAverages[i] = sum / float64(len(complexities))
	}

	// Compute coefficient of variation
	mean := 0.0
	for _, avg := range windowAverages {
		mean += avg
	}
	mean /= float64(len(windowAverages))

	variance := 0.0
	for _, avg := range windowAverages {
		diff := avg - mean
		variance += diff * diff
	}
	variance /= float64(len(windowAverages) - 1)

	cv := math.Sqrt(variance) / math.Abs(mean)

	// Convert to stability score
	return math.Max(0, 1-cv*3) // Scale so CV=0.33 gives score=0
}

// bootstrapComplexityStability bootstraps complexity stability analysis
func (ac *AlgorithmicComplexity) bootstrapComplexityStability(x, y []float64, windowSize, numWindows int, methods []string, nBootstrap int) []float64 {
	scores := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample
		xBoot, yBoot := make([]float64, n), make([]float64, n)
		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
		}

		// Compute stability score for bootstrap sample
		windows := ac.createComplexityWindows(xBoot, yBoot, windowSize, numWindows)
		windowComplexities := make([][]float64, len(windows))
		for j, window := range windows {
			windowComplexities[j] = ac.computeWindowComplexities(window, methods)
		}
		scores[i] = ac.analyzeComplexityStability(windowComplexities)
	}

	return scores
}

// computeStabilityPValue computes p-value for stability analysis
func (ac *AlgorithmicComplexity) computeStabilityPValue(bootstrapScores []float64, observedScore float64) float64 {
	count := 0
	for _, score := range bootstrapScores {
		if score >= observedScore {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapScores))
}
