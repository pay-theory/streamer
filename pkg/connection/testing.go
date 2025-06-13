// Package connection provides testing utilities for connection management.
//
// These mocks are designed to work with any interface that has Send/IsActive methods,
// making them usable across different packages that define their own ConnectionManager interfaces.
package connection

import (
	"context"
	"sync"
)

// SendOnlyMock implements just the Send method, suitable for packages that only need Send.
// This works with pkg/streamer's ConnectionManager interface.
type SendOnlyMock struct {
	// SendFunc allows custom behavior for Send method
	SendFunc func(ctx context.Context, connectionID string, message interface{}) error

	// Messages stores all sent messages for verification
	Messages map[string][]interface{}
	mu       sync.Mutex
}

// NewSendOnlyMock creates a mock that only implements Send
func NewSendOnlyMock() *SendOnlyMock {
	return &SendOnlyMock{
		Messages: make(map[string][]interface{}),
	}
}

// Send implements the minimal ConnectionManager.Send method
func (m *SendOnlyMock) Send(ctx context.Context, connectionID string, message interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SendFunc != nil {
		return m.SendFunc(ctx, connectionID, message)
	}

	// Default behavior: store the message
	m.Messages[connectionID] = append(m.Messages[connectionID], message)
	return nil
}

// GetMessages returns all messages sent to a connection (thread-safe)
func (m *SendOnlyMock) GetMessages(connectionID string) []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]interface{}{}, m.Messages[connectionID]...)
}

// Reset clears all stored messages
func (m *SendOnlyMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = make(map[string][]interface{})
}

// ProgressReporterMock implements Send and IsActive for pkg/progress
type ProgressReporterMock struct {
	SendOnlyMock // Embed for Send functionality

	// IsActiveFunc allows custom behavior for IsActive
	IsActiveFunc func(ctx context.Context, connectionID string) bool

	// ActiveConnections tracks which connections are active
	ActiveConnections map[string]bool
}

// NewProgressReporterMock creates a mock suitable for progress reporting
func NewProgressReporterMock() *ProgressReporterMock {
	return &ProgressReporterMock{
		SendOnlyMock:      SendOnlyMock{Messages: make(map[string][]interface{})},
		ActiveConnections: make(map[string]bool),
	}
}

// IsActive implements the IsActive method needed by pkg/progress
func (m *ProgressReporterMock) IsActive(ctx context.Context, connectionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.IsActiveFunc != nil {
		return m.IsActiveFunc(ctx, connectionID)
	}

	// Default behavior: check ActiveConnections map
	return m.ActiveConnections[connectionID]
}

// SetActive sets a connection's active status
func (m *ProgressReporterMock) SetActive(connectionID string, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveConnections[connectionID] = active
}

// TestifyMock is a minimal testify-based mock for simple cases
type TestifyMock struct {
	mock interface {
		Called(arguments ...interface{}) interface {
			Error(int) error
			Bool(int) bool
			Get(int) interface{}
		}
	}
}

// Send for testify mocks
func (m *TestifyMock) Send(ctx context.Context, connectionID string, message interface{}) error {
	args := m.mock.Called(ctx, connectionID, message)
	return args.Error(0)
}

// IsActive for testify mocks (if needed)
func (m *TestifyMock) IsActive(ctx context.Context, connectionID string) bool {
	args := m.mock.Called(ctx, connectionID)
	return args.Bool(0)
}

// RecordingMock provides detailed recording of all method calls
type RecordingMock struct {
	SendOnlyMock
	IsActiveCalls []string // Track IsActive calls

	// Override for IsActive
	IsActiveFunc func(ctx context.Context, connectionID string) bool
}

// NewRecordingMock creates a mock that records all interactions
func NewRecordingMock() *RecordingMock {
	return &RecordingMock{
		SendOnlyMock:  SendOnlyMock{Messages: make(map[string][]interface{})},
		IsActiveCalls: make([]string, 0),
	}
}

// IsActive records the call and returns result
func (m *RecordingMock) IsActive(ctx context.Context, connectionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IsActiveCalls = append(m.IsActiveCalls, connectionID)

	if m.IsActiveFunc != nil {
		return m.IsActiveFunc(ctx, connectionID)
	}
	return true // Default to active
}

// GetIsActiveCalls returns all IsActive calls
func (m *RecordingMock) GetIsActiveCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.IsActiveCalls...)
}

// FailingMock always returns errors - useful for error handling tests
type FailingMock struct {
	Error error
}

// NewFailingMock creates a mock that always fails
func NewFailingMock(err error) *FailingMock {
	return &FailingMock{Error: err}
}

// Send always returns the configured error
func (m *FailingMock) Send(ctx context.Context, connectionID string, message interface{}) error {
	return m.Error
}

// IsActive always returns false
func (m *FailingMock) IsActive(ctx context.Context, connectionID string) bool {
	return false
}
