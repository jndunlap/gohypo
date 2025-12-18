package senses

import (
	"context"
	"fmt"
	"math"
	"time"

	"gohypo/adapters/stats/temporal"
	"gohypo/domain/core"
)

// TemporalSense is the 6th statistical sense: it detects lead/lag dynamics
// using a temporally aligned series and a lag scanner.
//
// IMPORTANT: This sense requires timestamps supplied via SenseContext.
// Without timestamps, it deterministically returns a skipped/weak result.
type TemporalSense struct {
	config temporal.AlignmentConfig
}

func NewTemporalSense(interval temporal.ResolutionInterval) *TemporalSense {
	return &TemporalSense{
		config: temporal.AlignmentConfig{
			Interval:      interval,
			FillMissing:   temporal.FillZero,
			AggregateFunc: temporal.AggSum,
			MinDataPoints: 10,
			MaxGapRatio:   0.9, // allow sparse series; callers may tighten via config later
		},
	}
}

func (s *TemporalSense) Name() string {
	return "temporal_lag"
}

func (s *TemporalSense) Description() string {
	return "Detects lead/lag dynamics by aligning series onto a time grid and scanning cross-correlation over lags"
}

func (s *TemporalSense) RequiresGroups() bool {
	return false
}

// Analyze satisfies StatisticalSense but cannot run without timestamps.
// Use SenseEngine.AnalyzeAllWithContext to provide SenseContext.
func (s *TemporalSense) Analyze(ctx context.Context, x, y []float64, varX, varY core.VariableKey) SenseResult {
	return s.AnalyzeWithContext(ctx, x, y, varX, varY, nil)
}

// AnalyzeWithContext runs temporal alignment + lag scanning when timestamps exist.
func (s *TemporalSense) AnalyzeWithContext(ctx context.Context, x, y []float64, varX, varY core.VariableKey, senseCtx *SenseContext) SenseResult {
	if len(x) != len(y) || len(x) < 3 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Insufficient data for temporal lag analysis",
			Metadata: map[string]interface{}{
				"skipped": true,
				"reason":  "length_mismatch_or_too_small",
			},
		}
	}

	if senseCtx == nil || len(senseCtx.Timestamps) == 0 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Temporal lag analysis skipped (no timestamps provided)",
			Metadata: map[string]interface{}{
				"skipped": true,
				"reason":  "missing_timestamps",
			},
		}
	}

	if len(senseCtx.Timestamps) != len(x) {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Temporal lag analysis skipped (timestamps length mismatch)",
			Metadata: map[string]interface{}{
				"skipped":             true,
				"reason":              "timestamps_length_mismatch",
				"timestamps_length":   len(senseCtx.Timestamps),
				"series_length":       len(x),
				"variable_x":          string(varX),
				"variable_y":          string(varY),
				"recommended_fix":     "provide one timestamp per sample",
				"recommended_context": "use SenseEngine.AnalyzeAllWithContext",
			},
		}
	}

	// Build event streams (skip NaN values deterministically).
	sourceEvents := make([]temporal.EventData, 0, len(x))
	targetEvents := make([]temporal.EventData, 0, len(y))
	for i := range x {
		ts := senseCtx.Timestamps[i]
		if ts.IsZero() {
			continue
		}
		if !math.IsNaN(x[i]) {
			sourceEvents = append(sourceEvents, temporal.EventData{Timestamp: ts, Value: x[i]})
		}
		if !math.IsNaN(y[i]) {
			targetEvents = append(targetEvents, temporal.EventData{Timestamp: ts, Value: y[i]})
		}
	}

	if len(sourceEvents) == 0 || len(targetEvents) == 0 {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: "Temporal lag analysis skipped (no valid timestamped samples after filtering)",
			Metadata: map[string]interface{}{
				"skipped":       true,
				"reason":        "no_valid_samples",
				"source_points": len(sourceEvents),
				"target_points": len(targetEvents),
			},
		}
	}

	aligned, err := temporal.AlignTemporalSeries(sourceEvents, targetEvents, varX, varY, s.config)
	if err != nil {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: fmt.Sprintf("Temporal alignment failed: %v", err),
			Metadata: map[string]interface{}{
				"skipped": true,
				"reason":  "alignment_failed",
				"error":   err.Error(),
			},
		}
	}

	// Default: scan up to 20% of series length (bounded) via FindCausalLead's default when maxLag=0.
	lead, err := temporal.FindCausalLead(aligned.SourceSeries.Values, aligned.TargetSeries.Values, 0)
	if err != nil {
		return SenseResult{
			SenseName:   s.Name(),
			EffectSize:  0,
			PValue:      1.0,
			Confidence:  0,
			Signal:      "weak",
			Description: fmt.Sprintf("Lag scan failed: %v", err),
			Metadata: map[string]interface{}{
				"skipped": true,
				"reason":  "lag_scan_failed",
				"error":   err.Error(),
			},
		}
	}

	dir := directionFromLag(lead.BestLag)
	period := intervalToPeriodString(aligned.Interval)

	description := fmt.Sprintf("Temporal lag detected: best_lag=%d %s, r=%.3f, p=%.3f (%s)",
		lead.BestLag, period, lead.BestCorrelation, lead.PValue, dir)

	// Signal strength uses absolute correlation.
	signal := classifySignal(math.Abs(lead.BestCorrelation), "cross_correlation")

	return SenseResult{
		SenseName:   s.Name(),
		EffectSize:  lead.BestCorrelation,
		PValue:      lead.PValue,
		Confidence:  lead.Confidence,
		Signal:      signal,
		Description: description,
		Metadata: map[string]interface{}{
			"best_lag":            lead.BestLag,
			"direction":           dir,
			"period":              period,
			"interval":            string(aligned.Interval),
			"effective_samples":   lead.EffectiveSamples,
			"lag_range_tested":    lead.LagRange,
			"alignment_start":     aligned.StartTime.Format(time.RFC3339),
			"alignment_end":       aligned.EndTime.Format(time.RFC3339),
			"aligned_points":      aligned.Length,
			"variable_x":          string(varX),
			"variable_y":          string(varY),
			"narrative":           lead.Narrative,
			"skipped":             false,
			"requires_timestamps": true,
		},
	}
}

func directionFromLag(lag int) string {
	switch {
	case lag == 0:
		return "simultaneous"
	case lag > 0:
		return "x_leads_y"
	default:
		return "y_leads_x"
	}
}

func intervalToPeriodString(interval temporal.ResolutionInterval) string {
	switch interval {
	case temporal.IntervalHour:
		return "hours"
	case temporal.IntervalDay:
		return "days"
	case temporal.IntervalWeek:
		return "weeks"
	case temporal.IntervalMonth:
		return "months"
	default:
		return "periods"
	}
}
