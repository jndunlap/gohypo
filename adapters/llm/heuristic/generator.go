package heuristic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// Generator creates hypotheses using algorithmic rules from relationship artifacts
type Generator struct{}

// NewGenerator creates a new heuristic hypothesis generator
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateHypotheses creates hypothesis candidates from statistical relationships
func (g *Generator) GenerateHypotheses(ctx context.Context, req ports.HypothesisRequest) (*ports.HypothesisGeneration, error) {
	candidates := []ports.HypothesisCandidate{}

	// Extract and rank relationships by statistical strength
	relationships := g.extractRelationships(req.Context.RelationshipArts)
	sort.Slice(relationships, func(i, j int) bool {
		return g.scoreRelationship(relationships[i]) > g.scoreRelationship(relationships[j])
	})

	// Take top relationships for hypothesis generation
	topRelationships := relationships
	if len(relationships) > req.MaxHypotheses*2 {
		topRelationships = relationships[:req.MaxHypotheses*2]
	}

	// Generate hypotheses from each strong relationship
	for _, rel := range topRelationships {
		if len(candidates) >= req.MaxHypotheses {
			break
		}

		// Only generate hypotheses from statistically significant relationships
		if rel.PValue > 0.05 || rel.EffectSize < 0.1 {
			continue
		}

		hypothesis := g.generateHypothesisFromRelationship(rel, req.RigorProfile)
		candidates = append(candidates, hypothesis)
	}

	return &ports.HypothesisGeneration{
		Candidates: candidates,
		Audit: ports.GenerationAudit{
			GeneratorType: "heuristic",
		},
	}, nil
}

// extractRelationships pulls relationship data from artifacts
func (g *Generator) extractRelationships(artifacts []core.Artifact) []Relationship {
	relationships := []Relationship{}

	for _, artifact := range artifacts {
		if artifact.Kind == core.ArtifactRelationship {
			var rel Relationship

			// Handle both struct (direct) and map (deserialized) payloads
			switch p := artifact.Payload.(type) {
			case stats.RelationshipPayload:
				rel = Relationship{
					VariableX:        p.VariableX,
					VariableY:        p.VariableY,
					TestUsed:         string(p.TestType),
					EffectSize:       p.EffectSize,
					PValue:           p.PValue,
					PermutationP:     p.PValue, // Fallback if missing
					StabilityScore:   0.5,      // Default
					PhantomBenchmark: 0.0,      // Default
					CohortSize:       p.SampleSize,
					ArtifactID:       core.ArtifactID(artifact.ID),
				}
			case map[string]interface{}:
				// Legacy/JSON map support
				vx, _ := p["variable_x"].(string)
				vy, _ := p["variable_y"].(string)
				test, _ := p["test_used"].(string)
				eff, _ := p["effect_size"].(float64)
				pval, _ := p["p_value"].(float64)
				size, _ := p["sample_size"].(float64) // JSON numbers are float64

				// Safe extraction with defaults
				rel = Relationship{
					VariableX:        core.VariableKey(vx),
					VariableY:        core.VariableKey(vy),
					TestUsed:         test,
					EffectSize:       eff,
					PValue:           pval,
					PermutationP:     g.getFloat(p, "permutation_p", pval),
					StabilityScore:   g.getFloat(p, "stability_score", 0.5),
					PhantomBenchmark: g.getFloat(p, "phantom_benchmark", 0.0),
					CohortSize:       int(size),
					ArtifactID:       core.ArtifactID(artifact.ID),
				}
			default:
				continue
			}

			relationships = append(relationships, rel)
		}
	}

	return relationships
}

// scoreRelationship provides a composite score for relationship strength
func (g *Generator) scoreRelationship(rel Relationship) float64 {
	// Combine multiple statistical indicators
	significanceScore := 1.0 - rel.PValue   // Higher for more significant
	stabilityScore := rel.StabilityScore    // Higher for more stable
	effectScore := min(rel.EffectSize, 1.0) // Cap at 1.0

	// Weight the components
	return significanceScore*0.5 + stabilityScore*0.3 + effectScore*0.2
}

// generateHypothesisFromRelationship creates a structured hypothesis
func (g *Generator) generateHypothesisFromRelationship(rel Relationship, rigor ports.RigorProfile) ports.HypothesisCandidate {
	// Infer direction: stronger variable is typically the cause
	var cause, effect core.VariableKey
	if g.shouldReverseDirection(rel) {
		cause, effect = rel.VariableY, rel.VariableX
	} else {
		cause, effect = rel.VariableX, rel.VariableY
	}

	// Generate mechanism based on variable names and relationship
	mechanism := g.inferMechanism(rel)
	confounders := g.suggestConfounders(rel)

	// Convert string slice to VariableKey slice
	confounderKeys := make([]core.VariableKey, len(confounders))
	for i, conf := range confounders {
		confounderKeys[i] = core.VariableKey(conf)
	}

	return ports.HypothesisCandidate{
		CauseKey:          cause,
		EffectKey:         effect,
		ConfounderKeys:    confounderKeys,
		MechanismCategory: mechanism,
		Rationale:         g.generateRationale(rel, mechanism),
		SuggestedRigor:    rigor,
	}
}

// shouldReverseDirection decides if relationship should be reversed
func (g *Generator) shouldReverseDirection(rel Relationship) bool {
	// Simple heuristics based on variable naming
	causeIndicators := []string{"count", "frequency", "rate", "size", "age", "time"}
	effectIndicators := []string{"score", "severity", "risk", "outcome", "result"}

	causeX := g.containsAny(string(rel.VariableX), causeIndicators)
	effectY := g.containsAny(string(rel.VariableY), effectIndicators)

	return causeX && effectY
}

// inferMechanism generates a mechanism category
func (g *Generator) inferMechanism(rel Relationship) string {
	// Rule-based mechanism inference
	if rel.EffectSize > 0.7 && rel.StabilityScore > 0.8 {
		return "direct_causal"
	}
	if rel.TestUsed == "ttest" && rel.EffectSize > 0.4 {
		return "effect_modification"
	}
	if rel.PValue < 0.01 && rel.StabilityScore < 0.6 {
		return "confounding_path"
	}
	if rel.PhantomBenchmark > 0.05 {
		return "proxy_relationship"
	}

	return "measurement_bias"
}

// suggestConfounders provides confounder suggestions
func (g *Generator) suggestConfounders(rel Relationship) []string {
	confounders := []string{}

	// Always suggest defaults unless specific confounders make sense
	confounders = append(confounders, "use_defaults")

	// Add domain-specific confounders based on variable names
	if g.containsAny(string(rel.VariableX), []string{"inspection", "audit", "check"}) ||
		g.containsAny(string(rel.VariableY), []string{"severity", "violation", "risk"}) {
		confounders = append(confounders, "size_proxy", "quality_proxy")
	}

	if g.containsAny(string(rel.VariableX), []string{"count", "frequency"}) {
		confounders = append(confounders, "time_proxy")
	}

	return confounders
}

// generateRationale creates explanatory text for the hypothesis
func (g *Generator) generateRationale(rel Relationship, mechanism string) string {
	switch mechanism {
	case "direct_causal":
		return fmt.Sprintf("Strong statistical relationship (effect=%.2f, p=%.4f) suggests direct causation between %s and %s",
			rel.EffectSize, rel.PValue, rel.VariableX, rel.VariableY)
	case "effect_modification":
		return fmt.Sprintf("%s appears to modify the effect of %s (standardized difference: %.2f)",
			rel.VariableX, rel.VariableY, rel.EffectSize)
	case "proxy_relationship":
		return fmt.Sprintf("%s may serve as a proxy indicator for factors affecting %s (correlation: %.2f)",
			rel.VariableX, rel.VariableY, rel.EffectSize)
	case "confounding_path":
		return fmt.Sprintf("Statistical association between %s and %s requires investigation for confounding factors",
			rel.VariableX, rel.VariableY)
	default:
		return fmt.Sprintf("Detected relationship between %s and %s (effect: %.2f) warrants further study",
			rel.VariableX, rel.VariableY, rel.EffectSize)
	}
}

// containsAny checks if string contains any of the substrings
func (g *Generator) containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// getFloat safely extracts float from map
func (g *Generator) getFloat(m map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := m[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return defaultValue
}

// Relationship represents extracted relationship data
type Relationship struct {
	VariableX        core.VariableKey
	VariableY        core.VariableKey
	TestUsed         string
	EffectSize       float64
	PValue           float64
	PermutationP     float64
	StabilityScore   float64
	PhantomBenchmark float64
	CohortSize       int
	ArtifactID       core.ArtifactID
}
