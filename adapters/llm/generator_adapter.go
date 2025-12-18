package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// Config holds LLM adapter configuration
type Config struct {
	Model               string        // e.g., "gpt-4.1-mini"
	APIKey              string        // OpenAI API key
	BaseURL             string        // Optional override (default: https://api.openai.com/v1)
	Temperature         float64       // 0.0-1.0, lower = more deterministic
	MaxTokens           int           // Max tokens in response
	Timeout             time.Duration // Request timeout
	FallbackToHeuristic bool          // Fallback to heuristic on error
}

// GeneratorAdapter implements GeneratorPort using LLM
type GeneratorAdapter struct {
	config      Config
	llmClient   LLMClient
	fallbackGen ports.GeneratorPort
}

// LLMClient interface for LLM providers
type LLMClient interface {
	ChatCompletion(ctx context.Context, model string, prompt string, maxTokens int) (string, error)
}

// NewGeneratorAdapter creates a new LLM generator adapter
func NewGeneratorAdapter(config Config, fallbackGen ports.GeneratorPort) (*GeneratorAdapter, error) {
	client, err := newLLMClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &GeneratorAdapter{
		config:      config,
		llmClient:   client,
		fallbackGen: fallbackGen,
	}, nil
}

// relationshipWithScore holds a relationship and its score for sorting
type relationshipWithScore struct {
	Payload stats.RelationshipPayload
	Score   float64
}

// ExtractTopRelationships filters and ranks relationships by statistical strength
func (g *GeneratorAdapter) ExtractTopRelationships(
	artifacts []core.Artifact,
	maxCount int,
) ([]stats.RelationshipPayload, map[string]core.ArtifactID, error) {

	relationships := []relationshipWithScore{}
	relKeyToID := make(map[string]core.ArtifactID)

	// Extract relationships matching heuristic pattern
	for _, artifact := range artifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		var relPayload stats.RelationshipPayload
		switch p := artifact.Payload.(type) {
		case stats.RelationshipPayload:
			relPayload = p
		case map[string]interface{}:
			// Extract from map payload (JSON deserialized)
			varX, _ := p["variable_x"].(string)
			varY, _ := p["variable_y"].(string)
			testType, _ := p["test_type"].(string)
			effectSize, _ := p["effect_size"].(float64)
			pValue, _ := p["p_value"].(float64)
			sampleSize, _ := p["sample_size"].(float64)
			qValue, _ := p["q_value"].(float64)
			familyID, _ := p["family_id"].(string)

			relPayload = stats.RelationshipPayload{
				VariableX:  core.VariableKey(varX),
				VariableY:  core.VariableKey(varY),
				TestType:   stats.TestType(testType),
				EffectSize: effectSize,
				PValue:     pValue,
				QValue:     qValue,
				SampleSize: int(sampleSize),
				FamilyID:   core.Hash(familyID),
			}
		default:
			continue
		}

		// GUARDRAIL: Only statistically significant relationships
		if relPayload.PValue > 0.05 || relPayload.EffectSize < 0.1 {
			continue
		}

		// Build stable relationship key for citation lookup
		relKey := g.buildRelationshipKey(relPayload)
		relKeyToID[relKey] = core.ArtifactID(artifact.ID)

		// Score relationship (same as heuristic)
		score := g.scoreRelationship(relPayload)
		relationships = append(relationships, relationshipWithScore{
			Payload: relPayload,
			Score:   score,
		})
	}

	// Sort by score (descending)
	sort.Slice(relationships, func(i, j int) bool {
		return relationships[i].Score > relationships[j].Score
	})

	// Take top N
	if len(relationships) > maxCount {
		relationships = relationships[:maxCount]
	}

	// Extract payloads
	result := make([]stats.RelationshipPayload, len(relationships))
	for i, rel := range relationships {
		result[i] = rel.Payload
	}

	return result, relKeyToID, nil
}

// buildRelationshipKey creates stable key matching artifacts.relationshipKey() pattern
func (g *GeneratorAdapter) buildRelationshipKey(rel stats.RelationshipPayload) string {
	varX, varY := string(rel.VariableX), string(rel.VariableY)
	if varX > varY {
		varX, varY = varY, varX // Canonical ordering
	}
	return fmt.Sprintf("relationship:%s:%s:%s:%s",
		rel.TestType, rel.FamilyID, varX, varY)
}

// scoreRelationship computes composite score (same as heuristic)
func (g *GeneratorAdapter) scoreRelationship(rel stats.RelationshipPayload) float64 {
	significanceScore := 1.0 - rel.PValue
	effectScore := min(rel.EffectSize, 1.0)
	return significanceScore*0.5 + effectScore*0.5
}

// PromptData structures the input for LLM
type PromptData struct {
	Relationships []relationshipForPrompt `json:"relationships"`
	Variables     []string                `json:"variables"` // Unique variable names
	MaxHypotheses int                     `json:"max_hypotheses"`
	RigorProfile  string                  `json:"rigor_profile"` // "basic", "standard", "decision"
}

type relationshipForPrompt struct {
	VariableX  string  `json:"variable_x"`
	VariableY  string  `json:"variable_y"`
	TestType   string  `json:"test_type"`
	EffectSize float64 `json:"effect_size"`
	PValue     float64 `json:"p_value"`
	QValue     float64 `json:"q_value,omitempty"`
	SampleSize int     `json:"sample_size"`
	RelKey     string  `json:"rel_key"` // For citation
}

// BuildPrompt creates LLM prompt from relationships
func (g *GeneratorAdapter) BuildPrompt(
	relationships []stats.RelationshipPayload,
	relKeyToID map[string]core.ArtifactID,
	req ports.HypothesisRequest,
) (string, error) {

	// Collect unique variables
	varSet := make(map[core.VariableKey]bool)
	for _, rel := range relationships {
		varSet[rel.VariableX] = true
		varSet[rel.VariableY] = true
	}
	variables := make([]string, 0, len(varSet))
	for v := range varSet {
		variables = append(variables, string(v))
	}
	sort.Strings(variables) // Deterministic ordering

	// Convert relationships to prompt format
	relForPrompt := make([]relationshipForPrompt, len(relationships))
	for i, rel := range relationships {
		relKey := g.buildRelationshipKey(rel)
		relForPrompt[i] = relationshipForPrompt{
			VariableX:  string(rel.VariableX),
			VariableY:  string(rel.VariableY),
			TestType:   string(rel.TestType),
			EffectSize: rel.EffectSize,
			PValue:     rel.PValue,
			QValue:     rel.QValue,
			SampleSize: rel.SampleSize,
			RelKey:     relKey,
		}
	}

	promptData := PromptData{
		Relationships: relForPrompt,
		Variables:     variables,
		MaxHypotheses: req.MaxHypotheses,
		RigorProfile:  string(req.RigorProfile),
	}

	jsonData, err := json.MarshalIndent(promptData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal prompt data: %w", err)
	}

	prompt := fmt.Sprintf(`You are a causal inference expert generating testable hypotheses from statistical relationships.

Statistical Relationships:
%s

Domain Context:
- Variables: %s
- Total relationships analyzed: %d

Requirements:
- Generate up to %d hypotheses
- Rigor level: %s
- Each hypothesis MUST be JSON with:
  - cause_key: variable that likely causes the effect (must exist in Variables list)
  - effect_key: variable that is likely affected (must exist in Variables list)
  - mechanism_category: one of [direct_causal, effect_modification, confounding_path, proxy_relationship, measurement_bias]
  - confounder_keys: array of variable names to control for (must exist in Variables list)
  - rationale: 2-3 sentence explanation of why this might be causal
  - suggested_rigor: one of [basic, standard, decision]
  - supporting_artifacts: array of relationship keys (use "rel_key" from relationships above)

Output ONLY a JSON array of hypothesis candidates, no other text.`,
		string(jsonData),
		strings.Join(variables, ", "),
		len(relationships),
		req.MaxHypotheses,
		req.RigorProfile)

	return prompt, nil
}

// LLMCandidate is the raw LLM response structure
type LLMCandidate struct {
	CauseKey            string   `json:"cause_key"`
	EffectKey           string   `json:"effect_key"`
	MechanismCategory   string   `json:"mechanism_category"`
	ConfounderKeys      []string `json:"confounder_keys"`
	Rationale           string   `json:"rationale"`
	SuggestedRigor      string   `json:"suggested_rigor"`
	SupportingArtifacts []string `json:"supporting_artifacts"` // Relationship keys
}

// ParseCandidates parses LLM JSON response
func (g *GeneratorAdapter) ParseCandidates(jsonResponse string) ([]LLMCandidate, error) {
	// Extract JSON array from response (handle markdown code blocks)
	jsonStr := jsonResponse
	if strings.Contains(jsonStr, "```json") {
		start := strings.Index(jsonStr, "```json")
		end := strings.Index(jsonStr[start:], "```")
		if end > 0 {
			jsonStr = jsonStr[start+7 : start+end]
		}
	} else if strings.Contains(jsonStr, "```") {
		start := strings.Index(jsonStr, "```")
		end := strings.Index(jsonStr[start+3:], "```")
		if end > 0 {
			jsonStr = jsonStr[start+3 : start+3+end]
		}
	}
	jsonStr = strings.TrimSpace(jsonStr)

	var candidates []LLMCandidate
	if err := json.Unmarshal([]byte(jsonStr), &candidates); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return candidates, nil
}

// ValidateCandidates enforces guardrails
func (g *GeneratorAdapter) ValidateCandidates(
	candidates []LLMCandidate,
	relationships []stats.RelationshipPayload,
	relKeyToID map[string]core.ArtifactID,
	validVariables map[core.VariableKey]bool,
) ([]ports.HypothesisCandidate, []ports.DroppedCandidate) {

	validated := []ports.HypothesisCandidate{}
	dropped := []ports.DroppedCandidate{}

	for i, cand := range candidates {
		// GUARDRAIL 1: Citations are mandatory
		if len(cand.SupportingArtifacts) == 0 {
			dropped = append(dropped, ports.DroppedCandidate{
				CandidateIndex: i,
				Reason:         "missing_citations",
				Message:        "Candidate dropped: no supporting_artifacts provided",
			})
			continue
		}

		// GUARDRAIL 2: Variables must exist
		causeKey, err := core.ParseVariableKey(cand.CauseKey)
		if err != nil || !validVariables[causeKey] {
			dropped = append(dropped, ports.DroppedCandidate{
				CandidateIndex: i,
				Reason:         "invalid_cause_key",
				Message:        fmt.Sprintf("Candidate dropped: cause_key '%s' not in registry", cand.CauseKey),
			})
			continue
		}

		effectKey, err := core.ParseVariableKey(cand.EffectKey)
		if err != nil || !validVariables[effectKey] {
			dropped = append(dropped, ports.DroppedCandidate{
				CandidateIndex: i,
				Reason:         "invalid_effect_key",
				Message:        fmt.Sprintf("Candidate dropped: effect_key '%s' not in registry", cand.EffectKey),
			})
			continue
		}

		// Validate confounders exist
		confounderKeys := []core.VariableKey{}
		for _, confStr := range cand.ConfounderKeys {
			confKey, err := core.ParseVariableKey(confStr)
			if err != nil || !validVariables[confKey] {
				// Log but don't drop - just skip invalid confounder
				continue
			}
			confounderKeys = append(confounderKeys, confKey)
		}

		// GUARDRAIL 3: Validate mechanism category
		validMechanisms := map[string]bool{
			"direct_causal":       true,
			"effect_modification": true,
			"confounding_path":    true,
			"proxy_relationship":  true,
			"measurement_bias":    true,
		}
		if !validMechanisms[cand.MechanismCategory] {
			cand.MechanismCategory = "direct_causal" // Default
		}

		// GUARDRAIL 4: Validate rigor profile
		rigor := ports.RigorProfile(cand.SuggestedRigor)
		if rigor != ports.RigorBasic && rigor != ports.RigorStandard && rigor != ports.RigorDecision {
			rigor = ports.RigorStandard // Default
		}

		// Verify supporting artifacts exist
		supportingIDs := []core.ArtifactID{}
		for _, relKey := range cand.SupportingArtifacts {
			if artifactID, exists := relKeyToID[relKey]; exists {
				supportingIDs = append(supportingIDs, artifactID)
			}
		}

		if len(supportingIDs) == 0 {
			dropped = append(dropped, ports.DroppedCandidate{
				CandidateIndex: i,
				Reason:         "invalid_citations",
				Message:        "Candidate dropped: supporting_artifacts do not match any relationship",
			})
			continue
		}

		validated = append(validated, ports.HypothesisCandidate{
			CauseKey:            causeKey,
			EffectKey:           effectKey,
			ConfounderKeys:      confounderKeys,
			MechanismCategory:   cand.MechanismCategory,
			Rationale:           cand.Rationale,
			SuggestedRigor:      rigor,
			SupportingArtifacts: supportingIDs,
			GeneratorType:       "llm",
		})
	}

	return validated, dropped
}

// GenerateHypotheses implements GeneratorPort
func (g *GeneratorAdapter) GenerateHypotheses(
	ctx context.Context,
	req ports.HypothesisRequest,
) (*ports.HypothesisGeneration, error) {

	// GUARDRAIL: Timeout
	ctx, cancel := context.WithTimeout(ctx, g.config.Timeout)
	defer cancel()

	// Extract top relationships
	relationships, relKeyToID, err := g.ExtractTopRelationships(
		req.Context.RelationshipArts,
		req.MaxHypotheses*2, // Get 2x for filtering
	)
	if err != nil {
		return nil, fmt.Errorf("failed to extract relationships: %w", err)
	}

	if len(relationships) == 0 {
		// Fallback to heuristic if no relationships
		if g.config.FallbackToHeuristic && g.fallbackGen != nil {
			return g.fallbackGen.GenerateHypotheses(ctx, req)
		}
		return &ports.HypothesisGeneration{
			Candidates: []ports.HypothesisCandidate{},
			Audit: ports.GenerationAudit{
				GeneratorType: "llm",
				Model:         g.config.Model,
				Temperature:   g.config.Temperature,
				MaxTokens:     g.config.MaxTokens,
			},
		}, nil
	}

	// Build valid variable set from relationships
	validVariables := make(map[core.VariableKey]bool)
	for _, rel := range relationships {
		validVariables[rel.VariableX] = true
		validVariables[rel.VariableY] = true
	}

	// Build prompt
	prompt, err := g.BuildPrompt(relationships, relKeyToID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}
	promptHash := core.NewHash([]byte(prompt))

	// Call LLM
	response, err := g.llmClient.ChatCompletion(
		ctx,
		g.config.Model,
		prompt,
		g.config.MaxTokens,
	)
	if err != nil {
		// Fallback to heuristic on LLM error
		if g.config.FallbackToHeuristic && g.fallbackGen != nil {
			return g.fallbackGen.GenerateHypotheses(ctx, req)
		}
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}
	responseHash := core.NewHash([]byte(response))

	// Parse candidates
	llmCandidates, err := g.ParseCandidates(response)
	if err != nil {
		// Fallback on parse error
		if g.config.FallbackToHeuristic && g.fallbackGen != nil {
			return g.fallbackGen.GenerateHypotheses(ctx, req)
		}
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Validate candidates
	validated, dropped := g.ValidateCandidates(
		llmCandidates,
		relationships,
		relKeyToID,
		validVariables,
	)

	// Limit to MaxHypotheses
	if len(validated) > req.MaxHypotheses {
		validated = validated[:req.MaxHypotheses]
	}

	return &ports.HypothesisGeneration{
		Candidates: validated,
		Audit: ports.GenerationAudit{
			GeneratorType: "llm",
			Model:         g.config.Model,
			Temperature:   g.config.Temperature,
			MaxTokens:     g.config.MaxTokens,
			PromptHash:    promptHash,
			ResponseHash:  responseHash,
			Dropped:       dropped,
		},
	}, nil
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
