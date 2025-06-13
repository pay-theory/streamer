package progress

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockReporter is a mock implementation of Reporter
type mockReporter struct {
	mock.Mock
	mu      sync.Mutex
	reports []reportCall
}

type reportCall struct {
	percentage float64
	message    string
}

func (m *mockReporter) Report(percentage float64, message string) error {
	m.mu.Lock()
	m.reports = append(m.reports, reportCall{percentage: percentage, message: message})
	m.mu.Unlock()

	args := m.Called(percentage, message)
	return args.Error(0)
}

func (m *mockReporter) SetMetadata(key string, value interface{}) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *mockReporter) Complete(result interface{}) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *mockReporter) Fail(err error) error {
	args := m.Called(err)
	return args.Error(0)
}

func (m *mockReporter) GetReports() []reportCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]reportCall{}, m.reports...)
}

// TestNewBatcher tests the NewBatcher constructor
func TestNewBatcher(t *testing.T) {
	mockRep := new(mockReporter)

	// Test with default options
	batcher := NewBatcher(mockRep)
	assert.NotNil(t, batcher)
	assert.Equal(t, mockRep, batcher.reporter)
	assert.Equal(t, 100*time.Millisecond, batcher.interval)
	assert.Equal(t, 10, batcher.maxBatch)
	assert.Equal(t, 95.0, batcher.flushThreshold)

	// Shutdown the batcher
	ctx := context.Background()
	err := batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherOptions tests batcher configuration options
func TestBatcherOptions(t *testing.T) {
	mockRep := new(mockReporter)

	batcher := NewBatcher(mockRep,
		WithInterval(200*time.Millisecond),
		WithMaxBatch(20),
		WithFlushThreshold(90.0),
	)

	assert.Equal(t, 200*time.Millisecond, batcher.interval)
	assert.Equal(t, 20, batcher.maxBatch)
	assert.Equal(t, 90.0, batcher.flushThreshold)

	// Shutdown
	ctx := context.Background()
	err := batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherReport tests basic reporting functionality
func TestBatcherReport(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil)

	batcher := NewBatcher(mockRep, WithInterval(50*time.Millisecond))

	// Send some reports
	err := batcher.Report(10.0, "Starting")
	assert.NoError(t, err)

	err = batcher.Report(20.0, "Continuing")
	assert.NoError(t, err)

	// Wait for batch to flush
	time.Sleep(100 * time.Millisecond)

	// Verify reports were sent
	mockRep.AssertCalled(t, "Report", 10.0, "Starting")
	mockRep.AssertCalled(t, "Report", 20.0, "Continuing")

	// Shutdown
	ctx := context.Background()
	err = batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherFlushThreshold tests immediate flush on high percentage
func TestBatcherFlushThreshold(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil)

	batcher := NewBatcher(mockRep,
		WithInterval(1*time.Second), // Long interval
		WithFlushThreshold(90.0),
	)

	// Report below threshold - should not flush immediately
	err := batcher.Report(50.0, "Halfway")
	assert.NoError(t, err)

	// Give it a small delay to ensure no flush happens
	time.Sleep(50 * time.Millisecond)
	mockRep.AssertNotCalled(t, "Report", 50.0, "Halfway")

	// Report at threshold - should flush immediately
	err = batcher.Report(95.0, "Almost done")
	assert.NoError(t, err)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)
	mockRep.AssertCalled(t, "Report", 50.0, "Halfway")
	mockRep.AssertCalled(t, "Report", 95.0, "Almost done")

	// Shutdown
	ctx := context.Background()
	err = batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherMaxBatch tests flush on reaching max batch size
func TestBatcherMaxBatch(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil)

	batcher := NewBatcher(mockRep,
		WithInterval(1*time.Second), // Long interval
		WithMaxBatch(3),
	)

	// Send reports up to max batch
	for i := 1; i <= 3; i++ {
		err := batcher.Report(float64(i*10), "Progress")
		assert.NoError(t, err)
	}

	// Should flush immediately after reaching max
	time.Sleep(50 * time.Millisecond)

	reports := mockRep.GetReports()
	assert.Len(t, reports, 3)

	// Shutdown
	ctx := context.Background()
	err := batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherChannelFull tests behavior when update channel is full
func TestBatcherChannelFull(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil)

	// Create batcher with small channel
	batcher := NewBatcher(mockRep, WithInterval(1*time.Second))

	// Fill the channel
	for i := 0; i < 105; i++ { // Channel size is 100
		err := batcher.Report(float64(i), "Update")
		assert.NoError(t, err)
	}

	// Should not block or panic
	// Shutdown
	ctx := context.Background()
	err := batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherSetMetadata tests metadata setting
func TestBatcherSetMetadata(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("SetMetadata", "key1", "value1").Return(nil)
	mockRep.On("SetMetadata", "key2", 123).Return(nil)

	batcher := NewBatcher(mockRep)

	err := batcher.SetMetadata("key1", "value1")
	assert.NoError(t, err)

	err = batcher.SetMetadata("key2", 123)
	assert.NoError(t, err)

	mockRep.AssertExpectations(t)

	// Shutdown
	ctx := context.Background()
	err = batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherComplete tests completion handling
func TestBatcherComplete(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Report", 100.0, "Processing complete").Return(nil)
	mockRep.On("Complete", map[string]interface{}{"status": "success"}).Return(nil)

	batcher := NewBatcher(mockRep, WithInterval(50*time.Millisecond))

	result := map[string]interface{}{"status": "success"}
	err := batcher.Complete(result)
	assert.NoError(t, err)

	// Wait for completion
	time.Sleep(150 * time.Millisecond)

	mockRep.AssertExpectations(t)

	// Shutdown
	ctx := context.Background()
	err = batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherFail tests failure handling
func TestBatcherFail(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Fail", mock.MatchedBy(func(err error) bool {
		return err.Error() == "processing failed"
	})).Return(nil)
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil).Maybe()

	batcher := NewBatcher(mockRep, WithInterval(50*time.Millisecond))

	err := batcher.Fail(errors.New("processing failed"))
	assert.NoError(t, err)

	// Wait for failure to process
	time.Sleep(150 * time.Millisecond)

	mockRep.AssertCalled(t, "Fail", mock.MatchedBy(func(err error) bool {
		return err.Error() == "processing failed"
	}))

	// Shutdown
	ctx := context.Background()
	err = batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatcherShutdown tests graceful shutdown
func TestBatcherShutdown(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil)

	batcher := NewBatcher(mockRep, WithInterval(50*time.Millisecond))

	// Send some reports
	err := batcher.Report(10.0, "Progress 1")
	assert.NoError(t, err)
	err = batcher.Report(20.0, "Progress 2")
	assert.NoError(t, err)

	// Shutdown should flush pending updates
	ctx := context.Background()
	err = batcher.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify reports were flushed
	mockRep.AssertCalled(t, "Report", 10.0, "Progress 1")
	mockRep.AssertCalled(t, "Report", 20.0, "Progress 2")
}

// TestBatcherShutdownTimeout tests shutdown with timeout
func TestBatcherShutdownTimeout(t *testing.T) {
	mockRep := new(mockReporter)
	// Slow reporter that blocks
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		time.Sleep(100 * time.Millisecond)
	})

	batcher := NewBatcher(mockRep, WithInterval(50*time.Millisecond))

	// Send report
	err := batcher.Report(10.0, "Progress")
	assert.NoError(t, err)

	// Shutdown with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = batcher.Shutdown(ctx)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// TestCombineUpdates tests the update combining logic
func TestCombineUpdates(t *testing.T) {
	mockRep := new(mockReporter)
	batcher := NewBatcher(mockRep)

	tests := []struct {
		name     string
		updates  []*Update
		expected int // Expected number of combined updates
	}{
		{
			name:     "single update",
			updates:  []*Update{{Percentage: 10.0, Message: "Start"}},
			expected: 1,
		},
		{
			name: "two updates",
			updates: []*Update{
				{Percentage: 10.0, Message: "Start"},
				{Percentage: 20.0, Message: "Progress"},
			},
			expected: 2,
		},
		{
			name: "many updates with small increments",
			updates: []*Update{
				{Percentage: 10.0, Message: "Start"},
				{Percentage: 15.0, Message: "15%"},
				{Percentage: 18.0, Message: "18%"},
				{Percentage: 20.0, Message: "20%"},
				{Percentage: 35.0, Message: "35%"}, // Big jump
				{Percentage: 38.0, Message: "38%"},
				{Percentage: 50.0, Message: "Halfway"}, // Big jump
			},
			expected: 4, // First, 35% jump, 50% jump, and last
		},
		{
			name: "updates with error",
			updates: []*Update{
				{Percentage: 10.0, Message: "Start"},
				{Percentage: 20.0, Message: "Progress"},
				{Error: errors.New("failed"), Message: "Error"},
				{Percentage: 30.0, Message: "Continue"},
			},
			expected: 4, // All included due to error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			combined := batcher.combineUpdates(tt.updates)
			assert.Len(t, combined, tt.expected)

			// First and last should always be included
			if len(tt.updates) > 1 {
				assert.Equal(t, tt.updates[0].Percentage, combined[0].Percentage)
				assert.Equal(t, tt.updates[len(tt.updates)-1].Percentage, combined[len(combined)-1].Percentage)
			}
		})
	}

	// Shutdown
	ctx := context.Background()
	err := batcher.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestBatchedReporter tests the BatchedReporter wrapper
func TestBatchedReporter(t *testing.T) {
	mockConn := new(mockConnectionManager)
	mockConn.On("IsActive", mock.Anything, "conn456").Return(true)
	mockConn.On("Send", mock.Anything, "conn456", mock.Anything).Return(nil)

	reporter := NewBatchedReporter("req123", "conn456", mockConn,
		WithInterval(50*time.Millisecond),
		WithMaxBatch(5),
	)

	assert.NotNil(t, reporter)
	assert.NotNil(t, reporter.Batcher)

	// Test reporting through batched reporter
	err := reporter.Report(25.0, "Quarter done")
	assert.NoError(t, err)

	err = reporter.Report(50.0, "Half done")
	assert.NoError(t, err)

	// Wait for batch to flush
	time.Sleep(100 * time.Millisecond)

	// Verify messages were sent
	mockConn.AssertExpectations(t)

	// Shutdown
	ctx := context.Background()
	err = reporter.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestConcurrentBatcherAccess tests thread safety
func TestConcurrentBatcherAccess(t *testing.T) {
	mockRep := new(mockReporter)
	mockRep.On("Report", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRep.On("SetMetadata", mock.Anything, mock.Anything).Return(nil).Maybe()

	batcher := NewBatcher(mockRep, WithInterval(50*time.Millisecond))

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent reports
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				err := batcher.Report(float64(idx*10+j), "Progress")
				assert.NoError(t, err)
			}
		}(i)
	}

	// Concurrent metadata
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			err := batcher.SetMetadata(string(rune('a'+idx)), idx)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Shutdown
	ctx := context.Background()
	err := batcher.Shutdown(ctx)
	assert.NoError(t, err)

	// Should not panic
	mockRep.AssertExpectations(t)
}

// TestBatcherReporterInterface tests that Batcher implements Reporter interface
func TestBatcherReporterInterface(t *testing.T) {
	var _ Reporter = (*Batcher)(nil)
	var _ Reporter = (*BatchedReporter)(nil)
}
