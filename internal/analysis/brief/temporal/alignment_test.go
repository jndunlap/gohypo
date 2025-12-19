package temporal

import (
	"math"
	"testing"
	"time"

	"gohypo/domain/core"
)

// ============================================================================
// TEST: AlignTemporalSeries
// ============================================================================

func TestAlignTemporalSeries_BasicAlignment(t *testing.T) {
	// Scenario: Two event streams with gaps that need alignment
	now := time.Now().Truncate(24 * time.Hour)

	sourceEvents := []EventData{
		{Timestamp: now.Add(0 * 24 * time.Hour), Value: 10},
		{Timestamp: now.Add(2 * 24 * time.Hour), Value: 20}, // Gap on day 1
		{Timestamp: now.Add(3 * 24 * time.Hour), Value: 15},
	}

	targetEvents := []EventData{
		{Timestamp: now.Add(0 * 24 * time.Hour), Value: 100},
		{Timestamp: now.Add(1 * 24 * time.Hour), Value: 150},
		{Timestamp: now.Add(3 * 24 * time.Hour), Value: 200}, // Gap on day 2
	}

	config := AlignmentConfig{
		Interval:      IntervalDay,
		FillMissing:   FillZero,
		AggregateFunc: AggSum,
		MinDataPoints: 3,
		MaxGapRatio:   0.5,
	}

	aligned, err := AlignTemporalSeries(sourceEvents, targetEvents, "source_var", "target_var", config)
	if err != nil {
		t.Fatalf("AlignTemporalSeries failed: %v", err)
	}

	// Validate alignment
	if aligned.Length != 4 {
		t.Errorf("Expected 4 time points, got %d", aligned.Length)
	}

	// Check zero-padding worked
	if aligned.SourceSeries.Values[1] != 0 { // Day 1 should be zero
		t.Errorf("Expected zero-padding on day 1 for source, got %.2f", aligned.SourceSeries.Values[1])
	}

	if aligned.TargetSeries.Values[2] != 0 { // Day 2 should be zero
		t.Errorf("Expected zero-padding on day 2 for target, got %.2f", aligned.TargetSeries.Values[2])
	}
}

func TestAlignTemporalSeries_Aggregation(t *testing.T) {
	// Scenario: Multiple events in the same time bucket
	now := time.Now().Truncate(24 * time.Hour)

	sourceEvents := []EventData{
		{Timestamp: now.Add(1 * time.Hour), Value: 5},
		{Timestamp: now.Add(2 * time.Hour), Value: 10},
		{Timestamp: now.Add(3 * time.Hour), Value: 15},
	}

	targetEvents := []EventData{
		{Timestamp: now.Add(1 * time.Hour), Value: 100},
	}

	config := AlignmentConfig{
		Interval:      IntervalDay,
		FillMissing:   FillZero,
		AggregateFunc: AggSum,
		MinDataPoints: 1,
		MaxGapRatio:   1.0,
	}

	aligned, err := AlignTemporalSeries(sourceEvents, targetEvents, "source_var", "target_var", config)
	if err != nil {
		t.Fatalf("AlignTemporalSeries failed: %v", err)
	}

	// All three events should be aggregated into day 0
	expectedSum := 5.0 + 10.0 + 15.0
	if aligned.SourceSeries.Values[0] != expectedSum {
		t.Errorf("Expected aggregated sum of %.2f, got %.2f", expectedSum, aligned.SourceSeries.Values[0])
	}
}

func TestAlignTemporalSeries_InsufficientData(t *testing.T) {
	now := time.Now()

	sourceEvents := []EventData{
		{Timestamp: now, Value: 10},
	}

	targetEvents := []EventData{
		{Timestamp: now, Value: 100},
	}

	config := AlignmentConfig{
		Interval:      IntervalDay,
		FillMissing:   FillZero,
		AggregateFunc: AggSum,
		MinDataPoints: 10, // Require at least 10 days
		MaxGapRatio:   0.5,
	}

	_, err := AlignTemporalSeries(sourceEvents, targetEvents, "source_var", "target_var", config)
	if err == nil {
		t.Error("Expected error for insufficient data points, got nil")
	}
}

func TestAlignTemporalSeries_ExcessiveGaps(t *testing.T) {
	now := time.Now().Truncate(24 * time.Hour)

	// Create sparse data (only 2 out of 10 days have events)
	sourceEvents := []EventData{
		{Timestamp: now, Value: 10},
		{Timestamp: now.Add(9 * 24 * time.Hour), Value: 20},
	}

	targetEvents := []EventData{
		{Timestamp: now, Value: 100},
		{Timestamp: now.Add(9 * 24 * time.Hour), Value: 200},
	}

	config := AlignmentConfig{
		Interval:      IntervalDay,
		FillMissing:   FillZero,
		AggregateFunc: AggSum,
		MinDataPoints: 5,
		MaxGapRatio:   0.3, // Allow max 30% missing
	}

	_, err := AlignTemporalSeries(sourceEvents, targetEvents, "source_var", "target_var", config)
	if err == nil {
		t.Error("Expected error for excessive gaps, got nil")
	}
}

// ============================================================================
// TEST: FindCausalLead
// ============================================================================

func TestFindCausalLead_ZeroLag(t *testing.T) {
	// Scenario: Two perfectly synchronized series
	n := 50
	source := make([]float64, n)
	target := make([]float64, n)

	for i := 0; i < n; i++ {
		source[i] = float64(i)
		target[i] = float64(i) * 2 // Perfect linear relationship, no lag
	}

	result, err := FindCausalLead(source, target, 10)
	if err != nil {
		t.Fatalf("FindCausalLead failed: %v", err)
	}

	if result.BestLag != 0 {
		t.Errorf("Expected lag=0 for synchronized series, got lag=%d", result.BestLag)
	}

	if result.Direction != DirectionSimultaneous {
		t.Errorf("Expected simultaneous direction, got %s", result.Direction)
	}

	// Correlation should be near-perfect
	if math.Abs(result.BestCorrelation) < 0.99 {
		t.Errorf("Expected near-perfect correlation, got %.3f", result.BestCorrelation)
	}
}

func TestFindCausalLead_PositiveLag(t *testing.T) {
	// Scenario: Source leads target by 3 periods
	n := 50
	source := make([]float64, n)
	target := make([]float64, n)

	// Generate source signal
	for i := 0; i < n; i++ {
		source[i] = math.Sin(float64(i) * 0.3)
	}

	// Target is source shifted by 3 periods
	for i := 0; i < n; i++ {
		if i >= 3 {
			target[i] = source[i-3]
		} else {
			target[i] = 0
		}
	}

	result, err := FindCausalLead(source, target, 10)
	if err != nil {
		t.Fatalf("FindCausalLead failed: %v", err)
	}

	// Should detect positive lag (source leads)
	if result.BestLag != 3 {
		t.Errorf("Expected lag=3, got lag=%d", result.BestLag)
	}

	if result.Direction != DirectionSourceLeadsShort {
		t.Errorf("Expected source_leads_short, got %s", result.Direction)
	}

	// Should have strong correlation at lag=3
	if math.Abs(result.BestCorrelation) < 0.9 {
		t.Errorf("Expected strong correlation at lag=3, got %.3f", result.BestCorrelation)
	}
}

func TestFindCausalLead_NegativeLag(t *testing.T) {
	// Scenario: Target leads source (negative lag)
	n := 50
	source := make([]float64, n)
	target := make([]float64, n)

	// Generate target signal
	for i := 0; i < n; i++ {
		target[i] = math.Sin(float64(i) * 0.3)
	}

	// Source is target shifted forward (target happened first)
	for i := 0; i < n; i++ {
		if i >= 2 {
			source[i] = target[i-2]
		} else {
			source[i] = 0
		}
	}

	result, err := FindCausalLead(source, target, 10)
	if err != nil {
		t.Fatalf("FindCausalLead failed: %v", err)
	}

	// Should detect negative lag (target leads)
	if result.BestLag != -2 {
		t.Errorf("Expected lag=-2, got lag=%d", result.BestLag)
	}

	if result.Direction != DirectionTargetLeads {
		t.Errorf("Expected target_leads, got %s", result.Direction)
	}
}

func TestFindCausalLead_NoRelationship(t *testing.T) {
	// Scenario: Two uncorrelated series
	n := 50
	source := make([]float64, n)
	target := make([]float64, n)

	for i := 0; i < n; i++ {
		source[i] = float64(i % 5)       // Repeating pattern
		target[i] = float64((i + 2) % 7) // Different repeating pattern
	}

	result, err := FindCausalLead(source, target, 10)
	if err != nil {
		t.Fatalf("FindCausalLead failed: %v", err)
	}

	// Should detect weak or no relationship
	if math.Abs(result.BestCorrelation) > 0.5 {
		t.Errorf("Expected weak correlation, got %.3f", result.BestCorrelation)
	}

	if result.PValue < 0.05 {
		t.Errorf("Expected non-significant p-value, got %.3f", result.PValue)
	}
}

func TestFindCausalLead_InsufficientData(t *testing.T) {
	source := []float64{1, 2, 3}
	target := []float64{4, 5, 6}

	_, err := FindCausalLead(source, target, 10)
	if err == nil {
		t.Error("Expected error for insufficient data, got nil")
	}
}

// ============================================================================
// TEST: DetectInactivityAcceleration
// ============================================================================

func TestDetectInactivityAcceleration_FadingOut(t *testing.T) {
	// Scenario: User gaps are widening (churn pattern)
	now := time.Now()
	timestamps := []time.Time{
		now,
		now.Add(1 * 24 * time.Hour),  // 1 day gap
		now.Add(3 * 24 * time.Hour),  // 2 day gap
		now.Add(6 * 24 * time.Hour),  // 3 day gap
		now.Add(10 * 24 * time.Hour), // 4 day gap
	}

	result, err := DetectInactivityAcceleration(timestamps)
	if err != nil {
		t.Fatalf("DetectInactivityAcceleration failed: %v", err)
	}

	// Acceleration rate should be positive (gaps increasing)
	if result.AccelerationRate <= 0 {
		t.Errorf("Expected positive acceleration (fading out), got %.3f", result.AccelerationRate)
	}

	if result.TrendDirection != "increasing" {
		t.Errorf("Expected increasing trend, got %s", result.TrendDirection)
	}

	// Should be statistically significant
	if result.TrendSignificance > 0.05 {
		t.Errorf("Expected significant trend, got p=%.3f", result.TrendSignificance)
	}
}

func TestDetectInactivityAcceleration_Accelerating(t *testing.T) {
	// Scenario: User gaps are shrinking (engagement increasing)
	now := time.Now()
	timestamps := []time.Time{
		now,
		now.Add(5 * 24 * time.Hour),  // 5 day gap
		now.Add(9 * 24 * time.Hour),  // 4 day gap
		now.Add(12 * 24 * time.Hour), // 3 day gap
		now.Add(14 * 24 * time.Hour), // 2 day gap
		now.Add(15 * 24 * time.Hour), // 1 day gap
	}

	result, err := DetectInactivityAcceleration(timestamps)
	if err != nil {
		t.Fatalf("DetectInactivityAcceleration failed: %v", err)
	}

	// Acceleration rate should be negative (gaps decreasing)
	if result.AccelerationRate >= 0 {
		t.Errorf("Expected negative acceleration (accelerating), got %.3f", result.AccelerationRate)
	}

	if result.TrendDirection != "decreasing" {
		t.Errorf("Expected decreasing trend, got %s", result.TrendDirection)
	}
}

func TestDetectInactivityAcceleration_StablePattern(t *testing.T) {
	// Scenario: Consistent gap pattern (no acceleration)
	now := time.Now()
	timestamps := []time.Time{
		now,
		now.Add(3 * 24 * time.Hour),
		now.Add(6 * 24 * time.Hour),
		now.Add(9 * 24 * time.Hour),
		now.Add(12 * 24 * time.Hour),
	}

	result, err := DetectInactivityAcceleration(timestamps)
	if err != nil {
		t.Fatalf("DetectInactivityAcceleration failed: %v", err)
	}

	if result.TrendDirection != "stable" {
		t.Errorf("Expected stable trend, got %s (p=%.3f)", result.TrendDirection, result.TrendSignificance)
	}

	// Gaps should be exactly 3 days
	if math.Abs(result.MeanGapDays-3.0) > 0.01 {
		t.Errorf("Expected mean gap of 3.0 days, got %.2f", result.MeanGapDays)
	}
}

func TestDetectInactivityAcceleration_InsufficientEvents(t *testing.T) {
	now := time.Now()
	timestamps := []time.Time{
		now,
		now.Add(1 * 24 * time.Hour),
	}

	_, err := DetectInactivityAcceleration(timestamps)
	if err == nil {
		t.Error("Expected error for insufficient events, got nil")
	}
}

// ============================================================================
// TEST: Helper Functions
// ============================================================================

func TestGenerateTimeGrid(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)

	grid := generateTimeGrid(start, end, IntervalDay)

	// Should generate 5 days (Jan 1-5)
	if len(grid) != 5 {
		t.Errorf("Expected 5 days, got %d", len(grid))
	}

	// First point should be Jan 1
	if !grid[0].Equal(start) {
		t.Errorf("Expected first point to be %s, got %s", start, grid[0])
	}

	// Last point should be Jan 5
	if !grid[len(grid)-1].Equal(end) {
		t.Errorf("Expected last point to be %s, got %s", end, grid[len(grid)-1])
	}
}

func TestTruncateToInterval(t *testing.T) {
	// Test day truncation
	dt := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	truncated := truncateToInterval(dt, IntervalDay)

	expected := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	if !truncated.Equal(expected) {
		t.Errorf("Expected %s, got %s", expected, truncated)
	}

	// Test hour truncation
	truncated = truncateToInterval(dt, IntervalHour)
	expected = time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC)
	if !truncated.Equal(expected) {
		t.Errorf("Expected %s, got %s", expected, truncated)
	}

	// Test month truncation
	truncated = truncateToInterval(dt, IntervalMonth)
	expected = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !truncated.Equal(expected) {
		t.Errorf("Expected %s, got %s", expected, truncated)
	}
}

func TestPearsonCorrelation(t *testing.T) {
	// Perfect positive correlation
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}

	corr := pearsonCorrelation(x, y)
	if math.Abs(corr-1.0) > 0.001 {
		t.Errorf("Expected correlation of 1.0, got %.3f", corr)
	}

	// Perfect negative correlation
	y = []float64{10, 8, 6, 4, 2}
	corr = pearsonCorrelation(x, y)
	if math.Abs(corr+1.0) > 0.001 {
		t.Errorf("Expected correlation of -1.0, got %.3f", corr)
	}

	// No correlation
	x = []float64{1, 2, 3, 4, 5}
	y = []float64{5, 3, 6, 2, 4}
	corr = pearsonCorrelation(x, y)
	if math.Abs(corr) > 0.5 {
		t.Errorf("Expected weak correlation, got %.3f", corr)
	}
}

// ============================================================================
// INTEGRATION TEST: Full Pipeline
// ============================================================================

func TestFullPipeline_SupportTicketsToUsageDrop(t *testing.T) {
	// Scenario: Support tickets lead to usage drops after 3 days
	now := time.Now().Truncate(24 * time.Hour)

	// Support tickets (cause)
	supportTickets := []EventData{
		{Timestamp: now.Add(0 * 24 * time.Hour), Value: 5},
		{Timestamp: now.Add(7 * 24 * time.Hour), Value: 8},
		{Timestamp: now.Add(14 * 24 * time.Hour), Value: 3},
		{Timestamp: now.Add(21 * 24 * time.Hour), Value: 6},
	}

	// Usage drops (effect) - appears 3 days after tickets
	usageDrops := []EventData{
		{Timestamp: now.Add(3 * 24 * time.Hour), Value: 10},
		{Timestamp: now.Add(10 * 24 * time.Hour), Value: 15},
		{Timestamp: now.Add(17 * 24 * time.Hour), Value: 7},
		{Timestamp: now.Add(24 * 24 * time.Hour), Value: 12},
	}

	// Step 1: Align series
	config := AlignmentConfig{
		Interval:      IntervalDay,
		FillMissing:   FillZero,
		AggregateFunc: AggSum,
		MinDataPoints: 10,
		MaxGapRatio:   0.9,
	}

	aligned, err := AlignTemporalSeries(
		supportTickets, usageDrops,
		core.VariableKey("support_tickets"),
		core.VariableKey("usage_drops"),
		config,
	)
	if err != nil {
		t.Fatalf("Alignment failed: %v", err)
	}

	// Step 2: Find causal lead
	result, err := FindCausalLead(aligned.SourceSeries.Values, aligned.TargetSeries.Values, 7)
	if err != nil {
		t.Fatalf("Causal lead detection failed: %v", err)
	}

	// Step 3: Validate the 3-day lag is detected
	if result.BestLag != 3 {
		t.Errorf("Expected 3-day lag between support tickets and usage drops, got %d", result.BestLag)
	}

	// Step 4: Validate narrative describes the behavioral gap
	t.Logf("Detected causal pattern: %s", result.Narrative)

	if result.Direction != DirectionSourceLeadsShort {
		t.Errorf("Expected short-term causal lead, got %s", result.Direction)
	}

	if result.PValue > 0.05 {
		t.Logf("Warning: p-value %.3f suggests weak evidence (may need more data)", result.PValue)
	}
}
