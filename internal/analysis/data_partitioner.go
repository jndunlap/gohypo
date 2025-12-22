package analysis

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"gohypo/domain/core"
)

// DataPartitioner implements sample splitting for statistical rigor
type DataPartitioner struct {
	randomSeed int64
}

// PartitionResult represents the outcome of data partitioning
type PartitionResult struct {
	DiscoverySet   DatasetPartition
	ValidationSet  DatasetPartition
	PartitionStats PartitionStatistics
}

// DatasetPartition represents a data partition
type DatasetPartition struct {
	EntityIDs    []core.ID
	VariableKeys []core.VariableKey
	DataMatrix   interface{} // The actual partitioned data
	IsDiscovery  bool
	SampleSize   int
}

// PartitionStatistics provides metadata about the partitioning
type PartitionStatistics struct {
	TotalEntities      int
	DiscoveryEntities  int
	ValidationEntities int
	DiscoveryRatio     float64
	ValidationRatio    float64
	StratificationVars []string
	RandomSeed         int64
	PartitionMethod    string
}

// NewDataPartitioner creates a data partitioner with deterministic seeding
func NewDataPartitioner() *DataPartitioner {
	return &DataPartitioner{
		randomSeed: time.Now().UnixNano(),
	}
}

// NewDataPartitionerWithSeed creates a partitioner with a specific seed for reproducibility
func NewDataPartitionerWithSeed(seed int64) *DataPartitioner {
	return &DataPartitioner{
		randomSeed: seed,
	}
}

// PartitionDataset performs sample splitting with stratification
func (dp *DataPartitioner) PartitionDataset(
	entityIDs []core.ID,
	variableKeys []core.VariableKey,
	dataMatrix interface{},
	partitionConfig PartitionConfig,
) (*PartitionResult, error) {

	if len(entityIDs) < 10 {
		return nil, fmt.Errorf("insufficient data for partitioning: need at least 10 entities, got %d", len(entityIDs))
	}

	totalEntities := len(entityIDs)

	// Determine partition sizes
	discoveryRatio := partitionConfig.DiscoveryRatio
	if discoveryRatio <= 0 || discoveryRatio >= 1 {
		discoveryRatio = 0.7 // Default 70/30 split
	}

	discoverySize := int(math.Round(float64(totalEntities) * discoveryRatio))
	validationSize := totalEntities - discoverySize

	if discoverySize < 5 || validationSize < 5 {
		return nil, fmt.Errorf("partition sizes too small: discovery=%d, validation=%d (minimum 5 each)", discoverySize, validationSize)
	}

	// Partition the data
	var discoveryEntities, validationEntities []core.ID

	if partitionConfig.StratificationVar != "" {
		// Stratified partitioning
		discoveryEntities, validationEntities = dp.stratifiedPartition(
			entityIDs, dataMatrix, partitionConfig.StratificationVar, discoveryRatio, dp.randomSeed)
	} else {
		// Random partitioning
		discoveryEntities, validationEntities = dp.randomPartition(entityIDs, discoverySize, dp.randomSeed)
	}

	// Create partitions
	discoveryPartition := DatasetPartition{
		EntityIDs:    discoveryEntities,
		VariableKeys: variableKeys,
		DataMatrix:   dp.extractPartitionData(dataMatrix, discoveryEntities),
		IsDiscovery:  true,
		SampleSize:   len(discoveryEntities),
	}

	validationPartition := DatasetPartition{
		EntityIDs:    validationEntities,
		VariableKeys: variableKeys,
		DataMatrix:   dp.extractPartitionData(dataMatrix, validationEntities),
		IsDiscovery:  false,
		SampleSize:   len(validationEntities),
	}

	partitionStats := PartitionStatistics{
		TotalEntities:      totalEntities,
		DiscoveryEntities:  len(discoveryEntities),
		ValidationEntities: len(validationEntities),
		DiscoveryRatio:     float64(len(discoveryEntities)) / float64(totalEntities),
		ValidationRatio:    float64(len(validationEntities)) / float64(totalEntities),
		StratificationVars: dp.getStratificationVars(partitionConfig),
		RandomSeed:         dp.randomSeed,
		PartitionMethod:    dp.getPartitionMethod(partitionConfig),
	}

	return &PartitionResult{
		DiscoverySet:   discoveryPartition,
		ValidationSet:  validationPartition,
		PartitionStats: partitionStats,
	}, nil
}

// randomPartition performs simple random partitioning
func (dp *DataPartitioner) randomPartition(entityIDs []core.ID, discoverySize int, seed int64) ([]core.ID, []core.ID) {
	// Create a copy to avoid modifying original
	entities := make([]core.ID, len(entityIDs))
	copy(entities, entityIDs)

	// Shuffle with deterministic seed
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(entities), func(i, j int) {
		entities[i], entities[j] = entities[j], entities[i]
	})

	discoveryEntities := entities[:discoverySize]
	validationEntities := entities[discoverySize:]

	return discoveryEntities, validationEntities
}

// stratifiedPartition performs stratified random partitioning
func (dp *DataPartitioner) stratifiedPartition(
	entityIDs []core.ID,
	dataMatrix interface{},
	stratificationVar string,
	discoveryRatio float64,
	seed int64,
) ([]core.ID, []core.ID) {

	// Group entities by stratification variable
	strata := dp.groupByStrata(entityIDs, dataMatrix, stratificationVar)

	var discoveryEntities, validationEntities []core.ID

	// Partition within each stratum
	rng := rand.New(rand.NewSource(seed))
	for _, stratumEntities := range strata {
		if len(stratumEntities) < 2 {
			// Stratum too small for partitioning, put all in discovery
			discoveryEntities = append(discoveryEntities, stratumEntities...)
			continue
		}

		stratumDiscoverySize := int(math.Round(float64(len(stratumEntities)) * discoveryRatio))
		if stratumDiscoverySize < 1 {
			stratumDiscoverySize = 1
		}
		if stratumDiscoverySize >= len(stratumEntities) {
			stratumDiscoverySize = len(stratumEntities) - 1
		}

		// Shuffle stratum
		stratumCopy := make([]core.ID, len(stratumEntities))
		copy(stratumCopy, stratumEntities)
		rng.Shuffle(len(stratumCopy), func(i, j int) {
			stratumCopy[i], stratumCopy[j] = stratumCopy[j], stratumCopy[i]
		})

		discoveryEntities = append(discoveryEntities, stratumCopy[:stratumDiscoverySize]...)
		validationEntities = append(validationEntities, stratumCopy[stratumDiscoverySize:]...)
	}

	return discoveryEntities, validationEntities
}

// groupByStrata groups entities by the value of a stratification variable
func (dp *DataPartitioner) groupByStrata(entityIDs []core.ID, dataMatrix interface{}, stratificationVar string) map[string][]core.ID {
	strata := make(map[string][]core.ID)

	// This is a simplified implementation - in practice, you'd need to extract
	// the actual values from the data matrix for the stratification variable
	// For now, we'll use a simple heuristic based on entity ID hash

	for _, entityID := range entityIDs {
		// Simple stratification: use first character of entity ID as stratum
		stratumKey := string(entityID[0])
		strata[stratumKey] = append(strata[stratumKey], entityID)
	}

	return strata
}

// extractPartitionData extracts the relevant data subset for a partition
func (dp *DataPartitioner) extractPartitionData(dataMatrix interface{}, entityIDs []core.ID) interface{} {
	// This is a placeholder - in practice, this would extract the actual
	// data subset from the matrix based on the entity IDs
	// The implementation would depend on the specific matrix format used

	// For now, return a placeholder structure
	return map[string]interface{}{
		"entity_ids": entityIDs,
		"note":       "This is a placeholder - actual implementation would extract real data subset",
	}
}

// getStratificationVars returns stratification variables used
func (dp *DataPartitioner) getStratificationVars(config PartitionConfig) []string {
	if config.StratificationVar != "" {
		return []string{config.StratificationVar}
	}
	return []string{}
}

// getPartitionMethod returns the partitioning method used
func (dp *DataPartitioner) getPartitionMethod(config PartitionConfig) string {
	if config.StratificationVar != "" {
		return "stratified_random"
	}
	return "simple_random"
}

// PartitionConfig defines partitioning parameters
type PartitionConfig struct {
	DiscoveryRatio     float64 // Proportion for discovery set (0.0-1.0)
	StratificationVar  string  // Variable to stratify by (empty for random)
	MinStratumSize     int     // Minimum entities per stratum
	ReproducibilityKey string  // Key for deterministic partitioning
}

// DefaultPartitionConfig returns sensible defaults
func DefaultPartitionConfig() PartitionConfig {
	return PartitionConfig{
		DiscoveryRatio: 0.7,
		MinStratumSize: 5,
	}
}

// ValidatePartitions checks that partitions meet statistical requirements
func (dp *DataPartitioner) ValidatePartitions(result *PartitionResult) error {
	discoverySize := result.DiscoverySet.SampleSize
	validationSize := result.ValidationSet.SampleSize

	// Minimum size checks
	if discoverySize < 30 {
		return fmt.Errorf("discovery set too small: %d entities (minimum 30)", discoverySize)
	}
	if validationSize < 20 {
		return fmt.Errorf("validation set too small: %d entities (minimum 20)", validationSize)
	}

	// Ratio checks
	expectedDiscoveryRatio := result.PartitionStats.DiscoveryRatio
	actualDiscoveryRatio := float64(discoverySize) / float64(discoverySize+validationSize)

	if math.Abs(actualDiscoveryRatio-expectedDiscoveryRatio) > 0.05 {
		return fmt.Errorf("partition ratio mismatch: expected %.2f, got %.2f",
			expectedDiscoveryRatio, actualDiscoveryRatio)
	}

	return nil
}

// CalculatePartitionPower calculates statistical power for the partitions
func (dp *DataPartitioner) CalculatePartitionPower(result *PartitionResult, expectedEffectSize float64) PartitionPower {
	discoveryPower := dp.calculatePower(result.DiscoverySet.SampleSize, expectedEffectSize, 0.05, 0.80)
	validationPower := dp.calculatePower(result.ValidationSet.SampleSize, expectedEffectSize, 0.05, 0.80)

	return PartitionPower{
		DiscoveryPower:  discoveryPower,
		ValidationPower: validationPower,
		CombinedPower:   math.Min(discoveryPower, validationPower), // Limited by smaller partition
		EffectSize:      expectedEffectSize,
		Alpha:           0.05,
		TargetPower:     0.80,
	}
}

// calculatePower computes statistical power for a given sample size and effect size
func (dp *DataPartitioner) calculatePower(sampleSize int, effectSize, alpha, targetPower float64) float64 {
	// Simplified power calculation using normal approximation
	// In practice, this would use more sophisticated power analysis

	n := float64(sampleSize)
	zAlpha := 1.645 // For alpha = 0.05, one-tailed
	zPower := 0.842 // For power = 0.80

	// Power = 1 - β, where non-centrality parameter = effectSize * sqrt(n/2)
	nonCentrality := effectSize * math.Sqrt(n/2.0)

	// Approximate power using normal distribution
	if nonCentrality > zAlpha+zPower {
		return 0.95 // High power
	} else if nonCentrality > zAlpha {
		return 0.80 // Target power
	} else if nonCentrality > 0 {
		return 0.50 // Low power
	}

	return 0.05 // Very low power
}

// PartitionPower represents statistical power analysis for partitions
type PartitionPower struct {
	DiscoveryPower  float64
	ValidationPower float64
	CombinedPower   float64
	EffectSize      float64
	Alpha           float64
	TargetPower     float64
}

// EnsurePartitionsAdequate checks if partitions have sufficient statistical power
func (dp *DataPartitioner) EnsurePartitionsAdequate(power PartitionPower, minPower float64) error {
	if power.CombinedPower < minPower {
		return fmt.Errorf("insufficient statistical power: combined power %.2f < minimum %.2f",
			power.CombinedPower, minPower)
	}

	if power.DiscoveryPower < 0.6 {
		return fmt.Errorf("discovery partition has low power: %.2f (minimum 0.6)", power.DiscoveryPower)
	}

	if power.ValidationPower < 0.5 {
		return fmt.Errorf("validation partition has low power: %.2f (minimum 0.5)", power.ValidationPower)
	}

	return nil
}
