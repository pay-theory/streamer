package progress

import (
	"context"
	"log"
	"sync"
	"time"
)

// Reporter provides progress reporting functionality for async requests
type Reporter interface {
	// Report sends a progress update with percentage and message
	Report(percentage float64, message string) error

	// SetMetadata adds metadata to the progress update
	SetMetadata(key string, value interface{}) error

	// Complete marks the request as complete with the result
	Complete(result interface{}) error

	// Fail marks the request as failed with an error
	Fail(err error) error
}

// ConnectionManager interface for sending WebSocket messages
type ConnectionManager interface {
	Send(ctx context.Context, connectionID string, message interface{}) error
	IsActive(ctx context.Context, connectionID string) bool
}

// DefaultReporter implements the Reporter interface
type DefaultReporter struct {
	requestID      string
	connectionID   string
	connManager    ConnectionManager
	metadata       map[string]interface{}
	lastUpdate     time.Time
	updateInterval time.Duration
	mu             sync.Mutex
}

// NewReporter creates a new progress reporter
func NewReporter(requestID, connectionID string, connManager ConnectionManager) *DefaultReporter {
	return &DefaultReporter{
		requestID:      requestID,
		connectionID:   connectionID,
		connManager:    connManager,
		metadata:       make(map[string]interface{}),
		updateInterval: 100 * time.Millisecond, // Batch updates every 100ms
	}
}

// Report sends a progress update
func (r *DefaultReporter) Report(percentage float64, message string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Rate limit updates
	if time.Since(r.lastUpdate) < r.updateInterval && percentage < 100 {
		return nil
	}

	// Check if connection is still active
	ctx := context.Background()
	if !r.connManager.IsActive(ctx, r.connectionID) {
		log.Printf("[Progress] Connection %s no longer active for request %s", r.connectionID, r.requestID)
		return nil // Don't fail the whole process
	}

	update := map[string]interface{}{
		"type":       "progress",
		"request_id": r.requestID,
		"percentage": percentage,
		"message":    message,
		"timestamp":  time.Now().Unix(),
	}

	if len(r.metadata) > 0 {
		update["metadata"] = r.metadata
	}

	// Log the message being sent for debugging
	log.Printf("[Progress] Sending update for request %s: %.0f%% - %s", r.requestID, percentage, message)

	// Send via connection manager
	err := r.connManager.Send(ctx, r.connectionID, update)
	if err != nil {
		log.Printf("[Progress] Failed to send update for request %s: %v", r.requestID, err)
		// Don't return error to avoid failing the whole process
		return nil
	}

	r.lastUpdate = time.Now()
	return nil
}

// SetMetadata adds metadata to future progress updates
func (r *DefaultReporter) SetMetadata(key string, value interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.metadata[key] = value
	return nil
}

// Complete sends a completion notification
func (r *DefaultReporter) Complete(result interface{}) error {
	completion := map[string]interface{}{
		"type":       "complete",
		"request_id": r.requestID,
		"result":     result,
		"timestamp":  time.Now().Unix(),
	}

	ctx := context.Background()
	return r.connManager.Send(ctx, r.connectionID, completion)
}

// Fail sends a failure notification
func (r *DefaultReporter) Fail(err error) error {
	failure := map[string]interface{}{
		"type":       "error",
		"request_id": r.requestID,
		"error": map[string]interface{}{
			"message": err.Error(),
			"code":    "PROCESSING_FAILED",
		},
		"timestamp": time.Now().Unix(),
	}

	ctx := context.Background()
	return r.connManager.Send(ctx, r.connectionID, failure)
}

// contextKey is the type for context keys
type contextKey string

const reporterKey contextKey = "progress_reporter"

// WithReporter adds a progress reporter to the context
func WithReporter(ctx context.Context, reporter Reporter) context.Context {
	return context.WithValue(ctx, reporterKey, reporter)
}

// FromContext retrieves the progress reporter from context
func FromContext(ctx context.Context) (Reporter, bool) {
	reporter, ok := ctx.Value(reporterKey).(Reporter)
	return reporter, ok
}

// ReportProgress is a helper function to report progress from within handlers
func ReportProgress(ctx context.Context, percentage float64, message string) error {
	reporter, ok := FromContext(ctx)
	if !ok {
		// No reporter in context, silently ignore
		return nil
	}
	return reporter.Report(percentage, message)
}
