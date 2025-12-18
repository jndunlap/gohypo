package ui

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

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
	Significant  bool
	StrengthDesc string
	IsShadow     bool
}

// ExclusionRow is a high-density diagnostic for “empty”/filtered states.
type ExclusionRow struct {
	Pair      string
	VetoLogic string
	Metric    string
	PValue    float64
	QValue    float64
	SampleN   int
}

// handleRelationships renders the Layer 0 relationships screen
func (a *App) handleRelationships(w http.ResponseWriter, r *http.Request) {
	showShadow := r.URL.Query().Get("shadow") == "1"

	// Get relationship artifacts
	relKind := core.ArtifactRelationship
	artifacts, err := a.reader.GetArtifactsByKind(r.Context(), relKind, 1000)
	if err != nil {
		http.Error(w, "Failed to load relationships", http.StatusInternalServerError)
		return
	}

	// Extract relationships + build exclusion rationale for “null state”
	var relationships []FieldRelationship
	exclusions := make([]ExclusionRow, 0, 64)

	for _, artifact := range artifacts {
		if artifact.Kind != relKind {
			continue
		}

		var relX, relY string
		var relEffectSize, relPValue, relQValue float64
		var relTestType string
		var relSampleSize int
		var warnings []string

		// Extract from map payload (testkit format)
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
			} else if ss, ok := payload["sample_size"].(int); ok {
				relSampleSize = ss
			}
			// Optional warnings from upstream gates
			if ws, ok := payload["warnings"].([]string); ok {
				warnings = append(warnings, ws...)
			} else if ws, ok := payload["warnings"].([]interface{}); ok {
				for _, w := range ws {
					if s, ok := w.(string); ok && s != "" {
						warnings = append(warnings, s)
					}
				}
			}
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			relX = string(relArtifact.Key.VariableX)
			relY = string(relArtifact.Key.VariableY)
			relEffectSize = relArtifact.Metrics.EffectSize
			relPValue = relArtifact.Metrics.PValue
			relTestType = string(relArtifact.Key.TestType)
			relSampleSize = relArtifact.Metrics.SampleSize
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			relX = string(relPayload.VariableX)
			relY = string(relPayload.VariableY)
			relEffectSize = relPayload.EffectSize
			relPValue = relPayload.PValue
			relQValue = relPayload.QValue
			relTestType = string(relPayload.TestType)
			relSampleSize = relPayload.SampleSize
		}

		if relX != "" && relY != "" {
			strengthDesc := "Unknown"
			if relEffectSize >= 0.7 {
				strengthDesc = "Very Strong"
			} else if relEffectSize >= 0.5 {
				strengthDesc = "Strong"
			} else if relEffectSize >= 0.3 {
				strengthDesc = "Moderate"
			} else if relEffectSize > 0 {
				strengthDesc = "Weak"
			}

			// Determine significance: p-value < 0.05 AND (q-value < 0.05 if available)
			significant := relPValue > 0 && relPValue < 0.05
			if relQValue > 0 {
				significant = significant && relQValue < 0.05
			}

			isShadow := relPValue > 0 && relPValue < 0.05 && (relQValue == 0 || relQValue >= 0.05)
			relationships = append(relationships, FieldRelationship{
				FieldX:       relX,
				FieldY:       relY,
				EffectSize:   relEffectSize,
				PValue:       relPValue,
				QValue:       relQValue,
				TestType:     relTestType,
				SampleSize:   relSampleSize,
				Significant:  significant,
				StrengthDesc: strengthDesc,
				IsShadow:     isShadow,
			})

			// Exclusion rationale:
			// - If it misses q threshold but passes p, mark as BH veto.
			// - If it misses p, mark as FILTERED_BY_P (still useful when shadow toggle on).
			if isShadow {
				exclusions = append(exclusions, ExclusionRow{
					Pair:      relX + " ← " + relY,
					VetoLogic: "ERR_FDR_BH",
					Metric:    "q ≥ 0.05 (BH)",
					PValue:    relPValue,
					QValue:    relQValue,
					SampleN:   relSampleSize,
				})
			} else if !significant {
				veto := "ERR_PVALUE"
				metric := "p ≥ 0.05"
				if len(warnings) > 0 {
					veto = "ERR_" + strings.ToUpper(warnings[0])
					metric = "warnings: " + strings.Join(warnings, ",")
				}
				exclusions = append(exclusions, ExclusionRow{
					Pair:      relX + " ← " + relY,
					VetoLogic: veto,
					Metric:    metric,
					PValue:    relPValue,
					QValue:    relQValue,
					SampleN:   relSampleSize,
				})
			}
		}
	}

	// Sort for UX: validated first, then shadow, then filtered.
	sort.Slice(relationships, func(i, j int) bool {
		if relationships[i].Significant != relationships[j].Significant {
			return relationships[i].Significant
		}
		if relationships[i].IsShadow != relationships[j].IsShadow {
			return relationships[i].IsShadow
		}
		if relationships[i].PValue != relationships[j].PValue {
			return relationships[i].PValue < relationships[j].PValue
		}
		return relationships[i].EffectSize > relationships[j].EffectSize
	})

	validated := 0
	shadow := 0
	for _, rel := range relationships {
		if rel.Significant {
			validated++
		} else if rel.IsShadow {
			shadow++
		}
	}

	data := map[string]interface{}{
		"Title":              "Relationships - GoHypo",
		"FieldRelationships": relationships,
		"RelationshipCount":  len(relationships),
		"ValidatedCount":     validated,
		"ShadowCount":        shadow,
		"ExcludedCount":      len(exclusions),
		"Exclusions":         exclusions,
		"ShowShadow":         showShadow,
		"SignificanceRule":   "q ≤ 0.05 (BH) + p ≤ 0.05",
	}

	a.renderTemplate(w, "relationships.html", data)
}

// handleRelationshipsJSON returns relationships as JSON for D3.js
func (a *App) handleRelationshipsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get relationship artifacts
	relKind := core.ArtifactRelationship
	artifacts, err := a.reader.GetArtifactsByKind(r.Context(), relKind, 1000)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to load relationships"})
		return
	}

	// Build nodes and links for D3.js
	nodes := make(map[string]bool)
	var links []map[string]interface{}

	for _, artifact := range artifacts {
		if artifact.Kind != relKind {
			continue
		}

		var relX, relY string
		var relEffectSize, relPValue float64

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
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			relX = string(relArtifact.Key.VariableX)
			relY = string(relArtifact.Key.VariableY)
			relEffectSize = relArtifact.Metrics.EffectSize
			relPValue = relArtifact.Metrics.PValue
		}

		if relX != "" && relY != "" && relPValue > 0 && relPValue < 0.05 {
			nodes[relX] = true
			nodes[relY] = true
			links = append(links, map[string]interface{}{
				"source": relX,
				"target": relY,
				"value":  relEffectSize,
			})
		}
	}

	nodeList := make([]map[string]interface{}, 0, len(nodes))
	for node := range nodes {
		nodeList = append(nodeList, map[string]interface{}{"id": node})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodeList,
		"links": links,
	})
}

// handleRelationshipsTable returns relationships table rows for HTMX
func (a *App) handleRelationshipsTable(w http.ResponseWriter, r *http.Request) {
	// Get relationship artifacts
	relKind := core.ArtifactRelationship
	artifacts, err := a.reader.GetArtifactsByKind(r.Context(), relKind, 1000)
	if err != nil {
		http.Error(w, "Failed to load relationships", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Artifacts": artifacts,
	}

	a.renderTemplate(w, "relationships_table.html", data)
}

// handleFieldsList returns all unique fields/variables
func (a *App) handleFieldsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get all artifacts to extract unique variables
	allArtifacts, err := a.reader.ListArtifacts(r.Context(), ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to load artifacts"})
		return
	}

	fieldSet := make(map[string]bool)
	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok && vx != "" {
					fieldSet[vx] = true
				}
				if vy, ok := payload["variable_y"].(string); ok && vy != "" {
					fieldSet[vy] = true
				}
			}
		} else if artifact.Kind == core.ArtifactVariableProfile {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vk, ok := payload["variable_key"].(string); ok && vk != "" {
					fieldSet[vk] = true
				}
			}
		}
	}

	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"fields": fields,
		"count":  len(fields),
	})
}
