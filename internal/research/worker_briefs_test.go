package research

import (
	"testing"

	"gohypo/domain/core"
	"gohypo/domain/stats"
)

func TestBuildDiscoveryBriefs_FromRelationshipPayload_IncludesBriefs(t *testing.T) {
	rw := &ResearchWorker{}

	statsArtifacts := []map[string]interface{}{
		{
			"kind": "relationship",
			"payload": stats.RelationshipPayload{
				VariableX:        core.VariableKey("x"),
				VariableY:        core.VariableKey("y"),
				TestType:         stats.TestPearson,
				FamilyID:         core.Hash("family_test"),
				EffectSize:       0.8,
				PValue:           0.001,
				SampleSize:       100,
				TotalComparisons: 1,
			},
		},
	}

	briefs := rw.buildDiscoveryBriefs("session-123", statsArtifacts)
	if len(briefs) == 0 {
		t.Fatalf("expected discovery briefs, got 0")
	}

	// Should include briefs for both x and y.
	seen := map[string]bool{}
	for _, b := range briefs {
		seen[string(b.VariableKey)] = true
	}
	if !seen["x"] || !seen["y"] {
		t.Fatalf("expected briefs for x and y, got keys=%v", seen)
	}
}

func TestPrepareFieldMetadata_EmbedsNonNullDiscoveryBriefs(t *testing.T) {
	rw := &ResearchWorker{}

	statsArtifacts := []map[string]interface{}{
		{
			"kind": "relationship",
			"payload": stats.RelationshipPayload{
				VariableX:        core.VariableKey("x"),
				VariableY:        core.VariableKey("y"),
				TestType:         stats.TestPearson,
				FamilyID:         core.Hash("family_test"),
				EffectSize:       0.8,
				PValue:           0.001,
				SampleSize:       100,
				TotalComparisons: 1,
			},
		},
	}

	briefs := rw.buildDiscoveryBriefs("session-123", statsArtifacts)
	jsonStr, err := rw.prepareFieldMetadata(nil, statsArtifacts, briefs)
	if err != nil {
		t.Fatalf("prepareFieldMetadata: %v", err)
	}

	// If briefs are non-empty, they must not be encoded as `null`.
	if contains(jsonStr, "\"discovery_briefs\": null") {
		t.Fatalf("expected discovery_briefs to be non-null; got JSON:\n%s", jsonStr)
	}
	if !contains(jsonStr, "\"discovery_briefs\": [") {
		t.Fatalf("expected discovery_briefs array; got JSON:\n%s", jsonStr)
	}
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
