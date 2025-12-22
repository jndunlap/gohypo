package referee

import (
	"gohypo/domain/stats"
	"math"
	"sort"
	"time"
)

// EValueCalibrator handles all E-value operations and conversions
type EValueCalibrator struct {
	// Calibration parameters
	oneTailedThreshold float64
	twoTailedThreshold float64
	permutationScale   float64
	correlationMatrix  map[string]map[string]float64
	historicalData     []CalibrationDatum
}

// CalibrationDatum stores historical performance data for calibration
type CalibrationDatum struct {
	TestType        string
	PValue          float64
	EValue          float64
	TruePositive    bool
	TestReliability float64
}

// NewEValueCalibrator creates a calibrated E-value service
func NewEValueCalibrator() *EValueCalibrator {
	return &EValueCalibrator{
		oneTailedThreshold: 0.5,
		twoTailedThreshold: 0.25,
		permutationScale:   1.0,
		correlationMatrix:  initializeCorrelationMatrix(),
		historicalData:     loadHistoricalCalibrationData(),
	}
}

// ConvertPValueToEValue converts p-values to E-values with proper calibration
func (ec *EValueCalibrator) ConvertPValueToEValue(pValue float64, testType string, isTwoTailed bool) stats.EValue {
	var eValue float64

	switch testType {
	case "permutation_shredder", "shredder":
		eValue = ec.convertPermutationPValue(pValue)
	case "correlation_pearson", "correlation_spearman":
		eValue = ec.convertCorrelationPValue(pValue, isTwoTailed)
	case "chisquare_test":
		eValue = ec.convertChiSquarePValue(pValue)
	case "ttest_two_sample":
		eValue = ec.convertTTestPValue(pValue, isTwoTailed)
	default:
		eValue = ec.convertGeneralPValue(pValue, isTwoTailed)
	}

	// Apply historical calibration adjustment
	calibratedE := ec.applyHistoricalCalibration(eValue, testType)

	// Calculate confidence bounds using bootstrap
	lower, upper := ec.calculateConfidenceBounds(calibratedE, testType)

	confidence := ec.calculateConfidence(calibratedE, lower, upper)

	return stats.EValue{
		Value:        calibratedE,
		Confidence:   confidence,
		LowerBound:   lower,
		UpperBound:   upper,
		TestType:     testType,
		CalculatedAt: time.Now(),
	}
}

// convertPermutationPValue handles permutation test specific conversion
func (ec *EValueCalibrator) convertPermutationPValue(pValue float64) float64 {
	if pValue <= 0 {
		return 1000.0 // Very strong evidence
	}
	if pValue >= 1 {
		return 0.001 // Very weak evidence
	}

	// For permutation tests: E = 1/p, but with scaling
	rawE := 1.0 / pValue
	return math.Min(rawE*ec.permutationScale, 1000.0)
}

// convertCorrelationPValue handles correlation test conversion
func (ec *EValueCalibrator) convertCorrelationPValue(pValue float64, isTwoTailed bool) float64 {
	if isTwoTailed {
		// For two-tailed correlation tests
		if pValue > ec.twoTailedThreshold {
			return 1.0 / (2.0 * pValue) // Conservative conversion
		}
		return 1.0 / pValue
	}
	return 1.0 / pValue
}

// convertChiSquarePValue handles chi-square test conversion
func (ec *EValueCalibrator) convertChiSquarePValue(pValue float64) float64 {
	if pValue <= 0 {
		return 1000.0
	}
	if pValue >= 1 {
		return 0.001
	}
	return 1.0 / pValue
}

// convertTTestPValue handles t-test conversion
func (ec *EValueCalibrator) convertTTestPValue(pValue float64, isTwoTailed bool) float64 {
	if isTwoTailed {
		if pValue > ec.twoTailedThreshold {
			return 1.0 / (2.0 * pValue)
		}
	}
	return 1.0 / pValue
}

// convertGeneralPValue handles general p-value conversion
func (ec *EValueCalibrator) convertGeneralPValue(pValue float64, isTwoTailed bool) float64 {
	if pValue <= 0 {
		return 1000.0
	}
	if pValue >= 1 {
		return 0.001
	}

	if isTwoTailed && pValue > ec.twoTailedThreshold {
		return 1.0 / (2.0 * pValue)
	}
	return 1.0 / pValue
}

// CombineEvidence combines multiple E-values with correlation handling
func (ec *EValueCalibrator) CombineEvidence(eValues []stats.EValue, testCount int) stats.EvidenceCombination {
	if len(eValues) == 0 {
		return stats.EvidenceCombination{
			CombinedEValue: 1.0,
			Verdict:        stats.VerdictInconclusive,
		}
	}

	// Start with first E-value
	combined := eValues[0].Value
	correlationFactor := 1.0

	// Combine subsequent E-values with correlation adjustment
	for i := 1; i < len(eValues); i++ {
		correlation := ec.getCorrelationFactor(eValues[i-1].TestType, eValues[i].TestType)
		weight := math.Sqrt(1.0 - correlation) // Attenuation factor

		combined *= math.Pow(eValues[i].Value, weight)
		correlationFactor *= (1.0 - correlation)
	}

	// Calculate overall confidence
	confidence := ec.calculateCombinedConfidence(eValues)

	// Apply test count normalization
	normalizedE := ec.normalizeForTestCount(combined, testCount)

	// Determine verdict with early stopping
	verdict := ec.determineVerdict(normalizedE, testCount, confidence)

	return stats.EvidenceCombination{
		CombinedEValue:    normalizedE,
		TestCount:         len(eValues),
		CorrelationFactor: correlationFactor,
		Confidence:        confidence,
		EarlyStopEligible: ec.CheckEarlyStopEligibility(normalizedE, len(eValues)),
		Verdict:           verdict,
		IndividualResults: eValues,
	}
}

// getCorrelationFactor returns correlation between test types
func (ec *EValueCalibrator) getCorrelationFactor(testType1, testType2 string) float64 {
	if matrix, exists := ec.correlationMatrix[testType1]; exists {
		if corr, exists := matrix[testType2]; exists {
			return corr
		}
	}
	return 0.1 // Default low correlation
}

// normalizeForTestCount adjusts evidence based on test count expectations
func (ec *EValueCalibrator) normalizeForTestCount(combinedE float64, testCount int) float64 {
	expectedBaseline := ec.getExpectedBaselineForTestCount(testCount)
	return combinedE / expectedBaseline
}

// getExpectedBaselineForTestCount returns expected E-value for random tests
func (ec *EValueCalibrator) getExpectedBaselineForTestCount(testCount int) float64 {
	switch {
	case testCount <= 1:
		return 20.0 // Very high bar for single tests
	case testCount <= 3:
		return 8.0 // Moderate bar for e-value validation
	case testCount <= 6:
		return 4.0 // Lower bar for comprehensive testing
	default:
		return 2.5 // Reasonable bar for extensive testing
	}
}

// determineVerdict makes the final decision with dynamic thresholding
func (ec *EValueCalibrator) determineVerdict(normalizedE float64, testCount int, confidence float64) stats.HypothesisVerdict {
	threshold := ec.GetDynamicThreshold(testCount, confidence)

	switch {
	case normalizedE >= threshold*2.0:
		return stats.VerdictAccepted
	case normalizedE <= 1.0/threshold:
		return stats.VerdictRejected
	case confidence >= 0.8 && normalizedE >= threshold*0.5:
		return stats.VerdictEarlyStop
	default:
		return stats.VerdictInconclusive
	}
}

// GetDynamicThreshold returns appropriate threshold based on test count and confidence
func (ec *EValueCalibrator) GetDynamicThreshold(testCount int, confidence float64) float64 {
	baseThreshold := ec.getExpectedBaselineForTestCount(testCount)

	// Confidence multiplier: higher confidence allows lower thresholds
	confidenceMultiplier := 1.0
	if confidence >= 0.9 {
		confidenceMultiplier = 0.8
	} else if confidence >= 0.7 {
		confidenceMultiplier = 0.9
	} else if confidence < 0.5 {
		confidenceMultiplier = 1.2
	}

	return baseThreshold * confidenceMultiplier
}

// CheckEarlyStopEligibility determines if hypothesis can be decided early
func (ec *EValueCalibrator) CheckEarlyStopEligibility(normalizedE float64, currentTestCount int) bool {
	// Can accept early if evidence is very strong
	if normalizedE >= 15.0 {
		return true
	}

	// Can reject early if evidence is very weak
	if normalizedE <= 0.15 {
		return true
	}

	// Check if we've reached minimum test count for early decision
	minTestsForEarlyStop := 2
	return currentTestCount >= minTestsForEarlyStop
}

// applyHistoricalCalibration adjusts E-values based on past performance
func (ec *EValueCalibrator) applyHistoricalCalibration(rawE float64, testType string) float64 {
	calibrationFactor := ec.getCalibrationFactor(testType)
	return rawE * calibrationFactor
}

// getCalibrationFactor computes adjustment based on historical accuracy
func (ec *EValueCalibrator) getCalibrationFactor(testType string) float64 {
	data := ec.getHistoricalDataForTestType(testType)

	if len(data) < 10 {
		return 1.0 // No adjustment
	}

	// Calculate how well this test type predicts true outcomes
	truePositiveRate := 0.0
	count := 0

	for _, datum := range data {
		if datum.EValue >= 5.0 { // Threshold for "positive" call
			if datum.TruePositive {
				truePositiveRate += 1.0
			}
			count++
		}
	}

	if count == 0 {
		return 1.0
	}

	truePositiveRate /= float64(count)

	// Adjust calibration to improve accuracy
	if truePositiveRate > 0.8 {
		return 0.9 // Slightly conservative
	} else if truePositiveRate < 0.6 {
		return 1.1 // Slightly liberal
	}

	return 1.0
}

// calculateConfidenceBounds uses bootstrap to estimate uncertainty
func (ec *EValueCalibrator) calculateConfidenceBounds(eValue float64, testType string) (float64, float64) {
	historicalData := ec.getHistoricalDataForTestType(testType)

	if len(historicalData) < 10 {
		// Not enough data, use conservative bounds
		return eValue * 0.5, eValue * 2.0
	}

	// Bootstrap confidence interval calculation
	bootstrapSamples := 1000
	var bootstrapValues []float64

	for i := 0; i < bootstrapSamples; i++ {
		sample := ec.resampleHistoricalData(historicalData)
		bootstrapE := ec.calculateBootstrapEValue(sample, eValue)
		bootstrapValues = append(bootstrapValues, bootstrapE)
	}

	// Calculate 95% confidence interval
	sort.Float64s(bootstrapValues)
	lowerIndex := int(0.025 * float64(len(bootstrapValues)))
	upperIndex := int(0.975 * float64(len(bootstrapValues)))

	return bootstrapValues[lowerIndex], bootstrapValues[upperIndex]
}

// calculateConfidence computes confidence score from bounds
func (ec *EValueCalibrator) calculateConfidence(eValue, lower, upper float64) float64 {
	if upper <= lower || eValue <= 0 {
		return 0.0
	}

	relativeWidth := (upper - lower) / eValue
	return math.Max(0.0, math.Min(1.0, 1.0-relativeWidth))
}

// calculateCombinedConfidence computes confidence for evidence combination
func (ec *EValueCalibrator) calculateCombinedConfidence(eValues []stats.EValue) float64 {
	if len(eValues) == 0 {
		return 0.0
	}

	totalConfidence := 0.0
	for _, e := range eValues {
		totalConfidence += e.Confidence
	}

	avgConfidence := totalConfidence / float64(len(eValues))
	consistencyPenalty := ec.calculateConsistencyPenalty(eValues)

	return avgConfidence * consistencyPenalty
}

// calculateConsistencyPenalty penalizes inconsistent results
func (ec *EValueCalibrator) calculateConsistencyPenalty(eValues []stats.EValue) float64 {
	if len(eValues) <= 1 {
		return 1.0
	}

	// Measure consistency across test results
	var values []float64
	for _, e := range eValues {
		values = append(values, e.Value)
	}

	// Calculate coefficient of variation
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(values))

	if mean == 0 {
		return 0.5 // High penalty for inconsistent results
	}

	cv := math.Sqrt(variance) / mean

	// Lower penalty for more consistent results
	return math.Max(0.5, 1.0-cv)
}

// Helper methods for data management
func (ec *EValueCalibrator) getHistoricalDataForTestType(testType string) []CalibrationDatum {
	var filtered []CalibrationDatum
	for _, datum := range ec.historicalData {
		if datum.TestType == testType {
			filtered = append(filtered, datum)
		}
	}
	return filtered
}

func (ec *EValueCalibrator) resampleHistoricalData(data []CalibrationDatum) []CalibrationDatum {
	if len(data) == 0 {
		return data
	}

	result := make([]CalibrationDatum, len(data))
	for i := range result {
		randomIndex := int(math.Floor(float64(len(data)) * math.Abs(math.Sin(float64(i*31)))))
		result[i] = data[randomIndex%len(data)]
	}
	return result
}

func (ec *EValueCalibrator) calculateBootstrapEValue(sample []CalibrationDatum, originalE float64) float64 {
	// Simplified bootstrap calculation
	if len(sample) == 0 {
		return originalE
	}

	// Use median of sample as bootstrap estimate
	var values []float64
	for _, datum := range sample {
		values = append(values, datum.EValue)
	}
	sort.Float64s(values)
	return values[len(values)/2]
}

// initializeCorrelationMatrix sets up test correlation relationships
func initializeCorrelationMatrix() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"permutation_shredder": {
			"shredder":             0.9, // Same test
			"bootstrap_validation": 0.7, // Similar statistical validation
			"correlation_pearson":  0.2, // Low correlation with correlation tests
		},
		"correlation_pearson": {
			"correlation_spearman": 0.8, // Similar monotonic tests
			"transfer_entropy":     0.4, // Some directional relationship
		},
		"transfer_entropy": {
			"directional_causality": 0.9, // Same directional concept
			"ccm":                   0.6, // Related embedding methods
		},
	}
}

// loadHistoricalCalibrationData initializes with sample historical data
func loadHistoricalCalibrationData() []CalibrationDatum {
	// This would normally load from a database or file
	// For now, return sample data
	return []CalibrationDatum{
		{"permutation_shredder", 0.001, 100.0, true, 0.85},
		{"correlation_pearson", 0.05, 20.0, true, 0.78},
		{"chisquare_test", 0.01, 100.0, true, 0.82},
		{"transfer_entropy", 0.02, 50.0, true, 0.75},
		{"permutation_shredder", 0.5, 2.0, false, 0.85},
		{"correlation_pearson", 0.8, 1.25, false, 0.78},
	}
}
