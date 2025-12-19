package ai

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// PromptManager - Simple external prompt loader
type PromptManager struct {
	PromptsDir string
}

// NewPromptManager creates a prompt manager
func NewPromptManager(promptsDir string) *PromptManager {
	log.Printf("[PromptManager] Initializing prompt manager - directory: %s", promptsDir)
	return &PromptManager{PromptsDir: promptsDir}
}

// LoadPrompt loads a prompt template by name
func (pm *PromptManager) LoadPrompt(name string) (string, error) {
	path := filepath.Join(pm.PromptsDir, name+".txt")
	log.Printf("[PromptManager] 📂 Loading prompt template: %s", name)
	log.Printf("[PromptManager]   • Directory: %s", pm.PromptsDir)
	log.Printf("[PromptManager]   • Full path: %s", path)

	// Check if file exists first
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			log.Printf("[PromptManager] ❌ File does not exist: %s", path)
			return "", fmt.Errorf("prompt template not found: %s", name)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[PromptManager] ❌ ERROR: Failed to read prompt file %s: %v", name, err)
		return "", fmt.Errorf("failed to load prompt %s: %w", name, err)
	}

	lineCount := strings.Count(string(content), "\n") + 1
	log.Printf("[PromptManager] ✅ Successfully loaded prompt: %s", name)
	log.Printf("[PromptManager]   • Size: %d bytes", len(content))
	log.Printf("[PromptManager]   • Lines: %d", lineCount)

	return string(content), nil
}

// RenderPrompt replaces {PLACEHOLDER} with values
func (pm *PromptManager) RenderPrompt(name string, replacements map[string]string) (string, error) {
	log.Printf("[PromptManager] ═══ RENDERING PROMPT TEMPLATE ═══")
	log.Printf("[PromptManager] 📝 Template name: %s", name)
	log.Printf("[PromptManager] 📂 Template path: prompts/%s.txt", name)
	log.Printf("[PromptManager] 🔧 Replacements to apply: %d", len(replacements))

	template, err := pm.LoadPrompt(name)
	if err != nil {
		log.Printf("[PromptManager] ❌ Failed to load template: %v", err)
		return "", err
	}

	originalSize := len(template)
	log.Printf("[PromptManager] ✅ Template loaded successfully (%d bytes)", originalSize)

	result := template
	replacementsMade := 0
	for placeholder, value := range replacements {
		placeholderKey := "{" + placeholder + "}"
		if strings.Contains(result, placeholderKey) {
			beforeLen := len(result)
			result = strings.ReplaceAll(result, placeholderKey, value)
			afterLen := len(result)
			delta := afterLen - beforeLen
			replacementsMade++
			log.Printf("[PromptManager]   ✓ Replaced {%s}: injected %d bytes (Δ %+d)", placeholder, len(value), delta)
		} else {
			log.Printf("[PromptManager]   ⚠️  Warning: Placeholder {%s} not found in %s.txt", placeholder, name)
		}
	}

	finalSize := len(result)
	log.Printf("[PromptManager] ═══ RENDERING COMPLETE ═══")
	log.Printf("[PromptManager]   • Template: prompts/%s.txt", name)
	log.Printf("[PromptManager]   • Replacements: %d/%d successful", replacementsMade, len(replacements))
	log.Printf("[PromptManager]   • Original size: %d bytes", originalSize)
	log.Printf("[PromptManager]   • Final size: %d bytes", finalSize)
	log.Printf("[PromptManager]   • Delta: %+d bytes", finalSize-originalSize)
	log.Printf("[PromptManager] ═══════════════════════════════\n")

	return result, nil
}
