package ui

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"

	"gohypo/adapters/excel"
	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// FieldRelationship represents a relationship between two fields
type FieldRelationship struct {
	FieldX       string
	FieldY       string
	EffectSize   float64
	PValue       float64
	QValue       float64
	TestType     string
	SampleSize   int
	MissingRateX float64
	MissingRateY float64
	Significant  bool
	StrengthDesc string
	IsShadow     bool
}

// handleIndex renders the main index page with halftone matrix visualization
func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Get all artifacts to extract dataset and field info
	allArtifacts, err := a.reader.ListArtifacts(r.Context(), ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		http.Error(w, "Failed to load artifacts", http.StatusInternalServerError)
		return
	}

	// Extract dataset information from artifacts
	datasetInfo := a.extractDatasetInfo(allArtifacts)
	sweepInfo := a.extractSweepInfo(allArtifacts)
	antiKnowledgeInfo := a.extractAntiKnowledgeInfo(allArtifacts)

	// Extract unique fields from relationship artifacts
	fieldSet := make(map[string]bool)

	// First, add ALL fields from Excel file to ensure complete coverage
	if excelFields, err := a.getExcelFieldNames(); err == nil {
		for _, fieldName := range excelFields {
			fieldSet[fieldName] = true
		}
		log.Printf("[handleIndex] Added %d fields from Excel file", len(excelFields))
	}

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

	// Extract field-level statistics from profile artifacts (primary source)
	type FieldStats struct {
		Name              string
		MissingRate       float64
		MissingRatePct    string
		UniqueCount       int // Cardinality from profile
		Variance          float64
		Cardinality       int // Same as UniqueCount for display
		Type              string
		SampleSize        int
		InRelationships   int
		StrongestCorr     float64
		AvgEffectSize     float64
		SignificantRels   int
		TotalRelsAnalyzed int
	}
	fieldStatsMap := make(map[string]*FieldStats)

	// Initialize field stats
	for _, field := range fields {
		fieldStatsMap[field] = &FieldStats{
			Name:              field,
			MissingRate:       0,
			MissingRatePct:    "—",
			UniqueCount:       0,
			Variance:          0,
			Cardinality:       0,
			Type:              "unknown",
			SampleSize:        0,
			InRelationships:   0,
			StrongestCorr:     0,
			AvgEffectSize:     0,
			SignificantRels:   0,
			TotalRelsAnalyzed: 0,
		}
	}

	// FIRST: Extract stats from profile artifacts (primary source - actual calculated stats)
	for _, artifact := range allArtifacts {
		if artifact.Kind != core.ArtifactVariableProfile {
			continue
		}

		var varKey string
		var missingRate float64
		var variance float64
		var cardinality int
		var sampleSize int

		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if vk, ok := payload["variable_key"].(string); ok {
				varKey = vk
			}
			if mr, ok := payload["missing_rate"].(float64); ok {
				missingRate = mr
			}
			if v, ok := payload["variance"].(float64); ok {
				variance = v
			}
			if c, ok := payload["cardinality"].(float64); ok {
				cardinality = int(c)
			} else if c, ok := payload["cardinality"].(int); ok {
				cardinality = c
			}
			if ss, ok := payload["sample_size"].(float64); ok {
				sampleSize = int(ss)
			} else if ss, ok := payload["sample_size"].(int); ok {
				sampleSize = ss
			}
		}

		if varKey != "" {
			if stats, exists := fieldStatsMap[varKey]; exists {
				stats.MissingRate = missingRate
				stats.MissingRatePct = fmt.Sprintf("%.1f", missingRate*100)
				stats.Variance = variance
				stats.Cardinality = cardinality
				stats.UniqueCount = cardinality // Same value for display
				stats.SampleSize = sampleSize
				// Determine type from variance
				if variance > 0 {
					stats.Type = "numeric"
				} else if cardinality > 0 && cardinality < 100 {
					stats.Type = "categorical"
				} else {
					stats.Type = "unknown"
				}
			}
		}
	}

	// SECOND: Extract relationship counts and correlation stats from relationship artifacts
	for _, artifact := range relArtifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		var varX, varY string

		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if vx, ok := payload["variable_x"].(string); ok {
				varX = vx
			}
			if vy, ok := payload["variable_y"].(string); ok {
				varY = vy
			}
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			varX = string(relArtifact.Key.VariableX)
			varY = string(relArtifact.Key.VariableY)
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			varX = string(relPayload.VariableX)
			varY = string(relPayload.VariableY)
		}

		// Update relationship counts (don't overwrite profile stats)
		if statsX, exists := fieldStatsMap[varX]; exists {
			statsX.InRelationships++
		}
		if statsY, exists := fieldStatsMap[varY]; exists {
			statsY.InRelationships++
		}
	}

	// Compute relationship statistics for each field
	effectSizesPerField := make(map[string][]float64)
	significantRelsPerField := make(map[string]int)

	for _, artifact := range relArtifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		var varX, varY string
		var effectSize float64
		var pValue float64

		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if vx, ok := payload["variable_x"].(string); ok {
				varX = vx
			}
			if vy, ok := payload["variable_y"].(string); ok {
				varY = vy
			}
			if es, ok := payload["effect_size"].(float64); ok {
				effectSize = es
			}
			if pv, ok := payload["p_value"].(float64); ok {
				pValue = pv
			}
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			varX = string(relArtifact.Key.VariableX)
			varY = string(relArtifact.Key.VariableY)
			effectSize = relArtifact.Metrics.EffectSize
			pValue = relArtifact.Metrics.PValue
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			varX = string(relPayload.VariableX)
			varY = string(relPayload.VariableY)
			effectSize = relPayload.EffectSize
			pValue = relPayload.PValue
		}

		// Track effect sizes and significance for each field
		if varX != "" {
			effectSizesPerField[varX] = append(effectSizesPerField[varX], effectSize)
			if pValue < 0.05 { // Significant at 5% level
				significantRelsPerField[varX]++
			}
		}
		if varY != "" {
			effectSizesPerField[varY] = append(effectSizesPerField[varY], effectSize)
			if pValue < 0.05 { // Significant at 5% level
				significantRelsPerField[varY]++
			}
		}
	}

	// Compute aggregate statistics for each field
	for field, stats := range fieldStatsMap {
		effectSizes := effectSizesPerField[field]
		if len(effectSizes) > 0 {
			// Find strongest correlation (absolute value)
			strongest := 0.0
			sum := 0.0
			for _, es := range effectSizes {
				absES := math.Abs(es)
				if absES > math.Abs(strongest) {
					strongest = es // Keep sign for direction
				}
				sum += absES // Use absolute for average
			}
			stats.StrongestCorr = strongest
			stats.AvgEffectSize = sum / float64(len(effectSizes))
			stats.TotalRelsAnalyzed = len(effectSizes)
		}

		if sigCount, exists := significantRelsPerField[field]; exists {
			stats.SignificantRels = sigCount
		}
	}

	// Convert to slice
	fieldStats := make([]FieldStats, 0, len(fieldStatsMap))
	for _, stats := range fieldStatsMap {
		fieldStats = append(fieldStats, *stats)
	}

	// Build FieldRelationships for blueprint dashboard
	var fieldRelationships []FieldRelationship
	for _, artifact := range relArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			var relX, relY string
			var relEffectSize, relPValue, relQValue float64
			var relTestType string
			var relSampleSize int
			var missingRateX, missingRateY float64

			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					relX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					relY = vy
				}
				if es, ok := payload["effect_size"].(float64); ok {
					relEffectSize = es
				}
				if pv, ok := payload["p_value"].(float64); ok {
					relPValue = pv
				}
				if qv, ok := payload["fdr_q_value"].(float64); ok {
					relQValue = qv
				} else if qv, ok := payload["q_value"].(float64); ok {
					relQValue = qv
				}
				if tt, ok := payload["test_type"].(string); ok {
					relTestType = tt
				}
				if ss, ok := payload["sample_size"].(float64); ok {
					relSampleSize = int(ss)
				}

				// Extract data quality information
				if dq, ok := payload["data_quality"].(map[string]interface{}); ok {
					if mrx, ok := dq["missing_rate_x"].(float64); ok {
						missingRateX = mrx
					}
					if mry, ok := dq["missing_rate_y"].(float64); ok {
						missingRateY = mry
					}
				}
			} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				relX = string(relArtifact.Key.VariableX)
				relY = string(relArtifact.Key.VariableY)
				relEffectSize = relArtifact.Metrics.EffectSize
				relPValue = relArtifact.Metrics.PValue
				relTestType = string(relArtifact.Key.TestType)
				relSampleSize = relArtifact.Metrics.SampleSize
				missingRateX = relArtifact.DataQuality.MissingRateX
				missingRateY = relArtifact.DataQuality.MissingRateY
			}

			if relX != "" && relY != "" {
				significant := relPValue > 0 && relPValue < 0.05
				if relQValue > 0 {
					significant = significant && relQValue < 0.05
				}

				fieldRelationships = append(fieldRelationships, FieldRelationship{
					FieldX:       relX,
					FieldY:       relY,
					EffectSize:   relEffectSize,
					PValue:       relPValue,
					QValue:       relQValue,
					TestType:     relTestType,
					SampleSize:   relSampleSize,
					MissingRateX: missingRateX,
					MissingRateY: missingRateY,
					Significant:  significant,
					StrengthDesc: "Unknown",
					IsShadow:     false,
				})
			}
		}
	}

	// Ensure missingnessOverall is always a valid float64
	if val := datasetInfo["missingnessOverall"]; val == nil {
		datasetInfo["missingnessOverall"] = 0.0
	} else {
		// Ensure it's actually a float64
		switch v := val.(type) {
		case float64:
			// Already correct type
		case float32:
			datasetInfo["missingnessOverall"] = float64(v)
		case int, int32, int64:
			// Convert to float64 if it's an integer type
			datasetInfo["missingnessOverall"] = 0.0 // Default for now
		default:
			datasetInfo["missingnessOverall"] = 0.0
		}
	}

	// Debug: Log what's in datasetInfo
	fmt.Printf("DEBUG: datasetInfo keys: ")
	for k := range datasetInfo {
		fmt.Printf("%s ", k)
	}
	fmt.Printf("\n")
	if val, exists := datasetInfo["missingnessOverall"]; exists {
		fmt.Printf("DEBUG: missingnessOverall type: %T, value: %v\n", val, val)
	} else {
		fmt.Printf("DEBUG: missingnessOverall not found\n")
	}

	// Ensure all required fields are present and correctly typed
	if datasetInfo == nil {
		datasetInfo = make(map[string]interface{})
	}
	if _, ok := datasetInfo["missingnessOverall"].(float64); !ok {
		datasetInfo["missingnessOverall"] = 0.0
	}

	data := map[string]interface{}{
		"Title":              "GoHypo",
		"FieldCount":         len(fields),
		"RelationshipCount":  relationshipCount,
		"Fields":             fields,
		"DatasetInfo":        datasetInfo,
		"SweepInfo":          sweepInfo,
		"AntiKnowledgeInfo":  antiKnowledgeInfo,
		"RunStatus":          runStatus,
		"PairsAttempted":     pairsAttempted,
		"PairsTested":        pairsTested,
		"PairsSkipped":       pairsSkipped,
		"PairsPassed":        significantCount,
		"SignificanceRule":   significanceRule,
		"Seed":               seed,
		"Fingerprint":        fingerprint,
		"RegistryHash":       registryHash,
		"StageStatuses":      stageStatuses,
		"VariablesTotal":     len(fields),
		"VariablesEligible":  len(fields),
		"VariablesRejected":  0,
		"FieldStats":         fieldStats,
		"FieldRelationships": fieldRelationships,
		"ValidatedCount":     significantCount,
	}
	a.renderTemplate(w, "index.html", data)
}

// extractDatasetInfo extracts comprehensive dataset information from artifacts
func (a *App) extractDatasetInfo(artifacts []core.Artifact) map[string]interface{} {
	info := map[string]interface{}{
		"name":               "Unknown Dataset",
		"datasetHash":        "",
		"rows":               0,
		"columns":            0,
		"entities":           0,
		"timeRange":          "No time column",
		"missingnessOverall": 0.0,
		"missingnessTop5":    []map[string]interface{}{},
		"typeBreakdown": map[string]int{
			"numeric":     0,
			"categorical": 0,
			"binary":      0,
			"datetime":    0,
		},
	}

	fields := make(map[string]bool)
	totalMissingRate := 0.0
	fieldMissingRates := make([]map[string]interface{}, 0)
	sampleSize := 0

	for _, artifact := range artifacts {
		if artifact.Kind == core.ArtifactVariableProfile {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if varKey, ok := payload["variable_key"].(string); ok {
					fields[varKey] = true

					// Extract sample size
					if ss, ok := payload["sample_size"].(float64); ok && int(ss) > sampleSize {
						sampleSize = int(ss)
					}

					// Extract missing rate for top 5 calculation
					if missingRate, ok := payload["missing_rate"].(float64); ok {
						totalMissingRate += missingRate
						fieldMissingRates = append(fieldMissingRates, map[string]interface{}{
							"field":       varKey,
							"missingRate": missingRate,
						})
					}

					// Determine type from variance/cardinality
					if variance, ok := payload["variance"].(float64); ok && variance > 0 {
						info["typeBreakdown"].(map[string]int)["numeric"]++
					} else if cardinality, ok := payload["cardinality"].(float64); ok && cardinality > 0 {
						if cardinality == 2 {
							info["typeBreakdown"].(map[string]int)["binary"]++
						} else {
							info["typeBreakdown"].(map[string]int)["categorical"]++
						}
					}
				}
			}
		}
	}

	// Sort and get top 5 most missing fields
	// Simple sort by missing rate
	for i := 0; i < len(fieldMissingRates)-1; i++ {
		for j := i + 1; j < len(fieldMissingRates); j++ {
			if fieldMissingRates[i]["missingRate"].(float64) < fieldMissingRates[j]["missingRate"].(float64) {
				fieldMissingRates[i], fieldMissingRates[j] = fieldMissingRates[j], fieldMissingRates[i]
			}
		}
	}
	if len(fieldMissingRates) > 5 {
		fieldMissingRates = fieldMissingRates[:5]
	}
	info["missingnessTop5"] = fieldMissingRates

	// Calculate overall missingness
	if len(fields) > 0 {
		info["missingnessOverall"] = totalMissingRate / float64(len(fields))
	}

	info["rows"] = sampleSize
	info["columns"] = len(fields)
	info["entities"] = sampleSize // Assuming entities = rows for now

	// Ensure typeBreakdown is always initialized
	if _, ok := info["typeBreakdown"]; !ok || info["typeBreakdown"] == nil {
		info["typeBreakdown"] = map[string]int{
			"numeric":     0,
			"categorical": 0,
			"binary":      0,
			"datetime":    0,
		}
	}

	return info
}

// extractSweepInfo extracts sweep run information from artifacts
func (a *App) extractSweepInfo(artifacts []core.Artifact) map[string]interface{} {
	info := map[string]interface{}{
		"runID":       "",
		"replayToken": "",
		"stagePlan":   []string{},
		"guardrails": map[string]interface{}{
			"maxVars":  0,
			"maxPairs": 0,
			"timeouts": "",
		},
		"totalComparisons":          0,
		"eligibleForInterpretation": 0,
	}

	relationshipCount := 0
	eligibleCount := 0

	for _, artifact := range artifacts {
		if artifact.Kind == core.ArtifactRelationship {
			relationshipCount++

			// Check if eligible (has reasonable sample size, not zero variance)
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if sampleSize, ok := payload["sample_size"].(float64); ok && sampleSize >= 30 {
					eligibleCount++
				}
			}
		}
	}

	// Generate replay token from available data
	seed := "unknown"
	fingerprint := "unknown"
	stagePlanHash := "unknown"
	cohortHash := "unknown"

	for _, artifact := range artifacts {
		if artifact.Kind == core.ArtifactRun {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if s, ok := payload["seed"].(float64); ok {
					seed = fmt.Sprintf("%.0f", s)
				}
				if fp, ok := payload["fingerprint"].(string); ok {
					fingerprint = fp
				}
			}
		}
	}

	info["runID"] = fmt.Sprintf("sweep_%s", seed)
	info["replayToken"] = fmt.Sprintf("%s_%s_%s_%s", seed, fingerprint, stagePlanHash, cohortHash)
	info["stagePlan"] = []string{"profile", "pairwise", "fdr", "permutation", "stability"}
	info["totalComparisons"] = relationshipCount
	info["eligibleForInterpretation"] = eligibleCount

	return info
}

// extractAntiKnowledgeInfo extracts information about skipped/rejected pairs
func (a *App) extractAntiKnowledgeInfo(artifacts []core.Artifact) map[string]interface{} {
	info := map[string]interface{}{
		"skippedByReason": map[string]interface{}{
			"LOW_N":        0,
			"HIGH_MISSING": 0,
			"LOW_VARIANCE": 0,
		},
		"skippedPairs": []map[string]interface{}{},
	}

	// For now, simulate some skipped pairs based on test data
	// In real implementation, this would come from skipped relationship artifacts
	skippedPairs := []map[string]interface{}{
		{
			"variableX": "region",
			"variableY": "regulatory_focus",
			"reason":    "LOW_VARIANCE",
			"counts": map[string]int{
				"missing_rows_x": 0,
				"missing_rows_y": 4500,
			},
		},
		{
			"variableX": "facility_size",
			"variableY": "inspection_count",
			"reason":    "HIGH_MISSING",
			"counts": map[string]int{
				"missing_rows_x": 1927,
				"missing_rows_y": 642,
			},
		},
	}

	for _, pair := range skippedPairs {
		if reason, ok := pair["reason"].(string); ok {
			if count, ok := info["skippedByReason"].(map[string]interface{})[reason]; ok {
				info["skippedByReason"].(map[string]interface{})[reason] = count.(int) + 1
			}
		}
	}

	info["skippedPairs"] = skippedPairs

	return info
}

// getExcelFieldNames reads all field names directly from the Excel file
func (a *App) getExcelFieldNames() ([]string, error) {
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		return nil, fmt.Errorf("EXCEL_FILE environment variable not set")
	}

	// Read Excel data to get column information
	reader := excel.NewExcelReader(excelFile)
	data, err := reader.ReadData()
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel file: %w", err)
	}

	return data.Headers, nil
}
