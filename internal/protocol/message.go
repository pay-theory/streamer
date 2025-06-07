package protocol

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	// Incoming message types
	MessageTypeRequest = "request"

	// Outgoing message types
	MessageTypeResponse       = "response"
	MessageTypeAcknowledgment = "acknowledgment"
	MessageTypeProgress       = "progress"
	MessageTypeError          = "error"
)

// IncomingMessage represents a message received from a WebSocket client
type IncomingMessage struct {
	Type     MessageType            `json:"type"`
	ID       string                 `json:"id,omitempty"`
	Action   string                 `json:"action"`
	Payload  json.RawMessage        `json:"payload,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// OutgoingMessage represents a message sent to a WebSocket client
type OutgoingMessage struct {
	Type      MessageType `json:"type"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ResponseMessage represents a response to a request
type ResponseMessage struct {
	OutgoingMessage
	Success  bool                   `json:"success"`
	Data     interface{}            `json:"data,omitempty"`
	Error    *ErrorData             `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AcknowledgmentMessage represents an acknowledgment of an async request
type AcknowledgmentMessage struct {
	OutgoingMessage
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ProgressMessage represents a progress update for an async operation
type ProgressMessage struct {
	OutgoingMessage
	Percentage float64                `json:"percentage"`
	Message    string                 `json:"message"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorMessage represents an error message
type ErrorMessage struct {
	OutgoingMessage
	Error *ErrorData `json:"error"`
}

// ErrorData contains error details
type ErrorData struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewResponseMessage creates a new response message
func NewResponseMessage(requestID string, success bool, data interface{}, err *ErrorData) *ResponseMessage {
	return &ResponseMessage{
		OutgoingMessage: OutgoingMessage{
			Type:      MessageTypeResponse,
			RequestID: requestID,
			Timestamp: time.Now(),
		},
		Success: success,
		Data:    data,
		Error:   err,
	}
}

// NewAcknowledgmentMessage creates a new acknowledgment message
func NewAcknowledgmentMessage(requestID, status, message string) *AcknowledgmentMessage {
	return &AcknowledgmentMessage{
		OutgoingMessage: OutgoingMessage{
			Type:      MessageTypeAcknowledgment,
			RequestID: requestID,
			Timestamp: time.Now(),
		},
		Status:  status,
		Message: message,
	}
}

// NewProgressMessage creates a new progress message
func NewProgressMessage(requestID string, percentage float64, message string) *ProgressMessage {
	return &ProgressMessage{
		OutgoingMessage: OutgoingMessage{
			Type:      MessageTypeProgress,
			RequestID: requestID,
			Timestamp: time.Now(),
		},
		Percentage: percentage,
		Message:    message,
	}
}

// NewErrorMessage creates a new error message
func NewErrorMessage(err *ErrorData) *ErrorMessage {
	return &ErrorMessage{
		OutgoingMessage: OutgoingMessage{
			Type:      MessageTypeError,
			Timestamp: time.Now(),
		},
		Error: err,
	}
}
