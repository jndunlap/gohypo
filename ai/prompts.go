package ai

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Global map to track initialized prompt directories (to avoid duplicate logs)
var (
	initializedDirs   = make(map[string]bool)
	initializedDirsMu sync.RWMutex
)

// PromptManager - Simple external prompt loader
type PromptManager struct {
	PromptsDir string
}

// NewPromptManager creates a prompt manager
func NewPromptManager(promptsDir string) *PromptManager {
	// Only log initialization once per directory
	initializedDirsMu.Lock()
	if !initializedDirs[promptsDir] {
		initializedDirs[promptsDir] = true
		log.Printf("[PromptManager] Initialized for directory: %s", promptsDir)
	}
	initializedDirsMu.Unlock()

	return &PromptManager{PromptsDir: promptsDir}
}

// LoadPrompt loads a prompt template by name
func (pm *PromptManager) LoadPrompt(name string) (string, error) {
	path := filepath.Join(pm.PromptsDir, name+".txt")

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("prompt template not found: %s", name)
		}
		return "", fmt.Errorf("failed to load prompt %s: %w", name, err)
	}

	return string(content), nil
}

// RenderPrompt replaces {PLACEHOLDER} with values
func (pm *PromptManager) RenderPrompt(name string, replacements map[string]string) (string, error) {
	template, err := pm.LoadPrompt(name)
	if err != nil {
		return "", err
	}

	result := template
	for placeholder, value := range replacements {
		placeholderKey := "{" + placeholder + "}"
		result = strings.ReplaceAll(result, placeholderKey, value)
	}

	return result, nil
}
