package referee

import (
	"fmt"
	"math"
	"math/cmplx"
)

// WaveletCoherence implements wavelet coherence analysis for frequency-domain relationships
type WaveletCoherence struct {
	WaveletType        string    // Type of wavelet ('morlet', 'paul', 'dog')
	Scales             []float64 // Wavelet scales to analyze
	MinFrequency       float64   // Minimum frequency to analyze
	MaxFrequency       float64   // Maximum frequency to analyze
	NumScales          int       // Number of scales to use
	CoherenceThreshold float64   // Threshold for significant coherence
}

// Execute performs wavelet coherence analysis to detect frequency-specific relationships
func (wc *WaveletCoherence) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Wavelet_Coherence",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if wc.WaveletType == "" {
		wc.WaveletType = "morlet"
	}
	if wc.NumScales == 0 {
		wc.NumScales = 32
	}
	if wc.MinFrequency == 0 {
		wc.MinFrequency = 0.01
	}
	if wc.MaxFrequency == 0 {
		wc.MaxFrequency = 0.5
	}
	if wc.CoherenceThreshold == 0 {
		wc.CoherenceThreshold = 0.5
	}

	// Generate scales for analysis
	if wc.Scales == nil {
		wc.Scales = wc.generateScales(wc.NumScales, wc.MinFrequency, wc.MaxFrequency, len(x))
	}

	// Compute wavelet transforms
	Wx := wc.continuousWaveletTransform(x, wc.Scales, wc.WaveletType)
	Wy := wc.continuousWaveletTransform(y, wc.Scales, wc.WaveletType)

	// Compute wavelet coherence
	coherence := wc.computeWaveletCoherence(Wx, Wy)

	// Analyze coherence patterns
	coherenceStats := wc.analyzeCoherencePatterns(coherence, wc.CoherenceThreshold)

	// Bootstrap for significance
	coherenceScores := wc.bootstrapCoherence(x, y, wc.Scales, wc.WaveletType, wc.CoherenceThreshold, 500)
	pValue := wc.computeCoherencePValue(coherenceScores, coherenceStats.MeanCoherence)

	// Apply hardcoded standard: significant coherence structure (score > 0.4) with p < 0.05
	passed := coherenceStats.MeanCoherence > 0.4 && pValue < 0.05

	failureReason := ""
	if !passed {
		if coherenceStats.MeanCoherence <= 0.2 {
			failureReason = fmt.Sprintf("NO FREQUENCY COUPLING: Signals show no coherent relationship across frequencies (coherence=%.3f). Data may be independent or relationship exists only in time domain, not frequency domain.", coherenceStats.MeanCoherence)
		} else if coherenceStats.MeanCoherence <= 0.4 {
			failureReason = fmt.Sprintf("WEAK FREQUENCY COUPLING: Some spectral coherence detected but insufficient strength (coherence=%.3f). Relationship may exist at specific frequencies or be masked by noise.", coherenceStats.MeanCoherence)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT COHERENCE CONFIDENCE: Spectral coupling detected but statistical uncertainty too high (p=%.4f). May have genuine frequency relationships but requires larger sample.", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Wavelet_Coherence",
		Passed:        passed,
		Statistic:     coherenceStats.MeanCoherence,
		PValue:        pValue,
		StandardUsed:  "Mean coherence > 0.4 with p < 0.05 (significant frequency-domain coupling)",
		FailureReason: failureReason,
	}
}

// generateScales generates logarithmically spaced scales for wavelet analysis
func (wc *WaveletCoherence) generateScales(numScales int, minFreq, maxFreq float64, signalLength int) []float64 {
	scales := make([]float64, numScales)

	// Logarithmic spacing from min to max frequency
	minScale := 1.0 / maxFreq
	maxScale := 1.0 / minFreq

	scaleFactor := math.Pow(maxScale/minScale, 1.0/float64(numScales-1))

	for i := 0; i < numScales; i++ {
		scales[i] = minScale * math.Pow(scaleFactor, float64(i))
	}

	return scales
}

// continuousWaveletTransform computes the continuous wavelet transform
func (wc *WaveletCoherence) continuousWaveletTransform(signal []float64, scales []float64, waveletType string) [][]complex128 {
	n := len(signal)
	W := make([][]complex128, len(scales))

	for sIdx, scale := range scales {
		W[sIdx] = make([]complex128, n)

		for t := 0; t < n; t++ {
			sum := 0.0 + 0.0i

			// Convolve signal with scaled wavelet
			for tau := 0; tau < n; tau++ {
				wavelet := wc.waveletFunction(float64(t-tau)/scale, waveletType)
				sum += complex(signal[tau], 0) * wavelet / complex(scale, 0)
			}

			W[sIdx][t] = sum
		}
	}

	return W
}

// waveletFunction computes wavelet basis function
func (wc *WaveletCoherence) waveletFunction(t float64, waveletType string) complex128 {
	switch waveletType {
	case "morlet":
		// Morlet wavelet: exp(-t²/2) * exp(iω₀t)
		// ω₀ = 5 (commonly used)
		omega0 := 5.0
		envelope := math.Exp(-t * t / 2.0)
		oscillation := cmplx.Exp(complex(0, omega0*t))
		return complex(envelope, 0) * oscillation

	case "paul":
		// Paul wavelet: (1 - it)^(-m) * normalization
		m := 4 // Order
		z := complex(1, -t)
		if cmplx.Abs(z) > 0 {
			wavelet := cmplx.Pow(z, complex(-float64(m), 0))
			// Normalization factor
			norm := math.Sqrt(math.Abs(float64(m-1)) / math.Gamma(float64(m)))
			return wavelet * complex(norm, 0)
		}
		return 0

	case "dog":
		// Derivative of Gaussian (DOG) wavelet
		// Second derivative for m=2
		m := 2
		hermiteCoeff := []float64{1, -2, 1} // Coefficients for m=2
		sum := 0.0
		for k := 0; k <= m; k++ {
			if k < len(hermiteCoeff) {
				sum += hermiteCoeff[k] * math.Pow(t, float64(k)) / math.Gamma(float64(k+1))
			}
		}
		envelope := math.Exp(-t * t / 2.0)
		return complex(sum*envelope, 0)

	default:
		// Default to Morlet
		return wc.waveletFunction(t, "morlet")
	}
}

// computeWaveletCoherence computes wavelet coherence between two wavelet transforms
func (wc *WaveletCoherence) computeWaveletCoherence(Wx, Wy [][]complex128) [][]float64 {
	if len(Wx) != len(Wy) || len(Wx) == 0 {
		return [][]float64{}
	}

	numScales, n := len(Wx), len(Wx[0])
	coherence := make([][]float64, numScales)

	for s := 0; s < numScales; s++ {
		coherence[s] = make([]float64, n)

		for t := 0; t < n; t++ {
			// Compute cross-wavelet transform
			crossWavelet := Wx[s][t] * cmplx.Conj(Wy[s][t])

			// Compute auto-wavelet power
			powerX := cmplx.Abs(Wx[s][t]) * cmplx.Abs(Wx[s][t])
			powerY := cmplx.Abs(Wy[s][t]) * cmplx.Abs(Wy[s][t])

			// Coherence = |Wxy|² / (Wx² * Wy²)
			if powerX > 0 && powerY > 0 {
				coherence[s][t] = cmplx.Abs(crossWavelet) * cmplx.Abs(crossWavelet) / (powerX * powerY)
			} else {
				coherence[s][t] = 0
			}
		}
	}

	return coherence
}

// analyzeCoherencePatterns analyzes coherence matrices for structural patterns
func (wc *WaveletCoherence) analyzeCoherencePatterns(coherence [][]float64, threshold float64) CoherenceStats {
	stats := CoherenceStats{}

	totalCoherence := 0.0
	significantPoints := 0
	totalPoints := 0

	maxCoherence := 0.0
	coherenceDistribution := []float64{}

	for s := 0; s < len(coherence); s++ {
		for t := 0; t < len(coherence[s]); t++ {
			coh := coherence[s][t]
			totalCoherence += coh
			totalPoints++

			coherenceDistribution = append(coherenceDistribution, coh)

			if coh > maxCoherence {
				maxCoherence = coh
			}

			if coh > threshold {
				significantPoints++
			}
		}
	}

	if totalPoints > 0 {
		stats.MeanCoherence = totalCoherence / float64(totalPoints)
		stats.MaxCoherence = maxCoherence
		stats.SignificantFraction = float64(significantPoints) / float64(totalPoints)
	}

	// Compute coherence variability
	if len(coherenceDistribution) > 0 {
		stats.CoherenceVariance = wc.variance(coherenceDistribution)
	}

	return stats
}

type CoherenceStats struct {
	MeanCoherence       float64
	MaxCoherence        float64
	SignificantFraction float64
	CoherenceVariance   float64
}

// variance computes variance of a slice
func (wc *WaveletCoherence) variance(data []float64) float64 {
	if len(data) <= 1 {
		return 0
	}

	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	sumSq := 0.0
	for _, v := range data {
		diff := v - mean
		sumSq += diff * diff
	}

	return sumSq / float64(len(data)-1)
}

// bootstrapCoherence performs bootstrap sampling for coherence analysis
func (wc *WaveletCoherence) bootstrapCoherence(x, y []float64, scales []float64, waveletType string, threshold float64, nBootstrap int) []float64 {
	scores := make([]float64, nBootstrap)
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

		// Compute coherence for bootstrap sample
		WxBoot := wc.continuousWaveletTransform(xBoot, scales, waveletType)
		WyBoot := wc.continuousWaveletTransform(yBoot, scales, waveletType)
		coherenceBoot := wc.computeWaveletCoherence(WxBoot, WyBoot)
		statsBoot := wc.analyzeCoherencePatterns(coherenceBoot, threshold)

		scores[i] = statsBoot.MeanCoherence
	}

	return scores
}

// computeCoherencePValue computes p-value from bootstrap distribution
func (wc *WaveletCoherence) computeCoherencePValue(bootstrapScores []float64, observedScore float64) float64 {
	count := 0
	for _, score := range bootstrapScores {
		if score >= observedScore {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapScores))
}

// SpectralAnalysis implements comprehensive spectral analysis
type SpectralAnalysis struct {
	Method          string  // 'periodogram', 'welch', 'multitaper'
	WindowSize      int     // Size of analysis window
	Overlap         float64 // Overlap fraction between windows
	NumTapers       int     // Number of tapers for multitaper
	ConfidenceLevel float64 // Confidence level for significance tests
	PhaseAnalysis   bool    // Whether to perform phase analysis
}

// Execute performs comprehensive spectral analysis of relationships
func (sa *SpectralAnalysis) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Spectral_Analysis",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if sa.Method == "" {
		sa.Method = "periodogram"
	}
	if sa.WindowSize == 0 {
		sa.WindowSize = len(x) / 4 // Quarter of signal length
	}
	if sa.Overlap == 0 {
		sa.Overlap = 0.5 // 50% overlap
	}
	if sa.NumTapers == 0 {
		sa.NumTapers = 5
	}
	if sa.ConfidenceLevel == 0 {
		sa.ConfidenceLevel = 0.95
	}

	// Compute power spectral densities
	px := sa.computePowerSpectralDensity(x, sa.Method, sa.WindowSize, sa.Overlap, sa.NumTapers)
	py := sa.computePowerSpectralDensity(y, sa.Method, sa.WindowSize, sa.Overlap, sa.NumTapers)

	// Compute cross-spectral density
	cxy := sa.computeCrossSpectralDensity(x, y, sa.Method, sa.WindowSize, sa.Overlap, sa.NumTapers)

	// Convert to complex for coherence calculation
	pxComplex := make([]complex128, len(px))
	pyComplex := make([]complex128, len(py))
	for i := range px {
		pxComplex[i] = complex(px[i], 0)
	}
	for i := range py {
		pyComplex[i] = complex(py[i], 0)
	}

	// Compute coherence and phase
	coherence, phase := sa.computeCoherenceAndPhase(pxComplex, pyComplex, cxy)

	// Analyze spectral relationships
	spectralStats := sa.analyzeSpectralRelationships(coherence, phase, sa.ConfidenceLevel)

	// Bootstrap for significance
	spectralScores := sa.bootstrapSpectralAnalysis(x, y, sa.Method, sa.WindowSize, sa.Overlap, sa.NumTapers, sa.ConfidenceLevel, 500)
	pValue := sa.computeSpectralPValue(spectralScores, spectralStats.MeanCoherence)

	// Apply hardcoded standard: significant spectral coupling (score > 0.5) with p < 0.05
	passed := spectralStats.MeanCoherence > 0.5 && pValue < 0.05

	failureReason := ""
	if !passed {
		if spectralStats.MeanCoherence <= 0.3 {
			failureReason = fmt.Sprintf("NO SPECTRAL STRUCTURE: Signals show no meaningful frequency-domain relationship (coherence=%.3f). Data may be white noise or relationship exists only in time domain.", spectralStats.MeanCoherence)
		} else if spectralStats.MeanCoherence <= 0.5 {
			failureReason = fmt.Sprintf("WEAK SPECTRAL COUPLING: Some frequency-domain relationship detected but insufficient strength (coherence=%.3f). May indicate weak periodic patterns or harmonics.", spectralStats.MeanCoherence)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT SPECTRAL CONFIDENCE: Frequency coupling detected but statistical uncertainty too high (p=%.4f). May have genuine spectral patterns but needs larger sample.", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Spectral_Analysis",
		Passed:        passed,
		Statistic:     spectralStats.MeanCoherence,
		PValue:        pValue,
		StandardUsed:  "Mean coherence > 0.5 with p < 0.05 (significant spectral domain coupling)",
		FailureReason: failureReason,
	}
}

// computePowerSpectralDensity computes power spectral density using specified method
func (sa *SpectralAnalysis) computePowerSpectralDensity(signal []float64, method string, windowSize int, overlap float64, numTapers int) []float64 {
	switch method {
	case "periodogram":
		return sa.periodogram(signal)
	case "welch":
		return sa.welchPeriodogram(signal, windowSize, overlap)
	case "multitaper":
		return sa.multitaperPeriodogram(signal, numTapers)
	default:
		return sa.periodogram(signal)
	}
}

// periodogram computes basic periodogram
func (sa *SpectralAnalysis) periodogram(signal []float64) []float64 {
	n := len(signal)
	if n == 0 {
		return []float64{}
	}

	// Compute FFT
	fft := sa.fft(signal)

	// Power spectral density (magnitude squared, normalized)
	psd := make([]float64, n/2)
	for i := 0; i < n/2; i++ {
		psd[i] = cmplx.Abs(fft[i]) * cmplx.Abs(fft[i]) / float64(n)
	}

	return psd
}

// welchPeriodogram computes Welch's method for PSD estimation
func (sa *SpectralAnalysis) welchPeriodogram(signal []float64, windowSize int, overlap float64) []float64 {
	n := len(signal)
	step := int(float64(windowSize) * (1 - overlap))

	psds := [][]float64{}

	for start := 0; start+windowSize <= n; start += step {
		window := signal[start : start+windowSize]

		// Apply Hann window
		windowed := sa.applyHannWindow(window)

		// Compute periodogram for this window
		psd := sa.periodogram(windowed)
		psds = append(psds, psd)
	}

	// Average across windows
	if len(psds) == 0 {
		return []float64{}
	}

	avgPSD := make([]float64, len(psds[0]))
	for i := range avgPSD {
		sum := 0.0
		for j := range psds {
			if i < len(psds[j]) {
				sum += psds[j][i]
			}
		}
		avgPSD[i] = sum / float64(len(psds))
	}

	return avgPSD
}

// multitaperPeriodogram computes multitaper periodogram
func (sa *SpectralAnalysis) multitaperPeriodogram(signal []float64, numTapers int) []float64 {
	n := len(signal)

	// Generate DPSS tapers (simplified: use Hann windows with different phases)
	tapers := sa.generateDPSS(n, numTapers)

	psds := make([][]float64, numTapers)
	for i := 0; i < numTapers; i++ {
		// Apply taper
		tapered := make([]float64, n)
		for j := 0; j < n; j++ {
			tapered[j] = signal[j] * tapers[i][j]
		}

		// Compute eigenspectrum
		psds[i] = sa.periodogram(tapered)
	}

	// Average across tapers
	avgPSD := make([]float64, len(psds[0]))
	for i := range avgPSD {
		sum := 0.0
		for j := range psds {
			if i < len(psds[j]) {
				sum += psds[j][i]
			}
		}
		avgPSD[i] = sum / float64(numTapers)
	}

	return avgPSD
}

// generateDPSS generates discrete prolate spheroidal sequence tapers (simplified)
func (sa *SpectralAnalysis) generateDPSS(n, numTapers int) [][]float64 {
	tapers := make([][]float64, numTapers)

	// Simplified: use shifted Hann windows
	for i := 0; i < numTapers; i++ {
		tapers[i] = make([]float64, n)
		phase := 2 * math.Pi * float64(i) / float64(numTapers)

		for j := 0; j < n; j++ {
			// Hann window with phase shift
			hann := 0.5 * (1 - math.Cos(2*math.Pi*float64(j)/float64(n-1)))
			tapers[i][j] = hann * math.Cos(phase*float64(j)/float64(n))
		}
	}

	return tapers
}

// applyHannWindow applies Hann window to signal
func (sa *SpectralAnalysis) applyHannWindow(signal []float64) []float64 {
	windowed := make([]float64, len(signal))
	for i, s := range signal {
		windowed[i] = s * 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(len(signal)-1)))
	}
	return windowed
}

// fft computes Fast Fourier Transform (simplified implementation)
func (sa *SpectralAnalysis) fft(signal []float64) []complex128 {
	n := len(signal)
	if n == 0 {
		return []complex128{}
	}

	// Cooley-Tukey FFT (simplified recursive implementation)
	if n == 1 {
		return []complex128{complex(signal[0], 0)}
	}

	// Split into even and odd indices
	even := []float64{}
	odd := []float64{}
	for i := 0; i < n; i += 2 {
		even = append(even, signal[i])
		if i+1 < n {
			odd = append(odd, signal[i+1])
		}
	}

	// Recursive FFT
	fftEven := sa.fft(even)
	fftOdd := sa.fft(odd)

	// Combine
	result := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		t := cmplx.Exp(complex(0, -2*math.Pi*float64(k)/float64(n))) * fftOdd[k]
		result[k] = fftEven[k] + t
		result[k+n/2] = fftEven[k] - t
	}

	return result
}

// computeCrossSpectralDensity computes cross-spectral density
func (sa *SpectralAnalysis) computeCrossSpectralDensity(x, y []float64, method string, windowSize int, overlap float64, numTapers int) []complex128 {
	// Compute cross-spectrum using same method as PSD
	n := len(x)

	switch method {
	case "periodogram":
		// Cross-periodogram
		fftX := sa.fft(x)
		fftY := sa.fft(y)
		csd := make([]complex128, n/2)
		for i := 0; i < n/2; i++ {
			csd[i] = fftX[i] * cmplx.Conj(fftY[i]) / complex(float64(n), 0)
		}
		return csd

	case "welch":
		// Cross-Welch
		step := int(float64(windowSize) * (1 - overlap))
		csds := []complex128{}

		for start := 0; start+windowSize <= n; start += step {
			xWindow := sa.applyHannWindow(x[start : start+windowSize])
			yWindow := sa.applyHannWindow(y[start : start+windowSize])

			fftX := sa.fft(xWindow)
			fftY := sa.fft(yWindow)

			csd := make([]complex128, len(fftX)/2)
			for i := 0; i < len(fftX)/2; i++ {
				csd[i] = fftX[i] * cmplx.Conj(fftY[i]) / complex(float64(windowSize), 0)
			}
			csds = append(csds, csd...)
		}

		// Average across windows (simplified)
		if len(csds) == 0 {
			return []complex128{}
		}
		return csds[:len(csds)/2] // Return half for simplicity

	default:
		return sa.computeCrossSpectralDensity(x, y, "periodogram", windowSize, overlap, numTapers)
	}
}

// computeCoherenceAndPhase computes coherence and phase from spectral densities
func (sa *SpectralAnalysis) computeCoherenceAndPhase(px, py, cxy []complex128) ([]float64, []float64) {
	if len(px) != len(py) || len(px) != len(cxy) {
		return []float64{}, []float64{}
	}

	coherence := make([]float64, len(px))
	phase := make([]float64, len(px))

	for i := 0; i < len(px); i++ {
		// Coherence = |Cxy|² / (Px * Py)
		pxReal := real(px[i])
		pyReal := real(py[i])
		cxyMagSq := cmplx.Abs(cxy[i]) * cmplx.Abs(cxy[i])

		if pxReal > 0 && pyReal > 0 {
			coherence[i] = cxyMagSq / (pxReal * pyReal)
		} else {
			coherence[i] = 0
		}

		// Phase = arg(Cxy)
		phase[i] = cmplx.Phase(cxy[i])
	}

	return coherence, phase
}

// analyzeSpectralRelationships analyzes spectral coherence and phase relationships
func (sa *SpectralAnalysis) analyzeSpectralRelationships(coherence, phase []float64, confidenceLevel float64) SpectralStats {
	stats := SpectralStats{}

	if len(coherence) == 0 {
		return stats
	}

	// Basic statistics
	sumCoherence := 0.0
	maxCoherence := 0.0
	significantPoints := 0

	for _, c := range coherence {
		sumCoherence += c
		if c > maxCoherence {
			maxCoherence = c
		}
		if c > (1 - confidenceLevel) { // Significance threshold
			significantPoints++
		}
	}

	stats.MeanCoherence = sumCoherence / float64(len(coherence))
	stats.MaxCoherence = maxCoherence
	stats.SignificantFraction = float64(significantPoints) / float64(len(coherence))

	// Phase analysis if requested
	if sa.PhaseAnalysis && len(phase) > 0 {
		stats.MeanPhase = sa.mean(phase)
		stats.PhaseVariance = sa.variance(phase)
	}

	return stats
}

type SpectralStats struct {
	MeanCoherence       float64
	MaxCoherence        float64
	SignificantFraction float64
	MeanPhase           float64
	PhaseVariance       float64
}

// bootstrapSpectralAnalysis performs bootstrap sampling for spectral analysis
func (sa *SpectralAnalysis) bootstrapSpectralAnalysis(x, y []float64, method string, windowSize int, overlap float64, numTapers int, confidenceLevel float64, nBootstrap int) []float64 {
	scores := make([]float64, nBootstrap)
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

		// Compute spectral analysis for bootstrap sample
		px := sa.computePowerSpectralDensity(xBoot, method, windowSize, overlap, numTapers)
		py := sa.computePowerSpectralDensity(yBoot, method, windowSize, overlap, numTapers)
		cxy := sa.computeCrossSpectralDensity(xBoot, yBoot, method, windowSize, overlap, numTapers)

		// Convert to complex for coherence calculation
		pxComplex := make([]complex128, len(px))
		pyComplex := make([]complex128, len(py))
		for i := range px {
			pxComplex[i] = complex(px[i], 0)
		}
		for i := range py {
			pyComplex[i] = complex(py[i], 0)
		}

		coherence, _ := sa.computeCoherenceAndPhase(pxComplex, pyComplex, cxy)
		stats := sa.analyzeSpectralRelationships(coherence, []float64{}, confidenceLevel)

		scores[i] = stats.MeanCoherence
	}

	return scores
}

// computeSpectralPValue computes p-value from bootstrap distribution
func (sa *SpectralAnalysis) computeSpectralPValue(bootstrapScores []float64, observedScore float64) float64 {
	count := 0
	for _, score := range bootstrapScores {
		if score >= observedScore {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapScores))
}

// mean computes arithmetic mean
func (sa *SpectralAnalysis) mean(data []float64) float64 {
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// variance computes variance
func (sa *SpectralAnalysis) variance(data []float64) float64 {
	if len(data) <= 1 {
		return 0
	}

	m := sa.mean(data)
	sumSq := 0.0
	for _, v := range data {
		diff := v - m
		sumSq += diff * diff
	}

	return sumSq / float64(len(data)-1)
}

// AuditEvidence performs evidence auditing for wavelet coherence using discovery q-values
func (wc *WaveletCoherence) AuditEvidence(discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) RefereeResult {
	// Wavelet coherence is about frequency-domain relationships - use default audit logic
	// since spectral analysis requires time-frequency processing that's hard to audit from q-values alone
	return DefaultAuditEvidence("Wavelet_Coherence", discoveryEvidence, validationData, metadata)
}
