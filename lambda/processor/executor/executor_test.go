package executor

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
	dynamocks "github.com/pay-theory/dynamorm/pkg/mocks"
	storedynamorm "github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Note: Using MockConnectionManager from pkg/connection/mocks.go
// following CENTRALIZED_MOCKS.md architecture

// Helper to create executor for testing
func createTestExecutor(mockDB core.DB, logger *log.Logger) *AsyncExecutor {
	// For testing purposes, we can use nil for connection manager
	// since the progress reporter will handle the nil case
	return &AsyncExecutor{
		connManager:      nil,
		db:               mockDB,
		handlers:         make(map[string]streamer.Handler),
		progressHandlers: make(map[string]streamer.HandlerWithProgress),
		logger:           logger,
	}
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
	mockDB := new(dynamocks.MockDB)
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Using connection mock
	mockConnMgr := connection.NewMockConnectionManager()
	executor := New(mockConnMgr, mockDB, logger)

	assert.NotNil(t, executor)
	assert.Equal(t, mockDB, executor.db)
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
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-123",
			ConnectionID: "conn-456",
			Action:       "test-action",
			Status:       storedynamorm.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			CreatedAt:    time.Now(),
		}

		// Mock expectations for status updates
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockQuery.On("Update", []string{"status", "result", "progress"}).Return(nil).Once()

		mockHandler.On("Validate", mock.MatchedBy(func(req *streamer.Request) bool {
			return req.ID == "req-123" && req.Action == "test-action"
		})).Return(nil)

		result := &streamer.Result{
			RequestID: "req-123",
			Success:   true,
			Data:      map[string]interface{}{"result": "success"},
		}
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(result, nil)

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("successful processing with progress", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandlerWithProgress)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"progress-action": mockHandler},
			progressHandlers: map[string]streamer.HandlerWithProgress{"progress-action": mockHandler},
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-789",
			ConnectionID: "conn-012",
			Action:       "progress-action",
			Status:       storedynamorm.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			CreatedAt:    time.Now(),
		}

		// Mock expectations for status updates
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockQuery.On("Update", []string{"status", "result", "progress"}).Return(nil).Once()

		mockHandler.On("Validate", mock.Anything).Return(nil)

		result := &streamer.Result{
			RequestID: "req-789",
			Success:   true,
			Data:      map[string]interface{}{"result": "success with progress"},
		}
		mockHandler.On("ProcessWithProgress", mock.Anything, mock.Anything, mock.Anything).Return(result, nil)

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("unknown action error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         make(map[string]streamer.Handler),
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-unknown",
			ConnectionID: "conn-unknown",
			Action:       "unknown-action",
			Status:       storedynamorm.StatusPending,
			CreatedAt:    time.Now(),
		}

		// Mock expectations
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockQuery.On("Update", []string{"status", "error"}).Return(nil).Once()

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown action")

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-invalid",
			ConnectionID: "conn-invalid",
			Action:       "test-action",
			Status:       storedynamorm.StatusPending,
			CreatedAt:    time.Now(),
		}

		// Mock expectations
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockHandler.On("Validate", mock.Anything).Return(errors.New("validation failed"))
		mockQuery.On("Update", []string{"status", "error"}).Return(nil).Once()

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("handler processing error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-error",
			ConnectionID: "conn-error",
			Action:       "test-action",
			Status:       storedynamorm.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			CreatedAt:    time.Now(),
		}

		// Mock expectations
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(nil, errors.New("processing failed"))
		mockQuery.On("Update", []string{"status", "error"}).Return(nil).Once()

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessRequest(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler failed")

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})
}

func TestProcessWithRetry(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	t.Run("successful on first attempt", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-retry-1",
			ConnectionID: "conn-retry-1",
			Action:       "test-action",
			Status:       storedynamorm.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			MaxRetries:   3,
			CreatedAt:    time.Now(),
		}

		// Mock expectations for successful processing
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(&streamer.Result{Success: true}, nil)
		mockQuery.On("Update", []string{"status", "result", "progress"}).Return(nil).Once()

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessWithRetry(context.Background(), asyncReq)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("retryable error then success", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-retry-2",
			ConnectionID: "conn-retry-2",
			Action:       "test-action",
			Status:       storedynamorm.StatusPending,
			Payload:      map[string]interface{}{"data": "test"},
			MaxRetries:   3,
			CreatedAt:    time.Now(),
		}

		// Mock expectations
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		
		// First attempt fails with timeout
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(nil, errors.New("timeout")).Once()
		mockQuery.On("Update", []string{"status", "error"}).Return(nil).Once()

		// Retry attempt
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Twice() // Once for retry status, once for processing
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(&streamer.Result{Success: true}, nil).Once()
		mockQuery.On("Update", []string{"status", "result", "progress"}).Return(nil).Once()

		// Set up connection manager mock behavior
		mockConnMgr.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
			return nil
		}

		err := executor.ProcessWithRetry(context.Background(), asyncReq)
		assert.NoError(t, err)
		assert.Equal(t, 1, asyncReq.RetryCount)

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:    "req-retry-3",
			ConnectionID: "conn-retry-3",
			Action:       "test-action",
			Status:       storedynamorm.StatusPending,
			MaxRetries:   3,
			CreatedAt:    time.Now(),
		}

		// Validation error is not retryable
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Once()
		mockHandler.On("Validate", mock.Anything).Return(errors.New("validation error"))
		mockQuery.On("Update", []string{"status", "error"}).Return(nil).Once()

		err := executor.ProcessWithRetry(context.Background(), asyncReq)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed after 1 attempts")

		mockDB.AssertExpectations(t)
		mockQuery.AssertExpectations(t)
		mockHandler.AssertExpectations(t)
	})

	t.Run("context cancellation during retry", func(t *testing.T) {
		mockConnMgr := connection.NewMockConnectionManager()
		mockDB := new(dynamocks.MockDB)
		mockQuery := new(dynamocks.MockQuery)
		mockHandler := new(mockHandler)

		executor := &AsyncExecutor{
			connManager:      mockConnMgr,
			db:               mockDB,
			handlers:         map[string]streamer.Handler{"test-action": mockHandler},
			progressHandlers: make(map[string]streamer.HandlerWithProgress),
			logger:           logger,
		}

		asyncReq := &storedynamorm.AsyncRequest{
			RequestID:  "req-cancel",
			Action:     "test-action",
			MaxRetries: 3,
			CreatedAt:  time.Now(),
		}

		ctx, cancel := context.WithCancel(context.Background())

		// First attempt fails
		mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
		mockQuery.On("Update", []string{"status", "progress_message"}).Return(nil).Twice() // Once for processing, once for retry
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(nil, errors.New("timeout")).Once()
		mockQuery.On("Update", []string{"status", "error"}).Return(nil).Once()

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
