package research

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ResearchStorage handles persistence of research hypotheses
type ResearchStorage struct {
	BaseDir            string
	simulatedArtifacts []map[string]interface{}
}

// NewResearchStorage creates a new research storage instance
func NewResearchStorage(baseDir string) *ResearchStorage {
	return &ResearchStorage{
		BaseDir: baseDir,
	}
}

// EnsureBaseDir creates the base directory if it doesn't exist
func (rs *ResearchStorage) EnsureBaseDir() error {
	return os.MkdirAll(rs.BaseDir, 0755)
}

// SaveHypothesis saves a hypothesis result to disk
func (rs *ResearchStorage) SaveHypothesis(result *HypothesisResult) error {
	if err := rs.EnsureBaseDir(); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.json", result.CreatedAt.Format("2006-01-02_15-04-05"), result.ID)
	filepath := filepath.Join(rs.BaseDir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hypothesis: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write hypothesis file: %w", err)
	}

	return nil
}

// GetByID retrieves a hypothesis by its ID
func (rs *ResearchStorage) GetByID(id string) (*HypothesisResult, error) {
	files, err := rs.listHypothesisFiles()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		result, err := rs.loadHypothesisFile(file)
		if err != nil {
			continue // Skip corrupted files
		}

		if result.ID == id {
			return result, nil
		}
	}

	return nil, fmt.Errorf("hypothesis not found: %s", id)
}

// ListRecent returns the most recent hypotheses, limited by count
func (rs *ResearchStorage) ListRecent(limit int) ([]*HypothesisResult, error) {
	files, err := rs.listHypothesisFiles()
	if err != nil {
		return nil, err
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		infoI, errI := os.Stat(files[i])
		infoJ, errJ := os.Stat(files[j])

		if errI != nil || errJ != nil {
			return false
		}

		return infoI.ModTime().After(infoJ.ModTime())
	})

	var results []*HypothesisResult
	for i, file := range files {
		if i >= limit {
			break
		}

		result, err := rs.loadHypothesisFile(file)
		if err != nil {
			continue // Skip corrupted files
		}

		results = append(results, result)
	}

	return results, nil
}

// ListByValidationState returns hypotheses filtered by validation state
func (rs *ResearchStorage) ListByValidationState(validated bool, limit int) ([]*HypothesisResult, error) {
	files, err := rs.listHypothesisFiles()
	if err != nil {
		return nil, err
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		infoI, errI := os.Stat(files[i])
		infoJ, errJ := os.Stat(files[j])

		if errI != nil || errJ != nil {
			return false
		}

		return infoI.ModTime().After(infoJ.ModTime())
	})

	var results []*HypothesisResult
	for _, file := range files {
		result, err := rs.loadHypothesisFile(file)
		if err != nil {
			continue // Skip corrupted files
		}

		if result.Validated == validated {
			results = append(results, result)
			if len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// ListAll returns all hypotheses sorted by creation time (newest first)
func (rs *ResearchStorage) ListAll() ([]*HypothesisResult, error) {
	files, err := rs.listHypothesisFiles()
	if err != nil {
		return nil, err
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		infoI, errI := os.Stat(files[i])
		infoJ, errJ := os.Stat(files[j])

		if errI != nil || errJ != nil {
			return false
		}

		return infoI.ModTime().After(infoJ.ModTime())
	})

	var results []*HypothesisResult
	for _, file := range files {
		result, err := rs.loadHypothesisFile(file)
		if err != nil {
			continue // Skip corrupted files
		}

		results = append(results, result)
	}

	return results, nil
}

// GetStats returns statistics about stored hypotheses
func (rs *ResearchStorage) GetStats() (map[string]interface{}, error) {
	files, err := rs.listHypothesisFiles()
	if err != nil {
		return nil, err
	}

	totalCount := 0
	validatedCount := 0
	rejectedCount := 0
	var earliest, latest *time.Time

	for _, file := range files {
		result, err := rs.loadHypothesisFile(file)
		if err != nil {
			continue // Skip corrupted files
		}

		totalCount++

		if result.Validated {
			validatedCount++
		} else {
			rejectedCount++
		}

		if earliest == nil || result.CreatedAt.Before(*earliest) {
			earliest = &result.CreatedAt
		}

		if latest == nil || result.CreatedAt.After(*latest) {
			latest = &result.CreatedAt
		}
	}

	return map[string]interface{}{
		"total_hypotheses":    totalCount,
		"validated_count":     validatedCount,
		"rejected_count":      rejectedCount,
		"validation_rate":     float64(validatedCount) / float64(totalCount) * 100,
		"earliest_hypothesis": earliest,
		"latest_hypothesis":   latest,
	}, nil
}

// listHypothesisFiles returns all hypothesis JSON files
func (rs *ResearchStorage) listHypothesisFiles() ([]string, error) {
	var files []string

	err := filepath.WalkDir(rs.BaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// loadHypothesisFile loads a single hypothesis from a JSON file
func (rs *ResearchStorage) loadHypothesisFile(filepath string) (*HypothesisResult, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var result HypothesisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CleanupOldFiles removes hypothesis files older than the specified duration
func (rs *ResearchStorage) CleanupOldFiles(maxAge time.Duration) (int, error) {
	files, err := rs.listHypothesisFiles()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(file); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// StoreSimulatedArtifact stores a simulated statistical artifact for testing
func (rs *ResearchStorage) StoreSimulatedArtifact(sessionID string, artifact map[string]interface{}) error {
	rs.simulatedArtifacts = append(rs.simulatedArtifacts, artifact)
	return nil
}

// GetSimulatedArtifacts returns all simulated statistical artifacts
func (rs *ResearchStorage) GetSimulatedArtifacts() []map[string]interface{} {
	return rs.simulatedArtifacts
}

// ClearSimulatedArtifacts clears all simulated artifacts
func (rs *ResearchStorage) ClearSimulatedArtifacts() {
	rs.simulatedArtifacts = nil
}
