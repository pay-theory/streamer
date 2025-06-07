package connection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/pay-theory/streamer/internal/store"
)

// Metrics holds performance metrics for the connection manager
type Metrics struct {
	SendLatency      *LatencyTracker
	BroadcastLatency *LatencyTracker
	ErrorsByType     map[string]*atomic.Int64
	ActiveSends      *atomic.Int32
	mu               sync.RWMutex
}

// LatencyTracker tracks latency percentiles
type LatencyTracker struct {
	samples []time.Duration
	mu      sync.Mutex
}

// CircuitBreaker tracks connection health
type CircuitBreaker struct {
	failures   map[string]int
	lastFailed map[string]time.Time
	mu         sync.RWMutex
}

// Manager handles WebSocket connections through API Gateway
type Manager struct {
	store      store.ConnectionStore
	apiGateway *apigatewaymanagementapi.Client
	endpoint   string

	// Production features
	workerPool     chan struct{}
	circuitBreaker *CircuitBreaker
	metrics        *Metrics
	shutdownCh     chan struct{}
	wg             sync.WaitGroup

	mu     sync.RWMutex
	logger func(format string, args ...interface{})
}

// NewManager creates a new connection manager
func NewManager(store store.ConnectionStore, apiGateway *apigatewaymanagementapi.Client, endpoint string) *Manager {
	m := &Manager{
		store:      store,
		apiGateway: apiGateway,
		endpoint:   endpoint,
		workerPool: make(chan struct{}, 10), // 10 concurrent workers
		circuitBreaker: &CircuitBreaker{
			failures:   make(map[string]int),
			lastFailed: make(map[string]time.Time),
		},
		metrics: &Metrics{
			SendLatency:      &LatencyTracker{},
			BroadcastLatency: &LatencyTracker{},
			ErrorsByType:     make(map[string]*atomic.Int64),
			ActiveSends:      &atomic.Int32{},
		},
		shutdownCh: make(chan struct{}),
		logger:     func(format string, args ...interface{}) { fmt.Printf(format+"\n", args...) },
	}

	// Initialize error counters
	errorTypes := []string{"connection_not_found", "connection_stale", "marshal_error", "network_error", "timeout"}
	for _, errType := range errorTypes {
		m.metrics.ErrorsByType[errType] = &atomic.Int64{}
	}

	return m
}

// SetLogger sets a custom logger function
func (m *Manager) SetLogger(logger func(format string, args ...interface{})) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = logger
}

// Shutdown gracefully shuts down the manager
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger("Shutting down connection manager...")

	// Signal shutdown
	close(m.shutdownCh)

	// Wait for active operations to complete
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger("Connection manager shutdown complete")
		return nil
	case <-ctx.Done():
		m.logger("Connection manager shutdown timed out")
		return ctx.Err()
	}
}

// Send sends a message to a specific connection
func (m *Manager) Send(ctx context.Context, connectionID string, message interface{}) error {
	// Check if shutting down
	select {
	case <-m.shutdownCh:
		return errors.New("manager is shutting down")
	default:
	}

	// Track active sends
	m.metrics.ActiveSends.Add(1)
	defer m.metrics.ActiveSends.Add(-1)

	// Track latency
	start := time.Now()
	defer func() {
		m.metrics.SendLatency.Record(time.Since(start))
	}()

	// Check circuit breaker
	if m.circuitBreaker.IsOpen(connectionID) {
		m.metrics.ErrorsByType["circuit_open"].Add(1)
		return fmt.Errorf("circuit breaker open for connection %s", connectionID)
	}

	// Validate connection exists
	conn, err := m.store.Get(ctx, connectionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			m.metrics.ErrorsByType["connection_not_found"].Add(1)
			return ErrConnectionNotFound
		}
		m.metrics.ErrorsByType["network_error"].Add(1)
		return fmt.Errorf("failed to get connection: %w", err)
	}

	// Marshal message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		m.metrics.ErrorsByType["marshal_error"].Add(1)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send with retry logic
	err = m.sendWithRetry(ctx, connectionID, data)
	if err != nil {
		// Handle 410 Gone - connection is stale
		if isConnectionGone(err) {
			m.metrics.ErrorsByType["connection_stale"].Add(1)
			m.logger("Connection %s is stale, removing from store", connectionID)
			if delErr := m.store.Delete(ctx, connectionID); delErr != nil {
				m.logger("Failed to delete stale connection %s: %v", connectionID, delErr)
			}
			return ErrConnectionStale
		}

		// Record failure for circuit breaker
		m.circuitBreaker.RecordFailure(connectionID)
		m.metrics.ErrorsByType["network_error"].Add(1)
		return err
	}

	// Record success
	m.circuitBreaker.RecordSuccess(connectionID)

	// Update last ping time
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.UpdateLastPing(updateCtx, conn.ConnectionID); err != nil {
			m.logger("Failed to update last ping for connection %s: %v", connectionID, err)
		}
	}()

	return nil
}

// Broadcast sends a message to multiple connections
func (m *Manager) Broadcast(ctx context.Context, connectionIDs []string, message interface{}) error {
	if len(connectionIDs) == 0 {
		return nil
	}

	// Track broadcast latency
	start := time.Now()
	defer func() {
		m.metrics.BroadcastLatency.Record(time.Since(start))
	}()

	// Marshal message once
	data, err := json.Marshal(message)
	if err != nil {
		m.metrics.ErrorsByType["marshal_error"].Add(1)
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Use worker pool for parallel sending
	jobs := make(chan string, len(connectionIDs))
	results := make(chan error, len(connectionIDs))

	// Start workers
	numWorkers := 10
	if len(connectionIDs) < numWorkers {
		numWorkers = len(connectionIDs)
	}

	m.wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer m.wg.Done()
			for connID := range jobs {
				select {
				case <-m.shutdownCh:
					results <- errors.New("shutdown in progress")
					return
				case m.workerPool <- struct{}{}:
					err := m.sendWithRetry(ctx, connID, data)
					<-m.workerPool

					if err != nil {
						m.logger("Failed to send to connection %s: %v", connID, err)

						// Handle stale connections
						if isConnectionGone(err) {
							go func(id string) {
								delCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
								defer cancel()
								if delErr := m.store.Delete(delCtx, id); delErr != nil {
									m.logger("Failed to delete stale connection %s: %v", id, delErr)
								}
							}(connID)
						}
					}
					results <- err
				}
			}
		}()
	}

	// Queue jobs
	for _, connID := range connectionIDs {
		jobs <- connID
	}
	close(jobs)

	// Collect results
	var errs []error
	for i := 0; i < len(connectionIDs); i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		m.logger("Broadcast completed with %d errors out of %d connections", len(errs), len(connectionIDs))
		return fmt.Errorf("broadcast had %d failures: %w", len(errs), errs[0])
	}

	return nil
}

// IsActive checks if a connection is active
func (m *Manager) IsActive(ctx context.Context, connectionID string) bool {
	// Check circuit breaker first
	if m.circuitBreaker.IsOpen(connectionID) {
		return false
	}

	conn, err := m.store.Get(ctx, connectionID)
	if err != nil {
		return false
	}

	// Check if connection is stale (no ping in last 5 minutes)
	if time.Since(conn.LastPing) > 5*time.Minute {
		// Try to ping the connection
		testData := []byte(`{"type":"ping"}`)
		err := m.sendMessage(ctx, connectionID, testData)
		if err != nil {
			m.circuitBreaker.RecordFailure(connectionID)
			return false
		}
		m.circuitBreaker.RecordSuccess(connectionID)
	}

	return true
}

// GetMetrics returns current performance metrics
func (m *Manager) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"active_sends":          m.metrics.ActiveSends.Load(),
		"send_latency_p50":      m.metrics.SendLatency.Percentile(0.5),
		"send_latency_p99":      m.metrics.SendLatency.Percentile(0.99),
		"broadcast_latency_p50": m.metrics.BroadcastLatency.Percentile(0.5),
		"broadcast_latency_p99": m.metrics.BroadcastLatency.Percentile(0.99),
		"errors": map[string]int64{
			"connection_not_found": m.metrics.ErrorsByType["connection_not_found"].Load(),
			"connection_stale":     m.metrics.ErrorsByType["connection_stale"].Load(),
			"marshal_error":        m.metrics.ErrorsByType["marshal_error"].Load(),
			"network_error":        m.metrics.ErrorsByType["network_error"].Load(),
		},
		"circuit_breakers_open": m.circuitBreaker.CountOpen(),
	}
}

// sendWithRetry sends a message with exponential backoff retry
func (m *Manager) sendWithRetry(ctx context.Context, connectionID string, data []byte) error {
	const maxRetries = 3
	baseDelay := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := m.sendMessage(ctx, connectionID, data)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on 4xx errors (except 429)
		var clientErr interface{ HTTPStatusCode() int }
		if errors.As(err, &clientErr) {
			statusCode := clientErr.HTTPStatusCode()
			if statusCode >= 400 && statusCode < 500 && statusCode != 429 {
				return err
			}
		}

		// Calculate delay with exponential backoff
		delay := baseDelay * time.Duration(1<<attempt)
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}

		// Add jitter (±25%)
		jitter := time.Duration(float64(delay) * 0.25 * (2*rand.Float64() - 1))
		delay += jitter

		m.logger("Retry %d/%d for connection %s after %v", attempt+1, maxRetries, connectionID, delay)

		select {
		case <-time.After(delay):
			// Continue to next retry
		case <-ctx.Done():
			return ctx.Err()
		case <-m.shutdownCh:
			return errors.New("shutdown in progress")
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// sendMessage sends a single message to a connection
func (m *Manager) sendMessage(ctx context.Context, connectionID string, data []byte) error {
	input := &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	}

	_, err := m.apiGateway.PostToConnection(ctx, input)
	return err
}

// isConnectionGone checks if the error indicates a 410 Gone status
func isConnectionGone(err error) bool {
	if err == nil {
		return false
	}

	// Check for GoneException type
	var goneErr *types.GoneException
	if errors.As(err, &goneErr) {
		return true
	}

	// Also check for HTTP status code in error response
	var apiErr interface{ HTTPStatusCode() int }
	if errors.As(err, &apiErr) && apiErr.HTTPStatusCode() == 410 {
		return true
	}

	return false
}

// CircuitBreaker methods

func (cb *CircuitBreaker) IsOpen(connectionID string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	failures, exists := cb.failures[connectionID]
	if !exists {
		return false
	}

	// Open circuit after 3 consecutive failures
	if failures >= 3 {
		// Check if we should try again (after 30 seconds)
		if time.Since(cb.lastFailed[connectionID]) > 30*time.Second {
			return false
		}
		return true
	}

	return false
}

func (cb *CircuitBreaker) RecordFailure(connectionID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[connectionID]++
	cb.lastFailed[connectionID] = time.Now()
}

func (cb *CircuitBreaker) RecordSuccess(connectionID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.failures, connectionID)
	delete(cb.lastFailed, connectionID)
}

func (cb *CircuitBreaker) CountOpen() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	count := 0
	for connID, failures := range cb.failures {
		if failures >= 3 && time.Since(cb.lastFailed[connID]) <= 30*time.Second {
			count++
		}
	}
	return count
}

// LatencyTracker methods

func (lt *LatencyTracker) Record(d time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.samples = append(lt.samples, d)

	// Keep only last 1000 samples
	if len(lt.samples) > 1000 {
		lt.samples = lt.samples[len(lt.samples)-1000:]
	}
}

func (lt *LatencyTracker) Percentile(p float64) time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.samples) == 0 {
		return 0
	}

	// Simple percentile calculation (not optimized)
	sorted := make([]time.Duration, len(lt.samples))
	copy(sorted, lt.samples)

	// Sort samples
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}
