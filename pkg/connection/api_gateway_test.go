package connection

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
)

// mockAPIError implements smithy.APIError interface
type mockAPIError struct {
	code    string
	message string
	fault   smithy.ErrorFault
}

func (e *mockAPIError) Error() string {
	return e.message
}

func (e *mockAPIError) ErrorCode() string {
	return e.code
}

func (e *mockAPIError) ErrorMessage() string {
	return e.message
}

func (e *mockAPIError) ErrorFault() smithy.ErrorFault {
	return e.fault
}

// Helper function to create mock AWS SDK v2 errors
func createGenericAPIError(code, message string) error {
	return &smithy.GenericAPIError{
		Code:    code,
		Message: message,
	}
}

// TestIsGoneError tests the isGoneError function
func TestIsGoneError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "GoneException returns true",
			err:      &types.GoneException{Message: aws.String("Connection gone")},
			expected: true,
		},
		{
			name:     "APIError with GoneException code returns true",
			err:      createGenericAPIError("GoneException", "Connection is gone"),
			expected: true,
		},
		{
			name:     "APIError with 410 in message returns true",
			err:      createGenericAPIError("UnknownError", "Status: 410 Gone"),
			expected: true,
		},
		{
			name:     "APIError with Gone in message returns true",
			err:      createGenericAPIError("UnknownError", "Connection Gone"),
			expected: true,
		},
		{
			name:     "Other error returns false",
			err:      errors.New("random error"),
			expected: false,
		},
		{
			name:     "ForbiddenException returns false",
			err:      &types.ForbiddenException{Message: aws.String("Forbidden")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGoneError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsForbiddenError tests the isForbiddenError function
func TestIsForbiddenError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "ForbiddenException returns true",
			err:      &types.ForbiddenException{Message: aws.String("Access denied")},
			expected: true,
		},
		{
			name:     "APIError with ForbiddenException code returns true",
			err:      createGenericAPIError("ForbiddenException", "Forbidden access"),
			expected: true,
		},
		{
			name:     "Other error returns false",
			err:      errors.New("random error"),
			expected: false,
		},
		{
			name:     "GoneException returns false",
			err:      &types.GoneException{Message: aws.String("Gone")},
			expected: false,
		},
		{
			name:     "APIError with different code returns false",
			err:      createGenericAPIError("UnauthorizedException", "Not authorized"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isForbiddenError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsPayloadTooLargeError tests the isPayloadTooLargeError function
func TestIsPayloadTooLargeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error returns false",
			err:      nil,
			expected: false,
		},
		{
			name:     "PayloadTooLargeException returns true",
			err:      &types.PayloadTooLargeException{Message: aws.String("Payload too large")},
			expected: true,
		},
		{
			name:     "APIError with PayloadTooLargeException code returns true",
			err:      createGenericAPIError("PayloadTooLargeException", "Message too big"),
			expected: true,
		},
		{
			name:     "Other error returns false",
			err:      errors.New("random error"),
			expected: false,
		},
		{
			name:     "GoneException returns false",
			err:      &types.GoneException{Message: aws.String("Gone")},
			expected: false,
		},
		{
			name:     "ForbiddenException returns false",
			err:      &types.ForbiddenException{Message: aws.String("Forbidden")},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPayloadTooLargeError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestErrorDetectionWithWrappedErrors tests error detection with wrapped errors
func TestErrorDetectionWithWrappedErrors(t *testing.T) {
	// Test Gone error wrapped in multiple layers
	goneErr := &types.GoneException{Message: aws.String("Connection gone")}
	wrappedOnce := errors.New("wrapped: " + goneErr.Error())

	// Direct type check should work
	assert.True(t, isGoneError(goneErr))
	// Wrapped error without errors.As won't work
	assert.False(t, isGoneError(wrappedOnce))

	// Test with smithy.GenericAPIError
	smithyErr := &smithy.GenericAPIError{
		Code:    "GoneException",
		Message: "Connection is gone",
	}
	assert.True(t, isGoneError(smithyErr))

	// Test forbidden error
	forbiddenErr := &types.ForbiddenException{Message: aws.String("Forbidden")}
	assert.True(t, isForbiddenError(forbiddenErr))

	// Test payload too large error
	payloadErr := &types.PayloadTooLargeException{Message: aws.String("Too large")}
	assert.True(t, isPayloadTooLargeError(payloadErr))
}

// TestAPIErrorCombinations tests various combinations of API errors
func TestAPIErrorCombinations(t *testing.T) {
	tests := []struct {
		name              string
		err               error
		isGone            bool
		isForbidden       bool
		isPayloadTooLarge bool
	}{
		{
			name: "Gone error with various messages",
			err: &mockAPIError{
				code:    "SomeOtherCode",
				message: "The connection is Gone and cannot be restored",
			},
			isGone:            true,
			isForbidden:       false,
			isPayloadTooLarge: false,
		},
		{
			name: "410 status in message",
			err: &mockAPIError{
				code:    "HTTPError",
				message: "HTTP 410 Gone: Connection not found",
			},
			isGone:            true,
			isForbidden:       false,
			isPayloadTooLarge: false,
		},
		{
			name: "Multiple error types shouldn't match",
			err: &mockAPIError{
				code:    "ForbiddenException",
				message: "Access forbidden",
			},
			isGone:            false,
			isForbidden:       true,
			isPayloadTooLarge: false,
		},
		{
			name: "Case sensitivity test",
			err: &mockAPIError{
				code:    "goneexception", // lowercase
				message: "gone",
			},
			isGone:            false, // Should be case sensitive for error codes
			isForbidden:       false,
			isPayloadTooLarge: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isGone, isGoneError(tt.err), "isGoneError mismatch")
			assert.Equal(t, tt.isForbidden, isForbiddenError(tt.err), "isForbiddenError mismatch")
			assert.Equal(t, tt.isPayloadTooLarge, isPayloadTooLargeError(tt.err), "isPayloadTooLargeError mismatch")
		})
	}
}

// Test multiple error types in one function to ensure they work together correctly
func TestErrorHelpersCombined(t *testing.T) {
	// Create different error types
	goneErr := &types.GoneException{Message: aws.String("Gone")}
	forbiddenErr := &types.ForbiddenException{Message: aws.String("Forbidden")}
	payloadErr := &types.PayloadTooLargeException{Message: aws.String("Too large")}

	// Test that each error is only recognized by its specific helper
	t.Run("GoneException only recognized by isGoneError", func(t *testing.T) {
		assert.True(t, isGoneError(goneErr))
		assert.False(t, isForbiddenError(goneErr))
		assert.False(t, isPayloadTooLargeError(goneErr))
	})

	t.Run("ForbiddenException only recognized by isForbiddenError", func(t *testing.T) {
		assert.False(t, isGoneError(forbiddenErr))
		assert.True(t, isForbiddenError(forbiddenErr))
		assert.False(t, isPayloadTooLargeError(forbiddenErr))
	})

	t.Run("PayloadTooLargeException only recognized by isPayloadTooLargeError", func(t *testing.T) {
		assert.False(t, isGoneError(payloadErr))
		assert.False(t, isForbiddenError(payloadErr))
		assert.True(t, isPayloadTooLargeError(payloadErr))
	})
}

// Benchmark tests to ensure performance
func BenchmarkIsGoneError(b *testing.B) {
	err := &types.GoneException{Message: aws.String("Gone")}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isGoneError(err)
	}
}

func BenchmarkIsForbiddenError(b *testing.B) {
	err := &types.ForbiddenException{Message: aws.String("Forbidden")}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isForbiddenError(err)
	}
}

func BenchmarkIsPayloadTooLargeError(b *testing.B) {
	err := &types.PayloadTooLargeException{Message: aws.String("Too large")}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isPayloadTooLargeError(err)
	}
}
