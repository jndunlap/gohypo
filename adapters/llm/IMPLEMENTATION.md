# LLM Generator Adapter - Implementation Status

## ✅ Implemented

### Core Functions
- ✅ `ExtractTopRelationships()` - Filters and ranks relationships by statistical strength
- ✅ `BuildPrompt()` - Creates LLM prompt with relationship data
- ✅ `ParseCandidates()` - Parses LLM JSON response (handles markdown code blocks)
- ✅ `ValidateCandidates()` - Enforces guardrails (citations, variables, schema)
- ✅ `GenerateHypotheses()` - Main implementation with timeout and fallback

### Guardrails
- ✅ Citations mandatory - candidates without `supporting_artifacts` are dropped
- ✅ Variables must exist - validated against relationship registry
- ✅ Schema validation - mechanism categories and rigor profiles validated
- ✅ Timeout + fallback - automatic fallback to heuristic on error
- ✅ Budget limits - `MaxHypotheses` enforced

### LLM Client Interface
- ✅ `LLMClient` interface for provider abstraction
- ✅ `MockLLMClient` for testing
- ✅ Stubs for OpenAI, Anthropic, Local providers (ready for implementation)

### Testing
- ✅ Unit tests for `ExtractTopRelationships`
- ✅ Unit tests for `ValidateCandidates` (missing citations, unknown variables)
- ✅ Integration test for fallback path

## ⚠️ Limitations / Future Work

### Supporting Artifacts
**Current:** Citations are validated but not stored in artifact payloads.

**Reason:** `ports.HypothesisCandidate` doesn't include `SupportingArtifacts` field.

**Solution Options:**
1. Extend `ports.HypothesisCandidate` to include `SupportingArtifacts []core.ArtifactID`
2. Store supporting artifacts separately and merge in `HypothesisService.convertHypothesesToArtifacts()`
3. Create wrapper type that includes both candidate and metadata

**Recommendation:** Option 1 - extend the interface to include supporting artifacts.

### Generator Type Tracking
**Current:** `EnhanceHypothesisArtifact()` helper exists but not integrated.

**To integrate:** Update `HypothesisService.convertHypothesesToArtifacts()` to call:
```go
artifact = llm.EnhanceHypothesisArtifact(artifact, "llm")
```

### Real LLM Providers
**Current:** Mock client works, real providers return "not implemented" errors.

**To implement:**
- Add OpenAI SDK dependency
- Implement `OpenAIClient.ChatCompletion()`
- Implement `AnthropicClient.ChatCompletion()`
- Implement `LocalLLMClient.ChatCompletion()` (for Ollama, etc.)

### Generation Audit Artifacts
**Current:** Audit artifacts are created but not persisted.

**To integrate:** Update `HypothesisService.ProposeHypotheses()` to persist audit artifacts:
```go
for _, audit := range auditArtifacts {
    s.ledgerPort.StoreArtifact(ctx, string(req.RunID), audit)
}
```

## Usage Example

```go
// Create config
config := llm.Config{
    Provider:            "mock", // or "openai", "anthropic", "local"
    Model:               "gpt-4",
    APIKey:              os.Getenv("LLM_API_KEY"),
    Temperature:         0.3,
    MaxTokens:           2000,
    Timeout:             30 * time.Second,
    FallbackToHeuristic: true,
}

// Create fallback generator
fallbackGen := heuristic.NewGenerator()

// Create LLM adapter
llmGen, err := llm.NewGeneratorAdapter(config, fallbackGen)
if err != nil {
    log.Fatal(err)
}

// Use in HypothesisService
hypothesisService := app.NewHypothesisService(
    llmGen, // or fallbackGen
    batteryPort,
    stageRunner,
    ledgerPort,
    rngPort,
)
```

## Testing

Run tests:
```bash
go test ./adapters/llm/... -v
```

All tests pass ✅

## Next Steps

1. **Extend HypothesisCandidate interface** to include `SupportingArtifacts []core.ArtifactID`
2. **Integrate generator_type** in `HypothesisService.convertHypothesesToArtifacts()`
3. **Persist audit artifacts** in `HypothesisService.ProposeHypotheses()`
4. **Implement real LLM providers** (OpenAI, Anthropic, Local)
5. **Add integration tests** with golden fixtures
6. **Wire via config flag** (`GENERATOR_MODE=llm|heuristic`)

