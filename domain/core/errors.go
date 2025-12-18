package core

import (
	"errors"
	"fmt"
)

// Domain errors - centralized error definitions
var (
	// Not found errors
	ErrNotFound           = errors.New("resource not found")
	ErrHypothesisNotFound = fmt.Errorf("%w: hypothesis", ErrNotFound)
	ErrRunNotFound        = fmt.Errorf("%w: run", ErrNotFound)
	ErrSnapshotNotFound   = fmt.Errorf("%w: snapshot", ErrNotFound)
	ErrVariableNotFound   = fmt.Errorf("%w: variable", ErrNotFound)
	ErrArtifactNotFound   = fmt.Errorf("%w: artifact", ErrNotFound)

	// Validation errors
	ErrInvalidContract  = errors.New("invalid variable contract")
	ErrNonScalar        = errors.New("variable resolved to non-scalar value")
	ErrLeakage          = errors.New("data leakage detected")
	ErrInadmissible     = errors.New("hypothesis inadmissible")
	ErrConfounded       = errors.New("hypothesis confounded")
	ErrUnstable         = errors.New("hypothesis unstable")
	ErrInsufficientData = errors.New("insufficient data for analysis")

	// Determinism errors
	ErrNonDeterministic = errors.New("non-deterministic result")
	ErrSeedMismatch     = errors.New("seed mismatch")
	ErrHashMismatch     = errors.New("hash mismatch")

	// Resolution errors
	ErrResolutionFailed    = errors.New("variable resolution failed")
	ErrAsOfModeUnsupported = errors.New("unsupported as-of mode")
	ErrLagTooLarge         = errors.New("lag buffer too large")
	ErrFutureData          = errors.New("future data detected")
)

// Error constructors with context
func NewNotFoundError(resource string, id string) error {
	return fmt.Errorf("%w: %s with id %s", ErrNotFound, resource, id)
}

func NewValidationError(field string, reason string) error {
	return fmt.Errorf("validation failed for %s: %s", field, reason)
}

func NewLeakageError(maxTimestamp, cutoff string) error {
	return fmt.Errorf("%w: max_timestamp %s > cutoff %s", ErrLeakage, maxTimestamp, cutoff)
}

func NewResolutionError(varKey string, err error) error {
	return fmt.Errorf("%w for variable %s: %v", ErrResolutionFailed, varKey, err)
}

// Error checking helpers
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidContract) ||
		errors.Is(err, ErrNonScalar) ||
		errors.Is(err, ErrLeakage) ||
		errors.Is(err, ErrInadmissible)
}

func IsDeterminismError(err error) bool {
	return errors.Is(err, ErrNonDeterministic) ||
		errors.Is(err, ErrSeedMismatch) ||
		errors.Is(err, ErrHashMismatch)
}

func IsResolutionError(err error) bool {
	return errors.Is(err, ErrResolutionFailed) ||
		errors.Is(err, ErrAsOfModeUnsupported)
}
