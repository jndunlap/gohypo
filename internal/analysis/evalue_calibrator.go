package analysis

import (
	"context"
	"math"
	"sort"
	"time"

	"gohypo/domain/stats"
)

// EValueCalibrator provides calibrated E-value conversion and evidence aggregation
type EValueCalibrator struct {
	// Configuration parameters
	oneTailedThreshold float64
	twoTailedThreshold float64
	permutationScale   float64
	correlationMatrix  map[string]map[string]float64

	// Historical calibration data
	historicalData []CalibrationDatum

	// Dynamic thresholds
	riskMultipliers map[string]float64 // Domain-specific risk adjustments
}

// CalibrationDatum stores historical performance data
type CalibrationDatum struct {
	TestType        string
	PValue          float64
	EValue          float64
	QValue          float64 // FDR-corrected value
	TruePositive    bool
	TestReliability float64
	DataDomain      string
	SampleSize      int
}

// NewEValueCalibrator creates a calibrated E-value service
func NewEValueCalibrator() *EValueCalibrator {
	return &EValueCalibrator{
		oneTailedThreshold: 0.5,
		twoTailedThreshold: 0.25,
		permutationScale:   1.0,
		correlationMatrix:  initializeCorrelationMatrix(),
		historicalData:     loadHistoricalCalibrationData(),
		riskMultipliers:    initializeRiskMultipliers(),
	}
}

// ConvertQValueToEValue converts FDR-corrected q-values to E-values with domain calibration
func (ec *EValueCalibrator) ConvertQValueToEValue(qValue, pValue float64, testType string, dataDomain string, sampleSize int) stats.EValue {
	// Use q-value as the primary input for multiple testing correction
	var eValue float64

	// Apply domain-specific risk adjustment
	riskMultiplier := ec.getRiskMultiplier(dataDomain)

	// Convert based on test type with q-value awareness
	switch testType {
	case "permutation_shredder", "shredder":
		eValue = ec.convertPermutationQValue(qValue, pValue)
	case "correlation_pearson", "correlation_spearman":
		eValue = ec.convertCorrelationQValue(qValue, pValue)
	case "chisquare_test":
		eValue = ec.convertChiSquareQValue(qValue, pValue)
	case "ttest_two_sample":
		eValue = ec.convertTTestQValue(qValue, pValue)
	default:
		eValue = ec.convertGeneralQValue(qValue, pValue)
	}

	// Apply domain risk adjustment
	eValue *= riskMultiplier

	// Apply historical calibration
	calibratedE := ec.applyHistoricalCalibration(eValue, testType, dataDomain)

	// Calculate confidence bounds
	lower, upper := ec.calculateConfidenceBounds(calibratedE, testType, sampleSize)

	confidence := ec.calculateConfidence(calibratedE, lower, upper)

	return stats.EValue{
		Value:        calibratedE,
		Normalized:   ec.NormalizeEValueTo01(calibratedE),
		Confidence:   confidence,
		LowerBound:   lower,
		UpperBound:   upper,
		TestType:     testType,
		CalculatedAt: time.Now(),
	}
}

// convertPermutationQValue handles permutation test conversion using q-values
func (ec *EValueCalibrator) convertPermutationQValue(qValue, pValue float64) float64 {
	if qValue <= 0 {
		return 1000.0 // Very strong evidence
	}
	if qValue >= 1 {
		return 0.001 // Very weak evidence
	}

	// For permutation tests: E = 1/q, but more conservative than p-value conversion
	rawE := 1.0 / qValue

	// Apply FDR awareness - q-values are already corrected, so be more conservative
	if qValue < 0.01 {
		return math.Min(rawE*0.8, 1000.0) // Slightly conservative for strong signals
	}

	return math.Min(rawE*ec.permutationScale, 1000.0)
}

// convertCorrelationQValue handles correlation test conversion
func (ec *EValueCalibrator) convertCorrelationQValue(qValue, pValue float64) float64 {
	if qValue <= 0 {
		return 1000.0
	}
	if qValue >= 1 {
		return 0.001
	}

	// Use q-value for primary conversion, p-value for nuance
	rawE := 1.0 / qValue

	// If q-value is much smaller than p-value, this suggests strong correction was needed
	// indicating this might be a false positive from multiple testing
	correctionRatio := pValue / qValue
	if correctionRatio > 10 {
		rawE *= 0.7 // Apply conservative penalty
	}

	return rawE
}

// convertChiSquareQValue handles chi-square test conversion
func (ec *EValueCalibrator) convertChiSquareQValue(qValue, pValue float64) float64 {
	if qValue <= 0 {
		return 1000.0
	}
	if qValue >= 1 {
		return 0.001
	}
	return 1.0 / qValue
}

// convertTTestQValue handles t-test conversion
func (ec *EValueCalibrator) convertTTestQValue(qValue, pValue float64) float64 {
	if qValue <= 0 {
		return 1000.0
	}
	if qValue >= 1 {
		return 0.001
	}

	rawE := 1.0 / qValue

	// For two-tailed tests, be slightly more conservative
	if pValue < qValue*2 {
		rawE *= 0.9
	}

	return rawE
}

// convertGeneralQValue handles general q-value conversion
func (ec *EValueCalibrator) convertGeneralQValue(qValue, pValue float64) float64 {
	if qValue <= 0 {
		return 1000.0
	}
	if qValue >= 1 {
		return 0.001
	}

	rawE := 1.0 / qValue

	// Apply general FDR awareness
	if qValue < 0.05 && pValue > 0.01 {
		// This was heavily corrected, suggesting multiple testing concern
		rawE *= 0.8
	}

	return rawE
}

// AggregateEValueEvidence combines multiple E-values with correlation awareness
func (ec *EValueCalibrator) AggregateEValueEvidence(ctx context.Context, eValues []stats.EValue, hypothesisProfile stats.HypothesisProfile) stats.EvidenceCombination {
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
		weight := ec.calculateAttenuationWeight(correlation, hypothesisProfile)

		combined *= math.Pow(eValues[i].Value, weight)
		correlationFactor *= (1.0 - correlation)
	}

	// Apply profile-based adjustments
	combined = ec.applyProfileAdjustments(combined, hypothesisProfile)

	// Calculate combined confidence
	confidence := ec.calculateCombinedConfidence(eValues, hypothesisProfile)

	// Apply test count normalization
	normalizedE := ec.normalizeForTestCount(combined, len(eValues), hypothesisProfile)

	// Determine verdict with dynamic thresholding
	verdict := ec.determineVerdict(normalizedE, len(eValues), confidence, hypothesisProfile)

	return stats.EvidenceCombination{
		CombinedEValue:    normalizedE,
		NormalizedEValue:  ec.NormalizeEValueTo01(normalizedE),
		QualityRating:     ec.RateHypothesisQuality(ec.NormalizeEValueTo01(normalizedE)),
		TestCount:         len(eValues),
		CorrelationFactor: correlationFactor,
		Confidence:        confidence,
		EarlyStopEligible: ec.CheckEarlyStopEligibility(normalizedE, len(eValues), hypothesisProfile),
		Verdict:           verdict,
		IndividualResults: eValues,
	}
}

// calculateAttenuationWeight computes correlation-based attenuation
func (ec *EValueCalibrator) calculateAttenuationWeight(correlation float64, profile stats.HypothesisProfile) float64 {
	// Base attenuation: reduce weight for correlated tests
	baseWeight := math.Sqrt(1.0 - correlation)

	// Apply profile adjustments
	switch profile.DataComplexity {
	case stats.DataComplexityComplex:
		baseWeight *= 0.9 // More conservative for complex data
	case stats.DataComplexitySimple:
		baseWeight *= 1.1 // Can be more aggressive for simple data
	}

	switch profile.DomainRisk {
	case stats.DomainRiskCritical:
		baseWeight *= 0.8 // Much more conservative for critical domains
	case stats.DomainRiskHigh:
		baseWeight *= 0.9
	case stats.DomainRiskLow:
		baseWeight *= 1.1
	}

	return math.Max(0.1, math.Min(1.0, baseWeight)) // Clamp to reasonable range
}

// applyProfileAdjustments applies domain and data-specific adjustments
func (ec *EValueCalibrator) applyProfileAdjustments(combined float64, profile stats.HypothesisProfile) float64 {
	adjusted := combined

	// Sample size adjustment
	switch profile.SampleSize {
	case stats.SampleSizeSmall:
		adjusted *= 0.7 // Much more conservative
	case stats.SampleSizeLarge:
		adjusted *= 1.2 // Can be more confident
	}

	// Effect size adjustment
	switch profile.EffectMagnitude {
	case stats.EffectSizeLarge:
		adjusted *= 1.3 // Stronger evidence for large effects
	case stats.EffectSizeSmall:
		adjusted *= 0.8 // More conservative for small effects
	}

	// Confounding adjustment
	switch profile.ConfoundingRisk {
	case stats.ConfoundingHigh:
		adjusted *= 0.6 // Much more conservative
	case stats.ConfoundingLow:
		adjusted *= 1.1
	}

	return adjusted
}

// GetDynamicThreshold returns context-aware threshold
func (ec *EValueCalibrator) GetDynamicThreshold(testCount int, confidence float64, profile stats.HypothesisProfile) float64 {
	baseThreshold := ec.getExpectedBaselineForTestCount(testCount, profile)

	// Confidence multiplier
	confidenceMultiplier := 1.0
	if confidence >= 0.9 {
		confidenceMultiplier = 0.7
	} else if confidence >= 0.7 {
		confidenceMultiplier = 0.8
	} else if confidence < 0.5 {
		confidenceMultiplier = 1.3
	}

	// Domain risk multiplier
	domainMultiplier := 1.0
	switch profile.DomainRisk {
	case stats.DomainRiskCritical:
		domainMultiplier = 1.5
	case stats.DomainRiskHigh:
		domainMultiplier = 1.3
	case stats.DomainRiskLow:
		domainMultiplier = 0.8
	}

	return baseThreshold * confidenceMultiplier * domainMultiplier
}

// getExpectedBaselineForTestCount returns baseline with profile awareness
func (ec *EValueCalibrator) getExpectedBaselineForTestCount(testCount int, profile stats.HypothesisProfile) float64 {
	switch {
	case testCount <= 1:
		return 20.0
	case testCount <= 3:
		return 8.0
	case testCount <= 6:
		return 4.0
	default:
		return 2.5
	}
}

// CheckEarlyStopEligibility determines if hypothesis can be decided early
func (ec *EValueCalibrator) CheckEarlyStopEligibility(normalizedE float64, currentTestCount int, profile stats.HypothesisProfile) bool {
	// Very strong evidence allows early acceptance
	if normalizedE >= 15.0 {
		return true
	}

	// Very weak evidence allows early rejection
	if normalizedE <= 0.15 {
		return true
	}

	// Minimum tests required
	minTests := 2
	if profile.DomainRisk == stats.DomainRiskCritical {
		minTests = 3 // Require more tests for critical domains
	}

	return currentTestCount >= minTests
}

// Helper methods
func (ec *EValueCalibrator) getCorrelationFactor(testType1, testType2 string) float64 {
	if matrix, exists := ec.correlationMatrix[testType1]; exists {
		if corr, exists := matrix[testType2]; exists {
			return corr
		}
	}
	return 0.1 // Default low correlation
}

func (ec *EValueCalibrator) getRiskMultiplier(domain string) float64 {
	if multiplier, exists := ec.riskMultipliers[domain]; exists {
		return multiplier
	}
	return 1.0 // Default
}

func (ec *EValueCalibrator) applyHistoricalCalibration(rawE float64, testType, dataDomain string) float64 {
	data := ec.getHistoricalDataForTestTypeAndDomain(testType, dataDomain)

	if len(data) < 5 {
		return rawE // Not enough data for calibration
	}

	// Calculate calibration factor based on historical performance
	totalWeight := 0.0
	weightedAdjustment := 0.0

	for _, datum := range data {
		weight := datum.TestReliability
		totalWeight += weight

		// Calculate expected vs actual E-value ratio
		if datum.QValue > 0 {
			expectedRatio := datum.EValue / (1.0 / datum.QValue) // Theoretical vs actual
			weightedAdjustment += expectedRatio * weight
		}
	}

	if totalWeight > 0 {
		avgAdjustment := weightedAdjustment / totalWeight
		return rawE * avgAdjustment
	}

	return rawE
}

func (ec *EValueCalibrator) calculateConfidenceBounds(eValue float64, testType string, sampleSize int) (float64, float64) {
	historicalData := ec.getHistoricalDataForTestTypeAndDomain(testType, "Business")

	if len(historicalData) < 5 {
		// Conservative bounds based on sample size
		width := 0.5
		if sampleSize > 1000 {
			width = 0.3
		} else if sampleSize < 100 {
			width = 0.8
		}
		return eValue * (1 - width), eValue * (1 + width)
	}

	// Bootstrap confidence interval
	bootstrapSamples := 1000
	var bootstrapValues []float64

	for i := 0; i < bootstrapSamples; i++ {
		sample := ec.resampleHistoricalData(historicalData)
		bootstrapE := ec.calculateBootstrapEValue(sample, eValue)
		bootstrapValues = append(bootstrapValues, bootstrapE)
	}

	sort.Float64s(bootstrapValues)
	lowerIndex := int(0.025 * float64(len(bootstrapValues)))
	upperIndex := int(0.975 * float64(len(bootstrapValues)))

	return bootstrapValues[lowerIndex], bootstrapValues[upperIndex]
}

func (ec *EValueCalibrator) calculateConfidence(eValue, lower, upper float64) float64 {
	if upper <= lower || eValue <= 0 {
		return 0.0
	}

	relativeWidth := (upper - lower) / eValue
	return math.Max(0.0, math.Min(1.0, 1.0-relativeWidth))
}

func (ec *EValueCalibrator) calculateCombinedConfidence(eValues []stats.EValue, profile stats.HypothesisProfile) float64 {
	if len(eValues) == 0 {
		return 0.0
	}

	totalConfidence := 0.0
	for _, e := range eValues {
		totalConfidence += e.Confidence
	}

	avgConfidence := totalConfidence / float64(len(eValues))

	// Apply profile-based confidence adjustments
	switch profile.SampleSize {
	case stats.SampleSizeSmall:
		avgConfidence *= 0.8
	case stats.SampleSizeLarge:
		avgConfidence *= 1.1
	}

	switch profile.DataComplexity {
	case stats.DataComplexityComplex:
		avgConfidence *= 0.9
	case stats.DataComplexitySimple:
		avgConfidence *= 1.05
	}

	return math.Max(0.0, math.Min(1.0, avgConfidence))
}

func (ec *EValueCalibrator) normalizeForTestCount(combinedE float64, testCount int, profile stats.HypothesisProfile) float64 {
	expectedBaseline := ec.getExpectedBaselineForTestCount(testCount, profile)
	return combinedE / expectedBaseline
}

func (ec *EValueCalibrator) determineVerdict(normalizedE float64, testCount int, confidence float64, profile stats.HypothesisProfile) stats.HypothesisVerdict {
	threshold := ec.GetDynamicThreshold(testCount, confidence, profile)

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

// Data management methods
func (ec *EValueCalibrator) getHistoricalDataForTestTypeAndDomain(testType, dataDomain string) []CalibrationDatum {
	var filtered []CalibrationDatum
	for _, datum := range ec.historicalData {
		if datum.TestType == testType && datum.DataDomain == dataDomain {
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
	if len(sample) == 0 {
		return originalE
	}

	var values []float64
	for _, datum := range sample {
		values = append(values, datum.EValue)
	}
	sort.Float64s(values)
	return values[len(values)/2] // Median
}

// Initialize correlation matrix
func initializeCorrelationMatrix() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"permutation_shredder": {
			"shredder":             0.9,
			"bootstrap_validation": 0.7,
			"correlation_pearson":  0.2,
		},
		"correlation_pearson": {
			"correlation_spearman": 0.8,
			"transfer_entropy":     0.4,
		},
		"transfer_entropy": {
			"directional_causality": 0.9,
			"ccm":                   0.6,
		},
	}
}

// Initialize risk multipliers for different domains
func initializeRiskMultipliers() map[string]float64 {
	return map[string]float64{
		"Healthcare":  0.7, // More conservative
		"Finance":     0.8,
		"Scientific":  0.9,
		"Business":    1.0, // Baseline
		"Sports":      1.1, // Can be more lenient
		"Engineering": 0.95,
	}
}

// Load historical calibration data
func loadHistoricalCalibrationData() []CalibrationDatum {
	return []CalibrationDatum{
		{"permutation_shredder", 0.001, 100.0, 0.001, true, 0.85, "Business", 500},
		{"correlation_pearson", 0.05, 20.0, 0.08, true, 0.78, "Healthcare", 200},
		{"chisquare_test", 0.01, 100.0, 0.02, true, 0.82, "Finance", 1000},
		{"transfer_entropy", 0.02, 50.0, 0.03, true, 0.75, "Scientific", 300},
		{"permutation_shredder", 0.5, 2.0, 0.6, false, 0.85, "Business", 100},
		{"correlation_pearson", 0.8, 1.25, 0.9, false, 0.78, "Sports", 50},
	}
}

// NormalizeEValueTo01 converts E-values to 0-1 scale for UX
func (ec *EValueCalibrator) NormalizeEValueTo01(rawEValue float64) float64 {
	if rawEValue <= 0 {
		return 0.0
	}

	// Use sigmoid transformation: 1 / (1 + e^(-log(E)))
	// This maps E-values to 0-1 scale where:
	// E=0.1 (weak) → ~0.3
	// E=1.0 (no evidence) → 0.5
	// E=10 (strong) → ~0.7
	// E=100+ (very strong) → ~0.9+

	logE := math.Log(rawEValue)
	normalized := 1.0 / (1.0 + math.Exp(-logE))

	return math.Max(0.0, math.Min(1.0, normalized))
}

// RateHypothesisQuality converts normalized E-value to quality rating
func (ec *EValueCalibrator) RateHypothesisQuality(normalizedE float64) stats.HypothesisQuality {
	switch {
	case normalizedE >= 0.8:
		return stats.QualityVeryStrong
	case normalizedE >= 0.6:
		return stats.QualityStrong
	case normalizedE >= 0.4:
		return stats.QualityModerate
	case normalizedE >= 0.2:
		return stats.QualityWeak
	default:
		return stats.QualityVeryWeak
	}
}
