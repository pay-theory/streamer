package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMessageTypes tests the message type constants
func TestMessageTypes(t *testing.T) {
	assert.Equal(t, "request", string(MessageTypeRequest))
	assert.Equal(t, "response", string(MessageTypeResponse))
	assert.Equal(t, "acknowledgment", string(MessageTypeAcknowledgment))
	assert.Equal(t, "progress", string(MessageTypeProgress))
	assert.Equal(t, "error", string(MessageTypeError))
}

// TestNewResponseMessage tests the NewResponseMessage constructor
func TestNewResponseMessage(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		success   bool
		data      interface{}
		err       *ErrorData
	}{
		{
			name:      "successful response with data",
			requestID: "req123",
			success:   true,
			data: map[string]string{
				"result": "success",
				"id":     "12345",
			},
			err: nil,
		},
		{
			name:      "error response",
			requestID: "req456",
			success:   false,
			data:      nil,
			err: &ErrorData{
				Code:    "VALIDATION_ERROR",
				Message: "Invalid input",
				Details: map[string]interface{}{
					"field": "email",
				},
			},
		},
		{
			name:      "success with no data",
			requestID: "req789",
			success:   true,
			data:      nil,
			err:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			msg := NewResponseMessage(tt.requestID, tt.success, tt.data, tt.err)
			after := time.Now()

			assert.Equal(t, string(MessageTypeResponse), string(msg.Type))
			assert.Equal(t, tt.requestID, msg.RequestID)
			assert.Equal(t, tt.success, msg.Success)
			assert.Equal(t, tt.data, msg.Data)
			assert.Equal(t, tt.err, msg.Error)

			// Verify timestamp is within reasonable bounds
			assert.True(t, msg.Timestamp.After(before) || msg.Timestamp.Equal(before))
			assert.True(t, msg.Timestamp.Before(after) || msg.Timestamp.Equal(after))
		})
	}
}

// TestNewAcknowledgmentMessage tests the NewAcknowledgmentMessage constructor
func TestNewAcknowledgmentMessage(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		status    string
		message   string
	}{
		{
			name:      "processing acknowledgment",
			requestID: "req123",
			status:    "processing",
			message:   "Request is being processed",
		},
		{
			name:      "queued acknowledgment",
			requestID: "req456",
			status:    "queued",
			message:   "Request has been queued",
		},
		{
			name:      "received acknowledgment",
			requestID: "req789",
			status:    "received",
			message:   "Request received successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			msg := NewAcknowledgmentMessage(tt.requestID, tt.status, tt.message)
			after := time.Now()

			assert.Equal(t, string(MessageTypeAcknowledgment), string(msg.Type))
			assert.Equal(t, tt.requestID, msg.RequestID)
			assert.Equal(t, tt.status, msg.Status)
			assert.Equal(t, tt.message, msg.Message)

			// Verify timestamp
			assert.True(t, msg.Timestamp.After(before) || msg.Timestamp.Equal(before))
			assert.True(t, msg.Timestamp.Before(after) || msg.Timestamp.Equal(after))
		})
	}
}

// TestNewProgressMessage tests the NewProgressMessage constructor
func TestNewProgressMessage(t *testing.T) {
	tests := []struct {
		name       string
		requestID  string
		percentage float64
		message    string
	}{
		{
			name:       "0% progress",
			requestID:  "req123",
			percentage: 0,
			message:    "Starting operation",
		},
		{
			name:       "50% progress",
			requestID:  "req456",
			percentage: 50,
			message:    "Halfway complete",
		},
		{
			name:       "100% progress",
			requestID:  "req789",
			percentage: 100,
			message:    "Operation complete",
		},
		{
			name:       "fractional progress",
			requestID:  "req999",
			percentage: 33.33,
			message:    "Processing item 1 of 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			msg := NewProgressMessage(tt.requestID, tt.percentage, tt.message)
			after := time.Now()

			assert.Equal(t, string(MessageTypeProgress), string(msg.Type))
			assert.Equal(t, tt.requestID, msg.RequestID)
			assert.Equal(t, tt.percentage, msg.Percentage)
			assert.Equal(t, tt.message, msg.Message)

			// Verify timestamp
			assert.True(t, msg.Timestamp.After(before) || msg.Timestamp.Equal(before))
			assert.True(t, msg.Timestamp.Before(after) || msg.Timestamp.Equal(after))
		})
	}
}

// TestNewErrorMessage tests the NewErrorMessage constructor
func TestNewErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *ErrorData
	}{
		{
			name: "validation error",
			err: &ErrorData{
				Code:    "VALIDATION_ERROR",
				Message: "Invalid input data",
				Details: map[string]interface{}{
					"fields": []string{"email", "password"},
				},
			},
		},
		{
			name: "authentication error",
			err: &ErrorData{
				Code:    "AUTH_ERROR",
				Message: "Authentication failed",
				Details: nil,
			},
		},
		{
			name: "internal error with details",
			err: &ErrorData{
				Code:    "INTERNAL_ERROR",
				Message: "An internal error occurred",
				Details: map[string]interface{}{
					"trace_id": "xyz123",
					"retry":    true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			msg := NewErrorMessage(tt.err)
			after := time.Now()

			assert.Equal(t, string(MessageTypeError), string(msg.Type))
			assert.Equal(t, "", msg.RequestID) // Error messages don't have request IDs
			assert.Equal(t, tt.err, msg.Error)

			// Verify timestamp
			assert.True(t, msg.Timestamp.After(before) || msg.Timestamp.Equal(before))
			assert.True(t, msg.Timestamp.Before(after) || msg.Timestamp.Equal(after))
		})
	}
}

// TestIncomingMessageJSON tests JSON marshaling/unmarshaling of IncomingMessage
func TestIncomingMessageJSON(t *testing.T) {
	tests := []struct {
		name string
		msg  IncomingMessage
		want string
	}{
		{
			name: "complete message",
			msg: IncomingMessage{
				Type:    MessageTypeRequest,
				ID:      "msg123",
				Action:  "user.create",
				Payload: json.RawMessage(`{"name":"John","email":"john@example.com"}`),
				Metadata: map[string]interface{}{
					"source":  "web",
					"version": "1.0",
				},
			},
			want: `{"type":"request","id":"msg123","action":"user.create","payload":{"name":"John","email":"john@example.com"},"metadata":{"source":"web","version":"1.0"}}`,
		},
		{
			name: "minimal message",
			msg: IncomingMessage{
				Type:   MessageTypeRequest,
				Action: "ping",
			},
			want: `{"type":"request","action":"ping"}`,
		},
		{
			name: "message with empty fields",
			msg: IncomingMessage{
				Type:     MessageTypeRequest,
				ID:       "",
				Action:   "test",
				Payload:  nil,
				Metadata: nil,
			},
			want: `{"type":"request","action":"test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.msg)
			assert.NoError(t, err)

			// Compare JSON (ignoring field order)
			var got, want map[string]interface{}
			assert.NoError(t, json.Unmarshal(data, &got))
			assert.NoError(t, json.Unmarshal([]byte(tt.want), &want))
			assert.Equal(t, want, got)

			// Test unmarshaling
			var decoded IncomingMessage
			err = json.Unmarshal(data, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, tt.msg.Type, decoded.Type)
			assert.Equal(t, tt.msg.Action, decoded.Action)

			if tt.msg.ID != "" {
				assert.Equal(t, tt.msg.ID, decoded.ID)
			}
		})
	}
}

// TestOutgoingMessageJSON tests JSON marshaling of OutgoingMessage types
func TestOutgoingMessageJSON(t *testing.T) {
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	// Test ResponseMessage
	respMsg := &ResponseMessage{
		OutgoingMessage: OutgoingMessage{
			Type:      MessageTypeResponse,
			RequestID: "req123",
			Timestamp: fixedTime,
		},
		Success: true,
		Data: map[string]interface{}{
			"id":   "12345",
			"name": "Test",
		},
		Metadata: map[string]interface{}{
			"duration": 123,
		},
	}

	data, err := json.Marshal(respMsg)
	assert.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, "response", decoded["type"])
	assert.Equal(t, "req123", decoded["request_id"])
	assert.Equal(t, true, decoded["success"])
	assert.NotNil(t, decoded["data"])
	assert.NotNil(t, decoded["metadata"])
	assert.NotNil(t, decoded["timestamp"])
}

// TestProgressMessageWithMetadata tests ProgressMessage with metadata
func TestProgressMessageWithMetadata(t *testing.T) {
	msg := &ProgressMessage{
		OutgoingMessage: OutgoingMessage{
			Type:      MessageTypeProgress,
			RequestID: "req123",
			Timestamp: time.Now(),
		},
		Percentage: 75.5,
		Message:    "Processing records",
		Metadata: map[string]interface{}{
			"processed": float64(755),
			"total":     float64(1000),
			"current":   "batch-3",
		},
	}

	// Marshal and unmarshal
	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var decoded ProgressMessage
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, msg.Percentage, decoded.Percentage)
	assert.Equal(t, msg.Message, decoded.Message)
	assert.Equal(t, msg.Metadata, decoded.Metadata)
}

// TestErrorDataDetails tests ErrorData with various detail types
func TestErrorDataDetails(t *testing.T) {
	tests := []struct {
		name string
		err  ErrorData
	}{
		{
			name: "error with string details",
			err: ErrorData{
				Code:    "PARSE_ERROR",
				Message: "Failed to parse input",
				Details: map[string]interface{}{
					"line":   10,
					"column": 5,
					"token":  "unexpected",
				},
			},
		},
		{
			name: "error with nested details",
			err: ErrorData{
				Code:    "VALIDATION_ERROR",
				Message: "Multiple validation errors",
				Details: map[string]interface{}{
					"errors": []interface{}{
						map[string]interface{}{
							"field":   "email",
							"message": "invalid format",
						},
						map[string]interface{}{
							"field":   "age",
							"message": "must be positive",
						},
					},
				},
			},
		},
		{
			name: "error without details",
			err: ErrorData{
				Code:    "NOT_FOUND",
				Message: "Resource not found",
				Details: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal and unmarshal
			data, err := json.Marshal(tt.err)
			assert.NoError(t, err)

			var decoded ErrorData
			err = json.Unmarshal(data, &decoded)
			assert.NoError(t, err)

			assert.Equal(t, tt.err.Code, decoded.Code)
			assert.Equal(t, tt.err.Message, decoded.Message)

			if tt.err.Details != nil {
				assert.NotNil(t, decoded.Details)
				// Deep comparison of details
				expectedJSON, _ := json.Marshal(tt.err.Details)
				actualJSON, _ := json.Marshal(decoded.Details)
				assert.JSONEq(t, string(expectedJSON), string(actualJSON))
			} else {
				assert.Nil(t, decoded.Details)
			}
		})
	}
}
