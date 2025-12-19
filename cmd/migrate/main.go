package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gohypo/adapters/postgres"
	"gohypo/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var defaultUserID = uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: migrate <database_url> <research_output_dir>")
	}

	databaseURL := os.Args[1]
	researchDir := os.Args[2]

	log.Printf("Starting migration from %s to database %s", researchDir, databaseURL)

	// Connect to database
	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := postgres.NewUserRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	hypothesisRepo := postgres.NewHypothesisRepository(db)

	// Get or create default user
	ctx := context.Background()
	user, err := userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		log.Fatalf("Failed to get/create default user: %v", err)
	}
	log.Printf("Using user: %s (%s)", user.Username, user.ID)

	// Find all JSON files
	files, err := findHypothesisFiles(researchDir)
	if err != nil {
		log.Fatalf("Failed to find hypothesis files: %v", err)
	}

	log.Printf("Found %d hypothesis files to migrate", len(files))

	migrated := 0
	skipped := 0

	for _, file := range files {
		// Load hypothesis first to check if it's valid
		hypothesis, err := loadHypothesisFromFile(file)
		if err != nil {
			log.Printf("Failed to load hypothesis from %s: %v", file, err)
			skipped++
			continue
		}

		// Extract session ID from execution metadata if available
		sessionID, err := extractSessionID(file)
		if err != nil {
			log.Printf("Could not extract session ID from %s: %v", file, err)
			sessionID = uuid.New() // Create new session for migrated data
		}

		// Try to get existing session, create new one if it doesn't exist
		session, err := sessionRepo.GetSession(ctx, user.ID, sessionID)
		if err != nil {
			// Create a new session for migrated data
			session, err = sessionRepo.CreateSession(ctx, user.ID, map[string]interface{}{
				"migrated":    true,
				"source_file": filepath.Base(file),
				"migrated_at": time.Now(),
			})
			if err != nil {
				log.Printf("Failed to create session for %s: %v", file, err)
				skipped++
				continue
			}
			log.Printf("Created new session %s for migrated file %s", session.ID, filepath.Base(file))
		}

		// Set session ID on hypothesis
		hypothesis.SessionID = session.ID.String()

		// Save hypothesis
		if err := hypothesisRepo.SaveHypothesis(ctx, user.ID, session.ID, hypothesis); err != nil {
			log.Printf("Failed to save hypothesis %s: %v", hypothesis.ID, err)
			skipped++
			continue
		}

		// Mark session as completed
		if err := sessionRepo.UpdateSessionState(ctx, user.ID, session.ID, models.SessionStateComplete); err != nil {
			log.Printf("Failed to update session state for %s: %v", session.ID, err)
			// Don't skip - hypothesis was saved successfully
		}

		migrated++
		log.Printf("Migrated hypothesis %s from %s", hypothesis.ID, filepath.Base(file))
	}

	log.Printf("Migration complete: %d migrated, %d skipped", migrated, skipped)
}

func findHypothesisFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".json") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func extractSessionID(filePath string) (uuid.UUID, error) {
	hypothesis, err := loadHypothesisFromFile(filePath)
	if err != nil {
		return uuid.Nil, err
	}

	// Try to extract session ID from execution metadata
	if hypothesis.ExecutionMetadata != nil {
		if sessionIDStr, ok := hypothesis.ExecutionMetadata["session_id"].(string); ok {
			return uuid.Parse(sessionIDStr)
		}
	}

	// Fallback: generate deterministic UUID based on file path
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(filePath)), nil
}

func loadHypothesisFromFile(filePath string) (*models.HypothesisResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var result models.HypothesisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
