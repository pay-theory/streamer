package connection

import (
	"context"
)

// ConnectionManager defines the interface for managing WebSocket connections
type ConnectionManager interface {
	// Send sends a message to a specific connection
	Send(ctx context.Context, connectionID string, message interface{}) error

	// Broadcast sends a message to multiple connections
	Broadcast(ctx context.Context, connectionIDs []string, message interface{}) error

	// IsActive checks if a connection is active
	IsActive(ctx context.Context, connectionID string) bool

	// GetMetrics returns current performance metrics
	GetMetrics() map[string]interface{}

	// Shutdown gracefully shuts down the manager
	Shutdown(ctx context.Context) error

	// SetLogger sets a custom logger function
	SetLogger(logger func(format string, args ...interface{}))
}

// Ensure Manager implements ConnectionManager interface
var _ ConnectionManager = (*Manager)(nil)

// APIGatewayClient defines the interface for API Gateway Management API operations
type APIGatewayClient interface {
	// PostToConnection sends data to a WebSocket connection
	PostToConnection(ctx context.Context, connectionID string, data []byte) error

	// DeleteConnection terminates a WebSocket connection
	DeleteConnection(ctx context.Context, connectionID string) error

	// GetConnection retrieves connection information
	GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error)
}

// ConnectionInfo represents information about a WebSocket connection
type ConnectionInfo struct {
	ConnectionID string
	ConnectedAt  string
	LastActiveAt string
	SourceIP     string
	UserAgent    string
}

// APIError represents an error from the API Gateway service
type APIError interface {
	error
	// HTTPStatusCode returns the HTTP status code of the error
	HTTPStatusCode() int
	// ErrorCode returns the error code (e.g., "GoneException")
	ErrorCode() string
	// IsRetryable returns true if the error is retryable
	IsRetryable() bool
}

// Common API Gateway error types
type (
	// GoneError indicates the connection no longer exists (410 Gone)
	GoneError struct {
		ConnectionID string
		Message      string
	}

	// ForbiddenError indicates access is forbidden (403 Forbidden)
	ForbiddenError struct {
		ConnectionID string
		Message      string
	}

	// PayloadTooLargeError indicates the payload exceeds size limits
	PayloadTooLargeError struct {
		ConnectionID string
		PayloadSize  int
		MaxSize      int
		Message      string
	}

	// ThrottlingError indicates rate limit exceeded (429 Too Many Requests)
	ThrottlingError struct {
		ConnectionID string
		RetryAfter   int // seconds
		Message      string
	}

	// InternalServerError indicates a server error (500)
	InternalServerError struct {
		Message string
	}
)

// Error implementations
func (e GoneError) Error() string {
	return e.Message
}

func (e GoneError) HTTPStatusCode() int {
	return 410
}

func (e GoneError) ErrorCode() string {
	return "GoneException"
}

func (e GoneError) IsRetryable() bool {
	return false
}

func (e ForbiddenError) Error() string {
	return e.Message
}

func (e ForbiddenError) HTTPStatusCode() int {
	return 403
}

func (e ForbiddenError) ErrorCode() string {
	return "ForbiddenException"
}

func (e ForbiddenError) IsRetryable() bool {
	return false
}

func (e PayloadTooLargeError) Error() string {
	return e.Message
}

func (e PayloadTooLargeError) HTTPStatusCode() int {
	return 413
}

func (e PayloadTooLargeError) ErrorCode() string {
	return "PayloadTooLargeException"
}

func (e PayloadTooLargeError) IsRetryable() bool {
	return false
}

func (e ThrottlingError) Error() string {
	return e.Message
}

func (e ThrottlingError) HTTPStatusCode() int {
	return 429
}

func (e ThrottlingError) ErrorCode() string {
	return "ThrottlingException"
}

func (e ThrottlingError) IsRetryable() bool {
	return true
}

func (e InternalServerError) Error() string {
	return e.Message
}

func (e InternalServerError) HTTPStatusCode() int {
	return 500
}

func (e InternalServerError) ErrorCode() string {
	return "InternalServerError"
}

func (e InternalServerError) IsRetryable() bool {
	return true
}
