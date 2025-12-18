package temporal

import (
	"fmt"
	"math"
	"sort"
	"time"

	"gohypo/domain/core"
)

// ============================================================================
// TEMPORAL ALIGNMENT LAYER
// ============================================================================
// This package transforms "bag of rows" into chronologically aligned time series
// suitable for lag-based causal analysis (TLCC, inactivity detection).
//
// Without temporal alignment, the engine is blind to causality.
// ============================================================================

// ResolutionInterval defines the "heartbeat" of the time series
type ResolutionInterval string

const (
	IntervalHour  ResolutionInterval = "hour"
	IntervalDay   ResolutionInterval = "day"
	IntervalWeek  ResolutionInterval = "week"
	IntervalMonth ResolutionInterval = "month"
)

// Duration returns the time.Duration for this interval
func (r ResolutionInterval) Duration() time.Duration {
	switch r {
	case IntervalHour:
		return time.Hour
	case IntervalDay:
		return 24 * time.Hour
	case IntervalWeek:
		return 7 * 24 * time.Hour
	case IntervalMonth:
		return 30 * 24 * time.Hour // Approximation
	default:
		return 24 * time.Hour
	}
}

// TimeSeries represents a single time-aligned variable
type TimeSeries struct {
	Timestamps []time.Time // The time grid (sorted, evenly spaced)
	Values     []float64   // Aligned values (same length as Timestamps)
	VarKey     core.VariableKey
	Interval   ResolutionInterval
}

// AlignedPair represents two time series on the same temporal grid
type AlignedPair struct {
	SourceSeries TimeSeries
	TargetSeries TimeSeries
	StartTime    time.Time
	EndTime      time.Time
	Interval     ResolutionInterval
	Length       int // Number of aligned time points
}

// ============================================================================
// FUNCTION 1: AlignTemporalSeries
// ============================================================================
// Takes raw event data and transforms it into two perfectly aligned time series
// with zero-padding for missing periods. This is the foundation for all lag analysis.

// EventData represents a single timestamped event with a numeric value
type EventData struct {
	Timestamp time.Time
	Value     float64
}

// AlignmentConfig controls the resampling behavior
type AlignmentConfig struct {
	Interval      ResolutionInterval
	FillMissing   FillStrategy
	AggregateFunc AggregationFunc
	MinDataPoints int     // Minimum required data points (default: 10)
	MaxGapRatio   float64 // Maximum allowed gap ratio (default: 0.5 = 50% missing)
	PadStart      bool    // Whether to pad before first observation
	PadEnd        bool    // Whether to pad after last observation
}

// FillStrategy defines how to handle missing periods
type FillStrategy string

const (
	FillZero    FillStrategy = "zero"    // Fill with 0.0
	FillForward FillStrategy = "forward" // Forward-fill last observed value
	FillMean    FillStrategy = "mean"    // Fill with series mean
	FillNaN     FillStrategy = "nan"     // Fill with NaN (math.NaN())
)

// AggregationFunc defines how to aggregate multiple events in the same period
type AggregationFunc string

const (
	AggSum   AggregationFunc = "sum"   // Sum all values in period
	AggMean  AggregationFunc = "mean"  // Average of values in period
	AggCount AggregationFunc = "count" // Count of events in period
	AggMax   AggregationFunc = "max"   // Maximum value in period
	AggMin   AggregationFunc = "min"   // Minimum value in period
)

// AlignTemporalSeries transforms raw event data into aligned time series
// This is the "Resampling Engine" that creates the chronological sequence.
func AlignTemporalSeries(
	sourceEvents, targetEvents []EventData,
	sourceKey, targetKey core.VariableKey,
	config AlignmentConfig,
) (*AlignedPair, error) {
	// Validate inputs
	if len(sourceEvents) == 0 || len(targetEvents) == 0 {
		return nil, fmt.Errorf("both source and target must have at least one event")
	}

	// Set defaults
	if config.MinDataPoints == 0 {
		config.MinDataPoints = 10
	}
	if config.MaxGapRatio == 0 {
		config.MaxGapRatio = 0.5
	}

	// Step 1: Sort both series by timestamp
	sort.Slice(sourceEvents, func(i, j int) bool {
		return sourceEvents[i].Timestamp.Before(sourceEvents[j].Timestamp)
	})
	sort.Slice(targetEvents, func(i, j int) bool {
		return targetEvents[i].Timestamp.Before(targetEvents[j].Timestamp)
	})

	// Step 2: Determine the temporal range (union of both series)
	startTime := minTime(sourceEvents[0].Timestamp, targetEvents[0].Timestamp)
	endTime := maxTime(sourceEvents[len(sourceEvents)-1].Timestamp, targetEvents[len(targetEvents)-1].Timestamp)

	// Step 3: Generate the time grid
	timeGrid := generateTimeGrid(startTime, endTime, config.Interval)

	// Validate minimum data points
	if len(timeGrid) < config.MinDataPoints {
		return nil, fmt.Errorf("insufficient time periods: got %d, need at least %d", len(timeGrid), config.MinDataPoints)
	}

	// Step 4: Resample both series to the grid
	sourceValues, sourceObserved := resampleToGrid(sourceEvents, timeGrid, config.Interval, config.AggregateFunc, config.FillMissing)
	targetValues, targetObserved := resampleToGrid(targetEvents, timeGrid, config.Interval, config.AggregateFunc, config.FillMissing)

	// Step 5: Validate gap ratio
	sourceGapRatio := gapRatioFromObserved(sourceObserved)
	targetGapRatio := gapRatioFromObserved(targetObserved)

	if sourceGapRatio > config.MaxGapRatio || targetGapRatio > config.MaxGapRatio {
		return nil, fmt.Errorf("excessive missing data: source=%.2f%%, target=%.2f%%, max=%.2f%%",
			sourceGapRatio*100, targetGapRatio*100, config.MaxGapRatio*100)
	}

	// Step 6: Create aligned pair
	startBound := timeGrid[0]
	endBound := timeGrid[len(timeGrid)-1]
	aligned := &AlignedPair{
		SourceSeries: TimeSeries{
			Timestamps: timeGrid,
			Values:     sourceValues,
			VarKey:     sourceKey,
			Interval:   config.Interval,
		},
		TargetSeries: TimeSeries{
			Timestamps: timeGrid,
			Values:     targetValues,
			VarKey:     targetKey,
			Interval:   config.Interval,
		},
		StartTime: startBound,
		EndTime:   endBound,
		Interval:  config.Interval,
		Length:    len(timeGrid),
	}

	return aligned, nil
}

// ============================================================================
// FUNCTION 2: FindCausalLead
// ============================================================================
// This is the "Lag Scanner" that shifts Variable A against Variable B to find
// the temporal offset that produces the strongest correlation signal.

// CausalLeadResult captures the lag analysis results
type CausalLeadResult struct {
	BestLag         int     // Optimal lag in periods (positive = source leads target)
	BestCorrelation float64 // Correlation coefficient at best lag
	PValue          float64 // Statistical significance of correlation
	Confidence      float64 // Confidence score (0-1)

	// Diagnostic metadata
	LagRange         int             // Maximum lag tested
	AllCorrelations  map[int]float64 // Correlation at each lag
	EffectiveSamples int             // Sample size used at best lag
	Direction        CausalDirection // Interpretation of lag direction
	Narrative        string          // Human-readable description
}

// CausalDirection interprets the lag result
type CausalDirection string

const (
	DirectionSimultaneous     CausalDirection = "simultaneous"       // Lag = 0
	DirectionSourceLeadsShort CausalDirection = "source_leads_short" // Lag 1-3 periods
	DirectionSourceLeadsLong  CausalDirection = "source_leads_long"  // Lag > 3 periods
	DirectionTargetLeads      CausalDirection = "target_leads"       // Negative lag
	DirectionNoRelationship   CausalDirection = "no_relationship"    // Weak correlation
)

// FindCausalLead performs time-lagged cross-correlation analysis
// Positive lag means source leads target (source at t predicts target at t+lag)
func FindCausalLead(source, target []float64, maxLag int) (*CausalLeadResult, error) {
	if len(source) != len(target) {
		return nil, fmt.Errorf("source and target must have equal length")
	}

	n := len(source)
	if n < 10 {
		return nil, fmt.Errorf("insufficient data points: %d (minimum 10 required)", n)
	}

	if maxLag == 0 {
		maxLag = n / 4 // Default: up to 25% of series length
	}
	if maxLag > 20 {
		maxLag = 20 // Cap at 20 periods for stability
	}
	if maxLag >= n/2 {
		maxLag = n/2 - 1 // Never use more than half the series
	}

	// Scan all lags (positive and negative)
	correlations := make(map[int]float64)
	bestLag := 0
	bestCorr := 0.0

	for lag := -maxLag; lag <= maxLag; lag++ {
		corr := computeLaggedCorrelation(source, target, lag)
		correlations[lag] = corr

		// Prefer larger absolute correlation; on ties, prefer smaller absolute lag (closest to 0),
		// and finally prefer lag=0 if still tied.
		if math.Abs(corr) > math.Abs(bestCorr) ||
			(math.Abs(corr) == math.Abs(bestCorr) && math.Abs(float64(lag)) < math.Abs(float64(bestLag))) ||
			(math.Abs(corr) == math.Abs(bestCorr) && math.Abs(float64(lag)) == math.Abs(float64(bestLag)) && lag == 0) {
			bestCorr = corr
			bestLag = lag
		}
	}

	// Calculate effective sample size at best lag
	effectiveSamples := n - int(math.Abs(float64(bestLag)))

	// Assess statistical significance
	pValue := assessCorrelationSignificance(bestCorr, effectiveSamples)
	confidence := calculateConfidence(pValue)

	// Determine causal direction
	direction := interpretCausalDirection(bestLag, bestCorr)

	// Generate narrative
	narrative := generateCausalNarrative(bestLag, bestCorr, pValue, direction)

	return &CausalLeadResult{
		BestLag:          bestLag,
		BestCorrelation:  bestCorr,
		PValue:           pValue,
		Confidence:       confidence,
		LagRange:         maxLag,
		AllCorrelations:  correlations,
		EffectiveSamples: effectiveSamples,
		Direction:        direction,
		Narrative:        narrative,
	}, nil
}

// ============================================================================
// FUNCTION 3: DetectInactivityAcceleration
// ============================================================================
// Measures whether the gaps between user actions are widening over time.
// This detects "fade-out" patterns without needing a second variable.

// InactivityResult captures the engagement decay analysis
type InactivityResult struct {
	AccelerationRate  float64 // Rate of gap widening (positive = fading out)
	TrendSignificance float64 // P-value for monotonic trend
	MeanGapDays       float64 // Average gap between events
	TrendDirection    string  // "increasing", "decreasing", "stable"

	// Diagnostic metadata
	TotalEvents int     // Number of events analyzed
	MinGapDays  float64 // Shortest gap
	MaxGapDays  float64 // Longest gap
	GapStdDev   float64 // Variability in gaps
	Narrative   string  // Human-readable description
}

// DetectInactivityAcceleration analyzes whether event gaps are widening
// Positive acceleration = user is fading out
func DetectInactivityAcceleration(timestamps []time.Time) (*InactivityResult, error) {
	if len(timestamps) < 3 {
		return nil, fmt.Errorf("need at least 3 events to detect acceleration (got %d)", len(timestamps))
	}

	// Step 1: Sort timestamps
	sortedTimes := make([]time.Time, len(timestamps))
	copy(sortedTimes, timestamps)
	sort.Slice(sortedTimes, func(i, j int) bool {
		return sortedTimes[i].Before(sortedTimes[j])
	})

	// Step 2: Calculate gaps between consecutive events (in days)
	gaps := make([]float64, len(sortedTimes)-1)
	for i := 0; i < len(sortedTimes)-1; i++ {
		duration := sortedTimes[i+1].Sub(sortedTimes[i])
		gaps[i] = duration.Hours() / 24.0 // Convert to days
	}

	// Step 3: Compute gap statistics
	meanGap := mean(gaps)
	minGap := min(gaps)
	maxGap := max(gaps)
	stdDev := standardDeviation(gaps, meanGap)

	// Step 4: Test for monotonic trend in gaps (are they getting longer?)
	// Use linear regression: gap[i] = β0 + β1 * i
	beta1, trendPValue := computeTrendSlope(gaps)

	// Step 5: Determine trend direction
	var trendDirection string
	if trendPValue > 0.1 {
		trendDirection = "stable"
	} else if beta1 > 0 {
		trendDirection = "increasing"
	} else {
		trendDirection = "decreasing"
	}

	// Step 6: Generate narrative
	narrative := generateInactivityNarrative(beta1, trendPValue, meanGap, trendDirection, len(timestamps))

	return &InactivityResult{
		AccelerationRate:  beta1,
		TrendSignificance: trendPValue,
		MeanGapDays:       meanGap,
		TrendDirection:    trendDirection,
		TotalEvents:       len(timestamps),
		MinGapDays:        minGap,
		MaxGapDays:        maxGap,
		GapStdDev:         stdDev,
		Narrative:         narrative,
	}, nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// generateTimeGrid creates evenly spaced time points
func generateTimeGrid(start, end time.Time, interval ResolutionInterval) []time.Time {
	duration := interval.Duration()
	grid := []time.Time{}

	// Truncate start to interval boundary
	current := truncateToInterval(start, interval)

	for !current.After(end) {
		grid = append(grid, current)
		current = current.Add(duration)
	}

	return grid
}

// truncateToInterval rounds time down to interval boundary
func truncateToInterval(t time.Time, interval ResolutionInterval) time.Time {
	switch interval {
	case IntervalHour:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case IntervalDay:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case IntervalWeek:
		// Round down to Monday
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		daysToSubtract := weekday - 1
		monday := t.AddDate(0, 0, -daysToSubtract)
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
	case IntervalMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return t
	}
}

// resampleToGrid aggregates events onto the time grid.
// Returns:
// - values: aligned values for each grid bucket
// - observed: true if the bucket contained ≥1 event (i.e. not an imputed fill)
func resampleToGrid(events []EventData, grid []time.Time, interval ResolutionInterval, agg AggregationFunc, fill FillStrategy) ([]float64, []bool) {
	values := make([]float64, len(grid))
	observed := make([]bool, len(grid))
	duration := interval.Duration()

	// For each grid point, aggregate events that fall in its bucket
	for i, gridTime := range grid {
		bucketStart := gridTime
		bucketEnd := gridTime.Add(duration)

		// Find events in this bucket
		bucketEvents := []float64{}
		for _, event := range events {
			if (event.Timestamp.Equal(bucketStart) || event.Timestamp.After(bucketStart)) && event.Timestamp.Before(bucketEnd) {
				bucketEvents = append(bucketEvents, event.Value)
			}
		}

		// Aggregate or fill
		if len(bucketEvents) > 0 {
			observed[i] = true
			values[i] = aggregate(bucketEvents, agg)
		} else {
			observed[i] = false
			values[i] = fillMissingValue(fill, values, observed, i)
		}
	}

	return values, observed
}

// aggregate applies the aggregation function
func aggregate(values []float64, fn AggregationFunc) float64 {
	if len(values) == 0 {
		return 0
	}

	switch fn {
	case AggSum:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	case AggMean:
		return mean(values)
	case AggCount:
		return float64(len(values))
	case AggMax:
		return max(values)
	case AggMin:
		return min(values)
	default:
		return mean(values)
	}
}

// fillMissingValue determines what to use for missing data.
// NOTE: observed indicates which prior buckets contained real events.
func fillMissingValue(strategy FillStrategy, values []float64, observed []bool, idx int) float64 {
	switch strategy {
	case FillZero:
		return 0.0
	case FillForward:
		// Forward-fill from last observed bucket (even if the value was 0.0).
		for i := idx - 1; i >= 0; i-- {
			if i < len(observed) && observed[i] {
				return values[i]
			}
		}
		return 0.0
	case FillMean:
		// Fill with mean of observed values so far (ignore prior imputed values).
		if idx == 0 {
			return 0.0
		}
		sum := 0.0
		count := 0
		for i := 0; i < idx; i++ {
			if i < len(observed) && observed[i] && !math.IsNaN(values[i]) {
				sum += values[i]
				count++
			}
		}
		if count == 0 {
			return 0.0
		}
		return sum / float64(count)
	case FillNaN:
		return math.NaN()
	default:
		return 0.0
	}
}

// gapRatioFromObserved returns the proportion of buckets that were missing (i.e. filled/imputed).
// This avoids conflating true zeros with missing data.
func gapRatioFromObserved(observed []bool) float64 {
	if len(observed) == 0 {
		return 1.0
	}
	missing := 0
	for _, ok := range observed {
		if !ok {
			missing++
		}
	}
	return float64(missing) / float64(len(observed))
}

// computeLaggedCorrelation calculates Pearson correlation at a specific lag
func computeLaggedCorrelation(source, target []float64, lag int) float64 {
	n := len(source)

	// Determine valid overlap region
	var sourceSlice, targetSlice []float64

	if lag >= 0 {
		// Positive lag: source at t correlates with target at t+lag
		if lag >= n {
			return 0
		}
		sourceSlice = source[:n-lag]
		targetSlice = target[lag:]
	} else {
		// Negative lag: source at t correlates with target at t+lag (lag is negative)
		absLag := -lag
		if absLag >= n {
			return 0
		}
		sourceSlice = source[absLag:]
		targetSlice = target[:n-absLag]
	}

	return pearsonCorrelation(sourceSlice, targetSlice)
}

// pearsonCorrelation computes the Pearson correlation coefficient
func pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	meanX := mean(x)
	meanY := mean(y)

	numerator := 0.0
	sumXX := 0.0
	sumYY := 0.0

	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	denominator := math.Sqrt(sumXX * sumYY)
	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

// assessCorrelationSignificance calculates p-value using Fisher's z-transform
func assessCorrelationSignificance(r float64, n int) float64 {
	absR := math.Abs(r)
	if absR >= 1.0 {
		return 0.0001 // Perfect correlation (likely data issue)
	}
	if absR == 0 || n < 3 {
		return 1.0
	}

	// Fisher's z-transformation
	z := 0.5 * math.Log((1+absR)/(1-absR))
	se := 1.0 / math.Sqrt(float64(n-3))
	testStat := math.Abs(z / se)

	// Two-tailed p-value using normal approximation
	pValue := 2 * (1 - 0.5*(1+math.Erf(testStat/math.Sqrt(2))))

	// Clamp to reasonable range
	if pValue < 1e-10 {
		pValue = 1e-10
	}

	return pValue
}

// calculateConfidence converts p-value to confidence score
func calculateConfidence(pValue float64) float64 {
	if pValue <= 0.001 {
		return 0.999
	}
	return 1.0 - pValue
}

// interpretCausalDirection classifies the lag pattern
func interpretCausalDirection(lag int, corr float64) CausalDirection {
	if math.Abs(corr) < 0.3 {
		return DirectionNoRelationship
	}

	if lag == 0 {
		return DirectionSimultaneous
	} else if lag > 0 {
		if lag <= 3 {
			return DirectionSourceLeadsShort
		}
		return DirectionSourceLeadsLong
	} else {
		return DirectionTargetLeads
	}
}

// generateCausalNarrative creates human-readable description
func generateCausalNarrative(lag int, corr float64, pValue float64, direction CausalDirection) string {
	if pValue > 0.05 {
		return fmt.Sprintf("No significant temporal relationship detected (r=%.3f at lag %d, p=%.3f)", corr, lag, pValue)
	}

	strength := ""
	absCorr := math.Abs(corr)
	if absCorr < 0.3 {
		strength = "weak"
	} else if absCorr < 0.6 {
		strength = "moderate"
	} else if absCorr < 0.8 {
		strength = "strong"
	} else {
		strength = "very strong"
	}

	sign := "positive"
	if corr < 0 {
		sign = "negative"
	}

	switch direction {
	case DirectionSimultaneous:
		return fmt.Sprintf("%s %s contemporaneous relationship (r=%.3f, p=%.3f)", strength, sign, corr, pValue)
	case DirectionSourceLeadsShort:
		return fmt.Sprintf("%s %s echo: source leads by %d periods (r=%.3f, p=%.3f). This suggests immediate causal impact.", strength, sign, lag, corr, pValue)
	case DirectionSourceLeadsLong:
		return fmt.Sprintf("%s %s delayed echo: source leads by %d periods (r=%.3f, p=%.3f). This suggests a longer causal pathway.", strength, sign, lag, corr, pValue)
	case DirectionTargetLeads:
		return fmt.Sprintf("%s %s reverse echo: target leads by %d periods (r=%.3f, p=%.3f). Your causal hypothesis may be backwards.", strength, sign, -lag, corr, pValue)
	default:
		return fmt.Sprintf("No clear relationship (r=%.3f)", corr)
	}
}

// generateInactivityNarrative creates human-readable engagement trend description
func generateInactivityNarrative(slope, pValue, meanGap float64, direction string, n int) string {
	if pValue > 0.1 {
		return fmt.Sprintf("Stable engagement pattern across %d events (mean gap: %.1f days, p=%.3f)", n, meanGap, pValue)
	}

	if direction == "increasing" {
		return fmt.Sprintf("User is fading out: gaps between actions are widening by %.2f days per event (mean gap: %.1f days, p=%.3f). This is a strong churn signal.", slope, meanGap, pValue)
	} else if direction == "decreasing" {
		return fmt.Sprintf("User is accelerating: gaps between actions are shrinking by %.2f days per event (mean gap: %.1f days, p=%.3f). This indicates growing engagement.", -slope, meanGap, pValue)
	}

	return fmt.Sprintf("Engagement pattern unclear across %d events", n)
}

// computeTrendSlope performs simple linear regression on gap sequence
func computeTrendSlope(gaps []float64) (float64, float64) {
	n := float64(len(gaps))
	if n < 2 {
		return 0, 1.0
	}

	// Create x values (time indices)
	xVals := make([]float64, len(gaps))
	for i := range xVals {
		xVals[i] = float64(i)
	}

	// Calculate means
	meanX := mean(xVals)
	meanY := mean(gaps)

	// Calculate slope (β1)
	numerator := 0.0
	denominator := 0.0
	for i := range gaps {
		dx := xVals[i] - meanX
		dy := gaps[i] - meanY
		numerator += dx * dy
		denominator += dx * dx
	}

	if denominator == 0 {
		return 0, 1.0
	}

	slope := numerator / denominator

	// Calculate residuals for significance test
	residuals := make([]float64, len(gaps))
	for i := range gaps {
		predicted := meanY + slope*(xVals[i]-meanX)
		residuals[i] = gaps[i] - predicted
	}

	// Simple t-test for slope significance
	residualStd := standardDeviation(residuals, 0)
	se := residualStd / math.Sqrt(denominator)

	// Perfect linear fit => extremely significant unless slope is exactly 0.
	if se == 0 {
		if slope == 0 {
			return slope, 1.0
		}
		return slope, 1e-10
	}

	tStat := math.Abs(slope / se)
	// Approximate p-value for t-distribution (df = n-2)
	pValue := 2 * (1 - 0.5*(1+math.Erf(tStat/math.Sqrt(2))))

	return slope, pValue
}

// ============================================================================
// BASIC STATISTICS
// ============================================================================

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func standardDeviation(values []float64, meanVal float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sumSquares := 0.0
	for _, v := range values {
		diff := v - meanVal
		sumSquares += diff * diff
	}

	variance := sumSquares / float64(len(values))
	return math.Sqrt(variance)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
