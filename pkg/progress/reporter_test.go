package progress

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockConnectionManager is a mock implementation of ConnectionManager
type mockConnectionManager struct {
	mock.Mock
	mu       sync.Mutex
	messages []interface{}
}

func (m *mockConnectionManager) Send(ctx context.Context, connectionID string, message interface{}) error {
	m.mu.Lock()
	m.messages = append(m.messages, message)
	m.mu.Unlock()

	args := m.Called(ctx, connectionID, message)
	return args.Error(0)
}

func (m *mockConnectionManager) IsActive(ctx context.Context, connectionID string) bool {
	args := m.Called(ctx, connectionID)
	return args.Bool(0)
}

func (m *mockConnectionManager) GetMessages() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]interface{}{}, m.messages...)
}

// TestNewReporter tests the NewReporter constructor
func TestNewReporter(t *testing.T) {
	mockConn := new(mockConnectionManager)
	reporter := NewReporter("req123", "conn456", mockConn)

	assert.NotNil(t, reporter)
	assert.Equal(t, "req123", reporter.requestID)
	assert.Equal(t, "conn456", reporter.connectionID)
	assert.Equal(t, mockConn, reporter.connManager)
	assert.NotNil(t, reporter.metadata)
	assert.Equal(t, 100*time.Millisecond, reporter.updateInterval)
}

// TestReport tests the Report method
func TestReport(t *testing.T) {
	tests := []struct {
		name        string
		percentage  float64
		message     string
		isActive    bool
		sendError   error
		expectSend  bool
		expectError error
	}{
		{
			name:       "successful report",
			percentage: 50.0,
			message:    "Processing halfway",
			isActive:   true,
			sendError:  nil,
			expectSend: true,
		},
		{
			name:       "inactive connection",
			percentage: 25.0,
			message:    "Processing",
			isActive:   false,
			expectSend: false,
		},
		{
			name:       "send failure - no error returned",
			percentage: 75.0,
			message:    "Almost done",
			isActive:   true,
			sendError:  errors.New("send failed"),
			expectSend: true,
		},
		{
			name:       "100 percent always sent",
			percentage: 100.0,
			message:    "Complete",
			isActive:   true,
			expectSend: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := new(mockConnectionManager)
			reporter := NewReporter("req123", "conn456", mockConn)

			mockConn.On("IsActive", mock.Anything, "conn456").Return(tt.isActive)
			if tt.expectSend {
				mockConn.On("Send", mock.Anything, "conn456", mock.Anything).Return(tt.sendError)
			}

			err := reporter.Report(tt.percentage, tt.message)
			assert.Equal(t, tt.expectError, err)

			mockConn.AssertExpectations(t)

			if tt.expectSend && tt.isActive {
				calls := mockConn.Calls
				assert.Len(t, calls, 2) // IsActive + Send

				// Check the message content
				sendCall := calls[1]
				msg := sendCall.Arguments[2].(map[string]interface{})
				assert.Equal(t, "progress", msg["type"])
				assert.Equal(t, "req123", msg["request_id"])
				assert.Equal(t, tt.percentage, msg["percentage"])
				assert.Equal(t, tt.message, msg["message"])
				assert.NotNil(t, msg["timestamp"])
			}
		})
	}
}

// TestReportRateLimiting tests that reports are rate limited
func TestReportRateLimiting(t *testing.T) {
	mockConn := new(mockConnectionManager)
	reporter := NewReporter("req123", "conn456", mockConn)
	reporter.updateInterval = 100 * time.Millisecond

	// Always active
	mockConn.On("IsActive", mock.Anything, "conn456").Return(true)
	mockConn.On("Send", mock.Anything, "conn456", mock.Anything).Return(nil)

	// First report should go through
	err := reporter.Report(10.0, "Starting")
	assert.NoError(t, err)

	// Immediate second report should be rate limited
	err = reporter.Report(20.0, "Continuing")
	assert.NoError(t, err)

	// Check only one send was called
	mockConn.AssertNumberOfCalls(t, "Send", 1)

	// Wait for rate limit to expire
	time.Sleep(150 * time.Millisecond)

	// This should go through
	err = reporter.Report(30.0, "More progress")
	assert.NoError(t, err)

	mockConn.AssertNumberOfCalls(t, "Send", 2)
}

// TestSetMetadata tests the SetMetadata method
func TestSetMetadata(t *testing.T) {
	mockConn := new(mockConnectionManager)
	reporter := NewReporter("req123", "conn456", mockConn)

	// Set some metadata
	err := reporter.SetMetadata("key1", "value1")
	assert.NoError(t, err)
	err = reporter.SetMetadata("key2", 123)
	assert.NoError(t, err)

	// Set up mocks for report
	mockConn.On("IsActive", mock.Anything, "conn456").Return(true)
	mockConn.On("Send", mock.Anything, "conn456", mock.Anything).Return(nil)

	// Report progress
	err = reporter.Report(50.0, "With metadata")
	assert.NoError(t, err)

	// Check the sent message includes metadata
	calls := mockConn.Calls
	sendCall := calls[1]
	msg := sendCall.Arguments[2].(map[string]interface{})

	metadata, ok := msg["metadata"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value1", metadata["key1"])
	assert.Equal(t, 123, metadata["key2"])
}

// TestComplete tests the Complete method
func TestComplete(t *testing.T) {
	tests := []struct {
		name      string
		result    interface{}
		sendError error
		wantError bool
	}{
		{
			name:      "successful completion",
			result:    map[string]interface{}{"count": 100},
			sendError: nil,
			wantError: false,
		},
		{
			name:      "completion with send error",
			result:    "done",
			sendError: errors.New("send failed"),
			wantError: true,
		},
		{
			name:      "nil result",
			result:    nil,
			sendError: nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := new(mockConnectionManager)
			reporter := NewReporter("req123", "conn456", mockConn)

			mockConn.On("Send", mock.Anything, "conn456", mock.Anything).Return(tt.sendError)

			err := reporter.Complete(tt.result)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check the message content
			calls := mockConn.Calls
			assert.Len(t, calls, 1)

			msg := calls[0].Arguments[2].(map[string]interface{})
			assert.Equal(t, "complete", msg["type"])
			assert.Equal(t, "req123", msg["request_id"])
			assert.Equal(t, tt.result, msg["result"])
			assert.NotNil(t, msg["timestamp"])
		})
	}
}

// TestFail tests the Fail method
func TestFail(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		sendError error
		wantError bool
	}{
		{
			name:      "successful failure report",
			err:       errors.New("processing failed"),
			sendError: nil,
			wantError: false,
		},
		{
			name:      "failure report with send error",
			err:       errors.New("validation error"),
			sendError: errors.New("send failed"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := new(mockConnectionManager)
			reporter := NewReporter("req123", "conn456", mockConn)

			mockConn.On("Send", mock.Anything, "conn456", mock.Anything).Return(tt.sendError)

			err := reporter.Fail(tt.err)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check the message content
			calls := mockConn.Calls
			assert.Len(t, calls, 1)

			msg := calls[0].Arguments[2].(map[string]interface{})
			assert.Equal(t, "error", msg["type"])
			assert.Equal(t, "req123", msg["request_id"])

			errorInfo, ok := msg["error"].(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, tt.err.Error(), errorInfo["message"])
			assert.Equal(t, "PROCESSING_FAILED", errorInfo["code"])
			assert.NotNil(t, msg["timestamp"])
		})
	}
}

// TestWithReporter tests context functions
func TestWithReporter(t *testing.T) {
	mockConn := new(mockConnectionManager)
	reporter := NewReporter("req123", "conn456", mockConn)

	ctx := context.Background()
	ctxWithReporter := WithReporter(ctx, reporter)

	// Retrieve reporter from context
	retrievedReporter, ok := FromContext(ctxWithReporter)
	assert.True(t, ok)
	assert.Equal(t, reporter, retrievedReporter)

	// Test with context without reporter
	_, ok = FromContext(ctx)
	assert.False(t, ok)
}

// TestReportProgress tests the helper function
func TestReportProgress(t *testing.T) {
	tests := []struct {
		name        string
		hasReporter bool
		percentage  float64
		message     string
		expectError error
	}{
		{
			name:        "with reporter in context",
			hasReporter: true,
			percentage:  50.0,
			message:     "Halfway",
		},
		{
			name:        "without reporter in context",
			hasReporter: false,
			percentage:  50.0,
			message:     "Halfway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.hasReporter {
				mockConn := new(mockConnectionManager)
				reporter := NewReporter("req123", "conn456", mockConn)
				ctx = WithReporter(ctx, reporter)

				mockConn.On("IsActive", mock.Anything, "conn456").Return(true)
				mockConn.On("Send", mock.Anything, "conn456", mock.Anything).Return(nil)
			}

			err := ReportProgress(ctx, tt.percentage, tt.message)
			assert.Equal(t, tt.expectError, err)
		})
	}
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	mockConn := new(mockConnectionManager)
	reporter := NewReporter("req123", "conn456", mockConn)

	// Mock always active and successful sends
	mockConn.On("IsActive", mock.Anything, mock.Anything).Return(true)
	mockConn.On("Send", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent reports
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			err := reporter.Report(float64(idx*10), "Progress")
			assert.NoError(t, err)
		}(i)
	}

	// Concurrent metadata updates
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			err := reporter.SetMetadata(string(rune('a'+idx)), idx)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Should not panic and should have processed some messages
	mockConn.AssertExpectations(t)
}

// TestReporterInterface tests that DefaultReporter implements Reporter interface
func TestReporterInterface(t *testing.T) {
	var _ Reporter = (*DefaultReporter)(nil)
}
