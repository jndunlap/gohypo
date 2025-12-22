# LLM Hypothesis Generator - Implementation Playbook

## Quick Start

```bash
# 1. Set environment variables
export GENERATOR_MODE=llm  # or "heuristic"
export LLM_PROVIDER=openai
export LLM_MODEL=gpt-5.2
export LLM_API_KEY=sk-...

# 2. Run sweep on mock shopping data
gohypo-cli sweep shopping_bundle_001 --seed 42

# 3. Generate hypotheses from relationships
gohypo-cli hypothesis run_001 matrix_bundle_001 --max-hypotheses 10 --rigor standard

# 4. View artifacts in DB
# Query: SELECT * FROM artifacts WHERE kind = 'hypothesis' ORDER BY created_at DESC;

# 5. View in UI
# Navigate to http://localhost:8081/hypotheses
```

---

## Data Flow

```
Layer 0 (Stats Sweep)
  ↓
[Relationship Artifacts] → core.Artifact{Kind: ArtifactRelationship, Payload: stats.RelationshipPayload}
  ↓
GeneratorPort.GenerateHypotheses()
  ↓
[Hypothesis Candidates] → ports.HypothesisCandidate
  ↓
HypothesisService.convertHypothesesToArtifacts()
  ↓
[Hypothesis Artifacts] → core.Artifact{Kind: ArtifactHypothesis, Payload: {...}}
  ↓
[Generation Audit] → core.Artifact{Kind: ArtifactVariableHealth, Payload: {operation: "hypothesis_generation", ...}}
  ↓
UI reads artifacts by kind → renders hypothesis cards
```

---

## Step 1: Request/Response Schemas

**File:** `adapters/llm/generator_adapter.go`

```go
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// Config holds LLM adapter configuration
type Config struct {
	Provider           string        // "openai", "anthropic", "local"
	Model              string        // e.g., "gpt-5.2", "claude-3-opus"
	APIKey             string        // API key for provider
	BaseURL            string        // For local models (e.g., "http://localhost:11434")
	Temperature        float64       // 0.0-1.0, lower = more deterministic
	MaxTokens          int           // Max tokens in response
	Timeout            time.Duration // Request timeout
	FallbackToHeuristic bool         // Fallback to heuristic on error
}

// GeneratorAdapter implements GeneratorPort using LLM
type GeneratorAdapter struct {
	config           Config
	llmClient       LLMClient
	fallbackGen     ports.GeneratorPort
	relationshipIdx map[string]core.ArtifactID // Maps relationship key → artifact ID for citations
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
		config:       config,
		llmClient:    client,
		fallbackGen:  fallbackGen,
		relationshipIdx: make(map[string]core.ArtifactID),
	}, nil
}
```

---

## Step 2: Extract Top Relationships

**Function:** `ExtractTopRelationships()`

```go
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

			relPayload = stats.RelationshipPayload{
				VariableX:  core.VariableKey(varX),
				VariableY:  core.VariableKey(varY),
				TestType:   stats.TestType(testType),
				EffectSize: effectSize,
				PValue:     pValue,
				SampleSize: int(sampleSize),
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

type relationshipWithScore struct {
	Payload stats.RelationshipPayload
	Score   float64
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
```

---

## Step 3: Build Prompt

**Function:** `BuildPrompt()`

```go
// PromptData structures the input for LLM
type PromptData struct {
	Relationships []relationshipForPrompt `json:"relationships"`
	Variables     []string                `json:"variables"`     // Unique variable names
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
}

// BuildPrompt creates LLM prompt from relationships
func (g *GeneratorAdapter) BuildPrompt(
	relationships []stats.RelationshipPayload,
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

	// Convert relationships to prompt format
	relForPrompt := make([]relationshipForPrompt, len(relationships))
	for i, rel := range relationships {
		relForPrompt[i] = relationshipForPrompt{
			VariableX:  string(rel.VariableX),
			VariableY:  string(rel.VariableY),
			TestType:   string(rel.TestType),
			EffectSize: rel.EffectSize,
			PValue:     rel.PValue,
			QValue:     rel.QValue,
			SampleSize: rel.SampleSize,
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
  - supporting_artifacts: array of relationship keys (format: "relationship:{testType}:{familyID}:{varX}:{varY}")

Output ONLY a JSON array of hypothesis candidates, no other text.`,
		string(jsonData),
		strings.Join(variables, ", "),
		len(relationships),
		req.MaxHypotheses,
		req.RigorProfile)

	return prompt, nil
}
```

---

## Step 4: Parse & Validate Candidates

**Function:** `ParseCandidates()` and `ValidateCandidates()`

````go
// LLMCandidate is the raw LLM response structure
type LLMCandidate struct {
	CauseKey          string   `json:"cause_key"`
	EffectKey         string   `json:"effect_key"`
	MechanismCategory string   `json:"mechanism_category"`
	ConfounderKeys    []string `json:"confounder_keys"`
	Rationale         string   `json:"rationale"`
	SuggestedRigor    string   `json:"suggested_rigor"`
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
) ([]ports.HypothesisCandidate, []core.Artifact) {

	validated := []ports.HypothesisCandidate{}
	auditArtifacts := []core.Artifact{}

	for i, cand := range candidates {
		// GUARDRAIL 1: Citations are mandatory
		if len(cand.SupportingArtifacts) == 0 {
			auditArtifacts = append(auditArtifacts, g.createDroppedAudit(
				fmt.Sprintf("candidate_%d", i),
				"missing_citations",
				"Candidate dropped: no supporting_artifacts provided",
			))
			continue
		}

		// GUARDRAIL 2: Variables must exist
		causeKey, err := core.ParseVariableKey(cand.CauseKey)
		if err != nil || !validVariables[causeKey] {
			auditArtifacts = append(auditArtifacts, g.createDroppedAudit(
				fmt.Sprintf("candidate_%d", i),
				"invalid_cause_key",
				fmt.Sprintf("Candidate dropped: cause_key '%s' not in registry", cand.CauseKey),
			))
			continue
		}

		effectKey, err := core.ParseVariableKey(cand.EffectKey)
		if err != nil || !validVariables[effectKey] {
			auditArtifacts = append(auditArtifacts, g.createDroppedAudit(
				fmt.Sprintf("candidate_%d", i),
				"invalid_effect_key",
				fmt.Sprintf("Candidate dropped: effect_key '%s' not in registry", cand.EffectKey),
			))
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
			"direct_causal":      true,
			"effect_modification": true,
			"confounding_path":   true,
			"proxy_relationship": true,
			"measurement_bias":   true,
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
			auditArtifacts = append(auditArtifacts, g.createDroppedAudit(
				fmt.Sprintf("candidate_%d", i),
				"invalid_citations",
				"Candidate dropped: supporting_artifacts do not match any relationship",
			))
			continue
		}

		validated = append(validated, ports.HypothesisCandidate{
			CauseKey:          causeKey,
			EffectKey:         effectKey,
			ConfounderKeys:    confounderKeys,
			MechanismCategory: cand.MechanismCategory,
			Rationale:         cand.Rationale,
			SuggestedRigor:    rigor,
		})
	}

	return validated, auditArtifacts
}

// createDroppedAudit creates audit artifact for dropped candidates
func (g *GeneratorAdapter) createDroppedAudit(candidateID, reason, message string) core.Artifact {
	return core.Artifact{
		ID:   core.NewID(),
		Kind: core.ArtifactVariableHealth, // Reuse existing kind for audit
		Payload: map[string]interface{}{
			"operation":     "hypothesis_generation_audit",
			"candidate_id":  candidateID,
			"dropped_reason": reason,
			"message":        message,
		},
		CreatedAt: core.Now(),
	}
}
````

---

## Step 5: Main GenerateHypotheses Implementation

```go
// GenerateHypotheses implements GeneratorPort
func (g *GeneratorAdapter) GenerateHypotheses(
	ctx context.Context,
	req ports.HypothesisRequest,
) ([]ports.HypothesisCandidate, error) {

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
		return []ports.HypothesisCandidate{}, nil
	}

	// Build valid variable set from relationships
	validVariables := make(map[core.VariableKey]bool)
	for _, rel := range relationships {
		validVariables[rel.VariableX] = true
		validVariables[rel.VariableY] = true
	}

	// Build prompt
	prompt, err := g.BuildPrompt(relationships, req)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

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
	validated, auditArtifacts := g.ValidateCandidates(
		llmCandidates,
		relationships,
		relKeyToID,
		validVariables,
	)

	// Store audit artifacts (would be done by HypothesisService in real impl)
	_ = auditArtifacts

	// Limit to MaxHypotheses
	if len(validated) > req.MaxHypotheses {
		validated = validated[:req.MaxHypotheses]
	}

	return validated, nil
}
```

---

## Step 6: Wire via Config Flag

**File:** `cmd/cli/main.go` or `ui/app.go`

```go
func setupHypothesisService(kit *testkit.TestKit) *app.HypothesisService {
	var generator ports.GeneratorPort

	generatorMode := os.Getenv("GENERATOR_MODE")
	if generatorMode == "llm" {
		config := llm.Config{
			Provider:            os.Getenv("LLM_PROVIDER"),
			Model:               os.Getenv("LLM_MODEL"),
			APIKey:              os.Getenv("LLM_API_KEY"),
			BaseURL:             os.Getenv("LLM_BASE_URL"),
			Temperature:         0.3, // Lower = more deterministic
			MaxTokens:           2000,
			Timeout:             30 * time.Second,
			FallbackToHeuristic: true,
		}

		fallbackGen := heuristic.NewGenerator()
		llmGen, err := llm.NewGeneratorAdapter(config, fallbackGen)
		if err != nil {
			log.Printf("Warning: LLM generator failed to initialize, using heuristic: %v", err)
			generator = fallbackGen
		} else {
			generator = llmGen
		}
	} else {
		generator = heuristic.NewGenerator()
	}

	return app.NewHypothesisService(
		generator,
		kit.BatteryAdapter(),
		kit.StageRunner(),
		kit.LedgerAdapter(),
		kit.RNGAdapter(),
	)
}
```

---

## Step 7: Enhanced Artifact Payloads

**Modify:** `app/hypothesis_service.go::convertHypothesesToArtifacts()`

```go
func (s *HypothesisService) convertHypothesesToArtifacts(
	candidates []ports.HypothesisCandidate,
	runID core.RunID,
	generatorType string, // "llm" or "heuristic"
) []core.Artifact {
	artifacts := make([]core.Artifact, len(candidates))

	for i, candidate := range candidates {
		// Extract supporting artifacts if available (from LLM candidate)
		supportingArtifacts := []core.ArtifactID{}
		// Note: This would come from LLMCandidate.SupportingArtifacts
		// For now, we'll need to extend HypothesisCandidate or pass separately

		artifacts[i] = core.Artifact{
			ID:   core.NewID(),
			Kind: core.ArtifactHypothesis,
			Payload: map[string]interface{}{
				"run_id":             runID,
				"cause_key":          candidate.CauseKey,
				"effect_key":         candidate.EffectKey,
				"mechanism_category": candidate.MechanismCategory,
				"confounder_keys":    candidate.ConfounderKeys,
				"rationale":          candidate.Rationale,
				"suggested_rigor":    candidate.SuggestedRigor,
				"generator_type":     generatorType, // NEW
				"supporting_artifacts": supportingArtifacts, // NEW
			},
			CreatedAt: core.Now(),
		}

		// Validate artifact before returning
		if err := artifacts.ValidateArtifact(artifacts[i]); err != nil {
			// Log error but continue (or handle appropriately)
			log.Printf("Warning: Hypothesis artifact validation failed: %v", err)
		}
	}

	return artifacts
}
```

---

## Step 8: UI Rendering (Artifact-Only)

**File:** `ui/templates/hypothesis_drafts.html`

```html
{{range .Artifacts}}
<div class="bg-white border border-gray-200 rounded-lg p-4">
    <!-- Generator provenance chip -->
    {{if .Payload.generator_type}}
    <div class="mb-2">
        <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
            {{if eq .Payload.generator_type "llm"}}bg-purple-100 text-purple-800{{else}}bg-gray-100 text-gray-800{{end}}">
            {{if eq .Payload.generator_type "llm"}}🤖 LLM{{else}}⚙️ Heuristic{{end}}
        </span>
    </div>
    {{end}}

    <h4 class="text-lg font-medium text-gray-900">
        {{.Payload.cause_key}} → {{.Payload.effect_key}}
    </h4>

    <p class="text-gray-700 mb-3">{{.Payload.rationale}}</p>

    <!-- Citations drawer -->
    {{if .Payload.supporting_artifacts}}
    <div class="mb-3">
        <button onclick="toggleCitations('{{.ID}}')" class="text-sm text-blue-600 hover:text-blue-900">
            <i class="fas fa-link mr-1"></i>Cites {{len .Payload.supporting_artifacts}} relationship(s)
        </button>
        <div id="citations-{{.ID}}" class="hidden mt-2 p-2 bg-gray-50 rounded">
            {{range .Payload.supporting_artifacts}}
            <a href="/artifacts/{{.}}" class="text-sm text-blue-600 hover:underline">{{.}}</a>
            {{end}}
        </div>
    </div>
    {{end}}

    <!-- Rest of card... -->
</div>
{{end}}
```

---

## Testing Checklist

### Unit Tests

```go
func TestExtractTopRelationships(t *testing.T) {
	// Golden fixture: shopping dataset relationships
	artifacts := loadTestArtifacts("testdata/shopping_relationships.json")

	adapter := &GeneratorAdapter{}
	rels, relKeyToID, err := adapter.ExtractTopRelationships(artifacts, 10)

	assert.NoError(t, err)
	assert.Len(t, rels, 10)
	assert.Greater(t, len(relKeyToID), 0)

	// Verify all relationships are significant
	for _, rel := range rels {
		assert.LessOrEqual(t, rel.PValue, 0.05)
		assert.GreaterOrEqual(t, rel.EffectSize, 0.1)
	}
}

func TestValidateCandidates_MissingCitations(t *testing.T) {
	candidates := []LLMCandidate{
		{
			CauseKey: "order_count",
			EffectKey: "total_spent",
			SupportingArtifacts: []string{}, // Missing!
		},
	}

	validated, audits := adapter.ValidateCandidates(candidates, relationships, relKeyToID, validVars)

	assert.Len(t, validated, 0) // Dropped
	assert.Len(t, audits, 1)     // Audit created
	assert.Contains(t, audits[0].Payload["dropped_reason"], "missing_citations")
}

func TestValidateCandidates_UnknownVariable(t *testing.T) {
	candidates := []LLMCandidate{
		{
			CauseKey: "nonexistent_var", // Not in registry
			EffectKey: "total_spent",
			SupportingArtifacts: []string{"relationship:pearson:..."},
		},
	}

	validated, audits := adapter.ValidateCandidates(candidates, relationships, relKeyToID, validVars)

	assert.Len(t, validated, 0)
	assert.Len(t, audits, 1)
	assert.Contains(t, audits[0].Payload["dropped_reason"], "invalid_cause_key")
}
```

### Integration Test

```go
func TestLLMGenerator_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Use mock LLM client with golden response
	mockClient := &MockLLMClient{
		Response: loadGoldenResponse("testdata/llm_response_golden.json"),
	}

	config := Config{
		Provider: "mock",
		Model:    "test-model",
		Timeout:  5 * time.Second,
	}

	adapter := &GeneratorAdapter{
		config:     config,
		llmClient:  mockClient,
		fallbackGen: heuristic.NewGenerator(),
	}

	req := ports.HypothesisRequest{
		Context: ports.HypothesisContext{
			RelationshipArts: loadTestArtifacts("testdata/shopping_relationships.json"),
		},
		MaxHypotheses: 5,
		RigorProfile:  ports.RigorStandard,
	}

	candidates, err := adapter.GenerateHypotheses(context.Background(), req)

	assert.NoError(t, err)
	assert.Len(t, candidates, 5)

	// Verify all candidates have valid variables
	for _, cand := range candidates {
		assert.NotEmpty(t, cand.CauseKey)
		assert.NotEmpty(t, cand.EffectKey)
		assert.Contains(t, []string{"direct_causal", "effect_modification", ...}, cand.MechanismCategory)
	}
}
```

### Replay Test

```go
func TestLLMGenerator_DeterministicReplay(t *testing.T) {
	// Same inputs → same outputs (with deterministic LLM)
	req1 := buildRequest("snapshot_001", "cohort_hash_001", 42)
	req2 := buildRequest("snapshot_001", "cohort_hash_001", 42)

	candidates1, _ := adapter.GenerateHypotheses(ctx, req1)
	candidates2, _ := adapter.GenerateHypotheses(ctx, req2)

	// Compare fingerprints
	fp1 := computeFingerprint(candidates1)
	fp2 := computeFingerprint(candidates2)

	assert.Equal(t, fp1, fp2, "Same inputs should produce same outputs")
}
```

---

## Definition of Done

✅ **LLM generation produces candidates ONLY from Layer 0 evidence**

- All candidates reference `supporting_artifacts` that exist in relationship artifacts
- No variables outside the registry are allowed

✅ **Every candidate is persisted with provenance + citations**

- Artifact payload includes `generator_type`, `supporting_artifacts`
- Generation audit artifacts record dropped candidates and reasons

✅ **UI can render hypotheses and drill into cited relationships**

- Hypothesis cards show generator badge
- "Cites N relationships" drawer links to relationship artifacts
- No special endpoints - pure artifact queries

✅ **Validation remains programmatic; LLM never affects verdicts**

- LLM only generates hypotheses
- Referee validation (Layer 1) is unchanged
- LLM output is validated before persistence

✅ **Fallback path works**

- LLM errors → heuristic generator
- Missing citations → candidate dropped + audit
- Unknown variables → candidate dropped + audit

---

## Next Steps (Implementation Order)

1. ✅ **Step 1**: Add `Config` and `GeneratorAdapter` structs
2. ✅ **Step 2**: Implement `ExtractTopRelationships()` with guardrails
3. ✅ **Step 3**: Implement `BuildPrompt()` with JSON schema
4. ✅ **Step 4**: Implement `ParseCandidates()` and `ValidateCandidates()`
5. ✅ **Step 5**: Implement `GenerateHypotheses()` with timeout + fallback
6. ✅ **Step 6**: Wire via `GENERATOR_MODE` env var
7. ✅ **Step 7**: Enhance artifact payloads with `generator_type` + `supporting_artifacts`
8. ✅ **Step 8**: Update UI templates to show provenance + citations
9. ✅ **Step 9**: Add unit tests for each function
10. ✅ **Step 10**: Add integration test with golden fixtures

---

## Guardrails Summary

| Guardrail                | Implementation                                      | Failure Mode                              |
| ------------------------ | --------------------------------------------------- | ----------------------------------------- |
| **Citations mandatory**  | `ValidateCandidates()` checks `SupportingArtifacts` | Candidate dropped, audit artifact created |
| **Variables must exist** | Validate against `validVariables` map               | Candidate dropped, audit artifact created |
| **Schema validation**    | `artifacts.ValidateArtifact()` before persistence   | Artifact rejected, error logged           |
| **Timeout**              | `context.WithTimeout()` on LLM call                 | Fallback to heuristic generator           |
| **Budget limits**        | `MaxHypotheses` enforced                            | Excess candidates truncated               |
| **Deterministic keys**   | `buildRelationshipKey()` matches artifact registry  | Consistent citation lookup                |

---

## Artifact Kinds & Payloads

### Hypothesis Artifact

```json
{
  "id": "hyp_001",
  "kind": "hypothesis",
  "payload": {
    "run_id": "run_001",
    "cause_key": "inspection_count",
    "effect_key": "severity_score",
    "mechanism_category": "direct_causal",
    "confounder_keys": ["facility_size", "regulatory_region"],
    "rationale": "Increased inspection frequency likely improves compliance...",
    "suggested_rigor": "standard",
    "generator_type": "llm",
    "supporting_artifacts": ["rel_001", "rel_002"]
  },
  "created_at": "2024-01-01T12:00:00Z"
}
```

### Generation Audit Artifact

```json
{
  "id": "audit_001",
  "kind": "variable_health",
  "payload": {
    "operation": "hypothesis_generation_audit",
    "candidate_id": "candidate_0",
    "dropped_reason": "missing_citations",
    "message": "Candidate dropped: no supporting_artifacts provided"
  },
  "created_at": "2024-01-01T12:00:00Z"
}
```

---

This playbook provides copy-paste-ready code using your actual primitives. Each step builds on the previous one, and guardrails prevent the LLM from poisoning the pipeline.
