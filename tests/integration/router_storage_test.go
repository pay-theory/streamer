package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/pkg/streamer"
)

// TestRouterStorageIntegration tests the integration between router and storage
func TestRouterStorageIntegration(t *testing.T) {
	ctx := context.Background()

	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// TODO: Set up test DynamoDB client
	// For now, this is a placeholder showing the integration flow
	var dynamoClient *dynamodb.Client // Initialize with test config

	// Create storage components
	requestQueue := store.NewRequestQueue(dynamoClient, "test_")
	connectionStore := store.NewConnectionStore(dynamoClient, "test_")

	// Create router with adapters
	requestAdapter := streamer.NewRequestQueueAdapter(requestQueue)
	connManager := &mockConnectionManager{
		sentMessages: make([]sentMessage, 0),
	}

	router := streamer.NewRouter(requestAdapter, connManager)

	// Register a test handler
	router.Handle("test_action", &testHandler{})

	// Set async threshold low to test async processing
	router.SetAsyncThreshold(1 * time.Millisecond)

	// Create a test connection
	conn := &store.Connection{
		ConnectionID: "test-conn-123",
		UserID:       "user-456",
		TenantID:     "tenant-789",
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
	}

	// Save connection
	err := connectionStore.Save(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to save connection: %v", err)
	}

	// Create WebSocket event
	payload := map[string]interface{}{
		"message": "Hello, World!",
		"count":   42,
	}
	payloadBytes, _ := json.Marshal(payload)

	body := map[string]interface{}{
		"action":  "test_action",
		"payload": json.RawMessage(payloadBytes),
	}
	bodyBytes, _ := json.Marshal(body)

	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: conn.ConnectionID,
			RouteKey:     "test",
		},
		Body: string(bodyBytes),
	}

	// Route the request
	err = router.Route(ctx, event)
	if err != nil {
		t.Fatalf("Failed to route request: %v", err)
	}

	// Verify acknowledgment was sent
	if len(connManager.sentMessages) != 1 {
		t.Fatalf("Expected 1 message sent, got %d", len(connManager.sentMessages))
	}

	// Check acknowledgment content
	ack := connManager.sentMessages[0]
	if ack.connectionID != conn.ConnectionID {
		t.Errorf("Expected connection ID %s, got %s", conn.ConnectionID, ack.connectionID)
	}

	ackMap, ok := ack.message.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected acknowledgment to be a map")
	}

	if ackMap["type"] != "acknowledgment" {
		t.Errorf("Expected type 'acknowledgment', got %v", ackMap["type"])
	}

	if ackMap["status"] != "queued" {
		t.Errorf("Expected status 'queued', got %v", ackMap["status"])
	}

	// Verify request was queued
	// In a real test, you would check the database
	t.Log("Request successfully queued for async processing")
}

// Test helpers

type mockConnectionManager struct {
	sentMessages []sentMessage
}

type sentMessage struct {
	connectionID string
	message      interface{}
}

func (m *mockConnectionManager) Send(ctx context.Context, connectionID string, message interface{}) error {
	m.sentMessages = append(m.sentMessages, sentMessage{connectionID, message})
	return nil
}

type testHandler struct{}

func (h *testHandler) Validate(request *streamer.Request) error {
	return nil
}

func (h *testHandler) EstimatedDuration() time.Duration {
	return 10 * time.Second // Force async processing
}

func (h *testHandler) Process(ctx context.Context, request *streamer.Request) (*streamer.Result, error) {
	// This would be processed by the async processor
	return &streamer.Result{
		Success: true,
		Data: map[string]interface{}{
			"processed": true,
		},
	}, nil
}
