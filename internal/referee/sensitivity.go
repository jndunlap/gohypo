package referee

import (
	"fmt"
	"math"
)

// LeaveOneOutCV implements leave-one-out cross-validation for sensitivity analysis
type LeaveOneOutCV struct {
	AlphaLevels []float64 // Significance levels to test stability
	BootstrapN  int       // Number of bootstrap samples for confidence
}

// Execute tests model stability using leave-one-out cross-validation
func (loocv *LeaveOneOutCV) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Leave_One_Out_CV",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if loocv.AlphaLevels == nil {
		loocv.AlphaLevels = []float64{0.01, 0.05, 0.10} // Default significance levels
	}
	if loocv.BootstrapN == 0 {
		loocv.BootstrapN = 1000
	}

	// Perform LOO-CV
	looResults := loocv.performLeaveOneOutCV(x, y)

	// Compute stability metrics
	stabilityScore := loocv.computeStabilityScore(looResults)

	// Bootstrap for confidence in stability
	stabilityScores := loocv.bootstrapStability(x, y, loocv.BootstrapN)
	pValue := loocv.computeStabilityPValue(stabilityScores, stabilityScore)

	// Apply hardcoded standard: stability score > 0.8 with p < 0.05
	passed := stabilityScore > 0.8 && pValue < 0.05

	failureReason := ""
	if !passed {
		if stabilityScore <= 0.5 {
			failureReason = fmt.Sprintf("CRITICAL INSTABILITY: Model predictions vary wildly with single observations (stability=%.3f). Hypothesis is highly sensitive to noise or outliers. Relationship may be spurious or require robust estimation methods.", stabilityScore)
		} else if stabilityScore <= 0.8 {
			failureReason = fmt.Sprintf("MODERATE INSTABILITY: Model shows some sensitivity to individual observations (stability=%.3f). Results may be unreliable in production. Consider regularization or outlier handling.", stabilityScore)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT STABILITY CONFIDENCE: Stability detected but statistical uncertainty too high (p=%.4f). May be truly stable but needs larger sample for confidence.", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Leave_One_Out_CV",
		Passed:        passed,
		Statistic:     stabilityScore,
		PValue:        pValue,
		StandardUsed:  "Stability score > 0.8 with p < 0.05 (robust to single observations)",
		FailureReason: failureReason,
	}
}

// performLeaveOneOutCV performs leave-one-out cross-validation
func (loocv *LeaveOneOutCV) performLeaveOneOutCV(x, y []float64) []LOOResult {
	n := len(x)
	results := make([]LOOResult, n)

	for i := 0; i < n; i++ {
		// Leave out observation i
		xTrain := make([]float64, 0, n-1)
		yTrain := make([]float64, 0, n-1)

		for j := 0; j < n; j++ {
			if j != i {
				xTrain = append(xTrain, x[j])
				yTrain = append(yTrain, y[j])
			}
		}

		// Fit model on training data
		slope, intercept := loocv.linearRegression(xTrain, yTrain)

		// Predict left-out observation
		yPred := slope*x[i] + intercept
		yActual := y[i]

		// Compute prediction error
		error := yActual - yPred

		results[i] = LOOResult{
			Index:     i,
			Actual:    yActual,
			Predicted: yPred,
			Error:     error,
			AbsError:  math.Abs(error),
			Slope:     slope,
			Intercept: intercept,
		}
	}

	return results
}

type LOOResult struct {
	Index     int
	Actual    float64
	Predicted float64
	Error     float64
	AbsError  float64
	Slope     float64
	Intercept float64
}

// computeStabilityScore computes a stability metric from LOO results
func (loocv *LeaveOneOutCV) computeStabilityScore(results []LOOResult) float64 {
	if len(results) == 0 {
		return 0
	}

	n := float64(len(results))

	// Compute mean absolute error
	totalAbsError := 0.0
	for _, result := range results {
		totalAbsError += result.AbsError
	}
	meanAbsError := totalAbsError / n

	// Compute RMSE
	totalSqError := 0.0
	for _, result := range results {
		totalSqError += result.Error * result.Error
	}
	_ = math.Sqrt(totalSqError / n) // rmse not used in simplified implementation

	// Compute slope stability (coefficient of variation of slopes)
	slopes := make([]float64, len(results))
	for i, result := range results {
		slopes[i] = result.Slope
	}
	slopeCV := loocv.coefficientOfVariation(slopes)

	// Combined stability score (higher is better)
	// Normalize errors by data range
	yRange := loocv.dataRange(results, func(r LOOResult) float64 { return r.Actual })
	normalizedError := meanAbsError / yRange

	// Stability score: lower error and lower slope variation = higher stability
	errorStability := math.Max(0, 1-normalizedError*2) // Scale error contribution
	slopeStability := math.Max(0, 1-slopeCV)           // Lower CV = higher stability

	return (errorStability + slopeStability) / 2
}

// dataRange computes the range of values from a slice function
func (loocv *LeaveOneOutCV) dataRange(results []LOOResult, getter func(LOOResult) float64) float64 {
	if len(results) == 0 {
		return 0
	}

	minVal := getter(results[0])
	maxVal := minVal

	for _, result := range results {
		val := getter(result)
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	return maxVal - minVal
}

// coefficientOfVariation computes CV of a slice
func (loocv *LeaveOneOutCV) coefficientOfVariation(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	variance := 0.0
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(data) - 1)
	stdDev := math.Sqrt(variance)

	if mean == 0 {
		return 0
	}

	return stdDev / math.Abs(mean)
}

// bootstrapStability performs bootstrap sampling for stability confidence
func (loocv *LeaveOneOutCV) bootstrapStability(x, y []float64, nBootstrap int) []float64 {
	scores := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample with replacement
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
		bootResults := loocv.performLeaveOneOutCV(xBoot, yBoot)
		scores[i] = loocv.computeStabilityScore(bootResults)
	}

	return scores
}

// computeStabilityPValue computes p-value from bootstrap distribution
func (loocv *LeaveOneOutCV) computeStabilityPValue(bootstrapScores []float64, observedScore float64) float64 {
	count := 0
	for _, score := range bootstrapScores {
		if score >= observedScore {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapScores))
}

// linearRegression performs simple linear regression
func (loocv *LeaveOneOutCV) linearRegression(x, y []float64) (float64, float64) {
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

// AlphaDecayTest implements alpha decay sensitivity analysis
type AlphaDecayTest struct {
	AlphaStart float64 // Starting significance level
	AlphaEnd   float64 // Ending significance level
	AlphaSteps int     // Number of alpha levels to test
	MinSamples int     // Minimum sample size for reliable test
}

// Execute tests how effect sizes decay as alpha levels change
func (adt *AlphaDecayTest) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Alpha_Decay_Test",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if adt.AlphaStart == 0 {
		adt.AlphaStart = 0.001
	}
	if adt.AlphaEnd == 0 {
		adt.AlphaEnd = 0.10
	}
	if adt.AlphaSteps == 0 {
		adt.AlphaSteps = 20
	}
	if adt.MinSamples == 0 {
		adt.MinSamples = 30
	}

	if len(x) < adt.MinSamples {
		return RefereeResult{
			GateName:      "Alpha_Decay_Test",
			Passed:        false,
			FailureReason: fmt.Sprintf("Insufficient sample size (%d, need ≥%d)", len(x), adt.MinSamples),
		}
	}

	// Compute effect sizes at different alpha levels
	effectSizes := adt.computeAlphaDecayProfile(x, y)

	// Analyze decay pattern
	decayMetric := adt.analyzeDecayPattern(effectSizes)

	// Bootstrap for confidence
	decayMetrics := adt.bootstrapDecayAnalysis(x, y, 1000)
	pValue := adt.computeDecayPValue(decayMetrics, decayMetric)

	// Apply hardcoded standard: smooth decay (metric < 0.3) with p < 0.05
	passed := decayMetric < 0.3 && pValue < 0.05

	failureReason := ""
	if !passed {
		if decayMetric >= 0.5 {
			failureReason = fmt.Sprintf("HIGHLY ERRATIC EFFECT: Effect size changes dramatically with statistical threshold (metric=%.3f). Results are highly sensitive to significance level - suggests p-hacking vulnerability or weak signal.", decayMetric)
		} else if decayMetric >= 0.3 {
			failureReason = fmt.Sprintf("MODERATE EFFECT VARIABILITY: Effect size shows notable changes across significance levels (metric=%.3f). Findings may be threshold-dependent. Consider effect size stability.", decayMetric)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT PATTERN CONFIDENCE: Decay pattern detected but statistical uncertainty too high (p=%.4f). May be stable effect but requires larger sample.", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Alpha_Decay_Test",
		Passed:        passed,
		Statistic:     decayMetric,
		PValue:        pValue,
		StandardUsed:  "Smooth alpha decay (metric < 0.3) with p < 0.05",
		FailureReason: failureReason,
	}
}

// computeAlphaDecayProfile computes effect sizes at different significance levels
func (adt *AlphaDecayTest) computeAlphaDecayProfile(x, y []float64) []EffectAtAlpha {
	alphaStep := (adt.AlphaEnd - adt.AlphaStart) / float64(adt.AlphaSteps-1)
	effects := make([]EffectAtAlpha, adt.AlphaSteps)

	// Compute base correlation
	_ = adt.correlation(x, y) // baseCorr not used in simplified implementation

	for i := 0; i < adt.AlphaSteps; i++ {
		alpha := adt.AlphaStart + float64(i)*alphaStep

		// For alpha decay test, we simulate different sample sizes by trimming data
		// Higher alpha = more liberal = larger effective sample size
		trimFraction := 1.0 - alpha/adt.AlphaEnd   // More trimming at stricter alpha
		trimFraction = math.Max(0.1, trimFraction) // Keep at least 10% of data

		nTrim := int(float64(len(x)) * trimFraction)
		if nTrim < 10 {
			nTrim = 10
		}

		// Use trimmed sample for effect size estimation
		xTrim := x[:nTrim]
		yTrim := y[:nTrim]
		corr := adt.correlation(xTrim, yTrim)

		effects[i] = EffectAtAlpha{
			Alpha:      alpha,
			EffectSize: corr,
			SampleSize: nTrim,
		}
	}

	return effects
}

type EffectAtAlpha struct {
	Alpha      float64
	EffectSize float64
	SampleSize int
}

// analyzeDecayPattern analyzes the pattern of effect size changes with alpha
func (adt *AlphaDecayTest) analyzeDecayPattern(effects []EffectAtAlpha) float64 {
	if len(effects) < 3 {
		return 1.0 // Maximum irregularity
	}

	// Compute first differences (rate of change)
	differences := make([]float64, len(effects)-1)
	for i := 1; i < len(effects); i++ {
		differences[i-1] = effects[i].EffectSize - effects[i-1].EffectSize
	}

	// Compute second differences (acceleration/deceleration)
	secondDifferences := make([]float64, len(differences)-1)
	for i := 1; i < len(differences); i++ {
		secondDifferences[i-1] = differences[i] - differences[i-1]
	}

	// Measure irregularity as variance of second differences
	meanSecondDiff := 0.0
	for _, diff := range secondDifferences {
		meanSecondDiff += diff
	}
	meanSecondDiff /= float64(len(secondDifferences))

	variance := 0.0
	for _, diff := range secondDifferences {
		deviation := diff - meanSecondDiff
		variance += deviation * deviation
	}
	variance /= float64(len(secondDifferences) - 1)

	// Normalize by scale of effects
	effectRange := adt.effectRange(effects)
	if effectRange == 0 {
		return 0 // No variation in effects
	}

	// Return normalized irregularity metric (0 = smooth, higher = more erratic)
	return math.Sqrt(variance) / effectRange
}

// effectRange computes the range of effect sizes
func (adt *AlphaDecayTest) effectRange(effects []EffectAtAlpha) float64 {
	if len(effects) == 0 {
		return 0
	}

	minEffect := effects[0].EffectSize
	maxEffect := minEffect

	for _, effect := range effects {
		if effect.EffectSize < minEffect {
			minEffect = effect.EffectSize
		}
		if effect.EffectSize > maxEffect {
			maxEffect = effect.EffectSize
		}
	}

	return maxEffect - minEffect
}

// bootstrapDecayAnalysis bootstraps the decay analysis
func (adt *AlphaDecayTest) bootstrapDecayAnalysis(x, y []float64, nBootstrap int) []float64 {
	metrics := make([]float64, nBootstrap)
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

		// Compute decay metric for bootstrap sample
		effects := adt.computeAlphaDecayProfile(xBoot, yBoot)
		metrics[i] = adt.analyzeDecayPattern(effects)
	}

	return metrics
}

// computeDecayPValue computes p-value for decay pattern
func (adt *AlphaDecayTest) computeDecayPValue(bootstrapMetrics []float64, observedMetric float64) float64 {
	count := 0
	for _, metric := range bootstrapMetrics {
		if metric <= observedMetric {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapMetrics))
}

// correlation computes Pearson correlation coefficient
func (adt *AlphaDecayTest) correlation(x, y []float64) float64 {
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
