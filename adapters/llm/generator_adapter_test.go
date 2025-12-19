package llm

import (
	"context"
	"testing"

	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
)

func TestExtractTopRelationships(t *testing.T) {
	adapter := &GeneratorAdapter{}

	// Create test artifacts
	artifacts := []core.Artifact{
		{
			ID:   core.NewID(),
			Kind: core.ArtifactRelationship,
			Payload: stats.RelationshipPayload{
				VariableX:  core.VariableKey("inspection_count"),
				VariableY:  core.VariableKey("severity_score"),
				TestType:   stats.TestPearson,
				EffectSize: 0.67,
				PValue:     0.0001,
				SampleSize: 12847,
				FamilyID:   core.Hash("family_001"),
			},
			CreatedAt: core.Now(),
		},
		{
			ID:   core.NewID(),
			Kind: core.ArtifactRelationship,
			Payload: stats.RelationshipPayload{
				VariableX:  core.VariableKey("region"),
				VariableY:  core.VariableKey("severity_score"),
				TestType:   stats.TestANOVA,
				EffectSize: 0.34,
				PValue:     0.02,
				SampleSize: 12847,
				FamilyID:   core.Hash("family_001"),
			},
			CreatedAt: core.Now(),
		},
		{
			ID:   core.NewID(),
			Kind: core.ArtifactRelationship,
			Payload: stats.RelationshipPayload{
				VariableX:  core.VariableKey("weak_var"),
				VariableY:  core.VariableKey("other_var"),
				TestType:   stats.TestPearson,
				EffectSize: 0.05, // Too weak - should be filtered
				PValue:     0.10, // Not significant - should be filtered
				SampleSize: 1000,
				FamilyID:   core.Hash("family_001"),
			},
			CreatedAt: core.Now(),
		},
	}

	rels, _, relKeyToID, err := adapter.ExtractTopRelationships(artifacts, 10)
	if err != nil {
		t.Fatalf("ExtractTopRelationships failed: %v", err)
	}

	// Should only return 2 relationships (weak one filtered out)
	if len(rels) != 2 {
		t.Errorf("Expected 2 relationships, got %d", len(rels))
	}

	// Verify all relationships are significant
	for _, rel := range rels {
		if rel.PValue > 0.05 {
			t.Errorf("Relationship has p-value > 0.05: %f", rel.PValue)
		}
		if rel.EffectSize < 0.1 {
			t.Errorf("Relationship has effect size < 0.1: %f", rel.EffectSize)
		}
	}

	// Verify relationship keys are created
	if len(relKeyToID) == 0 {
		t.Error("Expected relationship keys to be indexed")
	}
}

func TestValidateCandidates_MissingCitations(t *testing.T) {
	adapter := &GeneratorAdapter{}

	candidates := []LLMCandidate{
		{
			CauseKey:            "inspection_count",
			EffectKey:           "severity_score",
			MechanismCategory:   "direct_causal",
			ConfounderKeys:      []string{"facility_size"},
			Rationale:           "Test rationale",
			SuggestedRigor:      "standard",
			SupportingArtifacts: []string{}, // Missing!
		},
	}

	relationships := []stats.RelationshipPayload{
		{
			VariableX: core.VariableKey("inspection_count"),
			VariableY: core.VariableKey("severity_score"),
		},
	}

	relKeyToID := make(map[string]core.ArtifactID)
	validVariables := map[core.VariableKey]bool{
		core.VariableKey("inspection_count"): true,
		core.VariableKey("severity_score"):   true,
		core.VariableKey("facility_size"):    true,
	}

	validated, dropped := adapter.ValidateCandidates(candidates, relationships, relKeyToID, validVariables)

	// Should be dropped due to missing citations
	if len(validated) != 0 {
		t.Errorf("Expected 0 validated candidates, got %d", len(validated))
	}

	// Should have dropped record
	if len(dropped) != 1 {
		t.Errorf("Expected 1 dropped record, got %d", len(dropped))
	}

	if dropped[0].Reason != "missing_citations" {
		t.Errorf("Expected reason 'missing_citations', got '%s'", dropped[0].Reason)
	}
}

func TestValidateCandidates_UnknownVariable(t *testing.T) {
	adapter := &GeneratorAdapter{}

	candidates := []LLMCandidate{
		{
			CauseKey:            "nonexistent_var", // Not in registry
			EffectKey:           "severity_score",
			MechanismCategory:   "direct_causal",
			ConfounderKeys:      []string{},
			Rationale:           "Test rationale",
			SuggestedRigor:      "standard",
			SupportingArtifacts: []string{"relationship:pearson:family_001:inspection_count:severity_score"},
		},
	}

	relationships := []stats.RelationshipPayload{}
	relKeyToID := map[string]core.ArtifactID{
		"relationship:pearson:family_001:inspection_count:severity_score": core.ArtifactID("rel_001"),
	}
	validVariables := map[core.VariableKey]bool{
		core.VariableKey("severity_score"): true,
		// nonexistent_var not in map
	}

	validated, dropped := adapter.ValidateCandidates(candidates, relationships, relKeyToID, validVariables)

	// Should be dropped due to unknown variable
	if len(validated) != 0 {
		t.Errorf("Expected 0 validated candidates, got %d", len(validated))
	}

	// Should have dropped record
	if len(dropped) != 1 {
		t.Errorf("Expected 1 dropped record, got %d", len(dropped))
	}

	if dropped[0].Reason != "invalid_cause_key" {
		t.Errorf("Expected reason 'invalid_cause_key', got '%s'", dropped[0].Reason)
	}
}

func TestGenerateHypotheses_Fallback(t *testing.T) {
	// Create mock client that returns error
	mockClient := &MockLLMClient{
		Error: context.DeadlineExceeded,
	}

	// Create fallback generator
	fallbackGen := &MockGenerator{
		Candidates: []ports.HypothesisCandidate{
			{
				CauseKey:          core.VariableKey("inspection_count"),
				EffectKey:         core.VariableKey("severity_score"),
				MechanismCategory: "direct_causal",
				Rationale:         "Fallback rationale",
				SuggestedRigor:    ports.RigorStandard,
			},
		},
	}

	adapter := &GeneratorAdapter{
		config: Config{
			FallbackToHeuristic: true,
			Timeout:             5,
		},
		llmClient:   mockClient,
		fallbackGen: fallbackGen,
	}

	req := ports.HypothesisRequest{
		Context: ports.HypothesisContext{
			RelationshipArts: []core.Artifact{
				{
					ID:   core.NewID(),
					Kind: core.ArtifactRelationship,
					Payload: stats.RelationshipPayload{
						VariableX:  core.VariableKey("inspection_count"),
						VariableY:  core.VariableKey("severity_score"),
						TestType:   stats.TestPearson,
						EffectSize: 0.67,
						PValue:     0.0001,
						SampleSize: 12847,
						FamilyID:   core.Hash("family_001"),
					},
					CreatedAt: core.Now(),
				},
			},
		},
		MaxHypotheses: 5,
		RigorProfile:  ports.RigorStandard,
	}

	gen, err := adapter.GenerateHypotheses(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateHypotheses failed: %v", err)
	}

	// Should fallback to heuristic generator
	if len(gen.Candidates) != 1 {
		t.Errorf("Expected 1 candidate from fallback, got %d", len(gen.Candidates))
	}

	if gen.Candidates[0].CauseKey != "inspection_count" {
		t.Errorf("Expected cause_key 'inspection_count', got '%s'", gen.Candidates[0].CauseKey)
	}
}

// MockGenerator implements GeneratorPort for testing
type MockGenerator struct {
	Candidates []ports.HypothesisCandidate
	Error      error
}

func (m *MockGenerator) GenerateHypotheses(ctx context.Context, req ports.HypothesisRequest) (*ports.HypothesisGeneration, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return &ports.HypothesisGeneration{
		Candidates: m.Candidates,
		Audit: ports.GenerationAudit{
			GeneratorType: "heuristic",
		},
	}, nil
}
