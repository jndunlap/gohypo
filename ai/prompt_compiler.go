package ai

import (
	"fmt"
	"strings"

	"gohypo/domain/discovery"
)

// CompileResearchDirectiveFragments converts a DiscoveryBrief into a set of
// high-salience prompt fragments that explicitly anchor the LLM to the math.
//
// These fragments are designed to be injected into LLM prompts (or shown to humans)
// and intentionally avoid relying on semantic meaning of variable names.
func CompileResearchDirectiveFragments(brief discovery.DiscoveryBrief) []string {
	var out []string

	// Temporal lag anchoring
	if brief.CrossCorrelation.OptimalLag != 0 {
		lag := brief.CrossCorrelation.OptimalLag
		period := "periods"
		out = append(out, fmt.Sprintf(
			"PRIORITY: Observe the %d-%s temporal delay between driver and outcome (lag=%d).",
			absInt(lag), period, lag,
		))
		if lag > 0 {
			out = append(out, "INTERPRETATION: Treat X as leading indicator of Y; test causal ordering explicitly.")
		} else {
			out = append(out, "INTERPRETATION: Treat Y as leading indicator of X; your causal direction may be reversed.")
		}
	}

	// Non-linearity anchoring (MI-driven)
	// NOTE: In this codebase, MutualInformation.NormalizedMI is often set equal to MIValue.
	mi := brief.MutualInformation.MIValue
	if mi == 0 {
		mi = brief.MutualInformation.NormalizedMI
	}
	if mi > 0.10 && brief.MutualInformation.PValue <= 0.05 {
		out = append(out, "CAUTION: Avoid linear regression-only explanations; focus on threshold/phase-shift behavior (non-linear signal present).")
	}

	// Cliff/phase shift hint: if MI is strong but monotonic signal is weak, nudge away from monotone narratives.
	if mi > 0.20 && brief.MutualInformation.PValue <= 0.05 && abs(brief.Spearman.Correlation) < 0.2 {
		out = append(out, "CAUTION: Relationship is likely non-monotonic; look for a cliff point or regime change rather than a smooth slope.")
	}

	// Hub / systemic focus
	// Use BlastRadius / centrality when available.
	if brief.BlastRadius.CentralityScore >= 0.8 || brief.BlastRadius.RadiusScore >= 0.8 {
		out = append(out, "SYSTEMIC FOCUS: This variable behaves like a hub; prioritize explaining its blast radius across the dataset.")
	}
	if n := len(brief.BlastRadius.AffectedVariables); n >= 10 {
		out = append(out, fmt.Sprintf("SYSTEMIC FOCUS: Blast radius touches %d other variables; hypothesize a shared upstream driver or instrumentation artifact.", n))
	}

	// Confidence gating
	if brief.ConfidenceScore > 0 {
		out = append(out, fmt.Sprintf("EVIDENCE: Overall confidence %.2f (risk=%s).", brief.ConfidenceScore, brief.RiskAssessment))
	}

	// Deduplicate while preserving order
	seen := make(map[string]struct{}, len(out))
	dedup := out[:0]
	for _, s := range out {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		dedup = append(dedup, s)
	}
	return dedup
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
