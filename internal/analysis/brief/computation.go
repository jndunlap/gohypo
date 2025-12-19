package brief

import (
	"fmt"
	"math"
	"sort"

	"gohypo/domain/stats/brief"

	"github.com/montanaflynn/stats"
	"gonum.org/v1/gonum/stat/distuv"
)

// StatisticalBriefComputer consolidates all statistical analysis into unified briefs
type StatisticalBriefComputer struct{}

// NewComputer creates a new brief computer
func NewComputer() *StatisticalBriefComputer {
	return &StatisticalBriefComputer{}
}

// ComputeBrief performs comprehensive statistical analysis based on the computation request
func (c *StatisticalBriefComputer) ComputeBrief(data []float64, fieldKey, source string, request brief.ComputationRequest) (*brief.StatisticalBrief, error) {
	if len(data) == 0 {
		return nil, NewComputationError("empty dataset", nil)
	}

	// Create the brief with context
	b := brief.NewBrief(fieldKey, source, len(data), request)

	// Always compute core statistics
	if err := c.computeCoreStats(data, b); err != nil {
		return nil, err
	}

	// Conditionally compute extended analysis based on use case
	if request.ForIngestion || request.ForValidation || request.ForHypothesis {
		if err := c.computeExtendedStats(data, b, request); err != nil {
			return nil, err
		}
	}

	// Compute validation-specific stats for referee rules
	if request.ForValidation {
		c.computeValidationStats(b, request)
	}

	return b, nil
}

// computeCoreStats computes always-needed statistics (summary, distribution, quality)
func (c *StatisticalBriefComputer) computeCoreStats(data []float64, b *brief.StatisticalBrief) error {
	// Summary statistics - merging from multiple systems
	mean, _ := stats.Mean(data)
	stdDev, _ := stats.StandardDeviation(data)
	min, _ := stats.Min(data)
	max, _ := stats.Max(data)
	median, _ := stats.Median(data)
	q25, _ := stats.Percentile(data, 25)
	q75, _ := stats.Percentile(data, 75)

	b.Summary = brief.SummaryStats{
		Mean:   mean,
		StdDev: stdDev,
		Min:    min,
		Max:    max,
		Median: median,
		Q25:    q25,
		Q75:    q75,
	}

	// Distribution analysis - from internal/profiling
	skewness := c.calculateSkewness(data, mean, stdDev)
	kurtosis := c.calculateKurtosis(data, mean, stdDev)
	isNormal, shapiroP := c.testNormality(data)

	b.Distribution = brief.DistributionStats{
		Skewness: skewness,
		Kurtosis: kurtosis,
		IsNormal: isNormal,
		ShapiroP: shapiroP,
	}

	// Quality analysis - merging from all systems
	missingRatio := c.calculateMissingRatio(data)
	sparsityRatio := c.calculateSparsityRatio(data)
	noiseCoeff := c.calculateNoiseCoefficient(data, stdDev, mean)
	outlierCount := c.detectOutliers(data, q25, q75)

	b.Quality = brief.QualityStats{
		MissingRatio:     missingRatio,
		SparsityRatio:    sparsityRatio,
		NoiseCoefficient: noiseCoeff,
		OutlierCount:     outlierCount,
	}

	return nil
}

// computeExtendedStats computes conditional analysis based on use case
func (c *StatisticalBriefComputer) computeExtendedStats(data []float64, b *brief.StatisticalBrief, request brief.ComputationRequest) error {
	// Categorical analysis - always useful for data understanding
	if c.shouldComputeCategorical(data) {
		categorical := c.computeCategoricalStats(data)
		b.Categorical = &categorical
	}

	// Temporal analysis - for time series or causal inference
	if request.ForValidation || request.ForHypothesis {
		temporal := c.computeTemporalStats(data)
		b.Temporal = &temporal
	}

	return nil
}

// computeValidationStats generates referee configuration recommendations
func (c *StatisticalBriefComputer) computeValidationStats(b *brief.StatisticalBrief, request brief.ComputationRequest) {
	validation := brief.ValidationStats{}

	// Adaptive significance level based on data characteristics
	baseAlpha := 0.001 // Hard floor
	if !b.Distribution.IsNormal {
		baseAlpha = math.Max(baseAlpha, 0.005)
	}
	if b.Quality.SparsityRatio > 0.5 {
		baseAlpha = math.Max(baseAlpha, 0.01)
	}
	if b.Quality.NoiseCoefficient > 0.7 {
		baseAlpha = math.Max(baseAlpha, 0.01)
	}

	// Adaptive permutation count
	iterations := 2500
	if b.Quality.NoiseCoefficient > 0.7 {
		iterations = 10000
	} else if b.Quality.SparsityRatio > 0.5 {
		iterations = 5000
	}

	// Use nonparametric tests for non-normal data
	useNonParametric := !b.Distribution.IsNormal
	if b.Categorical != nil && b.Categorical.IsCategorical {
		useNonParametric = true
	}

	// Bootstrap requirements
	requiresBootstrap := false
	bootstrapSamples := 0

	if b.Temporal != nil && b.Temporal.AutocorrLag1 > 0.3 {
		requiresBootstrap = true
		bootstrapSamples = 2000
	}

	validation.RecommendedAlpha = baseAlpha
	validation.OptimalIterations = iterations
	validation.UseNonParametric = useNonParametric
	validation.RequiresBootstrap = requiresBootstrap
	validation.BootstrapSamples = bootstrapSamples

	b.Validation = &validation
}

// Statistical computation methods (consolidated from all systems)

// Distribution analysis
func (c *StatisticalBriefComputer) calculateSkewness(data []float64, mean, stdDev float64) float64 {
	if len(data) < 3 || stdDev == 0 {
		return 0
	}

	n := float64(len(data))
	sumCubedDeviations := 0.0

	for _, x := range data {
		deviation := (x - mean) / stdDev
		sumCubedDeviations += deviation * deviation * deviation
	}

	// Adjusted Fisher-Pearson coefficient
	skewness := sumCubedDeviations / n
	correction := math.Sqrt(n*(n-1)) / (n - 2)
	skewness *= correction

	return skewness
}

func (c *StatisticalBriefComputer) calculateKurtosis(data []float64, mean, stdDev float64) float64 {
	if len(data) < 4 || stdDev == 0 {
		return 3.0 // Normal kurtosis
	}

	n := float64(len(data))
	sumFourthDeviations := 0.0

	for _, x := range data {
		deviation := (x - mean) / stdDev
		sumFourthDeviations += deviation * deviation * deviation * deviation
	}

	// Sample kurtosis
	kurtosis := sumFourthDeviations / n

	// Bias correction for sample excess kurtosis
	if n > 3 {
		correction := (n - 1) / ((n - 2) * (n - 3))
		kurtosis = kurtosis*correction + 6/(n+1)
	}

	return kurtosis + 3 // Return total kurtosis (not excess)
}

func (c *StatisticalBriefComputer) testNormality(data []float64) (isNormal bool, pValue float64) {
	if len(data) < 3 {
		return false, 1.0
	}

	// D'Agostino's K^2 normality test (more robust than JB for moderate samples).
	// For very small samples, fall back to a conservative JB-like approximation.
	if len(data) >= 8 {
		return c.dagostinoK2Normality(data)
	}

	// Fallback: JB-like approximation using skewness & kurtosis.
	mean, _ := stats.Mean(data)
	stdDev, _ := stats.StandardDeviation(data)
	skewness := c.calculateSkewness(data, mean, stdDev)
	kurtosis := c.calculateKurtosis(data, mean, stdDev) - 3 // Excess kurtosis

	// Combined test statistic
	testStat := math.Abs(skewness) + math.Abs(kurtosis)/2

	// Approximate p-value using chi-square distribution
	degreesFreedom := 2.0
	chiDist := distuv.ChiSquared{K: degreesFreedom}
	pValue = 1 - chiDist.CDF(testStat*testStat)

	// Conservative threshold
	isNormal = pValue > 0.05

	return isNormal, pValue
}

func (c *StatisticalBriefComputer) dagostinoK2Normality(data []float64) (isNormal bool, pValue float64) {
	n := float64(len(data))

	mean, _ := stats.Mean(data)
	stdDev, _ := stats.StandardDeviation(data)
	if stdDev == 0 || math.IsNaN(stdDev) || math.IsInf(stdDev, 0) {
		return false, 1.0
	}

	g1 := c.calculateSkewness(data, mean, stdDev)
	g2excess := c.calculateKurtosis(data, mean, stdDev) - 3

	// ---- Skewness transform to Z1 (D'Agostino) ----
	y := g1 * math.Sqrt((n+1)*(n+3)/(6*(n-2)))
	beta2 := (3 * (n*n + 27*n - 70) * (n + 1) * (n + 3)) / ((n - 2) * (n + 5) * (n + 7) * (n + 9))
	w2 := -1 + math.Sqrt(2*(beta2-1))
	if w2 <= 0 {
		return false, 1.0
	}
	delta := 1 / math.Sqrt(math.Log(math.Sqrt(w2)))
	alpha := math.Sqrt(2 / (w2 - 1))
	ay := y / alpha
	z1 := delta * math.Log(ay+math.Sqrt(ay*ay+1))

	// ---- Kurtosis transform to Z2 (Anscombe–Glynn, used in K^2) ----
	// Here g2 is total kurtosis, E and Var are for total kurtosis.
	g2 := g2excess + 3
	e := 3 * (n - 1) / (n + 1)
	v := 24 * n * (n - 2) * (n - 3) / ((n + 1) * (n + 1) * (n + 3) * (n + 5))
	if v <= 0 {
		return false, 1.0
	}
	x := (g2 - e) / math.Sqrt(v)

	sqrtBeta1 := 6 * (n*n - 5*n + 2) / ((n + 7) * (n + 9)) * math.Sqrt(6*(n+3)*(n+5)/(n*(n-2)*(n-3)))
	a := 6 + 8/sqrtBeta1*(2/sqrtBeta1+math.Sqrt(1+4/(sqrtBeta1*sqrtBeta1)))
	if a <= 4 {
		return false, 1.0
	}

	term := 1 - 2/(9*a)
	den := 1 + x*math.Sqrt(2/(a-4))
	if den <= 0 {
		// Avoid invalid fractional power if den <= 0; treat as non-normal.
		return false, 0.0
	}
	z2 := (term - math.Pow((1-2/a)/den, 1.0/3.0)) / math.Sqrt(2/(9*a))

	k2 := z1*z1 + z2*z2
	chi2 := distuv.ChiSquared{K: 2}
	pValue = 1 - chi2.CDF(k2)
	isNormal = pValue > 0.05
	return isNormal, pValue
}

// Quality analysis
func (c *StatisticalBriefComputer) calculateMissingRatio(data []float64) float64 {
	missingCount := 0
	for _, val := range data {
		if math.IsNaN(val) || math.IsInf(val, 0) {
			missingCount++
		}
	}
	return float64(missingCount) / float64(len(data))
}

func (c *StatisticalBriefComputer) calculateSparsityRatio(data []float64) float64 {
	zeroCount := 0
	for _, val := range data {
		if val == 0 {
			zeroCount++
		}
	}
	return float64(zeroCount) / float64(len(data))
}

func (c *StatisticalBriefComputer) calculateNoiseCoefficient(data []float64, stdDev, mean float64) float64 {
	if mean == 0 {
		return 1.0 // Maximum noise for zero mean
	}

	// Coefficient of variation as noise measure
	cv := stdDev / math.Abs(mean)

	// Normalize to [0,1] range
	noiseCoeff := math.Min(cv/2.0, 1.0)

	return noiseCoeff
}

func (c *StatisticalBriefComputer) detectOutliers(data []float64, q25, q75 float64) int {
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

// Categorical analysis
func (c *StatisticalBriefComputer) shouldComputeCategorical(data []float64) bool {
	// Check if data has reasonable cardinality for categorical analysis
	uniqueValues := make(map[float64]bool)
	for _, val := range data {
		uniqueValues[val] = true
	}

	uniqueCount := len(uniqueValues)
	totalCount := len(data)

	// Consider categorical if: < 50 unique values OR < 30% unique ratio
	return uniqueCount < 50 || float64(uniqueCount)/float64(totalCount) < 0.3
}

func (c *StatisticalBriefComputer) computeCategoricalStats(data []float64) brief.CategoricalStats {
	// Count unique values
	uniqueValues := make(map[float64]bool)
	frequency := make(map[float64]int)

	for _, val := range data {
		uniqueValues[val] = true
		frequency[val]++
	}

	uniqueCount := len(uniqueValues)
	totalCount := len(data)

	// Calculate entropy
	entropy := 0.0
	for _, count := range frequency {
		prob := float64(count) / float64(totalCount)
		if prob > 0 {
			entropy -= prob * math.Log2(prob)
		}
	}

	// Determine if categorical
	maxReasonableCategories := min(50, totalCount/10)
	isCategorical := uniqueCount <= maxReasonableCategories

	// Find mode
	var mode float64
	maxFreq := 0
	for val, freq := range frequency {
		if freq > maxFreq {
			maxFreq = freq
			mode = val
		}
	}

	// Calculate Gini index (inequality measure)
	giniIndex := c.calculateGiniIndex(frequency, totalCount)

	return brief.CategoricalStats{
		IsCategorical: isCategorical,
		Cardinality:   uniqueCount,
		Entropy:       entropy,
		Mode:          fmt.Sprintf("%.6g", mode), // Avoid float precision issues
		ModeFrequency: maxFreq,
		GiniIndex:     giniIndex,
	}
}

func (c *StatisticalBriefComputer) calculateGiniIndex(frequency map[float64]int, totalCount int) float64 {
	if totalCount == 0 {
		return 0
	}

	// Gini index calculation for categorical distributions
	sumSquares := 0.0
	for _, count := range frequency {
		prob := float64(count) / float64(totalCount)
		sumSquares += prob * prob
	}

	// Gini = 1 - sum(p_i^2)
	gini := 1.0 - sumSquares

	return gini
}

// Temporal analysis
func (c *StatisticalBriefComputer) computeTemporalStats(data []float64) brief.TemporalStats {
	temporal := brief.TemporalStats{}

	// Stationarity test (variance F-test between halves, two-sided)
	temporal.IsStationary, temporal.VariancePValue = c.varianceStationarityFTest(data)

	// Autocorrelation
	if len(data) > 1 {
		temporal.AutocorrLag1 = c.calculateAutocorrelation(data, 1)
	}

	// Suggested causal lags
	temporal.SuggestedLags = c.detectSuggestedLags(data)

	// Stability score (inverse of variability)
	temporal.StabilityScore = c.calculateStabilityScore(data)

	// Placeholder ADF values (would need full implementation)
	temporal.AdfStatistic = 0.0
	temporal.AdfPValue = 0.5

	return temporal
}

func (c *StatisticalBriefComputer) varianceStationarityFTest(data []float64) (isStationary bool, pValue float64) {
	if len(data) < 10 {
		return false, 1.0
	}

	halfPoint := len(data) / 2
	firstHalf := data[:halfPoint]
	secondHalf := data[halfPoint:]

	var1, _ := stats.Variance(firstHalf)
	var2, _ := stats.Variance(secondHalf)

	// Two-sided F-test for equality of variances.
	df1 := len(firstHalf) - 1
	df2 := len(secondHalf) - 1
	if df1 <= 0 || df2 <= 0 {
		return false, 1.0
	}

	den := var2
	num := var1
	if den <= 0 {
		return false, 1.0
	}

	f := num / den
	if f <= 0 {
		return false, 1.0
	}

	fDist := distuv.F{D1: float64(df1), D2: float64(df2)}
	cdf := fDist.CDF(f)
	// Two-sided p-value: 2*min(CDF, 1-CDF)
	pValue = 2 * math.Min(cdf, 1-cdf)
	if pValue > 1 {
		pValue = 1
	}

	isStationary = pValue > 0.05
	return isStationary, pValue
}

func (c *StatisticalBriefComputer) calculateAutocorrelation(data []float64, lag int) float64 {
	if len(data) <= lag {
		return 0
	}

	n := len(data) - lag
	mean, _ := stats.Mean(data)

	numerator := 0.0
	denom1 := 0.0
	denom2 := 0.0

	for i := 0; i < n; i++ {
		diff1 := data[i] - mean
		diff2 := data[i+lag] - mean

		numerator += diff1 * diff2
		denom1 += diff1 * diff1
		denom2 += diff2 * diff2
	}

	if denom1 == 0 || denom2 == 0 {
		return 0
	}

	return numerator / math.Sqrt(denom1*denom2)
}

func (c *StatisticalBriefComputer) detectSuggestedLags(data []float64) []int {
	type lagScore struct {
		lag  int
		absR float64
	}
	var candidates []lagScore

	maxLag := min(10, len(data)/3)
	if maxLag < 1 {
		return nil
	}

	// Bartlett-style standard error for ACF:
	// Var(r_k) ≈ (1 + 2 * Σ_{j=1}^{k-1} r_j^2) / n  under mild assumptions.
	n := float64(len(data))
	sumSqPrev := 0.0

	for lag := 1; lag <= maxLag; lag++ {
		r := c.calculateAutocorrelation(data, lag)
		absR := math.Abs(r)

		se := math.Sqrt((1 + 2*sumSqPrev) / n)
		bound := 1.96 * se // ~95% CI

		if absR > bound {
			candidates = append(candidates, lagScore{lag: lag, absR: absR})
		}

		sumSqPrev += r * r
	}

	if len(candidates) == 0 {
		return nil
	}

	// Keep strongest 3 by |r|
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].absR > candidates[j].absR
	})
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}

	// Return sorted by lag for stable downstream behavior.
	out := make([]int, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.lag)
	}
	sort.Ints(out)
	return out
}

func (c *StatisticalBriefComputer) calculateStabilityScore(data []float64) float64 {
	if len(data) < 10 {
		return 0.0
	}

	// Rolling window variance analysis
	windowSize := min(30, len(data)/3)
	if windowSize < 5 {
		windowSize = len(data)
	}

	variances := []float64{}
	for i := 0; i <= len(data)-windowSize; i++ {
		window := data[i : i+windowSize]
		variance, _ := stats.Variance(window)
		variances = append(variances, variance)
	}

	// Coefficient of variation of variances (lower = more stable)
	meanVar, _ := stats.Mean(variances)
	stdVar, _ := stats.StandardDeviation(variances)

	if meanVar == 0 {
		return 1.0 // Perfect stability
	}

	cv := stdVar / meanVar

	// Convert to 0-1 score (lower CV = higher stability)
	stabilityScore := 1.0 / (1.0 + cv)

	return math.Min(stabilityScore, 1.0)
}

// Utility functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ComputationError represents brief computation errors
type ComputationError struct {
	Message string
	Cause   error
}

func (e ComputationError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func NewComputationError(message string, cause error) ComputationError {
	return ComputationError{Message: message, Cause: cause}
}
