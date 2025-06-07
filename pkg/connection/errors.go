package connection

import (
	"errors"
	"fmt"
)

// Common errors
var (
	// ErrConnectionNotFound indicates the connection ID doesn't exist
	ErrConnectionNotFound = errors.New("connection not found")

	// ErrConnectionStale indicates the connection is no longer valid (410 Gone)
	ErrConnectionStale = errors.New("connection is stale")

	// ErrInvalidMessage indicates the message could not be marshaled
	ErrInvalidMessage = errors.New("invalid message format")

	// ErrBroadcastPartialFailure indicates some connections failed during broadcast
	ErrBroadcastPartialFailure = errors.New("broadcast partially failed")
)

// ConnectionError represents a connection-specific error
type ConnectionError struct {
	ConnectionID string
	Err          error
}

func (e *ConnectionError) Error() string {
	return "connection " + e.ConnectionID + ": " + e.Err.Error()
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// ConnectionGoneError indicates a WebSocket connection no longer exists
type ConnectionGoneError struct {
	ConnectionID string
}

func (e *ConnectionGoneError) Error() string {
	return fmt.Sprintf("connection %s is no longer active", e.ConnectionID)
}

// BroadcastError contains errors from broadcasting to multiple connections
type BroadcastError struct {
	Failed []string // Connection IDs that failed
	Errors []error  // Corresponding errors
}

func (e *BroadcastError) Error() string {
	return fmt.Sprintf("broadcast failed for %d connections", len(e.Failed))
}

// IsConnectionGone checks if an error indicates the connection is gone
func IsConnectionGone(err error) bool {
	_, ok := err.(*ConnectionGoneError)
	return ok
}
