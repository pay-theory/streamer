package connection_test

import (
	"context"
	"errors"
	"testing"

	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/progress"
	"github.com/pay-theory/streamer/pkg/streamer"
	"github.com/stretchr/testify/assert"
)

// Example 1: Using SendOnlyMock with pkg/streamer (which only needs Send)
func TestWithStreamerPackage(t *testing.T) {
	// Create mock
	mock := connection.NewSendOnlyMock()

	// Configure custom behavior if needed
	mock.SendFunc = func(ctx context.Context, connectionID string, message interface{}) error {
		// Custom validation
		if connectionID == "invalid" {
			return errors.New("invalid connection")
		}
		// Let default behavior handle storage
		mock.Messages[connectionID] = append(mock.Messages[connectionID], message)
		return nil
	}

	// Use with router (pkg/streamer only needs Send method)
	store := &mockRequestStore{}
	router := streamer.NewRouter(store, mock) // Works! mock implements Send

	// Register handler and route message
	router.Handle("test", streamer.NewEchoHandler())

	// Verify messages were sent
	messages := mock.GetMessages("conn-123")
	assert.Len(t, messages, 0) // No messages yet

	// Test error case
	err := mock.Send(context.Background(), "invalid", "test")
	assert.Error(t, err)
}

// Example 2: Using ProgressReporterMock with pkg/progress (needs Send + IsActive)
func TestWithProgressPackage(t *testing.T) {
	// Create mock
	mock := connection.NewProgressReporterMock()

	// Set up active connections
	mock.SetActive("conn-123", true)
	mock.SetActive("conn-456", false)

	// Create progress reporter
	reporter := progress.NewReporter("req-123", "conn-123", mock)

	// Report progress
	err := reporter.Report(50.0, "Processing...")
	assert.NoError(t, err)

	// Verify message was sent
	messages := mock.GetMessages("conn-123")
	assert.Len(t, messages, 1)

	progressMsg := messages[0].(map[string]interface{})
	assert.Equal(t, "progress", progressMsg["type"])
	assert.Equal(t, 50.0, progressMsg["percentage"])

	// Test with inactive connection
	reporter2 := progress.NewReporter("req-456", "conn-456", mock)
	err = reporter2.Report(25.0, "Should not send")
	assert.NoError(t, err) // Progress reporter doesn't return error for inactive

	// Verify no message was sent to inactive connection
	messages2 := mock.GetMessages("conn-456")
	assert.Len(t, messages2, 0)
}

// Example 3: Using RecordingMock for detailed interaction testing
func TestWithRecordingMock(t *testing.T) {
	mock := connection.NewRecordingMock()

	ctx := context.Background()

	// Perform operations
	mock.IsActive(ctx, "conn-1")
	mock.Send(ctx, "conn-1", "message 1")
	mock.IsActive(ctx, "conn-2")
	mock.Send(ctx, "conn-1", "message 2")
	mock.IsActive(ctx, "conn-1")

	// Verify IsActive was called correctly
	isActiveCalls := mock.GetIsActiveCalls()
	assert.Equal(t, []string{"conn-1", "conn-2", "conn-1"}, isActiveCalls)

	// Verify messages
	conn1Messages := mock.GetMessages("conn-1")
	assert.Len(t, conn1Messages, 2)
	assert.Equal(t, "message 1", conn1Messages[0])
	assert.Equal(t, "message 2", conn1Messages[1])
}

// Example 4: Using FailingMock for error handling tests
func TestErrorHandling(t *testing.T) {
	// Create a mock that always fails
	mock := connection.NewFailingMock(errors.New("network error"))

	// Test with progress reporter
	reporter := progress.NewReporter("req-123", "conn-123", mock)

	// Report should handle the error gracefully
	err := reporter.Report(50.0, "This will fail")
	assert.NoError(t, err) // Progress reporter swallows send errors

	// Direct send should return error
	err = mock.Send(context.Background(), "any-conn", "any-message")
	assert.Error(t, err)
	assert.Equal(t, "network error", err.Error())
}

// Example 5: Switching between mocks for different test scenarios
func TestDifferentScenarios(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		mock := connection.NewProgressReporterMock()
		mock.SetActive("conn-123", true)

		reporter := progress.NewReporter("req-123", "conn-123", mock)
		err := reporter.Report(100.0, "Complete")
		assert.NoError(t, err)

		messages := mock.GetMessages("conn-123")
		assert.Len(t, messages, 1)
	})

	t.Run("connection gone", func(t *testing.T) {
		mock := connection.NewProgressReporterMock()
		mock.IsActiveFunc = func(ctx context.Context, connectionID string) bool {
			return false // All connections are inactive
		}

		reporter := progress.NewReporter("req-123", "conn-123", mock)
		err := reporter.Report(50.0, "Should not send")
		assert.NoError(t, err)

		// No messages should be sent
		assert.Len(t, mock.Messages, 0)
	})

	t.Run("network errors", func(t *testing.T) {
		mock := connection.NewFailingMock(errors.New("connection timeout"))

		err := mock.Send(context.Background(), "conn-123", "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})
}

// mockRequestStore for router tests
type mockRequestStore struct {
	requests []*streamer.Request
}

func (m *mockRequestStore) Enqueue(ctx context.Context, request *streamer.Request) error {
	m.requests = append(m.requests, request)
	return nil
}
