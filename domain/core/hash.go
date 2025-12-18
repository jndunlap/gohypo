package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Hash represents a cryptographic hash
type Hash string

// NewHash creates a new hash from data
func NewHash(data []byte) Hash {
	sum := sha256.Sum256(data)
	return Hash(hex.EncodeToString(sum[:]))
}

// String returns the string representation
func (h Hash) String() string {
	return string(h)
}

// IsEmpty checks if the hash is empty
func (h Hash) IsEmpty() bool {
	return h == ""
}

// Equals checks if two hashes are equal
func (h Hash) Equals(other Hash) bool {
	return h == other
}

// Domain-specific hash types
type (
	RegistryHash  Hash
	StageListHash Hash
	CohortHash    Hash
	CodeVersion   Hash
)

// Constructors
func NewRegistryHash(data []byte) RegistryHash   { return RegistryHash(NewHash(data)) }
func NewStageListHash(data []byte) StageListHash { return StageListHash(NewHash(data)) }
func NewCohortHash(data []byte) CohortHash       { return CohortHash(NewHash(data)) }
func NewCodeVersion(data []byte) CodeVersion     { return CodeVersion(NewHash(data)) }

// String conversions
func (h RegistryHash) String() string  { return Hash(h).String() }
func (h StageListHash) String() string { return Hash(h).String() }
func (h CohortHash) String() string    { return Hash(h).String() }
func (h CodeVersion) String() string   { return Hash(h).String() }

// Hash computation helpers
func ComputeRegistryHash(contracts map[string]interface{}) RegistryHash {
	keys := make([]string, 0, len(contracts))
	for k := range contracts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var data strings.Builder
	for _, key := range keys {
		data.WriteString(key)
		data.WriteString(fmt.Sprintf("%v", contracts[key]))
	}

	return NewRegistryHash([]byte(data.String()))
}

func ComputeStageListHash(stages []interface{}) StageListHash {
	var data strings.Builder
	for _, stage := range stages {
		data.WriteString(fmt.Sprintf("%v", stage))
	}
	return NewStageListHash([]byte(data.String()))
}

func ComputeCohortHash(entityIDs []string, filters map[string]interface{}) CohortHash {
	sort.Strings(entityIDs)

	var data strings.Builder
	for _, id := range entityIDs {
		data.WriteString(id)
	}

	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		data.WriteString(key)
		data.WriteString(fmt.Sprintf("%v", filters[key]))
	}

	return NewCohortHash([]byte(data.String()))
}
