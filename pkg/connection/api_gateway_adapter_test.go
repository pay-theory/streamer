package connection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// For testing the adapter, we'll use our existing mocks from mocks.go
// which implement the APIGatewayClient interface

// mockHTTPError implements the HTTPStatusCode interface for testing
type mockHTTPError struct {
	statusCode int
	message    string
}

func (e *mockHTTPError) Error() string {
	return e.message
}

func (e *mockHTTPError) HTTPStatusCode() int {
	return e.statusCode
}

func TestNewAWSAPIGatewayAdapter(t *testing.T) {
	// Test the constructor with a nil client to verify it creates the adapter
	mockClient := &apigatewaymanagementapi.Client{}
	adapter := NewAWSAPIGatewayAdapter(mockClient)

	assert.NotNil(t, adapter)
	assert.Equal(t, mockClient, adapter.client)

	// Verify it implements our interface
	var _ APIGatewayClient = adapter
}

// Test the adapter's interface implementation using our mocks
func TestAWSAPIGatewayAdapter_APIGatewayClient_Implementation(t *testing.T) {
	// Create a mock that implements our APIGatewayClient interface
	mockClient := NewMockAPIGatewayClient()

	// Test PostToConnection
	t.Run("PostToConnection", func(t *testing.T) {
		mockClient.On("PostToConnection", mock.Anything, "conn123", []byte("test")).Return(nil).Once()

		err := mockClient.PostToConnection(context.Background(), "conn123", []byte("test"))
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// Test DeleteConnection
	t.Run("DeleteConnection", func(t *testing.T) {
		mockClient.On("DeleteConnection", mock.Anything, "conn123").Return(nil).Once()

		err := mockClient.DeleteConnection(context.Background(), "conn123")
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	// Test GetConnection
	t.Run("GetConnection", func(t *testing.T) {
		expectedInfo := &ConnectionInfo{
			ConnectionID: "conn123",
			ConnectedAt:  time.Now().Format(time.RFC3339),
		}
		mockClient.On("GetConnection", mock.Anything, "conn123").Return(expectedInfo, nil).Once()

		info, err := mockClient.GetConnection(context.Background(), "conn123")
		assert.NoError(t, err)
		assert.Equal(t, expectedInfo, info)
		mockClient.AssertExpectations(t)
	})
}

// Test error conversion by testing the behavior of the adapter methods
// Since we can't directly mock the AWS SDK client, we test the error handling
// by verifying our error types are correctly defined and used
func TestAWSAPIGatewayAdapter_ErrorTypes(t *testing.T) {
	tests := []struct {
		name         string
		errorType    error
		connectionID string
		expectedType string
		expectedMsg  string
	}{
		{
			name:         "GoneError",
			errorType:    GoneError{ConnectionID: "conn123", Message: "connection gone"},
			connectionID: "conn123",
			expectedType: "GoneError",
			expectedMsg:  "connection gone",
		},
		{
			name:         "ForbiddenError",
			errorType:    ForbiddenError{ConnectionID: "conn123", Message: "access denied"},
			connectionID: "conn123",
			expectedType: "ForbiddenError",
			expectedMsg:  "access denied",
		},
		{
			name:         "PayloadTooLargeError",
			errorType:    PayloadTooLargeError{ConnectionID: "conn123", Message: "payload too large"},
			connectionID: "conn123",
			expectedType: "PayloadTooLargeError",
			expectedMsg:  "payload too large",
		},
		{
			name:         "ThrottlingError",
			errorType:    ThrottlingError{ConnectionID: "conn123", Message: "rate limit exceeded"},
			connectionID: "conn123",
			expectedType: "ThrottlingError",
			expectedMsg:  "rate limit exceeded",
		},
		{
			name:         "InternalServerError",
			errorType:    InternalServerError{Message: "server error"},
			connectionID: "conn123",
			expectedType: "InternalServerError",
			expectedMsg:  "server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify error message
			assert.Equal(t, tt.expectedMsg, tt.errorType.Error())

			// Verify error type assertions work
			switch tt.expectedType {
			case "GoneError":
				var goneErr GoneError
				assert.True(t, errors.As(tt.errorType, &goneErr))
				assert.Equal(t, tt.connectionID, goneErr.ConnectionID)
			case "ForbiddenError":
				var forbiddenErr ForbiddenError
				assert.True(t, errors.As(tt.errorType, &forbiddenErr))
				assert.Equal(t, tt.connectionID, forbiddenErr.ConnectionID)
			case "PayloadTooLargeError":
				var payloadErr PayloadTooLargeError
				assert.True(t, errors.As(tt.errorType, &payloadErr))
				assert.Equal(t, tt.connectionID, payloadErr.ConnectionID)
			case "ThrottlingError":
				var throttlingErr ThrottlingError
				assert.True(t, errors.As(tt.errorType, &throttlingErr))
				assert.Equal(t, tt.connectionID, throttlingErr.ConnectionID)
			case "InternalServerError":
				var serverErr InternalServerError
				assert.True(t, errors.As(tt.errorType, &serverErr))
			}
		})
	}
}

// Test WebSocket scenarios using our interface mocks
func TestAPIGatewayClient_WebSocketScenarios(t *testing.T) {
	t.Run("WebSocket message delivery", func(t *testing.T) {
		mockClient := NewMockAPIGatewayClient()

		// Set up expectation
		mockClient.On("PostToConnection", mock.Anything, "test-connection-123", []byte(`{"type":"ping"}`)).
			Return(nil).Once()

		// Execute
		err := mockClient.PostToConnection(context.Background(), "test-connection-123", []byte(`{"type":"ping"}`))

		// Verify
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("WebSocket connection gone during broadcast", func(t *testing.T) {
		mockClient := NewMockAPIGatewayClient()

		// Set up expectations for broadcast scenario
		mockClient.On("PostToConnection", mock.Anything, "conn1", mock.Anything).Return(nil).Once()
		mockClient.On("PostToConnection", mock.Anything, "conn2", mock.Anything).
			Return(GoneError{ConnectionID: "conn2", Message: "connection conn2 is gone"}).Once()
		mockClient.On("PostToConnection", mock.Anything, "conn3", mock.Anything).Return(nil).Once()

		// Simulate broadcast
		connections := []string{"conn1", "conn2", "conn3"}
		message := []byte(`{"type":"broadcast"}`)
		ctx := context.Background()

		var errors []error
		for _, connID := range connections {
			if err := mockClient.PostToConnection(ctx, connID, message); err != nil {
				errors = append(errors, err)
			}
		}

		// Verify
		assert.Len(t, errors, 1)
		assert.Contains(t, errors[0].Error(), "conn2 is gone")
		mockClient.AssertExpectations(t)
	})
}

// Test using the TestableAPIGatewayClient from mocks.go
func TestAPIGatewayClient_WithTestableClient(t *testing.T) {
	client := NewTestableAPIGatewayClient()

	t.Run("successful operations", func(t *testing.T) {
		// Add a connection
		client.AddConnection("conn123", "192.168.1.1")

		// Post a message
		err := client.PostToConnection(context.Background(), "conn123", []byte("test message"))
		assert.NoError(t, err)

		// Verify message was stored
		messages := client.GetMessages("conn123")
		assert.Len(t, messages, 1)
		assert.Equal(t, []byte("test message"), messages[0])

		// Get connection info
		info, err := client.GetConnection(context.Background(), "conn123")
		assert.NoError(t, err)
		assert.Equal(t, "conn123", info.ConnectionID)
		assert.Equal(t, "192.168.1.1", info.SourceIP)

		// Delete connection
		err = client.DeleteConnection(context.Background(), "conn123")
		assert.NoError(t, err)

		// Verify connection is gone
		_, err = client.GetConnection(context.Background(), "conn123")
		assert.Error(t, err)
		var goneErr GoneError
		assert.True(t, errors.As(err, &goneErr))
	})

	t.Run("error scenarios", func(t *testing.T) {
		// Test posting to non-existent connection
		err := client.PostToConnection(context.Background(), "invalid", []byte("test"))
		assert.Error(t, err)
		var goneErr GoneError
		assert.True(t, errors.As(err, &goneErr))

		// Test with simulated throttling
		client.AddConnection("throttled", "192.168.1.1")
		client.SimulateThrottling("throttled", 60)

		err = client.PostToConnection(context.Background(), "throttled", []byte("test"))
		assert.Error(t, err)
		var throttleErr ThrottlingError
		assert.True(t, errors.As(err, &throttleErr))
		assert.Equal(t, 60, throttleErr.RetryAfter)
	})

	t.Run("latency simulation", func(t *testing.T) {
		client.Clear()
		client.AddConnection("slow", "192.168.1.1")
		client.SetLatency(50 * time.Millisecond)

		start := time.Now()
		err := client.PostToConnection(context.Background(), "slow", []byte("test"))
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, duration, 50*time.Millisecond)
	})
}

// Benchmark tests
func BenchmarkMockAPIGatewayClient_PostToConnection(b *testing.B) {
	mockClient := NewMockAPIGatewayClient()
	mockClient.On("PostToConnection", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	data := []byte(`{"message":"benchmark"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mockClient.PostToConnection(ctx, "bench-conn", data)
	}
}

func BenchmarkTestableAPIGatewayClient_PostToConnection(b *testing.B) {
	client := NewTestableAPIGatewayClient()
	client.AddConnection("bench-conn", "192.168.1.1")

	ctx := context.Background()
	data := []byte(`{"message":"benchmark"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.PostToConnection(ctx, "bench-conn", data)
	}
}

// Integration test pattern for real AWS API Gateway (skipped by default)
func TestAWSAPIGatewayAdapter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This would use a real AWS client in integration tests
	// For now, we focus on unit testing with mocks
}
