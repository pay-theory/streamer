package streamer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock RequestStore
type mockRequestStore struct {
	mock.Mock
}

func (m *mockRequestStore) Enqueue(ctx context.Context, request *Request) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

// Mock ConnectionManager
type mockConnectionManager struct {
	mock.Mock
}

func (m *mockConnectionManager) Send(ctx context.Context, connectionID string, message interface{}) error {
	args := m.Called(ctx, connectionID, message)
	return args.Error(0)
}

// Mock Handler
type mockHandler struct {
	mock.Mock
}

func (m *mockHandler) EstimatedDuration() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *mockHandler) Validate(req *Request) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *mockHandler) Process(ctx context.Context, req *Request) (*Result, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Result), args.Error(1)
}

func TestNewRouter(t *testing.T) {
	mockStore := new(mockRequestStore)
	mockConnMgr := new(mockConnectionManager)

	router := NewRouter(mockStore, mockConnMgr)

	assert.NotNil(t, router)
	assert.Equal(t, mockStore, router.requestStore)
	assert.Equal(t, mockConnMgr, router.connManager)
	assert.Equal(t, 5*time.Second, router.asyncThreshold)
	assert.NotNil(t, router.handlers)
	assert.Empty(t, router.handlers)
	assert.Empty(t, router.middlewares)
}

func TestDefaultRouter_Handle(t *testing.T) {
	t.Run("successful handler registration", func(t *testing.T) {
		router := NewRouter(nil, nil)
		handler := new(mockHandler)

		err := router.Handle("test-action", handler)
		assert.NoError(t, err)
		assert.Equal(t, handler, router.handlers["test-action"])
	})

	t.Run("empty action error", func(t *testing.T) {
		router := NewRouter(nil, nil)
		handler := new(mockHandler)

		err := router.Handle("", handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action cannot be empty")
	})

	t.Run("nil handler error", func(t *testing.T) {
		router := NewRouter(nil, nil)

		err := router.Handle("test-action", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler cannot be nil")
	})

	t.Run("duplicate handler error", func(t *testing.T) {
		router := NewRouter(nil, nil)
		handler1 := new(mockHandler)
		handler2 := new(mockHandler)

		err := router.Handle("test-action", handler1)
		assert.NoError(t, err)

		err = router.Handle("test-action", handler2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler already registered for action: test-action")
	})

	t.Run("handler with middleware", func(t *testing.T) {
		router := NewRouter(nil, nil)

		// Add middleware first
		middlewareCalled := false
		middleware := func(next Handler) Handler {
			return NewHandlerFunc(
				func(ctx context.Context, req *Request) (*Result, error) {
					middlewareCalled = true
					return next.Process(ctx, req)
				},
				next.EstimatedDuration(),
				next.Validate,
			)
		}
		router.SetMiddleware(middleware)

		// Register handler
		handler := NewHandlerFunc(
			func(ctx context.Context, req *Request) (*Result, error) {
				return &Result{Success: true}, nil
			},
			100*time.Millisecond,
			nil,
		)

		err := router.Handle("test-action", handler)
		assert.NoError(t, err)

		// Process request to test middleware
		wrappedHandler := router.handlers["test-action"]
		result, err := wrappedHandler.Process(context.Background(), &Request{})
		assert.NoError(t, err)
		assert.True(t, result.Success)
		assert.True(t, middlewareCalled)
	})
}

func TestDefaultRouter_SetAsyncThreshold(t *testing.T) {
	router := NewRouter(nil, nil)

	// Default threshold
	assert.Equal(t, 5*time.Second, router.asyncThreshold)

	// Update threshold
	router.SetAsyncThreshold(10 * time.Second)
	assert.Equal(t, 10*time.Second, router.asyncThreshold)
}

func TestDefaultRouter_SetMiddleware(t *testing.T) {
	router := NewRouter(nil, nil)

	// Add single middleware
	middleware1 := func(next Handler) Handler { return next }
	router.SetMiddleware(middleware1)
	assert.Len(t, router.middlewares, 1)

	// Add multiple middlewares
	middleware2 := func(next Handler) Handler { return next }
	middleware3 := func(next Handler) Handler { return next }
	router.SetMiddleware(middleware2, middleware3)
	assert.Len(t, router.middlewares, 3)
}

func TestDefaultRouter_Route(t *testing.T) {
	t.Run("invalid message format", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		// Expect error response
		mockConnMgr.On("Send", mock.Anything, "conn-123", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			if !ok {
				return false
			}
			if m["type"] != "error" {
				return false
			}
			err, ok := m["error"].(*Error)
			return ok && err.Code == ErrCodeValidation
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-123",
			},
			Body: "invalid json",
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)
		mockConnMgr.AssertExpectations(t)
	})

	t.Run("missing action", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		mockConnMgr.On("Send", mock.Anything, "conn-456", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			if !ok {
				return false
			}
			err, ok := m["error"].(*Error)
			return ok && err.Code == ErrCodeValidation && err.Message == "Missing or invalid action"
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-456",
			},
			Body: `{"payload": "test"}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)
		mockConnMgr.AssertExpectations(t)
	})

	t.Run("unknown action", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		mockConnMgr.On("Send", mock.Anything, "conn-789", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			if !ok {
				return false
			}
			err, ok := m["error"].(*Error)
			return ok && err.Code == ErrCodeInvalidAction
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-789",
			},
			Body: `{"action": "unknown"}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)
		mockConnMgr.AssertExpectations(t)
	})

	t.Run("sync handler execution", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		// Register sync handler
		mockHandler := new(mockHandler)
		mockHandler.On("EstimatedDuration").Return(100 * time.Millisecond)
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.MatchedBy(func(req *Request) bool {
			return req.Action == "sync-action" && req.ConnectionID == "conn-sync"
		})).Return(&Result{
			RequestID: "req-123",
			Success:   true,
			Data:      "sync result",
		}, nil)

		router.Handle("sync-action", mockHandler)

		// Expect sync response
		mockConnMgr.On("Send", mock.Anything, "conn-sync", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			return ok && m["type"] == "response" && m["success"] == true
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-sync",
			},
			Body: `{"action": "sync-action", "id": "req-123"}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)

		mockHandler.AssertExpectations(t)
		mockConnMgr.AssertExpectations(t)
	})

	t.Run("async handler queueing", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)
		router.SetAsyncThreshold(1 * time.Second)

		// Register async handler
		mockHandler := new(mockHandler)
		mockHandler.On("EstimatedDuration").Return(2 * time.Second) // Above threshold
		mockHandler.On("Validate", mock.Anything).Return(nil)

		router.Handle("async-action", mockHandler)

		// Expect enqueue
		mockStore.On("Enqueue", mock.Anything, mock.MatchedBy(func(req *Request) bool {
			return req.Action == "async-action"
		})).Return(nil)

		// Expect acknowledgment
		mockConnMgr.On("Send", mock.Anything, "conn-async", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			return ok && m["type"] == "acknowledgment" && m["status"] == "queued"
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-async",
			},
			Body: `{"action": "async-action"}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)

		mockHandler.AssertExpectations(t)
		mockStore.AssertExpectations(t)
		mockConnMgr.AssertExpectations(t)
	})

	t.Run("validation failure", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		// Register handler with validation
		mockHandler := new(mockHandler)
		mockHandler.On("Validate", mock.Anything).Return(errors.New("validation failed"))

		router.Handle("validate-action", mockHandler)

		// Expect validation error
		mockConnMgr.On("Send", mock.Anything, "conn-validate", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			if !ok {
				return false
			}
			err, ok := m["error"].(*Error)
			return ok && err.Code == ErrCodeValidation && err.Message == "validation failed"
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-validate",
			},
			Body: `{"action": "validate-action"}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)

		mockHandler.AssertExpectations(t)
		mockConnMgr.AssertExpectations(t)
	})

	t.Run("async enqueue failure", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		// Register async handler
		mockHandler := new(mockHandler)
		mockHandler.On("EstimatedDuration").Return(10 * time.Second)
		mockHandler.On("Validate", mock.Anything).Return(nil)

		router.Handle("async-fail", mockHandler)

		// Enqueue fails
		mockStore.On("Enqueue", mock.Anything, mock.Anything).Return(errors.New("queue error"))

		// Expect error response
		mockConnMgr.On("Send", mock.Anything, "conn-queue-fail", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			if !ok {
				return false
			}
			err, ok := m["error"].(*Error)
			return ok && err.Code == ErrCodeInternalError
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-queue-fail",
			},
			Body: `{"action": "async-fail"}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)

		mockHandler.AssertExpectations(t)
		mockStore.AssertExpectations(t)
		mockConnMgr.AssertExpectations(t)
	})

	t.Run("process with metadata", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		// Register handler that checks metadata
		capturedReq := (*Request)(nil)
		handler := NewHandlerFunc(
			func(ctx context.Context, req *Request) (*Result, error) {
				capturedReq = req
				return &Result{Success: true}, nil
			},
			100*time.Millisecond,
			nil,
		)

		router.Handle("metadata-action", handler)

		mockConnMgr.On("Send", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-metadata",
			},
			Body: `{
				"action": "metadata-action",
				"id": "custom-id",
				"metadata": {
					"key1": "value1",
					"key2": "value2"
				},
				"payload": {"data": "test"}
			}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)

		// Verify request was properly constructed
		require.NotNil(t, capturedReq)
		assert.Equal(t, "custom-id", capturedReq.ID)
		assert.Equal(t, "conn-metadata", capturedReq.ConnectionID)
		assert.Equal(t, "metadata-action", capturedReq.Action)
		assert.Equal(t, "value1", capturedReq.Metadata["key1"])
		assert.Equal(t, "value2", capturedReq.Metadata["key2"])
		assert.NotNil(t, capturedReq.Payload)

		mockConnMgr.AssertExpectations(t)
	})

	t.Run("handler process error", func(t *testing.T) {
		mockStore := new(mockRequestStore)
		mockConnMgr := new(mockConnectionManager)
		router := NewRouter(mockStore, mockConnMgr)

		// Register handler that returns error
		mockHandler := new(mockHandler)
		mockHandler.On("EstimatedDuration").Return(100 * time.Millisecond)
		mockHandler.On("Validate", mock.Anything).Return(nil)
		mockHandler.On("Process", mock.Anything, mock.Anything).Return(nil, errors.New("processing failed"))

		router.Handle("error-action", mockHandler)

		// Expect error response
		mockConnMgr.On("Send", mock.Anything, "conn-error", mock.MatchedBy(func(msg interface{}) bool {
			m, ok := msg.(map[string]interface{})
			if !ok {
				return false
			}
			err, ok := m["error"].(*Error)
			return ok && err.Code == ErrCodeInternalError && err.Message == "processing failed"
		})).Return(nil)

		event := events.APIGatewayWebsocketProxyRequest{
			RequestContext: events.APIGatewayWebsocketProxyRequestContext{
				ConnectionID: "conn-error",
			},
			Body: `{"action": "error-action"}`,
		}

		err := router.Route(context.Background(), event)
		assert.NoError(t, err)

		mockHandler.AssertExpectations(t)
		mockConnMgr.AssertExpectations(t)
	})
}

func TestDefaultRouter_sendError(t *testing.T) {
	mockConnMgr := new(mockConnectionManager)
	router := &DefaultRouter{
		connManager: mockConnMgr,
	}

	testErr := NewError(ErrCodeValidation, "test error")

	mockConnMgr.On("Send", mock.Anything, "conn-123", mock.MatchedBy(func(msg interface{}) bool {
		m, ok := msg.(map[string]interface{})
		if !ok {
			return false
		}
		return m["type"] == "error" && m["error"] == testErr
	})).Return(nil)

	err := router.sendError(context.Background(), "conn-123", testErr)
	assert.NoError(t, err)

	mockConnMgr.AssertExpectations(t)
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	// Sleep for a nanosecond to ensure different timestamp
	time.Sleep(1 * time.Nanosecond)
	id2 := generateRequestID()

	// IDs should be unique
	assert.NotEqual(t, id1, id2)

	// IDs should have expected format
	assert.Contains(t, id1, "req_")
	assert.Contains(t, id2, "req_")
}

func TestLoggingMiddleware(t *testing.T) {
	logs := []string{}
	logger := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	middleware := LoggingMiddleware(logger)

	// Create test handler
	handler := NewHandlerFunc(
		func(ctx context.Context, req *Request) (*Result, error) {
			return &Result{Success: true}, nil
		},
		100*time.Millisecond,
		nil,
	)

	// Wrap with middleware
	wrappedHandler := middleware(handler)

	// Process request
	req := &Request{
		ID:     "test-123",
		Action: "test-action",
	}
	result, err := wrappedHandler.Process(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.Success)

	// Check logs
	assert.Len(t, logs, 2)
	assert.Contains(t, logs[0], "Processing request: test-123, action: test-action")
	assert.Contains(t, logs[1], "Request completed: test-123")
	assert.Contains(t, logs[1], "success: true")
}

func TestLoggingMiddleware_WithError(t *testing.T) {
	logs := []string{}
	logger := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	middleware := LoggingMiddleware(logger)

	// Create handler that returns error
	handler := NewHandlerFunc(
		func(ctx context.Context, req *Request) (*Result, error) {
			return nil, errors.New("test error")
		},
		100*time.Millisecond,
		nil,
	)

	// Wrap with middleware
	wrappedHandler := middleware(handler)

	// Process request
	req := &Request{
		ID:     "error-123",
		Action: "error-action",
	}
	result, err := wrappedHandler.Process(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, result)

	// Check logs
	assert.Len(t, logs, 2)
	assert.Contains(t, logs[0], "Processing request: error-123, action: error-action")
	assert.Contains(t, logs[1], "Request failed: error-123")
	assert.Contains(t, logs[1], "error: test error")
}
