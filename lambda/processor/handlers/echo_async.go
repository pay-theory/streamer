package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pay-theory/streamer/pkg/streamer"
)

// EchoAsyncHandler is a simple test handler that echoes the input with progress updates
type EchoAsyncHandler struct{}

// NewEchoAsyncHandler creates a new echo async handler
func NewEchoAsyncHandler() *EchoAsyncHandler {
	return &EchoAsyncHandler{}
}

// EstimatedDuration returns the expected processing time
func (h *EchoAsyncHandler) EstimatedDuration() time.Duration {
	return 10 * time.Second
}

// Validate validates the echo request
func (h *EchoAsyncHandler) Validate(req *streamer.Request) error {
	if req.Payload == nil {
		return fmt.Errorf("payload is required")
	}
	return nil
}

// Process should not be called for async handlers
func (h *EchoAsyncHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return nil, fmt.Errorf("use ProcessWithProgress for async handlers")
}

// ProcessWithProgress echoes the input with simulated progress
func (h *EchoAsyncHandler) ProcessWithProgress(
	ctx context.Context,
	req *streamer.Request,
	reporter streamer.ProgressReporter,
) (*streamer.Result, error) {
	// Parse the payload
	var input map[string]interface{}
	if err := json.Unmarshal(req.Payload, &input); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	// Simple progress steps
	steps := []struct {
		percentage float64
		message    string
		duration   time.Duration
	}{
		{0, "Starting echo processing...", 1 * time.Second},
		{25, "Validating input...", 1 * time.Second},
		{50, "Processing data...", 2 * time.Second},
		{75, "Preparing response...", 1 * time.Second},
		{100, "Echo complete!", 0},
	}

	for _, step := range steps {
		// Report progress
		if err := reporter.Report(step.percentage, step.message); err != nil {
			// Failed to report progress - continue processing
		}

		// Simulate work
		if step.duration > 0 {
			select {
			case <-time.After(step.duration):
				// Continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// Return the echoed result
	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"echo":         input,
			"processed_at": time.Now().Format(time.RFC3339),
			"handler":      "echo_async",
		},
		Metadata: map[string]string{
			"test": "true",
		},
	}, nil
}
