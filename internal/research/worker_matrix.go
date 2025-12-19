package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/discovery"
	"gohypo/domain/greenfield"
	"gohypo/models"
	"gohypo/ports"
)

// loadMatrixBundleForHypothesisWithContext loads matrix data for hypothesis validation with timeout and retry
func (rw *ResearchWorker) loadMatrixBundleForHypothesisWithContext(ctx context.Context, directive models.ResearchDirectiveResponse) (*dataset.MatrixBundle, error) {
	// Extract variable keys from hypothesis
	causeKey := core.VariableKey(directive.CauseKey)
	effectKey := core.VariableKey(directive.EffectKey)

	log.Printf("[ResearchWorker] 🔍 Resolving matrix for variables: cause=%s, effect=%s", causeKey, effectKey)

	// Retry logic for matrix resolution
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("[ResearchWorker] 🔄 Matrix resolution retry %d/%d for cause=%s, effect=%s", attempt, maxRetries, causeKey, effectKey)
			// Wait before retry with exponential backoff
			time.Sleep(time.Duration(attempt-1) * 2 * time.Second)
		}

		// Load from data source using testkit
		resolver := rw.testkit.MatrixResolverAdapter()
		bundle, err := resolver.ResolveMatrix(ctx, ports.MatrixResolutionRequest{
			ViewID:    core.ID("hypothesis_validation"),
			EntityIDs: nil, // Include all entities
			VarKeys:   []core.VariableKey{causeKey, effectKey},
		})

		if err == nil {
			if bundle == nil {
				log.Printf("[ResearchWorker] ❌ Matrix resolution returned nil bundle for cause=%s, effect=%s", causeKey, effectKey)
				lastErr = fmt.Errorf("matrix resolution returned nil bundle")
				continue // Retry even for nil bundle
			}

			log.Printf("[ResearchWorker] ✅ Matrix resolved successfully on attempt %d: %d entities, %d variables", attempt, len(bundle.Matrix.EntityIDs), len(bundle.Matrix.VariableKeys))
			return bundle, nil
		}

		lastErr = err
		log.Printf("[ResearchWorker] ❌ Matrix resolution attempt %d failed for cause=%s, effect=%s: %v", attempt, causeKey, effectKey, err)

		// Check if context was cancelled
		if ctx.Err() != nil {
			log.Printf("[ResearchWorker] ❌ Matrix resolution cancelled due to context: %v", ctx.Err())
			return nil, ctx.Err()
		}
	}

	log.Printf("[ResearchWorker] ❌ Matrix resolution failed after %d attempts for cause=%s, effect=%s: %v", maxRetries, causeKey, effectKey, lastErr)
	return nil, fmt.Errorf("matrix resolution failed after %d attempts: %w", maxRetries, lastErr)
}

// prepareFieldMetadata converts field metadata and statistical artifacts to JSON string
func (rw *ResearchWorker) prepareFieldMetadata(
	metadata []greenfield.FieldMetadata,
	statsArtifacts []map[string]interface{},
	discoveryBriefs []discovery.DiscoveryBrief,
) (string, error) {
	// Prepare comprehensive context for LLM
	contextData := map[string]interface{}{
		"field_metadata":        metadata,
		"statistical_artifacts": statsArtifacts,
		"discovery_briefs":      discoveryBriefs,
		"total_fields":          len(metadata),
		"total_stats_artifacts": len(statsArtifacts),
	}

	// Marshal to JSON for LLM processing
	data, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// validateHypothesis performs validation logic on a hypothesis
func (rw *ResearchWorker) validateHypothesis(directive *models.ResearchDirectiveResponse) bool {
	// This is a simplified validation - in a real implementation this would be much more sophisticated
	// For now, we'll randomly validate hypotheses (in practice this would run actual statistical tests)

	// Basic validation: check if required fields are present and make sense
	if directive.ID == "" || directive.BusinessHypothesis == "" || directive.ScienceHypothesis == "" {
		return false
	}

	// Check if validation methods array has at least one method
	if len(directive.ValidationMethods) == 0 {
		return false
	}

	// Validate each validation method has required fields
	for _, method := range directive.ValidationMethods {
		if method.Type == "" || method.MethodName == "" || method.ExecutionPlan == "" {
			return false
		}
	}

	// Check referee gates are reasonable
	if directive.RefereeGates.ConfidenceTarget <= 0 || directive.RefereeGates.ConfidenceTarget > 1 {
		return false
	}

	if directive.RefereeGates.StabilityThreshold < 0 || directive.RefereeGates.StabilityThreshold > 1 {
		return false
	}

	// For this demo, randomly validate 70% of hypotheses as valid
	// In practice, this would run actual statistical validation
	return time.Now().UnixNano()%10 < 7
}

// generateEffectSize generates a simulated effect size (Cohen's d)
func (rw *ResearchWorker) generateEffectSize() float64 {
	// Generate effect sizes between -0.8 and 2.0 (typical range for Cohen's d)
	return -0.8 + rand.Float64()*2.8
}

// generatePValue generates a simulated p-value based on validation status
func (rw *ResearchWorker) generatePValue(validated bool) float64 {
	if validated {
		// Validated hypotheses should have significant p-values (typically < 0.05)
		return rand.Float64() * 0.049 // 0.000 to 0.049
	} else {
		// Non-validated hypotheses can have any p-value
		return rand.Float64() // 0.000 to 1.000
	}
}

// generateSampleSize generates a simulated sample size
func (rw *ResearchWorker) generateSampleSize() int {
	// Generate sample sizes between 100 and 10000
	return 100 + rand.Intn(9900)
}