package brief

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/stat/distuv"
)

// StatisticalDistributions provides unified access to all statistical distributions
// This replaces fragmented CDF calculations throughout the codebase
type StatisticalDistributions struct{}

// NewDistributions creates a new distributions utility
func NewDistributions() *StatisticalDistributions {
	return &StatisticalDistributions{}
}

// TTestPValue computes exact p-value for t-test using Student's t-distribution
func (sd *StatisticalDistributions) TTestPValue(tStatistic float64, degreesOfFreedom int) float64 {
	if degreesOfFreedom <= 0 {
		return 1.0
	}

	df := float64(degreesOfFreedom)
	tDist := distuv.StudentsT{Mu: 0, Sigma: 1, Nu: df}

	// Two-tailed test
	return 2 * (1 - tDist.CDF(math.Abs(tStatistic)))
}

// CorrelationPValue computes exact p-value for correlation coefficient
func (sd *StatisticalDistributions) CorrelationPValue(correlation float64, sampleSize int) float64 {
	if sampleSize < 3 {
		return 1.0
	}

	// Transform correlation to t-statistic
	df := float64(sampleSize - 2)
	tStatistic := correlation * math.Sqrt(df/(1-correlation*correlation))

	return sd.TTestPValue(tStatistic, int(df))
}

// FTestPValue computes p-value for F-distribution (ANOVA, regression)
func (sd *StatisticalDistributions) FTestPValue(fStatistic float64, df1, df2 int) float64 {
	if df1 <= 0 || df2 <= 0 {
		return 1.0
	}

	fDist := distuv.F{D1: float64(df1), D2: float64(df2)}
	return 1 - fDist.CDF(fStatistic)
}

// ChiSquarePValue computes p-value for chi-square distribution
func (sd *StatisticalDistributions) ChiSquarePValue(chiSquare float64, degreesOfFreedom int) float64 {
	if degreesOfFreedom <= 0 {
		return 1.0
	}

	chiDist := distuv.ChiSquared{K: float64(degreesOfFreedom)}
	return 1 - chiDist.CDF(chiSquare)
}

// NormalCDF computes cumulative distribution function for standard normal
func (sd *StatisticalDistributions) NormalCDF(x float64) float64 {
	return distuv.UnitNormal.CDF(x)
}

// NormalQuantile computes quantile function for standard normal (inverse CDF)
func (sd *StatisticalDistributions) NormalQuantile(p float64) float64 {
	return distuv.UnitNormal.Quantile(p)
}

// BetaCDF computes cumulative distribution function for beta distribution
func (sd *StatisticalDistributions) BetaCDF(x, alpha, beta float64) float64 {
	if x <= 0 || x >= 1 || alpha <= 0 || beta <= 0 {
		return 0
	}

	betaDist := distuv.Beta{Alpha: alpha, Beta: beta}
	return betaDist.CDF(x)
}

// PermutationPValue computes exact p-value from permutation test results
func (sd *StatisticalDistributions) PermutationPValue(observedStatistic float64, nullDistribution []float64, twoTailed bool) float64 {
	if len(nullDistribution) == 0 {
		return 1.0
	}

	extremeCount := 0
	for _, nullStat := range nullDistribution {
		if twoTailed {
			if math.Abs(nullStat) >= math.Abs(observedStatistic) {
				extremeCount++
			}
		} else {
			if nullStat >= observedStatistic {
				extremeCount++
			}
		}
	}

	return float64(extremeCount) / float64(len(nullDistribution))
}

// BootstrapConfidenceInterval computes confidence interval from bootstrap samples
func (sd *StatisticalDistributions) BootstrapConfidenceInterval(samples []float64, confidenceLevel float64) (lower, upper float64) {
	if len(samples) == 0 {
		return 0, 0
	}

	if confidenceLevel <= 0 || confidenceLevel >= 1 {
		confidenceLevel = 0.95
	}

	// Sort samples (O(n log n), avoids quadratic bubble sort)
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)

	// Compute percentiles
	alpha := 1.0 - confidenceLevel
	lowerPercentile := alpha / 2.0
	upperPercentile := 1.0 - alpha/2.0

	lowerIdx := int(math.Round(float64(len(sorted)-1) * lowerPercentile))
	upperIdx := int(math.Round(float64(len(sorted)-1) * upperPercentile))

	if lowerIdx >= len(sorted) {
		lowerIdx = len(sorted) - 1
	}
	if upperIdx >= len(sorted) {
		upperIdx = len(sorted) - 1
	}

	return sorted[lowerIdx], sorted[upperIdx]
}

// MannWhitneyPValue computes approximate p-value for Mann-Whitney U test
func (sd *StatisticalDistributions) MannWhitneyPValue(uStatistic float64, n1, n2 int) float64 {
	if n1 <= 0 || n2 <= 0 {
		return 1.0
	}

	// Normal approximation for large samples
	meanU := float64(n1*n2) / 2.0
	stdU := math.Sqrt(float64(n1*n2*(n1+n2+1)) / 12.0)

	if stdU == 0 {
		return 1.0
	}

	// Convert to z-score
	z := (uStatistic - meanU) / stdU

	// Two-tailed test
	return 2 * (1 - sd.NormalCDF(math.Abs(z)))
}

// KruskalWallisPValue computes approximate p-value for Kruskal-Wallis test
func (sd *StatisticalDistributions) KruskalWallisPValue(hStatistic float64, k int, n int) float64 {
	if k < 2 || n < k {
		return 1.0
	}

	// Chi-square approximation
	df := k - 1
	return sd.ChiSquarePValue(hStatistic, df)
}

// WilcoxonSignedRankPValue computes approximate p-value for Wilcoxon signed-rank test
func (sd *StatisticalDistributions) WilcoxonSignedRankPValue(tStatistic float64, n int) float64 {
	if n <= 0 {
		return 1.0
	}

	// Normal approximation for n > 10
	if n > 10 {
		meanT := float64(n*(n+1)) / 4.0
		stdT := math.Sqrt(float64(n*(n+1)*(2*n+1)) / 24.0)

		if stdT == 0 {
			return 1.0
		}

		z := (tStatistic - meanT) / stdT
		return 2 * (1 - sd.NormalCDF(math.Abs(z)))
	}

	// For small n, compute the exact distribution of W+ (sum of positive ranks).
	return sd.wilcoxonSignedRankExactTwoSidedPValue(tStatistic, n)
}

func (sd *StatisticalDistributions) wilcoxonSignedRankExactTwoSidedPValue(tStatistic float64, n int) float64 {
	// W+ is integer-valued when there are no ties/zeros (we assume caller preprocessed).
	// We round to nearest int to be robust to float representation.
	wObs := int(math.Round(tStatistic))
	if wObs < 0 {
		wObs = 0
	}

	totalRankSum := n * (n + 1) / 2
	if wObs > totalRankSum {
		wObs = totalRankSum
	}

	// Two-sided p-value uses symmetry: P(W+ <= w) where w = min(W+, total-W+), then *2.
	w := wObs
	if totalRankSum-wObs < w {
		w = totalRankSum - wObs
	}

	// Dynamic programming for subset sums of ranks 1..n.
	// dp[s] = number of sign assignments producing W+ = s.
	dp := make([]uint64, totalRankSum+1)
	dp[0] = 1
	for r := 1; r <= n; r++ {
		for s := totalRankSum; s >= r; s-- {
			dp[s] += dp[s-r]
		}
	}

	totalOutcomes := uint64(1) << uint(n) // 2^n
	var cum uint64
	for s := 0; s <= w; s++ {
		cum += dp[s]
	}

	pOneSide := float64(cum) / float64(totalOutcomes)
	pTwoSide := 2 * pOneSide
	if pTwoSide > 1.0 {
		pTwoSide = 1.0
	}
	return pTwoSide
}

// EffectSizeCohenD computes Cohen's d effect size for two groups
func (sd *StatisticalDistributions) EffectSizeCohenD(mean1, mean2, std1, std2 float64, n1, n2 int) float64 {
	if n1 <= 0 || n2 <= 0 {
		return 0
	}

	// Pooled standard deviation
	pooledStd := math.Sqrt(((float64(n1-1) * std1 * std1) + (float64(n2-1) * std2 * std2)) / float64(n1+n2-2))

	if pooledStd == 0 {
		return 0
	}

	return (mean1 - mean2) / pooledStd
}

// EffectSizeHedgesG computes Hedges' g (bias-corrected Cohen's d)
func (sd *StatisticalDistributions) EffectSizeHedgesG(cohenD float64, totalN int) float64 {
	if totalN < 3 {
		return cohenD
	}

	// Hedges' g correction factor
	correction := 1.0 - (3.0 / (4.0*float64(totalN) - 9.0))
	return cohenD * correction
}

// ConfidenceIntervalMean computes confidence interval for population mean
func (sd *StatisticalDistributions) ConfidenceIntervalMean(sampleMean, sampleStd float64, sampleSize int, confidenceLevel float64) (lower, upper float64) {
	if sampleSize < 2 {
		return sampleMean, sampleMean
	}

	// t-critical value for confidence level
	df := float64(sampleSize - 1)
	alpha := 1.0 - confidenceLevel
	tCritical := distuv.StudentsT{Mu: 0, Sigma: 1, Nu: df}.Quantile(1.0 - alpha/2.0)

	// Standard error
	se := sampleStd / math.Sqrt(float64(sampleSize))

	// Confidence interval
	margin := tCritical * se
	return sampleMean - margin, sampleMean + margin
}

// PowerAnalysis computes statistical power for various tests
type PowerAnalysis struct {
	distributions *StatisticalDistributions
}

func NewPowerAnalysis() *PowerAnalysis {
	return &PowerAnalysis{
		distributions: NewDistributions(),
	}
}

// TTestPower computes power for two-sample t-test
func (pa *PowerAnalysis) TTestPower(effectSize, alpha float64, n1, n2 int) float64 {
	if n1 <= 0 || n2 <= 0 {
		return 0
	}

	// Non-central t-distribution
	df := float64(n1 + n2 - 2)
	nonCentrality := effectSize * math.Sqrt(float64(n1*n2)/float64(n1+n2))

	// Critical value
	tDist := distuv.StudentsT{Mu: 0, Sigma: 1, Nu: df}
	_ = tDist.Quantile(1.0 - alpha/2.0) // tCritical not used in approximation

	// Power = 1 - β = 1 - CDF(tCritical | non-centrality parameter)
	// NOTE: NonCentralT is not available in gonum/distuv, using approximation
	// For exact power calculation, would need non-central t-distribution implementation
	// Approximation: use normal approximation for large samples
	if df > 30 {
		zCritical := distuv.UnitNormal.Quantile(1.0 - alpha/2.0)
		zPower := nonCentrality - zCritical
		return distuv.UnitNormal.CDF(zPower)
	}
	// For small samples, return conservative estimate
	return 0.5
}

// SampleSizeTTest computes required sample size for desired power in t-test
func (pa *PowerAnalysis) SampleSizeTTest(effectSize, power, alpha float64) (n1, n2 int) {
	// Simplified calculation - in production would use more sophisticated methods
	// This is a rough approximation
	zAlpha := pa.distributions.NormalQuantile(1.0 - alpha/2.0)
	zBeta := pa.distributions.NormalQuantile(power)

	numerator := (zAlpha + zBeta) * (zAlpha + zBeta)
	denominator := effectSize * effectSize

	if denominator == 0 {
		return 100, 100 // Default
	}

	totalN := numerator / denominator
	nPerGroup := int(math.Ceil(totalN / 2.0))

	return nPerGroup, nPerGroup
}
