package streamer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	err := NewError(ErrCodeValidation, "validation failed")

	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "validation failed", err.Message)
	assert.NotNil(t, err.Details)
	assert.Empty(t, err.Details)
}

func TestError_WithDetail(t *testing.T) {
	err := NewError(ErrCodeInternalError, "internal error")

	// Add single detail
	err = err.WithDetail("key1", "value1")
	assert.Equal(t, "value1", err.Details["key1"])

	// Add multiple details
	err = err.WithDetail("key2", 42)
	err = err.WithDetail("key3", true)

	assert.Len(t, err.Details, 3)
	assert.Equal(t, "value1", err.Details["key1"])
	assert.Equal(t, 42, err.Details["key2"])
	assert.Equal(t, true, err.Details["key3"])

	// Overwrite existing detail
	err = err.WithDetail("key1", "new value")
	assert.Equal(t, "new value", err.Details["key1"])
}

func TestError_WithDetail_NilDetails(t *testing.T) {
	// Create error with nil details map
	err := &Error{
		Code:    ErrCodeNotFound,
		Message: "not found",
		Details: nil,
	}

	// WithDetail should initialize the map
	err = err.WithDetail("key", "value")
	assert.NotNil(t, err.Details)
	assert.Equal(t, "value", err.Details["key"])
}

func TestError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "simple error",
			err: &Error{
				Code:    ErrCodeValidation,
				Message: "validation failed",
			},
			want: "validation failed",
		},
		{
			name: "error with details",
			err: &Error{
				Code:    ErrCodeInternalError,
				Message: "internal server error",
				Details: map[string]interface{}{
					"request_id": "123",
					"timestamp":  "2023-01-01",
				},
			},
			want: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestErrorCodes(t *testing.T) {
	// Test that all error codes are defined correctly
	assert.Equal(t, "VALIDATION_ERROR", ErrCodeValidation)
	assert.Equal(t, "NOT_FOUND", ErrCodeNotFound)
	assert.Equal(t, "UNAUTHORIZED", ErrCodeUnauthorized)
	assert.Equal(t, "INTERNAL_ERROR", ErrCodeInternalError)
	assert.Equal(t, "TIMEOUT", ErrCodeTimeout)
	assert.Equal(t, "RATE_LIMITED", ErrCodeRateLimited)
	assert.Equal(t, "INVALID_ACTION", ErrCodeInvalidAction)
}

func TestError_ChainedOperations(t *testing.T) {
	// Test chaining WithDetail calls
	err := NewError(ErrCodeValidation, "validation error").
		WithDetail("field", "email").
		WithDetail("reason", "invalid format").
		WithDetail("example", "user@example.com")

	assert.Equal(t, ErrCodeValidation, err.Code)
	assert.Equal(t, "validation error", err.Message)
	assert.Len(t, err.Details, 3)
	assert.Equal(t, "email", err.Details["field"])
	assert.Equal(t, "invalid format", err.Details["reason"])
	assert.Equal(t, "user@example.com", err.Details["example"])
}

func TestError_ComplexDetails(t *testing.T) {
	// Test with complex detail values
	err := NewError(ErrCodeInternalError, "complex error")

	// Add different types of details
	err = err.WithDetail("string", "value")
	err = err.WithDetail("number", 123)
	err = err.WithDetail("float", 3.14)
	err = err.WithDetail("bool", false)
	err = err.WithDetail("slice", []string{"a", "b", "c"})
	err = err.WithDetail("map", map[string]int{"x": 1, "y": 2})
	err = err.WithDetail("nil", nil)

	assert.Len(t, err.Details, 7)
	assert.Equal(t, "value", err.Details["string"])
	assert.Equal(t, 123, err.Details["number"])
	assert.Equal(t, 3.14, err.Details["float"])
	assert.Equal(t, false, err.Details["bool"])
	assert.Equal(t, []string{"a", "b", "c"}, err.Details["slice"])
	assert.Equal(t, map[string]int{"x": 1, "y": 2}, err.Details["map"])
	assert.Nil(t, err.Details["nil"])
}
