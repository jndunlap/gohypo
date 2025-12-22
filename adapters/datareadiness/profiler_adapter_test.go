package datareadiness

import (
	"testing"

	"gohypo/adapters/datareadiness/coercer"
	"gohypo/domain/datareadiness/profiling"
)

func TestEliteTypeInference(t *testing.T) {
	coercerConfig := coercer.DefaultCoercionConfig()
	coercerInstance := coercer.NewTypeCoercer(coercerConfig)
	profiler := NewProfilerAdapter(coercerInstance)

	config := profiling.ProfilingConfig{
		SampleSize:                100,
		NumericThreshold:          0.95,
		BooleanThreshold:          0.98,
		TimestampThreshold:        0.90,
		AmbiguousNumericThreshold: 0.8,
		CategoricalUniqueRatio:    0.3,
		CategoricalIntegerRatio:   0.8,
	}

	tests := []struct {
		name          string
		values        []interface{}
		expectedType  profiling.InferredType
		expectNumeric bool
	}{
		{
			name:          "numeric strings should be numeric",
			values:        []interface{}{"25", "34", "45", "28", "52"},
			expectedType:  profiling.TypeNumeric,
			expectNumeric: true,
		},
		{
			name:          "boolean strings should be boolean",
			values:        []interface{}{"true", "false", "true", "false", "true"},
			expectedType:  profiling.TypeBoolean,
			expectNumeric: false,
		},
		{
			name:          "mixed numeric with currency should be numeric",
			values:        []interface{}{"$45000", "$78000", "$120000", "$56000", "$95000"},
			expectedType:  profiling.TypeNumeric,
			expectNumeric: true,
		},
		{
			name:          "low cardinality integers should be categorical",
			values:        []interface{}{"1", "2", "1", "2", "1", "2", "1", "2", "1", "2"},
			expectedType:  profiling.TypeCategorical,
			expectNumeric: false,
		},
		{
			name:          "text values should be categorical",
			values:        []interface{}{"North", "South", "East", "West", "North"},
			expectedType:  profiling.TypeCategorical,
			expectNumeric: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferredType, confidence := profiler.inferTypeWithConfidence(tt.values, config)

			if inferredType != tt.expectedType {
				t.Errorf("Expected type %s, got %s for values: %v", tt.expectedType, inferredType, tt.values)
			}

			if confidence <= 0 || confidence > 1 {
				t.Errorf("Confidence should be between 0 and 1, got %f", confidence)
			}

			// Check if numeric detection worked
			if tt.expectNumeric && inferredType != profiling.TypeNumeric {
				t.Errorf("Expected numeric type for numeric values, got %s", inferredType)
			}
		})
	}
}

func TestCategoricalCodeDetection(t *testing.T) {
	coercerConfig := coercer.DefaultCoercionConfig()
	coercerInstance := coercer.NewTypeCoercer(coercerConfig)
	profiler := NewProfilerAdapter(coercerInstance)

	config := profiling.ProfilingConfig{
		SampleSize:                100,
		NumericThreshold:          0.95,
		BooleanThreshold:          0.98,
		TimestampThreshold:        0.90,
		AmbiguousNumericThreshold: 0.8,
		CategoricalUniqueRatio:    0.3,
		CategoricalIntegerRatio:   0.8,
	}

	// Test data that looks like categorical codes (status codes, etc.)
	categoricalValues := []interface{}{"1", "2", "1", "2", "1", "2", "1", "2", "1", "2"}

	isCategorical := profiler.looksLikeCategoricalCodes(categoricalValues, config)
	if !isCategorical {
		t.Errorf("Expected categorical codes to be detected as categorical")
	}

	// Test data that should be continuous numeric
	continuousValues := []interface{}{"25", "34", "45", "28", "52", "67", "89", "12", "76", "43"}
	isContinuous := profiler.looksLikeCategoricalCodes(continuousValues, config)
	if isContinuous {
		t.Errorf("Expected continuous numbers to not be detected as categorical codes")
	}
}



