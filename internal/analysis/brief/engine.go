package brief

import (
	"context"
	"fmt"
	"math"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/stats/brief"
)

// StatisticalEngine provides unified access to all statistical computations
// This replaces the fragmented adapters/stats/engine/ system
type StatisticalEngine struct {
	computer      *StatisticalBriefComputer
	senses        *SenseEngine
	distributions *StatisticalDistributions
}

// NewStatisticalEngine creates a new unified statistical engine
func NewStatisticalEngine() *StatisticalEngine {
	return &StatisticalEngine{
		computer:      NewComputer(),
		senses:        NewSenseEngine(NewComputer()),
		distributions: NewDistributions(),
	}
}

// Computer returns the statistical brief computer
func (se *StatisticalEngine) Computer() *StatisticalBriefComputer {
	return se.computer
}

// AnalyzeRelationship performs comprehensive statistical analysis of a relationship
func (se *StatisticalEngine) AnalyzeRelationship(ctx context.Context, x, y []float64, testType string, varX, varY core.VariableKey) (*RelationshipAnalysis, error) {
	if len(x) != len(y) {
		return nil, fmt.Errorf("variable length mismatch: x=%d, y=%d", len(x), len(y))
	}

	analysis := &RelationshipAnalysis{
		VariableX:  varX,
		VariableY:  varY,
		TestType:   testType,
		SampleSize: len(x),
		ComputedAt: time.Now(),
	}

	// Generate statistical brief for x variable (primary)
	briefReq := brief.ComputationRequest{
		ForValidation: true,
		ForHypothesis: true,
	}

	statBrief, err := se.computer.ComputeBrief(x, varX.String(), "pairwise", briefReq)
	if err != nil {
		return nil, fmt.Errorf("failed to compute statistical brief: %w", err)
	}
	analysis.Brief = statBrief

	// Perform sense analysis based on test type
	senseResults := se.analyzeByTestType(ctx, x, y, testType, varX, varY)
	analysis.SenseResults = senseResults

	// Compute primary relationship metrics
	primaryMetrics := se.computePrimaryMetrics(x, y, testType)
	analysis.PrimaryMetrics = primaryMetrics

	return analysis, nil
}

// analyzeByTestType performs sense analysis appropriate for the test type
func (se *StatisticalEngine) analyzeByTestType(ctx context.Context, x, y []float64, testType string, varX, varY core.VariableKey) []brief.SenseResult {
	switch testType {
	case "correlation", "pearson":
		// Use correlation-focused senses
		results := []brief.SenseResult{}

		// Add specific senses we want
		if result, ok := se.senses.AnalyzeSingle(ctx, "mutual_information", x, y, varX, varY); ok {
			results = append(results, result)
		}
		if result, ok := se.senses.AnalyzeSingle(ctx, "spearman", x, y, varX, varY); ok {
			results = append(results, result)
		}
		if result, ok := se.senses.AnalyzeSingle(ctx, "cross_correlation", x, y, varX, varY); ok {
			results = append(results, result)
		}

		return results

	case "categorical", "chisquare":
		// Use categorical-focused senses
		if result, ok := se.senses.AnalyzeSingle(ctx, "chi_square", x, y, varX, varY); ok {
			return []brief.SenseResult{result}
		}

	case "timeseries", "temporal":
		// Use temporal senses with timestamp context
		results := []brief.SenseResult{}
		if result, ok := se.senses.AnalyzeSingle(ctx, "temporal_day", x, y, varX, varY); ok {
			results = append(results, result)
		}

		return results

	default:
		// Run all senses
		return se.senses.AnalyzeAll(ctx, x, y, varX, varY)
	}

	return []brief.SenseResult{}
}

// computePrimaryMetrics calculates the main statistical metrics for the relationship
func (se *StatisticalEngine) computePrimaryMetrics(x, y []float64, testType string) *PrimaryMetrics {
	metrics := &PrimaryMetrics{}

	switch testType {
	case "correlation", "pearson":
		// Compute Pearson correlation with proper statistical testing
		corr, pValue := se.computePearsonCorrelation(x, y)
		metrics.EffectSize = corr
		metrics.PValue = pValue
		lower, upper := se.computeCorrelationCI(corr, len(x))
		metrics.ConfidenceInterval = [2]float64{lower, upper}
		metrics.Interpretation = se.interpretCorrelation(corr, pValue)

	case "categorical", "chisquare":
		// Compute chi-square test
		chiSquare, pValue, cramersV := se.computeChiSquareTest(x, y)
		metrics.EffectSize = cramersV
		metrics.PValue = pValue
		metrics.TestStatistic = chiSquare
		metrics.Interpretation = se.interpretChiSquare(cramersV, pValue)

	case "difference", "ttest":
		// Compute t-test (simplified - assumes equal variances)
		tStat, pValue, cohenD := se.computeTTest(x, y)
		metrics.EffectSize = cohenD
		metrics.PValue = pValue
		metrics.TestStatistic = tStat
		metrics.Interpretation = se.interpretTTest(cohenD, pValue)

	default:
		metrics.Interpretation = "Test type not supported"
	}

	return metrics
}

// ===== PRIMARY STATISTICAL COMPUTATIONS =====

// computePearsonCorrelation computes Pearson correlation with proper error handling
func (se *StatisticalEngine) computePearsonCorrelation(x, y []float64) (correlation, pValue float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 1.0
	}

	// Compute correlation
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0
	n := float64(len(x))

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denomX := n*sumX2 - sumX*sumX
	denomY := n*sumY2 - sumY*sumY

	if denomX <= 0 || denomY <= 0 {
		return 0, 1.0
	}

	correlation = numerator / math.Sqrt(denomX*denomY)

	// Clamp to valid range
	if correlation > 1.0 {
		correlation = 1.0
	} else if correlation < -1.0 {
		correlation = -1.0
	}

	// Compute p-value using proper statistical distribution
	pValue = se.distributions.CorrelationPValue(correlation, len(x))

	return correlation, pValue
}

// computeChiSquareTest performs chi-square test for categorical association
func (se *StatisticalEngine) computeChiSquareTest(x, y []float64) (chiSquare, pValue, cramersV float64) {
	if len(x) != len(y) || len(x) == 0 {
		return 0, 1.0, 0
	}

	table, n, r, c := buildContingencyTable(x, y)
	if n == 0 || r < 2 || c < 2 {
		return 0, 1.0, 0
	}

	chiSquare = chiSquareFromContingency(table, n, r, c)
	df := (r - 1) * (c - 1)
	if df <= 0 {
		return 0, 1.0, 0
	}

	pValue = se.distributions.ChiSquarePValue(chiSquare, df)

	// Cramer's V = sqrt(chi^2 / (n * min(r-1, c-1)))
	den := float64(n) * float64(minInt(r-1, c-1))
	if den <= 0 {
		return chiSquare, pValue, 0
	}
	cramersV = math.Sqrt(chiSquare / den)
	return chiSquare, pValue, cramersV
}

// computeTTest performs two-sample t-test
func (se *StatisticalEngine) computeTTest(x, y []float64) (tStat, pValue, cohenD float64) {
	if len(x) < 2 || len(y) < 2 {
		return 0, 1.0, 0
	}

	// Compute means and variances
	meanX, varX := se.computeMeanVariance(x)
	meanY, varY := se.computeMeanVariance(y)

	// Pooled variance t-test
	n1, n2 := float64(len(x)), float64(len(y))
	pooledVar := ((n1-1)*varX + (n2-1)*varY) / (n1 + n2 - 2)

	if pooledVar <= 0 {
		return 0, 1.0, 0
	}

	// t-statistic
	tStat = (meanX - meanY) / math.Sqrt(pooledVar*(1.0/n1+1.0/n2))

	// p-value
	df := n1 + n2 - 2
	pValue = se.distributions.TTestPValue(tStat, int(df))

	// Cohen's d effect size
	pooledStd := math.Sqrt(pooledVar)
	cohenD = (meanX - meanY) / pooledStd

	return tStat, pValue, cohenD
}

// computeMeanVariance computes mean and variance for a sample
func (se *StatisticalEngine) computeMeanVariance(data []float64) (mean, variance float64) {
	if len(data) == 0 {
		return 0, 0
	}

	n := float64(len(data))

	// Compute mean
	sum := 0.0
	for _, val := range data {
		sum += val
	}
	mean = sum / n

	// Compute variance
	sumSqDiff := 0.0
	for _, val := range data {
		diff := val - mean
		sumSqDiff += diff * diff
	}

	// Sample variance (divide by n-1)
	if n > 1 {
		variance = sumSqDiff / (n - 1)
	}

	return mean, variance
}

// ===== CONFIDENCE INTERVALS =====

// computeCorrelationCI computes confidence interval for correlation coefficient
func (se *StatisticalEngine) computeCorrelationCI(r float64, n int) (lower, upper float64) {
	if n < 3 {
		return r, r
	}

	// Fisher transformation
	z := 0.5 * math.Log((1+r)/(1-r))
	stderr := 1.0 / math.Sqrt(float64(n-3))

	// 95% confidence interval
	zLower := z - 1.96*stderr
	zUpper := z + 1.96*stderr

	// Transform back to correlation
	lower = (math.Exp(2*zLower) - 1) / (math.Exp(2*zLower) + 1)
	upper = (math.Exp(2*zUpper) - 1) / (math.Exp(2*zUpper) + 1)

	return lower, upper
}

// ===== INTERPRETATIONS =====

// interpretCorrelation provides human-readable interpretation of correlation results
func (se *StatisticalEngine) interpretCorrelation(r, pValue float64) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No statistically significant correlation (r=%.3f, p=%.3f)", r, pValue)
	}

	strength := "weak"
	if math.Abs(r) > 0.8 {
		strength = "very strong"
	} else if math.Abs(r) > 0.6 {
		strength = "strong"
	} else if math.Abs(r) > 0.3 {
		strength = "moderate"
	}

	direction := "positive"
	if r < 0 {
		direction = "negative"
	}

	return fmt.Sprintf("%s %s correlation (r=%.3f, p=%.3f)", strength, direction, r, pValue)
}

// interpretChiSquare provides interpretation for chi-square test results
func (se *StatisticalEngine) interpretChiSquare(v, pValue float64) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant categorical association (V=%.3f, p=%.3f)", v, pValue)
	}

	strength := "weak"
	if v > 0.5 {
		strength = "very strong"
	} else if v > 0.3 {
		strength = "strong"
	} else if v > 0.1 {
		strength = "moderate"
	}

	return fmt.Sprintf("%s categorical association (Cramer's V=%.3f, p=%.3f)", strength, v, pValue)
}

// interpretTTest provides interpretation for t-test results
func (se *StatisticalEngine) interpretTTest(d, pValue float64) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No statistically significant difference (d=%.3f, p=%.3f)", d, pValue)
	}

	strength := "small"
	if math.Abs(d) > 0.8 {
		strength = "large"
	} else if math.Abs(d) > 0.5 {
		strength = "medium"
	}

	return fmt.Sprintf("%s effect size difference (Cohen's d=%.3f, p=%.3f)", strength, d, pValue)
}

// ===== RESULT STRUCTURES =====

// RelationshipAnalysis contains complete statistical analysis of a relationship
type RelationshipAnalysis struct {
	VariableX      core.VariableKey        `json:"variable_x"`
	VariableY      core.VariableKey        `json:"variable_y"`
	TestType       string                  `json:"test_type"`
	SampleSize     int                     `json:"sample_size"`
	Brief          *brief.StatisticalBrief `json:"brief"`
	SenseResults   []brief.SenseResult     `json:"sense_results"`
	PrimaryMetrics *PrimaryMetrics         `json:"primary_metrics"`
	ComputedAt     time.Time               `json:"computed_at"`
}

// PrimaryMetrics contains the main statistical metrics for the relationship
type PrimaryMetrics struct {
	EffectSize         float64    `json:"effect_size"`
	PValue             float64    `json:"p_value"`
	TestStatistic      float64    `json:"test_statistic,omitempty"`
	ConfidenceInterval [2]float64 `json:"confidence_interval,omitempty"`
	Interpretation     string     `json:"interpretation"`
}

// QuickAnalysis provides fast statistical summary without full brief computation
func (se *StatisticalEngine) QuickAnalysis(x, y []float64, testType string) (*QuickResult, error) {
	if len(x) != len(y) {
		return nil, fmt.Errorf("variable length mismatch")
	}

	result := &QuickResult{
		TestType:   testType,
		SampleSize: len(x),
	}

	switch testType {
	case "correlation":
		corr, pValue := se.computePearsonCorrelation(x, y)
		result.EffectSize = corr
		result.PValue = pValue
		result.Interpretation = se.interpretCorrelation(corr, pValue)

	case "difference":
		_, pValue, cohenD := se.computeTTest(x, y)
		result.EffectSize = cohenD
		result.PValue = pValue
		result.Interpretation = se.interpretTTest(cohenD, pValue)

	default:
		result.Interpretation = "Test type not supported for quick analysis"
	}

	return result, nil
}

// QuickResult provides fast statistical summary
type QuickResult struct {
	TestType       string  `json:"test_type"`
	SampleSize     int     `json:"sample_size"`
	EffectSize     float64 `json:"effect_size"`
	PValue         float64 `json:"p_value"`
	Interpretation string  `json:"interpretation"`
}

func buildContingencyTable(x, y []float64) (table [][]int, n, r, c int) {
	// Build row/col category indices based on observed levels.
	rowIndex := map[string]int{}
	colIndex := map[string]int{}

	type pair struct {
		r int
		c int
	}
	pairs := make([]pair, 0, len(x))

	for i := 0; i < len(x) && i < len(y); i++ {
		if math.IsNaN(x[i]) || math.IsNaN(y[i]) || math.IsInf(x[i], 0) || math.IsInf(y[i], 0) {
			continue
		}

		rKey := categoryKey(x[i])
		cKey := categoryKey(y[i])

		ri, ok := rowIndex[rKey]
		if !ok {
			ri = len(rowIndex)
			rowIndex[rKey] = ri
		}

		ci, ok := colIndex[cKey]
		if !ok {
			ci = len(colIndex)
			colIndex[cKey] = ci
		}

		pairs = append(pairs, pair{r: ri, c: ci})
	}

	r = len(rowIndex)
	c = len(colIndex)
	n = len(pairs)
	if n == 0 || r == 0 || c == 0 {
		return nil, 0, r, c
	}

	table = make([][]int, r)
	for i := range table {
		table[i] = make([]int, c)
	}
	for _, p := range pairs {
		table[p.r][p.c]++
	}
	return table, n, r, c
}

func chiSquareFromContingency(obs [][]int, n, r, c int) float64 {
	// Compute row/col sums
	rowSums := make([]int, r)
	colSums := make([]int, c)
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			rowSums[i] += obs[i][j]
			colSums[j] += obs[i][j]
		}
	}

	is2x2 := r == 2 && c == 2
	chi := 0.0
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			exp := float64(rowSums[i]*colSums[j]) / float64(n)
			if exp <= 0 {
				continue
			}
			o := float64(obs[i][j])

			if is2x2 {
				// Yates' continuity correction for 2x2 tables.
				diff := math.Abs(o-exp) - 0.5
				if diff < 0 {
					diff = 0
				}
				chi += (diff * diff) / exp
				continue
			}

			d := o - exp
			chi += (d * d) / exp
		}
	}
	return chi
}

func categoryKey(v float64) string {
	// Prefer integer bucket keys if value is effectively integer.
	r := math.Round(v)
	if math.Abs(v-r) < 1e-9 {
		return fmt.Sprintf("%.0f", r)
	}
	// Stable-ish float key.
	return fmt.Sprintf("%.6g", v)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}


