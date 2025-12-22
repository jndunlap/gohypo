package errors

import (
	"fmt"
)

// AppError represents a structured application error
type AppError struct {
	Code    string
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// New creates a new AppError
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return &AppError{
			Code:    appErr.Code,
			Message: message,
			Cause:   appErr,
		}
	}
	return &AppError{
		Code:    "INTERNAL_ERROR",
		Message: message,
		Cause:   err,
	}
}

// Wrapf wraps an error with formatted additional context
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return Wrap(err, fmt.Sprintf(format, args...))
}

// WithCode adds an error code to an existing error
func WithCode(code string, err error) error {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return &AppError{
			Code:    code,
			Message: appErr.Message,
			Cause:   appErr.Cause,
		}
	}
	return &AppError{
		Code:    code,
		Message: err.Error(),
		Cause:   err,
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetCode returns the error code if it's an AppError, otherwise returns "UNKNOWN"
func GetCode(err error) string {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return "UNKNOWN"
}

// Predefined error codes
const (
	CodeConfigInvalid    = "CONFIG_INVALID"
	CodeDatabaseError    = "DATABASE_ERROR"
	CodeValidationError  = "VALIDATION_ERROR"
	CodeNotFound         = "NOT_FOUND"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeInternalError    = "INTERNAL_ERROR"
	CodeExternalService  = "EXTERNAL_SERVICE_ERROR"
	CodeInvalidInput     = "INVALID_INPUT"
)

// Common error constructors
func ConfigInvalid(message string) *AppError {
	return New(CodeConfigInvalid, message)
}

func DatabaseError(message string) *AppError {
	return New(CodeDatabaseError, message)
}

func ValidationError(message string) *AppError {
	return New(CodeValidationError, message)
}

func NotFound(resource string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s not found", resource))
}

func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message)
}

func InternalError(message string) *AppError {
	return New(CodeInternalError, message)
}

func ExternalServiceError(service string, cause error) *AppError {
	return &AppError{
		Code:    CodeExternalService,
		Message: fmt.Sprintf("%s service error", service),
		Cause:   cause,
	}
}

func InvalidInput(message string) *AppError {
	return New(CodeInvalidInput, message)
}


