package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pay-theory/streamer/pkg/streamer"
)

// mockProgressReporter implements ProgressReporter for testing
type mockProgressReporter struct {
	mu       sync.Mutex
	updates  []progressUpdate
	metadata map[string]interface{}
}

type progressUpdate struct {
	percentage float64
	message    string
	timestamp  time.Time
}

func (m *mockProgressReporter) Report(percentage float64, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updates = append(m.updates, progressUpdate{
		percentage: percentage,
		message:    message,
		timestamp:  time.Now(),
	})
	return nil
}

func (m *mockProgressReporter) SetMetadata(key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.metadata == nil {
		m.metadata = make(map[string]interface{})
	}
	m.metadata[key] = value
	return nil
}

func (m *mockProgressReporter) ReportError(err error) error {
	return m.Report(-1, fmt.Sprintf("Error: %v", err))
}

func (m *mockProgressReporter) getUpdates() []progressUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]progressUpdate, len(m.updates))
	copy(result, m.updates)
	return result
}

// TestReportAsyncHandler tests the report generation handler
func TestReportAsyncHandler(t *testing.T) {
	handler := NewReportAsyncHandler()

	t.Run("EstimatedDuration", func(t *testing.T) {
		duration := handler.EstimatedDuration()
		if duration != 2*time.Minute {
			t.Errorf("Expected 2 minutes, got %v", duration)
		}
	})

	t.Run("Validate_Success", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"start_date":  "2024-01-01",
			"end_date":    "2024-01-31",
			"format":      "pdf",
			"report_type": "monthly",
		})

		req := &streamer.Request{
			ID:      "test-123",
			Payload: payload,
		}

		err := handler.Validate(req)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Validate_MissingDates", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"format":      "pdf",
			"report_type": "monthly",
		})

		req := &streamer.Request{
			ID:      "test-123",
			Payload: payload,
		}

		err := handler.Validate(req)
		if err == nil {
			t.Error("Expected validation error for missing dates")
		}
	})

	t.Run("Validate_InvalidFormat", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"start_date":  "2024-01-01",
			"end_date":    "2024-01-31",
			"format":      "docx", // Invalid
			"report_type": "monthly",
		})

		req := &streamer.Request{
			ID:      "test-123",
			Payload: payload,
		}

		err := handler.Validate(req)
		if err == nil {
			t.Error("Expected validation error for invalid format")
		}
	})

	t.Run("ProcessWithProgress", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"start_date":  "2024-01-01",
			"end_date":    "2024-01-31",
			"format":      "csv",
			"report_type": "monthly",
		})

		req := &streamer.Request{
			ID:           "test-report-123",
			ConnectionID: "conn-123",
			Payload:      payload,
		}

		reporter := &mockProgressReporter{}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Run with shorter delays for testing
		oldOutputPath := handler.outputPath
		handler.outputPath = "/tmp/test"
		defer func() { handler.outputPath = oldOutputPath }()

		result, err := handler.ProcessWithProgress(ctx, req, reporter)
		if err != nil {
			t.Fatalf("ProcessWithProgress failed: %v", err)
		}

		// Verify result
		if !result.Success {
			t.Error("Expected successful result")
		}

		dataMap, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result.Data to be a map")
		}
		if dataMap["url"] == nil {
			t.Error("Expected URL in result data")
		}

		// Verify progress updates
		updates := reporter.getUpdates()
		if len(updates) < 4 {
			t.Errorf("Expected at least 4 progress updates, got %d", len(updates))
		}

		// Verify progress sequence
		lastProgress := float64(-1)
		for i, update := range updates {
			if update.percentage < lastProgress {
				t.Errorf("Progress decreased at update %d: %f -> %f",
					i, lastProgress, update.percentage)
			}
			lastProgress = update.percentage
		}

		// Verify final progress is 100%
		if updates[len(updates)-1].percentage != 100 {
			t.Errorf("Expected final progress to be 100%%, got %f",
				updates[len(updates)-1].percentage)
		}

		// Verify metadata
		if reporter.metadata["report_type"] != "monthly" {
			t.Error("Expected report_type metadata")
		}
	})
}

// TestDataProcessorHandler tests the data processing handler
func TestDataProcessorHandler(t *testing.T) {
	handler := NewDataProcessorHandler()

	t.Run("EstimatedDuration", func(t *testing.T) {
		duration := handler.EstimatedDuration()
		if duration != 5*time.Minute {
			t.Errorf("Expected 5 minutes, got %v", duration)
		}
	})

	t.Run("Validate_Success", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"pipeline": "classification",
			"data_source": map[string]interface{}{
				"type": "file",
				"path": "/data/input.csv",
			},
			"output": map[string]interface{}{
				"format": "json",
			},
		})

		req := &streamer.Request{
			ID:      "test-123",
			Payload: payload,
		}

		err := handler.Validate(req)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Validate_InvalidPipeline", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"pipeline": "invalid_pipeline",
			"data_source": map[string]interface{}{
				"type": "file",
				"path": "/data/input.csv",
			},
		})

		req := &streamer.Request{
			ID:      "test-123",
			Payload: payload,
		}

		err := handler.Validate(req)
		if err == nil {
			t.Error("Expected validation error for invalid pipeline")
		}
	})

	t.Run("ProcessWithProgress_Classification", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"pipeline": "classification",
			"data_source": map[string]interface{}{
				"type":  "query",
				"query": "SELECT * FROM events",
			},
			"output": map[string]interface{}{
				"format": "json",
			},
		})

		req := &streamer.Request{
			ID:           "test-ml-123",
			ConnectionID: "conn-123",
			Payload:      payload,
		}

		reporter := &mockProgressReporter{}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := handler.ProcessWithProgress(ctx, req, reporter)
		if err != nil {
			t.Fatalf("ProcessWithProgress failed: %v", err)
		}

		// Verify ML results
		dataMap, ok := result.Data.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result.Data to be a map")
		}
		summary, ok := dataMap["summary"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected summary in result")
		}

		metrics, ok := summary["metrics"].(map[string]float64)
		if !ok {
			t.Fatal("Expected metrics in summary")
		}

		// Check classification metrics
		expectedMetrics := []string{"accuracy", "precision", "recall", "f1_score"}
		for _, metric := range expectedMetrics {
			if _, exists := metrics[metric]; !exists {
				t.Errorf("Missing expected metric: %s", metric)
			}
		}

		// Verify progress through pipeline stages
		updates := reporter.getUpdates()

		// Log messages for debugging
		t.Logf("Progress messages received:")
		for _, update := range updates {
			t.Logf("  %.0f%%: %s", update.percentage, update.message)
		}

		// Should have multiple progress updates
		if len(updates) < 4 {
			t.Errorf("Expected at least 4 progress updates, got %d", len(updates))
		}

		// Verify final progress is 100%
		if len(updates) > 0 && updates[len(updates)-1].percentage != 100 {
			t.Errorf("Expected final progress to be 100%%, got %.0f%%",
				updates[len(updates)-1].percentage)
		}
	})
}

// TestProgressReporting tests progress reporting functionality
func TestProgressReporting(t *testing.T) {
	t.Run("ProgressSequence", func(t *testing.T) {
		reporter := &mockProgressReporter{}

		// Simulate progress updates
		reporter.Report(0, "Starting")
		reporter.Report(25, "Quarter done")
		reporter.Report(50, "Halfway")
		reporter.Report(75, "Three quarters")
		reporter.Report(100, "Complete")

		updates := reporter.getUpdates()

		if len(updates) != 5 {
			t.Errorf("Expected 5 updates, got %d", len(updates))
		}

		// Verify order and values
		expectedProgress := []float64{0, 25, 50, 75, 100}
		for i, update := range updates {
			if update.percentage != expectedProgress[i] {
				t.Errorf("Update %d: expected %f%%, got %f%%",
					i, expectedProgress[i], update.percentage)
			}
		}
	})

	t.Run("MetadataTracking", func(t *testing.T) {
		reporter := &mockProgressReporter{}

		// Set various metadata
		reporter.SetMetadata("stage", "initialization")
		reporter.SetMetadata("records", 1000)
		reporter.SetMetadata("rate", 123.45)
		reporter.SetMetadata("active", true)

		// Verify all metadata stored
		if reporter.metadata["stage"] != "initialization" {
			t.Error("String metadata not stored correctly")
		}

		if reporter.metadata["records"] != 1000 {
			t.Error("Integer metadata not stored correctly")
		}

		if reporter.metadata["rate"] != 123.45 {
			t.Error("Float metadata not stored correctly")
		}

		if reporter.metadata["active"] != true {
			t.Error("Boolean metadata not stored correctly")
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
