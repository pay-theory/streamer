package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageTypes(t *testing.T) {
	// Test all message type constants
	assert.Equal(t, MessageType("request"), MessageTypeRequest)
	assert.Equal(t, MessageType("response"), MessageTypeResponse)
	assert.Equal(t, MessageType("acknowledgment"), MessageTypeAcknowledgment)
	assert.Equal(t, MessageType("progress"), MessageTypeProgress)
	assert.Equal(t, MessageType("error"), MessageTypeError)
	assert.Equal(t, MessageType("ping"), MessageTypePing)
	assert.Equal(t, MessageType("pong"), MessageTypePong)
}

func TestErrorCodes(t *testing.T) {
	// Test client error codes
	clientErrors := []string{
		ErrorCodeValidation,
		ErrorCodeInvalidAction,
		ErrorCodeNotFound,
		ErrorCodeUnauthorized,
		ErrorCodeForbidden,
		ErrorCodeRateLimited,
		ErrorCodeDuplicateRequest,
	}

	for _, code := range clientErrors {
		assert.NotEmpty(t, code)
		assert.True(t, IsClientError(code), "Expected %s to be a client error", code)
		assert.False(t, IsServerError(code), "Expected %s not to be a server error", code)
	}

	// Test server error codes
	serverErrors := []string{
		ErrorCodeInternal,
		ErrorCodeTimeout,
		ErrorCodeServiceUnavailable,
		ErrorCodeStorageError,
		ErrorCodeProcessingFailed,
	}

	for _, code := range serverErrors {
		assert.NotEmpty(t, code)
		assert.True(t, IsServerError(code), "Expected %s to be a server error", code)
		assert.False(t, IsClientError(code), "Expected %s not to be a client error", code)
	}

	// Test connection error codes
	connectionErrors := []string{
		ErrorCodeConnectionClosed,
		ErrorCodeInvalidMessage,
		ErrorCodeProtocolError,
	}

	for _, code := range connectionErrors {
		assert.NotEmpty(t, code)
		assert.False(t, IsClientError(code), "Expected %s not to be a client error", code)
		assert.False(t, IsServerError(code), "Expected %s not to be a server error", code)
	}
}

func TestNewMessage(t *testing.T) {
	before := time.Now().Unix()
	msg := NewMessage(MessageTypeRequest)
	after := time.Now().Unix()

	assert.Equal(t, MessageTypeRequest, msg.Type)
	assert.GreaterOrEqual(t, msg.Timestamp, before)
	assert.LessOrEqual(t, msg.Timestamp, after)
	assert.NotNil(t, msg.Metadata)
	assert.Empty(t, msg.Metadata)
	assert.Empty(t, msg.ID)
}

func TestNewRequestMessage(t *testing.T) {
	tests := []struct {
		name    string
		action  string
		payload map[string]interface{}
	}{
		{
			name:   "simple request",
			action: "test_action",
			payload: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name:    "request without payload",
			action:  "empty_action",
			payload: nil,
		},
		{
			name:   "complex payload",
			action: "complex_action",
			payload: map[string]interface{}{
				"string": "value",
				"number": 42,
				"nested": map[string]interface{}{
					"inner": "data",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewRequestMessage(tt.action, tt.payload)

			assert.Equal(t, MessageTypeRequest, msg.Type)
			assert.Equal(t, tt.action, msg.Action)
			assert.Equal(t, tt.payload, msg.Payload)
			assert.NotZero(t, msg.Timestamp)
			assert.NotNil(t, msg.Metadata)
		})
	}
}

func TestNewResponseMessage(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		success   bool
		data      interface{}
	}{
		{
			name:      "successful response",
			requestID: "req-123",
			success:   true,
			data: map[string]interface{}{
				"result": "ok",
			},
		},
		{
			name:      "failed response",
			requestID: "req-456",
			success:   false,
			data:      nil,
		},
		{
			name:      "response with array data",
			requestID: "req-789",
			success:   true,
			data:      []string{"item1", "item2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewResponseMessage(tt.requestID, tt.success, tt.data)

			assert.Equal(t, MessageTypeResponse, msg.Type)
			assert.Equal(t, tt.requestID, msg.RequestID)
			assert.Equal(t, tt.success, msg.Success)
			assert.Equal(t, tt.data, msg.Data)
			assert.Nil(t, msg.Error)
			assert.NotZero(t, msg.Timestamp)
		})
	}
}

func TestNewAcknowledgmentMessage(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		status    string
		message   string
	}{
		{
			name:      "queued status",
			requestID: "req-123",
			status:    "queued",
			message:   "Request queued for processing",
		},
		{
			name:      "processing status",
			requestID: "req-456",
			status:    "processing",
			message:   "Request is being processed",
		},
		{
			name:      "empty message",
			requestID: "req-789",
			status:    "queued",
			message:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewAcknowledgmentMessage(tt.requestID, tt.status, tt.message)

			assert.Equal(t, MessageTypeAcknowledgment, msg.Type)
			assert.Equal(t, tt.requestID, msg.RequestID)
			assert.Equal(t, tt.status, msg.Status)
			assert.Equal(t, tt.message, msg.Text)
			assert.NotZero(t, msg.Timestamp)
		})
	}
}

func TestNewProgressMessage(t *testing.T) {
	tests := []struct {
		name       string
		requestID  string
		percentage float64
		message    string
	}{
		{
			name:       "0% progress",
			requestID:  "req-123",
			percentage: 0.0,
			message:    "Starting",
		},
		{
			name:       "50% progress",
			requestID:  "req-456",
			percentage: 50.0,
			message:    "Halfway done",
		},
		{
			name:       "100% progress",
			requestID:  "req-789",
			percentage: 100.0,
			message:    "Complete",
		},
		{
			name:       "fractional progress",
			requestID:  "req-999",
			percentage: 33.33,
			message:    "One third complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewProgressMessage(tt.requestID, tt.percentage, tt.message)

			assert.Equal(t, MessageTypeProgress, msg.Type)
			assert.Equal(t, tt.requestID, msg.RequestID)
			assert.Equal(t, tt.percentage, msg.Percentage)
			assert.Equal(t, tt.message, msg.Text)
			assert.NotNil(t, msg.Details)
			assert.Empty(t, msg.Details)
			assert.NotZero(t, msg.Timestamp)
		})
	}
}

func TestNewErrorMessage(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		errorInfo *ErrorInfo
	}{
		{
			name:      "simple error",
			requestID: "req-123",
			errorInfo: NewErrorInfo(ErrorCodeValidation, "Invalid input"),
		},
		{
			name:      "error without request ID",
			requestID: "",
			errorInfo: NewErrorInfo(ErrorCodeInternal, "Server error"),
		},
		{
			name:      "error with details",
			requestID: "req-456",
			errorInfo: NewErrorInfo(ErrorCodeNotFound, "Resource not found").
				WithDetail("resource_id", "123").
				WithDetail("resource_type", "user"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewErrorMessage(tt.requestID, tt.errorInfo)

			assert.Equal(t, MessageTypeError, msg.Type)
			assert.Equal(t, tt.requestID, msg.RequestID)
			assert.Equal(t, tt.errorInfo, msg.Error)
			assert.NotZero(t, msg.Timestamp)
		})
	}
}

func TestNewErrorInfo(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
	}{
		{
			name:    "validation error",
			code:    ErrorCodeValidation,
			message: "Field 'email' is required",
		},
		{
			name:    "internal error",
			code:    ErrorCodeInternal,
			message: "Database connection failed",
		},
		{
			name:    "custom error",
			code:    "CUSTOM_ERROR",
			message: "Custom error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := NewErrorInfo(tt.code, tt.message)

			assert.Equal(t, tt.code, info.Code)
			assert.Equal(t, tt.message, info.Message)
			assert.NotNil(t, info.Details)
			assert.Empty(t, info.Details)
			assert.Nil(t, info.Retry)
		})
	}
}

func TestErrorInfo_WithDetail(t *testing.T) {
	info := NewErrorInfo(ErrorCodeValidation, "Validation failed")

	// Add single detail
	info.WithDetail("field", "email")
	assert.Equal(t, "email", info.Details["field"])

	// Add multiple details
	info.WithDetail("error", "invalid format").
		WithDetail("expected", "email@example.com").
		WithDetail("received", "not-an-email")

	assert.Len(t, info.Details, 4)
	assert.Equal(t, "invalid format", info.Details["error"])
	assert.Equal(t, "email@example.com", info.Details["expected"])
	assert.Equal(t, "not-an-email", info.Details["received"])

	// Test with nil details map (should initialize)
	info2 := &ErrorInfo{
		Code:    ErrorCodeInternal,
		Message: "Error",
		Details: nil,
	}

	info2.WithDetail("key", "value")
	assert.NotNil(t, info2.Details)
	assert.Equal(t, "value", info2.Details["key"])
}

func TestErrorInfo_WithRetry(t *testing.T) {
	tests := []struct {
		name      string
		retryable bool
		after     time.Time
		maxTries  int
	}{
		{
			name:      "retryable with delay",
			retryable: true,
			after:     time.Now().Add(5 * time.Second),
			maxTries:  3,
		},
		{
			name:      "not retryable",
			retryable: false,
			after:     time.Time{},
			maxTries:  0,
		},
		{
			name:      "retryable immediately",
			retryable: true,
			after:     time.Now(),
			maxTries:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := NewErrorInfo(ErrorCodeRateLimited, "Rate limit exceeded")
			info.WithRetry(tt.retryable, tt.after, tt.maxTries)

			require.NotNil(t, info.Retry)
			assert.Equal(t, tt.retryable, info.Retry.Retryable)
			assert.Equal(t, tt.after.Unix(), info.Retry.After.Unix())
			assert.Equal(t, tt.maxTries, info.Retry.MaxTries)
		})
	}
}

func TestIsClientError(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{ErrorCodeValidation, true},
		{ErrorCodeInvalidAction, true},
		{ErrorCodeNotFound, true},
		{ErrorCodeUnauthorized, true},
		{ErrorCodeForbidden, true},
		{ErrorCodeRateLimited, true},
		{ErrorCodeDuplicateRequest, true},
		{ErrorCodeInternal, false},
		{ErrorCodeTimeout, false},
		{ErrorCodeConnectionClosed, false},
		{"CUSTOM_ERROR", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := IsClientError(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsServerError(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{ErrorCodeInternal, true},
		{ErrorCodeTimeout, true},
		{ErrorCodeServiceUnavailable, true},
		{ErrorCodeStorageError, true},
		{ErrorCodeProcessingFailed, true},
		{ErrorCodeValidation, false},
		{ErrorCodeNotFound, false},
		{ErrorCodeConnectionClosed, false},
		{"CUSTOM_ERROR", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := IsServerError(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{ErrorCodeTimeout, true},
		{ErrorCodeServiceUnavailable, true},
		{ErrorCodeRateLimited, true},
		{ErrorCodeInternal, false},
		{ErrorCodeValidation, false},
		{ErrorCodeNotFound, false},
		{ErrorCodeForbidden, false},
		{"CUSTOM_ERROR", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := IsRetryableError(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessageChaining(t *testing.T) {
	// Test that methods can be chained
	info := NewErrorInfo(ErrorCodeValidation, "Multiple validation errors").
		WithDetail("field1", "email").
		WithDetail("field2", "password").
		WithRetry(false, time.Time{}, 0)

	assert.Equal(t, ErrorCodeValidation, info.Code)
	assert.Len(t, info.Details, 2)
	assert.NotNil(t, info.Retry)
	assert.False(t, info.Retry.Retryable)
}

func TestComplexMessageScenario(t *testing.T) {
	// Test a complex scenario with nested data
	payload := map[string]interface{}{
		"user_id": "user-123",
		"action":  "update_profile",
		"data": map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"preferences": map[string]interface{}{
				"notifications": true,
				"theme":         "dark",
			},
		},
	}

	// Create request
	req := NewRequestMessage("update_user", payload)
	assert.NotNil(t, req)

	// Create acknowledgment
	ack := NewAcknowledgmentMessage(req.ID, "queued", "Update request queued")
	assert.NotNil(t, ack)

	// Create progress updates
	progress1 := NewProgressMessage(req.ID, 25.0, "Validating data")
	progress2 := NewProgressMessage(req.ID, 50.0, "Updating database")
	progress3 := NewProgressMessage(req.ID, 75.0, "Syncing cache")

	assert.Equal(t, 25.0, progress1.Percentage)
	assert.Equal(t, 50.0, progress2.Percentage)
	assert.Equal(t, 75.0, progress3.Percentage)

	// Create final response
	response := NewResponseMessage(req.ID, true, map[string]interface{}{
		"updated_at": time.Now().Unix(),
		"version":    2,
	})
	assert.True(t, response.Success)
	assert.Nil(t, response.Error)
}

func TestErrorScenarios(t *testing.T) {
	// Test various error scenarios

	// Client error with retry info
	rateLimitError := NewErrorInfo(ErrorCodeRateLimited, "Too many requests").
		WithDetail("limit", 100).
		WithDetail("window", "1m").
		WithRetry(true, time.Now().Add(30*time.Second), 3)

	assert.True(t, IsClientError(rateLimitError.Code))
	assert.True(t, IsRetryableError(rateLimitError.Code))
	assert.True(t, rateLimitError.Retry.Retryable)

	// Server error without retry
	dbError := NewErrorInfo(ErrorCodeStorageError, "Database connection lost").
		WithDetail("db_host", "db.example.com").
		WithDetail("error_code", "CONNECTION_TIMEOUT")

	assert.True(t, IsServerError(dbError.Code))
	assert.False(t, IsRetryableError(dbError.Code))
	assert.Nil(t, dbError.Retry)

	// Validation error with field details
	validationError := NewErrorInfo(ErrorCodeValidation, "Input validation failed").
		WithDetail("fields", map[string]interface{}{
			"email": "Invalid email format",
			"age":   "Must be positive number",
		})

	assert.True(t, IsClientError(validationError.Code))
	assert.False(t, IsRetryableError(validationError.Code))
	fields, ok := validationError.Details["fields"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, fields, 2)
}
