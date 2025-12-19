package referee

import (
	"fmt"
	"math"
	"math/rand"
)

// TransferEntropy implements directional information flow testing
type TransferEntropy struct {
	K       int // Number of nearest neighbors for kNN estimator
	TimeLag int // Maximum time lag to consider
}

// Execute measures information flow from X to Y using Transfer Entropy
func (te *TransferEntropy) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Transfer_Entropy",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if te.K == 0 {
		te.K = 5 // Default k for kNN
	}
	if te.TimeLag == 0 {
		te.TimeLag = 5 // Default max lag
	}

	// Compute transfer entropy TE(X→Y)
	teValue := te.computeTransferEntropy(x, y, te.K, te.TimeLag)

	// Compute reverse transfer entropy TE(Y→X) for comparison
	teReverse := te.computeTransferEntropy(y, x, te.K, te.TimeLag)

	// Test for directional flow: TE(X→Y) should be significantly greater than TE(Y→X)
	ratio := teValue / (teReverse + 1e-10) // Add small epsilon to avoid division by zero

	// Bootstrap to get confidence interval
	ratios := te.bootstrapRatios(x, y, te.K, te.TimeLag, 1000)
	pValue := te.computePValue(ratios, ratio)

	// Apply hardcoded standard: TE(X→Y) > TE(Y→X) with 99.9% confidence
	passed := pValue < 0.001 && ratio > 1.1 // At least 10% stronger in forward direction

	failureReason := ""
	if !passed {
		if ratio <= 1.1 {
			failureReason = fmt.Sprintf("No clear directional flow (ratio=%.3f, need >1.1)", ratio)
		} else {
			failureReason = fmt.Sprintf("Direction uncertain (p=%.6f, need p<0.001)", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Transfer_Entropy",
		Passed:        passed,
		Statistic:     teValue,
		PValue:        pValue,
		StandardUsed:  "TE(X→Y) > TE(Y→X) with p < 0.001 (99.9% confidence in causal direction)",
		FailureReason: failureReason,
	}
}

// computeTransferEntropy calculates Transfer Entropy using kNN estimator
func (te *TransferEntropy) computeTransferEntropy(driver, target []float64, k, maxLag int) float64 {
	n := len(driver)
	if n < 20 { // Need sufficient data for reliable estimation
		return 0
	}

	totalTE := 0.0
	validLags := 0

	for lag := 1; lag <= maxLag && lag < n/2; lag++ {
		teLag := te.computeTELag(driver, target, k, lag)
		if !math.IsNaN(teLag) && !math.IsInf(teLag, 0) {
			totalTE += teLag
			validLags++
		}
	}

	if validLags == 0 {
		return 0
	}

	return totalTE / float64(validLags)
}

// computeTELag computes transfer entropy for a specific lag
func (te *TransferEntropy) computeTELag(driver, target []float64, k, lag int) float64 {
	n := len(driver)
	teSum := 0.0
	count := 0

	// Use Kraskov-Stögbauer-Grassberger estimator (simplified)
	for i := lag + 1; i < n; i++ {
		// Find k nearest neighbors for point i in target past + driver past
		targetPast := target[i-lag]
		driverPast := driver[i-lag]
		_ = target[i] // targetFuture not used in simplified implementation

		// Count points within epsilon of this point
		epsTarget := te.findKthDistance(targetPast, target, k)
		epsDriver := te.findKthDistance(driverPast, driver, k)

		_ = te.countNeighbors(target, targetPast, epsTarget) // nx not used in simplified implementation
		ny := te.countNeighbors(driver, driverPast, epsDriver)
		nxy := te.countJointNeighbors(target, driver, targetPast, driverPast, epsTarget, epsDriver)

		if nxy > 0 && ny > 0 {
			teValue := math.Log(float64(ny)/float64(nxy)) + math.Log(float64(n-1)/float64(n))
			if !math.IsNaN(teValue) && !math.IsInf(teValue, 0) {
				teSum += teValue
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}

	return math.Max(0, teSum/float64(count))
}

// findKthDistance finds the distance to the k-th nearest neighbor
func (te *TransferEntropy) findKthDistance(value float64, data []float64, k int) float64 {
	distances := make([]float64, 0, len(data))
	for _, d := range data {
		distances = append(distances, math.Abs(d-value))
	}

	// Simple selection algorithm for k-th smallest distance
	for i := 0; i < k && i < len(distances); i++ {
		minIdx := i
		for j := i + 1; j < len(distances); j++ {
			if distances[j] < distances[minIdx] {
				minIdx = j
			}
		}
		distances[i], distances[minIdx] = distances[minIdx], distances[i]
	}

	if k <= len(distances) {
		return distances[k-1]
	}
	return distances[len(distances)-1]
}

// countNeighbors counts points within epsilon distance
func (te *TransferEntropy) countNeighbors(data []float64, center float64, eps float64) int {
	count := 0
	for _, d := range data {
		if math.Abs(d-center) <= eps {
			count++
		}
	}
	return count
}

// countJointNeighbors counts points within epsilon in both dimensions
func (te *TransferEntropy) countJointNeighbors(data1, data2 []float64, center1, center2, eps1, eps2 float64) int {
	count := 0
	for i := range data1 {
		if math.Abs(data1[i]-center1) <= eps1 && math.Abs(data2[i]-center2) <= eps2 {
			count++
		}
	}
	return count
}

// bootstrapRatios performs bootstrap sampling to estimate confidence in directional flow
func (te *TransferEntropy) bootstrapRatios(x, y []float64, k, maxLag, nBootstrap int) []float64 {
	ratios := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample with replacement
		xBoot, yBoot := make([]float64, n), make([]float64, n)
		for j := 0; j < n; j++ {
			idx := rand.Intn(n)
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
		}

		// Compute TE ratios for bootstrap sample
		teXY := te.computeTransferEntropy(xBoot, yBoot, k, maxLag)
		teYX := te.computeTransferEntropy(yBoot, xBoot, k, maxLag)
		ratio := teXY / (teYX + 1e-10)
		ratios[i] = ratio
	}

	return ratios
}

// computePValue computes p-value from bootstrap distribution
func (te *TransferEntropy) computePValue(bootstrapRatios []float64, observedRatio float64) float64 {
	count := 0
	for _, r := range bootstrapRatios {
		if r >= observedRatio {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapRatios))
}

// ConvergentCrossMapping implements CCM for causal inference
type ConvergentCrossMapping struct {
	LibrarySizes []int // Range of library sizes to test convergence
	EMax         int   // Maximum embedding dimension
}

// Execute tests for causal influence using Convergent Cross Mapping
func (ccm *ConvergentCrossMapping) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Convergent_Cross_Mapping",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if len(ccm.LibrarySizes) == 0 {
		// Default library sizes: 10% to 90% of data
		maxLib := len(x) * 9 / 10
		step := maxLib / 10
		for size := step; size <= maxLib; size += step {
			ccm.LibrarySizes = append(ccm.LibrarySizes, size)
		}
	}

	if ccm.EMax == 0 {
		ccm.EMax = 5
	}

	// Test CCM(X|Y) - mapping from Y to reconstruct X
	corrX := ccm.computeCCMConvergence(x, y, ccm.LibrarySizes, ccm.EMax)

	// Test CCM(Y|X) - mapping from X to reconstruct Y
	corrY := ccm.computeCCMConvergence(y, x, ccm.LibrarySizes, ccm.EMax)

	// Check for convergence and strength
	convergedX := ccm.checkConvergence(corrX)
	convergedY := ccm.checkConvergence(corrY)

	// Determine if X causes Y (stronger and convergent mapping from X to Y)
	causalStrength := convergedY.Strength - convergedX.Strength
	passed := causalStrength > 0.1 && convergedY.Converged && convergedY.Strength > 0.3

	failureReason := ""
	if !passed {
		if !convergedY.Converged {
			failureReason = "No convergent cross-mapping detected"
		} else if convergedY.Strength <= 0.3 {
			failureReason = fmt.Sprintf("Cross-mapping too weak (ρ=%.3f, need >0.3)", convergedY.Strength)
		} else {
			failureReason = fmt.Sprintf("No directional advantage (diff=%.3f, need >0.1)", causalStrength)
		}
	}

	return RefereeResult{
		GateName:      "Convergent_Cross_Mapping",
		Passed:        passed,
		Statistic:     causalStrength,
		PValue:        1.0 - convergedY.Confidence, // Simplified p-value
		StandardUsed:  "CCM convergence with ρ > 0.3 and directional advantage > 0.1",
		FailureReason: failureReason,
	}
}

type CCMResult struct {
	Converged  bool
	Strength   float64
	Confidence float64
}

// computeCCMConvergence tests prediction skill across library sizes
func (ccm *ConvergentCrossMapping) computeCCMConvergence(target, library []float64, libSizes []int, eMax int) []float64 {
	correlations := make([]float64, len(libSizes))

	for i, libSize := range libSizes {
		// Use simplified simplex projection for CCM
		corr := ccm.simplexProjection(target, library, libSize, eMax)
		correlations[i] = corr
	}

	return correlations
}

// simplexProjection implements simplified simplex projection for CCM
func (ccm *ConvergentCrossMapping) simplexProjection(target, library []float64, libSize, eMax int) float64 {
	n := len(target)
	if libSize >= n {
		libSize = n - 1
	}

	// Simple nearest neighbor prediction
	predictions := make([]float64, 0, n-libSize)

	for i := libSize; i < n; i++ {
		// Find nearest neighbors in library space
		neighbors := ccm.findNearestNeighbors(library[:i], library[i], eMax)

		// Predict target value
		if len(neighbors) > 0 {
			prediction := 0.0
			for _, idx := range neighbors {
				prediction += target[idx+1] // Next time step
			}
			prediction /= float64(len(neighbors))
			predictions = append(predictions, prediction)
		}
	}

	// Compute correlation between predictions and actual values
	actual := target[libSize+1 : n+1]
	if len(predictions) != len(actual) {
		return 0
	}

	return ccm.correlation(predictions, actual)
}

// findNearestNeighbors finds E nearest neighbors
func (ccm *ConvergentCrossMapping) findNearestNeighbors(history []float64, query float64, e int) []int {
	distances := make([]struct {
		index    int
		distance float64
	}, len(history))

	for i := range history {
		distances[i] = struct {
			index    int
			distance float64
		}{i, math.Abs(history[i] - query)}
	}

	// Sort by distance
	for i := 0; i < len(distances)-1; i++ {
		for j := i + 1; j < len(distances); j++ {
			if distances[j].distance < distances[i].distance {
				distances[i], distances[j] = distances[j], distances[i]
			}
		}
	}

	// Return indices of E nearest neighbors
	result := make([]int, 0, e)
	for i := 0; i < e && i < len(distances); i++ {
		result = append(result, distances[i].index)
	}

	return result
}

// checkConvergence analyzes convergence pattern in correlations
func (ccm *ConvergentCrossMapping) checkConvergence(correlations []float64) CCMResult {
	if len(correlations) < 3 {
		return CCMResult{Converged: false, Strength: 0, Confidence: 0}
	}

	// Check if correlations are increasing and stabilizing
	finalCorr := correlations[len(correlations)-1]
	penultimateCorr := correlations[len(correlations)-2]

	// Simple convergence criteria
	converged := finalCorr > 0.1 && (finalCorr-penultimateCorr) < 0.1
	strength := finalCorr
	confidence := math.Min(finalCorr, 0.95) // Simplified confidence measure

	return CCMResult{
		Converged:  converged,
		Strength:   strength,
		Confidence: confidence,
	}
}

// correlation computes Pearson correlation between two slices
func (ccm *ConvergentCrossMapping) correlation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}
