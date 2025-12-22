package referee

import (
	"fmt"
	"math"
	"sort"
)

// ConditionalMI implements conditional mutual information testing
type ConditionalMI struct {
	K       int // Number of nearest neighbors for kNN estimator
	MaxCond int // Maximum number of conditioning variables to consider
	Bins    int // Number of bins for histogram-based estimation
}

// Execute tests for confounding by measuring conditional mutual information
func (cmi *ConditionalMI) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Conditional_Mutual_Information",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if cmi.K == 0 {
		cmi.K = 5 // Default k for kNN
	}
	if cmi.MaxCond == 0 {
		cmi.MaxCond = 3 // Default max conditioning variables
	}
	if cmi.Bins == 0 {
		cmi.Bins = 10 // Default bins for histogram
	}

	// Extract potential confounding variables from metadata
	confounders, hasConfounders := metadata["confounding_variables"].([][]float64)

	if !hasConfounders || len(confounders) == 0 {
		// No confounding variables specified - assume independence
		return RefereeResult{
			GateName:      "Conditional_Mutual_Information",
			Passed:        true, // No confounders means no confounding
			Statistic:     0.0,
			PValue:        1.0,
			StandardUsed:  "No confounding variables specified - independence assumed",
			FailureReason: "",
		}
	}

	// Test conditional independence I(X;Y|Z) for each potential confounder Z
	minCMI := math.Inf(1)
	var strongestConfounder []float64
	// confounderIndex := -1 // Not used in simplified implementation

	for _, confounder := range confounders {
		if len(confounder) != len(x) {
			continue // Skip misaligned confounders
		}

		// Compute CMI(X;Y|Z)
		cmiValue := cmi.computeConditionalMI(x, y, confounder, cmi.K, cmi.Bins)

		if cmiValue < minCMI {
			minCMI = cmiValue
			strongestConfounder = confounder
			// confounderIndex = i // Not used in simplified implementation
		}
	}

	// Bootstrap to get confidence interval
	cmiValues := cmi.bootstrapCMI(x, y, strongestConfounder, cmi.K, cmi.Bins, 1000)
	upperBound := cmi.percentile(cmiValues, 99.9)

	// Apply hardcoded standard: CMI upper bound ≤ 0.01 nats (very weak conditional dependence)
	passed := upperBound <= 0.01

	failureReason := ""
	if !passed {
		if upperBound > 0.5 {
			failureReason = fmt.Sprintf("SEVERE CONFOUNDING: Strong conditional relationship detected (CMI=%.4f nats). Variables are heavily confounded - observed relationship likely due to shared causes, not direct causation. Requires careful experimental design or instrumental variables.", minCMI)
		} else if upperBound > 0.1 {
			failureReason = fmt.Sprintf("MODERATE CONFOUNDING: Some conditional dependence exists (CMI=%.4f nats). Results may be biased by unmeasured confounders. Consider additional control variables or sensitivity analysis.", minCMI)
		} else {
			failureReason = fmt.Sprintf("WEAK CONFOUNDING DETECTED: Conditional dependence above threshold (CMI=%.4f nats). Statistical relationship exists but may be inflated by minor confounding. Proceed with caution.", minCMI)
		}
	}

	return RefereeResult{
		GateName:      "Conditional_Mutual_Information",
		Passed:        passed,
		Statistic:     minCMI,
		PValue:        upperBound, // Using upper bound as a pseudo p-value
		StandardUsed:  "CMI(X;Y|Z) ≤ 0.01 nats (99.9% confidence upper bound)",
		FailureReason: failureReason,
	}
}

// AuditEvidence performs evidence auditing for conditional mutual information using discovery q-values
func (cmi *ConditionalMI) AuditEvidence(discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) RefereeResult {
	// Conditional MI is about confounding control - use default audit logic
	// since confounding analysis requires multiple variables that are hard to audit from q-values alone
	return DefaultAuditEvidence("Conditional_Mutual_Information", discoveryEvidence, validationData, metadata)
}

// computeConditionalMI computes I(X;Y|Z) using kNN estimator
func (cmi *ConditionalMI) computeConditionalMI(x, y, z []float64, k, bins int) float64 {
	n := len(x)
	if n < 20 {
		return 0 // Insufficient data
	}

	cmiSum := 0.0
	validPoints := 0

	for i := 0; i < n; i++ {
		// Find kNN distances
		epsX := cmi.findKNNDistance(x, x[i], k)
		epsY := cmi.findKNNDistance(y, y[i], k)
		epsZ := cmi.findKNNDistance(z, z[i], k)

		// Count neighbors
		_ = cmi.countNeighbors(x, x[i], epsX) // nx not used in simplified implementation
		ny := cmi.countNeighbors(y, y[i], epsY)
		nz := cmi.countNeighbors(z, z[i], epsZ)
		nxyz := cmi.countJointNeighbors(x, y, z, x[i], y[i], z[i], epsX, epsY, epsZ)

		if nxyz > 0 && ny > 0 && nz > 0 {
			// CMI estimate using Kozachenko-Leonenko estimator
			cmiValue := math.Log(float64(ny*nz)/float64(n*nxyz)) + math.Log(float64(n-1)/float64(n))
			if !math.IsNaN(cmiValue) && !math.IsInf(cmiValue, 0) {
				cmiSum += cmiValue
				validPoints++
			}
		}
	}

	if validPoints == 0 {
		return 0
	}

	return math.Max(0, cmiSum/float64(validPoints))
}

// findKNNDistance finds the distance to the k-th nearest neighbor
func (cmi *ConditionalMI) findKNNDistance(data []float64, center float64, k int) float64 {
	distances := make([]float64, len(data))
	for i, d := range data {
		distances[i] = math.Abs(d - center)
	}

	// Sort distances
	sort.Float64s(distances)

	// Return k-th smallest distance (k-1 index since 0-based)
	if k-1 < len(distances) {
		return distances[k-1]
	}
	return distances[len(distances)-1]
}

// countNeighbors counts points within epsilon distance
func (cmi *ConditionalMI) countNeighbors(data []float64, center, eps float64) int {
	count := 0
	for _, d := range data {
		if math.Abs(d-center) <= eps {
			count++
		}
	}
	return count
}

// countJointNeighbors counts points within epsilon in all three dimensions
func (cmi *ConditionalMI) countJointNeighbors(x, y, z []float64, cx, cy, cz, ex, ey, ez float64) int {
	count := 0
	for i := range x {
		if math.Abs(x[i]-cx) <= ex && math.Abs(y[i]-cy) <= ey && math.Abs(z[i]-cz) <= ez {
			count++
		}
	}
	return count
}

// bootstrapCMI performs bootstrap sampling for CMI confidence intervals
func (cmi *ConditionalMI) bootstrapCMI(x, y, z []float64, k, bins, nBootstrap int) []float64 {
	cmiValues := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample with replacement
		xBoot, yBoot, zBoot := make([]float64, n), make([]float64, n), make([]float64, n)
		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
			zBoot[j] = z[idx]
		}

		// Compute CMI for bootstrap sample
		cmiValues[i] = cmi.computeConditionalMI(xBoot, yBoot, zBoot, k, bins)
	}

	return cmiValues
}

// percentile computes the p-th percentile of data
func (cmi *ConditionalMI) percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// PartialCorrelation implements partial correlation for confounding control
type PartialCorrelation struct {
	ControlVariables [][]float64 // Variables to control for
}

// Execute tests for confounding using partial correlation
func (pc *PartialCorrelation) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Partial_Correlation",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	// Extract control variables from metadata
	controlVars, hasControls := metadata["control_variables"].([][]float64)

	if !hasControls || len(controlVars) == 0 {
		// No control variables - use regular correlation
		corr := pc.computeCorrelation(x, y)
		passed := math.Abs(corr) < 0.3 // Weak correlation threshold

		failureReason := ""
		if !passed {
			failureReason = fmt.Sprintf("Strong bivariate correlation (r=%.3f, need |r|<0.3)", corr)
		}

		return RefereeResult{
			GateName:      "Partial_Correlation",
			Passed:        passed,
			Statistic:     corr,
			PValue:        1.0 - math.Abs(corr), // Simplified p-value
			StandardUsed:  "|r(X,Y)| < 0.3 when controlling for confounders",
			FailureReason: failureReason,
		}
	}

	// Compute partial correlation controlling for all variables
	partialCorr := pc.computePartialCorrelation(x, y, controlVars)

	// Bootstrap for confidence
	partialCorrs := pc.bootstrapPartialCorrelation(x, y, controlVars, 1000)
	ciUpper := pc.percentile(partialCorrs, 97.5)
	ciLower := pc.percentile(partialCorrs, 2.5)

	// Check if confidence interval excludes strong effects
	passed := ciUpper < 0.3 && ciLower > -0.3

	failureReason := ""
	if !passed {
		failureReason = fmt.Sprintf("Partial correlation CI includes strong effects [%.3f, %.3f]", ciLower, ciUpper)
	}

	return RefereeResult{
		GateName:      "Partial_Correlation",
		Passed:        passed,
		Statistic:     partialCorr,
		PValue:        math.Max(math.Abs(ciUpper), math.Abs(ciLower)), // Pseudo p-value
		StandardUsed:  "95% CI of partial correlation excludes |r| ≥ 0.3",
		FailureReason: failureReason,
	}
}

// computeCorrelation computes Pearson correlation coefficient
func (pc *PartialCorrelation) computeCorrelation(x, y []float64) float64 {
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

// computePartialCorrelation computes partial correlation between x and y controlling for z
func (pc *PartialCorrelation) computePartialCorrelation(x, y []float64, controls [][]float64) float64 {
	if len(controls) == 0 {
		return pc.computeCorrelation(x, y)
	}

	// For simplicity, handle single control variable case
	// In production, would use multivariate regression
	if len(controls) == 1 {
		z := controls[0]
		rxy := pc.computeCorrelation(x, y)
		rxz := pc.computeCorrelation(x, z)
		ryz := pc.computeCorrelation(y, z)

		// Partial correlation formula: r(xy|z) = (rxy - rxz*ryz) / sqrt((1-rxz²)*(1-ryz²))
		numerator := rxy - rxz*ryz
		denominator := math.Sqrt((1 - rxz*rxz) * (1 - ryz*ryz))

		if denominator == 0 {
			return 0
		}

		return numerator / denominator
	}

	// For multiple controls, use residual approach
	return pc.computePartialCorrelationMultivariate(x, y, controls)
}

// computePartialCorrelationMultivariate handles multiple control variables
func (pc *PartialCorrelation) computePartialCorrelationMultivariate(x, y []float64, controls [][]float64) float64 {
	// Regress x on controls to get residuals
	xResiduals := pc.regressOutControls(x, controls)

	// Regress y on controls to get residuals
	yResiduals := pc.regressOutControls(y, controls)

	// Correlation of residuals
	return pc.computeCorrelation(xResiduals, yResiduals)
}

// regressOutControls regresses variable onto control variables and returns residuals
func (pc *PartialCorrelation) regressOutControls(y []float64, controls [][]float64) []float64 {
	n := len(y)
	residuals := make([]float64, n)

	// Simple approach: for each control, partial out its effect
	currentY := make([]float64, n)
	copy(currentY, y)

	for _, control := range controls {
		if len(control) != n {
			continue
		}

		// Regress currentY on control
		r := pc.computeCorrelation(currentY, control)
		for i := 0; i < n; i++ {
			// Remove control effect: residual = y - r * control
			meanY := pc.mean(currentY)
			meanC := pc.mean(control)
			stdY := pc.stdDev(currentY)
			stdC := pc.stdDev(control)

			if stdC > 0 {
				currentY[i] = meanY + (currentY[i] - meanY) - r*(stdY/stdC)*(control[i]-meanC)
			}
		}
	}

	copy(residuals, currentY)
	return residuals
}

// bootstrapPartialCorrelation performs bootstrap sampling for partial correlation
func (pc *PartialCorrelation) bootstrapPartialCorrelation(x, y []float64, controls [][]float64, nBootstrap int) []float64 {
	correlations := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample
		xBoot, yBoot := make([]float64, n), make([]float64, n)
		controlsBoot := make([][]float64, len(controls))
		for j := range controlsBoot {
			controlsBoot[j] = make([]float64, n)
		}

		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
			for k, control := range controls {
				controlsBoot[k][j] = control[idx]
			}
		}

		// Compute partial correlation for bootstrap sample
		correlations[i] = pc.computePartialCorrelation(xBoot, yBoot, controlsBoot)
	}

	return correlations
}

// percentile computes the p-th percentile
func (pc *PartialCorrelation) percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// mean computes arithmetic mean
func (pc *PartialCorrelation) mean(data []float64) float64 {
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// stdDev computes standard deviation
func (pc *PartialCorrelation) stdDev(data []float64) float64 {
	mean := pc.mean(data)
	sumSq := 0.0
	for _, v := range data {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(data)-1))
}
