// Package dataset provides analysis result types for AI-powered data analysis
package dataset

// SchemaCompatibilityResult represents the result of schema compatibility analysis
type SchemaCompatibilityResult struct {
	CompatibilityScore float64  `json:"compatibility_score"`
	CommonFields       []string `json:"common_fields"`
	RelationshipType   string   `json:"relationship_type"`
	MergeStrategy      string   `json:"merge_strategy"`
	Issues             []string `json:"issues"`
	Confidence         float64  `json:"confidence"`
	AnalysisDetails    string   `json:"analysis_details"`
}

// SemanticSimilarityResult represents semantic relationship analysis
type SemanticSimilarityResult struct {
	Dataset1Domain            string   `json:"dataset1_domain"`
	Dataset2Domain            string   `json:"dataset2_domain"`
	Dataset1Entities          []string `json:"dataset1_entities"`
	Dataset2Entities          []string `json:"dataset2_entities"`
	RelationshipType          string   `json:"relationship_type"`
	SemanticSimilarity        float64  `json:"semantic_similarity"`
	BusinessContext           string   `json:"business_context"`
	IntegrationRecommendation string   `json:"integration_recommendation"`
	QualityConsiderations     []string `json:"quality_considerations"`
	Confidence                float64  `json:"confidence"`
	AnalysisDetails           string   `json:"analysis_details"`
}

// KeyRelationshipResult represents foreign key and relationship analysis
type KeyRelationshipResult struct {
	Dataset1Keys         map[string]interface{} `json:"dataset1_keys"`
	Dataset2Keys         map[string]interface{} `json:"dataset2_keys"`
	RelationshipType     string                 `json:"relationship_type"`
	JoinKeys             []string               `json:"join_keys"`
	JoinStrategy         string                 `json:"join_strategy"`
	Cardinality          string                 `json:"cardinality"`
	ReferentialIntegrity string                 `json:"referential_integrity"`
	RelationshipStrength float64                `json:"relationship_strength"`
	Issues               []string               `json:"issues"`
	Confidence           float64                `json:"confidence"`
	AnalysisDetails      string                 `json:"analysis_details"`
}

// TemporalPatternResult represents time-based relationship analysis
type TemporalPatternResult struct {
	Dataset1Temporal     map[string]interface{} `json:"dataset1_temporal"`
	Dataset2Temporal     map[string]interface{} `json:"dataset2_temporal"`
	TemporalRelationship string                 `json:"temporal_relationship"`
	JoinOpportunities    []string               `json:"join_opportunities"`
	TemporalConsistency  string                 `json:"temporal_consistency"`
	BusinessValue        string                 `json:"business_value"`
	TemporalStrength     float64                `json:"temporal_strength"`
	Issues               []string               `json:"issues"`
	Confidence           float64                `json:"confidence"`
	AnalysisDetails      string                 `json:"analysis_details"`
}

// DatasetProfileResult represents comprehensive dataset profiling
type DatasetProfileResult struct {
	DomainClassification map[string]interface{} `json:"domain_classification"`
	FieldAnalysis        map[string]interface{} `json:"field_analysis"`
	DataQualityProfile   map[string]interface{} `json:"data_quality_profile"`
	StructuralPatterns   string                 `json:"structural_patterns"`
	BusinessProcesses    []string               `json:"business_processes"`
	IntegrationReadiness map[string]interface{} `json:"integration_readiness"`
	GovernanceProfile    map[string]interface{} `json:"governance_profile"`
	ProfilingConfidence  float64                `json:"profiling_confidence"`
	Insights             []string               `json:"insights"`
	Recommendations      []string               `json:"recommendations"`
	AnalysisDetails      string                 `json:"analysis_details"`
}

// MergeReasoningResult represents intelligent merge strategy recommendations
type MergeReasoningResult struct {
	IntegrationStrategy      string            `json:"integration_strategy"`
	MergeSequence            []string          `json:"merge_sequence"`
	MergeTypes               map[string]string `json:"merge_types"`
	TransformationsRequired  []string          `json:"transformations_required"`
	BusinessValue            string            `json:"business_value"`
	Risks                    []string          `json:"risks"`
	ImplementationComplexity int               `json:"implementation_complexity"`
	SuccessConfidence        float64           `json:"success_confidence"`
	Alternatives             []string          `json:"alternatives"`
	Recommendations          string            `json:"recommendations"`
	AnalysisDetails          string            `json:"analysis_details"`
}
