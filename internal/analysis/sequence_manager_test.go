package analysis

import (
	"sync"
	"testing"
)

func TestSequenceManager_Next(t *testing.T) {
	sm := NewSequenceManager()

	// Test sequential calls
	id1 := sm.Next()
	id2 := sm.Next()
	id3 := sm.Next()

	if id1 != 1 {
		t.Errorf("Expected first ID to be 1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("Expected second ID to be 2, got %d", id2)
	}
	if id3 != 3 {
		t.Errorf("Expected third ID to be 3, got %d", id3)
	}
}

func TestSequenceManager_GetCurrent(t *testing.T) {
	sm := NewSequenceManager()

	// Initially should be 0
	if current := sm.GetCurrent(); current != 0 {
		t.Errorf("Expected initial current to be 0, got %d", current)
	}

	// After calling Next, GetCurrent should return the last issued ID
	sm.Next()
	if current := sm.GetCurrent(); current != 1 {
		t.Errorf("Expected current to be 1 after first Next(), got %d", current)
	}

	sm.Next()
	sm.Next()
	if current := sm.GetCurrent(); current != 3 {
		t.Errorf("Expected current to be 3 after three Next() calls, got %d", current)
	}
}

func TestSequenceManager_ConcurrentSafety(t *testing.T) {
	sm := NewSequenceManager()
	const numGoroutines = 100
	const idsPerGoroutine = 1000

	var wg sync.WaitGroup
	results := make(chan []int64, numGoroutines)

	// Launch multiple goroutines that each get many IDs
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := make([]int64, idsPerGoroutine)
			for j := 0; j < idsPerGoroutine; j++ {
				ids[j] = sm.Next()
			}
			results <- ids
		}()
	}

	wg.Wait()
	close(results)

	// Collect all IDs and check for uniqueness and completeness
	seen := make(map[int64]bool)
	expectedTotal := numGoroutines * idsPerGoroutine

	for ids := range results {
		for _, id := range ids {
			if seen[id] {
				t.Errorf("Duplicate ID found: %d", id)
			}
			seen[id] = true
		}
	}

	if len(seen) != expectedTotal {
		t.Errorf("Expected %d unique IDs, got %d", expectedTotal, len(seen))
	}

	// Check that all IDs from 1 to expectedTotal are present
	for i := int64(1); i <= int64(expectedTotal); i++ {
		if !seen[i] {
			t.Errorf("Missing ID: %d", i)
		}
	}
}

func TestSequenceManager_ConcurrentReads(t *testing.T) {
	sm := NewSequenceManager()

	// Start background goroutine that calls Next()
	go func() {
		for i := 0; i < 1000; i++ {
			sm.Next()
		}
	}()

	// Concurrently read GetCurrent() many times
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = sm.GetCurrent()
			}
		}()
	}

	wg.Wait()
	// If we get here without panics or races, the test passes
}

func BenchmarkSequenceManager_Next(b *testing.B) {
	sm := NewSequenceManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sm.Next()
		}
	})
}
