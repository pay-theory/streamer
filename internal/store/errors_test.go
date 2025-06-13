package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStoreError tests the StoreError type
func TestStoreError(t *testing.T) {
	tests := []struct {
		name    string
		err     *StoreError
		wantMsg string
	}{
		{
			name: "error with message",
			err: &StoreError{
				Op:      "Get",
				Table:   "connections",
				Key:     "conn123",
				Err:     ErrNotFound,
				Message: "connection not found",
			},
			wantMsg: "Get connection not found (table=connections, key=conn123): connection not found - item not found",
		},
		{
			name: "error without message",
			err: &StoreError{
				Op:    "Save",
				Table: "connections",
				Key:   "conn456",
				Err:   ErrAlreadyExists,
			},
			wantMsg: "Save failed (table=connections, key=conn456): item already exists",
		},
		{
			name: "error with nil underlying error",
			err: &StoreError{
				Op:      "Delete",
				Table:   "requests",
				Key:     "req789",
				Message: "unexpected error",
			},
			wantMsg: "Delete unexpected error (table=requests, key=req789): unexpected error - <nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

// TestStoreError_Unwrap tests the Unwrap method
func TestStoreError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	storeErr := &StoreError{
		Op:  "Query",
		Err: originalErr,
	}

	unwrapped := storeErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)

	// Test with nil wrapped error
	storeErr2 := &StoreError{
		Op:  "Scan",
		Err: nil,
	}
	assert.Nil(t, storeErr2.Unwrap())
}

// TestStoreError_Is tests the Is method for error comparison
func TestStoreError_Is(t *testing.T) {
	storeErr1 := &StoreError{
		Op:  "Get",
		Err: ErrNotFound,
	}
	storeErr2 := &StoreError{
		Op:  "Query",
		Err: ErrAlreadyExists,
	}

	// Should match the underlying error
	assert.True(t, storeErr1.Is(ErrNotFound))
	assert.False(t, storeErr1.Is(ErrAlreadyExists))
	assert.False(t, storeErr2.Is(ErrNotFound))
	assert.True(t, storeErr2.Is(ErrAlreadyExists))

	// Test with wrapped errors
	wrappedErr := &StoreError{
		Op:  "Get",
		Err: storeErr1,
	}
	assert.True(t, wrappedErr.Is(ErrNotFound))
}

// TestNewStoreError tests the NewStoreError constructor
func TestNewStoreError(t *testing.T) {
	tests := []struct {
		name  string
		op    string
		table string
		key   string
		err   error
	}{
		{
			name:  "complete error",
			op:    "PutItem",
			table: "connections",
			key:   "conn123",
			err:   ErrNotFound,
		},
		{
			name:  "error with empty fields",
			op:    "",
			table: "",
			key:   "",
			err:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStoreError(tt.op, tt.table, tt.key, tt.err)
			assert.Equal(t, tt.op, got.Op)
			assert.Equal(t, tt.table, got.Table)
			assert.Equal(t, tt.key, got.Key)
			assert.Equal(t, tt.err, got.Err)
			assert.Empty(t, got.Message) // Constructor doesn't set message
		})
	}
}

// TestIsNotFound tests the IsNotFound helper function
func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct ErrNotFound",
			err:  ErrNotFound,
			want: true,
		},
		{
			name: "StoreError wrapping ErrNotFound",
			err: &StoreError{
				Op:  "Get",
				Err: ErrNotFound,
			},
			want: true,
		},
		{
			name: "nested StoreError with ErrNotFound",
			err: &StoreError{
				Op: "Batch",
				Err: &StoreError{
					Op:  "Get",
					Err: ErrNotFound,
				},
			},
			want: true,
		},
		{
			name: "different error",
			err:  ErrAlreadyExists,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error type",
			err:  errors.New("some error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFound(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsAlreadyExists tests the IsAlreadyExists helper function
func TestIsAlreadyExists(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct ErrAlreadyExists",
			err:  ErrAlreadyExists,
			want: true,
		},
		{
			name: "StoreError wrapping ErrAlreadyExists",
			err: &StoreError{
				Op:  "Save",
				Err: ErrAlreadyExists,
			},
			want: true,
		},
		{
			name: "different error",
			err:  ErrNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error type",
			err:  errors.New("some error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAlreadyExists(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestValidationError tests the ValidationError type
func TestValidationError(t *testing.T) {
	ve := &ValidationError{
		Field:   "connectionID",
		Message: "must not be empty",
	}

	expected := "validation error on field 'connectionID': must not be empty"
	assert.Equal(t, expected, ve.Error())

	// Test with empty field
	ve2 := &ValidationError{
		Field:   "",
		Message: "general validation error",
	}
	assert.Equal(t, "validation error on field '': general validation error", ve2.Error())
}

// TestNewValidationError tests the NewValidationError constructor
func TestNewValidationError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
	}{
		{
			name:    "normal validation error",
			field:   "userID",
			message: "invalid format",
		},
		{
			name:    "empty field name",
			field:   "",
			message: "missing required field",
		},
		{
			name:    "long message",
			field:   "payload",
			message: "payload size exceeds maximum allowed limit of 256KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.field, tt.message)
			ve, ok := err.(*ValidationError)
			assert.True(t, ok)
			assert.Equal(t, tt.field, ve.Field)
			assert.Equal(t, tt.message, ve.Message)
			assert.Contains(t, err.Error(), tt.field)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

// TestPredefinedErrors tests that predefined errors are properly initialized
func TestPredefinedErrors(t *testing.T) {
	// Test all predefined errors
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrNotFound",
			err:  ErrNotFound,
			msg:  "item not found",
		},
		{
			name: "ErrAlreadyExists",
			err:  ErrAlreadyExists,
			msg:  "item already exists",
		},
		{
			name: "ErrInvalidInput",
			err:  ErrInvalidInput,
			msg:  "invalid input",
		},
		{
			name: "ErrConnectionClosed",
			err:  ErrConnectionClosed,
			msg:  "connection is closed",
		},
		{
			name: "ErrRequestNotPending",
			err:  ErrRequestNotPending,
			msg:  "request is not in pending state",
		},
		{
			name: "ErrConcurrentModification",
			err:  ErrConcurrentModification,
			msg:  "item was modified concurrently",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.Equal(t, tt.msg, tt.err.Error())
		})
	}
}

// TestErrorWrapping tests error wrapping scenarios
func TestErrorWrapping(t *testing.T) {
	originalErr := errors.New("database connection failed")
	storeErr := &StoreError{
		Op:    "Save",
		Table: "connections",
		Key:   "conn123",
		Err:   originalErr,
	}

	// Test that errors.Is works with wrapped errors
	assert.True(t, errors.Is(storeErr, originalErr))

	// Test wrapping predefined errors
	notFoundErr := &StoreError{
		Op:  "Get",
		Err: ErrNotFound,
	}
	assert.True(t, errors.Is(notFoundErr, ErrNotFound))
	assert.True(t, IsNotFound(notFoundErr))
}

// TestComplexErrorScenarios tests complex error scenarios
func TestComplexErrorScenarios(t *testing.T) {
	// Test multiple levels of wrapping
	baseErr := ErrNotFound
	level1 := &StoreError{Op: "Get", Err: baseErr}
	level2 := &StoreError{Op: "BatchGet", Err: level1}
	level3 := fmt.Errorf("operation failed: %w", level2)

	assert.True(t, IsNotFound(level1))
	assert.True(t, IsNotFound(level2))
	assert.True(t, errors.Is(level3, ErrNotFound))

	// Test error with all fields populated
	fullErr := &StoreError{
		Op:      "UpdateItem",
		Table:   "connections",
		Key:     "user#123#conn#456",
		Err:     ErrConcurrentModification,
		Message: "version mismatch",
	}
	errStr := fullErr.Error()
	assert.Contains(t, errStr, "UpdateItem")
	assert.Contains(t, errStr, "connections")
	assert.Contains(t, errStr, "user#123#conn#456")
	assert.Contains(t, errStr, "version mismatch")
	assert.Contains(t, errStr, "item was modified concurrently")
}
