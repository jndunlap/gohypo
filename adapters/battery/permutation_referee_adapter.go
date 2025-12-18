package battery

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"

	"gohypo/adapters/stats/senses"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/verdict"
	"gohypo/ports"
)

// PermutationReferee implements statistical validation through permutation testing
type PermutationReferee struct {
	senseEngine *senses.SenseEngine
	rngPort     ports.RNGPort
	numShuffles int // Number of permutations to perform (default: 1000)
}

// NewPermutationReferee creates a new permutation referee with default settings
func NewPermutationReferee(senseEngine *senses.SenseEngine, rngPort ports.RNGPort) *PermutationReferee {
	return &PermutationReferee{
		senseEngine: senseEngine,
		rngPort:     rngPort,
		numShuffles: 1000, // Default for MVP
	}
}

// SetNumShuffles configures the number of permutation shuffles (1000-100000 recommended)
func (pr *PermutationReferee) SetNumShuffles(num int) {
	if num < 1000 {
		num = 1000 // Minimum for statistical reliability
	}
	if num > 100000 {
		num = 100000 // Maximum to prevent excessive computation
	}
	pr.numShuffles = num
}

// ValidateHypothesis performs permutation testing to validate or reject a hypothesis
func (pr *PermutationReferee) ValidateHypothesis(ctx context.Context, hypothesisID core.HypothesisID, matrixBundle *dataset.MatrixBundle) (*ports.ValidationResult, error) {
	// Extract relationship from hypothesis
	// For MVP, we'll find the most significant relationship involving the hypothesis variables
	relationship, err := pr.findPrimaryRelationship(matrixBundle)
	if err != nil {
		return &ports.ValidationResult{
			HypothesisID: hypothesisID,
			Status:       verdict.StatusRejected,
			Reason:       verdict.ReasonNoData,
			PValue:       1.0,
			Confidence:   0.0,
		}, fmt.Errorf("failed to find relationship to validate: %w", err)
	}

	// Extract data for permutation testing
	xData, ok := matrixBundle.GetColumnData(relationship.VariableX)
	if !ok {
		return &ports.ValidationResult{
			HypothesisID: hypothesisID,
			Status:       verdict.StatusRejected,
			Reason:       verdict.ReasonInvalidData,
		}, fmt.Errorf("cannot retrieve data for variable %s", relationship.VariableX)
	}

	yData, ok := matrixBundle.GetColumnData(relationship.VariableY)
	if !ok {
		return &ports.ValidationResult{
			HypothesisID: hypothesisID,
			Status:       verdict.StatusRejected,
			Reason:       verdict.ReasonInvalidData,
		}, fmt.Errorf("cannot retrieve data for variable %s", relationship.VariableY)
	}

	// Perform permutation testing
	permutationP, nullDistribution := pr.performPermutationTest(ctx, xData, yData, relationship.TestUsed)

	// Calculate confidence and effect size preservation
	observedEffect := math.Abs(relationship.EffectSize)
	nullPercentile := pr.calculateNullPercentile(observedEffect, nullDistribution)

	confidence := 1.0 - permutationP
	if confidence < 0 {
		confidence = 0
	}

	// Determine validation status
	status := verdict.StatusValidated
	reason := verdict.ReasonStatisticallySignificant

	if permutationP >= 0.05 {
		status = verdict.StatusRejected
		if permutationP > 0.10 {
			reason = verdict.ReasonLikelyRandom
		} else {
			reason = verdict.ReasonMarginallySignificant
		}
	}

	// Create falsification log for rejected hypotheses
	var falsificationLog *verdict.FalsificationLog
	if status == verdict.StatusRejected {
		falsificationLog = &verdict.FalsificationLog{
			Reason:             reason,
			PermutationPValue:  permutationP,
			ObservedEffectSize: observedEffect,
			NullDistribution: verdict.NullDistributionSummary{
				Mean:         pr.mean(nullDistribution),
				StdDev:       pr.stdDev(nullDistribution),
				Min:          pr.min(nullDistribution),
				Max:          pr.max(nullDistribution),
				Percentile95: pr.percentile(nullDistribution, 95),
				Percentile99: pr.percentile(nullDistribution, 99),
			},
			SampleSize: len(xData),
			TestUsed:   relationship.TestUsed,
			VariableX:  relationship.VariableX,
			VariableY:  relationship.VariableY,
			RejectedAt: core.Now(),
		}
	}

	return &ports.ValidationResult{
		HypothesisID:     hypothesisID,
		Status:           status,
		Reason:           reason,
		PValue:           permutationP,
		Confidence:       confidence,
		EffectSize:       observedEffect,
		NullPercentile:   nullPercentile,
		FalsificationLog: falsificationLog,
		NumPermutations:  pr.numShuffles,
		ValidationMetadata: map[string]interface{}{
			"test_used":            relationship.TestUsed,
			"variable_x":           string(relationship.VariableX),
			"variable_y":           string(relationship.VariableY),
			"original_effect_size": relationship.EffectSize,
			"original_p_value":     relationship.PValue,
		},
	}, nil
}

// RelationshipArtifact represents internal relationship data for the referee
type RelationshipArtifact struct {
	VariableX  core.VariableKey
	VariableY  core.VariableKey
	TestUsed   string
	EffectSize float64
	PValue     float64
	CohortSize int
}

// findPrimaryRelationship finds the most significant statistical relationship in the dataset
// This is a proxy for hypothesis validation in the MVP
func (pr *PermutationReferee) findPrimaryRelationship(bundle *dataset.MatrixBundle) (*RelationshipArtifact, error) {
	// For MVP, we'll compute relationships on-the-fly from the matrix bundle
	// In production, this would extract from stored relationship artifacts

	var strongestRel *RelationshipArtifact
	maxEffectSize := 0.0

	// Iterate through all variable pairs
	for i := 0; i < bundle.ColumnCount(); i++ {
		for j := i + 1; j < bundle.ColumnCount(); j++ {
			metaX := bundle.ColumnMeta[i]
			metaY := bundle.ColumnMeta[j]

			xData, ok := bundle.GetColumnData(metaX.VariableKey)
			if !ok {
				continue
			}
			yData, ok := bundle.GetColumnData(metaY.VariableKey)
			if !ok {
				continue
			}

			// Select appropriate test based on types
			testName := pr.selectTest(metaX.StatisticalType, metaY.StatisticalType)

			// Compute effect size
			effectSize := pr.computeEffectSize(xData, yData, testName)
			absEffect := math.Abs(effectSize)

			if absEffect > maxEffectSize {
				maxEffectSize = absEffect
				strongestRel = &RelationshipArtifact{
					VariableX:  metaX.VariableKey,
					VariableY:  metaY.VariableKey,
					TestUsed:   testName,
					EffectSize: effectSize,
					PValue:     0.0, // Will be computed during permutation
					CohortSize: len(xData),
				}
			}
		}
	}

	if strongestRel == nil {
		return nil, fmt.Errorf("no statistical relationships found in dataset")
	}

	return strongestRel, nil
}

// selectTest chooses the appropriate statistical test
func (pr *PermutationReferee) selectTest(typeX, typeY dataset.StatisticalType) string {
	if typeX == dataset.TypeNumeric && typeY == dataset.TypeNumeric {
		return "pearson"
	}

	if (typeX == dataset.TypeCategorical || typeX == dataset.TypeBinary) &&
		(typeY == dataset.TypeCategorical || typeY == dataset.TypeBinary) {
		return "chisquare"
	}

	if typeX == dataset.TypeBinary && typeY == dataset.TypeNumeric {
		return "ttest"
	}

	return "pearson"
}

// performPermutationTest runs the core permutation testing algorithm
func (pr *PermutationReferee) performPermutationTest(ctx context.Context, xData, yData []float64, testName string) (float64, []float64) {
	// Compute observed effect size
	observedEffect := pr.computeEffectSize(xData, yData, testName)

	// Generate null distribution through permutation
	nullDistribution := make([]float64, pr.numShuffles)

	// Use concurrent workers for performance
	numWorkers := 4 // Balance between CPU cores and memory usage
	if pr.numShuffles < 100 {
		numWorkers = 1
	}

	workChan := make(chan int, pr.numShuffles)
	resultChan := make(chan struct {
		index  int
		effect float64
	}, pr.numShuffles)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pr.permutationWorker(ctx, xData, yData, testName, workChan, resultChan)
		}()
	}

	// Send work
	go func() {
		for i := 0; i < pr.numShuffles; i++ {
			workChan <- i
		}
		close(workChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		nullDistribution[result.index] = result.effect
	}

	// Calculate empirical p-value: proportion of null effects >= observed effect
	extremeCount := 0
	for _, nullEffect := range nullDistribution {
		if math.Abs(nullEffect) >= math.Abs(observedEffect) {
			extremeCount++
		}
	}

	permutationP := float64(extremeCount) / float64(pr.numShuffles)

	// Ensure p-value doesn't exceed 1.0 and handle edge cases
	if permutationP > 1.0 {
		permutationP = 1.0
	}

	return permutationP, nullDistribution
}

// permutationWorker performs permutation testing work in a goroutine
func (pr *PermutationReferee) permutationWorker(ctx context.Context, xData, yData []float64, testName string, workChan <-chan int, resultChan chan<- struct {
	index  int
	effect float64
}) {
	// Create seeded RNG for deterministic results
	rng, err := pr.rngPort.SeededStream(ctx, "permutation-test", int64(42)) // Fixed seed for reproducibility
	if err != nil {
		// Fallback to basic random if RNG port fails
		rng = rand.New(rand.NewSource(42))
	}

	for index := range workChan {
		select {
		case <-ctx.Done():
			return
		default:
			// Shuffle the x variable (driver variable)
			shuffledX := make([]float64, len(xData))
			copy(shuffledX, xData)

			// Fisher-Yates shuffle
			for i := len(shuffledX) - 1; i > 0; i-- {
				j := rng.Intn(i + 1)
				shuffledX[i], shuffledX[j] = shuffledX[j], shuffledX[i]
			}

			// Compute effect size with shuffled data
			effect := pr.computeEffectSize(shuffledX, yData, testName)

			resultChan <- struct {
				index  int
				effect float64
			}{index, effect}
		}
	}
}

// computeEffectSize calculates the effect size for a given statistical test
func (pr *PermutationReferee) computeEffectSize(x, y []float64, testName string) float64 {
	switch testName {
	case "pearson":
		return pr.pearsonCorrelation(x, y)
	case "spearman":
		return pr.spearmanCorrelation(x, y)
	case "mutual_information":
		// Simplified MI calculation for permutation testing
		return pr.mutualInformation(x, y)
	default:
		// Default to Pearson for unknown tests
		return pr.pearsonCorrelation(x, y)
	}
}

// pearsonCorrelation computes Pearson correlation coefficient
func (pr *PermutationReferee) pearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))
	sumX, sumY := 0.0, 0.0
	sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

// spearmanCorrelation computes Spearman rank correlation
func (pr *PermutationReferee) spearmanCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	// Rank the data
	xRanks := pr.rankData(x)
	yRanks := pr.rankData(y)

	// Compute Pearson correlation on ranks
	return pr.pearsonCorrelation(xRanks, yRanks)
}

// rankData assigns ranks to data, handling ties by averaging
func (pr *PermutationReferee) rankData(data []float64) []float64 {
	n := len(data)
	ranks := make([]float64, n)

	// Create index-value pairs for sorting
	type pair struct {
		value float64
		index int
	}

	pairs := make([]pair, n)
	for i, v := range data {
		pairs[i] = pair{v, i}
	}

	// Sort by value
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value < pairs[j].value
	})

	// Assign ranks, handling ties
	i := 0
	for i < n {
		j := i
		// Find group of equal values
		for j < n-1 && pairs[j+1].value == pairs[i].value {
			j++
		}

		// Assign average rank to tied values
		avgRank := float64(i+j)/2.0 + 1
		for k := i; k <= j; k++ {
			ranks[pairs[k].index] = avgRank
		}

		i = j + 1
	}

	return ranks
}

// mutualInformation computes a simplified mutual information estimate
func (pr *PermutationReferee) mutualInformation(x, y []float64) float64 {
	// Simplified MI using binning approach for permutation testing
	// In production, would use proper entropy estimation
	const bins = 10

	if len(x) == 0 {
		return 0
	}

	// Create histograms
	xBins := pr.binData(x, bins)
	yBins := pr.binData(y, bins)

	// Compute joint histogram
	joint := make([][]int, bins)
	for i := range joint {
		joint[i] = make([]int, bins)
	}

	for i := 0; i < len(x); i++ {
		xBin := xBins[i]
		yBin := yBins[i]
		if xBin >= 0 && xBin < bins && yBin >= 0 && yBin < bins {
			joint[xBin][yBin]++
		}
	}

	// Compute mutual information
	mi := 0.0
	n := float64(len(x))

	for i := 0; i < bins; i++ {
		for j := 0; j < bins; j++ {
			if joint[i][j] > 0 {
				pXY := float64(joint[i][j]) / n

				// Marginal probabilities
				pX := 0.0
				pY := 0.0
				for k := 0; k < bins; k++ {
					pX += float64(joint[i][k]) / n
					pY += float64(joint[k][j]) / n
				}

				if pX > 0 && pY > 0 {
					mi += pXY * math.Log2(pXY/(pX*pY))
				}
			}
		}
	}

	return mi
}

// binData assigns data points to histogram bins
func (pr *PermutationReferee) binData(data []float64, bins int) []int {
	if len(data) == 0 {
		return []int{}
	}

	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	binIndices := make([]int, len(data))
	binWidth := (max - min) / float64(bins)

	for i, v := range data {
		if binWidth == 0 {
			binIndices[i] = 0
		} else {
			bin := int((v - min) / binWidth)
			if bin >= bins {
				bin = bins - 1
			}
			binIndices[i] = bin
		}
	}

	return binIndices
}

// calculateNullPercentile computes what percentile the observed effect is in the null distribution
func (pr *PermutationReferee) calculateNullPercentile(observedEffect float64, nullDistribution []float64) float64 {
	if len(nullDistribution) == 0 {
		return 0
	}

	absObserved := math.Abs(observedEffect)
	count := 0
	for _, nullEffect := range nullDistribution {
		if math.Abs(nullEffect) <= absObserved {
			count++
		}
	}

	return float64(count) / float64(len(nullDistribution))
}

// Statistical helper functions
func (pr *PermutationReferee) mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func (pr *PermutationReferee) stdDev(data []float64) float64 {
	if len(data) <= 1 {
		return 0
	}
	m := pr.mean(data)
	sumSq := 0.0
	for _, v := range data {
		diff := v - m
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(data)-1))
}

func (pr *PermutationReferee) min(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	min := data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
	}
	return min
}

func (pr *PermutationReferee) max(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	return max
}

func (pr *PermutationReferee) percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
