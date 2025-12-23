package research

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gohypo/adapters/excel"
	"gohypo/app"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
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

	// Get the session to check for workspace-based datasets
	session, err := rw.sessionMgr.GetSession(ctx, sessionID)
	if err != nil {
		log.Printf("[ResearchWorker] ❌ Could not get session %s: %v", sessionID, err)
		return nil, fmt.Errorf("could not get session: %w", err)
	}

	var resolver ports.MatrixResolverPort
	var useUploadedDataset bool

	// Check if there's an uploaded dataset for this workspace
	if session.WorkspaceID != uuid.Nil && rw.datasetRepo != nil {
		log.Printf("[ResearchWorker] 🔍 Checking for uploaded datasets in workspace %s", session.WorkspaceID)

		// Get datasets for this workspace
		datasets, err := rw.datasetRepo.GetByWorkspace(ctx, core.ID(session.WorkspaceID.String()), 10, 0)
		log.Printf("[ResearchWorker] 📊 GetByWorkspace result: err=%v, found %d datasets", err, len(datasets))

		if err == nil && len(datasets) > 0 {
			// Find the most recently updated dataset that has a file path
			var selectedDataset *dataset.Dataset
			for _, ds := range datasets {
				log.Printf("[ResearchWorker] 📁 Found dataset: name=%s, status=%s, filepath=%s, updated=%v",
					ds.DisplayName, ds.Status, ds.FilePath, ds.UpdatedAt)
				if ds.Status == dataset.StatusReady && ds.FilePath != "" {
					if selectedDataset == nil || ds.UpdatedAt.After(selectedDataset.UpdatedAt) {
						selectedDataset = ds
					}
				}
			}

			if selectedDataset != nil {
				log.Printf("[ResearchWorker] ✅ Selected dataset: %s (file: %s)", selectedDataset.DisplayName, selectedDataset.FilePath)

				// Create a matrix resolver for the uploaded dataset
				excelConfig := excel.ExcelConfig{
					FilePath: selectedDataset.FilePath,
				}
				resolver = excel.NewExcelMatrixResolverAdapter(excelConfig)
				useUploadedDataset = true
				log.Printf("[ResearchWorker] 📊 Using uploaded dataset matrix resolver for session %s", sessionID)
			} else {
				log.Printf("[ResearchWorker] ❌ No ready datasets with file paths found in workspace")
			}
		} else {
			log.Printf("[ResearchWorker] ❌ No datasets found in workspace: err=%v", err)
		}
	} else {
		log.Printf("[ResearchWorker] ❌ Cannot check uploaded datasets: session.WorkspaceID=%s, datasetRepo=%v", session.WorkspaceID, rw.datasetRepo != nil)
	}

	// Fall back to testkit if no uploaded dataset found
	if !useUploadedDataset {
		if rw.testkit == nil {
			log.Printf("[ResearchWorker] ❌ Testkit not available for session %s", sessionID)
			return nil, fmt.Errorf("testkit not available")
		}
		resolver = rw.testkit.MatrixResolverAdapter()
		log.Printf("[ResearchWorker] 🧪 Using testkit matrix resolver for session %s", sessionID)
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