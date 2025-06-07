package executor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/progress"
	"github.com/pay-theory/streamer/pkg/streamer"
)

// AsyncExecutor handles async request processing
type AsyncExecutor struct {
	connManager      *connection.Manager
	requestQueue     store.RequestQueue
	handlers         map[string]streamer.Handler
	progressHandlers map[string]streamer.HandlerWithProgress
	mu               sync.RWMutex
	logger           *log.Logger
}

// New creates a new async executor
func New(connManager *connection.Manager, requestQueue store.RequestQueue, logger *log.Logger) *AsyncExecutor {
	return &AsyncExecutor{
		connManager:      connManager,
		requestQueue:     requestQueue,
		handlers:         make(map[string]streamer.Handler),
		progressHandlers: make(map[string]streamer.HandlerWithProgress),
		logger:           logger,
	}
}

// RegisterHandler registers an async handler
func (e *AsyncExecutor) RegisterHandler(action string, handler streamer.Handler) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.handlers[action]; exists {
		return fmt.Errorf("handler already registered for action: %s", action)
	}

	e.handlers[action] = handler

	// Check if handler supports progress reporting
	if progressHandler, ok := handler.(streamer.HandlerWithProgress); ok {
		e.progressHandlers[action] = progressHandler
		e.logger.Printf("Registered handler with progress support for action: %s", action)
	} else {
		e.logger.Printf("Registered handler without progress support for action: %s", action)
	}

	return nil
}

// ProcessRequest processes a single async request
func (e *AsyncExecutor) ProcessRequest(ctx context.Context, asyncReq *store.AsyncRequest) error {
	e.logger.Printf("Processing async request: %s, action: %s", asyncReq.RequestID, asyncReq.Action)

	// Update status to PROCESSING
	if err := e.requestQueue.UpdateStatus(ctx, asyncReq.RequestID, store.StatusProcessing, "Processing started"); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Update processing started time
	now := time.Now()
	asyncReq.ProcessingStarted = &now

	// Get handler
	e.mu.RLock()
	handler, exists := e.handlers[asyncReq.Action]
	progressHandler, hasProgress := e.progressHandlers[asyncReq.Action]
	e.mu.RUnlock()

	if !exists {
		errMsg := fmt.Sprintf("unknown action: %s", asyncReq.Action)
		e.logger.Printf("Error: %s", errMsg)
		e.requestQueue.FailRequest(ctx, asyncReq.RequestID, errMsg)
		return errors.New(errMsg)
	}

	// Convert AsyncRequest to streamer.Request
	request, err := streamer.ConvertAsyncRequestToRequest(asyncReq)
	if err != nil {
		errMsg := fmt.Sprintf("failed to convert request: %v", err)
		e.logger.Printf("Error: %s", errMsg)
		e.requestQueue.FailRequest(ctx, asyncReq.RequestID, errMsg)
		return fmt.Errorf(errMsg)
	}

	// Validate request
	if err := handler.Validate(request); err != nil {
		errMsg := fmt.Sprintf("validation failed: %v", err)
		e.logger.Printf("Error: %s", errMsg)
		e.requestQueue.FailRequest(ctx, asyncReq.RequestID, errMsg)
		return fmt.Errorf(errMsg)
	}

	// Create progress reporter with batching for better performance
	reporter := progress.NewBatchedReporter(
		asyncReq.RequestID,
		asyncReq.ConnectionID,
		e.connManager,
		progress.WithInterval(200*time.Millisecond), // Batch every 200ms
		progress.WithMaxBatch(5),                    // Max 5 updates per batch
		progress.WithFlushThreshold(90.0),           // Flush at 90% or higher
	)

	// Report initial progress
	reporter.Report(0, "Processing started")

	// Process with appropriate handler
	var result *streamer.Result
	if hasProgress {
		// Use handler with progress support
		e.logger.Printf("Processing with progress support")
		result, err = progressHandler.ProcessWithProgress(ctx, request, reporter)
	} else {
		// Use regular handler
		e.logger.Printf("Processing without progress support")

		// Add reporter to context for handlers that might use it
		ctxWithReporter := progress.WithReporter(ctx, reporter)
		result, err = handler.Process(ctxWithReporter, request)

		// Send 100% progress for handlers without built-in progress
		reporter.Report(100, "Processing complete")
	}

	// Handle processing result
	if err != nil {
		errMsg := fmt.Sprintf("handler failed: %v", err)
		e.logger.Printf("Error: %s", errMsg)

		// Update request status
		e.requestQueue.FailRequest(ctx, asyncReq.RequestID, errMsg)

		// Send failure notification
		reporter.Fail(err)

		return fmt.Errorf(errMsg)
	}

	// Convert result to map for storage
	resultMap := make(map[string]interface{})
	if result.Data != nil {
		switch v := result.Data.(type) {
		case map[string]interface{}:
			resultMap = v
		default:
			resultMap["data"] = v
		}
	}
	resultMap["success"] = result.Success
	if result.Metadata != nil {
		resultMap["metadata"] = result.Metadata
	}

	// Update processing ended time
	endTime := time.Now()
	asyncReq.ProcessingEnded = &endTime

	// Mark request as complete
	if err := e.requestQueue.CompleteRequest(ctx, asyncReq.RequestID, resultMap); err != nil {
		e.logger.Printf("Failed to complete request: %v", err)
		return fmt.Errorf("failed to complete request: %w", err)
	}

	// Send completion notification
	reporter.Complete(resultMap)

	e.logger.Printf("Successfully processed request %s in %v",
		asyncReq.RequestID,
		endTime.Sub(*asyncReq.ProcessingStarted))

	// Shutdown the batched reporter gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	reporter.Shutdown(shutdownCtx)

	return nil
}

// ProcessWithRetry processes a request with retry logic
func (e *AsyncExecutor) ProcessWithRetry(ctx context.Context, asyncReq *store.AsyncRequest) error {
	maxRetries := asyncReq.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Update retry count
			asyncReq.RetryCount = attempt
			retryMsg := fmt.Sprintf("Retry attempt %d/%d", attempt, maxRetries)
			e.logger.Printf("%s for request %s", retryMsg, asyncReq.RequestID)

			// Update status with retry info
			e.requestQueue.UpdateStatus(ctx, asyncReq.RequestID, store.StatusRetrying, retryMsg)

			// Wait before retry (exponential backoff)
			backoff := time.Duration(attempt) * time.Second * 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}

			select {
			case <-time.After(backoff):
				// Continue with retry
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Process the request
		err := e.ProcessRequest(ctx, asyncReq)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			e.logger.Printf("Non-retryable error: %v", err)
			break
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", asyncReq.RetryCount+1, lastErr)
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types
	errStr := err.Error()

	// Network/timeout errors are retryable
	if contains(errStr, []string{"timeout", "connection refused", "EOF", "broken pipe"}) {
		return true
	}

	// Validation errors are not retryable
	if contains(errStr, []string{"validation", "invalid", "required"}) {
		return false
	}

	// Default to not retrying
	return false
}

// contains checks if a string contains any of the given substrings
func contains(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) && containsString(s, substr) {
			return true
		}
	}
	return false
}

// containsString is a simple string contains check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
