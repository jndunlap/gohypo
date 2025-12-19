package research

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gohypo/ports"

	"github.com/google/uuid"
)

// savePromptToFile saves the rendered prompt to both database and file
func (rw *ResearchWorker) savePromptToFile(ctx context.Context, sessionID, prompt string) error {
	// Parse session ID
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Get default user
	user, err := rw.storage.GetDefaultUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	// Save to database
	if promptRepo, ok := rw.promptRepo.(ports.PromptRepository); ok {
		metadata := map[string]interface{}{
			"saved_at":    time.Now().Format(time.RFC3339),
			"file_backup": true,
		}
		if err := promptRepo.SavePrompt(ctx, user.ID, sessionUUID, prompt, "research_directive", metadata); err != nil {
			log.Printf("[ResearchWorker] ⚠️ Failed to save prompt to database: %v", err)
			// Continue with file backup even if DB save fails
		}
	}

	// Also save to file as backup (existing behavior)
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("research_prompts/%s_%s_prompt.txt", timestamp, sessionID[:8])

	// Ensure directory exists
	if err := os.MkdirAll("research_prompts", 0755); err != nil {
		return fmt.Errorf("failed to create research_prompts directory: %w", err)
	}

	// Write prompt to file
	err = os.WriteFile(filename, []byte(prompt), 0644)
	if err != nil {
		return fmt.Errorf("failed to write prompt file %s: %w", filename, err)
	}

	return nil
}