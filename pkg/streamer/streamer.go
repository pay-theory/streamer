// Package streamer provides the core interfaces and types for async request processing
package streamer

import (
	"context"
	"encoding/json"
	"time"
)

// Request represents an incoming request from a WebSocket connection
type Request struct {
	ID           string            `json:"id"`
	ConnectionID string            `json:"connection_id"`
	Action       string            `json:"action"`
	Payload      json.RawMessage   `json:"payload"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// Result represents the response from processing a request
type Result struct {
	RequestID string            `json:"request_id"`
	Success   bool              `json:"success"`
	Data      interface{}       `json:"data,omitempty"`
	Error     *Error            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Error represents a structured error response
type Error struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ProgressUpdate represents a progress notification for async operations
type ProgressUpdate struct {
	RequestID  string                 `json:"request_id"`
	Percentage float64                `json:"percentage"`
	Message    string                 `json:"message"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Handler defines the interface for request handlers
type Handler interface {
	// Validate checks if the request is valid
	Validate(request *Request) error

	// EstimatedDuration returns the expected processing time
	// Used to determine if the request should be processed sync or async
	EstimatedDuration() time.Duration

	// Process executes the handler logic
	Process(ctx context.Context, request *Request) (*Result, error)
}

// ProgressReporter allows handlers to report progress during async execution
type ProgressReporter interface {
	// Report sends a progress update
	Report(percentage float64, message string) error

	// SetMetadata adds metadata to the progress update
	SetMetadata(key string, value interface{}) error
}

// HandlerWithProgress extends Handler to support progress reporting
type HandlerWithProgress interface {
	Handler

	// ProcessWithProgress executes the handler with progress reporting capability
	ProcessWithProgress(ctx context.Context, request *Request, reporter ProgressReporter) (*Result, error)
}

// Common error codes
const (
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeInternalError = "INTERNAL_ERROR"
	ErrCodeTimeout       = "TIMEOUT"
	ErrCodeRateLimited   = "RATE_LIMITED"
	ErrCodeInvalidAction = "INVALID_ACTION"
)

// NewError creates a new Error instance
func NewError(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the error
func (e *Error) WithDetail(key string, value interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}
