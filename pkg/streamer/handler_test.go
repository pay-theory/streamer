package streamer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseHandler(t *testing.T) {
	t.Run("EstimatedDuration", func(t *testing.T) {
		handler := &BaseHandler{
			estimatedDuration: 5 * time.Second,
		}
		assert.Equal(t, 5*time.Second, handler.EstimatedDuration())
	})

	t.Run("Validate without validator", func(t *testing.T) {
		handler := &BaseHandler{}
		err := handler.Validate(&Request{})
		assert.NoError(t, err)
	})

	t.Run("Validate with validator", func(t *testing.T) {
		handler := &BaseHandler{
			validator: func(req *Request) error {
				if req.Action == "" {
					return errors.New("action is required")
				}
				return nil
			},
		}

		// Test validation failure
		err := handler.Validate(&Request{Action: ""})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "action is required")

		// Test validation success
		err = handler.Validate(&Request{Action: "test"})
		assert.NoError(t, err)
	})
}

func TestHandlerFunc(t *testing.T) {
	t.Run("Process", func(t *testing.T) {
		called := false
		expectedResult := &Result{
			RequestID: "test-123",
			Success:   true,
			Data:      "test data",
		}

		handler := &HandlerFunc{
			ProcessFunc: func(ctx context.Context, req *Request) (*Result, error) {
				called = true
				assert.Equal(t, "test-123", req.ID)
				return expectedResult, nil
			},
		}

		ctx := context.Background()
		req := &Request{ID: "test-123"}
		result, err := handler.Process(ctx, req)

		assert.True(t, called)
		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("Process with error", func(t *testing.T) {
		handler := &HandlerFunc{
			ProcessFunc: func(ctx context.Context, req *Request) (*Result, error) {
				return nil, errors.New("processing failed")
			},
		}

		result, err := handler.Process(context.Background(), &Request{})
		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "processing failed")
	})
}

func TestNewHandlerFunc(t *testing.T) {
	processFunc := func(ctx context.Context, req *Request) (*Result, error) {
		return &Result{Success: true}, nil
	}
	duration := 100 * time.Millisecond
	validator := func(req *Request) error {
		if req.Action == "" {
			return errors.New("action required")
		}
		return nil
	}

	handler := NewHandlerFunc(processFunc, duration, validator)

	// Test that it implements Handler interface
	var _ Handler = handler

	// Test estimated duration
	assert.Equal(t, duration, handler.EstimatedDuration())

	// Test validation
	err := handler.Validate(&Request{Action: ""})
	assert.Error(t, err)

	err = handler.Validate(&Request{Action: "test"})
	assert.NoError(t, err)

	// Test process
	result, err := handler.Process(context.Background(), &Request{})
	assert.NoError(t, err)
	assert.True(t, result.Success)
}

func TestSimpleHandler(t *testing.T) {
	processFunc := func(ctx context.Context, req *Request) (*Result, error) {
		return &Result{
			RequestID: req.ID,
			Success:   true,
		}, nil
	}

	handler := SimpleHandler("test-handler", processFunc)

	// Test that it has default duration
	assert.Equal(t, 100*time.Millisecond, handler.EstimatedDuration())

	// Test that validation passes (no validator)
	err := handler.Validate(&Request{})
	assert.NoError(t, err)

	// Test process
	req := &Request{ID: "simple-123"}
	result, err := handler.Process(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "simple-123", result.RequestID)
	assert.True(t, result.Success)
}

func TestEchoHandler(t *testing.T) {
	handler := NewEchoHandler()

	t.Run("EstimatedDuration", func(t *testing.T) {
		assert.Equal(t, 10*time.Millisecond, handler.EstimatedDuration())
	})

	t.Run("Process with payload", func(t *testing.T) {
		payload := map[string]interface{}{
			"message": "hello",
			"number":  42,
		}
		payloadBytes, _ := json.Marshal(payload)

		req := &Request{
			ID:      "echo-123",
			Action:  "echo",
			Payload: payloadBytes,
		}

		result, err := handler.Process(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "echo-123", result.RequestID)
		assert.True(t, result.Success)

		data, ok := result.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "echo", data["action"])
		assert.NotNil(t, data["received_at"])

		// Check echoed payload
		echo, ok := data["echo"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "hello", echo["message"])
		assert.Equal(t, float64(42), echo["number"])
	})

	t.Run("Process without payload", func(t *testing.T) {
		req := &Request{
			ID:     "echo-456",
			Action: "echo",
		}

		result, err := handler.Process(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)

		data, ok := result.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Nil(t, data["echo"])
	})

	t.Run("Process with invalid JSON payload", func(t *testing.T) {
		req := &Request{
			ID:      "echo-789",
			Action:  "echo",
			Payload: []byte("invalid json"),
		}

		result, err := handler.Process(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to unmarshal payload")
	})
}

func TestDelayHandler(t *testing.T) {
	t.Run("NewDelayHandler", func(t *testing.T) {
		delay := 500 * time.Millisecond
		handler := NewDelayHandler(delay)

		assert.Equal(t, delay, handler.delay)
		assert.Equal(t, delay, handler.EstimatedDuration())
	})

	t.Run("Process completes after delay", func(t *testing.T) {
		delay := 50 * time.Millisecond
		handler := NewDelayHandler(delay)

		req := &Request{ID: "delay-123"}
		start := time.Now()

		result, err := handler.Process(context.Background(), req)

		elapsed := time.Since(start)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "delay-123", result.RequestID)
		assert.True(t, result.Success)
		assert.GreaterOrEqual(t, elapsed, delay)

		data, ok := result.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Operation completed", data["message"])
		assert.Equal(t, delay.String(), data["duration"])
	})

	t.Run("Process cancelled by context", func(t *testing.T) {
		handler := NewDelayHandler(1 * time.Second)

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context after 10ms
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		req := &Request{ID: "delay-cancel"}
		result, err := handler.Process(ctx, req)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestDelayHandler_ProcessWithProgress(t *testing.T) {
	t.Run("successful progress tracking", func(t *testing.T) {
		handler := NewDelayHandler(100 * time.Millisecond)

		// Mock progress reporter
		progressReports := []struct {
			percentage float64
			message    string
		}{}

		reporter := &mockProgressReporter{
			reportFunc: func(percentage float64, message string) error {
				progressReports = append(progressReports, struct {
					percentage float64
					message    string
				}{percentage, message})
				return nil
			},
		}

		req := &Request{ID: "progress-123"}
		result, err := handler.ProcessWithProgress(context.Background(), req, reporter)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)

		// Should have 10 progress reports
		assert.Len(t, progressReports, 10)

		// Check progress percentages
		for i, report := range progressReports {
			expectedPercentage := float64(i+1) * 10
			assert.Equal(t, expectedPercentage, report.percentage)
			assert.Equal(t, fmt.Sprintf("Step %d of 10 completed", i+1), report.message)
		}

		data, ok := result.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 10, data["steps"])
	})

	t.Run("progress reporter error", func(t *testing.T) {
		handler := NewDelayHandler(100 * time.Millisecond)

		reporter := &mockProgressReporter{
			reportFunc: func(percentage float64, message string) error {
				return errors.New("failed to report")
			},
		}

		req := &Request{ID: "progress-error"}
		result, err := handler.ProcessWithProgress(context.Background(), req, reporter)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to report progress")
	})

	t.Run("context cancellation during progress", func(t *testing.T) {
		handler := NewDelayHandler(1 * time.Second)

		ctx, cancel := context.WithCancel(context.Background())

		reportCount := 0
		reporter := &mockProgressReporter{
			reportFunc: func(percentage float64, message string) error {
				reportCount++
				if reportCount == 2 {
					cancel() // Cancel after second report
				}
				return nil
			},
		}

		req := &Request{ID: "progress-cancel"}
		result, err := handler.ProcessWithProgress(ctx, req, reporter)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, 2, reportCount)
	})
}

func TestValidationExampleHandler(t *testing.T) {
	handler := NewValidationExampleHandler()

	t.Run("EstimatedDuration", func(t *testing.T) {
		assert.Equal(t, 50*time.Millisecond, handler.EstimatedDuration())
	})

	t.Run("Validate - missing payload", func(t *testing.T) {
		req := &Request{}
		err := handler.Validate(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "payload is required")
	})

	t.Run("Validate - invalid JSON", func(t *testing.T) {
		req := &Request{
			Payload: []byte("invalid json"),
		}
		err := handler.Validate(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid payload format")
	})

	t.Run("Validate - missing required fields", func(t *testing.T) {
		tests := []struct {
			name    string
			payload map[string]interface{}
			errMsg  string
		}{
			{
				name:    "missing name",
				payload: map[string]interface{}{"email": "test@example.com"},
				errMsg:  "required field missing: name",
			},
			{
				name:    "missing email",
				payload: map[string]interface{}{"name": "Test User"},
				errMsg:  "required field missing: email",
			},
			{
				name:    "missing both",
				payload: map[string]interface{}{"other": "value"},
				errMsg:  "required field missing:",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				payload, _ := json.Marshal(tt.payload)
				req := &Request{Payload: payload}
				err := handler.Validate(req)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			})
		}
	})

	t.Run("Validate - invalid email", func(t *testing.T) {
		tests := []struct {
			name   string
			email  interface{}
			errMsg string
		}{
			{
				name:   "not a string",
				email:  123,
				errMsg: "email must be a string",
			},
			{
				name:   "too short",
				email:  "ab",
				errMsg: "invalid email length",
			},
			{
				name:   "too long",
				email:  string(make([]byte, 101)),
				errMsg: "invalid email length",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				payload, _ := json.Marshal(map[string]interface{}{
					"name":  "Test User",
					"email": tt.email,
				})
				req := &Request{Payload: payload}
				err := handler.Validate(req)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			})
		}
	})

	t.Run("Validate - success", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"name":  "Test User",
			"email": "test@example.com",
		})
		req := &Request{Payload: payload}
		err := handler.Validate(req)
		assert.NoError(t, err)
	})

	t.Run("Process", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		})
		req := &Request{
			ID:      "val-123",
			Payload: payload,
		}

		result, err := handler.Process(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "val-123", result.RequestID)
		assert.True(t, result.Success)

		data, ok := result.Data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Hello, John Doe!", data["message"])
		assert.Equal(t, "john@example.com", data["email"])
	})
}

// Mock progress reporter for testing
type mockProgressReporter struct {
	reportFunc      func(percentage float64, message string) error
	setMetadataFunc func(key string, value interface{}) error
}

func (m *mockProgressReporter) Report(percentage float64, message string) error {
	if m.reportFunc != nil {
		return m.reportFunc(percentage, message)
	}
	return nil
}

func (m *mockProgressReporter) SetMetadata(key string, value interface{}) error {
	if m.setMetadataFunc != nil {
		return m.setMetadataFunc(key, value)
	}
	return nil
}

var _ ProgressReporter = (*mockProgressReporter)(nil)
