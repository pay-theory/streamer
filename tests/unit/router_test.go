package unit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/pay-theory/streamer/pkg/streamer"
)

// MockRequestStore implements the RequestStore interface for testing
type MockRequestStore struct {
	EnqueueFunc func(ctx context.Context, request *streamer.Request) error
	Requests    []*streamer.Request
}

func (m *MockRequestStore) Enqueue(ctx context.Context, request *streamer.Request) error {
	if m.EnqueueFunc != nil {
		return m.EnqueueFunc(ctx, request)
	}
	m.Requests = append(m.Requests, request)
	return nil
}

// MockConnectionManager implements the ConnectionManager interface for testing
type MockConnectionManager struct {
	SendFunc func(ctx context.Context, connectionID string, message interface{}) error
	Messages map[string][]interface{}
}

func NewMockConnectionManager() *MockConnectionManager {
	return &MockConnectionManager{
		Messages: make(map[string][]interface{}),
	}
}

func (m *MockConnectionManager) Send(ctx context.Context, connectionID string, message interface{}) error {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, connectionID, message)
	}
	m.Messages[connectionID] = append(m.Messages[connectionID], message)
	return nil
}

// Test basic router functionality
func TestRouter_HandleAndRoute(t *testing.T) {
	store := &MockRequestStore{}
	connManager := NewMockConnectionManager()
	router := streamer.NewRouter(store, connManager)

	// Register a simple handler
	echoHandler := streamer.NewEchoHandler()
	err := router.Handle("echo", echoHandler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Create a WebSocket event
	payload := map[string]interface{}{
		"message": "Hello, World!",
	}
	payloadBytes, _ := json.Marshal(payload)

	body := map[string]interface{}{
		"action":  "echo",
		"id":      "test-123",
		"payload": json.RawMessage(payloadBytes),
	}
	bodyBytes, _ := json.Marshal(body)

	event := events.APIGatewayWebsocketProxyRequest{
		Body: string(bodyBytes),
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-123",
		},
	}

	// Route the request
	ctx := context.Background()
	err = router.Route(ctx, event)
	if err != nil {
		t.Fatalf("Failed to route request: %v", err)
	}

	// Check that response was sent
	messages := connManager.Messages["conn-123"]
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	// Verify response structure
	response, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response to be a map")
	}

	if response["type"] != "response" {
		t.Errorf("Expected response type, got %v", response["type"])
	}

	if response["request_id"] != "test-123" {
		t.Errorf("Expected request_id test-123, got %v", response["request_id"])
	}

	if response["success"] != true {
		t.Errorf("Expected success to be true")
	}
}

// Test async request handling
func TestRouter_AsyncRequest(t *testing.T) {
	store := &MockRequestStore{}
	connManager := NewMockConnectionManager()
	router := streamer.NewRouter(store, connManager)

	// Set a low async threshold
	router.SetAsyncThreshold(100 * time.Millisecond)

	// Register a slow handler
	slowHandler := streamer.NewDelayHandler(5 * time.Second)
	err := router.Handle("slow-operation", slowHandler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Create request
	body := map[string]interface{}{
		"action": "slow-operation",
		"id":     "async-123",
	}
	bodyBytes, _ := json.Marshal(body)

	event := events.APIGatewayWebsocketProxyRequest{
		Body: string(bodyBytes),
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-456",
		},
	}

	// Route the request
	ctx := context.Background()
	err = router.Route(ctx, event)
	if err != nil {
		t.Fatalf("Failed to route request: %v", err)
	}

	// Check that request was queued
	if len(store.Requests) != 1 {
		t.Fatalf("Expected 1 queued request, got %d", len(store.Requests))
	}

	queuedRequest := store.Requests[0]
	if queuedRequest.ID != "async-123" {
		t.Errorf("Expected request ID async-123, got %s", queuedRequest.ID)
	}

	// Check acknowledgment was sent
	messages := connManager.Messages["conn-456"]
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	ack, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected acknowledgment to be a map")
	}

	if ack["type"] != "acknowledgment" {
		t.Errorf("Expected acknowledgment type, got %v", ack["type"])
	}

	if ack["status"] != "queued" {
		t.Errorf("Expected status queued, got %v", ack["status"])
	}
}

// Test validation
func TestRouter_Validation(t *testing.T) {
	store := &MockRequestStore{}
	connManager := NewMockConnectionManager()
	router := streamer.NewRouter(store, connManager)

	// Register validation handler
	validationHandler := streamer.NewValidationExampleHandler()
	err := router.Handle("create-user", validationHandler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	tests := []struct {
		name        string
		payload     interface{}
		shouldError bool
		errorCode   string
	}{
		{
			name: "valid payload",
			payload: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
			shouldError: false,
		},
		{
			name:        "missing payload",
			payload:     nil,
			shouldError: true,
			errorCode:   streamer.ErrCodeValidation,
		},
		{
			name: "missing required field",
			payload: map[string]interface{}{
				"name": "John Doe",
			},
			shouldError: true,
			errorCode:   streamer.ErrCodeValidation,
		},
		{
			name: "invalid email type",
			payload: map[string]interface{}{
				"name":  "John Doe",
				"email": 123,
			},
			shouldError: true,
			errorCode:   streamer.ErrCodeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]interface{}{
				"action": "create-user",
				"id":     fmt.Sprintf("test-%s", tt.name),
			}

			if tt.payload != nil {
				payloadBytes, _ := json.Marshal(tt.payload)
				body["payload"] = json.RawMessage(payloadBytes)
			}

			bodyBytes, _ := json.Marshal(body)

			event := events.APIGatewayWebsocketProxyRequest{
				Body: string(bodyBytes),
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: fmt.Sprintf("conn-%s", tt.name),
				},
			}

			ctx := context.Background()
			err := router.Route(ctx, event)
			if err != nil {
				t.Fatalf("Route failed: %v", err)
			}

			messages := connManager.Messages[fmt.Sprintf("conn-%s", tt.name)]
			if len(messages) != 1 {
				t.Fatalf("Expected 1 message, got %d", len(messages))
			}

			response := messages[0].(map[string]interface{})

			if tt.shouldError {
				if response["type"] != "error" {
					t.Errorf("Expected error response, got %v", response["type"])
				}

				if errorData, ok := response["error"].(*streamer.Error); ok {
					if errorData.Code != tt.errorCode {
						t.Errorf("Expected error code %s, got %s", tt.errorCode, errorData.Code)
					}
				}
			} else {
				if response["type"] != "response" {
					t.Errorf("Expected response type, got %v", response["type"])
				}

				if response["success"] != true {
					t.Errorf("Expected success to be true")
				}
			}
		})
	}
}

// Test middleware
func TestRouter_Middleware(t *testing.T) {
	store := &MockRequestStore{}
	connManager := NewMockConnectionManager()
	router := streamer.NewRouter(store, connManager)

	// Track middleware execution
	var executionOrder []string

	// Add middleware
	middleware1 := func(next streamer.Handler) streamer.Handler {
		return streamer.NewHandlerFunc(
			func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
				executionOrder = append(executionOrder, "middleware1-before")
				result, err := next.Process(ctx, req)
				executionOrder = append(executionOrder, "middleware1-after")
				return result, err
			},
			next.EstimatedDuration(),
			next.Validate,
		)
	}

	middleware2 := func(next streamer.Handler) streamer.Handler {
		return streamer.NewHandlerFunc(
			func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
				executionOrder = append(executionOrder, "middleware2-before")
				result, err := next.Process(ctx, req)
				executionOrder = append(executionOrder, "middleware2-after")
				return result, err
			},
			next.EstimatedDuration(),
			next.Validate,
		)
	}

	router.SetMiddleware(middleware1, middleware2)

	// Register handler
	handler := streamer.SimpleHandler("test", func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
		executionOrder = append(executionOrder, "handler")
		return &streamer.Result{
			RequestID: req.ID,
			Success:   true,
			Data:      "test",
		}, nil
	})

	err := router.Handle("test", handler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Execute request
	body := map[string]interface{}{
		"action": "test",
		"id":     "middleware-test",
	}
	bodyBytes, _ := json.Marshal(body)

	event := events.APIGatewayWebsocketProxyRequest{
		Body: string(bodyBytes),
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-middleware",
		},
	}

	ctx := context.Background()
	err = router.Route(ctx, event)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Verify execution order
	expected := []string{
		"middleware1-before",
		"middleware2-before",
		"handler",
		"middleware2-after",
		"middleware1-after",
	}

	if len(executionOrder) != len(expected) {
		t.Fatalf("Expected %d executions, got %d", len(expected), len(executionOrder))
	}

	for i, exec := range executionOrder {
		if exec != expected[i] {
			t.Errorf("Expected execution[%d] to be %s, got %s", i, expected[i], exec)
		}
	}
}

// Test error handling
func TestRouter_ErrorHandling(t *testing.T) {
	store := &MockRequestStore{}
	connManager := NewMockConnectionManager()
	router := streamer.NewRouter(store, connManager)

	// Register handler that returns an error
	errorHandler := streamer.SimpleHandler("error", func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
		return nil, errors.New("something went wrong")
	})

	err := router.Handle("error", errorHandler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Execute request
	body := map[string]interface{}{
		"action": "error",
		"id":     "error-test",
	}
	bodyBytes, _ := json.Marshal(body)

	event := events.APIGatewayWebsocketProxyRequest{
		Body: string(bodyBytes),
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-error",
		},
	}

	ctx := context.Background()
	err = router.Route(ctx, event)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Check error response
	messages := connManager.Messages["conn-error"]
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	response := messages[0].(map[string]interface{})
	if response["type"] != "error" {
		t.Errorf("Expected error type, got %v", response["type"])
	}

	if errorData, ok := response["error"].(*streamer.Error); ok {
		if errorData.Code != streamer.ErrCodeInternalError {
			t.Errorf("Expected internal error code, got %s", errorData.Code)
		}
		if errorData.Message != "something went wrong" {
			t.Errorf("Expected error message 'something went wrong', got %s", errorData.Message)
		}
	}
}

// Test invalid action
func TestRouter_InvalidAction(t *testing.T) {
	store := &MockRequestStore{}
	connManager := NewMockConnectionManager()
	router := streamer.NewRouter(store, connManager)

	// Execute request with unregistered action
	body := map[string]interface{}{
		"action": "unknown-action",
		"id":     "invalid-action-test",
	}
	bodyBytes, _ := json.Marshal(body)

	event := events.APIGatewayWebsocketProxyRequest{
		Body: string(bodyBytes),
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "conn-invalid",
		},
	}

	ctx := context.Background()
	err := router.Route(ctx, event)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// Check error response
	messages := connManager.Messages["conn-invalid"]
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	response := messages[0].(map[string]interface{})
	if response["type"] != "error" {
		t.Errorf("Expected error type, got %v", response["type"])
	}

	if errorData, ok := response["error"].(*streamer.Error); ok {
		if errorData.Code != streamer.ErrCodeInvalidAction {
			t.Errorf("Expected invalid action error code, got %s", errorData.Code)
		}
	}
}
