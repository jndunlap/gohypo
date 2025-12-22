package referee

import (
	"gohypo/domain/stats"
	"sort"
)

// DynamicSelector handles intelligent test selection based on hypothesis characteristics
type DynamicSelector struct {
	evalueCalibrator *EValueCalibrator
	categoryWeights  map[stats.RefereeCategory]float64
	testCosts        map[string]float64
	testReliability  map[string]float64
}

// NewDynamicSelector creates a new dynamic test selector
func NewDynamicSelector(evalueCalibrator *EValueCalibrator) *DynamicSelector {
	return &DynamicSelector{
		evalueCalibrator: evalueCalibrator,
		categoryWeights:  initializeCategoryWeights(),
		testCosts:        initializeTestCosts(),
		testReliability:  initializeTestReliability(),
	}
}

// SelectTests chooses optimal validation tests for a hypothesis
func (ds *DynamicSelector) SelectTests(profile stats.HypothesisProfile) ([]stats.SelectedTest, stats.SelectionRationale) {
	// Assess overall risk level
	riskLevel := ds.assessHypothesisRisk(profile)

	// Determine test count range
	minTests, maxTests := ds.GetTestCountRange(riskLevel)

	// Get required categories based on profile
	requiredCategories := ds.getRequiredCategories(profile)

	// Generate candidate tests
	candidates := ds.generateCandidateTests(requiredCategories, profile)

	// Optimize selection
	selectedTests := ds.optimizeSelection(candidates, minTests, maxTests, profile)

	// Calculate rationale
	rationale := ds.buildSelectionRationale(selectedTests, riskLevel, requiredCategories, minTests, maxTests)

	return selectedTests, rationale
}

// GetTestCountRange returns appropriate test count bounds for risk level
func (ds *DynamicSelector) GetTestCountRange(riskLevel stats.HypothesisRiskLevel) (minTests, maxTests int) {
	switch riskLevel {
	case stats.RiskLevelLow:
		return 1, 3
	case stats.RiskLevelMedium:
		return 3, 6
	case stats.RiskLevelHigh:
		return 6, 10
	case stats.RiskLevelVeryHigh:
		return 8, 10
	default:
		return 3, 6 // Safe default
	}
}

// assessHypothesisRisk evaluates overall hypothesis risk
func (ds *DynamicSelector) assessHypothesisRisk(profile stats.HypothesisProfile) stats.HypothesisRiskLevel {
	riskScore := 0.0

	// Data complexity contribution
	switch profile.DataComplexity {
	case stats.DataComplexityComplex:
		riskScore += 3.0
	case stats.DataComplexityModerate:
		riskScore += 1.5
	}

	// Effect size contribution (smaller effects need more validation)
	switch profile.EffectMagnitude {
	case stats.EffectSizeSmall:
		riskScore += 3.0
	case stats.EffectSizeMedium:
		riskScore += 1.5
	}

	// Sample size contribution (smaller samples need more validation)
	switch profile.SampleSize {
	case stats.SampleSizeSmall:
		riskScore += 2.0
	case stats.SampleSizeMedium:
		riskScore += 1.0
	}

	// Domain risk contribution
	switch profile.DomainRisk {
	case stats.DomainRiskCritical:
		riskScore += 4.0
	case stats.DomainRiskHigh:
		riskScore += 2.0
	case stats.DomainRiskMedium:
		riskScore += 1.0
	}

	// Temporal complexity contribution
	switch profile.TemporalNature {
	case stats.TemporalComplex:
		riskScore += 2.0
	case stats.TemporalSimple:
		riskScore += 1.0
	}

	// Confounding risk contribution
	switch profile.ConfoundingRisk {
	case stats.ConfoundingHigh:
		riskScore += 2.5
	case stats.ConfoundingMedium:
		riskScore += 1.5
	}

	// Convert risk score to risk level
	switch {
	case riskScore >= 8.0:
		return stats.RiskLevelVeryHigh
	case riskScore >= 5.0:
		return stats.RiskLevelHigh
	case riskScore >= 2.5:
		return stats.RiskLevelMedium
	default:
		return stats.RiskLevelLow
	}
}

// getRequiredCategories determines which test categories are required
func (ds *DynamicSelector) getRequiredCategories(profile stats.HypothesisProfile) []CategoryRequirement {
	requirements := []CategoryRequirement{}

	// Always require statistical integrity
	requirements = append(requirements, CategoryRequirement{
		Category:  stats.CategorySHREDDER,
		Priority:  stats.PriorityHigh,
		Rationale: "Statistical artifacts must always be ruled out first",
	})

	// Add based on hypothesis characteristics
	if profile.TemporalNature == stats.TemporalComplex {
		requirements = append(requirements, CategoryRequirement{
			Category:  stats.CategoryINVARIANCE,
			Priority:  stats.PriorityHigh,
			Rationale: "Temporal hypotheses require stability testing over time",
		})
	}

	if profile.ConfoundingRisk == stats.ConfoundingHigh {
		requirements = append(requirements, CategoryRequirement{
			Category:  stats.CategoryANTI_CONFOUNDER,
			Priority:  stats.PriorityHigh,
			Rationale: "High confounding risk requires control variable testing",
		})
	}

	if profile.DataComplexity == stats.DataComplexityComplex {
		requirements = append(requirements, CategoryRequirement{
			Category:  stats.CategoryMECHANISM,
			Priority:  stats.PriorityMedium,
			Rationale: "Complex data requires mechanism validation",
		})
	}

	if profile.DomainRisk >= stats.DomainRiskHigh {
		requirements = append(requirements, CategoryRequirement{
			Category:  stats.CategorySENSITIVITY,
			Priority:  stats.PriorityMedium,
			Rationale: "High-stakes domains require robustness testing",
		})
	}

	return requirements
}

// CategoryRequirement represents a required test category
type CategoryRequirement struct {
	Category  stats.RefereeCategory
	Priority  stats.TestPriority
	Rationale string
}

// generateCandidateTests creates all possible tests for the required categories
func (ds *DynamicSelector) generateCandidateTests(requirements []CategoryRequirement, profile stats.HypothesisProfile) []TestCandidate {
	candidates := []TestCandidate{}

	for _, req := range requirements {
		tests := ds.getTestsForCategory(req.Category)

		for _, test := range tests {
			candidate := TestCandidate{
				RefereeName:    test.Name,
				Category:       req.Category,
				Priority:       req.Priority,
				Rationale:      req.Rationale,
				Cost:           ds.getTestCost(test.Name),
				Reliability:    ds.getTestReliability(test.Name),
				ExpectedEValue: ds.estimateExpectedEValue(test.Name, profile),
			}
			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

// TestCandidate represents a potential test for selection
type TestCandidate struct {
	RefereeName    string
	Category       stats.RefereeCategory
	Priority       stats.TestPriority
	Rationale      string
	Cost           float64
	Reliability    float64
	ExpectedEValue float64
}

// optimizeSelection uses knapsack-like optimization to select best tests
func (ds *DynamicSelector) optimizeSelection(candidates []TestCandidate, minTests, maxTests int, profile stats.HypothesisProfile) []stats.SelectedTest {
	if len(candidates) <= maxTests {
		// If we have fewer candidates than max, select all
		return ds.convertCandidatesToSelected(candidates)
	}

	// Sort by efficiency (expected E-value per cost)
	sort.Slice(candidates, func(i, j int) bool {
		efficiencyI := candidates[i].ExpectedEValue / candidates[i].Cost * candidates[i].Reliability
		efficiencyJ := candidates[j].ExpectedEValue / candidates[j].Cost * candidates[j].Reliability
		return efficiencyI > efficiencyJ
	})

	// Select top candidates up to maxTests, ensuring minimum requirements
	selected := candidates[:maxTests]

	// Ensure minimum test count is met
	if len(selected) < minTests && len(candidates) >= minTests {
		selected = candidates[:minTests]
	}

	return ds.convertCandidatesToSelected(selected)
}

// convertCandidatesToSelected converts internal candidates to external format
func (ds *DynamicSelector) convertCandidatesToSelected(candidates []TestCandidate) []stats.SelectedTest {
	selected := make([]stats.SelectedTest, len(candidates))

	for i, candidate := range candidates {
		selected[i] = stats.SelectedTest{
			RefereeName:    candidate.RefereeName,
			Category:       candidate.Category,
			Priority:       candidate.Priority,
			Rationale:      candidate.Rationale,
			ExpectedEValue: candidate.ExpectedEValue,
		}
	}

	return selected
}

// buildSelectionRationale creates explanation for the selection
func (ds *DynamicSelector) buildSelectionRationale(selected []stats.SelectedTest, riskLevel stats.HypothesisRiskLevel, requirements []CategoryRequirement, minTests, maxTests int) stats.SelectionRationale {
	categoryCoverage := make(map[stats.RefereeCategory]float64)

	// Calculate category coverage
	for _, req := range requirements {
		categoryCoverage[req.Category] = 0.0
	}

	for _, test := range selected {
		if _, exists := categoryCoverage[test.Category]; exists {
			categoryCoverage[test.Category] = 1.0
		}
	}

	// Calculate efficiency score
	totalEfficiency := 0.0
	for _, test := range selected {
		cost := ds.getTestCost(test.RefereeName)
		reliability := ds.getTestReliability(test.RefereeName)
		if cost > 0 {
			totalEfficiency += (test.ExpectedEValue / cost) * reliability
		}
	}

	avgEfficiency := totalEfficiency / float64(len(selected))

	// Calculate expected threshold
	expectedThreshold := ds.evalueCalibrator.GetDynamicThreshold(len(selected), 0.8)

	return stats.SelectionRationale{
		RiskLevel:         riskLevel,
		CategoryCoverage:  categoryCoverage,
		EfficiencyScore:   avgEfficiency,
		ExpectedThreshold: expectedThreshold,
		TestCount:         len(selected),
		MinTests:          minTests,
		MaxTests:          maxTests,
	}
}

// getTestsForCategory returns available tests for a category
func (ds *DynamicSelector) getTestsForCategory(category stats.RefereeCategory) []TestInfo {
	// This would map categories to available tests
	// For now, return mock data - in reality this would be configurable
	switch category {
	case stats.CategorySHREDDER:
		return []TestInfo{
			{Name: "Permutation_Shredder", Description: "Statistical integrity test"},
			{Name: "Bootstrap_Validation", Description: "Resampling validation"},
		}
	case stats.CategoryDIRECTIONAL:
		return []TestInfo{
			{Name: "Transfer_Entropy", Description: "Information flow test"},
			{Name: "Convergent_Cross_Mapping", Description: "Causal embedding test"},
		}
	case stats.CategoryINVARIANCE:
		return []TestInfo{
			{Name: "Chow_Stability_Test", Description: "Structural stability test"},
			{Name: "CUSUM_Drift_Detection", Description: "Change point detection"},
		}
	case stats.CategoryANTI_CONFOUNDER:
		return []TestInfo{
			{Name: "Conditional_Mutual_Information", Description: "Confounder control test"},
			{Name: "Partial_Correlation", Description: "Conditional correlation test"},
		}
	case stats.CategoryMECHANISM:
		return []TestInfo{
			{Name: "Monotonicity_Stress_Test", Description: "Mechanism validation"},
			{Name: "Isotonic_Mechanism_Check", Description: "Functional form test"},
		}
	case stats.CategorySENSITIVITY:
		return []TestInfo{
			{Name: "Leave_One_Out_CV", Description: "Robustness test"},
			{Name: "Alpha_Decay_Test", Description: "Significance decay test"},
		}
	default:
		return []TestInfo{}
	}
}

// TestInfo represents basic test information
type TestInfo struct {
	Name        string
	Description string
}

// Helper methods for test properties
func (ds *DynamicSelector) getTestCost(testName string) float64 {
	if cost, exists := ds.testCosts[testName]; exists {
		return cost
	}
	return 1.0 // Default cost
}

func (ds *DynamicSelector) getTestReliability(testName string) float64 {
	if reliability, exists := ds.testReliability[testName]; exists {
		return reliability
	}
	return 0.8 // Default reliability
}

func (ds *DynamicSelector) estimateExpectedEValue(testName string, profile stats.HypothesisProfile) float64 {
	// Simplified estimation based on test type and hypothesis characteristics
	baseValue := 5.0 // Default expected E-value

	// Adjust based on effect size
	switch profile.EffectMagnitude {
	case stats.EffectSizeLarge:
		baseValue *= 2.0
	case stats.EffectSizeSmall:
		baseValue *= 0.5
	}

	// Adjust based on sample size
	switch profile.SampleSize {
	case stats.SampleSizeLarge:
		baseValue *= 1.5
	case stats.SampleSizeSmall:
		baseValue *= 0.7
	}

	return baseValue
}

// initializeCategoryWeights sets up relative importance of categories
func initializeCategoryWeights() map[stats.RefereeCategory]float64 {
	return map[stats.RefereeCategory]float64{
		stats.CategorySHREDDER:        1.0,
		stats.CategoryDIRECTIONAL:     0.9,
		stats.CategoryINVARIANCE:      0.8,
		stats.CategoryANTI_CONFOUNDER: 0.9,
		stats.CategoryMECHANISM:       0.7,
		stats.CategorySENSITIVITY:     0.6,
		stats.CategoryTOPOLOGICAL:     0.5,
		stats.CategoryTHERMODYNAMIC:   0.4,
		stats.CategoryCOUNTERFACTUAL:  0.8,
		stats.CategorySPECTRAL:        0.5,
	}
}

// initializeTestCosts sets up computational costs for tests
func initializeTestCosts() map[string]float64 {
	return map[string]float64{
		"Permutation_Shredder":           2.0,
		"Bootstrap_Validation":           1.5,
		"Transfer_Entropy":               3.0,
		"Convergent_Cross_Mapping":       4.0,
		"Chow_Stability_Test":            2.5,
		"CUSUM_Drift_Detection":          1.8,
		"Conditional_Mutual_Information": 3.5,
		"Partial_Correlation":            1.2,
		"Monotonicity_Stress_Test":       2.8,
		"Isotonic_Mechanism_Check":       2.2,
		"Leave_One_Out_CV":               5.0,
		"Alpha_Decay_Test":               2.0,
	}
}

// initializeTestReliability sets up historical reliability scores
func initializeTestReliability() map[string]float64 {
	return map[string]float64{
		"Permutation_Shredder":           0.85,
		"Bootstrap_Validation":           0.82,
		"Transfer_Entropy":               0.75,
		"Convergent_Cross_Mapping":       0.78,
		"Chow_Stability_Test":            0.80,
		"CUSUM_Drift_Detection":          0.77,
		"Conditional_Mutual_Information": 0.73,
		"Partial_Correlation":            0.88,
		"Monotonicity_Stress_Test":       0.79,
		"Isotonic_Mechanism_Check":       0.81,
		"Leave_One_Out_CV":               0.84,
		"Alpha_Decay_Test":               0.76,
	}
}

// RiskProfile represents the risk assessment from AI analysis
type RiskProfile struct {
	RiskLevel           stats.HypothesisRiskLevel
	RequiredTestCount   struct {
		Min int
		Max int
	}
	CriticalConcerns    []string
	RecommendedCategories []stats.RefereeCategory
	ConfidenceTarget    float64
	FeasibilityScore    float64
	StatisticalFragility float64
	DataTopology        DataTopologyAssessment
}

// DataTopologyAssessment represents dataset characteristics
type DataTopologyAssessment struct {
	SampleSize         int
	SparsityRatio      float64
	CardinalityCause   int
	CardinalityEffect  int
	SkewnessCause      float64
	SkewnessEffect     float64
	TemporalCoverage   float64
	ConfoundingSignals []string
}

// SelectTestsWithRiskProfile chooses optimal validation tests using AI-generated risk assessment
func (ds *DynamicSelector) SelectTestsWithRiskProfile(
	riskProfile *RiskProfile,
	dataTopology DataTopologyAssessment,
) ([]stats.SelectedTest, stats.SelectionRationale) {

	// Use the AI-generated risk profile instead of building one from scratch
	riskLevel := riskProfile.RiskLevel
	minTests := riskProfile.RequiredTestCount.Min
	maxTests := riskProfile.RequiredTestCount.Max

	// Get required categories based on risk profile recommendations
	requiredCategories := ds.getCategoriesFromRiskProfile(riskProfile)

	// Create enhanced hypothesis profile for compatibility with existing logic
	profile := ds.buildHypothesisProfileFromRisk(riskProfile, dataTopology)

	// Generate candidate tests
	candidates := ds.generateCandidateTests(requiredCategories, profile)

	// Optimize selection with AI-guided bounds
	selectedTests := ds.optimizeSelectionWithRiskGuidance(
		candidates,
		minTests,
		maxTests,
		profile,
		riskProfile,
	)

	// Calculate rationale
	rationale := ds.buildRiskAwareSelectionRationale(
		selectedTests,
		riskLevel,
		requiredCategories,
		minTests,
		maxTests,
		riskProfile,
	)

	return selectedTests, rationale
}

// getCategoriesFromRiskProfile extracts required categories from AI risk assessment
func (ds *DynamicSelector) getCategoriesFromRiskProfile(riskProfile *RiskProfile) []CategoryRequirement {
	requirements := []CategoryRequirement{}

	// Always include SHREDDER for statistical integrity
	requirements = append(requirements, CategoryRequirement{
		Category:  stats.CategorySHREDDER,
		Priority:  stats.PriorityHigh,
		Rationale: "Statistical artifacts must always be ruled out first",
	})

	// Add categories recommended by AI analysis
	for _, category := range riskProfile.RecommendedCategories {
		requirements = append(requirements, CategoryRequirement{
			Category:  category,
			Priority:  stats.PriorityHigh,
			Rationale: "Recommended by AI risk assessment",
		})
	}

	// Add categories based on statistical fragility
	if riskProfile.StatisticalFragility > 0.7 {
		requirements = append(requirements, CategoryRequirement{
			Category:  stats.CategoryINVARIANCE,
			Priority:  stats.PriorityHigh,
			Rationale: "High statistical fragility requires stability testing",
		})
	}

	// Add categories based on confounding signals
	if len(riskProfile.DataTopology.ConfoundingSignals) > 0 {
		requirements = append(requirements, CategoryRequirement{
			Category:  stats.CategoryANTI_CONFOUNDER,
			Priority:  stats.PriorityHigh,
			Rationale: "Confounding signals detected in data topology",
		})
	}

	return requirements
}

// buildHypothesisProfileFromRisk creates a compatible HypothesisProfile from AI risk assessment
func (ds *DynamicSelector) buildHypothesisProfileFromRisk(
	riskProfile *RiskProfile,
	dataTopology DataTopologyAssessment,
) stats.HypothesisProfile {

	// Map sample size
	var sampleSize stats.SampleSizeCategory
	switch {
	case dataTopology.SampleSize < 100:
		sampleSize = stats.SampleSizeSmall
	case dataTopology.SampleSize < 1000:
		sampleSize = stats.SampleSizeMedium
	default:
		sampleSize = stats.SampleSizeLarge
	}

	// Map data complexity based on cardinality and confounding
	var dataComplexity stats.DataComplexityScore
	confoundingCount := len(dataTopology.ConfoundingSignals)
	if confoundingCount > 3 || dataTopology.CardinalityCause > 100 || dataTopology.CardinalityEffect > 100 {
		dataComplexity = stats.DataComplexityComplex
	} else if confoundingCount > 1 || dataTopology.CardinalityCause > 20 || dataTopology.CardinalityEffect > 20 {
		dataComplexity = stats.DataComplexityModerate
	} else {
		dataComplexity = stats.DataComplexitySimple
	}

	// Map temporal nature
	var temporalNature stats.TemporalComplexity
	if dataTopology.TemporalCoverage < 0.5 {
		temporalNature = stats.TemporalStatic
	} else if dataTopology.TemporalCoverage < 0.8 {
		temporalNature = stats.TemporalSimple
	} else {
		temporalNature = stats.TemporalComplex
	}

	// Map confounding risk
	var confoundingRisk stats.ConfoundingAssessment
	switch len(dataTopology.ConfoundingSignals) {
	case 0:
		confoundingRisk = stats.ConfoundingLow
	case 1, 2:
		confoundingRisk = stats.ConfoundingMedium
	default:
		confoundingRisk = stats.ConfoundingHigh
	}

	return stats.HypothesisProfile{
		DataComplexity:  dataComplexity,
		EffectMagnitude: stats.EffectSizeMedium, // Default - could be enhanced
		SampleSize:      sampleSize,
		DomainRisk:      stats.DomainRiskMedium, // Default - could be enhanced
		TemporalNature:  temporalNature,
		ConfoundingRisk: confoundingRisk,
		PriorEvidence:   []stats.ExistingRelationship{}, // Could be enhanced
	}
}

// optimizeSelectionWithRiskGuidance uses AI risk profile to guide test selection
func (ds *DynamicSelector) optimizeSelectionWithRiskGuidance(
	candidates []TestCandidate,
	minTests, maxTests int,
	profile stats.HypothesisProfile,
	riskProfile *RiskProfile,
) []stats.SelectedTest {

	// Start with required tests based on risk profile
	selected := make([]stats.SelectedTest, 0, maxTests)

	// Sort candidates by alignment with risk profile priorities
	sort.Slice(candidates, func(i, j int) bool {
		scoreI := ds.scoreCandidateForRiskProfile(candidates[i], riskProfile)
		scoreJ := ds.scoreCandidateForRiskProfile(candidates[j], riskProfile)
		return scoreI > scoreJ
	})

	// Select tests within the AI-recommended bounds
	testCount := minTests
	if riskProfile.FeasibilityScore < 0.5 {
		// Low feasibility - use upper bound for more validation
		testCount = maxTests
	}

	for _, candidate := range candidates {
		if len(selected) >= testCount {
			break
		}

		selected = append(selected, stats.SelectedTest{
			RefereeName:    candidate.RefereeName,
			Category:       candidate.Category,
			Priority:       stats.PriorityHigh,
			Rationale:      candidate.Rationale,
			ExpectedEValue: candidate.ExpectedEValue,
		})
	}

	return selected
}

// scoreCandidateForRiskProfile evaluates how well a test candidate matches the AI risk profile
func (ds *DynamicSelector) scoreCandidateForRiskProfile(
	candidate TestCandidate,
	riskProfile *RiskProfile,
) float64 {

	score := 0.0

	// Category alignment with AI recommendations
	for _, recommended := range riskProfile.RecommendedCategories {
		if candidate.Category == recommended {
			score += 3.0 // Strong alignment
		}
	}

	// Reliability bonus for high-risk hypotheses
	if riskProfile.RiskLevel >= stats.RiskLevelHigh {
		if reliability, exists := ds.testReliability[candidate.RefereeName]; exists {
			score += reliability * 2.0
		}
	}

	// Feasibility adjustment - prefer simpler tests for low feasibility data
	if riskProfile.FeasibilityScore < 0.5 {
		if cost, exists := ds.testCosts[candidate.RefereeName]; exists && cost < 2.0 {
			score += 1.0 // Prefer low-cost tests for challenging data
		}
	}

	// Base score from existing optimization logic (calculate it)
	baseScore := candidate.ExpectedEValue / candidate.Cost * candidate.Reliability
	score += baseScore

	return score
}

// buildRiskAwareSelectionRationale creates selection rationale incorporating AI insights
func (ds *DynamicSelector) buildRiskAwareSelectionRationale(
	selectedTests []stats.SelectedTest,
	riskLevel stats.HypothesisRiskLevel,
	requiredCategories []CategoryRequirement,
	minTests, maxTests int,
	riskProfile *RiskProfile,
) stats.SelectionRationale {

	categoryCoverage := make(map[stats.RefereeCategory]float64)
	for _, test := range selectedTests {
		categoryCoverage[test.Category] += 1.0
	}

	// Normalize coverage
	for category := range categoryCoverage {
		categoryCoverage[category] /= float64(len(selectedTests))
	}

	// Calculate efficiency score
	totalCost := 0.0
	for _, test := range selectedTests {
		if cost, exists := ds.testCosts[test.RefereeName]; exists {
			totalCost += cost
		}
	}
	efficiencyScore := 1.0 / (1.0 + totalCost/float64(len(selectedTests)))

	// AI insights are incorporated into the SelectionRationale struct

	return stats.SelectionRationale{
		RiskLevel:         riskLevel,
		CategoryCoverage:  categoryCoverage,
		EfficiencyScore:   efficiencyScore,
		ExpectedThreshold: riskProfile.ConfidenceTarget,
		TestCount:         len(selectedTests),
		MinTests:          minTests,
		MaxTests:          maxTests,
	}
}
