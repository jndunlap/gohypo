package ai

import (
	"strings"
	"testing"

	"gohypo/domain/core"
	"gohypo/domain/discovery"
)

func TestCompileResearchDirectiveFragments_AnchorsLagAndNonLinear(t *testing.T) {
	brief := discovery.NewDiscoveryBrief(core.SnapshotID("snap"), core.RunID("run"), core.VariableKey("x"))
	brief.CrossCorrelation.OptimalLag = 3
	brief.MutualInformation.MIValue = 0.25
	brief.MutualInformation.PValue = 0.01
	brief.Spearman.Correlation = 0.05
	brief.ConfidenceScore = 0.9
	brief.RiskAssessment = discovery.RiskLow

	frags := CompileResearchDirectiveFragments(*brief)

	wantAny := []string{
		"PRIORITY: Observe the 3-periods temporal delay",
		"CAUTION: Avoid linear regression-only explanations",
		"CAUTION: Relationship is likely non-monotonic",
		"EVIDENCE: Overall confidence 0.90",
	}

	joined := ""
	for _, f := range frags {
		joined += f + "\n"
	}

	for _, w := range wantAny {
		if !strings.Contains(joined, w) {
			t.Fatalf("expected fragments to include %q; got:\n%s", w, joined)
		}
	}
}
