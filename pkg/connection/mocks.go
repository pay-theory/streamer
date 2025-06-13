// Package connection provides mock implementations for testing.
//
// This file consolidates all mock types for the connection package.
// We provide both manual mocks (with function fields) and testify-based mocks
// to support different testing styles across the codebase.
package connection

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stretchr/testify/mock"
)

// =============================================================================
// ConnectionManager Mocks
// =============================================================================

// MockConnectionManager is a manual mock implementation of ConnectionManager.
// Use this when you need fine control over mock behavior or want to avoid testify.
type MockConnectionManager struct {
	// Function fields for each method
	SendFunc       func(ctx context.Context, connectionID string, message interface{}) error
	BroadcastFunc  func(ctx context.Context, connectionIDs []string, message interface{}) error
	IsActiveFunc   func(ctx context.Context, connectionID string) bool
	GetMetricsFunc func() map[string]interface{}
	ShutdownFunc   func(ctx context.Context) error
	SetLoggerFunc  func(logger func(format string, args ...interface{}))

	// State tracking
	mu      sync.Mutex
	calls   []MethodCall
	logger  func(format string, args ...interface{})
	metrics ConnectionMetrics
}

// MethodCall records a method invocation
type MethodCall struct {
	Method    string
	Arguments []interface{}
	Timestamp time.Time
}

// ConnectionMetrics tracks mock connection metrics
type ConnectionMetrics struct {
	SendCount      int
	BroadcastCount int
	FailureCount   int
	ActiveConns    map[string]bool
}

// NewMockConnectionManager creates a mock with sensible defaults
func NewMockConnectionManager() *MockConnectionManager {
	m := &MockConnectionManager{
		calls:   make([]MethodCall, 0),
		metrics: ConnectionMetrics{ActiveConns: make(map[string]bool)},
		logger:  func(format string, args ...interface{}) {},
	}
	m.setupDefaults()
	return m
}

func (m *MockConnectionManager) setupDefaults() {
	m.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
		m.mu.Lock()
		m.metrics.SendCount++
		m.mu.Unlock()
		return nil
	}

	m.BroadcastFunc = func(ctx context.Context, connectionIDs []string, message interface{}) error {
		m.mu.Lock()
		m.metrics.BroadcastCount++
		m.mu.Unlock()
		return nil
	}

	m.IsActiveFunc = func(ctx context.Context, connectionID string) bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.metrics.ActiveConns[connectionID]
	}

	m.GetMetricsFunc = func() map[string]interface{} {
		m.mu.Lock()
		defer m.mu.Unlock()
		return map[string]interface{}{
			"send_count":      m.metrics.SendCount,
			"broadcast_count": m.metrics.BroadcastCount,
			"failure_count":   m.metrics.FailureCount,
			"active_conns":    len(m.metrics.ActiveConns),
		}
	}

	m.ShutdownFunc = func(ctx context.Context) error {
		return nil
	}

	m.SetLoggerFunc = func(logger func(format string, args ...interface{})) {
		m.mu.Lock()
		m.logger = logger
		m.mu.Unlock()
	}
}

// Send implements ConnectionManager
func (m *MockConnectionManager) Send(ctx context.Context, connectionID string, message interface{}) error {
	m.recordCall("Send", connectionID, message)
	if m.SendFunc != nil {
		return m.SendFunc(ctx, connectionID, message)
	}
	return nil
}

// Broadcast implements ConnectionManager
func (m *MockConnectionManager) Broadcast(ctx context.Context, connectionIDs []string, message interface{}) error {
	m.recordCall("Broadcast", connectionIDs, message)
	if m.BroadcastFunc != nil {
		return m.BroadcastFunc(ctx, connectionIDs, message)
	}
	return nil
}

// IsActive implements ConnectionManager
func (m *MockConnectionManager) IsActive(ctx context.Context, connectionID string) bool {
	m.recordCall("IsActive", connectionID)
	if m.IsActiveFunc != nil {
		return m.IsActiveFunc(ctx, connectionID)
	}
	return true
}

// GetMetrics implements ConnectionManager
func (m *MockConnectionManager) GetMetrics() map[string]interface{} {
	m.recordCall("GetMetrics")
	if m.GetMetricsFunc != nil {
		return m.GetMetricsFunc()
	}
	return make(map[string]interface{})
}

// Shutdown implements ConnectionManager
func (m *MockConnectionManager) Shutdown(ctx context.Context) error {
	m.recordCall("Shutdown")
	if m.ShutdownFunc != nil {
		return m.ShutdownFunc(ctx)
	}
	return nil
}

// SetLogger implements ConnectionManager
func (m *MockConnectionManager) SetLogger(logger func(format string, args ...interface{})) {
	m.recordCall("SetLogger", logger)
	if m.SetLoggerFunc != nil {
		m.SetLoggerFunc(logger)
	}
}

func (m *MockConnectionManager) recordCall(method string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, MethodCall{
		Method:    method,
		Arguments: args,
		Timestamp: time.Now(),
	})
}

// Testing helpers
func (m *MockConnectionManager) CallCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, call := range m.calls {
		if call.Method == method {
			count++
		}
	}
	return count
}

func (m *MockConnectionManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = m.calls[:0]
	m.metrics = ConnectionMetrics{ActiveConns: make(map[string]bool)}
}

// MockConnectionManagerTestify is a testify-based mock of ConnectionManager.
// Use this when you prefer testify's mocking style.
type MockConnectionManagerTestify struct {
	mock.Mock
}

func (m *MockConnectionManagerTestify) Send(ctx context.Context, connectionID string, message interface{}) error {
	args := m.Called(ctx, connectionID, message)
	return args.Error(0)
}

func (m *MockConnectionManagerTestify) Broadcast(ctx context.Context, connectionIDs []string, message interface{}) error {
	args := m.Called(ctx, connectionIDs, message)
	return args.Error(0)
}

func (m *MockConnectionManagerTestify) IsActive(ctx context.Context, connectionID string) bool {
	args := m.Called(ctx, connectionID)
	return args.Bool(0)
}

func (m *MockConnectionManagerTestify) GetMetrics() map[string]interface{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]interface{})
}

func (m *MockConnectionManagerTestify) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockConnectionManagerTestify) SetLogger(logger func(format string, args ...interface{})) {
	m.Called(logger)
}

// =============================================================================
// APIGatewayClient Mocks
// =============================================================================

// MockAPIGatewayClient is a testify-based mock of APIGatewayClient
type MockAPIGatewayClient struct {
	mock.Mock
	mu          sync.Mutex
	connections map[string]*ConnectionInfo
	messages    map[string][][]byte // connectionID -> messages sent
}

// NewMockAPIGatewayClient creates a new mock API Gateway client
func NewMockAPIGatewayClient() *MockAPIGatewayClient {
	return &MockAPIGatewayClient{
		connections: make(map[string]*ConnectionInfo),
		messages:    make(map[string][][]byte),
	}
}

func (m *MockAPIGatewayClient) PostToConnection(ctx context.Context, connectionID string, data []byte) error {
	m.mu.Lock()
	m.messages[connectionID] = append(m.messages[connectionID], data)
	m.mu.Unlock()

	args := m.Called(ctx, connectionID, data)
	return args.Error(0)
}

func (m *MockAPIGatewayClient) DeleteConnection(ctx context.Context, connectionID string) error {
	m.mu.Lock()
	delete(m.connections, connectionID)
	delete(m.messages, connectionID)
	m.mu.Unlock()

	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *MockAPIGatewayClient) GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ConnectionInfo), args.Error(1)
}

// Helper methods
func (m *MockAPIGatewayClient) AddConnection(connectionID string, info *ConnectionInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections[connectionID] = info
}

func (m *MockAPIGatewayClient) GetMessages(connectionID string) [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte{}, m.messages[connectionID]...)
}

func (m *MockAPIGatewayClient) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make(map[string][][]byte)
}

// TestableAPIGatewayClient is a configurable mock for complex test scenarios.
// This provides more control than the testify mock for simulating various conditions.
type TestableAPIGatewayClient struct {
	mu          sync.Mutex
	connections map[string]*ConnectionInfo
	messages    map[string][][]byte
	errors      map[string]error // connectionID -> error to return
	latency     time.Duration
	failureRate float32
}

// NewTestableAPIGatewayClient creates a new testable client
func NewTestableAPIGatewayClient() *TestableAPIGatewayClient {
	return &TestableAPIGatewayClient{
		connections: make(map[string]*ConnectionInfo),
		messages:    make(map[string][][]byte),
		errors:      make(map[string]error),
	}
}

func (t *TestableAPIGatewayClient) PostToConnection(ctx context.Context, connectionID string, data []byte) error {
	if t.latency > 0 {
		select {
		case <-time.After(t.latency):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err, exists := t.errors[connectionID]; exists {
		return err
	}

	if _, exists := t.connections[connectionID]; !exists {
		return GoneError{
			ConnectionID: connectionID,
			Message:      "connection does not exist",
		}
	}

	t.messages[connectionID] = append(t.messages[connectionID], data)
	return nil
}

func (t *TestableAPIGatewayClient) DeleteConnection(ctx context.Context, connectionID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.connections[connectionID]; !exists {
		return GoneError{
			ConnectionID: connectionID,
			Message:      "connection does not exist",
		}
	}

	delete(t.connections, connectionID)
	delete(t.messages, connectionID)
	return nil
}

func (t *TestableAPIGatewayClient) GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, exists := t.connections[connectionID]
	if !exists {
		return nil, GoneError{
			ConnectionID: connectionID,
			Message:      "connection does not exist",
		}
	}

	return &ConnectionInfo{
		ConnectionID: info.ConnectionID,
		ConnectedAt:  info.ConnectedAt,
		LastActiveAt: time.Now().Format(time.RFC3339),
		SourceIP:     info.SourceIP,
		UserAgent:    info.UserAgent,
	}, nil
}

// Configuration methods
func (t *TestableAPIGatewayClient) AddConnection(id string, sourceIP string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connections[id] = &ConnectionInfo{
		ConnectionID: id,
		ConnectedAt:  time.Now().Format(time.RFC3339),
		LastActiveAt: time.Now().Format(time.RFC3339),
		SourceIP:     sourceIP,
		UserAgent:    "test-agent",
	}
}

func (t *TestableAPIGatewayClient) SetError(connectionID string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errors[connectionID] = err
}

func (t *TestableAPIGatewayClient) SimulateGoneError(connectionID string) {
	t.SetError(connectionID, GoneError{
		ConnectionID: connectionID,
		Message:      fmt.Sprintf("Connection %s is gone", connectionID),
	})
}

func (t *TestableAPIGatewayClient) SimulateThrottling(connectionID string, retryAfter int) {
	t.SetError(connectionID, ThrottlingError{
		ConnectionID: connectionID,
		RetryAfter:   retryAfter,
		Message:      "Rate limit exceeded",
	})
}

func (t *TestableAPIGatewayClient) SetLatency(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.latency = d
}

func (t *TestableAPIGatewayClient) GetMessages(connectionID string) [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([][]byte{}, t.messages[connectionID]...)
}

func (t *TestableAPIGatewayClient) GetMessageCount(connectionID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.messages[connectionID])
}

func (t *TestableAPIGatewayClient) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connections = make(map[string]*ConnectionInfo)
	t.messages = make(map[string][][]byte)
	t.errors = make(map[string]error)
	t.latency = 0
	t.failureRate = 0
}

// Interface compliance checks
var (
	_ ConnectionManager = (*MockConnectionManager)(nil)
	_ ConnectionManager = (*MockConnectionManagerTestify)(nil)
	_ APIGatewayClient  = (*MockAPIGatewayClient)(nil)
	_ APIGatewayClient  = (*TestableAPIGatewayClient)(nil)
)
