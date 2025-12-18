package ui

import (
	"fmt"
	"net/http"

	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// handleIndex renders the main index page with D3.js visualization
func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Get all artifacts to extract dataset and field info
	allArtifacts, err := a.reader.ListArtifacts(r.Context(), ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		http.Error(w, "Failed to load artifacts", http.StatusInternalServerError)
		return
	}

	// Extract dataset information from run artifacts and relationship artifacts
	datasetInfo := map[string]interface{}{
		"name":       "Shopping Dataset",
		"snapshotID": "",
		"snapshotAt": "",
		"cutoffAt":   "",
		"cohortSize": 0,
		"runCount":   0,
		"createdAt":  "",
		"source":     "testkit",
	}

	// Extract unique fields from relationship artifacts
	fieldSet := make(map[string]bool)
	relationshipCount := 0
	runIDs := make(map[string]bool)
	earliestTimestamp := core.Timestamp{}
	latestTimestamp := core.Timestamp{}

	for _, artifact := range allArtifacts {
		// Track timestamps
		if earliestTimestamp.IsZero() || artifact.CreatedAt.Before(earliestTimestamp) {
			earliestTimestamp = artifact.CreatedAt
		}
		if latestTimestamp.IsZero() || artifact.CreatedAt.After(latestTimestamp) {
			latestTimestamp = artifact.CreatedAt
		}

		// Extract run ID and dataset info
		if artifact.Kind == core.ArtifactRun {
			runIDs[string(artifact.ID)] = true

			// Try to extract dataset info from run manifest
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if snapshotID, ok := payload["snapshot_id"].(string); ok && datasetInfo["snapshotID"] == "" {
					datasetInfo["snapshotID"] = snapshotID
				}
				if snapshotAt, ok := payload["snapshot_at"].(string); ok && datasetInfo["snapshotAt"] == "" {
					datasetInfo["snapshotAt"] = snapshotAt
				}
				if cutoffAt, ok := payload["cutoff_at"].(string); ok && datasetInfo["cutoffAt"] == "" {
					datasetInfo["cutoffAt"] = cutoffAt
				}
			}
		}

		if artifact.Kind == core.ArtifactRelationship {
			relationshipCount++
			var varX, varY string
			if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				varX = string(relArtifact.Key.VariableX)
				varY = string(relArtifact.Key.VariableY)
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				varX = string(relPayload.VariableX)
				varY = string(relPayload.VariableY)
			} else if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					varX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					varY = vy
				}
			}
			if varX != "" {
				fieldSet[varX] = true
			}
			if varY != "" {
				fieldSet[varY] = true
			}
		}
	}

	// Extract run count and creation time from artifacts
	datasetInfo["runCount"] = len(runIDs)
	if !earliestTimestamp.IsZero() {
		datasetInfo["createdAt"] = earliestTimestamp.Time().Format("2006-01-02 15:04:05")
	}

	// Calculate time span
	if !earliestTimestamp.IsZero() && !latestTimestamp.IsZero() {
		diff := latestTimestamp.Time().Sub(earliestTimestamp.Time())
		days := int(diff.Hours() / 24)
		if days > 0 {
			datasetInfo["timeSpan"] = fmt.Sprintf("%d days", days)
		} else {
			hours := int(diff.Hours())
			if hours > 0 {
				datasetInfo["timeSpan"] = fmt.Sprintf("%d hours", hours)
			}
		}
	}

	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}

	// Determine run status based on artifacts
	runStatus := "NOT_RUN"
	if relationshipCount > 0 {
		runStatus = "COMPLETE"
	} else if len(allArtifacts) > 0 {
		for _, a := range allArtifacts {
			if a.Kind == core.ArtifactRun {
				runStatus = "RUNNING"
				break
			}
		}
	}

	// Count artifacts by kind to determine stage completion
	profileArtifactCount := 0
	pairwiseArtifactCount := relationshipCount
	fdrArtifactCount := 0
	permutationArtifactCount := 0
	stabilityArtifactCount := 0
	batteryArtifactCount := 0

	relKind := core.ArtifactRelationship
	relFilters := ports.ArtifactFilters{
		Kind:  &relKind,
		Limit: 1000,
	}
	relArtifacts, _ := a.reader.ListArtifacts(r.Context(), relFilters)

	for _, artifact := range relArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if qv, ok := payload["q_value"].(float64); ok && qv > 0 {
					fdrArtifactCount++
				} else if qv, ok := payload["fdr_q_value"].(float64); ok && qv > 0 {
					fdrArtifactCount++
				}
			} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				if relArtifact.Metrics.QValue > 0 {
					fdrArtifactCount++
				}
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				if relPayload.QValue > 0 {
					fdrArtifactCount++
				}
			}
		}
	}

	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRun {
			profileArtifactCount++
		}
	}

	// Extract seed/fingerprint from run artifacts
	seed := ""
	fingerprint := ""
	registryHash := ""
	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRun {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if s, ok := payload["seed"].(float64); ok {
					seed = fmt.Sprintf("%.0f", s)
				}
				if fp, ok := payload["fingerprint"].(string); ok {
					fingerprint = fp
				}
				if rh, ok := payload["registry_hash"].(string); ok {
					registryHash = rh
				}
			}
		}
	}

	// Determine significance rule
	significanceRule := "p ≤ 0.05"
	hasQValue := fdrArtifactCount > 0
	if hasQValue {
		significanceRule = "q ≤ 0.05 (BH)"
	}

	// Calculate pairs attempted
	pairsAttempted := len(fields) * (len(fields) - 1) / 2
	if pairsAttempted < 0 {
		pairsAttempted = 0
	}
	pairsTested := relationshipCount
	pairsSkipped := pairsAttempted - pairsTested
	if pairsSkipped < 0 {
		pairsSkipped = 0
	}

	// Count significant relationships
	significantCount := 0
	for _, artifact := range relArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			var pValue float64
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if pv, ok := payload["p_value"].(float64); ok {
					pValue = pv
				}
			} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				pValue = relArtifact.Metrics.PValue
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				pValue = relPayload.PValue
			}
			if pValue > 0 && pValue < 0.05 {
				significantCount++
			}
		}
	}

	// Stage statuses
	stageStatuses := map[string]map[string]interface{}{
		"Profile": {
			"Status":        "NOT_RUN",
			"ArtifactCount": profileArtifactCount,
			"Runtime":       0,
		},
		"Pairwise": {
			"Status":        "NOT_RUN",
			"ArtifactCount": pairwiseArtifactCount,
			"Runtime":       0,
		},
		"FDR": {
			"Status":        "NOT_RUN",
			"ArtifactCount": fdrArtifactCount,
			"Runtime":       0,
		},
		"Permutation": {
			"Status":        "NOT_RUN",
			"ArtifactCount": permutationArtifactCount,
			"Runtime":       0,
		},
		"Stability": {
			"Status":        "NOT_RUN",
			"ArtifactCount": stabilityArtifactCount,
			"Runtime":       0,
		},
		"Battery": {
			"Status":        "NOT_RUN",
			"ArtifactCount": batteryArtifactCount,
			"Runtime":       0,
		},
	}

	if profileArtifactCount > 0 || len(fields) > 0 {
		stageStatuses["Profile"]["Status"] = "COMPLETE"
	}
	if pairwiseArtifactCount > 0 {
		stageStatuses["Pairwise"]["Status"] = "COMPLETE"
	}
	if fdrArtifactCount > 0 {
		stageStatuses["FDR"]["Status"] = "COMPLETE"
	}

	// Extract field-level statistics from relationship artifacts
	type FieldStats struct {
		Name            string
		MissingRate     float64
		MissingRatePct  string
		UniqueCount     int
		Variance        float64
		Cardinality     int
		Type            string
		SampleSize      int
		InRelationships int
	}
	fieldStatsMap := make(map[string]*FieldStats)

	// Initialize field stats
	for _, field := range fields {
		fieldStatsMap[field] = &FieldStats{
			Name:            field,
			MissingRate:     0,
			MissingRatePct:  "—",
			UniqueCount:     0,
			Variance:        0,
			Cardinality:     0,
			Type:            "unknown",
			SampleSize:      0,
			InRelationships: 0,
		}
	}

	// Extract stats from relationship artifacts
	for _, artifact := range relArtifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		var varX, varY string
		var missingRateX, missingRateY float64
		var uniqueCountX, uniqueCountY int
		var varianceX, varianceY float64
		var cardinalityX, cardinalityY int
		var sampleSize int

		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if vx, ok := payload["variable_x"].(string); ok {
				varX = vx
			}
			if vy, ok := payload["variable_y"].(string); ok {
				varY = vy
			}
			if ss, ok := payload["sample_size"].(float64); ok {
				sampleSize = int(ss)
			} else if ss, ok := payload["sample_size"].(int); ok {
				sampleSize = ss
			} else if ss, ok := payload["sample_size"].(int64); ok {
				sampleSize = int(ss)
			}
			if dq, ok := payload["data_quality"].(map[string]interface{}); ok {
				if mrx, ok := dq["missing_rate_x"].(float64); ok {
					missingRateX = mrx
				}
				if mry, ok := dq["missing_rate_y"].(float64); ok {
					missingRateY = mry
				}
				if ucx, ok := dq["unique_count_x"].(float64); ok {
					uniqueCountX = int(ucx)
				} else if ucx, ok := dq["unique_count_x"].(int); ok {
					uniqueCountX = ucx
				}
				if ucy, ok := dq["unique_count_y"].(float64); ok {
					uniqueCountY = int(ucy)
				} else if ucy, ok := dq["unique_count_y"].(int); ok {
					uniqueCountY = ucy
				}
				if vx, ok := dq["variance_x"].(float64); ok {
					varianceX = vx
				}
				if vy, ok := dq["variance_y"].(float64); ok {
					varianceY = vy
				}
				if cx, ok := dq["cardinality_x"].(float64); ok {
					cardinalityX = int(cx)
				} else if cx, ok := dq["cardinality_x"].(int); ok {
					cardinalityX = cx
				}
				if cy, ok := dq["cardinality_y"].(float64); ok {
					cardinalityY = int(cy)
				} else if cy, ok := dq["cardinality_y"].(int); ok {
					cardinalityY = cy
				}
			}
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			varX = string(relArtifact.Key.VariableX)
			varY = string(relArtifact.Key.VariableY)
			sampleSize = relArtifact.Metrics.SampleSize
			missingRateX = relArtifact.DataQuality.MissingRateX
			missingRateY = relArtifact.DataQuality.MissingRateY
			uniqueCountX = relArtifact.DataQuality.UniqueCountX
			uniqueCountY = relArtifact.DataQuality.UniqueCountY
			varianceX = relArtifact.DataQuality.VarianceX
			varianceY = relArtifact.DataQuality.VarianceY
			cardinalityX = relArtifact.DataQuality.CardinalityX
			cardinalityY = relArtifact.DataQuality.CardinalityY
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			varX = string(relPayload.VariableX)
			varY = string(relPayload.VariableY)
			sampleSize = relPayload.SampleSize
		}

		// Update field stats
		if statsX, exists := fieldStatsMap[varX]; exists {
			if missingRateX > 0 {
				statsX.MissingRate = missingRateX
				statsX.MissingRatePct = fmt.Sprintf("%.1f", missingRateX*100)
			}
			if uniqueCountX > 0 {
				statsX.UniqueCount = uniqueCountX
			}
			if varianceX > 0 {
				statsX.Variance = varianceX
				statsX.Type = "numeric"
			}
			if cardinalityX > 0 {
				statsX.Cardinality = cardinalityX
			}
			if sampleSize > 0 {
				statsX.SampleSize = sampleSize
			}
			statsX.InRelationships++
		}
		if statsY, exists := fieldStatsMap[varY]; exists {
			if missingRateY > 0 {
				statsY.MissingRate = missingRateY
				statsY.MissingRatePct = fmt.Sprintf("%.1f", missingRateY*100)
			}
			if uniqueCountY > 0 {
				statsY.UniqueCount = uniqueCountY
			}
			if varianceY > 0 {
				statsY.Variance = varianceY
				statsY.Type = "numeric"
			}
			if cardinalityY > 0 {
				statsY.Cardinality = cardinalityY
			}
			if sampleSize > 0 {
				statsY.SampleSize = sampleSize
			}
			statsY.InRelationships++
		}
	}

	// Convert to slice
	fieldStats := make([]FieldStats, 0, len(fieldStatsMap))
	for _, stats := range fieldStatsMap {
		fieldStats = append(fieldStats, *stats)
	}

	data := map[string]interface{}{
		"Title":             "GoHypo",
		"FieldCount":        len(fields),
		"RelationshipCount": relationshipCount,
		"Fields":            fields,
		"DatasetInfo":       datasetInfo,
		"RunStatus":         runStatus,
		"PairsAttempted":    pairsAttempted,
		"PairsTested":       pairsTested,
		"PairsSkipped":      pairsSkipped,
		"PairsPassed":       significantCount,
		"SignificanceRule":  significanceRule,
		"Seed":              seed,
		"Fingerprint":       fingerprint,
		"RegistryHash":      registryHash,
		"StageStatuses":     stageStatuses,
		"VariablesTotal":    len(fields),
		"VariablesEligible": len(fields),
		"VariablesRejected": 0,
		"FieldStats":        fieldStats,
	}
	a.renderTemplate(w, "index.html", data)
}
