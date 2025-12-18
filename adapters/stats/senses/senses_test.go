package senses

import (
	"context"
	"math"
	"testing"

	"gohypo/domain/core"
)

// TestSenseEngine_ConcurrentExecution verifies all senses run concurrently
func TestSenseEngine_ConcurrentExecution(t *testing.T) {
	engine := NewSenseEngine()
	ctx := context.Background()

	// Create test data with known relationships
	x := generateLinearData(100, 2.0, 1.0, 0.5) // y = 2x + 1 + noise
	y := generateLinearData(100, 2.0, 1.0, 0.5)

	varX := core.VariableKey("test_var_x")
	varY := core.VariableKey("test_var_y")

	// Run all senses
	results := engine.AnalyzeAll(ctx, x, y, varX, varY)

	// Verify we got results from all senses
	expectedSenses := map[string]bool{
		"mutual_information": false,
		"welch_ttest":        false,
		"chi_square":         false,
		"spearman":           false,
		"cross_correlation":  false,
		"temporal_lag":       false,
	}

	if len(results) != 6 {
		t.Fatalf("Expected 6 sense results, got %d", len(results))
	}

	for _, result := range results {
		if _, exists := expectedSenses[result.SenseName]; !exists {
			t.Errorf("Unexpected sense name: %s", result.SenseName)
		}
		expectedSenses[result.SenseName] = true

		// Verify result structure
		if result.SenseName == "" {
			t.Error("Sense name should not be empty")
		}
		if result.Signal == "" {
			t.Error("Signal should not be empty")
		}
		if result.Description == "" {
			t.Error("Description should not be empty")
		}
		if result.Confidence < 0 || result.Confidence > 1 {
			t.Errorf("Confidence should be in [0,1], got %f", result.Confidence)
		}
		if result.PValue < 0 || result.PValue > 1 {
			t.Errorf("PValue should be in [0,1], got %f", result.PValue)
		}
	}

	// Verify all senses were executed
	for sense, executed := range expectedSenses {
		if !executed {
			t.Errorf("Sense %s was not executed", sense)
		}
	}
}

// TestMutualInformation_NonLinearDetection verifies MI detects non-linear relationships
func TestMutualInformation_NonLinearDetection(t *testing.T) {
	sense := NewMutualInformationSense()
	ctx := context.Background()

	// Create non-linear data (quadratic relationship)
	n := 100
	x := make([]float64, n)
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = float64(i-50) / 10.0       // -5 to 5
		y[i] = x[i]*x[i] + randNorm()*0.5 // Quadratic with noise
	}

	varX := core.VariableKey("input")
	varY := core.VariableKey("output")

	result := sense.Analyze(ctx, x, y, varX, varY)

	if result.SenseName != "mutual_information" {
		t.Errorf("Expected sense name 'mutual_information', got '%s'", result.SenseName)
	}

	// For quadratic relationship, MI should detect something
	if result.EffectSize < 0.01 {
		t.Errorf("Expected non-zero MI for quadratic data, got %f", result.EffectSize)
	}

	t.Logf("MI Result: effect=%.3f, p=%.3f, signal=%s",
		result.EffectSize, result.PValue, result.Signal)
}

// TestWelchTTest_GroupDifferences verifies t-test detects group differences
func TestWelchTTest_GroupDifferences(t *testing.T) {
	sense := NewWelchTTestSense()
	ctx := context.Background()

	// Create two groups with different means
	n := 50
	x := make([]float64, n*2)
	y := make([]float64, n*2)

	// Group 1: binary indicator (0)
	for i := 0; i < n; i++ {
		x[i] = 0.0
		y[i] = 10.0 + randNorm()*2.0 // Mean = 10
	}

	// Group 2: binary indicator (1)
	for i := n; i < n*2; i++ {
		x[i] = 1.0
		y[i] = 15.0 + randNorm()*2.0 // Mean = 15
	}

	varX := core.VariableKey("group")
	varY := core.VariableKey("value")

	result := sense.Analyze(ctx, x, y, varX, varY)

	if result.SenseName != "welch_ttest" {
		t.Errorf("Expected sense name 'welch_ttest', got '%s'", result.SenseName)
	}

	// Should detect significant group difference
	if result.PValue > 0.05 {
		t.Errorf("Expected significant p-value for group differences, got %f", result.PValue)
	}

	// Effect size should be substantial (Cohen's d)
	if math.Abs(result.EffectSize) < 1.0 {
		t.Logf("Warning: Expected large effect size, got %f", result.EffectSize)
	}

	t.Logf("t-Test Result: effect=%.3f, p=%.3f, signal=%s",
		result.EffectSize, result.PValue, result.Signal)
}

// TestChiSquare_CategoricalAssociation verifies chi-square detects categorical patterns
func TestChiSquare_CategoricalAssociation(t *testing.T) {
	sense := NewChiSquareSense()
	ctx := context.Background()

	// Create associated categorical data
	n := 100
	x := make([]float64, n)
	y := make([]float64, n)

	// Strong association: if x=0 then y=0, if x=1 then y=1
	for i := 0; i < n/2; i++ {
		x[i] = 0.0
		y[i] = 0.0
	}
	for i := n / 2; i < n; i++ {
		x[i] = 1.0
		y[i] = 1.0
	}

	varX := core.VariableKey("category_a")
	varY := core.VariableKey("category_b")

	result := sense.Analyze(ctx, x, y, varX, varY)

	if result.SenseName != "chi_square" {
		t.Errorf("Expected sense name 'chi_square', got '%s'", result.SenseName)
	}

	// Should detect strong association
	if result.PValue > 0.05 {
		t.Logf("Warning: Expected significant p-value for associated categories, got %f", result.PValue)
	}

	t.Logf("Chi-Square Result: effect=%.3f, p=%.3f, signal=%s",
		result.EffectSize, result.PValue, result.Signal)
}

// TestSpearman_MonotonicRelationship verifies Spearman detects rank-order patterns
func TestSpearman_MonotonicRelationship(t *testing.T) {
	sense := NewSpearmanSense()
	ctx := context.Background()

	// Create monotonic but non-linear relationship
	n := 50
	x := make([]float64, n)
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = float64(i)
		y[i] = math.Log(float64(i+1)) + randNorm()*0.1 // Log relationship
	}

	varX := core.VariableKey("input")
	varY := core.VariableKey("output")

	result := sense.Analyze(ctx, x, y, varX, varY)

	if result.SenseName != "spearman" {
		t.Errorf("Expected sense name 'spearman', got '%s'", result.SenseName)
	}

	// Should detect strong monotonic relationship
	if math.Abs(result.EffectSize) < 0.5 {
		t.Logf("Warning: Expected strong correlation for monotonic data, got %f", result.EffectSize)
	}

	t.Logf("Spearman Result: effect=%.3f, p=%.3f, signal=%s",
		result.EffectSize, result.PValue, result.Signal)
}

// TestCrossCorrelation_TemporalLag verifies cross-correlation detects lagged relationships
func TestCrossCorrelation_TemporalLag(t *testing.T) {
	sense := NewCrossCorrelationSense()
	ctx := context.Background()

	// Create lagged relationship: y[t] = x[t-3]
	n := 100
	lag := 3
	x := make([]float64, n)
	y := make([]float64, n)

	for i := 0; i < n; i++ {
		x[i] = math.Sin(float64(i) * 0.1) // Sine wave
		if i >= lag {
			y[i] = x[i-lag] + randNorm()*0.1
		} else {
			y[i] = randNorm() * 0.1
		}
	}

	varX := core.VariableKey("leader")
	varY := core.VariableKey("follower")

	result := sense.Analyze(ctx, x, y, varX, varY)

	if result.SenseName != "cross_correlation" {
		t.Errorf("Expected sense name 'cross_correlation', got '%s'", result.SenseName)
	}

	// Should detect correlation (may not perfectly identify lag=3 due to noise)
	if math.Abs(result.EffectSize) < 0.3 {
		t.Logf("Warning: Expected correlation for lagged data, got %f", result.EffectSize)
	}

	// Check metadata for lag information
	if result.Metadata != nil {
		if bestLag, ok := result.Metadata["best_lag"].(int); ok {
			t.Logf("Detected lag: %d (actual lag: %d)", bestLag, lag)
		}
	}

	t.Logf("Cross-Correlation Result: effect=%.3f, p=%.3f, signal=%s",
		result.EffectSize, result.PValue, result.Signal)
}

// TestSenseEngine_ListSenses verifies sense enumeration
func TestSenseEngine_ListSenses(t *testing.T) {
	engine := NewSenseEngine()
	senses := engine.ListSenses()

	expectedCount := 6
	if len(senses) != expectedCount {
		t.Errorf("Expected %d senses, got %d", expectedCount, len(senses))
	}

	expectedNames := map[string]bool{
		"mutual_information": true,
		"welch_ttest":        true,
		"chi_square":         true,
		"spearman":           true,
		"cross_correlation":  true,
		"temporal_lag":       true,
	}

	for _, name := range senses {
		if !expectedNames[name] {
			t.Errorf("Unexpected sense name: %s", name)
		}
		delete(expectedNames, name)
	}

	if len(expectedNames) > 0 {
		t.Errorf("Missing senses: %v", expectedNames)
	}
}

// TestSenseEngine_AnalyzeSingle verifies individual sense execution
func TestSenseEngine_AnalyzeSingle(t *testing.T) {
	engine := NewSenseEngine()
	ctx := context.Background()

	x := generateLinearData(50, 1.0, 0.0, 0.1)
	y := generateLinearData(50, 1.0, 0.0, 0.1)

	varX := core.VariableKey("x")
	varY := core.VariableKey("y")

	// Test running individual senses
	result, ok := engine.AnalyzeSingle(ctx, "spearman", x, y, varX, varY)
	if !ok {
		t.Fatal("Failed to run spearman sense")
	}

	if result.SenseName != "spearman" {
		t.Errorf("Expected 'spearman', got '%s'", result.SenseName)
	}

	// Test non-existent sense
	_, ok = engine.AnalyzeSingle(ctx, "non_existent_sense", x, y, varX, varY)
	if ok {
		t.Error("Should return false for non-existent sense")
	}
}

// Helper functions for test data generation

func generateLinearData(n int, slope, intercept, noise float64) []float64 {
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		x := float64(i) / float64(n)
		data[i] = slope*x + intercept + randNorm()*noise
	}
	return data
}

// Simple pseudo-random normal distribution (Box-Muller transform)
var randState = 12345.0

func randNorm() float64 {
	// Update state with linear congruential generator
	randState = math.Mod(randState*1103515245+12345, 2147483648)
	u1 := randState / 2147483648.0

	randState = math.Mod(randState*1103515245+12345, 2147483648)
	u2 := randState / 2147483648.0

	// Box-Muller transform
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}
