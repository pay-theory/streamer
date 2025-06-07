package progress

import (
	"context"
	"sync"
	"time"
)

// Update represents a single progress update
type Update struct {
	RequestID  string
	Percentage float64
	Message    string
	Metadata   map[string]interface{}
	Error      error
	Timestamp  time.Time
}

// Batcher batches progress updates to reduce WebSocket message volume
type Batcher struct {
	reporter       Reporter
	updates        chan *Update
	interval       time.Duration
	maxBatch       int
	flushThreshold float64 // Percentage threshold for immediate flush
	mu             sync.Mutex
	shutdownCh     chan struct{}
	wg             sync.WaitGroup
}

// NewBatcher creates a new progress update batcher
func NewBatcher(reporter Reporter, opts ...BatcherOption) *Batcher {
	b := &Batcher{
		reporter:       reporter,
		updates:        make(chan *Update, 100),
		interval:       100 * time.Millisecond,
		maxBatch:       10,
		flushThreshold: 95.0, // Flush immediately at 95% or higher
		shutdownCh:     make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	// Start the batching goroutine
	b.wg.Add(1)
	go b.run()

	return b
}

// BatcherOption configures the batcher
type BatcherOption func(*Batcher)

// WithInterval sets the batch interval
func WithInterval(interval time.Duration) BatcherOption {
	return func(b *Batcher) {
		b.interval = interval
	}
}

// WithMaxBatch sets the maximum batch size
func WithMaxBatch(max int) BatcherOption {
	return func(b *Batcher) {
		b.maxBatch = max
	}
}

// WithFlushThreshold sets the percentage threshold for immediate flush
func WithFlushThreshold(threshold float64) BatcherOption {
	return func(b *Batcher) {
		b.flushThreshold = threshold
	}
}

// Report adds an update to the batch
func (b *Batcher) Report(percentage float64, message string) error {
	update := &Update{
		Percentage: percentage,
		Message:    message,
		Timestamp:  time.Now(),
	}

	select {
	case b.updates <- update:
		return nil
	case <-b.shutdownCh:
		return nil
	default:
		// Channel full, drop oldest update
		select {
		case <-b.updates:
			// Dropped oldest
		default:
		}
		// Try again
		select {
		case b.updates <- update:
			return nil
		default:
			// Still full, skip this update
			return nil
		}
	}
}

// SetMetadata sets metadata for future updates
func (b *Batcher) SetMetadata(key string, value interface{}) error {
	return b.reporter.SetMetadata(key, value)
}

// Complete flushes any pending updates and marks as complete
func (b *Batcher) Complete(result interface{}) error {
	// Send 100% progress first
	b.Report(100, "Processing complete")

	// Wait a bit for the batch to flush
	time.Sleep(b.interval * 2)

	return b.reporter.Complete(result)
}

// Fail flushes any pending updates and marks as failed
func (b *Batcher) Fail(err error) error {
	// Flush any pending updates by signaling error
	update := &Update{
		Error:     err,
		Timestamp: time.Now(),
	}

	select {
	case b.updates <- update:
	default:
	}

	// Wait a bit for the batch to flush
	time.Sleep(b.interval * 2)

	return b.reporter.Fail(err)
}

// Shutdown gracefully shuts down the batcher
func (b *Batcher) Shutdown(ctx context.Context) error {
	close(b.shutdownCh)

	// Wait for run goroutine to finish
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the main batching loop
func (b *Batcher) run() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	batch := make([]*Update, 0, b.maxBatch)

	for {
		select {
		case update := <-b.updates:
			batch = append(batch, update)

			// Check if we should flush immediately
			shouldFlush := false

			// Flush on error
			if update.Error != nil {
				shouldFlush = true
			}

			// Flush on high percentage
			if update.Percentage >= b.flushThreshold {
				shouldFlush = true
			}

			// Flush if batch is full
			if len(batch) >= b.maxBatch {
				shouldFlush = true
			}

			if shouldFlush {
				b.flush(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				b.flush(batch)
				batch = batch[:0]
			}

		case <-b.shutdownCh:
			// Flush any remaining updates
			if len(batch) > 0 {
				b.flush(batch)
			}
			return
		}
	}
}

// flush sends batched updates
func (b *Batcher) flush(updates []*Update) {
	if len(updates) == 0 {
		return
	}

	// Combine updates intelligently
	combined := b.combineUpdates(updates)

	// Send combined updates
	for _, update := range combined {
		if update.Error != nil {
			// Don't report error updates as progress
			continue
		}
		b.reporter.Report(update.Percentage, update.Message)
	}
}

// combineUpdates intelligently combines multiple updates
func (b *Batcher) combineUpdates(updates []*Update) []*Update {
	if len(updates) <= 1 {
		return updates
	}

	// If we have many updates, we want to:
	// 1. Always include the first update (shows initial progress)
	// 2. Always include the last update (most recent state)
	// 3. Include any error updates
	// 4. Include significant percentage jumps

	result := make([]*Update, 0, len(updates))

	// Always include first
	result = append(result, updates[0])
	lastPercentage := updates[0].Percentage

	// Process middle updates
	for i := 1; i < len(updates)-1; i++ {
		update := updates[i]

		// Include errors
		if update.Error != nil {
			result = append(result, update)
			continue
		}

		// Include significant jumps (>10%)
		if update.Percentage-lastPercentage >= 10.0 {
			result = append(result, update)
			lastPercentage = update.Percentage
		}
	}

	// Always include last if different from what we have
	lastUpdate := updates[len(updates)-1]
	if len(result) == 0 ||
		lastUpdate.Percentage != result[len(result)-1].Percentage ||
		lastUpdate.Message != result[len(result)-1].Message {
		result = append(result, lastUpdate)
	}

	return result
}

// BatchedReporter wraps a Reporter with batching capabilities
type BatchedReporter struct {
	*Batcher
}

// NewBatchedReporter creates a new batched reporter
func NewBatchedReporter(requestID, connectionID string, connManager ConnectionManager, opts ...BatcherOption) *BatchedReporter {
	reporter := NewReporter(requestID, connectionID, connManager)
	batcher := NewBatcher(reporter, opts...)

	return &BatchedReporter{
		Batcher: batcher,
	}
}
