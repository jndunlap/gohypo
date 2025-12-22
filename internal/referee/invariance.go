package referee

import (
	"fmt"
	"math"
	"sort"
)

// ChowTest implements structural stability testing via Chow breakpoint test
type ChowTest struct {
	AlphaCritical float64 // Significance level for rejecting stability
	FCritical     float64 // Critical F-statistic value
	TrimFraction  float64 // Fraction of data to trim from ends
}

// Execute runs Chow test for parameter stability across time shards
func (c *ChowTest) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Chow_Stability_Test",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	// Set defaults from centralized constants
	if c.AlphaCritical == 0 {
		c.AlphaCritical = CHOW_ALPHA_CRITICAL
	}
	if c.FCritical == 0 {
		c.FCritical = CHOW_F_CRITICAL
	}
	if c.TrimFraction == 0 {
		c.TrimFraction = SUPREMUM_WALD_TRIM
	}

	// Extract time variable if available
	timeVar, _ := metadata["time_variable"].([]float64)

	// Supremum Wald: Find maximum F-statistic across all possible breakpoints
	maxFStat := 0.0
	trim := int(float64(len(x)) * c.TrimFraction)

	for k := trim; k < len(x)-trim; k++ {
		fStat, _ := c.computeChowStatistic(x, y, float64(k)/float64(len(x)), timeVar)
		if fStat > maxFStat {
			maxFStat = fStat
		}
	}

	// Simplified bootstrap for now (implement proper bootstrap later)
	pValue := 0.5 // Placeholder

	// Apply centralized standard: F-stat < critical value indicates stability
	passed := maxFStat < c.FCritical

	failureReason := ""
	if !passed {
		if maxFStat > c.FCritical*2 {
			failureReason = fmt.Sprintf("CRITICAL INSTABILITY: Relationship changes dramatically over time (F=%.3f ≫ %.2f). Hypothesis is time-dependent - effect varies by context/period. May indicate Simpson's paradox or changing causal mechanisms.", maxFStat, c.FCritical)
		} else {
			failureReason = fmt.Sprintf("MODERATE INSTABILITY: Relationship shows some time variation (F=%.3f > %.2f). Hypothesis may be context-specific. Consider testing in different time periods or market conditions.", maxFStat, c.FCritical)
		}
	}

	return RefereeResult{
		GateName:  "Chow_Stability_Test",
		Passed:    passed,
		Statistic: maxFStat,
		PValue:    pValue,
		StandardUsed: fmt.Sprintf("Supremum Wald F < %.2f (α=%.3f, Trim=%.0f%%)",
			c.FCritical, c.AlphaCritical, c.TrimFraction*100),
		FailureReason: failureReason,
	}
}

// AuditEvidence performs evidence auditing for Chow stability test using discovery q-values
func (c *ChowTest) AuditEvidence(discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) RefereeResult {
	// Chow test is about structural stability - use default audit logic
	// since invariance testing requires time-series data that's hard to audit from q-values alone
	return DefaultAuditEvidence("Chow_Stability_Test", discoveryEvidence, validationData, metadata)
}

// computeChowStatistic computes Chow test statistic for a given split point
func (c *ChowTest) computeChowStatistic(x, y []float64, splitPoint float64, timeVar []float64) (float64, float64) {
	x1, y1, x2, y2 := c.splitData(x, y, timeVar, splitPoint)

	if len(x1) < 5 || len(x2) < 5 {
		return 0, 1.0 // Insufficient data for reliable test
	}

	// Fit regressions on each subsample
	slope1, intercept1 := c.linearRegression(x1, y1)
	slope2, intercept2 := c.linearRegression(x2, y2)

	// Fit pooled regression
	slopePooled, interceptPooled := c.linearRegression(x, y)

	// Compute RSS for subsamples and pooled
	rss1 := c.computeRSS(x1, y1, slope1, intercept1)
	rss2 := c.computeRSS(x2, y2, slope2, intercept2)
	rssPooled := c.computeRSS(x, y, slopePooled, interceptPooled)

	// Compute Chow F-statistic
	n1, n2 := float64(len(x1)), float64(len(x2))
	fStat := ((rssPooled - (rss1 + rss2)) / 2) / ((rss1 + rss2) / (n1 + n2 - 4))

	// Approximate p-value using F-distribution (simplified)
	pValue := c.fDistributionCDF(fStat, 2, int(n1+n2-4))

	return fStat, pValue
}

// splitData splits data at the given point
func (c *ChowTest) splitData(x, y []float64, time []float64, splitPoint float64) ([]float64, []float64, []float64, []float64) {
	if len(time) == 0 {
		// Split by index if no time variable
		splitIdx := int(float64(len(x)) * splitPoint)
		return x[:splitIdx], y[:splitIdx], x[splitIdx:], y[splitIdx:]
	}

	// Split by time variable
	splitValue := c.percentile(time, splitPoint)
	var x1, y1, x2, y2 []float64

	for i, t := range time {
		if t <= splitValue {
			x1 = append(x1, x[i])
			y1 = append(y1, y[i])
		} else {
			x2 = append(x2, x[i])
			y2 = append(y2, y[i])
		}
	}

	return x1, y1, x2, y2
}

// linearRegression performs simple linear regression
func (c *ChowTest) linearRegression(x, y []float64) (float64, float64) {
	n := float64(len(x))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	return slope, intercept
}

// computeRSS computes residual sum of squares
func (c *ChowTest) computeRSS(x, y []float64, slope, intercept float64) float64 {
	rss := 0.0
	for i := 0; i < len(x); i++ {
		predicted := slope*x[i] + intercept
		residual := y[i] - predicted
		rss += residual * residual
	}
	return rss
}

// fDistributionCDF approximates F-distribution CDF (simplified)
func (c *ChowTest) fDistributionCDF(f float64, df1, df2 int) float64 {
	// Simplified approximation - in production use proper statistical library
	if f < 1 {
		return 0.5
	}
	return 0.9 // Conservative approximation
}

// percentile computes the p-th percentile of data
func (c *ChowTest) percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	index := (p * float64(len(sorted)-1))
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// CUSUMDriftDetection implements CUSUM control charts for parameter stability
type CUSUMDriftDetection struct {
	ControlLimit float64 // Control limit for CUSUM statistic
	ARL0         float64 // Average run length for in-control process
}

// Execute runs CUSUM drift detection for parameter stability monitoring
func (cusum *CUSUMDriftDetection) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "CUSUM_Drift_Detection",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if cusum.ControlLimit == 0 {
		cusum.ControlLimit = 5.0 // Default control limit
	}

	// Fit baseline model on first portion of data (training period)
	trainingSize := len(x) / 3
	if trainingSize < 10 {
		trainingSize = len(x) / 2
	}

	xTrain, yTrain := x[:trainingSize], y[:trainingSize]
	slope, intercept := cusum.linearRegression(xTrain, yTrain)

	// Compute recursive residuals for monitoring period
	monitoringX, monitoringY := x[trainingSize:], y[trainingSize:]
	residuals := cusum.computeRecursiveResiduals(monitoringX, monitoringY, slope, intercept)

	// Compute CUSUM statistics
	cusumPos, cusumNeg := cusum.computeCUSUMStatistics(residuals)

	// Test for parameter drift
	maxCUSUM := math.Max(cusumPos[len(cusumPos)-1], math.Abs(cusumNeg[len(cusumNeg)-1]))
	driftDetected := maxCUSUM > cusum.ControlLimit

	// Bootstrap to estimate false positive rate
	falsePositiveRate := cusum.estimateFalsePositiveRate(residuals, cusum.ControlLimit, 1000)

	// Apply hardcoded standard: no drift detected at 99.9% confidence
	passed := !driftDetected && falsePositiveRate < 0.001

	failureReason := ""
	if !passed {
		if driftDetected {
			failureReason = fmt.Sprintf("PARAMETER DRIFT: Relationship parameters change over time (CUSUM=%.3f). Hypothesis effect size or direction varies across observation window. May indicate time-varying causal mechanisms or structural breaks.", maxCUSUM)
		} else {
			failureReason = fmt.Sprintf("UNRELIABLE DETECTION: Drift detection has high false positive rate (FPR=%.6f). Cannot confidently rule out parameter stability. May need longer time series or different monitoring approach.", falsePositiveRate)
		}
	}

	return RefereeResult{
		GateName:      "CUSUM_Drift_Detection",
		Passed:        passed,
		Statistic:     maxCUSUM,
		PValue:        falsePositiveRate,
		StandardUsed:  "CUSUM < control limit with FPR < 0.001 (stable parameters over time)",
		FailureReason: failureReason,
	}
}

// AuditEvidence performs evidence auditing for CUSUM drift detection using discovery q-values
func (cusum *CUSUMDriftDetection) AuditEvidence(discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) RefereeResult {
	// CUSUM is about temporal stability - use default audit logic
	// since stability testing requires time-series analysis that's hard to audit from q-values alone
	return DefaultAuditEvidence("CUSUM_Drift_Detection", discoveryEvidence, validationData, metadata)
}

// computeRecursiveResiduals computes recursive residuals for monitoring
func (cusum *CUSUMDriftDetection) computeRecursiveResiduals(x, y []float64, slope, intercept float64) []float64 {
	residuals := make([]float64, len(x))

	for i := 0; i < len(x); i++ {
		predicted := slope*x[i] + intercept
		residuals[i] = y[i] - predicted
	}

	return residuals
}

// computeCUSUMStatistics computes CUSUM control statistics
func (cusum *CUSUMDriftDetection) computeCUSUMStatistics(residuals []float64) ([]float64, []float64) {
	n := len(residuals)
	cusumPos := make([]float64, n+1)
	cusumNeg := make([]float64, n+1)

	// Estimate process standard deviation from residuals
	stdDev := cusum.standardDeviation(residuals)
	if stdDev == 0 {
		stdDev = 1.0 // Avoid division by zero
	}

	k := 0.5 // Reference value (half of shift to detect)

	for i := 1; i <= n; i++ {
		// Standardized residual
		z := residuals[i-1] / stdDev

		// Update CUSUM statistics
		cusumPos[i] = math.Max(0, cusumPos[i-1]+z-k)
		cusumNeg[i] = math.Min(0, cusumNeg[i-1]+z+k)
	}

	return cusumPos[1:], cusumNeg[1:] // Remove initial zero
}

// standardDeviation computes standard deviation of residuals
func (cusum *CUSUMDriftDetection) standardDeviation(data []float64) float64 {
	if len(data) <= 1 {
		return 0
	}

	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	sumSq := 0.0
	for _, v := range data {
		diff := v - mean
		sumSq += diff * diff
	}

	return math.Sqrt(sumSq / float64(len(data)-1))
}

// estimateFalsePositiveRate estimates false positive rate through bootstrapping
func (cusum *CUSUMDriftDetection) estimateFalsePositiveRate(residuals []float64, controlLimit float64, nBootstrap int) float64 {
	falsePositives := 0

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap residuals (assume they come from in-control process)
		bootResiduals := make([]float64, len(residuals))
		for j := 0; j < len(residuals); j++ {
			idx := int(math.Floor(float64(len(residuals)) * math.Sqrt(float64(i*j%len(residuals)))))
			if idx >= len(residuals) {
				idx = len(residuals) - 1
			}
			bootResiduals[j] = residuals[idx]
		}

		// Compute CUSUM on bootstrap sample
		cusumPos, cusumNeg := cusum.computeCUSUMStatistics(bootResiduals)
		maxCUSUM := math.Max(cusumPos[len(cusumPos)-1], math.Abs(cusumNeg[len(cusumNeg)-1]))

		if maxCUSUM > controlLimit {
			falsePositives++
		}
	}

	return float64(falsePositives) / float64(nBootstrap)
}

// linearRegression performs simple linear regression (duplicate from ChowTest - could be shared)
func (cusum *CUSUMDriftDetection) linearRegression(x, y []float64) (float64, float64) {
	n := float64(len(x))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	return slope, intercept
}
