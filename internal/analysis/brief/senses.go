package brief

import (
	"context"
	"fmt"
	"math"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/stats/brief"

	"github.com/montanaflynn/stats"
	"gonum.org/v1/gonum/stat/distuv"
)

// SenseContext provides optional auxiliary data for senses that need it (e.g. timestamps).
type SenseContext struct {
	Timestamps []time.Time            // Optional: one timestamp per sample
	Metadata   map[string]interface{} // Extensible auxiliary context
}

// ContextualSense is implemented by senses that can use SenseContext.
type ContextualSense interface {
	AnalyzeWithContext(ctx context.Context, x, y []float64, varX, varY core.VariableKey, senseCtx *SenseContext) brief.SenseResult
}

// StatisticalSense defines the interface for each statistical sense
type StatisticalSense interface {
	Name() string
	Description() string
	Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) brief.SenseResult
	RequiresGroups() bool // Some senses need group segmentation (like t-test)
}

// SenseEngine orchestrates all statistical senses using the unified brief system
type SenseEngine struct {
	computer *StatisticalBriefComputer
	senses   []StatisticalSense
}

// NewSenseEngine creates a new statistical senses engine integrated with briefs
func NewSenseEngine(computer *StatisticalBriefComputer) *SenseEngine {
	return &SenseEngine{
		computer: computer,
		senses: []StatisticalSense{
			NewMutualInformationSense(),
			NewWelchTTestSense(),
			NewChiSquareSense(),
			NewSpearmanSense(),
			NewCrossCorrelationSense(),
			NewTemporalSense("day"),
		},
	}
}

// AnalyzeAll runs all senses concurrently and returns results
func (e *SenseEngine) AnalyzeAll(ctx context.Context, x, y []float64, varX, varY core.VariableKey) []brief.SenseResult {
	return e.AnalyzeAllWithContext(ctx, x, y, varX, varY, nil)
}

// AnalyzeAllWithContext runs all senses concurrently and passes optional SenseContext
func (e *SenseEngine) AnalyzeAllWithContext(ctx context.Context, x, y []float64, varX, varY core.VariableKey, senseCtx *SenseContext) []brief.SenseResult {
	results := make([]brief.SenseResult, len(e.senses))

	// Create channels for concurrent execution
	type resultWithIndex struct {
		result brief.SenseResult
		index  int
	}

	resultChan := make(chan resultWithIndex, len(e.senses))

	// Run all senses concurrently
	for i, sense := range e.senses {
		go func(sense StatisticalSense, idx int) {
			// If the sense can consume context, prefer it.
			if cs, ok := sense.(ContextualSense); ok {
				result := cs.AnalyzeWithContext(ctx, x, y, varX, varY, senseCtx)
				resultChan <- resultWithIndex{result: result, index: idx}
				return
			}

			result := sense.Analyze(ctx, x, y, varX, varY)
			resultChan <- resultWithIndex{result: result, index: idx}
		}(sense, i)
	}

	// Collect results
	for i := 0; i < len(e.senses); i++ {
		result := <-resultChan
		results[result.index] = result.result
	}

	return results
}

// AnalyzeSingle runs a specific sense by name
func (e *SenseEngine) AnalyzeSingle(ctx context.Context, senseName string, x, y []float64, varX, varY core.VariableKey) (brief.SenseResult, bool) {
	for _, sense := range e.senses {
		if sense.Name() == senseName {
			result := sense.Analyze(ctx, x, y, varX, varY)
			return result, true
		}
	}
	return brief.SenseResult{}, false
}

// GetAvailableSenses returns list of available sense names
func (e *SenseEngine) GetAvailableSenses() []string {
	names := make([]string, len(e.senses))
	for i, sense := range e.senses {
		names[i] = sense.Name()
	}
	return names
}

// ===== INDIVIDUAL SENSE IMPLEMENTATIONS =====

// MutualInformationSense detects non-linear relationships using mutual information
type MutualInformationSense struct{}

func NewMutualInformationSense() *MutualInformationSense {
	return &MutualInformationSense{}
}

func (s *MutualInformationSense) Name() string {
	return "mutual_information"
}

func (s *MutualInformationSense) Description() string {
	return "Detects non-linear relationships robust to complex dependencies"
}

func (s *MutualInformationSense) RequiresGroups() bool {
	return false
}

func (s *MutualInformationSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) brief.SenseResult {
	if len(x) != len(y) || len(x) < 10 {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for mutual information analysis",
		}
	}

	// Use KSG mutual information estimator
	mi := s.computeMutualInformation(x, y)
	pValue := s.computeMIPValue(mi, len(x))

	// Classify signal strength
	signal := s.classifyMISignal(mi, pValue)

	return brief.SenseResult{
		SenseName:   s.Name(),
		EffectSize:  mi,
		PValue:      pValue,
		Confidence:  1.0 - pValue,
		Signal:      signal,
		Description: s.generateMIDescription(mi, pValue),
		Metadata: map[string]interface{}{
			"estimator":   "ksg",
			"k_neighbors": 5,
		},
	}
}

func (s *MutualInformationSense) computeMutualInformation(x, y []float64) float64 {
	// KSG mutual information estimator with proper joint/marginal logic
	n := len(x)
	if n < 5 {
		return 0
	}

	k := 5 // k-nearest neighbors
	mi := 0.0

	for i := 0; i < n; i++ {
		// Find k-th nearest neighbor distance in joint space (max norm)
		epsJoint := s.findKthJointDistance(x, y, i, k)

		// Count points within eps in marginal spaces
		nx := s.countWithinRadius(x, x[i], epsJoint)
		ny := s.countWithinRadius(y, y[i], epsJoint)

		// KSG estimator: ψ(k) - ψ(nx) - ψ(ny) + ψ(n)
		// Using approximation: ψ(x) ≈ ln(x) for large x
		if nx > 0 && ny > 0 {
			mi += math.Log(float64(n)) - math.Log(float64(nx)) - math.Log(float64(ny)) + math.Log(float64(k))
		}
	}

	return math.Max(0, mi/float64(n))
}

func (s *MutualInformationSense) findKthJointDistance(x, y []float64, idx int, k int) float64 {
	distances := make([]float64, len(x))
	for i := range x {
		// Max norm (Chebyshev distance) for joint space
		dx := math.Abs(x[i] - x[idx])
		dy := math.Abs(y[i] - y[idx])
		distances[i] = math.Max(dx, dy)
	}

	// Sort and find k-th smallest distance
	for i := 0; i < len(distances)-1; i++ {
		for j := i + 1; j < len(distances); j++ {
			if distances[j] < distances[i] {
				distances[i], distances[j] = distances[j], distances[i]
			}
		}
	}

	if k < len(distances) {
		return distances[k]
	}
	return distances[len(distances)-1]
}

func (s *MutualInformationSense) countWithinRadius(data []float64, center, radius float64) int {
	count := 0
	for _, val := range data {
		if math.Abs(val-center) <= radius {
			count++
		}
	}
	return count
}

func (s *MutualInformationSense) computeMIPValue(mi float64, n int) float64 {
	// Permutation-based p-value approximation
	if mi <= 0 {
		return 1.0
	}

	// Rough approximation: higher MI = lower p-value
	// For exact p-value, would need permutation testing
	normalizedMI := math.Min(mi/2.0, 1.0) // Cap at 2.0 nats
	return 1.0 - normalizedMI
}

func (s *MutualInformationSense) classifyMISignal(mi, pValue float64) string {
	if pValue > 0.05 {
		return "weak"
	}
	if mi > 1.0 {
		return "very_strong"
	}
	if mi > 0.5 {
		return "strong"
	}
	if mi > 0.2 {
		return "moderate"
	}
	return "weak"
}

func (s *MutualInformationSense) generateMIDescription(mi, pValue float64) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant mutual information detected (MI=%.3f, p=%.3f)", mi, pValue)
	}
	return fmt.Sprintf("Mutual information suggests non-linear relationship (MI=%.3f, p=%.3f)", mi, pValue)
}

// WelchTTestSense performs Welch's t-test for unequal variances
type WelchTTestSense struct{}

func NewWelchTTestSense() *WelchTTestSense {
	return &WelchTTestSense{}
}

func (s *WelchTTestSense) Name() string {
	return "welch_ttest"
}

func (s *WelchTTestSense) Description() string {
	return "Compares means assuming unequal variances (robust to heteroscedasticity)"
}

func (s *WelchTTestSense) RequiresGroups() bool {
	return true // Requires group segmentation
}

func (s *WelchTTestSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) brief.SenseResult {
	// This sense requires group segmentation - return placeholder
	return brief.SenseResult{
		SenseName:   s.Name(),
		EffectSize:  0,
		PValue:      1.0,
		Confidence:  0,
		Signal:      "weak",
		Description: "Requires group segmentation for t-test analysis",
	}
}

// ChiSquareSense performs chi-square test for categorical association
type ChiSquareSense struct{}

func NewChiSquareSense() *ChiSquareSense {
	return &ChiSquareSense{}
}

func (s *ChiSquareSense) Name() string {
	return "chi_square"
}

func (s *ChiSquareSense) Description() string {
	return "Tests association between categorical variables"
}

func (s *ChiSquareSense) RequiresGroups() bool {
	return false
}

func (s *ChiSquareSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) brief.SenseResult {
	// Chi-square requires categorical data - check if data looks categorical
	isXCategorical := s.isDataCategorical(x)
	isYCategorical := s.isDataCategorical(y)

	if !isXCategorical || !isYCategorical {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Chi-square requires categorical data",
		}
	}

	// Create contingency table and compute chi-square
	table, n, r, c := buildContingencyTable(x, y)
	if n == 0 || r < 2 || c < 2 {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Unable to create contingency table",
		}
	}

	chiSquare := chiSquareFromContingency(table, n, r, c)
	df := (r - 1) * (c - 1)

	// Compute p-value using chi-square distribution
	chiDist := distuv.ChiSquared{K: float64(df)}
	pValue := 1 - chiDist.CDF(chiSquare)

	// Cramer's V effect size
	cramersV := math.Sqrt(chiSquare / (float64(n) * float64(minInt(r-1, c-1))))

	signal := s.classifyChiSquareSignal(cramersV, pValue)

	return brief.SenseResult{
		SenseName:   s.Name(),
		EffectSize:  cramersV,
		PValue:      pValue,
		Confidence:  1.0 - pValue,
		Signal:      signal,
		Description: s.generateChiSquareDescription(cramersV, pValue, chiSquare),
		Metadata: map[string]interface{}{
			"chi_square_statistic":   chiSquare,
			"degrees_of_freedom":     df,
			"contingency_table_size": len(table),
		},
	}
}

func (s *ChiSquareSense) isDataCategorical(data []float64) bool {
	unique := make(map[float64]bool)
	for _, val := range data {
		unique[val] = true
	}

	// Consider categorical if < 20 unique values or < 50% unique
	return len(unique) < 20 || float64(len(unique))/float64(len(data)) < 0.5
}

func (s *ChiSquareSense) classifyChiSquareSignal(cramersV, pValue float64) string {
	if pValue > 0.05 {
		return "weak"
	}
	if cramersV > 0.5 {
		return "very_strong"
	}
	if cramersV > 0.3 {
		return "strong"
	}
	if cramersV > 0.1 {
		return "moderate"
	}
	return "weak"
}

func (s *ChiSquareSense) generateChiSquareDescription(cramersV, pValue, chiSquare float64) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant categorical association (V=%.3f, p=%.3f)", cramersV, pValue)
	}
	return fmt.Sprintf("Significant categorical association detected (V=%.3f, χ²=%.2f, p=%.3f)", cramersV, chiSquare, pValue)
}

// SpearmanSense detects monotonic relationships
type SpearmanSense struct{}

func NewSpearmanSense() *SpearmanSense {
	return &SpearmanSense{}
}

func (s *SpearmanSense) Name() string {
	return "spearman"
}

func (s *SpearmanSense) Description() string {
	return "Detects monotonic relationships robust to outliers and non-normality"
}

func (s *SpearmanSense) RequiresGroups() bool {
	return false
}

func (s *SpearmanSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) brief.SenseResult {
	if len(x) != len(y) || len(x) < 3 {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for Spearman correlation analysis",
		}
	}

	// Compute Spearman correlation using rank transformation
	corr, err := s.computeSpearmanCorrelation(x, y)
	if err != nil || math.IsNaN(corr) {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Unable to compute Spearman correlation",
		}
	}

	// Compute p-value using t-distribution approximation
	n := len(x)
	t := corr * math.Sqrt(float64(n-2)/(1-corr*corr))
	df := float64(n - 2)

	if df <= 0 {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  corr,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient degrees of freedom for p-value calculation",
		}
	}

	tDist := distuv.StudentsT{Mu: 0, Sigma: 1, Nu: df}
	pValue := 2 * (1 - tDist.CDF(math.Abs(t))) // Two-tailed test

	// Classify signal strength
	signal := s.classifySpearmanSignal(math.Abs(corr), pValue)

	return brief.SenseResult{
		SenseName:   s.Name(),
		EffectSize:  corr,
		PValue:      pValue,
		Confidence:  1.0 - pValue,
		Signal:      signal,
		Description: s.generateSpearmanDescription(corr, pValue),
		Metadata: map[string]interface{}{
			"t_statistic":        t,
			"degrees_of_freedom": df,
		},
	}
}

func (s *SpearmanSense) classifySpearmanSignal(absCorr, pValue float64) string {
	if pValue > 0.05 {
		return "weak"
	}
	if absCorr > 0.8 {
		return "very_strong"
	}
	if absCorr > 0.6 {
		return "strong"
	}
	if absCorr > 0.3 {
		return "moderate"
	}
	return "weak"
}

func (s *SpearmanSense) computeSpearmanCorrelation(x, y []float64) (float64, error) {
	// Rank transformation
	rankX := s.rank(x)
	rankY := s.rank(y)

	// Compute Pearson correlation on ranks
	return s.pearsonOnRanks(rankX, rankY)
}

func (s *SpearmanSense) rank(data []float64) []float64 {
	n := len(data)
	ranks := make([]float64, n)

	// Create index array
	type pair struct {
		value float64
		index int
	}
	pairs := make([]pair, n)
	for i, v := range data {
		pairs[i] = pair{value: v, index: i}
	}

	// Sort by value
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if pairs[j].value < pairs[i].value {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// Assign ranks
	for i, p := range pairs {
		ranks[p.index] = float64(i + 1)
	}

	return ranks
}

func (s *SpearmanSense) pearsonOnRanks(x, y []float64) (float64, error) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, fmt.Errorf("insufficient data")
	}

	n := float64(len(x))
	var sumX, sumY, sumXY, sumX2, sumY2 float64

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
		return 0, fmt.Errorf("zero variance")
	}

	corr := numerator / math.Sqrt(denomX*denomY)
	return corr, nil
}

func (s *SpearmanSense) generateSpearmanDescription(corr, pValue float64) string {
	direction := "positive"
	if corr < 0 {
		direction = "negative"
	}

	if pValue > 0.05 {
		return fmt.Sprintf("No significant monotonic relationship (ρ=%.3f, p=%.3f)", corr, pValue)
	}
	return fmt.Sprintf("%s monotonic relationship detected (ρ=%.3f, p=%.3f)", direction, corr, pValue)
}

// CrossCorrelationSense detects lagged relationships
type CrossCorrelationSense struct{}

func NewCrossCorrelationSense() *CrossCorrelationSense {
	return &CrossCorrelationSense{}
}

func (s *CrossCorrelationSense) Name() string {
	return "cross_correlation"
}

func (s *CrossCorrelationSense) Description() string {
	return "Detects lagged relationships between time series"
}

func (s *CrossCorrelationSense) RequiresGroups() bool {
	return false
}

func (s *CrossCorrelationSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) brief.SenseResult {
	if len(x) != len(y) || len(x) < 10 {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for cross-correlation analysis",
		}
	}

	// Find best lag using cross-correlation
	bestCorr, bestLag := s.findBestCrossCorrelation(x, y, 10) // Max lag of 10

	if math.Abs(bestCorr) < 0.1 {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  bestCorr,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "No significant cross-correlation at any lag",
			Metadata: map[string]interface{}{
				"best_lag": bestLag,
			},
		}
	}

	// Simplified p-value (in production, use proper statistical testing)
	pValue := 1.0 - math.Min(math.Abs(bestCorr), 0.99)

	signal := s.classifyCrossCorrelationSignal(bestCorr, pValue)

	return brief.SenseResult{
		SenseName:   s.Name(),
		EffectSize:  bestCorr,
		PValue:      pValue,
		Confidence:  1.0 - pValue,
		Signal:      signal,
		Description: s.generateCrossCorrelationDescription(bestCorr, bestLag, pValue),
		Metadata: map[string]interface{}{
			"best_lag":         bestLag,
			"max_lag_searched": 10,
		},
	}
}

func (s *CrossCorrelationSense) findBestCrossCorrelation(x, y []float64, maxLag int) (float64, int) {
	bestCorr := 0.0
	bestLag := 0

	for lag := -maxLag; lag <= maxLag; lag++ {
		corr := s.computeCrossCorrelation(x, y, lag)
		if math.Abs(corr) > math.Abs(bestCorr) {
			bestCorr = corr
			bestLag = lag
		}
	}

	return bestCorr, bestLag
}

func (s *CrossCorrelationSense) computeCrossCorrelation(x, y []float64, lag int) float64 {
	n := len(x)
	if lag >= n || lag <= -n {
		return 0
	}

	var sumXY, sumX, sumY, sumX2, sumY2 float64

	if lag >= 0 {
		// y is lagged behind x
		for i := lag; i < n; i++ {
			sumXY += x[i] * y[i-lag]
			sumX += x[i]
			sumY += y[i-lag]
			sumX2 += x[i] * x[i]
			sumY2 += y[i-lag] * y[i-lag]
		}
	} else {
		// x is lagged behind y
		lag = -lag
		for i := lag; i < n; i++ {
			sumXY += x[i-lag] * y[i]
			sumX += x[i-lag]
			sumY += y[i]
			sumX2 += x[i-lag] * x[i-lag]
			sumY2 += y[i] * y[i]
		}
	}

	count := float64(n - abs(lag))
	if count <= 1 {
		return 0
	}

	meanX := sumX / count
	meanY := sumY / count

	numerator := sumXY - count*meanX*meanY
	denomX := sumX2 - count*meanX*meanX
	denomY := sumY2 - count*meanY*meanY

	if denomX <= 0 || denomY <= 0 {
		return 0
	}

	return numerator / math.Sqrt(denomX*denomY)
}

func (s *CrossCorrelationSense) classifyCrossCorrelationSignal(corr, pValue float64) string {
	if pValue > 0.05 {
		return "weak"
	}
	if math.Abs(corr) > 0.7 {
		return "very_strong"
	}
	if math.Abs(corr) > 0.5 {
		return "strong"
	}
	if math.Abs(corr) > 0.3 {
		return "moderate"
	}
	return "weak"
}

func (s *CrossCorrelationSense) generateCrossCorrelationDescription(corr float64, lag int, pValue float64) string {
	direction := "leads"
	if lag < 0 {
		direction = "lags behind"
	}

	if pValue > 0.05 {
		return fmt.Sprintf("No significant cross-correlation (r=%.3f at lag %d, p=%.3f)", corr, lag, pValue)
	}
	return fmt.Sprintf("Cross-correlation shows %s relationship (r=%.3f at lag %d, p=%.3f)", direction, corr, lag, pValue)
}

// TemporalSense for time-based analysis
type TemporalSense struct {
	timeUnit string
}

func NewTemporalSense(timeUnit string) *TemporalSense {
	return &TemporalSense{timeUnit: timeUnit}
}

func (s *TemporalSense) Name() string {
	return "temporal_" + s.timeUnit
}

func (s *TemporalSense) Description() string {
	return fmt.Sprintf("Analyzes temporal patterns at %s granularity", s.timeUnit)
}

func (s *TemporalSense) RequiresGroups() bool {
	return false
}

func (s *TemporalSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) brief.SenseResult {
	return brief.SenseResult{
		SenseName:   s.Name(),
		EffectSize:  0,
		PValue:      1.0,
		Confidence:  0,
		Signal:      "weak",
		Description: "Temporal analysis requires timestamp context",
	}
}

func (s *TemporalSense) AnalyzeWithContext(ctx context.Context, x, y []float64, varX, varY core.VariableKey, senseCtx *SenseContext) brief.SenseResult {
	if senseCtx == nil || len(senseCtx.Timestamps) != len(x) {
		return brief.SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Temporal analysis requires timestamp data",
		}
	}

	// Simplified temporal analysis - detect trends over time
	trendStrength := s.computeTemporalTrend(x, senseCtx.Timestamps)

	// Simple trend test
	pValue := 1.0 - math.Min(math.Abs(trendStrength), 0.99)

	signal := "weak"
	if pValue < 0.05 {
		if math.Abs(trendStrength) > 0.7 {
			signal = "strong"
		} else if math.Abs(trendStrength) > 0.5 {
			signal = "moderate"
		}
	}

	return brief.SenseResult{
		SenseName:   s.Name(),
		EffectSize:  trendStrength,
		PValue:      pValue,
		Confidence:  1.0 - pValue,
		Signal:      signal,
		Description: s.generateTemporalDescription(trendStrength, pValue),
		Metadata: map[string]interface{}{
			"time_unit":   s.timeUnit,
			"data_points": len(senseCtx.Timestamps),
		},
	}
}

func (s *TemporalSense) computeTemporalTrend(data []float64, timestamps []time.Time) float64 {
	if len(data) != len(timestamps) || len(data) < 3 {
		return 0
	}

	// Convert timestamps to numeric time values
	timeValues := make([]float64, len(timestamps))
	minTime := timestamps[0].Unix()
	for i, ts := range timestamps {
		timeValues[i] = float64(ts.Unix() - minTime)
	}

	// Compute correlation between time and data
	corr, err := stats.Correlation(timeValues, data)
	if err != nil || math.IsNaN(corr) {
		return 0
	}

	return corr
}

func (s *TemporalSense) generateTemporalDescription(trend, pValue float64) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant temporal trend (r=%.3f, p=%.3f)", trend, pValue)
	}

	direction := "increasing"
	if trend < 0 {
		direction = "decreasing"
	}

	return fmt.Sprintf("%s temporal trend detected (r=%.3f, p=%.3f)", direction, trend, pValue)
}

// Utility function
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}


