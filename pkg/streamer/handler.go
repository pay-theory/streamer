package streamer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// BaseHandler provides common functionality for handlers
type BaseHandler struct {
	estimatedDuration time.Duration
	validator         func(*Request) error
}

// EstimatedDuration returns the expected processing time
func (h *BaseHandler) EstimatedDuration() time.Duration {
	return h.estimatedDuration
}

// Validate validates the request
func (h *BaseHandler) Validate(req *Request) error {
	if h.validator != nil {
		return h.validator(req)
	}
	return nil
}

// HandlerFunc is an adapter to allow the use of ordinary functions as handlers
type HandlerFunc struct {
	BaseHandler
	ProcessFunc func(context.Context, *Request) (*Result, error)
}

// Process executes the handler function
func (h *HandlerFunc) Process(ctx context.Context, req *Request) (*Result, error) {
	return h.ProcessFunc(ctx, req)
}

// NewHandlerFunc creates a new handler from a function
func NewHandlerFunc(
	processFunc func(context.Context, *Request) (*Result, error),
	estimatedDuration time.Duration,
	validator func(*Request) error,
) Handler {
	return &HandlerFunc{
		BaseHandler: BaseHandler{
			estimatedDuration: estimatedDuration,
			validator:         validator,
		},
		ProcessFunc: processFunc,
	}
}

// SimpleHandler creates a simple handler with minimal configuration
func SimpleHandler(name string, processFunc func(context.Context, *Request) (*Result, error)) Handler {
	return NewHandlerFunc(processFunc, 100*time.Millisecond, nil)
}

// Example handlers for testing and demonstration

// EchoHandler echoes back the request payload
type EchoHandler struct {
	BaseHandler
}

// NewEchoHandler creates a new echo handler
func NewEchoHandler() *EchoHandler {
	return &EchoHandler{
		BaseHandler: BaseHandler{
			estimatedDuration: 10 * time.Millisecond,
		},
	}
}

// Process echoes the request payload
func (h *EchoHandler) Process(ctx context.Context, req *Request) (*Result, error) {
	var payload interface{}
	if req.Payload != nil {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	}

	return &Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"echo":        payload,
			"action":      req.Action,
			"received_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// DelayHandler simulates a long-running operation
type DelayHandler struct {
	BaseHandler
	delay time.Duration
}

// NewDelayHandler creates a new delay handler
func NewDelayHandler(delay time.Duration) *DelayHandler {
	return &DelayHandler{
		BaseHandler: BaseHandler{
			estimatedDuration: delay,
		},
		delay: delay,
	}
}

// Process simulates processing with a delay
func (h *DelayHandler) Process(ctx context.Context, req *Request) (*Result, error) {
	select {
	case <-time.After(h.delay):
		return &Result{
			RequestID: req.ID,
			Success:   true,
			Data: map[string]interface{}{
				"message":  "Operation completed",
				"duration": h.delay.String(),
			},
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ProcessWithProgress implements processing with progress updates
func (h *DelayHandler) ProcessWithProgress(ctx context.Context, req *Request, reporter ProgressReporter) (*Result, error) {
	steps := 10
	stepDuration := h.delay / time.Duration(steps)

	for i := 1; i <= steps; i++ {
		select {
		case <-time.After(stepDuration):
			percentage := float64(i) / float64(steps) * 100
			if err := reporter.Report(percentage, fmt.Sprintf("Step %d of %d completed", i, steps)); err != nil {
				return nil, fmt.Errorf("failed to report progress: %w", err)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return &Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"message":  "Operation completed with progress tracking",
			"duration": h.delay.String(),
			"steps":    steps,
		},
	}, nil
}

// ValidationExampleHandler demonstrates request validation
type ValidationExampleHandler struct {
	BaseHandler
}

// NewValidationExampleHandler creates a handler that validates specific payload fields
func NewValidationExampleHandler() *ValidationExampleHandler {
	handler := &ValidationExampleHandler{
		BaseHandler: BaseHandler{
			estimatedDuration: 50 * time.Millisecond,
		},
	}

	handler.validator = func(req *Request) error {
		if req.Payload == nil {
			return fmt.Errorf("payload is required")
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return fmt.Errorf("invalid payload format: %w", err)
		}

		// Validate required fields
		requiredFields := []string{"name", "email"}
		for _, field := range requiredFields {
			if _, exists := payload[field]; !exists {
				return fmt.Errorf("required field missing: %s", field)
			}
		}

		// Validate email format (simple check)
		if email, ok := payload["email"].(string); ok {
			if len(email) < 3 || len(email) > 100 {
				return fmt.Errorf("invalid email length")
			}
		} else {
			return fmt.Errorf("email must be a string")
		}

		return nil
	}

	return handler
}

// Process handles the validated request
func (h *ValidationExampleHandler) Process(ctx context.Context, req *Request) (*Result, error) {
	var payload map[string]interface{}
	json.Unmarshal(req.Payload, &payload)

	return &Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"message": fmt.Sprintf("Hello, %s!", payload["name"]),
			"email":   payload["email"],
		},
	}, nil
}
