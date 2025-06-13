package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestStorageIntegration tests the integration between storage components using DynamORM mocks
func TestStorageIntegration(t *testing.T) {
	ctx := context.Background()

	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Create DynamORM mock
	mockDB := new(mocks.DB)
	mockQueryForConnection := new(mocks.Query)
	mockQueryForRequest := new(mocks.Query)

	// Create storage components using DynamORM implementations
	requestQueue := dynamorm.NewRequestQueue(mockDB)
	connectionStore := dynamorm.NewConnectionStore(mockDB)

	// Create a test connection manager
	connManager := connection.NewMockConnectionManager()

	// Track sent messages
	var sentMessages []interface{}
	connManager.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
		sentMessages = append(sentMessages, message)
		return nil
	}

	// Create a test connection
	conn := &store.Connection{
		ConnectionID: "test-conn-123",
		UserID:       "user-456",
		TenantID:     "tenant-789",
		Endpoint:     "https://test.execute-api.us-east-1.amazonaws.com/test",
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
	}

	// Set up mock expectations for saving connection
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQueryForConnection).Once()
	mockQueryForConnection.On("Create").Return(nil).Once()

	// Save connection
	err := connectionStore.Save(ctx, conn)
	assert.NoError(t, err, "Failed to save connection")

	// Create an async request
	request := &store.AsyncRequest{
		RequestID:    "req-123",
		ConnectionID: conn.ConnectionID,
		UserID:       conn.UserID,
		TenantID:     conn.TenantID,
		Action:       "test_action",
		Payload: map[string]interface{}{
			"message": "Hello, World!",
			"count":   42,
		},
		CreatedAt:  time.Now(),
		Status:     store.StatusPending,
		MaxRetries: 3,
	}

	// Set up mock expectations for queuing request
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQueryForRequest).Once()
	mockQueryForRequest.On("Create").Return(nil).Once()

	// Queue the request
	err = requestQueue.Enqueue(ctx, request)
	assert.NoError(t, err, "Failed to enqueue request")

	// Send acknowledgment through connection manager
	ack := map[string]interface{}{
		"type":       "acknowledgment",
		"request_id": request.RequestID,
		"status":     "queued",
		"message":    "Request has been queued for processing",
	}

	err = connManager.Send(ctx, conn.ConnectionID, ack)
	assert.NoError(t, err, "Failed to send acknowledgment")

	// Verify acknowledgment was sent
	assert.Len(t, sentMessages, 1, "Expected 1 message sent")

	// Check acknowledgment content
	ackMap, ok := sentMessages[0].(map[string]interface{})
	assert.True(t, ok, "Expected acknowledgment to be a map")
	assert.Equal(t, "acknowledgment", ackMap["type"])
	assert.Equal(t, "queued", ackMap["status"])
	assert.Equal(t, request.RequestID, ackMap["request_id"])

	// Verify all expected calls were made
	mockDB.AssertExpectations(t)
	mockQueryForConnection.AssertExpectations(t)
	mockQueryForRequest.AssertExpectations(t)

	t.Log("Storage integration test completed successfully")
}
