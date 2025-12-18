package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// determineVerdict returns a verdict ONLY if we have sufficient evidence (q-value + N)
// Returns nil if insufficient data
func determineVerdict(pValue, qValue, effectSize float64, sampleSize int, hasQValue bool) map[string]interface{} {
	// Gate: No verdict if missing critical metrics
	if sampleSize == 0 {
		return map[string]interface{}{
			"Status":      "INSUFFICIENT_DATA",
			"Title":       "Not enough data",
			"Explanation": fmt.Sprintf("Sample size missing (N=0). Cannot compute verdict."),
			"Reason":      "missing_sample_size",
		}
	}

	if !hasQValue && qValue == 0 {
		return map[string]interface{}{
			"Status":      "NOT_RUN",
			"Title":       "FDR correction not run",
			"Explanation": fmt.Sprintf("q-value not computed (FDR stage not run). p-value: %.4f, N=%d", pValue, sampleSize),
			"Reason":      "missing_q_value",
		}
	}

	// Only show verdict if we have q-value and sufficient N
	if !hasQValue || sampleSize < 50 {
		return map[string]interface{}{
			"Status":      "INSUFFICIENT_DATA",
			"Title":       "Insufficient data",
			"Explanation": fmt.Sprintf("N=%d (minimum is 50). Cannot determine verdict.", sampleSize),
			"Reason":      "low_sample_size",
		}
	}

	// Now we can compute a verdict
	var status, title, explanation string

	if qValue <= 0.01 && sampleSize >= 200 {
		status = "PASS"
		title = "Strong association"
		explanation = fmt.Sprintf("q=%.4f (FDR), N=%d", qValue, sampleSize)
	} else if qValue <= 0.05 && sampleSize >= 100 {
		status = "PASS"
		title = "Moderate association"
		explanation = fmt.Sprintf("q=%.4f (FDR), N=%d", qValue, sampleSize)
	} else if qValue <= 0.05 {
		status = "INCONCLUSIVE"
		title = "Weak evidence"
		explanation = fmt.Sprintf("q=%.4f but N=%d is low", qValue, sampleSize)
	} else {
		status = "FAIL"
		title = "No significant association"
		explanation = fmt.Sprintf("q=%.4f (not significant), N=%d", qValue, sampleSize)
	}

	return map[string]interface{}{
		"Status":      status,
		"Title":       title,
		"Explanation": explanation,
		"QValue":      qValue,
		"SampleSize":  sampleSize,
		"PValue":      pValue,
		"EffectSize":  effectSize,
	}
}

// intFromAny converts various numeric types to int
func intFromAny(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	case string:
		// Best-effort parse.
		if t == "" {
			return 0
		}
		if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return i
		}
	}
	return 0
}

// pointer returns a pointer to the given value
func pointer[T any](v T) *T {
	return &v
}
