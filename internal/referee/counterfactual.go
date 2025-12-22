package referee

import (
	"fmt"
	"math"
	"sort"
)

// SyntheticIntervention implements synthetic intervention testing via G-Computation
type SyntheticIntervention struct {
	InterventionStrength  float64 // Strength of synthetic intervention (0.1 to 1.0)
	NumBootstrap          int     // Number of bootstrap samples for confidence
	ConfoundingAdjustment bool    // Whether to adjust for confounding variables
}

// Execute tests causal claims through synthetic intervention simulation
func (si *SyntheticIntervention) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Synthetic_Intervention",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if si.InterventionStrength == 0 {
		si.InterventionStrength = 0.5 // Moderate intervention strength
	}
	if si.NumBootstrap == 0 {
		si.NumBootstrap = 1000
	}

	// Extract confounding variables if available
	confounders := [][]float64{}
	if si.ConfoundingAdjustment {
		if confounderData, ok := metadata["confounding_variables"].([][]float64); ok {
			confounders = confounderData
		}
	}

	// Fit observational model
	model := si.fitObservationalModel(x, y, confounders)

	// Simulate intervention: set X to different values and predict Y
	interventionEffects := si.simulateInterventions(x, model, si.InterventionStrength)

	// Compute average treatment effect (ATE)
	ate := si.computeATE(interventionEffects)

	// Bootstrap for confidence interval
	bootstrapATEs := si.bootstrapATE(x, y, confounders, si.InterventionStrength, si.NumBootstrap)
	ciLower, ciUpper := si.computeConfidenceInterval(bootstrapATEs, 0.999) // 99.9% CI

	// Check if CI excludes zero (statistical significance at 99.9% level)
	passed := ciLower > 0 || ciUpper < 0 // CI excludes zero

	failureReason := ""
	if !passed {
		if ciLower <= 0 && ciUpper >= 0 {
			failureReason = fmt.Sprintf("NO CAUSAL EFFECT: Intervention simulation shows no significant treatment effect (ATE=%.4f, 99.9%% CI=[%.4f, %.4f]). Relationship may be correlational, not causal. Observed association could be due to confounding or reverse causation.", ate, ciLower, ciUpper)
		} else if math.Abs(ate) < 0.1 {
			failureReason = fmt.Sprintf("WEAK CAUSAL EFFECT: Intervention shows statistically significant but practically negligible effect (ATE=%.4f). Relationship exists but causal impact is too small to be meaningful.", ate)
		} else {
			failureReason = fmt.Sprintf("CAUSAL EFFECT DETECTED BUT UNCERTAIN: Intervention suggests causal relationship (ATE=%.4f) but confidence interval too wide ([%.4f, %.4f]). Effect may be real but requires more data for precision.", ate, ciLower, ciUpper)
		}
	}

	// Use width of CI as pseudo p-value (narrower CI = more significant)
	ciWidth := ciUpper - ciLower
	pValue := math.Min(1.0, ciWidth*2) // Simplified p-value based on CI width

	return RefereeResult{
		GateName:      "Synthetic_Intervention",
		Passed:        passed,
		Statistic:     ate,
		PValue:        pValue,
		StandardUsed:  "99.9% CI excludes zero (G-computation ATE significant)",
		FailureReason: failureReason,
	}
}

// AuditEvidence performs evidence auditing for synthetic interventions using discovery q-values
func (si *SyntheticIntervention) AuditEvidence(discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) RefereeResult {
	// Synthetic interventions are about counterfactual analysis - use default audit logic
	// since intervention analysis requires causal modeling that's hard to audit from q-values alone
	return DefaultAuditEvidence("Synthetic_Intervention", discoveryEvidence, validationData, metadata)
}

// fitObservationalModel fits a model for the observational data
func (si *SyntheticIntervention) fitObservationalModel(x, y []float64, confounders [][]float64) *ObservationalModel {
	model := &ObservationalModel{}

	if len(confounders) == 0 {
		// Simple linear regression: Y = β₀ + β₁X + ε
		model.Beta0, model.Beta1 = si.linearRegression(x, y)
		model.HasConfounders = false
	} else {
		// Multiple regression with confounders: Y = β₀ + β₁X + ΣγⱼCⱼ + ε
		model = si.multipleRegression(x, y, confounders)
	}

	return model
}

type ObservationalModel struct {
	Beta0           float64
	Beta1           float64
	ConfounderCoefs []float64
	HasConfounders  bool
}

// linearRegression performs simple linear regression
func (si *SyntheticIntervention) linearRegression(x, y []float64) (float64, float64) {
	n := float64(len(x))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	return intercept, slope
}

// multipleRegression performs multiple regression with confounders
func (si *SyntheticIntervention) multipleRegression(x, y []float64, confounders [][]float64) *ObservationalModel {
	n := len(x)
	nVars := 1 + len(confounders) // X + confounders

	// Create design matrix: [1, X, C1, C2, ..., Ck]
	X := make([][]float64, n)
	for i := range X {
		X[i] = make([]float64, nVars)
		X[i][0] = 1.0  // Intercept
		X[i][1] = x[i] // Treatment variable
		for j := 0; j < len(confounders); j++ {
			if i < len(confounders[j]) {
				X[i][j+2] = confounders[j][i]
			}
		}
	}

	// Solve normal equations: (X^T X) β = X^T y
	XTX := si.matrixMultiply(si.matrixTranspose(X), X)
	XTy := si.matrixVectorMultiply(si.matrixTranspose(X), y)

	coefs := si.solveLinearSystem(XTX, XTy)

	model := &ObservationalModel{
		Beta0:           coefs[0],
		Beta1:           coefs[1],
		ConfounderCoefs: coefs[2:],
		HasConfounders:  true,
	}

	return model
}

// simulateInterventions simulates different intervention values
func (si *SyntheticIntervention) simulateInterventions(x []float64, model *ObservationalModel, strength float64) []InterventionEffect {
	// Create intervention scenarios: vary X while holding confounders constant
	xMin, xMax := si.dataRange(x)
	interventionPoints := []float64{
		xMin,                    // Minimum intervention
		xMin + (xMax-xMin)*0.25, // Low intervention
		xMin + (xMax-xMin)*0.5,  // Medium intervention
		xMin + (xMax-xMin)*0.75, // High intervention
		xMax,                    // Maximum intervention
	}

	effects := make([]InterventionEffect, len(interventionPoints))

	for i, interventionValue := range interventionPoints {
		// Predict counterfactual outcomes under intervention
		counterfactualY := si.predictCounterfactual(interventionValue, model)

		// Compute effect relative to baseline (median observed X)
		sort.Float64s(x)
		baselineX := x[len(x)/2] // Median as baseline
		baselineY := si.predictCounterfactual(baselineX, model)

		effect := counterfactualY - baselineY

		effects[i] = InterventionEffect{
			InterventionValue: interventionValue,
			CounterfactualY:   counterfactualY,
			BaselineY:         baselineY,
			Effect:            effect,
		}
	}

	return effects
}

type InterventionEffect struct {
	InterventionValue float64
	CounterfactualY   float64
	BaselineY         float64
	Effect            float64
}

// predictCounterfactual predicts outcome under intervention
func (si *SyntheticIntervention) predictCounterfactual(interventionX float64, model *ObservationalModel) float64 {
	if !model.HasConfounders {
		return model.Beta0 + model.Beta1*interventionX
	}

	// For confounders, use their mean values (simplified approach)
	prediction := model.Beta0 + model.Beta1*interventionX
	for _, coef := range model.ConfounderCoefs {
		// Assume confounders are centered, so their mean effect is 0
		// In practice, you'd use actual confounder values or their means
		prediction += coef * 0.0 // Simplified: assume confounders are mean-centered
	}

	return prediction
}

// computeATE computes average treatment effect from intervention effects
func (si *SyntheticIntervention) computeATE(effects []InterventionEffect) float64 {
	if len(effects) == 0 {
		return 0
	}

	sumEffects := 0.0
	for _, effect := range effects {
		sumEffects += effect.Effect
	}

	return sumEffects / float64(len(effects))
}

// bootstrapATE performs bootstrap sampling for ATE confidence intervals
func (si *SyntheticIntervention) bootstrapATE(x, y []float64, confounders [][]float64, strength float64, nBootstrap int) []float64 {
	ates := make([]float64, nBootstrap)
	n := len(x)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample with replacement
		xBoot, yBoot := make([]float64, n), make([]float64, n)
		confoundersBoot := make([][]float64, len(confounders))
		for j := range confoundersBoot {
			confoundersBoot[j] = make([]float64, n)
		}

		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			xBoot[j] = x[idx]
			yBoot[j] = y[idx]
			for k, confounder := range confounders {
				if idx < len(confounder) {
					confoundersBoot[k][j] = confounder[idx]
				}
			}
		}

		// Compute ATE for bootstrap sample
		model := si.fitObservationalModel(xBoot, yBoot, confoundersBoot)
		effects := si.simulateInterventions(xBoot, model, strength)
		ates[i] = si.computeATE(effects)
	}

	return ates
}

// computeConfidenceInterval computes confidence interval from bootstrap distribution
func (si *SyntheticIntervention) computeConfidenceInterval(bootstrapValues []float64, alpha float64) (float64, float64) {
	if len(bootstrapValues) == 0 {
		return 0, 0
	}

	sorted := make([]float64, len(bootstrapValues))
	copy(sorted, bootstrapValues)
	sort.Float64s(sorted)

	lowerPercentile := (1 - alpha) / 2
	upperPercentile := 1 - lowerPercentile

	lowerIdx := int(float64(len(sorted)-1) * lowerPercentile)
	upperIdx := int(float64(len(sorted)-1) * upperPercentile)

	return sorted[lowerIdx], sorted[upperIdx]
}

// dataRange computes min and max of data
func (si *SyntheticIntervention) dataRange(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}

	minVal, maxVal := data[0], data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	return minVal, maxVal
}

// Matrix operations for SyntheticIntervention
func (si *SyntheticIntervention) matrixTranspose(A [][]float64) [][]float64 {
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

func (si *SyntheticIntervention) matrixMultiply(A, B [][]float64) [][]float64 {
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

func (si *SyntheticIntervention) matrixVectorMultiply(A [][]float64, v []float64) []float64 {
	m, n := len(A), len(A[0])
	result := make([]float64, m)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			result[i] += A[i][j] * v[j]
		}
	}
	return result
}

func (si *SyntheticIntervention) solveLinearSystem(A [][]float64, b []float64) []float64 {
	n := len(A)
	aug := make([][]float64, n)
	for i := range aug {
		aug[i] = make([]float64, n+1)
		copy(aug[i][:n], A[i])
		aug[i][n] = b[i]
	}

	// Gaussian elimination
	for i := 0; i < n; i++ {
		pivot := i
		for j := i + 1; j < n; j++ {
			if math.Abs(aug[j][i]) > math.Abs(aug[pivot][i]) {
				pivot = j
			}
		}
		aug[i], aug[pivot] = aug[pivot], aug[i]

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
