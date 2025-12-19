package research

import (
	"context"
	"fmt"
	"log"
	"time"
)

// StartWorkerPool starts a pool of workers for handling research requests
func (rw *ResearchWorker) StartWorkerPool(numWorkers int) {
	log.Printf("[ResearchWorker] 🚀 Starting worker pool with %d workers", numWorkers)
	for i := 0; i < numWorkers; i++ {
		go rw.workerLoop(i)
	}
}

// workerLoop runs the worker event loop with timeout handling and session cleanup
func (rw *ResearchWorker) workerLoop(workerID int) {
	log.Printf("[ResearchWorker] 👷 Worker %d started", workerID)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	sessionTimeout := 45 * time.Minute // Increased timeout for complex research
	cleanupInterval := 5 * time.Minute

	lastCleanup := time.Now()
	lastSessionCount := 0

	for range ticker.C {
		now := time.Now()

		// Periodic cleanup of old sessions
		if now.Sub(lastCleanup) >= cleanupInterval {
			removed := rw.sessionMgr.CleanupOldSessions(2 * time.Hour) // Keep sessions for 2 hours
			if removed > 0 {
				log.Printf("[ResearchWorker] 🧹 Cleaned up %d old sessions", removed)
			}
			lastCleanup = now
		}

		// Check for timed-out sessions and clean them up
		activeSessions, err := rw.sessionMgr.GetActiveSessions(context.Background())
		if err != nil {
			log.Printf("[ResearchWorker] ❌ Failed to get active sessions: %v", err)
			return
		}
		timeoutCount := 0

		for _, session := range activeSessions {
			sessionAge := now.Sub(session.StartedAt)

			// Check if session has been running too long
			if sessionAge > sessionTimeout {
				log.Printf("[ResearchWorker] ⏰ Session %s timed out after %.1f minutes", session.ID, sessionAge.Minutes())
				rw.sessionMgr.SetSessionError(context.Background(), session.ID.String(), fmt.Sprintf("Session timed out after %.1f minutes", sessionAge.Minutes()))
				timeoutCount++
			}
		}

		if timeoutCount > 0 {
			log.Printf("[ResearchWorker] ⏰ Timed out %d sessions", timeoutCount)
		}

		// Log active session count periodically (only when count changes)
		if len(activeSessions) != lastSessionCount {
			log.Printf("[ResearchWorker] 📊 %d active research sessions", len(activeSessions))
			lastSessionCount = len(activeSessions)
		}
	}
}