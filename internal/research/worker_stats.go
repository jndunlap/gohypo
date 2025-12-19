package research

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"gohypo/app"
	"gohypo/domain/core"
	"gohypo/domain/discovery"
	"gohypo/domain/greenfield"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// runStatsSweep executes statistical analysis on the current dataset and returns
// a prompt-friendly artifact slice. This MUST be sourced from the active dataset
// (e.g. Excel file behind the UI), never from hardcoded examples.
func (rw *ResearchWorker) runStatsSweep(ctx context.Context, sessionID string, fieldMetadata []greenfield.FieldMetadata) ([]map[string]interface{}, error) {
	log.Printf("[ResearchWorker] 🔬 Starting stats sweep for session %s", sessionID)

	if rw.statsSweepSvc == nil {
		log.Printf("[ResearchWorker] ❌ Stats sweep service not available for session %s", sessionID)
		return nil, fmt.Errorf("stats sweep service not available")
	}
	if rw.testkit == nil {
		log.Printf("[ResearchWorker] ❌ Testkit not available for session %s", sessionID)
		return nil, fmt.Errorf("testkit not available")
	}

	// Resolve a matrix bundle for the variables we know about.
	// Note: the Excel resolver will ignore any requested keys it cannot resolve.
	varKeys := make([]core.VariableKey, 0, len(fieldMetadata))
	for _, fm := range fieldMetadata {
		if fm.Name == "" {
			continue
		}
		varKeys = append(varKeys, core.VariableKey(fm.Name))
	}
	if len(varKeys) == 0 {
		log.Printf("[ResearchWorker] ❌ No variable keys available for stats sweep in session %s", sessionID)
		return nil, fmt.Errorf("no variable keys available for stats sweep")
	}
	log.Printf("[ResearchWorker] 📊 Resolving matrix bundle for %d variables in session %s", len(varKeys), sessionID)

	matrixStart := time.Now()
	resolver := rw.testkit.MatrixResolverAdapter()
	bundle, err := resolver.ResolveMatrix(ctx, ports.MatrixResolutionRequest{
		ViewID:     core.ID("ui-research"),
		SnapshotID: core.SnapshotID(sessionID),
		EntityIDs:  nil, // include all entities in the dataset
		VarKeys:    varKeys,
	})
	matrixDuration := time.Since(matrixStart)

	if err != nil {
		log.Printf("[ResearchWorker] ❌ Matrix resolution failed after %.2fs for session %s: %v", matrixDuration.Seconds(), sessionID, err)
		return nil, fmt.Errorf("failed to resolve matrix bundle: %w", err)
	}
	log.Printf("[ResearchWorker] ✅ Matrix resolved in %.2fs for session %s (%d entities, %d variables)", matrixDuration.Seconds(), sessionID, len(bundle.Matrix.EntityIDs), len(bundle.Matrix.VariableKeys))

	// Run the sweep and return the resulting artifacts (relationships + manifest).
	log.Printf("[ResearchWorker] 🧮 Running statistical sweep for session %s", sessionID)
	sweepStart := time.Now()
	sweepResp, err := rw.statsSweepSvc.RunStatsSweep(ctx, app.StatsSweepRequest{MatrixBundle: bundle})
	sweepDuration := time.Since(sweepStart)

	if err != nil {
		log.Printf("[ResearchWorker] ❌ Stats sweep failed after %.2fs for session %s: %v", sweepDuration.Seconds(), sessionID, err)
		return nil, fmt.Errorf("stats sweep failed: %w", err)
	}
	log.Printf("[ResearchWorker] ✅ Stats sweep completed in %.2fs for session %s (%d relationships)", sweepDuration.Seconds(), sessionID, len(sweepResp.Relationships))

	artifacts := make([]map[string]interface{}, 0, len(sweepResp.Relationships)+1)
	for _, a := range sweepResp.Relationships {
		artifacts = append(artifacts, map[string]interface{}{
			"kind":       string(a.Kind),
			"id":         a.ID,
			"payload":    a.Payload,
			"created_at": a.CreatedAt,
		})
	}
	artifacts = append(artifacts, map[string]interface{}{
		"kind":       string(sweepResp.Manifest.Kind),
		"id":         sweepResp.Manifest.ID,
		"payload":    sweepResp.Manifest.Payload,
		"created_at": sweepResp.Manifest.CreatedAt,
	})

	log.Printf("[ResearchWorker] 📦 Stats sweep complete for session %s: %d total artifacts", sessionID, len(artifacts))
	return artifacts, nil
}

func (rw *ResearchWorker) buildDiscoveryBriefs(sessionID string, statsArtifacts []map[string]interface{}) []discovery.DiscoveryBrief {
	// Extract relationship payloads out of the stats artifacts list (best-effort).
	rels := make([]stats.RelationshipPayload, 0, len(statsArtifacts))
	for _, a := range statsArtifacts {
		kind, _ := a["kind"].(string)
		if kind != string(core.ArtifactRelationship) {
			continue
		}
		payload := a["payload"]

		switch p := payload.(type) {
		case stats.RelationshipPayload:
			rels = append(rels, p)
		case map[string]interface{}:
			if rp, ok := coerceRelationshipPayloadMap(p); ok {
				rels = append(rels, rp)
			}
		}
	}

	if len(rels) == 0 {
		return nil
	}

	briefs := discovery.BuildDiscoveryBriefsFromRelationships(
		core.SnapshotID(""), // snapshot unknown in UI research flow today
		core.RunID(sessionID),
		rels,
		nil, // No sense results available in worker context
	)

	// Sort by confidence (desc) and keep a small, LLM-friendly set.
	sort.Slice(briefs, func(i, j int) bool {
		return briefs[i].ConfidenceScore > briefs[j].ConfidenceScore
	})
	if len(briefs) > 8 {
		briefs = briefs[:8]
	}
	return briefs
}

func coerceRelationshipPayloadMap(m map[string]interface{}) (stats.RelationshipPayload, bool) {
	varX, _ := m["variable_x"].(string)
	varY, _ := m["variable_y"].(string)
	testType, _ := m["test_type"].(string)
	familyID, _ := m["family_id"].(string)
	if varX == "" || varY == "" || testType == "" || familyID == "" {
		return stats.RelationshipPayload{}, false
	}

	effectSize, _ := toFloat64(m["effect_size"])
	pValue, _ := toFloat64(m["p_value"])
	qValue, _ := toFloat64(m["q_value"])
	sampleSizeF, _ := toFloat64(m["sample_size"])
	totalComparisonsF, _ := toFloat64(m["total_comparisons"])

	warnings := []stats.WarningCode{}
	if ws, ok := m["warnings"].([]interface{}); ok {
		for _, w := range ws {
			if s, ok := w.(string); ok && s != "" {
				warnings = append(warnings, stats.WarningCode(s))
			}
		}
	}

	return stats.RelationshipPayload{
		VariableX:        core.VariableKey(varX),
		VariableY:        core.VariableKey(varY),
		TestType:         stats.TestType(testType),
		FamilyID:         core.Hash(familyID),
		EffectSize:       effectSize,
		PValue:           pValue,
		QValue:           qValue,
		SampleSize:       int(sampleSizeF),
		TotalComparisons: int(totalComparisonsF),
		Warnings:         warnings,
	}, true
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}