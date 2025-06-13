package connection

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConnectionError tests the ConnectionError type
func TestConnectionError(t *testing.T) {
	baseErr := errors.New("base error")
	connErr := &ConnectionError{
		ConnectionID: "conn123",
		Err:          baseErr,
	}

	t.Run("Error method", func(t *testing.T) {
		expected := "connection conn123: base error"
		assert.Equal(t, expected, connErr.Error())
	})

	t.Run("Unwrap method", func(t *testing.T) {
		unwrapped := connErr.Unwrap()
		assert.Equal(t, baseErr, unwrapped)
	})

	t.Run("errors.Is support", func(t *testing.T) {
		// Should be able to check for the wrapped error
		assert.True(t, errors.Is(connErr, baseErr))
	})

	t.Run("errors.As support", func(t *testing.T) {
		var ce *ConnectionError
		assert.True(t, errors.As(connErr, &ce))
		assert.Equal(t, "conn123", ce.ConnectionID)
	})
}

// TestConnectionGoneError tests the ConnectionGoneError type
func TestConnectionGoneError(t *testing.T) {
	err := &ConnectionGoneError{
		ConnectionID: "conn456",
	}

	t.Run("Error method", func(t *testing.T) {
		expected := "connection conn456 is no longer active"
		assert.Equal(t, expected, err.Error())
	})
}

// TestBroadcastError tests the BroadcastError type
func TestBroadcastError(t *testing.T) {
	broadcastErr := &BroadcastError{
		Failed: []string{"conn1", "conn2", "conn3"},
		Errors: []error{
			errors.New("error1"),
			errors.New("error2"),
			errors.New("error3"),
		},
	}

	t.Run("Error method", func(t *testing.T) {
		expected := "broadcast failed for 3 connections"
		assert.Equal(t, expected, broadcastErr.Error())
	})

	t.Run("empty broadcast error", func(t *testing.T) {
		emptyErr := &BroadcastError{
			Failed: []string{},
			Errors: []error{},
		}
		expected := "broadcast failed for 0 connections"
		assert.Equal(t, expected, emptyErr.Error())
	})
}

func TestIsConnectionGoneHelper(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ConnectionGoneError returns true",
			err:      &ConnectionGoneError{ConnectionID: "test"},
			expected: true,
		},
		{
			name:     "other error returns false",
			err:      errors.New("different error"),
			expected: false,
		},
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "wrapped ConnectionGoneError returns false",
			err:      fmt.Errorf("wrapped: %w", &ConnectionGoneError{ConnectionID: "test"}),
			expected: false, // This helper only checks direct type, not unwrapped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConnectionGone(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommonErrors(t *testing.T) {
	// Test that our common error variables are properly defined
	t.Run("ErrConnectionNotFound", func(t *testing.T) {
		assert.NotNil(t, ErrConnectionNotFound)
		assert.Equal(t, "connection not found", ErrConnectionNotFound.Error())
	})

	t.Run("ErrConnectionStale", func(t *testing.T) {
		assert.NotNil(t, ErrConnectionStale)
		assert.Equal(t, "connection is stale", ErrConnectionStale.Error())
	})

	t.Run("ErrInvalidMessage", func(t *testing.T) {
		assert.NotNil(t, ErrInvalidMessage)
		assert.Equal(t, "invalid message format", ErrInvalidMessage.Error())
	})

	t.Run("ErrBroadcastPartialFailure", func(t *testing.T) {
		assert.NotNil(t, ErrBroadcastPartialFailure)
		assert.Equal(t, "broadcast partially failed", ErrBroadcastPartialFailure.Error())
	})
}

// TestErrorWrapping tests error wrapping scenarios
func TestErrorWrapping(t *testing.T) {
	// Test various error wrapping scenarios
	t.Run("wrapping with ConnectionError", func(t *testing.T) {
		baseErr := ErrConnectionNotFound
		connErr := &ConnectionError{
			ConnectionID: "conn789",
			Err:          baseErr,
		}

		// Should be able to check for both types
		assert.True(t, errors.Is(connErr, ErrConnectionNotFound))

		// Should be able to extract ConnectionError
		var ce *ConnectionError
		assert.True(t, errors.As(connErr, &ce))
		assert.Equal(t, "conn789", ce.ConnectionID)
	})

	t.Run("multiple levels of wrapping", func(t *testing.T) {
		// Create a chain of wrapped errors
		level1 := ErrConnectionStale
		level2 := &ConnectionError{
			ConnectionID: "conn1",
			Err:          level1,
		}
		level3 := fmt.Errorf("operation failed: %w", level2)

		// Should be able to find the base error through multiple levels
		assert.True(t, errors.Is(level3, ErrConnectionStale))

		// Should be able to extract ConnectionError from wrapped error
		var ce *ConnectionError
		assert.True(t, errors.As(level3, &ce))
		assert.Equal(t, "conn1", ce.ConnectionID)
	})
}

// Benchmark error creation and checking
func BenchmarkConnectionError_Create(b *testing.B) {
	baseErr := errors.New("base error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &ConnectionError{
			ConnectionID: "conn123",
			Err:          baseErr,
		}
	}
}

func BenchmarkIsConnectionGone(b *testing.B) {
	err := &ConnectionGoneError{ConnectionID: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsConnectionGone(err)
	}
}

func BenchmarkErrorsIs(b *testing.B) {
	baseErr := ErrConnectionNotFound
	connErr := &ConnectionError{
		ConnectionID: "conn123",
		Err:          baseErr,
	}
	wrapped := fmt.Errorf("failed: %w", connErr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = errors.Is(wrapped, ErrConnectionNotFound)
	}
}
