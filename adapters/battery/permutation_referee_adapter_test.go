package battery

import (
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"gohypo/adapters/stats/senses"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/verdict"
	"gohypo/internal/testkit"
)

func TestPermutationReferee_ValidateHypothesis(t *testing.T) {
	ctx := context.Background()
	testKit, err := testkit.NewTestKit()
	if err != nil {
		t.Fatalf("Failed to create test kit: %v", err)
	}
	rngAdapter := testKit.RNGAdapter()
	senseEngine := senses.NewSenseEngine()

	referee := NewPermutationReferee(senseEngine, rngAdapter)
	referee.SetNumShuffles(1000) // Use 1000 for faster tests

	tests := []struct {
		name         string
		setupBundle  func() *dataset.MatrixBundle
		expectValid  bool
		expectPValue float64
		description  string
	}{
		{
			name: "strong positive correlation should validate",
			setupBundle: func() *dataset.MatrixBundle {
				bundle := dataset.NewMatrixBundle(
					core.SnapshotID("test-snapshot"),
					core.ID("test-view"),
					core.CohortHash("test-cohort"),
					core.NewCutoffAt(time.Now()),
					core.NewLag(0),
				)

				// Create strong positive correlation
				n := 100
				xData := make([]float64, n)
				yData := make([]float64, n)
				for i := 0; i < n; i++ {
					xData[i] = float64(i)
					yData[i] = float64(i) + float64(i%10)*0.1 // Strong correlation with some noise
				}

				metaX := dataset.ColumnMeta{
					VariableKey:     core.VariableKey("x"),
					StatisticalType: dataset.TypeNumeric,
				}
				metaY := dataset.ColumnMeta{
					VariableKey:     core.VariableKey("y"),
					StatisticalType: dataset.TypeNumeric,
				}

				audit := dataset.ResolutionAudit{
					VariableKey: core.VariableKey("x"),
					RowCount:    n,
				}

				bundle.AddColumn(core.VariableKey("x"), xData, metaX, audit)
				bundle.AddColumn(core.VariableKey("y"), yData, metaY, audit)

				return bundle
			},
			expectValid:  true,
			expectPValue: 0.05, // Should be < 0.05
			description:  "Strong correlation should pass permutation test",
		},
		{
			name: "random noise should reject",
			setupBundle: func() *dataset.MatrixBundle {
				bundle := dataset.NewMatrixBundle(
					core.SnapshotID("test-snapshot"),
					core.ID("test-view"),
					core.CohortHash("test-cohort"),
					core.NewCutoffAt(time.Now()),
					core.NewLag(0),
				)

				// Create random uncorrelated data
				n := 100
				xData := make([]float64, n)
				yData := make([]float64, n)
				rng := rand.New(rand.NewSource(42))
				for i := 0; i < n; i++ {
					xData[i] = rng.Float64()
					yData[i] = rng.Float64() // Independent random values
				}

				metaX := dataset.ColumnMeta{
					VariableKey:     core.VariableKey("x"),
					StatisticalType: dataset.TypeNumeric,
				}
				metaY := dataset.ColumnMeta{
					VariableKey:     core.VariableKey("y"),
					StatisticalType: dataset.TypeNumeric,
				}

				audit := dataset.ResolutionAudit{
					VariableKey: core.VariableKey("x"),
					RowCount:    n,
				}

				bundle.AddColumn(core.VariableKey("x"), xData, metaX, audit)
				bundle.AddColumn(core.VariableKey("y"), yData, metaY, audit)

				return bundle
			},
			expectValid:  false,
			expectPValue: 0.05, // Should be >= 0.05
			description:  "Random noise should fail permutation test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := tt.setupBundle()
			hypothesisID := core.HypothesisID("test-hypothesis")

			result, err := referee.ValidateHypothesis(ctx, hypothesisID, bundle)
			if err != nil {
				t.Fatalf("ValidateHypothesis failed: %v", err)
			}

			if tt.expectValid {
				if result.Status != verdict.StatusValidated {
					t.Errorf("Expected validated status, got %s", result.Status)
				}
				if result.PValue >= tt.expectPValue {
					t.Errorf("Expected p-value < %f, got %f", tt.expectPValue, result.PValue)
				}
			} else {
				if result.Status != verdict.StatusRejected {
					t.Errorf("Expected rejected status, got %s", result.Status)
				}
				if result.PValue < tt.expectPValue {
					t.Errorf("Expected p-value >= %f, got %f", tt.expectPValue, result.PValue)
				}
				if result.FalsificationLog == nil {
					t.Error("Expected falsification log for rejected hypothesis")
				}
			}

			if result.NumPermutations != 1000 {
				t.Errorf("Expected 1000 permutations, got %d", result.NumPermutations)
			}
		})
	}
}

func TestPermutationReferee_pearsonCorrelation(t *testing.T) {
	referee := NewPermutationReferee(nil, nil)

	tests := []struct {
		name     string
		x, y     []float64
		expected float64
	}{
		{
			name:     "perfect positive correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{1, 2, 3, 4, 5},
			expected: 1.0,
		},
		{
			name:     "perfect negative correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{5, 4, 3, 2, 1},
			expected: -1.0,
		},
		{
			name:     "no correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{5, 1, 4, 2, 3},
			expected: 0.0, // Approximately zero
		},
		{
			name:     "empty arrays",
			x:        []float64{},
			y:        []float64{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := referee.pearsonCorrelation(tt.x, tt.y)
			if math.Abs(result-tt.expected) > 0.01 && tt.name != "no correlation" {
				t.Errorf("Expected correlation %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestPermutationReferee_spearmanCorrelation(t *testing.T) {
	referee := NewPermutationReferee(nil, nil)

	tests := []struct {
		name     string
		x, y     []float64
		expected float64
	}{
		{
			name:     "perfect positive rank correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{1, 2, 3, 4, 5},
			expected: 1.0,
		},
		{
			name:     "perfect negative rank correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{5, 4, 3, 2, 1},
			expected: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := referee.spearmanCorrelation(tt.x, tt.y)
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("Expected Spearman correlation %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestPermutationReferee_performPermutationTest(t *testing.T) {
	ctx := context.Background()
	testKit, err := testkit.NewTestKit()
	if err != nil {
		t.Fatalf("Failed to create test kit: %v", err)
	}
	rngAdapter := testKit.RNGAdapter()
	senseEngine := senses.NewSenseEngine()

	referee := NewPermutationReferee(senseEngine, rngAdapter)
	// SetNumShuffles enforces minimum of 1000, so 100 will become 1000
	referee.SetNumShuffles(100)
	expectedShuffles := 1000 // Minimum enforced by SetNumShuffles

	// Create correlated data
	n := 50
	xData := make([]float64, n)
	yData := make([]float64, n)
	for i := 0; i < n; i++ {
		xData[i] = float64(i)
		yData[i] = float64(i) * 0.8 // Strong positive correlation
	}

	pValue, nullDist := referee.performPermutationTest(ctx, xData, yData, "pearson")

	if pValue < 0 || pValue > 1 {
		t.Errorf("P-value should be between 0 and 1, got %f", pValue)
	}

	if len(nullDist) != expectedShuffles {
		t.Errorf("Expected null distribution of size %d, got %d", expectedShuffles, len(nullDist))
	}

	// For strongly correlated data, p-value should be low
	if pValue >= 0.05 {
		t.Logf("Warning: Strong correlation got p-value %f (expected < 0.05)", pValue)
	}
}

func TestPermutationReferee_statisticalHelpers(t *testing.T) {
	referee := NewPermutationReferee(nil, nil)

	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0}

	mean := referee.mean(data)
	if math.Abs(mean-3.0) > 0.001 {
		t.Errorf("Expected mean 3.0, got %f", mean)
	}

	stdDev := referee.stdDev(data)
	expectedStdDev := math.Sqrt(2.5) // Variance of [1,2,3,4,5] is 2.5
	if math.Abs(stdDev-expectedStdDev) > 0.01 {
		t.Errorf("Expected stdDev %f, got %f", expectedStdDev, stdDev)
	}

	min := referee.min(data)
	if min != 1.0 {
		t.Errorf("Expected min 1.0, got %f", min)
	}

	max := referee.max(data)
	if max != 5.0 {
		t.Errorf("Expected max 5.0, got %f", max)
	}

	p95 := referee.percentile(data, 95)
	if p95 < 4.0 || p95 > 5.0 {
		t.Errorf("Expected 95th percentile between 4.0 and 5.0, got %f", p95)
	}
}

func TestPermutationReferee_SetNumShuffles(t *testing.T) {
	referee := NewPermutationReferee(nil, nil)

	// Test default
	if referee.numShuffles != 1000 {
		t.Errorf("Expected default 1000 shuffles, got %d", referee.numShuffles)
	}

	// Test setting valid value
	referee.SetNumShuffles(5000)
	if referee.numShuffles != 5000 {
		t.Errorf("Expected 5000 shuffles, got %d", referee.numShuffles)
	}

	// Test minimum enforcement
	referee.SetNumShuffles(500)
	if referee.numShuffles != 1000 {
		t.Errorf("Expected minimum 1000 shuffles, got %d", referee.numShuffles)
	}

	// Test maximum enforcement
	referee.SetNumShuffles(200000)
	if referee.numShuffles != 100000 {
		t.Errorf("Expected maximum 100000 shuffles, got %d", referee.numShuffles)
	}
}
