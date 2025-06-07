// Package types defines shared message types for WebSocket communication
package types

import (
	"time"
)

// MessageType defines the type of WebSocket message
type MessageType string

const (
	// Request message types
	MessageTypeRequest MessageType = "request"

	// Response message types
	MessageTypeResponse       MessageType = "response"
	MessageTypeAcknowledgment MessageType = "acknowledgment"
	MessageTypeProgress       MessageType = "progress"
	MessageTypeError          MessageType = "error"

	// Control message types
	MessageTypePing MessageType = "ping"
	MessageTypePong MessageType = "pong"
)

// Message is the base structure for all WebSocket messages
type Message struct {
	Type      MessageType            `json:"type"`
	ID        string                 `json:"id,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// RequestMessage represents an incoming WebSocket request
type RequestMessage struct {
	Message
	Action  string                 `json:"action"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// ResponseMessage represents a synchronous response
type ResponseMessage struct {
	Message
	RequestID string      `json:"request_id"`
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
}

// AcknowledgmentMessage is sent for async requests
type AcknowledgmentMessage struct {
	Message
	RequestID string `json:"request_id"`
	Status    string `json:"status"` // "queued", "processing"
	Text      string `json:"message,omitempty"`
}

// ProgressMessage represents async processing progress
type ProgressMessage struct {
	Message
	RequestID  string                 `json:"request_id"`
	Percentage float64                `json:"percentage"` // 0-100
	Text       string                 `json:"message,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// ErrorMessage represents an error response
type ErrorMessage struct {
	Message
	RequestID string     `json:"request_id,omitempty"`
	Error     *ErrorInfo `json:"error"`
}

// ErrorInfo contains structured error information
type ErrorInfo struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Retry   *RetryInfo             `json:"retry,omitempty"`
}

// RetryInfo provides retry guidance for errors
type RetryInfo struct {
	Retryable bool      `json:"retryable"`
	After     time.Time `json:"after,omitempty"`
	MaxTries  int       `json:"max_tries,omitempty"`
}

// Standard error codes
const (
	// Client errors (4xx equivalent)
	ErrorCodeValidation       = "VALIDATION_ERROR"
	ErrorCodeInvalidAction    = "INVALID_ACTION"
	ErrorCodeNotFound         = "NOT_FOUND"
	ErrorCodeUnauthorized     = "UNAUTHORIZED"
	ErrorCodeForbidden        = "FORBIDDEN"
	ErrorCodeRateLimited      = "RATE_LIMITED"
	ErrorCodeDuplicateRequest = "DUPLICATE_REQUEST"

	// Server errors (5xx equivalent)
	ErrorCodeInternal           = "INTERNAL_ERROR"
	ErrorCodeTimeout            = "TIMEOUT"
	ErrorCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrorCodeStorageError       = "STORAGE_ERROR"
	ErrorCodeProcessingFailed   = "PROCESSING_FAILED"

	// Connection errors
	ErrorCodeConnectionClosed = "CONNECTION_CLOSED"
	ErrorCodeInvalidMessage   = "INVALID_MESSAGE"
	ErrorCodeProtocolError    = "PROTOCOL_ERROR"
)

// NewMessage creates a base message with timestamp
func NewMessage(msgType MessageType) Message {
	return Message{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Metadata:  make(map[string]interface{}),
	}
}

// NewRequestMessage creates a new request message
func NewRequestMessage(action string, payload map[string]interface{}) *RequestMessage {
	msg := &RequestMessage{
		Message: NewMessage(MessageTypeRequest),
		Action:  action,
		Payload: payload,
	}
	return msg
}

// NewResponseMessage creates a new response message
func NewResponseMessage(requestID string, success bool, data interface{}) *ResponseMessage {
	msg := &ResponseMessage{
		Message:   NewMessage(MessageTypeResponse),
		RequestID: requestID,
		Success:   success,
		Data:      data,
	}
	return msg
}

// NewAcknowledgmentMessage creates a new acknowledgment message
func NewAcknowledgmentMessage(requestID, status, message string) *AcknowledgmentMessage {
	msg := &AcknowledgmentMessage{
		Message:   NewMessage(MessageTypeAcknowledgment),
		RequestID: requestID,
		Status:    status,
		Text:      message,
	}
	return msg
}

// NewProgressMessage creates a new progress message
func NewProgressMessage(requestID string, percentage float64, message string) *ProgressMessage {
	msg := &ProgressMessage{
		Message:    NewMessage(MessageTypeProgress),
		RequestID:  requestID,
		Percentage: percentage,
		Text:       message,
		Details:    make(map[string]interface{}),
	}
	return msg
}

// NewErrorMessage creates a new error message
func NewErrorMessage(requestID string, errorInfo *ErrorInfo) *ErrorMessage {
	msg := &ErrorMessage{
		Message:   NewMessage(MessageTypeError),
		RequestID: requestID,
		Error:     errorInfo,
	}
	return msg
}

// NewErrorInfo creates a new error info structure
func NewErrorInfo(code, message string) *ErrorInfo {
	return &ErrorInfo{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the error info
func (e *ErrorInfo) WithDetail(key string, value interface{}) *ErrorInfo {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithRetry adds retry information to the error
func (e *ErrorInfo) WithRetry(retryable bool, after time.Time, maxTries int) *ErrorInfo {
	e.Retry = &RetryInfo{
		Retryable: retryable,
		After:     after,
		MaxTries:  maxTries,
	}
	return e
}

// IsClientError checks if the error code represents a client error
func IsClientError(code string) bool {
	switch code {
	case ErrorCodeValidation, ErrorCodeInvalidAction, ErrorCodeNotFound,
		ErrorCodeUnauthorized, ErrorCodeForbidden, ErrorCodeRateLimited,
		ErrorCodeDuplicateRequest:
		return true
	default:
		return false
	}
}

// IsServerError checks if the error code represents a server error
func IsServerError(code string) bool {
	switch code {
	case ErrorCodeInternal, ErrorCodeTimeout, ErrorCodeServiceUnavailable,
		ErrorCodeStorageError, ErrorCodeProcessingFailed:
		return true
	default:
		return false
	}
}

// IsRetryableError checks if an error code indicates a retryable condition
func IsRetryableError(code string) bool {
	switch code {
	case ErrorCodeTimeout, ErrorCodeServiceUnavailable, ErrorCodeRateLimited:
		return true
	default:
		return false
	}
}
