package connection

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pay-theory/streamer/internal/store"
)

// marshalMessage is a helper function for benchmarking
func marshalMessage(message interface{}) ([]byte, error) {
	return json.Marshal(message)
}

// BenchmarkManager tests various performance characteristics
func BenchmarkManager_MessageMarshaling(b *testing.B) {
	messages := []struct {
		name    string
		message interface{}
	}{
		{"small", map[string]string{"type": "test", "data": "hello"}},
		{"medium", map[string]interface{}{
			"type": "update",
			"data": map[string]string{
				"field1": "value1",
				"field2": "value2",
				"field3": "value3",
				"field4": "value4",
				"field5": "value5",
			},
		}},
		{"large", generateLargeMessage()},
	}

	for _, msg := range messages {
		b.Run(msg.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = marshalMessage(msg.message)
			}
		})
	}
}

// BenchmarkManager_WorkerPool tests worker pool performance
func BenchmarkManager_WorkerPool(b *testing.B) {
	poolSizes := []int{1, 5, 10, 20, 50}

	for _, poolSize := range poolSizes {
		b.Run(fmt.Sprintf("pool_%d", poolSize), func(b *testing.B) {
			pool := make(chan struct{}, poolSize)
			for i := 0; i < poolSize; i++ {
				pool <- struct{}{}
			}

			var wg sync.WaitGroup
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// Simulate work
					select {
					case worker := <-pool:
						time.Sleep(100 * time.Microsecond)
						pool <- worker
					default:
						// Would block
					}
				}()
			}

			wg.Wait()
		})
	}
}

// BenchmarkManager_CircuitBreaker tests circuit breaker performance
func BenchmarkManager_CircuitBreaker(b *testing.B) {
	cb := &CircuitBreaker{
		failures:   make(map[string]int),
		lastFailed: make(map[string]time.Time),
		mu:         sync.RWMutex{},
	}

	// Test with different connection counts
	connCounts := []int{10, 100, 1000}

	for _, count := range connCounts {
		b.Run(fmt.Sprintf("%d_connections", count), func(b *testing.B) {
			// Pre-populate some failures
			for i := 0; i < count/10; i++ {
				connID := fmt.Sprintf("conn-%d", i)
				cb.RecordFailure(connID)
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					connID := fmt.Sprintf("conn-%d", i%count)
					_ = cb.IsOpen(connID)
					i++
				}
			})
		})
	}
}

// BenchmarkManager_LatencyTracker tests latency tracking performance
func BenchmarkManager_LatencyTracker(b *testing.B) {
	lt := &LatencyTracker{}

	b.Run("record", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				latency := time.Duration(100+b.N%1000) * time.Microsecond
				lt.Record(latency)
			}
		})
	})

	b.Run("percentile", func(b *testing.B) {
		// Pre-fill with data
		for i := 0; i < 1000; i++ {
			lt.Record(time.Duration(i) * time.Microsecond)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = lt.Percentile(0.99)
			}
		})
	})
}

// BenchmarkManager_ErrorCounters tests error counter performance
func BenchmarkManager_ErrorCounters(b *testing.B) {
	errorsByType := make(map[string]*atomic.Int64)
	errorTypes := []string{"connection_not_found", "connection_stale", "marshal_error", "network_error"}

	for _, errType := range errorTypes {
		errorsByType[errType] = &atomic.Int64{}
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			errType := errorTypes[i%len(errorTypes)]
			errorsByType[errType].Add(1)
			i++
		}
	})
}

// BenchmarkManager_ConcurrentMapAccess tests concurrent map operations
func BenchmarkManager_ConcurrentMapAccess(b *testing.B) {
	type safeMap struct {
		mu sync.RWMutex
		m  map[string]*store.Connection
	}

	sm := &safeMap{
		m: make(map[string]*store.Connection),
	}

	// Pre-populate
	for i := 0; i < 1000; i++ {
		connID := fmt.Sprintf("conn-%d", i)
		sm.m[connID] = &store.Connection{
			ConnectionID: connID,
			UserID:       fmt.Sprintf("user-%d", i),
			LastPing:     time.Now(),
		}
	}

	b.Run("read_heavy", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				connID := fmt.Sprintf("conn-%d", i%1000)
				sm.mu.RLock()
				_ = sm.m[connID]
				sm.mu.RUnlock()
				i++
			}
		})
	})

	b.Run("write_heavy", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				connID := fmt.Sprintf("conn-%d", i%1000)
				sm.mu.Lock()
				sm.m[connID] = &store.Connection{
					ConnectionID: connID,
					LastPing:     time.Now(),
				}
				sm.mu.Unlock()
				i++
			}
		})
	})

	b.Run("mixed", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				connID := fmt.Sprintf("conn-%d", i%1000)
				if i%10 == 0 {
					// Write
					sm.mu.Lock()
					sm.m[connID] = &store.Connection{
						ConnectionID: connID,
						LastPing:     time.Now(),
					}
					sm.mu.Unlock()
				} else {
					// Read
					sm.mu.RLock()
					_ = sm.m[connID]
					sm.mu.RUnlock()
				}
				i++
			}
		})
	})
}

// Helper function to generate a large message for benchmarking
func generateLargeMessage() map[string]interface{} {
	data := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		data[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d_with_some_additional_text_to_make_it_larger", i)
	}
	return map[string]interface{}{
		"type":      "large_update",
		"timestamp": time.Now().Unix(),
		"data":      data,
	}
}
