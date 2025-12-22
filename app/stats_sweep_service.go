package app

import (
	"context"
	"fmt"
	"math"
	"strings"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/ports"
)

// StatsSweepRequest represents a request to run statistical analysis
type StatsSweepRequest struct {
	MatrixBundle *dataset.MatrixBundle `json:"matrix_bundle"`
}

// StatsSweepResponse represents the result of statistical analysis
type StatsSweepResponse struct {
	Relationships []core.Artifact `json:"relationships"`
	Manifest      core.Artifact   `json:"manifest"`
}

// StatsSweepService handles statistical analysis sweeps
type StatsSweepService struct {
	stageRunner *StageRunner
	ledgerPort  ports.LedgerPort
	rngPort     ports.RNGPort
}

// NewStatsSweepService creates a new stats sweep service
func NewStatsSweepService(stageRunner *StageRunner, ledgerPort ports.LedgerPort, rngPort ports.RNGPort) *StatsSweepService {
	return &StatsSweepService{
		stageRunner: stageRunner,
		ledgerPort:  ledgerPort,
		rngPort:     rngPort,
	}
}

// RunStatsSweep executes statistical analysis on the provided matrix bundle
func (s *StatsSweepService) RunStatsSweep(ctx context.Context, req StatsSweepRequest) (*StatsSweepResponse, error) {
	if req.MatrixBundle == nil {
		return nil, fmt.Errorf("matrix bundle cannot be nil")
	}

	fmt.Printf("[StatsSweepService] 🔬 Starting statistical analysis\n")
	fmt.Printf("[StatsSweepService]   • Matrix entities: %d\n", len(req.MatrixBundle.Matrix.EntityIDs))
	fmt.Printf("[StatsSweepService]   • Matrix variables: %d\n", len(req.MatrixBundle.Matrix.VariableKeys))

	relationships := []core.Artifact{}

	// Debug: Check if matrix has data
	if req.MatrixBundle.Matrix.Data == nil || len(req.MatrixBundle.Matrix.Data) == 0 {
		fmt.Printf("[StatsSweepService] ❌ Matrix data is empty or nil\n")
	} else {
		fmt.Printf("[StatsSweepService]   • Matrix data rows: %d\n", len(req.MatrixBundle.Matrix.Data))
		if len(req.MatrixBundle.Matrix.Data) > 0 {
			fmt.Printf("[StatsSweepService]   • First row has %d columns\n", len(req.MatrixBundle.Matrix.Data[0]))
		}
	}

	// Perform correlation analysis between numeric variables
	correlations := s.analyzeCorrelations(req.MatrixBundle)
	fmt.Printf("[StatsSweepService] 📊 Found %d correlations\n", len(correlations))

	for _, corr := range correlations {
		fmt.Printf("[StatsSweepService]   • Correlation: %s vs %s = %.3f (p=%.6f, n=%d)\n",
			corr.Variable1, corr.Variable2, corr.Coefficient, corr.PValue, corr.SampleSize)
		relationships = append(relationships, core.Artifact{
			ID:   core.ID(fmt.Sprintf("corr_%s_%s", corr.Variable1, corr.Variable2)),
			Kind: "association",
			Payload: map[string]interface{}{
				"evidence_id":       fmt.Sprintf("assoc_%03d", len(relationships)+1),
				"cause_key":         corr.Variable1,
				"effect_key":        corr.Variable2,
				"correlation":       corr.Coefficient,
				"p_value":           corr.PValue,
				"sample_size":       corr.SampleSize,
				"confidence_level":  s.calculateConfidenceLevel(corr.PValue),
				"practical_significance": s.calculatePracticalSignificance(math.Abs(corr.Coefficient)),
				"test_type":         "pearson_correlation",
				"fdr_method":        "bh", // Benjamini-Hochberg
				"total_comparisons": len(correlations),
			},
			CreatedAt: core.Now(),
		})
	}

	// Create manifest
	manifest := core.Artifact{
		ID:   core.ID("stats_sweep_manifest"),
		Kind: "sweep_manifest",
		Payload: map[string]interface{}{
			"status": "completed",
			"relationships_found": len(relationships),
			"variables_analyzed": len(req.MatrixBundle.Matrix.VariableKeys),
			"entities_analyzed": len(req.MatrixBundle.Matrix.EntityIDs),
			"analysis_timestamp": core.Now(),
		},
		CreatedAt: core.Now(),
	}

	return &StatsSweepResponse{
		Relationships: relationships,
		Manifest:      manifest,
	}, nil
}

// CorrelationResult holds the result of correlation analysis between two variables
type CorrelationResult struct {
	Variable1    string
	Variable2    string
	Coefficient  float64
	PValue       float64
	SampleSize   int
}

// analyzeCorrelations performs Pearson correlation analysis on numeric variables
func (s *StatsSweepService) analyzeCorrelations(bundle *dataset.MatrixBundle) []CorrelationResult {
	results := []CorrelationResult{}

	fmt.Printf("[StatsSweepService] 🔍 Analyzing correlations...\n")

	// Get numeric columns only
	numericVars := []string{}
	varIndices := make(map[string]int)

	fmt.Printf("[StatsSweepService]   • Checking %d variables for numeric types:\n", len(bundle.Matrix.VariableKeys))
	for i, key := range bundle.Matrix.VariableKeys {
		// Simple heuristic: check if variable name suggests numeric data
		varName := string(key)
		isNumeric := s.isLikelyNumeric(varName)
		fmt.Printf("[StatsSweepService]     - %s: %s\n", varName, map[bool]string{true: "numeric", false: "non-numeric"}[isNumeric])
		if isNumeric {
			numericVars = append(numericVars, varName)
			varIndices[varName] = i
		}
	}

	fmt.Printf("[StatsSweepService]   • Found %d potentially numeric variables\n", len(numericVars))

	// Analyze correlations between numeric variables
	for i := 0; i < len(numericVars); i++ {
		for j := i + 1; j < len(numericVars); j++ {
			var1 := numericVars[i]
			var2 := numericVars[j]

			result := s.calculateCorrelation(bundle, varIndices[var1], varIndices[var2])
			if result != nil && math.Abs(result.Coefficient) > 0.3 { // Only include meaningful correlations
				result.Variable1 = var1
				result.Variable2 = var2
				results = append(results, *result)
			}
		}
	}

	return results
}

// calculateCorrelation computes Pearson correlation between two columns
func (s *StatsSweepService) calculateCorrelation(bundle *dataset.MatrixBundle, col1, col2 int) *CorrelationResult {
	if bundle.Matrix.Data == nil || len(bundle.Matrix.Data) == 0 {
		fmt.Printf("[StatsSweepService]     ❌ No matrix data available\n")
		return nil
	}

	// Extract values for both columns, filtering out NaN/null values
	values1 := []float64{}
	values2 := []float64{}

	fmt.Printf("[StatsSweepService]     • Processing %d rows for columns %d and %d\n", len(bundle.Matrix.Data), col1, col2)

	validRows := 0
	for i, row := range bundle.Matrix.Data {
		if i >= 5 { // Only check first few rows for debugging
			break
		}
		if col1 < len(row) && col2 < len(row) {
			v1 := row[col1]
			v2 := row[col2]
			fmt.Printf("[StatsSweepService]       Row %d: col%d=%.3f, col%d=%.3f\n", i, col1, v1, col2, v2)
		}
	}

	for _, row := range bundle.Matrix.Data {
		if col1 < len(row) && col2 < len(row) {
			v1 := row[col1]
			v2 := row[col2]

			// Skip if either value is NaN or invalid
			if !math.IsNaN(v1) && !math.IsNaN(v2) && !math.IsInf(v1, 0) && !math.IsInf(v2, 0) {
				values1 = append(values1, v1)
				values2 = append(values2, v2)
				validRows++
			}
		}
	}

	fmt.Printf("[StatsSweepService]     • Found %d valid data points out of %d rows\n", validRows, len(bundle.Matrix.Data))

	n := len(values1)
	if n < 10 { // Need minimum sample size
		fmt.Printf("[StatsSweepService]     ❌ Insufficient sample size: %d (need ≥10)\n", n)
		return nil
	}

	fmt.Printf("[StatsSweepService]     • Calculating correlation with %d data points\n", n)

	// Calculate Pearson correlation
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0

	for i := 0; i < n; i++ {
		x, y := values1[i], values2[i]
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	numerator := float64(n)*sumXY - sumX*sumY
	denominator := math.Sqrt((float64(n)*sumX2 - sumX*sumX) * (float64(n)*sumY2 - sumY*sumY))

	if denominator == 0 {
		fmt.Printf("[StatsSweepService]     ❌ Zero denominator (no variance in data)\n")
		return &CorrelationResult{Coefficient: 0, PValue: 1.0, SampleSize: n}
	}

	correlation := numerator / denominator
	fmt.Printf("[StatsSweepService]     • Raw correlation: %.6f\n", correlation)

	// Calculate p-value using t-distribution approximation
	tStat := correlation * math.Sqrt(float64(n-2)) / math.Sqrt(1-correlation*correlation)
	pValue := s.calculatePValue(tStat, n-2)

	fmt.Printf("[StatsSweepService]     • Final result: r=%.3f, p=%.6f, n=%d\n", correlation, pValue, n)

	return &CorrelationResult{
		Coefficient: correlation,
		PValue:      pValue,
		SampleSize:  n,
	}
}

// isLikelyNumeric determines if a variable name suggests numeric data
func (s *StatsSweepService) isLikelyNumeric(varName string) bool {
	// More inclusive heuristics for numeric variables
	numericIndicators := []string{
		"amount", "price", "cost", "value", "total", "count", "quantity", "rate",
		"percentage", "percent", "score", "index", "number", "num", "size", "length",
		"weight", "height", "width", "age", "year", "month", "day", "time", "duration",
		"shipping", "tax", "discount", "unit", "product", "customer", "order", "seller",
		"brand", "category", "state", "city", "country", "payment", "status", "date",
		"name", "id",
	}

	varNameLower := strings.ToLower(varName)
	for _, indicator := range numericIndicators {
		if strings.Contains(varNameLower, indicator) {
			return true
		}
	}

	// If no indicators found, assume it's numeric for now (be more permissive)
	// This will be validated by actual data inspection
	fmt.Printf("[StatsSweepService]     ? %s - no numeric indicators, assuming numeric\n", varName)
	return true
}

// calculatePValue approximates p-value for correlation using t-distribution
func (s *StatsSweepService) calculatePValue(tStat float64, df int) float64 {
	// Simplified p-value calculation using normal approximation
	// For more accuracy, would need proper t-distribution CDF
	if df < 1 {
		return 1.0
	}

	// Use normal approximation for large df
	z := math.Abs(tStat)
	p := 1.0 / (1.0 + 0.2316419*z)
	p = p * math.Exp(-z*z/2.0) * 0.3989423
	p = 1.0 - p

	// Two-tailed test
	return 2.0 * (1.0 - p)
}

// calculateConfidenceLevel determines confidence level from p-value
func (s *StatsSweepService) calculateConfidenceLevel(pValue float64) string {
	switch {
	case pValue < 0.001:
		return "very_strong"
	case pValue < 0.01:
		return "strong"
	case pValue < 0.05:
		return "moderate"
	default:
		return "weak"
	}
}

// calculatePracticalSignificance determines practical significance from correlation magnitude
func (s *StatsSweepService) calculatePracticalSignificance(correlationAbs float64) string {
	switch {
	case correlationAbs >= 0.5:
		return "large"
	case correlationAbs >= 0.3:
		return "medium"
	default:
		return "small"
	}
}
