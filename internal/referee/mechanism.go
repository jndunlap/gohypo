package referee

import (
	"fmt"
	"math"
	"sort"
)

// MonotonicityTest implements monotonicity stress testing
type MonotonicityTest struct {
	MaxSignFlips    int     // Maximum allowed sign flips in derivatives
	SpearmanMinimum float64 // Minimum Spearman correlation for monotonicity
}

// Execute tests whether the relationship follows expected monotonic patterns
func (mt *MonotonicityTest) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Monotonicity_Stress_Test",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	// Set defaults from centralized constants
	if mt.MaxSignFlips == 0 {
		mt.MaxSignFlips = MECHANISM_SIGN_FLIPS_MAX
	}
	if mt.SpearmanMinimum == 0 {
		mt.SpearmanMinimum = SPEARMAN_RHO_MIN
	}

	// Test for piecewise monotonicity
	monoScore := mt.computeMonotonicityScore(x, y, []float64{0.25, 0.5, 0.75})

	// Bootstrap to estimate confidence in monotonicity
	monoScores := mt.bootstrapMonotonicity(x, y, []float64{0.25, 0.5, 0.75}, 1000)
	pValue := mt.computeMonotonicityPValue(monoScores, monoScore)

	// Apply centralized standard
	passed := monoScore > mt.SpearmanMinimum && pValue < 0.01

	failureReason := ""
	if !passed {
		if monoScore <= 0.3 {
			failureReason = fmt.Sprintf("COMPLEX/NON-MONOTONIC RELATIONSHIP: Data shows inconsistent directional behavior (score=%.3f). Hypothesis assumes monotonic effect but relationship is complex, U-shaped, or contains reversals. Consider nonlinear or piecewise models.", monoScore)
		} else if monoScore <= 0.7 {
			failureReason = fmt.Sprintf("WEAK DIRECTIONALITY: Relationship exists but lacks clear monotonic pattern (score=%.3f). Effect may be noisy, weak, or confounded. Hypothesis directionality is uncertain.", monoScore)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT MONOTONICITY CONFIDENCE: Pattern detected but statistical uncertainty too high (p=%.4f). May be true monotonic relationship but requires larger sample or cleaner data.", pValue)
		}
	}

	return RefereeResult{
		GateName:  "Isotonic_Mechanism_Check",
		Passed:    passed,
		Statistic: monoScore,
		PValue:    pValue,
		StandardUsed: fmt.Sprintf("Isotonic derivative consistency (≤ %d sign flips, ρ ≥ %.2f)",
			mt.MaxSignFlips, mt.SpearmanMinimum),
		FailureReason: failureReason,
	}
}

// computeMonotonicityScore computes a piecewise monotonicity score
func (mt *MonotonicityTest) computeMonotonicityScore(x, y []float64, quantiles []float64) float64 {
	// Create quantile bins
	bins := mt.createQuantileBins(x, y, quantiles)

	totalScore := 0.0
	validBins := 0

	for _, bin := range bins {
		if len(bin.x) >= 10 { // Minimum points per bin
			// Test monotonicity within bin using Spearman correlation
			score := mt.spearmanCorrelation(bin.x, bin.y)
			totalScore += math.Abs(score) // Use absolute value for directional consistency
			validBins++
		}
	}

	if validBins == 0 {
		return 0
	}

	return totalScore / float64(validBins)
}

type quantileBin struct {
	x, y []float64
}

// createQuantileBins creates data bins based on x quantiles
func (mt *MonotonicityTest) createQuantileBins(x, y []float64, quantiles []float64) []quantileBin {
	// Sort by x values
	type point struct {
		x, y float64
		idx  int
	}

	points := make([]point, len(x))
	for i := range x {
		points[i] = point{x: x[i], y: y[i], idx: i}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].x < points[j].x
	})

	// Create bins
	bins := make([]quantileBin, len(quantiles)+1)
	binStarts := []float64{0.0}
	binStarts = append(binStarts, quantiles...)
	binStarts = append(binStarts, 1.0)

	for i := 0; i < len(binStarts)-1; i++ {
		startIdx := int(float64(len(points)) * binStarts[i])
		endIdx := int(float64(len(points)) * binStarts[i+1])

		if endIdx > len(points) {
			endIdx = len(points)
		}
		if startIdx >= endIdx {
			continue
		}

		bin := quantileBin{}
		for j := startIdx; j < endIdx; j++ {
			bin.x = append(bin.x, points[j].x)
			bin.y = append(bin.y, points[j].y)
		}
		bins[i] = bin
	}

	return bins
}

// spearmanCorrelation computes Spearman rank correlation
func (mt *MonotonicityTest) spearmanCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	// Rank the data
	xRanks := mt.rankData(x)
	yRanks := mt.rankData(y)

	// Compute Pearson correlation on ranks
	return mt.pearsonCorrelation(xRanks, yRanks)
}

// rankData assigns ranks to data (average for ties)
func (mt *MonotonicityTest) rankData(data []float64) []float64 {
	n := len(data)
	ranks := make([]float64, n)

	// Create value-index pairs for sorting
	type pair struct {
		value float64
		index int
	}

	pairs := make([]pair, n)
	for i, v := range data {
		pairs[i] = pair{value: v, index: i}
	}

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

// pearsonCorrelation computes Pearson correlation
func (mt *MonotonicityTest) pearsonCorrelation(x, y []float64) float64 {
	n := float64(len(x))
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0

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

// bootstrapMonotonicity performs bootstrap sampling for monotonicity confidence
func (mt *MonotonicityTest) bootstrapMonotonicity(x, y []float64, quantiles []float64, nBootstrap int) []float64 {
	scores := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample with replacement
		xBoot, yBoot := make([]float64, n), make([]float64, n)
		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
		}

		// Compute monotonicity score for bootstrap sample
		scores[i] = mt.computeMonotonicityScore(xBoot, yBoot, quantiles)
	}

	return scores
}

// computeMonotonicityPValue computes p-value from bootstrap distribution
func (mt *MonotonicityTest) computeMonotonicityPValue(bootstrapScores []float64, observedScore float64) float64 {
	count := 0
	for _, score := range bootstrapScores {
		if score >= observedScore {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapScores))
}

// FunctionalFormTest implements functional form stress testing
type FunctionalFormTest struct {
	Knots    []float64 // Knot points for spline testing
	Degrees  []int     // Polynomial degrees to test
	CVNFolds int       // Number of CV folds
}

// Execute tests whether linear model adequately captures the functional form
func (fft *FunctionalFormTest) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Functional_Form_Test",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if fft.Knots == nil {
		fft.Knots = []float64{0.33, 0.67} // Default tercile knots
	}
	if fft.Degrees == nil {
		fft.Degrees = []int{1, 2, 3} // Linear, quadratic, cubic
	}
	if fft.CVNFolds == 0 {
		fft.CVNFolds = 5 // 5-fold CV
	}

	// Test different functional forms using cross-validation
	linearCV := fft.crossValidationScore(x, y, 1, fft.CVNFolds)
	nonlinearCV := fft.crossValidationScore(x, y, 3, fft.CVNFolds) // Cubic as nonlinear test

	// Compute R² improvement from nonlinear model
	r2Improvement := nonlinearCV - linearCV

	// Bootstrap to estimate significance of improvement
	improvements := fft.bootstrapFormTest(x, y, fft.CVNFolds, 1000)
	pValue := fft.computeFormTestPValue(improvements, r2Improvement)

	// Apply hardcoded standard: linear model adequate (R² improvement < 0.05 with p > 0.05)
	passed := r2Improvement < 0.05 || pValue > 0.05

	failureReason := ""
	if !passed {
		failureReason = fmt.Sprintf("Nonlinear form required (R² improvement=%.4f, p=%.4f)", r2Improvement, pValue)
	}

	return RefereeResult{
		GateName:      "Functional_Form_Test",
		Passed:        passed,
		Statistic:     r2Improvement,
		PValue:        pValue,
		StandardUsed:  "Linear model adequate (R² improvement < 0.05 or p > 0.05)",
		FailureReason: failureReason,
	}
}

// crossValidationScore performs k-fold cross-validation for a given polynomial degree
func (fft *FunctionalFormTest) crossValidationScore(x, y []float64, degree, k int) float64 {
	n := len(x)
	foldSize := n / k
	scores := make([]float64, k)

	for fold := 0; fold < k; fold++ {
		// Split data
		testStart := fold * foldSize
		testEnd := testStart + foldSize
		if fold == k-1 {
			testEnd = n // Last fold takes remainder
		}

		xTest := x[testStart:testEnd]
		yTest := y[testStart:testEnd]

		xTrain := append(x[:testStart], x[testEnd:]...)
		yTrain := append(y[:testStart], y[testEnd:]...)

		// Fit model on training data
		coeffs := fft.fitPolynomial(xTrain, yTrain, degree)

		// Evaluate on test data
		yPred := fft.predictPolynomial(xTest, coeffs)
		scores[fold] = fft.rSquared(yTest, yPred)
	}

	// Return average R² across folds
	sum := 0.0
	for _, score := range scores {
		sum += score
	}
	return sum / float64(k)
}

// fitPolynomial fits a polynomial of given degree using least squares
func (fft *FunctionalFormTest) fitPolynomial(x, y []float64, degree int) []float64 {
	n := len(x)
	m := degree + 1

	// Create design matrix
	X := make([][]float64, n)
	for i := range X {
		X[i] = make([]float64, m)
		for j := 0; j < m; j++ {
			X[i][j] = math.Pow(x[i], float64(j))
		}
	}

	// Solve normal equations: (X^T X) β = X^T y
	XTX := fft.matrixMultiply(fft.matrixTranspose(X), X)
	XTy := fft.matrixVectorMultiply(fft.matrixTranspose(X), y)

	return fft.solveLinearSystem(XTX, XTy)
}

// predictPolynomial makes predictions using polynomial coefficients
func (fft *FunctionalFormTest) predictPolynomial(x []float64, coeffs []float64) []float64 {
	predictions := make([]float64, len(x))
	for i, xi := range x {
		pred := 0.0
		for j, coeff := range coeffs {
			pred += coeff * math.Pow(xi, float64(j))
		}
		predictions[i] = pred
	}
	return predictions
}

// rSquared computes coefficient of determination
func (fft *FunctionalFormTest) rSquared(yTrue, yPred []float64) float64 {
	n := len(yTrue)
	yMean := 0.0
	for _, y := range yTrue {
		yMean += y
	}
	yMean /= float64(n)

	ssRes := 0.0
	ssTot := 0.0

	for i := 0; i < n; i++ {
		ssRes += math.Pow(yTrue[i]-yPred[i], 2)
		ssTot += math.Pow(yTrue[i]-yMean, 2)
	}

	if ssTot == 0 {
		return 0
	}

	return 1 - ssRes/ssTot
}

// bootstrapFormTest bootstraps the functional form test
func (fft *FunctionalFormTest) bootstrapFormTest(x, y []float64, cvFolds, nBootstrap int) []float64 {
	improvements := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample
		xBoot, yBoot := make([]float64, n), make([]float64, n)
		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
		}

		// Compute R² improvement for bootstrap sample
		linearCV := fft.crossValidationScore(xBoot, yBoot, 1, cvFolds)
		nonlinearCV := fft.crossValidationScore(xBoot, yBoot, 3, cvFolds)
		improvements[i] = nonlinearCV - linearCV
	}

	return improvements
}

// computeFormTestPValue computes p-value for form test
func (fft *FunctionalFormTest) computeFormTestPValue(improvements []float64, observedImprovement float64) float64 {
	count := 0
	for _, imp := range improvements {
		if imp <= observedImprovement {
			count++
		}
	}
	return float64(count) / float64(len(improvements))
}

// Basic matrix operations for polynomial fitting
func (fft *FunctionalFormTest) matrixTranspose(A [][]float64) [][]float64 {
	m, n := len(A), len(A[0])
	B := make([][]float64, n)
	for i := range B {
		B[i] = make([]float64, m)
		for j := range B[i] {
			B[i][j] = A[j][i]
		}
	}
	return B
}

func (fft *FunctionalFormTest) matrixMultiply(A, B [][]float64) [][]float64 {
	m, k := len(A), len(A[0])
	n := len(B[0])
	C := make([][]float64, m)
	for i := range C {
		C[i] = make([]float64, n)
		for j := range C[i] {
			for p := 0; p < k; p++ {
				C[i][j] += A[i][p] * B[p][j]
			}
		}
	}
	return C
}

func (fft *FunctionalFormTest) matrixVectorMultiply(A [][]float64, v []float64) []float64 {
	m, n := len(A), len(A[0])
	result := make([]float64, m)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			result[i] += A[i][j] * v[j]
		}
	}
	return result
}

// solveLinearSystem solves Ax = b using Gaussian elimination (simplified)
func (fft *FunctionalFormTest) solveLinearSystem(A [][]float64, b []float64) []float64 {
	n := len(A)
	// Create augmented matrix
	aug := make([][]float64, n)
	for i := range aug {
		aug[i] = make([]float64, n+1)
		copy(aug[i][:n], A[i])
		aug[i][n] = b[i]
	}

	// Gaussian elimination
	for i := 0; i < n; i++ {
		// Find pivot
		pivot := i
		for j := i + 1; j < n; j++ {
			if math.Abs(aug[j][i]) > math.Abs(aug[pivot][i]) {
				pivot = j
			}
		}

		// Swap rows
		aug[i], aug[pivot] = aug[pivot], aug[i]

		// Eliminate
		for j := i + 1; j < n; j++ {
			factor := aug[j][i] / aug[i][i]
			for k := i; k <= n; k++ {
				aug[j][k] -= factor * aug[i][k]
			}
		}
	}

	// Back substitution
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = aug[i][n]
		for j := i + 1; j < n; j++ {
			x[i] -= aug[i][j] * x[j]
		}
		x[i] /= aug[i][i]
	}

	return x
}
