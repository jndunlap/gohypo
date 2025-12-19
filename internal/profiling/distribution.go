package profiling

import (
	"math"

	"github.com/montanaflynn/stats"
	"gonum.org/v1/gonum/stat/distuv"
)

// DistributionAnalyzer handles distribution shape analysis
type DistributionAnalyzer struct{}

// NewDistributionAnalyzer creates a new distribution analyzer
func NewDistributionAnalyzer() *DistributionAnalyzer {
	return &DistributionAnalyzer{}
}

// AnalyzeDistribution performs comprehensive distribution analysis
func (da *DistributionAnalyzer) AnalyzeDistribution(data []float64) (TopologyMarkers, error) {
	markers := TopologyMarkers{}

	// Calculate basic summary statistics
	mean, err := stats.Mean(data)
	if err != nil {
		return markers, err
	}

	stdDev, err := stats.StandardDeviation(data)
	if err != nil {
		return markers, err
	}

	min, err := stats.Min(data)
	if err != nil {
		return markers, err
	}

	max, err := stats.Max(data)
	if err != nil {
		return markers, err
	}

	median, err := stats.Median(data)
	if err != nil {
		return markers, err
	}

	// Quartiles for IQR-based outlier detection
	q25, err := stats.Percentile(data, 25)
	if err != nil {
		return markers, err
	}

	q75, err := stats.Percentile(data, 75)
	if err != nil {
		return markers, err
	}

	// Calculate skewness and kurtosis
	skewness := calculateSkewness(data, mean, stdDev)
	kurtosis := calculateKurtosis(data, mean, stdDev)

	// Normality test (Shapiro-Wilk approximation)
	isNormal, shapiroP := testNormality(data)

	// Populate distribution markers
	markers.Distribution.Skewness = skewness
	markers.Distribution.Kurtosis = kurtosis
	markers.Distribution.IsNormal = isNormal
	markers.Distribution.ShapiroP = shapiroP

	// Populate summary statistics
	markers.Summary.Mean = mean
	markers.Summary.StdDev = stdDev
	markers.Summary.Min = min
	markers.Summary.Max = max
	markers.Summary.Median = median
	markers.Summary.Q25 = q25
	markers.Summary.Q75 = q75

	return markers, nil
}

// calculateSkewness computes sample skewness using the adjusted Fisher-Pearson coefficient
func calculateSkewness(data []float64, mean, stdDev float64) float64 {
	if len(data) < 3 {
		return 0
	}

	n := float64(len(data))
	sumCubedDeviations := 0.0

	for _, x := range data {
		deviation := (x - mean) / stdDev
		sumCubedDeviations += deviation * deviation * deviation
	}

	// Adjusted Fisher-Pearson coefficient of skewness
	skewness := sumCubedDeviations / n

	// Bias correction for sample skewness
	correction := math.Sqrt(n*(n-1)) / (n - 2)
	skewness *= correction

	return skewness
}

// calculateKurtosis computes sample excess kurtosis
func calculateKurtosis(data []float64, mean, stdDev float64) float64 {
	if len(data) < 4 {
		return 0
	}

	n := float64(len(data))
	sumFourthDeviations := 0.0

	for _, x := range data {
		deviation := (x - mean) / stdDev
		sumFourthDeviations += deviation * deviation * deviation * deviation
	}

	// Sample kurtosis
	kurtosis := sumFourthDeviations / n

	// Convert to excess kurtosis (subtract 3 for normal distribution)
	excessKurtosis := kurtosis - 3

	// Bias correction for sample excess kurtosis
	if n > 3 {
		correction := (n - 1) / ((n - 2) * (n - 3))
		excessKurtosis = excessKurtosis*correction + 6/(n+1)
	}

	return excessKurtosis + 3 // Return total kurtosis (not excess)
}

// testNormality performs a simplified normality test
// This is an approximation - for production use, consider more sophisticated tests
func testNormality(data []float64) (isNormal bool, pValue float64) {
	if len(data) < 3 {
		return false, 1.0
	}

	// Get mean and standard deviation with error handling
	mean, err := stats.Mean(data)
	if err != nil {
		return false, 1.0
	}

	stdDev, err := stats.StandardDeviation(data)
	if err != nil {
		return false, 1.0
	}

	// Shapiro-Wilk test approximation using skewness and kurtosis
	// This is a simplified version - real implementation would use proper SW test
	skewness := calculateSkewness(data, mean, stdDev)
	kurtosis := calculateKurtosis(data, mean, stdDev)

	// Combined test statistic (simplified)
	testStat := math.Abs(skewness) + math.Abs(kurtosis-3)/2

	// Approximate p-value using chi-square distribution
	// This is a rough approximation - production code should use proper statistical tables
	degreesFreedom := 2.0 // Approximation for combined skewness/kurtosis test
	chiDist := distuv.ChiSquared{K: degreesFreedom}
	pValue = 1 - chiDist.CDF(testStat*testStat)

	// Conservative threshold for normality
	isNormal = pValue > 0.05

	return isNormal, pValue
}

// detectOutliers identifies outliers using IQR method
func detectOutliers(data []float64, q25, q75 float64) int {
	iqr := q75 - q25
	lowerBound := q25 - 1.5*iqr
	upperBound := q75 + 1.5*iqr

	outlierCount := 0
	for _, x := range data {
		if x < lowerBound || x > upperBound {
			outlierCount++
		}
	}

	return outlierCount
}

// calculateNoiseCoefficient estimates data noise using coefficient of variation
// and residual analysis
func calculateNoiseCoefficient(data []float64) float64 {
	if len(data) < 3 {
		return 1.0 // Maximum noise for insufficient data
	}

	// Coefficient of variation as basic noise measure
	mean, _ := stats.Mean(data)
	stdDev, _ := stats.StandardDeviation(data)

	if mean == 0 {
		return 1.0 // Avoid division by zero
	}

	cv := stdDev / math.Abs(mean)

	// Normalize to [0,1] range (higher values = more noise)
	noiseCoeff := math.Min(cv/2.0, 1.0) // Cap at 1.0

	return noiseCoeff
}


