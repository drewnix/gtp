package errors

import (
	"fmt"

	"github.com/pkg/errors"
)

// Common error sentinel values
// These act as unique identifiers for specific error conditions
// Kubernetes uses similar sentinel errors in its codebase
var (
	// Resource-oriented errors (common in Kubernetes API errors)
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input")

	// Operation-oriented errors
	ErrTimeout      = errors.New("operation timed out")
	ErrCancelled    = errors.New("operation was cancelled")
	ErrUnavailable  = errors.New("service unavailable")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// TaskError is a domain-specific error type that provides context
// about which task encountered an error.
// This pattern is common in Kubernetes for resource-specific errors.
type TaskError struct {
	// TaskID identifies which task had the error
	TaskID string
	// Operation describes what operation was being performed
	Operation string
	// Err is the underlying error
	Err error
}

// Error implements the error interface with a formatted message.
// This formatting approach maintains consistency across error reporting.
func (e *TaskError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("task %s: %s: %v", e.TaskID, e.Operation, e.Err)
	}
	return fmt.Sprintf("task %s: %v", e.TaskID, e.Err)
}

// Unwrap returns the underlying error to support errors.Is and errors.As.
// This pattern allows error inspection without type assertions.
func (e *TaskError) Unwrap() error {
	return e.Err
}

// NewTaskError creates a new task error with an operation context.
func NewTaskError(taskID, operation string, err error) error {
	return &TaskError{
		TaskID:    taskID,
		Operation: operation,
		Err:       err,
	}
}

// Error inspection helpers
// These functions abstract the error inspection logic and
// make error checking more readable in client code.

// IsNotFound checks if an error is or wraps a "not found" error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is or wraps an "already exists" error.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsInvalidInput checks if an error is or wraps an "invalid input" error.
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsTimeout checks if an error is or wraps a "timeout" error.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsCancelled checks if an error is or wraps a "cancelled" error.
func IsCancelled(err error) bool {
	return errors.Is(err, ErrCancelled)
}

// IsUnavailable checks if an error is or wraps an "unavailable" error.
func IsUnavailable(err error) bool {
	return errors.Is(err, ErrUnavailable)
}

// GetTaskID extracts the task ID from a TaskError if present.
// This demonstrates using errors.As for type-specific error handling.
func GetTaskID(err error) (string, bool) {
	var taskErr *TaskError
	if errors.As(err, &taskErr) {
		return taskErr.TaskID, true
	}
	return "", false
}

// FormatErrorResponse formats an error for API responses.
// This creates consistent error responses similar to Kubernetes API errors.
func FormatErrorResponse(err error) map[string]interface{} {
	// Extract task ID if present
	taskID, hasTaskID := GetTaskID(err)

	// Create base response
	response := map[string]interface{}{
		"error": err.Error(),
	}

	// Add type-specific fields
	if hasTaskID {
		response["taskID"] = taskID
	}

	// Add specific error type
	if IsNotFound(err) {
		response["code"] = "not_found"
	} else if IsAlreadyExists(err) {
		response["code"] = "already_exists"
	} else if IsInvalidInput(err) {
		response["code"] = "invalid_input"
	} else if IsTimeout(err) {
		response["code"] = "timeout"
	} else if IsCancelled(err) {
		response["code"] = "cancelled"
	} else {
		response["code"] = "internal_error"
	}

	return response
}
