package connection

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAPIGatewayInterface tests the API Gateway interface implementations
func TestAPIGatewayInterface(t *testing.T) {
	t.Run("MockAPIGatewayClient", func(t *testing.T) {
		mock := NewMockAPIGatewayClient()

		// Test PostToConnection
		mock.On("PostToConnection", context.Background(), "conn123", []byte(`{"test":"data"}`)).Return(nil)

		err := mock.PostToConnection(context.Background(), "conn123", []byte(`{"test":"data"}`))
		assert.NoError(t, err)

		messages := mock.GetMessages("conn123")
		assert.Len(t, messages, 1)
		assert.Equal(t, []byte(`{"test":"data"}`), messages[0])

		mock.AssertExpectations(t)
	})

	t.Run("TestableAPIGatewayClient", func(t *testing.T) {
		client := NewTestableAPIGatewayClient()

		// Add a connection
		client.AddConnection("test-conn", "127.0.0.1")

		// Test successful send
		err := client.PostToConnection(context.Background(), "test-conn", []byte(`{"message":"hello"}`))
		assert.NoError(t, err)

		messages := client.GetMessages("test-conn")
		assert.Len(t, messages, 1)

		// Test connection not found
		err = client.PostToConnection(context.Background(), "missing", []byte(`{"message":"hello"}`))
		assert.Error(t, err)
		var goneErr GoneError
		assert.ErrorAs(t, err, &goneErr)

		// Test with configured error
		client.SetError("test-conn", ForbiddenError{ConnectionID: "test-conn", Message: "access denied"})
		err = client.PostToConnection(context.Background(), "test-conn", []byte(`{"message":"hello"}`))
		assert.Error(t, err)
		var forbiddenErr ForbiddenError
		assert.ErrorAs(t, err, &forbiddenErr)
	})
}

// TestAWSAPIGatewayAdapter tests the adapter implementation
func TestAWSAPIGatewayAdapter(t *testing.T) {
	// This test would require mocking the AWS SDK client
	// For now, we'll just verify the adapter implements the interface
	var _ APIGatewayClient = (*AWSAPIGatewayAdapter)(nil)
}

// TestErrorTypes tests our custom error types
func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name         string
		err          APIError
		expectedCode int
		expectedName string
		isRetryable  bool
	}{
		{
			name:         "GoneError",
			err:          GoneError{ConnectionID: "test", Message: "connection gone"},
			expectedCode: 410,
			expectedName: "GoneException",
			isRetryable:  false,
		},
		{
			name:         "ForbiddenError",
			err:          ForbiddenError{ConnectionID: "test", Message: "forbidden"},
			expectedCode: 403,
			expectedName: "ForbiddenException",
			isRetryable:  false,
		},
		{
			name:         "PayloadTooLargeError",
			err:          PayloadTooLargeError{ConnectionID: "test", Message: "too large"},
			expectedCode: 413,
			expectedName: "PayloadTooLargeException",
			isRetryable:  false,
		},
		{
			name:         "ThrottlingError",
			err:          ThrottlingError{ConnectionID: "test", Message: "throttled"},
			expectedCode: 429,
			expectedName: "ThrottlingException",
			isRetryable:  true,
		},
		{
			name:         "InternalServerError",
			err:          InternalServerError{Message: "server error"},
			expectedCode: 500,
			expectedName: "InternalServerError",
			isRetryable:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedCode, tt.err.HTTPStatusCode())
			assert.Equal(t, tt.expectedName, tt.err.ErrorCode())
			assert.Equal(t, tt.isRetryable, tt.err.IsRetryable())
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

// TestMockUsageExample demonstrates how to use the mocks in tests
func TestMockUsageExample(t *testing.T) {
	// Create a testable API Gateway client
	apiClient := NewTestableAPIGatewayClient()
	apiClient.AddConnection("user123-conn", "192.168.1.1")

	// Create a mock store (simplified for this example)
	type mockStore struct{}

	// This would normally be a full mock implementation
	// For this example, we're just showing the pattern

	// Test sending a message
	ctx := context.Background()
	message := []byte(`{"type":"notification","content":"Hello, World!"}`)

	err := apiClient.PostToConnection(ctx, "user123-conn", message)
	assert.NoError(t, err)

	// Verify the message was recorded
	sentMessages := apiClient.GetMessages("user123-conn")
	assert.Len(t, sentMessages, 1)
	assert.Equal(t, message, sentMessages[0])

	// Test error scenarios
	apiClient.SetError("user123-conn", ThrottlingError{
		ConnectionID: "user123-conn",
		RetryAfter:   5,
		Message:      "Rate limit exceeded",
	})

	err = apiClient.PostToConnection(ctx, "user123-conn", message)
	assert.Error(t, err)

	var throttleErr ThrottlingError
	assert.ErrorAs(t, err, &throttleErr)
	assert.Equal(t, 5, throttleErr.RetryAfter)
}

// TestLatencySimulation tests the latency simulation feature
func TestLatencySimulation(t *testing.T) {
	client := NewTestableAPIGatewayClient()
	client.AddConnection("latency-test", "127.0.0.1")
	client.SetLatency(100 * time.Millisecond)

	start := time.Now()
	err := client.PostToConnection(context.Background(), "latency-test", []byte("test"))
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, duration >= 100*time.Millisecond, "Expected at least 100ms latency, got %v", duration)
}

// TestConcurrentMockAccess tests thread safety of mocks
func TestConcurrentMockAccess(t *testing.T) {
	client := NewTestableAPIGatewayClient()

	// Add connections
	for i := 0; i < 10; i++ {
		client.AddConnection(fmt.Sprintf("conn%d", i), "127.0.0.1")
	}

	// Concurrent sends
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			connID := fmt.Sprintf("conn%d", idx)
			message := []byte(fmt.Sprintf(`{"index":%d}`, idx))

			err := client.PostToConnection(context.Background(), connID, message)
			assert.NoError(t, err)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all messages were sent
	for i := 0; i < 10; i++ {
		connID := fmt.Sprintf("conn%d", i)
		messages := client.GetMessages(connID)
		assert.Len(t, messages, 1)
	}
}
