// Package dataset provides intelligent relationship discovery and automatic merging
// for building a web of data understanding within workspaces.
package dataset

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"gohypo/ai"
	"gohypo/domain/core"
	domainDataset "gohypo/domain/dataset"
	"gohypo/ports"

	"github.com/jmoiron/sqlx"
)

// RelationshipDiscoveryEngine automatically discovers and creates relationships between datasets
type RelationshipDiscoveryEngine struct {
	forensicScout *ai.ForensicScout
	datasetRepo   ports.DatasetRepository
	workspaceRepo ports.WorkspaceRepository
	merger        *Merger
	db            *sqlx.DB
}

// DiscoveryResult represents the result of relationship discovery
type DiscoveryResult struct {
	WorkspaceID      core.ID                          `json:"workspace_id"`
	Relationships    []*domainDataset.DatasetRelation `json:"relationships"`
	MergeSuggestions []MergeSuggestion                `json:"merge_suggestions"`
	ConfidenceScore  float64                          `json:"confidence_score"`
	AnalysisTime     int64                            `json:"analysis_time_ms"`
}

// MergeSuggestion represents an automatic merge recommendation
type MergeSuggestion struct {
	SourceDatasets   []core.ID `json:"source_datasets"`
	MergeType        MergeType `json:"merge_type"`
	Confidence       float64   `json:"confidence"`
	Reasoning        string    `json:"reasoning"`
	ExpectedRowCount int       `json:"expected_row_count"`
	ExpectedColumns  int       `json:"expected_columns"`
}

// MergeType defines different types of automatic merges
type MergeType string

const (
	AutoUnion       MergeType = "auto_union"       // Simple concatenation
	AutoJoin        MergeType = "auto_join"        // Key-based join
	AutoAppend      MergeType = "auto_append"      // Schema-compatible append
	AutoConsolidate MergeType = "auto_consolidate" // Remove duplicates
)

// NewRelationshipDiscoveryEngine creates a new relationship discovery engine
func NewRelationshipDiscoveryEngine(
	forensicScout *ai.ForensicScout,
	datasetRepo ports.DatasetRepository,
	workspaceRepo ports.WorkspaceRepository,
	merger *Merger,
	db *sqlx.DB,
) *RelationshipDiscoveryEngine {
	return &RelationshipDiscoveryEngine{
		forensicScout: forensicScout,
		datasetRepo:   datasetRepo,
		workspaceRepo: workspaceRepo,
		merger:        merger,
		db:            db,
	}
}

// DiscoverRelationships analyzes all datasets in a workspace and discovers relationships
func (rde *RelationshipDiscoveryEngine) DiscoverRelationships(ctx context.Context, workspaceID core.ID) (*DiscoveryResult, error) {
	return rde.DiscoverRelationshipsWithOptions(ctx, workspaceID, &DiscoveryOptions{
		ClearExisting: false, // Default to not clearing existing relationships
	})
}

// DiscoveryOptions configures relationship discovery behavior
type DiscoveryOptions struct {
	ClearExisting bool // Whether to clear existing relationships before discovery
}

// DiscoverRelationshipsWithOptions analyzes datasets with configurable options
func (rde *RelationshipDiscoveryEngine) DiscoverRelationshipsWithOptions(ctx context.Context, workspaceID core.ID, options *DiscoveryOptions) (*DiscoveryResult, error) {
	if options == nil {
		options = &DiscoveryOptions{}
	}

	startTime := time.Now()

	// Optionally clear existing relationships
	if options.ClearExisting {
		err := rde.clearExistingRelationships(ctx, workspaceID)
		if err != nil {
			log.Printf("[DiscoverRelationships] Warning: Failed to clear existing relationships: %v", err)
		}
	}

	// Get all datasets in the workspace
	datasets, err := rde.datasetRepo.GetByWorkspace(ctx, workspaceID, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace datasets: %w", err)
	}

	// Filter out datasets with invalid IDs
	validDatasets := make([]*domainDataset.Dataset, 0, len(datasets))
	for _, ds := range datasets {
		if ds != nil && ds.ID != "" && ds.WorkspaceID != "" {
			validDatasets = append(validDatasets, ds)
		} else {
			log.Printf("[DiscoverRelationships] Skipping dataset with invalid ID: %v (workspace: %v)", ds.ID, ds.WorkspaceID)
		}
	}

	if len(validDatasets) < 2 {
		return &DiscoveryResult{
			WorkspaceID:      workspaceID,
			Relationships:    []*domainDataset.DatasetRelation{},
			MergeSuggestions: []MergeSuggestion{},
			ConfidenceScore:  0,
			AnalysisTime:     time.Since(startTime).Milliseconds(),
		}, nil
	}

	// Analyze all pairwise relationships
	relationships := rde.analyzePairwiseRelationships(validDatasets)

	// Generate merge suggestions based on relationships
	mergeSuggestions := rde.generateMergeSuggestions(validDatasets, relationships)

	// Calculate overall confidence score
	confidenceScore := rde.calculateOverallConfidence(relationships, mergeSuggestions)

	return &DiscoveryResult{
		WorkspaceID:      workspaceID,
		Relationships:    relationships,
		MergeSuggestions: mergeSuggestions,
		ConfidenceScore:  confidenceScore,
		AnalysisTime:     time.Since(startTime).Milliseconds(),
	}, nil
}

// clearExistingRelationships removes all stored relationships for a workspace
func (rde *RelationshipDiscoveryEngine) clearExistingRelationships(ctx context.Context, workspaceID core.ID) error {
	// Get all existing relationships for this workspace
	existingRelations, err := rde.workspaceRepo.GetRelations(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get existing relations: %w", err)
	}

	// Delete each relationship
	for _, relation := range existingRelations {
		err := rde.workspaceRepo.DeleteRelation(ctx, relation.ID)
		if err != nil {
			log.Printf("[clearExistingRelationships] Failed to delete relation %s: %v", relation.ID, err)
		}
	}

	log.Printf("[clearExistingRelationships] Cleared %d existing relationships for workspace %s", len(existingRelations), workspaceID)
	return nil
}

// analyzePairwiseRelationships examines relationships between every pair of datasets
func (rde *RelationshipDiscoveryEngine) analyzePairwiseRelationships(datasets []*domainDataset.Dataset) []*domainDataset.DatasetRelation {
	var relationships []*domainDataset.DatasetRelation

	// Analyze each pair of datasets
	for i := 0; i < len(datasets); i++ {
		for j := i + 1; j < len(datasets); j++ {
			dataset1 := datasets[i]
			dataset2 := datasets[j]

			// Skip if already analyzed (check database)
			if existing := rde.getExistingRelationship(dataset1.ID, dataset2.ID); existing != nil {
				relationships = append(relationships, existing)
				continue
			}

			// Analyze the relationship
			relationship := rde.analyzeDatasetPair(dataset1, dataset2)
			if relationship != nil {
				relationships = append(relationships, relationship)

				// Store the relationship
				rde.storeRelationship(relationship)
			}
		}
	}

	return relationships
}

// analyzeDatasetPair analyzes the relationship between two specific datasets
func (rde *RelationshipDiscoveryEngine) analyzeDatasetPair(ds1, ds2 *domainDataset.Dataset) *domainDataset.DatasetRelation {
	// Multiple analysis strategies
	strategies := []func(*domainDataset.Dataset, *domainDataset.Dataset) *domainDataset.DatasetRelation{
		rde.analyzeSchemaCompatibility,
		rde.analyzeSemanticSimilarity,
		rde.analyzeKeyRelationships,
		rde.analyzeTemporalPatterns,
		rde.analyzeTimeseriesCompatibility, // New timeseries-specific analysis
	}

	var bestRelationship *domainDataset.DatasetRelation
	bestConfidence := 0.0

	for _, strategy := range strategies {
		relationship := strategy(ds1, ds2)
		if relationship != nil && relationship.Confidence > bestConfidence {
			bestRelationship = relationship
			bestConfidence = relationship.Confidence
		}
	}

	return bestRelationship
}

// analyzeSchemaCompatibility checks if datasets have compatible schemas
func (rde *RelationshipDiscoveryEngine) analyzeSchemaCompatibility(ds1, ds2 *domainDataset.Dataset) *domainDataset.DatasetRelation {
	// Skip if no forensic scout available
	if rde.forensicScout == nil {
		return rde.fallbackSchemaCompatibility(ds1, ds2)
	}

	// Extract field names for AI analysis
	fields1 := make([]string, len(ds1.Metadata.Fields))
	for i, field := range ds1.Metadata.Fields {
		fields1[i] = field.Name
	}

	fields2 := make([]string, len(ds2.Metadata.Fields))
	for i, field := range ds2.Metadata.Fields {
		fields2[i] = field.Name
	}

	// Use enhanced schema compatibility analysis
	schemaResult, err := rde.forensicScout.AnalyzeSchemaCompatibility(context.Background(), fields1, fields2)
	if err != nil {
		log.Printf("[RelationshipDiscoveryEngine] Schema compatibility analysis failed: %v", err)
		return rde.fallbackSchemaCompatibility(ds1, ds2)
	}

	if schemaResult.CompatibilityScore > 0.3 {
		var relationType string
		switch schemaResult.RelationshipType {
		case "APPENDABLE", "UNIONABLE":
			relationType = "schema_match"
		case "JOINABLE":
			relationType = "potential_join"
		default:
			relationType = "weak_schema_match"
		}

		return &domainDataset.DatasetRelation{
			WorkspaceID:     ds1.WorkspaceID,
			SourceDatasetID: ds1.ID,
			TargetDatasetID: ds2.ID,
			RelationType:    relationType,
			Confidence:      schemaResult.CompatibilityScore,
			Metadata: map[string]interface{}{
				"common_fields":       schemaResult.CommonFields,
				"compatibility_score": schemaResult.CompatibilityScore,
				"relationship_type":   schemaResult.RelationshipType,
				"merge_strategy":      schemaResult.MergeStrategy,
				"issues":              schemaResult.Issues,
				"analysis_type":       "enhanced_schema_compatibility",
			},
			DiscoveredAt: time.Now(),
		}
	}

	return nil
}

// fallbackSchemaCompatibility provides basic schema compatibility when AI is unavailable
func (rde *RelationshipDiscoveryEngine) fallbackSchemaCompatibility(ds1, ds2 *domainDataset.Dataset) *domainDataset.DatasetRelation {
	fields1 := ds1.Metadata.Fields
	fields2 := ds2.Metadata.Fields

	// Check for exact column name matches
	commonFields := 0
	totalFields := len(fields1) + len(fields2)

	fieldMap1 := make(map[string]bool)
	for _, field := range fields1 {
		fieldMap1[strings.ToLower(field.Name)] = true
	}

	for _, field := range fields2 {
		if fieldMap1[strings.ToLower(field.Name)] {
			commonFields++
		}
	}

	if commonFields == 0 {
		return nil
	}

	// Calculate compatibility score
	compatibility := float64(commonFields*2) / float64(totalFields)

	var relationType string
	var confidence float64

	if compatibility > 0.8 {
		relationType = "schema_match"
		confidence = compatibility
	} else if compatibility > 0.5 {
		relationType = "partial_schema_match"
		confidence = compatibility * 0.8
	} else {
		relationType = "weak_schema_match"
		confidence = compatibility * 0.6
	}

	return &domainDataset.DatasetRelation{
		WorkspaceID:     ds1.WorkspaceID,
		SourceDatasetID: ds1.ID,
		TargetDatasetID: ds2.ID,
		RelationType:    relationType,
		Confidence:      confidence,
		Metadata: map[string]interface{}{
			"common_fields":       commonFields,
			"compatibility_score": compatibility,
			"analysis_type":       "fallback_schema_compatibility",
		},
		DiscoveredAt: time.Now(),
	}
}

// analyzeSemanticSimilarity uses AI to understand if datasets are semantically related
func (rde *RelationshipDiscoveryEngine) analyzeSemanticSimilarity(ds1, ds2 *domainDataset.Dataset) *domainDataset.DatasetRelation {
	// Skip semantic analysis if no forensic scout available
	if rde.forensicScout == nil {
		return nil
	}

	// Extract field names for AI analysis
	fields1 := make([]string, len(ds1.Metadata.Fields))
	for i, field := range ds1.Metadata.Fields {
		fields1[i] = field.Name
	}

	fields2 := make([]string, len(ds2.Metadata.Fields))
	for i, field := range ds2.Metadata.Fields {
		fields2[i] = field.Name
	}

	// Use enhanced semantic similarity analysis
	semanticResult, err := rde.forensicScout.AnalyzeSemanticSimilarity(context.Background(), fields1, fields2)
	if err != nil {
		log.Printf("[RelationshipDiscoveryEngine] Semantic analysis failed: %v", err)
		return nil
	}

	if semanticResult.SemanticSimilarity > 0.6 {
		return &domainDataset.DatasetRelation{
			WorkspaceID:     ds1.WorkspaceID,
			SourceDatasetID: ds1.ID,
			TargetDatasetID: ds2.ID,
			RelationType:    "semantic_similarity",
			Confidence:      semanticResult.SemanticSimilarity,
			Metadata: map[string]interface{}{
				"domain1":                semanticResult.Dataset1Domain,
				"domain2":                semanticResult.Dataset2Domain,
				"entities1":              semanticResult.Dataset1Entities,
				"entities2":              semanticResult.Dataset2Entities,
				"relationship_type":      semanticResult.RelationshipType,
				"business_context":       semanticResult.BusinessContext,
				"integration_rec":        semanticResult.IntegrationRecommendation,
				"quality_considerations": semanticResult.QualityConsiderations,
				"analysis_type":          "enhanced_semantic_similarity",
			},
			DiscoveredAt: time.Now(),
		}
	}

	return nil
}

// analyzeKeyRelationships looks for foreign key relationships
func (rde *RelationshipDiscoveryEngine) analyzeKeyRelationships(ds1, ds2 *domainDataset.Dataset) *domainDataset.DatasetRelation {
	// This would analyze actual data to find key relationships
	// For now, return a placeholder
	return nil
}

// analyzeTemporalPatterns looks for time-based relationships
func (rde *RelationshipDiscoveryEngine) analyzeTemporalPatterns(ds1, ds2 *domainDataset.Dataset) *domainDataset.DatasetRelation {
	// Check if both datasets have temporal fields
	hasTime1 := rde.hasTemporalFields(ds1)
	hasTime2 := rde.hasTemporalFields(ds2)

	if hasTime1 && hasTime2 {
		return &domainDataset.DatasetRelation{
			WorkspaceID:     ds1.WorkspaceID,
			SourceDatasetID: ds1.ID,
			TargetDatasetID: ds2.ID,
			RelationType:    "temporal_relationship",
			Confidence:      0.6,
			Metadata: map[string]interface{}{
				"analysis_type":  "temporal_patterns",
				"both_have_time": true,
			},
			DiscoveredAt: time.Now(),
		}
	}

	return nil
}

// analyzeTimeseriesCompatibility performs comprehensive timeseries analysis
func (rde *RelationshipDiscoveryEngine) analyzeTimeseriesCompatibility(ds1, ds2 *domainDataset.Dataset) *domainDataset.DatasetRelation {
	// Check if both datasets have temporal characteristics
	hasTime1 := rde.hasTemporalFields(ds1)
	hasTime2 := rde.hasTemporalFields(ds2)

	if !hasTime1 || !hasTime2 {
		return nil
	}

	// Look for common temporal patterns
	fields1 := make([]string, len(ds1.Metadata.Fields))
	for i, field := range ds1.Metadata.Fields {
		fields1[i] = field.Name
	}

	fields2 := make([]string, len(ds2.Metadata.Fields))
	for i, field := range ds2.Metadata.Fields {
		fields2[i] = field.Name
	}

	// Find time columns
	timeCol1 := rde.detectTimeColumn(fields1)
	timeCol2 := rde.detectTimeColumn(fields2)

	if timeCol1 == "" || timeCol2 == "" {
		return nil
	}

	// Check if time columns have the same name (strong indicator of compatibility)
	timeColumnsCompatible := timeCol1 == timeCol2

	// Determine confidence based on compatibility factors
	confidence := 0.7
	recommendedStrategy := "temporal_join"

	if timeColumnsCompatible {
		confidence = 0.9
		recommendedStrategy = "timeseries_union"
	}

	// Check for frequency compatibility
	freq1 := rde.inferFrequency(fields1)
	freq2 := rde.inferFrequency(fields2)

	frequencyMatch := false
	if freq1 != "" && freq2 != "" && freq1 == freq2 {
		confidence += 0.1
		frequencyMatch = true
	}

	// Determine merge strategy based on analysis
	mergeStrategy := "auto_join"
	if timeColumnsCompatible && frequencyMatch {
		mergeStrategy = "auto_append" // Same time column and frequency suggests append
	} else if timeColumnsCompatible {
		mergeStrategy = "auto_join" // Same time column, different frequency suggests join
	}

	// Analyze potential data overlap and gaps
	dataCharacteristics := rde.analyzeTimeseriesCharacteristics(fields1, fields2)

	return &domainDataset.DatasetRelation{
		WorkspaceID:     ds1.WorkspaceID,
		SourceDatasetID: ds1.ID,
		TargetDatasetID: ds2.ID,
		RelationType:    "timeseries_merge_candidate",
		Confidence:      confidence,
		Metadata: map[string]interface{}{
			"analysis_type":            "timeseries_compatibility",
			"time_column_1":            timeCol1,
			"time_column_2":            timeCol2,
			"time_columns_compatible":  timeColumnsCompatible,
			"inferred_frequency_1":     freq1,
			"inferred_frequency_2":     freq2,
			"frequency_match":          frequencyMatch,
			"recommended_merge_type":   recommendedStrategy,
			"merge_strategy":           mergeStrategy,
			"temporal_alignment":       "timestamp_based",
			"data_characteristics":     dataCharacteristics,
			"expected_gap_handling":    "forward_fill",
			"timezone_normalization":   "UTC",
		},
		DiscoveredAt: time.Now(),
	}
}

// analyzeTimeseriesCharacteristics analyzes the characteristics of timeseries datasets
func (rde *RelationshipDiscoveryEngine) analyzeTimeseriesCharacteristics(fields1, fields2 []string) map[string]interface{} {
	characteristics := make(map[string]interface{})

	// Analyze numeric columns (potential metrics)
	numericCols1 := rde.countNumericColumns(fields1)
	numericCols2 := rde.countNumericColumns(fields2)

	characteristics["numeric_columns_1"] = numericCols1
	characteristics["numeric_columns_2"] = numericCols2
	characteristics["numeric_columns_match"] = numericCols1 == numericCols2

	// Analyze categorical columns (potential dimensions)
	categoricalCols1 := rde.countCategoricalColumns(fields1)
	categoricalCols2 := rde.countCategoricalColumns(fields2)

	characteristics["categorical_columns_1"] = categoricalCols1
	characteristics["categorical_columns_2"] = categoricalCols2
	characteristics["categorical_columns_match"] = categoricalCols1 == categoricalCols2

	// Check for common column patterns
	commonCols := rde.findCommonColumns(fields1, fields2)
	characteristics["common_columns"] = commonCols
	characteristics["common_columns_count"] = len(commonCols)

	// Assess merge complexity
	if len(commonCols) > 0 && numericCols1 == numericCols2 {
		characteristics["merge_complexity"] = "low"
		characteristics["recommended_approach"] = "direct_merge"
	} else if len(commonCols) > 0 {
		characteristics["merge_complexity"] = "medium"
		characteristics["recommended_approach"] = "align_and_merge"
	} else {
		characteristics["merge_complexity"] = "high"
		characteristics["recommended_approach"] = "manual_review"
	}

	return characteristics
}

// countNumericColumns estimates numeric columns based on naming patterns
func (rde *RelationshipDiscoveryEngine) countNumericColumns(fields []string) int {
	count := 0
	numericPatterns := []string{"count", "amount", "value", "price", "cost", "rate", "ratio", "percentage", "score", "metric", "quantity"}

	for _, field := range fields {
		fieldLower := strings.ToLower(field)
		for _, pattern := range numericPatterns {
			if strings.Contains(fieldLower, pattern) {
				count++
				break
			}
		}
	}

	return count
}

// countCategoricalColumns estimates categorical columns
func (rde *RelationshipDiscoveryEngine) countCategoricalColumns(fields []string) int {
	count := 0
	categoricalPatterns := []string{"name", "type", "category", "class", "group", "status", "state", "id", "code"}

	for _, field := range fields {
		fieldLower := strings.ToLower(field)
		for _, pattern := range categoricalPatterns {
			if strings.Contains(fieldLower, pattern) {
				count++
				break
			}
		}
	}

	return count
}

// findCommonColumns finds columns with similar names
func (rde *RelationshipDiscoveryEngine) findCommonColumns(fields1, fields2 []string) []string {
	common := []string{}

	for _, field1 := range fields1 {
		field1Lower := strings.ToLower(strings.ReplaceAll(field1, "_", ""))

		for _, field2 := range fields2 {
			field2Lower := strings.ToLower(strings.ReplaceAll(field2, "_", ""))

			// Check for exact match or high similarity
			if field1Lower == field2Lower {
				common = append(common, field1)
				break
			}

			// Check for partial matches
			if len(field1Lower) > 3 && len(field2Lower) > 3 {
				if strings.Contains(field1Lower, field2Lower) || strings.Contains(field2Lower, field1Lower) {
					common = append(common, field1)
					break
				}
			}
		}
	}

	return common
}

// detectTimeColumn identifies the most likely timestamp column
func (rde *RelationshipDiscoveryEngine) detectTimeColumn(headers []string) string {
	priorityPatterns := []string{
		"timestamp", "datetime", "time", "date",
		"created_at", "updated_at", "occurred_at", "recorded_at",
		"period", "event_time", "transaction_time",
	}

	for _, pattern := range priorityPatterns {
		for _, header := range headers {
			if strings.ToLower(header) == pattern {
				return header
			}
		}
	}

	// Fallback to any column containing temporal keywords
	temporalKeywords := []string{"time", "date", "created", "updated", "occurred"}
	for _, header := range headers {
		headerLower := strings.ToLower(header)
		for _, keyword := range temporalKeywords {
			if strings.Contains(headerLower, keyword) {
				return header
			}
		}
	}

	return ""
}

// inferFrequency attempts to infer data frequency from column names
func (rde *RelationshipDiscoveryEngine) inferFrequency(headers []string) string {
	frequencyIndicators := map[string]string{
		"hourly":   "hour",
		"daily":    "day",
		"weekly":   "week",
		"monthly":  "month",
		"yearly":   "year",
		"annual":   "year",
		"quarterly": "month", // Approximate
	}

	for _, header := range headers {
		headerLower := strings.ToLower(header)
		for indicator, freq := range frequencyIndicators {
			if strings.Contains(headerLower, indicator) {
				return freq
			}
		}
	}

	return ""
}

// generateMergeSuggestions creates automatic merge recommendations
func (rde *RelationshipDiscoveryEngine) generateMergeSuggestions(datasets []*domainDataset.Dataset, relationships []*domainDataset.DatasetRelation) []MergeSuggestion {
	var suggestions []MergeSuggestion

	// Group datasets by relationship strength
	relationshipMap := make(map[string][]*domainDataset.DatasetRelation)
	for _, rel := range relationships {
		key := fmt.Sprintf("%s-%s", rel.SourceDatasetID, rel.TargetDatasetID)
		relationshipMap[key] = append(relationshipMap[key], rel)
	}

	// Find strongly related dataset pairs
	for _, dataset := range datasets {
		relatedDatasets := rde.findStronglyRelatedDatasets(dataset.ID, relationships)

		if len(relatedDatasets) > 0 {
			suggestion := rde.createMergeSuggestion(dataset, relatedDatasets, relationships)
			if suggestion.Confidence > 0.8 { // Only suggest high-confidence merges
				suggestions = append(suggestions, suggestion)
			}
		}
	}

	// Sort by confidence
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Confidence > suggestions[j].Confidence
	})

	return suggestions
}

// createMergeSuggestion builds a merge suggestion for related datasets
func (rde *RelationshipDiscoveryEngine) createMergeSuggestion(primary *domainDataset.Dataset, related []*domainDataset.Dataset, relationships []*domainDataset.DatasetRelation) MergeSuggestion {
	allDatasets := append([]*domainDataset.Dataset{primary}, related...)
	sourceIDs := make([]core.ID, len(allDatasets))
	for i, ds := range allDatasets {
		sourceIDs[i] = ds.ID
	}

	// Determine merge type based on relationships
	mergeType := rde.determineMergeType(allDatasets, relationships)

	// Calculate expected dimensions
	expectedRows, expectedCols := rde.calculateExpectedDimensions(allDatasets, mergeType)

	// Calculate confidence
	confidence := rde.calculateMergeConfidence(allDatasets, relationships)

	return MergeSuggestion{
		SourceDatasets:   sourceIDs,
		MergeType:        mergeType,
		Confidence:       confidence,
		Reasoning:        rde.generateMergeReasoning(allDatasets, relationships),
		ExpectedRowCount: expectedRows,
		ExpectedColumns:  expectedCols,
	}
}

// Helper methods

func (rde *RelationshipDiscoveryEngine) calculateStringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	// Simple Jaccard similarity for words
	words1 := strings.Fields(strings.ToLower(s1))
	words2 := strings.Fields(strings.ToLower(s2))

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}

	wordSet1 := make(map[string]bool)
	wordSet2 := make(map[string]bool)

	for _, word := range words1 {
		wordSet1[word] = true
	}
	for _, word := range words2 {
		wordSet2[word] = true
	}

	intersection := 0
	for word := range wordSet1 {
		if wordSet2[word] {
			intersection++
		}
	}

	union := len(wordSet1) + len(wordSet2) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (rde *RelationshipDiscoveryEngine) hasTemporalFields(ds *domainDataset.Dataset) bool {
	temporalKeywords := []string{"date", "time", "timestamp", "created", "updated", "year", "month", "day"}

	for _, field := range ds.Metadata.Fields {
		fieldName := strings.ToLower(field.Name)
		for _, keyword := range temporalKeywords {
			if strings.Contains(fieldName, keyword) {
				return true
			}
		}
	}
	return false
}

func (rde *RelationshipDiscoveryEngine) findStronglyRelatedDatasets(datasetID core.ID, relationships []*domainDataset.DatasetRelation) []*domainDataset.Dataset {
	// Build adjacency list for graph traversal
	graph := make(map[core.ID][]core.ID)
	confidenceMap := make(map[string]float64)

	for _, rel := range relationships {
		if rel.Confidence >= 0.7 { // Only consider strong relationships
			key1 := fmt.Sprintf("%s-%s", rel.SourceDatasetID, rel.TargetDatasetID)
			key2 := fmt.Sprintf("%s-%s", rel.TargetDatasetID, rel.SourceDatasetID)

			graph[rel.SourceDatasetID] = append(graph[rel.SourceDatasetID], rel.TargetDatasetID)
			graph[rel.TargetDatasetID] = append(graph[rel.TargetDatasetID], rel.SourceDatasetID)

			confidenceMap[key1] = rel.Confidence
			confidenceMap[key2] = rel.Confidence
		}
	}

	// Find connected components using BFS
	visited := make(map[core.ID]bool)
	var connectedDatasets []core.ID

	queue := []core.ID{datasetID}
	visited[datasetID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range graph[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				connectedDatasets = append(connectedDatasets, neighbor)
				queue = append(queue, neighbor)
			}
		}
	}

	// Convert IDs to Dataset objects (simplified - in practice we'd query the database)
	var result []*domainDataset.Dataset
	for _, id := range connectedDatasets {
		// This is a placeholder - we'd need to fetch actual dataset objects
		// For now, create minimal dataset objects
		result = append(result, &domainDataset.Dataset{ID: id})
	}

	return result
}

func (rde *RelationshipDiscoveryEngine) determineMergeType(datasets []*domainDataset.Dataset, relationships []*domainDataset.DatasetRelation) MergeType {
	if len(relationships) == 0 {
		return AutoUnion // Default fallback
	}

	// Count relationship types
	hasSchemaMatch := false
	hasSemanticMatch := false
	hasTemporalMatch := false
	hasTimeseriesMatch := false

	for _, rel := range relationships {
		switch rel.RelationType {
		case "schema_match", "partial_schema_match":
			hasSchemaMatch = true
		case "semantic_similarity":
			hasSemanticMatch = true
		case "temporal_relationship":
			hasTemporalMatch = true
		case "timeseries_merge_candidate":
			hasTimeseriesMatch = true
		}
	}

	// Prioritize timeseries relationships
	if hasTimeseriesMatch {
		// Timeseries data typically benefits from temporal joins
		return AutoJoin
	}

	// Determine merge type based on relationship patterns
	if hasSchemaMatch && hasSemanticMatch {
		// Strong schema and semantic matches suggest append/consolidate
		return AutoAppend
	} else if hasSchemaMatch {
		// Schema matches without strong semantic alignment suggest union
		return AutoUnion
	} else if hasSemanticMatch && hasTemporalMatch {
		// Semantic + temporal suggests potential join relationships
		return AutoJoin
	} else if len(datasets) > 2 && hasSemanticMatch {
		// Multiple datasets with semantic relationships might benefit from consolidation
		return AutoConsolidate
	}

	// Default to union for most cases
	return AutoUnion
}

func (rde *RelationshipDiscoveryEngine) calculateExpectedDimensions(datasets []*domainDataset.Dataset, mergeType MergeType) (int, int) {
	if len(datasets) == 0 {
		return 0, 0
	}

	totalRows := 0
	maxCols := 0

	// Sum up rows and find max columns based on merge type
	for _, ds := range datasets {
		if ds.RecordCount > 0 {
			totalRows += ds.RecordCount
		}
		if ds.FieldCount > maxCols {
			maxCols = ds.FieldCount
		}
	}

	switch mergeType {
	case AutoUnion:
		// Union typically keeps all rows and combines columns
		return totalRows, maxCols
	case AutoJoin:
		// Join might reduce rows due to matching criteria
		expectedRows := int(float64(totalRows) * 0.7) // Estimate 70% match rate
		return expectedRows, maxCols
	case AutoAppend:
		// Append combines rows with same columns
		return totalRows, maxCols
	case AutoConsolidate:
		// Consolidate might reduce rows by removing duplicates
		expectedRows := int(float64(totalRows) * 0.8) // Estimate 80% unique rows
		return expectedRows, maxCols
	default:
		return totalRows, maxCols
	}
}

func (rde *RelationshipDiscoveryEngine) calculateMergeConfidence(datasets []*domainDataset.Dataset, relationships []*domainDataset.DatasetRelation) float64 {
	if len(relationships) == 0 {
		return 0.0
	}

	totalConfidence := 0.0
	weightedSum := 0.0
	totalWeight := 0.0

	// Calculate weighted confidence based on relationship types
	for _, rel := range relationships {
		weight := 1.0
		switch rel.RelationType {
		case "schema_match":
			weight = 3.0 // Schema matches are strongest indicators
		case "semantic_similarity":
			weight = 2.5 // Semantic matches are very strong
		case "partial_schema_match":
			weight = 2.0 // Partial matches are good
		case "temporal_relationship":
			weight = 1.5 // Time-based relationships are moderate
		case "weak_schema_match":
			weight = 1.0 // Weak matches are minimal
		default:
			weight = 1.0
		}

		weightedSum += rel.Confidence * weight
		totalWeight += weight
		totalConfidence += rel.Confidence
	}

	// Return weighted average, capped at 0.95
	if totalWeight > 0 {
		weightedAvg := weightedSum / totalWeight
		if weightedAvg > 0.95 {
			return 0.95
		}
		return weightedAvg
	}

	// Fallback to simple average
	avgConfidence := totalConfidence / float64(len(relationships))
	if avgConfidence > 0.95 {
		return 0.95
	}
	return avgConfidence
}

func (rde *RelationshipDiscoveryEngine) generateMergeReasoning(datasets []*domainDataset.Dataset, relationships []*domainDataset.DatasetRelation) string {
	// Generate human-readable reasoning for the merge suggestion
	return "Datasets appear to be related based on schema and semantic analysis"
}

func (rde *RelationshipDiscoveryEngine) getExistingRelationship(id1, id2 core.ID) *domainDataset.DatasetRelation {
	// For now, we'll implement this as a check during pairwise analysis
	// In a full implementation, this would query the database by workspace ID
	// Since we don't have workspace ID here, return nil to force re-analysis
	return nil
}

func (rde *RelationshipDiscoveryEngine) storeRelationship(relationship *domainDataset.DatasetRelation) {
	// Validate relationship has valid UUIDs before storing
	if relationship == nil {
		log.Printf("[RelationshipDiscoveryEngine] Skipping nil relationship")
		return
	}
	if relationship.WorkspaceID == "" || relationship.SourceDatasetID == "" || relationship.TargetDatasetID == "" {
		log.Printf("[RelationshipDiscoveryEngine] Skipping relationship with empty UUIDs: workspace=%s, source=%s, target=%s",
			relationship.WorkspaceID, relationship.SourceDatasetID, relationship.TargetDatasetID)
		return
	}

	// Store relationship in database using workspace repository
	ctx := context.Background()
	err := rde.workspaceRepo.CreateRelation(ctx, relationship)
	if err != nil {
		log.Printf("[RelationshipDiscoveryEngine] Failed to store relationship: %v", err)
	}
}

func (rde *RelationshipDiscoveryEngine) calculateOverallConfidence(relationships []*domainDataset.DatasetRelation, suggestions []MergeSuggestion) float64 {
	if len(relationships) == 0 {
		return 0
	}

	totalConfidence := 0.0
	for _, rel := range relationships {
		totalConfidence += rel.Confidence
	}

	return totalConfidence / float64(len(relationships))
}
