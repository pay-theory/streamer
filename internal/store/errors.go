package store

import (
	"errors"
	"fmt"
)

// Common errors
var (
	// ErrNotFound is returned when an item is not found in DynamoDB
	ErrNotFound = errors.New("item not found")

	// ErrAlreadyExists is returned when trying to create an item that already exists
	ErrAlreadyExists = errors.New("item already exists")

	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")

	// ErrConnectionClosed is returned when operating on a closed connection
	ErrConnectionClosed = errors.New("connection is closed")

	// ErrRequestNotPending is returned when trying to process a non-pending request
	ErrRequestNotPending = errors.New("request is not in pending state")

	// ErrConcurrentModification is returned when an item was modified concurrently
	ErrConcurrentModification = errors.New("item was modified concurrently")
)

// StoreError wraps storage-related errors with additional context
type StoreError struct {
	Op      string // Operation that failed
	Table   string // DynamoDB table
	Key     string // Item key
	Err     error  // Underlying error
	Message string // Additional context
}

// Error implements the error interface
func (e *StoreError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s %s (table=%s, key=%s): %s - %v",
			e.Op, e.Message, e.Table, e.Key, e.Message, e.Err)
	}
	return fmt.Sprintf("%s failed (table=%s, key=%s): %v",
		e.Op, e.Table, e.Key, e.Err)
}

// Unwrap returns the underlying error
func (e *StoreError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target
func (e *StoreError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewStoreError creates a new storage error
func NewStoreError(op, table, key string, err error) *StoreError {
	return &StoreError{
		Op:    op,
		Table: table,
		Key:   key,
		Err:   err,
	}
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	var storeErr *StoreError
	if errors.As(err, &storeErr) {
		return errors.Is(storeErr.Err, ErrNotFound)
	}

	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is an already exists error
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}

	var storeErr *StoreError
	if errors.As(err, &storeErr) {
		return errors.Is(storeErr.Err, ErrAlreadyExists)
	}

	return errors.Is(err, ErrAlreadyExists)
}

// ValidationError represents input validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
