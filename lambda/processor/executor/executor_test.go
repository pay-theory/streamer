package executor

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Note: Using MockConnectionManager from pkg/connection/mocks.go
// following CENTRALIZED_MOCKS.md architecture

// Helper to create executor for testing
func createTestExecutor(mockQueue store.RequestQueue, logger *log.Logger) *AsyncExecutor {
	// For testing purposes, we can use nil for connection manager
	// since the progress reporter will handle the nil case
	return &AsyncExecutor{
		connManager:      nil,
		requestQueue:     mockQueue,
		handlers:         make(map[string]streamer.Handler),
		progressHandlers: make(map[string]streamer.HandlerWithProgress),
		logger:           logger,
	}
}

// Mock request queue
type mockRequestQueue struct {
	mock.Mock
}

func (m *mockRequestQueue) Enqueue(ctx context.Context, req *store.AsyncRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *mockRequestQueue) Dequeue(ctx context.Context, limit int) ([]*store.AsyncRequest, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.AsyncRequest), args.Error(1)
}

func (m *mockRequestQueue) Get(ctx context.Context, requestID string) (*store.AsyncRequest, error) {
	args := m.Called(ctx, requestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.AsyncRequest), args.Error(1)
}

func (m *mockRequestQueue) UpdateStatus(ctx context.Context, requestID string, status store.RequestStatus, message string) error {
	args := m.Called(ctx, requestID, status, message)
	return args.Error(0)
}

func (m *mockRequestQueue) UpdateProgress(ctx context.Context, requestID string, progress float64, message string, details map[string]interface{}) error {
	args := m.Called(ctx, requestID, progress, message, details)
	return args.Error(0)
}

func (m *mockRequestQueue) CompleteRequest(ctx context.Context, requestID string, result map[string]interface{}) error {
	args := m.Called(ctx, requestID, result)
	return args.Error(0)
}

func (m *mockRequestQueue) FailRequest(ctx context.Context, requestID string, errMsg string) error {
	args := m.Called(ctx, requestID, errMsg)
	return args.Error(0)
}

func (m *mockRequestQueue) GetByConnection(ctx context.Context, connectionID string, limit int) ([]*store.AsyncRequest, error) {
	args := m.Called(ctx, connectionID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.AsyncRequest), args.Error(1)
}

func (m *mockRequestQueue) GetByStatus(ctx context.Context, status store.RequestStatus, limit int) ([]*store.AsyncRequest, error) {
	args := m.Called(ctx, status, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.AsyncRequest), args.Error(1)
}

func (m *mockRequestQueue) Delete(ctx context.Context, requestID string) error {
	args := m.Called(ctx, requestID)
	return args.Error(0)
}

// Mock handler
type mockHandler struct {
	mock.Mock
}

func (m *mockHandler) EstimatedDuration() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *mockHandler) Validate(req *streamer.Request) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *mockHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*streamer.Result), args.Error(1)
}

// Mock handler with progress
type mockHandlerWithProgress struct {
	mockHandler
}

func (m *mockHandlerWithProgress) ProcessWithProgress(ctx context.Context, req *streamer.Request, reporter streamer.ProgressReporter) (*streamer.Result, error) {
	args := m.Called(ctx, req, reporter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*streamer.Result), args.Error(1)
}

func TestNew(t *testing.T) {
	mockQueue := new(mockRequestQueue)
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Using connection mock
	mockConnMgr := connection.NewMockConnectionManager()
	executor := New(mockConnMgr, mockQueue, logger)

	assert.NotNil(t, executor)
	assert.Equal(t, mockQueue, executor.requestQueue)
	assert.NotNil(t, executor.handlers)
	assert.NotNil(t, executor.progressHandlers)
	assert.Equal(t, logger, executor.logger)
}

func TestRegisterHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	t.Run("register basic handler", func(t *testing.T) {
		executor := &AsyncExecutor{
			handlers:         make(map[string]streamer.Handler),
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		handler := new(mockHandler)
		err := executor.RegisterHandler("test-action", handler)

		assert.NoError(t, err)
		assert.Equal(t, handler, executor.handlers["test-action"])
		assert.Nil(t, executor.progressHandlers["test-action"])
	})

	t.Run("register handler with progress", func(t *testing.T) {
		executor := &AsyncExecutor{
			handlers:         make(map[string]streamer.Handler),
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		handler := new(mockHandlerWithProgress)
		err := executor.RegisterHandler("progress-action", handler)

		assert.NoError(t, err)
		assert.Equal(t, handler, executor.handlers["progress-action"])
		assert.Equal(t, handler, executor.progressHandlers["progress-action"])
	})

	t.Run("duplicate handler error", func(t *testing.T) {
		executor := &AsyncExecutor{
			handlers:         make(map[string]streamer.Handler),
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		handler1 := new(mockHandler)
		handler2 := new(mockHandler)

		err := executor.RegisterHandler("test-action", handler1)
		assert.NoError(t, err)

		err = executor.RegisterHandler("test-action", handler2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler already registered")
	})
}

func TestProcessRequest(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	t.Run("successful processing without progress", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-123",
			ConnectionID: "conn-456",
			Action:       "test-action",
			Status:       store.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			CreatedAt:    time.Now(),
		}

		// Mock expectations
		mockQueue.On("UpdateStatus", mock.Anything, "req-123", store.StatusProcessing, "Processing started").Return(nil)

		mockHandler.On("Validate", mock.MatchedBy(func(req *streamer.Request) bool {
			return req.ID == "req-123" && req.Action == "test-action"
		})).Return(nil)

		result := &streamer.Result{
			RequestID: "req-123",
			Success:   true,
			Data:      map[string]interface{}{"result": "success"},
		}
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(result, nil)

		mockQueue.On("CompleteRequest", mock.Anything, "req-123", mock.MatchedBy(func(resultMap map[string]interface{}) bool {
			return resultMap["success"] == true
		})).Return(nil)

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.NoError(t, err)

		mockQueue.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("successful processing with progress", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandlerWithProgress)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"progress-action": mockHandler},
			progressHandlers: map[string]streamer.HandlerWithProgress{"progress-action": mockHandler},
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-789",
			ConnectionID: "conn-012",
			Action:       "progress-action",
			Status:       store.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			CreatedAt:    time.Now(),
		}

		// Mock expectations
		mockQueue.On("UpdateStatus", mock.Anything, "req-789", store.StatusProcessing, "Processing started").Return(nil)

		mockHandler.On("Validate", mock.Anything).Return(nil)

		result := &streamer.Result{
			RequestID: "req-789",
			Success:   true,
			Data:      map[string]interface{}{"result": "success with progress"},
		}
		mockHandler.On("ProcessWithProgress", mock.Anything, mock.Anything, mock.Anything).Return(result, nil)

		mockQueue.On("CompleteRequest", mock.Anything, "req-789", mock.Anything).Return(nil)

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.NoError(t, err)

		mockQueue.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("unknown action error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         make(map[string]streamer.Handler),
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-unknown",
			ConnectionID: "conn-unknown",
			Action:       "unknown-action",
			Status:       store.StatusPending,
			CreatedAt:    time.Now(),
		}

		mockQueue.On("UpdateStatus", mock.Anything, "req-unknown", store.StatusProcessing, "Processing started").Return(nil)
		mockQueue.On("FailRequest", mock.Anything, "req-unknown", "unknown action: unknown-action").Return(nil)

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown action")

		mockQueue.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-invalid",
			ConnectionID: "conn-invalid",
			Action:       "test-action",
			Status:       store.StatusPending,
			CreatedAt:    time.Now(),
		}

		mockQueue.On("UpdateStatus", mock.Anything, "req-invalid", store.StatusProcessing, "Processing started").Return(nil)
		mockHandler.On("Validate", mock.Anything).Return(errors.New("validation failed"))
		mockQueue.On("FailRequest", mock.Anything, "req-invalid", "validation failed: validation failed").Return(nil)

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")

		mockQueue.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("handler processing error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-error",
			ConnectionID: "conn-error",
			Action:       "test-action",
			Status:       store.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			CreatedAt:    time.Now(),
		}

		mockQueue.On("UpdateStatus", mock.Anything, "req-error", store.StatusProcessing, "Processing started").Return(nil)
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(nil, errors.New("processing failed"))
		mockQueue.On("FailRequest", mock.Anything, "req-error", "handler failed: processing failed").Return(nil)

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler failed")

		mockQueue.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})
}

func TestProcessWithRetry(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	t.Run("successful on first attempt", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-retry-1",
			ConnectionID: "conn-retry-1",
			Action:       "test-action",
			Status:       store.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			MaxRetries:   3,
			CreatedAt:    time.Now(),
		}

		// Mock expectations for successful processing
		mockQueue.On("UpdateStatus", mock.Anything, "req-retry-1", store.StatusProcessing, "Processing started").Return(nil)
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(&streamer.Result{Success: true}, nil)
		mockQueue.On("CompleteRequest", mock.Anything, "req-retry-1", mock.Anything).Return(nil)

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessWithRetry(context.Background(), asyncReq)
		assert.NoError(t, err)

		mockQueue.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("retryable error then success", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-retry-2",
			ConnectionID: "conn-retry-2",
			Action:       "test-action",
			Status:       store.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			MaxRetries:   3,
			CreatedAt:    time.Now(),
		}

		// First attempt fails with timeout
		mockQueue.On("UpdateStatus", mock.Anything, "req-retry-2", store.StatusProcessing, "Processing started").Return(nil).Once()
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(nil, errors.New("timeout")).Once()
		mockQueue.On("FailRequest", mock.Anything, "req-retry-2", mock.Anything).Return(nil).Once()

		// Retry attempt
		mockQueue.On("UpdateStatus", mock.Anything, "req-retry-2", store.StatusRetrying, "Retry attempt 1/3").Return(nil).Once()
		mockQueue.On("UpdateStatus", mock.Anything, "req-retry-2", store.StatusProcessing, "Processing started").Return(nil).Once()
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(&streamer.Result{Success: true}, nil).Once()
		mockQueue.On("CompleteRequest", mock.Anything, "req-retry-2", mock.Anything).Return(nil)

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessWithRetry(context.Background(), asyncReq)
		assert.NoError(t, err)
		assert.Equal(t, 1, asyncReq.RetryCount)

		mockQueue.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:    "req-retry-3",
			ConnectionID: "conn-retry-3",
			Action:       "test-action",
			Status:       store.StatusPending,
			MaxRetries:   3,
			CreatedAt:    time.Now(),
		}

		// Validation error is not retryable
		mockQueue.On("UpdateStatus", mock.Anything, "req-retry-3", store.StatusProcessing, "Processing started").Return(nil)
		mockHandler.On("Validate", mock.Anything).Return(errors.New("validation error"))
		mockQueue.On("FailRequest", mock.Anything, "req-retry-3", mock.Anything).Return(nil)

		err := executor.ProcessWithRetry(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed after 1 attempts")

		mockQueue.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("context cancellation during retry", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockQueue := new(mockRequestQueue)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			requestQueue:     mockQueue,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &store.AsyncRequest{
			RequestID:  "req-cancel",
			Action:     "test-action",
			MaxRetries: 3,
			CreatedAt:  time.Now(),
		}

		ctx, cancel := context.WithCancel(context.Background())

		// First attempt fails
		mockQueue.On("UpdateStatus", mock.Anything, "req-cancel", store.StatusProcessing, "Processing started").Return(nil).Once()
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(nil, errors.New("timeout")).Once()
		mockQueue.On("FailRequest", mock.Anything, "req-cancel", mock.Anything).Return(nil).Once()
		mockQueue.On("UpdateStatus", mock.Anything, "req-cancel", store.StatusRetrying, "Retry attempt 1/3").Return(nil).Once()

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		// Cancel context during retry wait
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := executor.ProcessWithRetry(ctx, asyncReq)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "EOF error",
			err:      errors.New("unexpected EOF"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      errors.New("broken pipe"),
			expected: true,
		},
		{
			name:     "validation error",
			err:      errors.New("validation failed: missing field"),
			expected: false,
		},
		{
			name:     "invalid request",
			err:      errors.New("invalid request format"),
			expected: false,
		},
		{
			name:     "required field missing",
			err:      errors.New("required field not provided"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substrs  []string
		expected bool
	}{
		{
			name:     "contains single substring",
			s:        "connection timeout occurred",
			substrs:  []string{"timeout"},
			expected: true,
		},
		{
			name:     "contains one of multiple substrings",
			s:        "network error: connection refused",
			substrs:  []string{"timeout", "refused", "broken"},
			expected: true,
		},
		{
			name:     "contains none of substrings",
			s:        "successful operation",
			substrs:  []string{"error", "failed", "timeout"},
			expected: false,
		},
		{
			name:     "empty string",
			s:        "",
			substrs:  []string{"test"},
			expected: false,
		},
		{
			name:     "empty substrs",
			s:        "test string",
			substrs:  []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substrs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "timeout",
			substr:   "timeout",
			expected: true,
		},
		{
			name:     "contains substring",
			s:        "connection timeout occurred",
			substr:   "timeout",
			expected: true,
		},
		{
			name:     "does not contain",
			s:        "success",
			substr:   "error",
			expected: false,
		},
		{
			name:     "empty string",
			s:        "",
			substr:   "test",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "test",
			substr:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "basic substring",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "substring at start",
			s:        "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "not found",
			s:        "hello world",
			substr:   "test",
			expected: false,
		},
		{
			name:     "longer substring",
			s:        "test",
			substr:   "testing",
			expected: false,
		},
		{
			name:     "empty substring",
			s:        "test",
			substr:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsSubstring(tt.s, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
