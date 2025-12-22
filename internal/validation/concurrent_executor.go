package validation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gohypo/internal/referee"
)

// RefereeCost defines the computational cost weight for each referee type
type RefereeCost struct {
	RefereeName string
	Cost        int    // Computational units (1-10 scale)
	Category    string // SHREDDER, DIRECTIONAL, etc.
}

// WeightedSemaphore manages concurrent referee execution with cost-based throttling
type WeightedSemaphore struct {
	maxCapacity int
	currentUsed int
	mu          sync.Mutex
	cond        *sync.Cond
}

// NewWeightedSemaphore creates a semaphore with total capacity
func NewWeightedSemaphore(capacity int) *WeightedSemaphore {
	ws := &WeightedSemaphore{
		maxCapacity: capacity,
		currentUsed: 0,
	}
	ws.cond = sync.NewCond(&ws.mu)
	return ws
}

// Acquire attempts to acquire capacity for a referee
func (ws *WeightedSemaphore) Acquire(ctx context.Context, cost int) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	for ws.currentUsed+cost > ws.maxCapacity {
		// Wait for capacity to become available
		ch := make(chan struct{})
		go func() {
			defer close(ch)
			ws.cond.Wait()
		}()

		select {
		case <-ch:
			// Capacity became available, check again
			continue
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for computational capacity: %w", ctx.Err())
		}
	}

	ws.currentUsed += cost
	return nil
}

// Release returns capacity to the semaphore
func (ws *WeightedSemaphore) Release(cost int) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.currentUsed -= cost
	if ws.currentUsed < 0 {
		ws.currentUsed = 0 // Safety check
	}

	ws.cond.Broadcast()
}

// GetRefereeCosts returns cost assignments based on referee complexity
func GetRefereeCosts() map[string]RefereeCost {
	return map[string]RefereeCost{
		// Low-cost statistical tests (fast computation)
		"Permutation_Shredder": {RefereeName: "Permutation_Shredder", Cost: 2, Category: "SHREDDER"},
		"LOO_Cross_Validation": {RefereeName: "LOO_Cross_Validation", Cost: 2, Category: "SENSITIVITY"},

		// Medium-cost causal inference tests
		"Chow_Stability_Test":      {RefereeName: "Chow_Stability_Test", Cost: 4, Category: "INVARIANCE"},
		"Isotonic_Mechanism_Check": {RefereeName: "Isotonic_Mechanism_Check", Cost: 4, Category: "MECHANISM"},
		"Conditional_MI":           {RefereeName: "Conditional_MI", Cost: 4, Category: "ANTI_CONFOUNDER"},

		// High-cost advanced mathematical tests
		"Transfer_Entropy":         {RefereeName: "Transfer_Entropy", Cost: 6, Category: "DIRECTIONAL"},
		"Convergent_Cross_Mapping": {RefereeName: "Convergent_Cross_Mapping", Cost: 6, Category: "DIRECTIONAL"},
		"Wavelet_Coherence":        {RefereeName: "Wavelet_Coherence", Cost: 6, Category: "SPECTRAL"},

		// Very high-cost topological/complexity tests
		"Persistent_Homology":    {RefereeName: "Persistent_Homology", Cost: 8, Category: "TOPOLOGICAL"},
		"Algorithmic_Complexity": {RefereeName: "Algorithmic_Complexity", Cost: 8, Category: "THERMODYNAMIC"},
		"Synthetic_Intervention": {RefereeName: "Synthetic_Intervention", Cost: 8, Category: "COUNTERFACTUAL"},
	}
}

// ConcurrentExecutor manages weighted referee execution
type ConcurrentExecutor struct {
	semaphore    *WeightedSemaphore
	refereeCosts map[string]RefereeCost
	maxTimeout   time.Duration
}

// NewConcurrentExecutor creates an executor with capacity management
func NewConcurrentExecutor(totalCapacity int) *ConcurrentExecutor {
	return &ConcurrentExecutor{
		semaphore:    NewWeightedSemaphore(totalCapacity),
		refereeCosts: GetRefereeCosts(),
		maxTimeout:   5 * time.Minute, // Maximum time to wait for capacity
	}
}

// ExecuteReferees runs multiple referees with cost-based throttling
func (ce *ConcurrentExecutor) ExecuteReferees(
	ctx context.Context,
	refereeNames []string,
	xData, yData []float64,
) ([]referee.RefereeResult, error) {

	results := make([]referee.RefereeResult, len(refereeNames))
	jobs := make(chan refereeJob, len(refereeNames))

	// Launch referees concurrently with cost management
	for i, refereeName := range refereeNames {
		go func(index int, name string) {
			cost := ce.refereeCosts[name].Cost
			if cost == 0 {
				cost = 3 // Default cost for unknown referees
			}

			// Acquire computational capacity
			execCtx, cancel := context.WithTimeout(ctx, ce.maxTimeout)
			defer cancel()

			if err := ce.semaphore.Acquire(execCtx, cost); err != nil {
				jobs <- refereeJob{
					index: index,
					result: referee.RefereeResult{
						GateName:      name,
						Passed:        false,
						FailureReason: fmt.Sprintf("Computational capacity unavailable: %v", err),
					},
				}
				return
			}

			// Execute referee
			start := time.Now()
			refereeInstance, err := referee.GetRefereeFactory(name)
			if err != nil {
				ce.semaphore.Release(cost)
				jobs <- refereeJob{
					index: index,
					result: referee.RefereeResult{
						GateName:      name,
						Passed:        false,
						FailureReason: fmt.Sprintf("Referee creation failed: %v", err),
					},
				}
				return
			}

			result := refereeInstance.Execute(xData, yData, nil)
			duration := time.Since(start)

			// Release capacity
			ce.semaphore.Release(cost)

			jobs <- refereeJob{
				index:    index,
				name:     name,
				result:   result,
				duration: duration,
				cost:     cost,
			}
		}(i, refereeName)
	}

	// Collect results
	for i := 0; i < len(refereeNames); i++ {
		job := <-jobs
		results[job.index] = job.result

		// Log execution metrics
		if job.result.Passed {
			log.Printf("[ConcurrentExecutor] ✅ %s passed (cost: %d, duration: %v)",
				job.name, job.cost, job.duration)
		} else {
			log.Printf("[ConcurrentExecutor] ❌ %s failed (cost: %d, duration: %v): %s",
				job.name, job.cost, job.duration, job.result.FailureReason)
		}
	}

	return results, nil
}

type refereeJob struct {
	index    int
	name     string
	result   referee.RefereeResult
	duration time.Duration
	cost     int
}
