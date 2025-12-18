package contracts

import (
	"fmt"
	"sort"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

// RegistryVersion represents an immutable snapshot of all variable contracts
type RegistryVersion struct {
	Hash      core.RegistryHash                    `json:"hash"`
	Contracts map[string]*dataset.VariableContract `json:"contracts"`
	CreatedAt core.Timestamp                       `json:"created_at"`
}

// NewRegistryVersion creates a registry version from contracts
func NewRegistryVersion(contracts map[string]*dataset.VariableContract) *RegistryVersion {
	hash := ComputeRegistryHash(contracts)

	return &RegistryVersion{
		Hash:      hash,
		Contracts: contracts,
		CreatedAt: core.Now(),
	}
}

// ComputeRegistryHash creates a deterministic hash of all contracts
func ComputeRegistryHash(contracts map[string]*dataset.VariableContract) core.RegistryHash {
	// Sort keys for deterministic hashing
	keys := make([]string, 0, len(contracts))
	for k := range contracts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var data string
	for _, key := range keys {
		contract := contracts[key]
		data += fmt.Sprintf("%s:%s:%s:%v:%s:%t;",
			key,
			contract.AsOfMode,
			contract.StatisticalType,
			contract.WindowDays,
			contract.ImputationPolicy,
			contract.ScalarGuarantee,
		)
	}

	return core.NewRegistryHash([]byte(data))
}

// RegistryManager handles registry versioning and contract compilation
type RegistryManager struct {
	versions map[core.RegistryHash]*RegistryVersion
	current  *RegistryVersion
}

// NewRegistryManager creates a registry manager
func NewRegistryManager() *RegistryManager {
	return &RegistryManager{
		versions: make(map[core.RegistryHash]*RegistryVersion),
	}
}

// AddContracts creates a new registry version from contracts
func (rm *RegistryManager) AddContracts(contracts map[string]*dataset.VariableContract) *RegistryVersion {
	version := NewRegistryVersion(contracts)
	rm.versions[version.Hash] = version
	rm.current = version
	return version
}

// GetVersion retrieves a registry version by hash
func (rm *RegistryManager) GetVersion(hash core.RegistryHash) (*RegistryVersion, bool) {
	version, exists := rm.versions[hash]
	return version, exists
}

// GetCurrent returns the current registry version
func (rm *RegistryManager) GetCurrent() *RegistryVersion {
	return rm.current
}

// GetContract retrieves a contract from the current registry
func (rm *RegistryManager) GetContract(varKey string) (*dataset.VariableContract, bool) {
	if rm.current == nil {
		return nil, false
	}
	contract, exists := rm.current.Contracts[varKey]
	return contract, exists
}

// ValidateContract checks if a contract is valid for the current registry
func (rm *RegistryManager) ValidateContract(varKey string) error {
	contract, exists := rm.GetContract(varKey)
	if !exists {
		return core.NewNotFoundError("variable contract", varKey)
	}

	// Validate contract fields
	if contract.AsOfMode == "" {
		return core.NewValidationError("contract", "as_of_mode is required")
	}

	if contract.StatisticalType == "" {
		return core.NewValidationError("contract", "statistical_type is required")
	}

	if contract.ImputationPolicy == "" {
		return core.NewValidationError("contract", "imputation_policy is required")
	}

	return nil
}
