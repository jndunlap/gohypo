package referee

import (
	"context"
	"fmt"
	"gohypo/domain/stats"
	"log"
	"sync"
	"time"

	"gohypo/models"
	"golang.org/x/sync/semaphore"
)

// UIBroadcaster interface for broadcasting UI updates (to avoid import cycles)
type UIBroadcaster interface {
	BroadcastHypothesisValidating(sessionID, hypothesisID string, totalTests, activeTests int, currentEValue, confidence float64) error
	BroadcastHypothesisCompleted(sessionID, hypothesisID string, result *models.HypothesisResult) error
	BroadcastTestStatusUpdate(sessionID, hypothesisID, testName, shortName string, passed, completed bool) error
}

// ValidationEngine manages concurrent test execution with weighted resource allocation
type ValidationEngine struct {
	// Phase-specific semaphores (weighted resource allocation)
	integritySem  *semaphore.Weighted // Phase 0: Integrity (10 concurrent)
	causalitySem  *semaphore.Weighted // Phase 1: Causality (5 concurrent)
	complexitySem *semaphore.Weighted // Phase 2: Complexity (2 concurrent)

	// Job management
	jobQueue      chan ValidationJob
	resultsChan   chan TestResult
	activeJobs    map[string]*ValidationJob
	jobMu         sync.RWMutex

	// UI broadcasting for live updates
	uiBroadcaster UIBroadcaster

	// Context for graceful shutdown
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// ValidationJob represents a test execution job
type ValidationJob struct {
	ID            string
	HypothesisID  string
	TestName      string
	Referee       Referee
	TestConfig    TestConfig
	Phase         int
	ComputeWeight int64
	Timeout       time.Duration
	CreatedAt     time.Time
}

// TestConfig holds test-specific configuration
type TestConfig struct {
	X []float64
	Y []float64
	Metadata map[string]interface{}
}

// TestResult holds the outcome of a test execution
type TestResult struct {
	Job       ValidationJob
	Result    RefereeResult
	Duration  time.Duration
	Error     error
	StartedAt time.Time
	EndedAt   time.Time
}

// NewValidationEngine creates a new validation engine with weighted semaphores
func NewValidationEngine(uiBroadcaster UIBroadcaster) *ValidationEngine {
	ctx, cancel := context.WithCancel(context.Background())

	return &ValidationEngine{
		// Weighted semaphores: higher numbers = more concurrent jobs allowed
		integritySem:  semaphore.NewWeighted(10), // Allow 10 concurrent integrity tests
		causalitySem:  semaphore.NewWeighted(5),  // Allow 5 concurrent causality tests
		complexitySem: semaphore.NewWeighted(2),  // Allow 2 concurrent complexity tests

		// Buffered channels to prevent blocking
		jobQueue:      make(chan ValidationJob, 100),
		resultsChan:   make(chan TestResult, 200),
		activeJobs:    make(map[string]*ValidationJob),

		uiBroadcaster: uiBroadcaster,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start launches the validation engine workers
func (ve *ValidationEngine) Start() {
	log.Printf("[ValidationEngine] Starting with semaphores: Integrity(10), Causality(5), Complexity(2)")

	// Start phase-specific worker pools
	go ve.processIntegrityPhase()
	go ve.processCausalityPhase()
	go ve.processComplexityPhase()

	// Start result aggregator
	go ve.aggregateResults()

	log.Printf("[ValidationEngine] Validation engine started successfully")
}

// Stop gracefully shuts down the validation engine
func (ve *ValidationEngine) Stop() {
	log.Printf("[ValidationEngine] Initiating graceful shutdown...")

	ve.cancel()
	close(ve.jobQueue)

	// Wait for all workers to finish
	done := make(chan struct{})
	go func() {
		ve.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("[ValidationEngine] All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Printf("[ValidationEngine] Timeout waiting for workers to stop")
	}
}

// SubmitJob queues a validation job for execution
func (ve *ValidationEngine) SubmitJob(job ValidationJob) error {
	select {
	case ve.jobQueue <- job:
		ve.jobMu.Lock()
		ve.activeJobs[job.ID] = &job
		ve.jobMu.Unlock()

		log.Printf("[ValidationEngine] Job %s submitted for hypothesis %s (phase %d, weight %d)",
			job.ID, job.HypothesisID, job.Phase, job.ComputeWeight)
		return nil
	default:
		return fmt.Errorf("job queue full - cannot accept job %s", job.ID)
	}
}

// ExecuteAdaptiveValidation runs a full validation battery with phase gating
func (ve *ValidationEngine) ExecuteAdaptiveValidation(
	ctx context.Context,
	hypothesisID string,
	testBattery []stats.SelectedTest,
	sessionID string,
) (*ValidationSummary, error) {

	log.Printf("[ValidationEngine] Starting adaptive validation for hypothesis %s with %d tests",
		hypothesisID, len(testBattery))

	summary := &ValidationSummary{
		HypothesisID: hypothesisID,
		SessionID:    sessionID,
		StartedAt:    time.Now(),
		PhaseResults: make(map[int]*PhaseResult),
	}

	// Execute tests in phases with fail-fast logic
	for phase := 0; phase < 3; phase++ {
		phaseTests := ve.filterTestsByPhase(testBattery, phase)
		if len(phaseTests) == 0 {
			continue
		}

		log.Printf("[ValidationEngine] Executing phase %d with %d tests", phase, len(phaseTests))

		phaseResult, err := ve.executePhase(ctx, hypothesisID, phaseTests, phase, sessionID)
		if err != nil {
			return nil, fmt.Errorf("phase %d execution failed: %w", phase, err)
		}

		summary.PhaseResults[phase] = phaseResult

		// Fail-fast check: if integrity tests fail, stop execution
		if phase == 0 && !ve.passesIntegrityThreshold(phaseResult) {
			log.Printf("[ValidationEngine] Early termination: integrity tests failed for hypothesis %s", hypothesisID)
			summary.EarlyTermination = true
			summary.TerminationReason = "Failed integrity gate"
			break
		}
	}

	summary.CompletedAt = time.Now()
	summary.Duration = summary.CompletedAt.Sub(summary.StartedAt)
	summary.OverallEValue = ve.calculateOverallEValue(summary)

	log.Printf("[ValidationEngine] Validation completed for hypothesis %s in %v (E-value: %.2f)",
		hypothesisID, summary.Duration, summary.OverallEValue)

	return summary, nil
}

// executePhase runs all tests in a specific phase concurrently
func (ve *ValidationEngine) executePhase(
	ctx context.Context,
	hypothesisID string,
	tests []stats.SelectedTest,
	phase int,
	sessionID string,
) (*PhaseResult, error) {

	result := &PhaseResult{
		Phase:       phase,
		TestResults: make([]TestResult, 0, len(tests)),
		StartedAt:   time.Now(),
	}

	// Create jobs for this phase
	jobs := make([]ValidationJob, len(tests))
	for i, test := range tests {
		jobs[i] = ValidationJob{
			ID:           fmt.Sprintf("%s-%s-%d", hypothesisID, test.RefereeName, i),
			HypothesisID: hypothesisID,
			TestName:     test.RefereeName,
			Referee:      GetRefereeByName(test.RefereeName),
			Phase:        phase,
			ComputeWeight: ve.getComputeWeightForTest(test.RefereeName),
			Timeout:      ve.getTimeoutForTest(test.RefereeName),
			CreatedAt:    time.Now(),
		}
	}

	// Submit all jobs for this phase
	resultChan := make(chan TestResult, len(jobs))
	for _, job := range jobs {
		jobCopy := job // Create a copy to avoid closure issues
		go func(j ValidationJob) {
			testResult := ve.executeTest(j)
			resultChan <- testResult
		}(jobCopy)
	}

	// Collect results with timeout
	timeout := time.After(5 * time.Minute) // 5 minute timeout per phase

	for i := 0; i < len(jobs); i++ {
		select {
		case testResult := <-resultChan:
			result.TestResults = append(result.TestResults, testResult)
			log.Printf("[ValidationEngine] Test %s completed in phase %d: passed=%v, e-value=%.2f",
				testResult.Job.TestName, phase, testResult.Result.Passed, testResult.Result.EValue)

		case <-timeout:
			return nil, fmt.Errorf("phase %d timed out after 5 minutes", phase)

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.CombinedEValue = ve.calculatePhaseEValue(result.TestResults)

	return result, nil
}

// executeTest runs a single test with proper error handling and UI updates
func (ve *ValidationEngine) executeTest(job ValidationJob) TestResult {
	result := TestResult{
		Job:       job,
		StartedAt: time.Now(),
	}

	defer func() {
		result.EndedAt = time.Now()
		result.Duration = result.EndedAt.Sub(result.StartedAt)
	}()

	// Get semaphore for this phase
	sem := ve.getSemaphoreForPhase(job.Phase)
	if sem == nil {
		result.Error = fmt.Errorf("no semaphore available for phase %d", job.Phase)
		return result
	}

	// Acquire semaphore (weighted by compute cost)
	ctx, cancel := context.WithTimeout(context.Background(), job.Timeout)
	defer cancel()

	if err := sem.Acquire(ctx, job.ComputeWeight); err != nil {
		result.Error = fmt.Errorf("failed to acquire semaphore: %w", err)
		return result
	}
	defer sem.Release(job.ComputeWeight)

	// Broadcast test start
	if err := ve.uiBroadcaster.BroadcastTestStatusUpdate(
		"", // sessionID would come from job context
		job.HypothesisID,
		job.TestName,
		ve.getShortNameForTest(job.TestName),
		false, // not passed yet
		false, // not completed yet
	); err != nil {
		log.Printf("[ValidationEngine] Failed to broadcast test start: %v", err)
	}

	// Execute the test
	// Note: In real implementation, we'd need to get actual test data (x, y values)
	// For now, this is a placeholder
	testResult := job.Referee.Execute([]float64{1, 2, 3}, []float64{4, 5, 6}, job.TestConfig.Metadata)

	result.Result = testResult

	// Broadcast test completion
	passed := testResult.Passed
	if err := ve.uiBroadcaster.BroadcastTestStatusUpdate(
		"", // sessionID would come from job context
		job.HypothesisID,
		job.TestName,
		ve.getShortNameForTest(job.TestName),
		passed,
		true, // completed
	); err != nil {
		log.Printf("[ValidationEngine] Failed to broadcast test completion: %v", err)
	}

	return result
}

// Helper methods

func (ve *ValidationEngine) getSemaphoreForPhase(phase int) *semaphore.Weighted {
	switch phase {
	case 0:
		return ve.integritySem
	case 1:
		return ve.causalitySem
	case 2:
		return ve.complexitySem
	default:
		return nil
	}
}

func (ve *ValidationEngine) getComputeWeightForTest(testName string) int64 {
	// Weight tests by computational complexity
	weights := map[string]int64{
		"Permutation_Shredder": 1,
		"Chow_Stability_Test":  2,
		"Transfer_Entropy":     3,
		"Persistent_Homology":  8,
		"Algorithmic_Complexity": 6,
	}

	if weight, exists := weights[testName]; exists {
		return weight
	}
	return 1 // Default weight
}

func (ve *ValidationEngine) getTimeoutForTest(testName string) time.Duration {
	// Timeout based on test complexity
	timeouts := map[string]time.Duration{
		"Permutation_Shredder": 30 * time.Second,
		"Chow_Stability_Test":  45 * time.Second,
		"Transfer_Entropy":     60 * time.Second,
		"Persistent_Homology":  5 * time.Minute,
		"Algorithmic_Complexity": 3 * time.Minute,
	}

	if timeout, exists := timeouts[testName]; exists {
		return timeout
	}
	return 60 * time.Second // Default timeout
}

func (ve *ValidationEngine) getShortNameForTest(testName string) string {
	// Convert full test names to UI-friendly short names
	shortNames := map[string]string{
		"Permutation_Shredder":    "Shredder",
		"Chow_Stability_Test":     "Stability",
		"Transfer_Entropy":        "Direction",
		"Conditional_MI":          "Anti-Conf",
		"LOO_Cross_Validation":    "Sensitivity",
		"Isotonic_Mechanism_Check": "Mechanism",
		"Persistent_Homology":     "Topology",
		"Algorithmic_Complexity":  "Thermo",
		"Synthetic_Intervention":  "Counterfactual",
		"Wavelet_Coherence":       "Spectral",
	}

	if short, exists := shortNames[testName]; exists {
		return short
	}
	return testName
}

func (ve *ValidationEngine) filterTestsByPhase(tests []stats.SelectedTest, phase int) []stats.SelectedTest {
	var filtered []stats.SelectedTest
	for _, test := range tests {
		// Map category to phase (this would be more sophisticated in practice)
		testPhase := ve.getPhaseForCategory(test.Category)
		if testPhase == phase {
			filtered = append(filtered, test)
		}
	}
	return filtered
}

func (ve *ValidationEngine) getPhaseForCategory(category stats.RefereeCategory) int {
	switch category {
	case stats.CategorySHREDDER:
		return 0 // Integrity
	case stats.CategoryDIRECTIONAL, stats.CategoryANTI_CONFOUNDER:
		return 1 // Causality
	case stats.CategoryINVARIANCE, stats.CategoryMECHANISM, stats.CategorySENSITIVITY,
		 stats.CategoryTOPOLOGICAL, stats.CategoryTHERMODYNAMIC, stats.CategoryCOUNTERFACTUAL,
		 stats.CategorySPECTRAL:
		return 2 // Complexity
	default:
		return 0
	}
}

func (ve *ValidationEngine) passesIntegrityThreshold(phaseResult *PhaseResult) bool {
	// Integrity threshold: combined E-value must be >= 0.1 to proceed
	return phaseResult.CombinedEValue >= 0.1
}

func (ve *ValidationEngine) calculatePhaseEValue(results []TestResult) float64 {
	if len(results) == 0 {
		return 0.0
	}

	// Simple average for now (could be more sophisticated)
	totalEValue := 0.0
	for _, result := range results {
		totalEValue += result.Result.EValue
	}

	return totalEValue / float64(len(results))
}

func (ve *ValidationEngine) calculateOverallEValue(summary *ValidationSummary) float64 {
	// Weighted combination of phase E-values
	integrityWeight := 0.5
	causalityWeight := 0.3
	complexityWeight := 0.2

	overall := 0.0

	if result, exists := summary.PhaseResults[0]; exists {
		overall += result.CombinedEValue * integrityWeight
	}
	if result, exists := summary.PhaseResults[1]; exists {
		overall += result.CombinedEValue * causalityWeight
	}
	if result, exists := summary.PhaseResults[2]; exists {
		overall += result.CombinedEValue * complexityWeight
	}

	return overall
}

// Worker pool processors (run as goroutines)

func (ve *ValidationEngine) processIntegrityPhase() {
	ve.wg.Add(1)
	defer ve.wg.Done()

	for {
		select {
		case job := <-ve.jobQueue:
			if job.Phase != 0 {
				// Re-queue for correct phase
				select {
				case ve.jobQueue <- job:
				default:
					log.Printf("[ValidationEngine] Failed to re-queue job %s for wrong phase", job.ID)
				}
				continue
			}

			go ve.processJobWithSemaphore(job, ve.integritySem)

		case <-ve.ctx.Done():
			return
		}
	}
}

func (ve *ValidationEngine) processCausalityPhase() {
	ve.wg.Add(1)
	defer ve.wg.Done()

	for {
		select {
		case job := <-ve.jobQueue:
			if job.Phase != 1 {
				select {
				case ve.jobQueue <- job:
				default:
					log.Printf("[ValidationEngine] Failed to re-queue job %s for wrong phase", job.ID)
				}
				continue
			}

			go ve.processJobWithSemaphore(job, ve.causalitySem)

		case <-ve.ctx.Done():
			return
		}
	}
}

func (ve *ValidationEngine) processComplexityPhase() {
	ve.wg.Add(1)
	defer ve.wg.Done()

	for {
		select {
		case job := <-ve.jobQueue:
			if job.Phase != 2 {
				select {
				case ve.jobQueue <- job:
				default:
					log.Printf("[ValidationEngine] Failed to re-queue job %s for wrong phase", job.ID)
				}
				continue
			}

			go ve.processJobWithSemaphore(job, ve.complexitySem)

		case <-ve.ctx.Done():
			return
		}
	}
}

func (ve *ValidationEngine) processJobWithSemaphore(job ValidationJob, sem *semaphore.Weighted) {
	// Acquire semaphore
	if err := sem.Acquire(ve.ctx, job.ComputeWeight); err != nil {
		log.Printf("[ValidationEngine] Failed to acquire semaphore for job %s: %v", job.ID, err)
		return
	}
	defer sem.Release(job.ComputeWeight)

	// Execute job
	result := ve.executeTest(job)

	// Send result
	select {
	case ve.resultsChan <- result:
	case <-ve.ctx.Done():
		return
	}
}

func (ve *ValidationEngine) aggregateResults() {
	ve.wg.Add(1)
	defer ve.wg.Done()

	for {
		select {
		case result := <-ve.resultsChan:
			log.Printf("[ValidationEngine] Aggregated result for job %s: passed=%v, duration=%v",
				result.Job.ID, result.Result.Passed, result.Duration)

			// Remove from active jobs
			ve.jobMu.Lock()
			delete(ve.activeJobs, result.Job.ID)
			ve.jobMu.Unlock()

		case <-ve.ctx.Done():
			return
		}
	}
}

// Data structures for results

type ValidationSummary struct {
	HypothesisID      string
	SessionID         string
	OverallEValue     float64
	PhaseResults      map[int]*PhaseResult
	StartedAt         time.Time
	CompletedAt       time.Time
	Duration          time.Duration
	EarlyTermination  bool
	TerminationReason string
}

type PhaseResult struct {
	Phase          int
	TestResults    []TestResult
	CombinedEValue float64
	StartedAt      time.Time
	CompletedAt    time.Time
	Duration       time.Duration
}
