package senses

import (
	"context"
	"math"
	"time"

	"gohypo/domain/core"
)

// SenseContext provides optional auxiliary data for senses that need it (e.g. timestamps).
type SenseContext struct {
	Timestamps []time.Time            // Optional: one timestamp per sample
	Metadata   map[string]interface{} // Extensible auxiliary context
}

// ContextualSense is implemented by senses that can use SenseContext.
// This keeps the base StatisticalSense interface stable.
type ContextualSense interface {
	AnalyzeWithContext(ctx context.Context, x, y []float64, varX, varY core.VariableKey, senseCtx *SenseContext) SenseResult
}

// SenseResult represents the output of a single statistical sense
type SenseResult struct {
	SenseName   string                 `json:"sense_name"`
	EffectSize  float64                `json:"effect_size"`
	PValue      float64                `json:"p_value"`
	Confidence  float64                `json:"confidence"`  // 0-1 confidence score
	Signal      string                 `json:"signal"`      // "weak", "moderate", "strong", "very_strong"
	Description string                 `json:"description"` // Human-readable explanation
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// StatisticalSense defines the interface for each statistical sense
type StatisticalSense interface {
	Name() string
	Description() string
	Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) SenseResult
	RequiresGroups() bool // Some senses need group segmentation (like t-test)
}

// SenseEngine orchestrates all five statistical senses
type SenseEngine struct {
	senses []StatisticalSense
}

// NewSenseEngine creates a new statistical senses engine
func NewSenseEngine() *SenseEngine {
	return &SenseEngine{
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

// AnalyzeAll runs all five senses concurrently and returns results
func (e *SenseEngine) AnalyzeAll(ctx context.Context, x, y []float64, varX, varY core.VariableKey) []SenseResult {
	return e.AnalyzeAllWithContext(ctx, x, y, varX, varY, nil)
}

// AnalyzeAllWithContext runs all senses concurrently and passes optional SenseContext
// to senses that support it.
func (e *SenseEngine) AnalyzeAllWithContext(ctx context.Context, x, y []float64, varX, varY core.VariableKey, senseCtx *SenseContext) []SenseResult {
	results := make([]SenseResult, len(e.senses))

	// Create channels for concurrent execution
	type resultWithIndex struct {
		result SenseResult
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
		res := <-resultChan
		results[res.index] = res.result
	}

	return results
}

// AnalyzeSingle runs a specific sense by name
func (e *SenseEngine) AnalyzeSingle(ctx context.Context, senseName string, x, y []float64, varX, varY core.VariableKey) (SenseResult, bool) {
	return e.AnalyzeSingleWithContext(ctx, senseName, x, y, varX, varY, nil)
}

// AnalyzeSingleWithContext runs a specific sense by name with optional SenseContext.
func (e *SenseEngine) AnalyzeSingleWithContext(ctx context.Context, senseName string, x, y []float64, varX, varY core.VariableKey, senseCtx *SenseContext) (SenseResult, bool) {
	for _, sense := range e.senses {
		if sense.Name() == senseName {
			if cs, ok := sense.(ContextualSense); ok {
				result := cs.AnalyzeWithContext(ctx, x, y, varX, varY, senseCtx)
				return result, true
			}
			result := sense.Analyze(ctx, x, y, varX, varY)
			return result, true
		}
	}
	return SenseResult{}, false
}

// ListSenses returns all available sense names
func (e *SenseEngine) ListSenses() []string {
	names := make([]string, len(e.senses))
	for i, sense := range e.senses {
		names[i] = sense.Name()
	}
	return names
}

// Helper functions for result interpretation

// classifySignal converts effect size to signal strength
func classifySignal(effectSize float64, senseType string) string {
	absEffect := math.Abs(effectSize)

	switch senseType {
	case "mutual_information":
		if absEffect < 0.1 {
			return "weak"
		} else if absEffect < 0.3 {
			return "moderate"
		} else if absEffect < 0.5 {
			return "strong"
		}
		return "very_strong"

	case "welch_ttest", "chi_square", "spearman":
		if absEffect < 0.2 {
			return "weak"
		} else if absEffect < 0.5 {
			return "moderate"
		} else if absEffect < 0.8 {
			return "strong"
		}
		return "very_strong"

	case "cross_correlation":
		if absEffect < 0.3 {
			return "weak"
		} else if absEffect < 0.6 {
			return "moderate"
		} else if absEffect < 0.8 {
			return "strong"
		}
		return "very_strong"

	default:
		if absEffect < 0.3 {
			return "weak"
		} else if absEffect < 0.6 {
			return "moderate"
		}
		return "strong"
	}
}

// calculateConfidence converts p-value to confidence score (0-1)
func calculateConfidence(pValue float64) float64 {
	if pValue >= 1.0 {
		return 0.0
	}
	if pValue <= 0.001 {
		return 0.99
	}
	// Convert p-value to confidence: higher confidence for lower p-values
	return 1.0 - math.Log10(pValue+0.001)/3.0 // Scale to 0-1 range
}
