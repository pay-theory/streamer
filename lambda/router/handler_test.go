package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/streamer/pkg/streamer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestMain sets up environment variables required for tests
func TestMain(m *testing.M) {
	// Set required environment variables for Lambda tests
	os.Setenv("WEBSOCKET_ENDPOINT", "wss://mock.execute-api.us-east-1.amazonaws.com/dev")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("_X_AMZN_TRACE_ID", "Root=1-mock-trace")

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// Mock Request Store
type mockRequestStore struct {
	mock.Mock
}

func (m *mockRequestStore) Enqueue(ctx context.Context, request *streamer.Request) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

// Mock Connection Manager
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
	duration time.Duration
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

// Test handler for actual handler tests
type testHandler struct {
	validateFunc func(req *streamer.Request) error
	processFunc  func(ctx context.Context, req *streamer.Request) (*streamer.Result, error)
	duration     time.Duration
}

func (h *testHandler) EstimatedDuration() time.Duration {
	return h.duration
}

func (h *testHandler) Validate(req *streamer.Request) error {
	if h.validateFunc != nil {
		return h.validateFunc(req)
	}
	return nil
}

func (h *testHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	if h.processFunc != nil {
		return h.processFunc(ctx, req)
	}
	return &streamer.Result{Success: true}, nil
}

func TestHandler_Route(t *testing.T) {
	tests := []struct {
		name       string
		event      events.APIGatewayWebsocketProxyRequest
		setupMocks func(*mockRequestStore, *mockConnectionManager, *streamer.DefaultRouter)
		wantErr    bool
		checkMocks func(t *testing.T, reqStore *mockRequestStore, connMgr *mockConnectionManager)
	}{
		{
			name: "successful sync handler execution",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-123",
					RouteKey:     "$default",
				},
				Body: `{"action": "echo", "payload": {"message": "hello"}, "id": "req-123"}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				// Register sync handler
				handler := &testHandler{
					duration: 100 * time.Millisecond, // Below async threshold
					processFunc: func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
						return &streamer.Result{
							RequestID: req.ID,
							Success:   true,
							Data:      map[string]interface{}{"echo": "hello"},
						}, nil
					},
				}
				router.Handle("echo", handler)

				// Expect sync response
				connMgr.On("Send", mock.Anything, "conn-123", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					return ok && m["type"] == "response" && m["request_id"] == "req-123"
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "successful async handler queueing",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-456",
				},
				Body: `{"action": "generate_report", "payload": {"type": "monthly"}}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				// Register async handler
				handler := &testHandler{
					duration: 10 * time.Second, // Above async threshold
				}
				router.Handle("generate_report", handler)

				// Expect request to be queued
				reqStore.On("Enqueue", mock.Anything, mock.MatchedBy(func(req *streamer.Request) bool {
					return req.Action == "generate_report" && req.ConnectionID == "conn-456"
				})).Return(nil)

				// Expect acknowledgment
				connMgr.On("Send", mock.Anything, "conn-456", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					return ok && m["type"] == "acknowledgment" && m["status"] == "queued"
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "invalid message format",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-789",
				},
				Body: `invalid json`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				// Expect error response
				connMgr.On("Send", mock.Anything, "conn-789", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					if !ok || m["type"] != "error" {
						return false
					}
					err, ok := m["error"].(*streamer.Error)
					return ok && err.Code == streamer.ErrCodeValidation
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "missing action",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-no-action",
				},
				Body: `{"payload": {"data": "test"}}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				connMgr.On("Send", mock.Anything, "conn-no-action", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					if !ok || m["type"] != "error" {
						return false
					}
					err, ok := m["error"].(*streamer.Error)
					return ok && err.Code == streamer.ErrCodeValidation
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "unknown action",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-unknown",
				},
				Body: `{"action": "unknown_action"}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				connMgr.On("Send", mock.Anything, "conn-unknown", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					if !ok || m["type"] != "error" {
						return false
					}
					err, ok := m["error"].(*streamer.Error)
					return ok && err.Code == streamer.ErrCodeInvalidAction
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "handler validation failure",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-validate-fail",
				},
				Body: `{"action": "validate_fail", "payload": {}}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				handler := &testHandler{
					validateFunc: func(req *streamer.Request) error {
						return errors.New("validation failed: missing required field")
					},
				}
				router.Handle("validate_fail", handler)

				connMgr.On("Send", mock.Anything, "conn-validate-fail", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					if !ok || m["type"] != "error" {
						return false
					}
					err, ok := m["error"].(*streamer.Error)
					return ok && err.Code == streamer.ErrCodeValidation
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "handler processing error",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-process-fail",
				},
				Body: `{"action": "process_fail"}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				handler := &testHandler{
					duration: 100 * time.Millisecond,
					processFunc: func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
						return nil, errors.New("processing failed")
					},
				}
				router.Handle("process_fail", handler)

				connMgr.On("Send", mock.Anything, "conn-process-fail", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					if !ok || m["type"] != "error" {
						return false
					}
					err, ok := m["error"].(*streamer.Error)
					return ok && err.Code == streamer.ErrCodeInternalError
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "async queue failure",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-queue-fail",
				},
				Body: `{"action": "async_fail"}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				handler := &testHandler{
					duration: 10 * time.Second, // Async
				}
				router.Handle("async_fail", handler)

				// Queue fails
				reqStore.On("Enqueue", mock.Anything, mock.Anything).Return(errors.New("queue error"))

				connMgr.On("Send", mock.Anything, "conn-queue-fail", mock.MatchedBy(func(msg interface{}) bool {
					m, ok := msg.(map[string]interface{})
					if !ok || m["type"] != "error" {
						return false
					}
					err, ok := m["error"].(*streamer.Error)
					return ok && err.Code == streamer.ErrCodeInternalError
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "request with metadata",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-metadata",
					Authorizer: map[string]interface{}{
						"userId":   "user-123",
						"tenantId": "tenant-456",
					},
				},
				Body: `{"action": "echo", "metadata": {"trace_id": "trace-123"}}`,
			},
			setupMocks: func(reqStore *mockRequestStore, connMgr *mockConnectionManager, router *streamer.DefaultRouter) {
				handler := &testHandler{
					duration: 100 * time.Millisecond,
					processFunc: func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
						// Verify metadata was preserved
						assert.Equal(t, "trace-123", req.Metadata["trace_id"])
						return &streamer.Result{Success: true}, nil
					},
				}
				router.Handle("echo", handler)

				connMgr.On("Send", mock.Anything, "conn-metadata", mock.Anything).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockReqStore := new(mockRequestStore)
			mockConnMgr := new(mockConnectionManager)

			// Create router
			router := streamer.NewRouter(mockReqStore, mockConnMgr)
			router.SetAsyncThreshold(5 * time.Second)

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(mockReqStore, mockConnMgr, router)
			}

			// Execute
			err := router.Route(context.Background(), tt.event)

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mocks
			mockReqStore.AssertExpectations(t)
			mockConnMgr.AssertExpectations(t)

			// Additional checks
			if tt.checkMocks != nil {
				tt.checkMocks(t, mockReqStore, mockConnMgr)
			}
		})
	}
}

func TestRegisterHandlers(t *testing.T) {
	router := streamer.NewRouter(nil, nil)

	err := registerHandlers(router)
	require.NoError(t, err)

	// Test that all expected handlers are registered
	testCases := []struct {
		action string
		body   string
	}{
		{"echo", `{"action": "echo", "payload": {"message": "test"}}`},
		{"health", `{"action": "health"}`},
		{"generate_report", `{"action": "generate_report"}`},
		{"process_data", `{"action": "process_data"}`},
		{"bulk_operation", `{"action": "bulk_operation"}`},
	}

	for _, tc := range testCases {
		t.Run(tc.action, func(t *testing.T) {
			// Create a test event
			event := events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "test-conn",
				},
				Body: tc.body,
			}

			// Mock connection manager to capture the response
			mockConnMgr := new(mockConnectionManager)
			mockConnMgr.On("Send", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			// Create router with mocks
			testRouter := streamer.NewRouter(new(mockRequestStore), mockConnMgr)
			err := registerHandlers(testRouter)
			require.NoError(t, err)

			// Route should not return an error (handler exists)
			err = testRouter.Route(context.Background(), event)
			assert.NoError(t, err)
		})
	}
}

func TestMiddleware(t *testing.T) {
	t.Run("validation middleware", func(t *testing.T) {
		middleware := validationMiddleware()

		// Test payload size validation
		handler := &testHandler{
			processFunc: func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
				return &streamer.Result{Success: true}, nil
			},
		}

		wrappedHandler := middleware(handler)

		// Test large payload
		largePayload := make([]byte, 1024*1024+1)
		req := &streamer.Request{
			Payload:  largePayload,
			Metadata: make(map[string]string),
		}

		_, err := wrappedHandler.Process(context.Background(), req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Payload too large")

		// Test context values are added to metadata
		ctx := context.WithValue(context.Background(), "userId", "user-123")
		ctx = context.WithValue(ctx, "tenantId", "tenant-456")

		smallReq := &streamer.Request{
			Payload:  []byte("small payload"),
			Metadata: make(map[string]string),
		}

		_, err = wrappedHandler.Process(ctx, smallReq)
		assert.NoError(t, err)
		assert.Equal(t, "user-123", smallReq.Metadata["user_id"])
		assert.Equal(t, "tenant-456", smallReq.Metadata["tenant_id"])
	})

	t.Run("metrics middleware", func(t *testing.T) {
		middleware := metricsMiddleware()

		handler := &testHandler{
			processFunc: func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
				time.Sleep(10 * time.Millisecond) // Simulate work
				return &streamer.Result{Success: true}, nil
			},
		}

		wrappedHandler := middleware(handler)

		req := &streamer.Request{
			Action: "test_action",
		}

		result, err := wrappedHandler.Process(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
	})
}

func TestHandlerImplementations(t *testing.T) {
	t.Run("HealthHandler", func(t *testing.T) {
		handler := NewHealthHandler()

		// Test estimated duration
		assert.Equal(t, 10*time.Millisecond, handler.EstimatedDuration())

		// Test validation (should always pass)
		err := handler.Validate(&streamer.Request{})
		assert.NoError(t, err)

		// Test processing
		result, err := handler.Process(context.Background(), &streamer.Request{ID: "test-123"})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, "test-123", result.RequestID)

		data, ok := result.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "healthy", data["status"])
		assert.Equal(t, "1.0.0", data["version"])
	})

	t.Run("ReportHandler validation", func(t *testing.T) {
		handler := NewReportHandler()

		// Test EstimatedDuration
		assert.Equal(t, 2*time.Minute, handler.EstimatedDuration())

		tests := []struct {
			name    string
			payload interface{}
			wantErr string
		}{
			{
				name:    "nil payload",
				payload: nil,
				wantErr: "payload is required",
			},
			{
				name:    "invalid JSON",
				payload: "invalid json",
				wantErr: "invalid payload format",
			},
			{
				name:    "invalid format",
				payload: map[string]interface{}{"format": "invalid"},
				wantErr: "start_date and end_date are required", // It checks dates first
			},
			{
				name: "missing dates",
				payload: map[string]interface{}{
					"format":      "pdf",
					"report_type": "monthly",
				},
				wantErr: "start_date and end_date are required",
			},
			{
				name: "invalid date range",
				payload: map[string]interface{}{
					"start_date":  "2024-01-01T00:00:00Z",
					"end_date":    "2023-01-01T00:00:00Z",
					"format":      "pdf",
					"report_type": "monthly",
				},
				wantErr: "start_date must be before end_date",
			},
			{
				name: "valid request",
				payload: map[string]interface{}{
					"start_date":     "2023-01-01T00:00:00Z",
					"end_date":       "2024-01-01T00:00:00Z",
					"format":         "pdf",
					"report_type":    "monthly",
					"include_charts": true,
				},
				wantErr: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var payloadBytes []byte
				if tt.payload != nil {
					payloadBytes, _ = json.Marshal(tt.payload)
				}

				req := &streamer.Request{
					Payload: payloadBytes,
				}

				err := handler.Validate(req)
				if tt.wantErr != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.wantErr)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("DataProcessingHandler validation", func(t *testing.T) {
		handler := NewDataProcessingHandler()

		// Test EstimatedDuration
		assert.Equal(t, 5*time.Minute, handler.EstimatedDuration())

		tests := []struct {
			name    string
			payload interface{}
			wantErr string
		}{
			{
				name:    "nil payload",
				payload: nil,
				wantErr: "payload is required",
			},
			{
				name:    "invalid JSON",
				payload: "not json",
				wantErr: "invalid payload format",
			},
			{
				name: "missing dataset_id",
				payload: map[string]interface{}{
					"operations": []string{"filter"},
				},
				wantErr: "dataset_id is required",
			},
			{
				name: "empty operations",
				payload: map[string]interface{}{
					"dataset_id": "data-123",
					"operations": []string{},
				},
				wantErr: "at least one operation is required",
			},
			{
				name: "invalid operation",
				payload: map[string]interface{}{
					"dataset_id":    "data-123",
					"operations":    []string{"invalid_op"},
					"output_format": "json",
				},
				wantErr: "invalid operation: invalid_op",
			},
			{
				name: "invalid output format",
				payload: map[string]interface{}{
					"dataset_id":    "data-123",
					"operations":    []string{"filter"},
					"output_format": "xml",
				},
				wantErr: "invalid output_format: xml",
			},
			{
				name: "valid request",
				payload: map[string]interface{}{
					"dataset_id":    "data-123",
					"operations":    []string{"filter", "transform", "aggregate"},
					"output_format": "parquet",
				},
				wantErr: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := &streamer.Request{}
				if tt.payload != nil {
					if str, ok := tt.payload.(string); ok {
						req.Payload = []byte(str)
					} else {
						payload, _ := json.Marshal(tt.payload)
						req.Payload = payload
					}
				}

				err := handler.Validate(req)
				if tt.wantErr != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.wantErr)
				} else {
					assert.NoError(t, err)
				}

				// Test Process method (should return error for async)
				_, err = handler.Process(context.Background(), req)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "use ProcessWithProgress")
			})
		}
	})

	t.Run("BulkHandler validation", func(t *testing.T) {
		handler := NewBulkHandler()

		// Test EstimatedDuration
		assert.Equal(t, 10*time.Minute, handler.EstimatedDuration())

		tests := []struct {
			name    string
			payload interface{}
			wantErr string
		}{
			{
				name:    "nil payload",
				payload: nil,
				wantErr: "payload is required",
			},
			{
				name:    "invalid JSON",
				payload: "bad json",
				wantErr: "invalid payload format",
			},
			{
				name: "invalid operation type",
				payload: map[string]interface{}{
					"operation_type": "invalid",
					"entity_type":    "user",
					"items":          []map[string]interface{}{{"id": 1}},
				},
				wantErr: "invalid operation_type: invalid",
			},
			{
				name: "invalid entity type",
				payload: map[string]interface{}{
					"operation_type": "create",
					"entity_type":    "invalid",
					"items":          []map[string]interface{}{{"id": 1}},
				},
				wantErr: "invalid entity_type: invalid",
			},
			{
				name: "empty items",
				payload: map[string]interface{}{
					"operation_type": "create",
					"entity_type":    "user",
					"items":          []map[string]interface{}{},
				},
				wantErr: "items cannot be empty",
			},
			{
				name: "too many items",
				payload: map[string]interface{}{
					"operation_type": "create",
					"entity_type":    "user",
					"items":          make([]map[string]interface{}, 10001),
				},
				wantErr: "too many items",
			},
			{
				name: "batch size too large",
				payload: map[string]interface{}{
					"operation_type": "create",
					"entity_type":    "user",
					"items":          []map[string]interface{}{{"id": 1}},
					"batch_size":     101,
				},
				wantErr: "batch_size too large",
			},
			{
				name: "valid request with default batch size",
				payload: map[string]interface{}{
					"operation_type": "update",
					"entity_type":    "product",
					"items":          []map[string]interface{}{{"id": 1}, {"id": 2}},
				},
				wantErr: "",
			},
			{
				name: "valid request with custom batch size",
				payload: map[string]interface{}{
					"operation_type": "delete",
					"entity_type":    "order",
					"items":          []map[string]interface{}{{"id": 1}},
					"batch_size":     50,
				},
				wantErr: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := &streamer.Request{}
				if tt.payload != nil {
					if str, ok := tt.payload.(string); ok {
						req.Payload = []byte(str)
					} else {
						payload, _ := json.Marshal(tt.payload)
						req.Payload = payload
					}
				}

				err := handler.Validate(req)
				if tt.wantErr != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.wantErr)
				} else {
					assert.NoError(t, err)
				}

				// Test Process method (should return error for async)
				_, err = handler.Process(context.Background(), req)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "use ProcessWithProgress")
			})
		}
	})
}

// TestLambdaHandler tests the main Lambda handler function
func TestLambdaHandler(t *testing.T) {
	// Mock stores and managers
	mockReqStore := new(mockRequestStore)
	mockConnMgr := new(mockConnectionManager)

	// Create a test router
	testRouter := streamer.NewRouter(mockReqStore, mockConnMgr)
	testRouter.SetAsyncThreshold(5 * time.Second)

	// Set the global router for testing
	oldRouter := router
	router = testRouter
	defer func() { router = oldRouter }()

	// Register test handlers
	testRouter.Handle("test", &testHandler{
		duration: 100 * time.Millisecond,
		processFunc: func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
			return &streamer.Result{Success: true}, nil
		},
	})

	tests := []struct {
		name         string
		event        events.APIGatewayWebsocketProxyRequest
		setupMocks   func()
		wantStatus   int
		wantResponse string
	}{
		{
			name: "successful request",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-123",
					RouteKey:     "$default",
				},
				Body: `{"action": "test"}`,
			},
			setupMocks: func() {
				mockConnMgr.On("Send", mock.Anything, "conn-123", mock.Anything).Return(nil)
			},
			wantStatus: 200,
		},
		{
			name: "request with authorizer context",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-auth",
					Authorizer: map[string]interface{}{
						"userId":   "user-123",
						"tenantId": "tenant-456",
					},
				},
				Body: `{"action": "test"}`,
			},
			setupMocks: func() {
				mockConnMgr.On("Send", mock.Anything, "conn-auth", mock.Anything).Return(nil)
			},
			wantStatus: 200,
		},
		{
			name: "error in routing",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "conn-error",
				},
				Body: `{"action": "test"}`,
			},
			setupMocks: func() {
				// Mock Send to return an error, which will cause Route to fail
				mockConnMgr.On("Send", mock.Anything, "conn-error", mock.Anything).Return(errors.New("connection error"))
			},
			wantStatus:   500,
			wantResponse: "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockReqStore.ExpectedCalls = nil
			mockConnMgr.ExpectedCalls = nil

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			// Call handler
			response, err := handler(context.Background(), tt.event)

			// Check results
			assert.NoError(t, err)
			assert.Equal(t, tt.wantStatus, response.StatusCode)
			if tt.wantResponse != "" {
				assert.Equal(t, tt.wantResponse, response.Body)
			}

			// Verify mocks
			mockReqStore.AssertExpectations(t)
			mockConnMgr.AssertExpectations(t)
		})
	}
}

// TestReportHandlerEdgeCases tests edge cases for ReportHandler validation
func TestReportHandlerEdgeCases(t *testing.T) {
	handler := NewReportHandler()

	// Test invalid report type
	req := &streamer.Request{
		Payload: []byte(`{
			"start_date": "2023-01-01T00:00:00Z",
			"end_date": "2024-01-01T00:00:00Z",
			"format": "pdf",
			"report_type": "invalid_type"
		}`),
	}

	err := handler.Validate(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid report_type")

	// Test Process method
	_, err = handler.Process(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "use ProcessWithProgress")
}
