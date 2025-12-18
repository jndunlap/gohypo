package ai

import (
	"fmt"
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
	return &PromptManager{PromptsDir: promptsDir}
}

// LoadPrompt loads a prompt template by name
func (pm *PromptManager) LoadPrompt(name string) (string, error) {
	path := filepath.Join(pm.PromptsDir, name+".txt")
	content, err := os.ReadFile(path)
	if err != nil {
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
		result = strings.ReplaceAll(result, "{"+placeholder+"}", value)
	}

	return result, nil
}
