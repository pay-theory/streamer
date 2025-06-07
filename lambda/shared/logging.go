package shared

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

// StructuredLog represents a structured log entry
type StructuredLog struct {
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	RequestID     string                 `json:"request_id,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Timestamp     int64                  `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// LogLevel constants
const (
	LogLevelDebug = "DEBUG"
	LogLevelInfo  = "INFO"
	LogLevelWarn  = "WARN"
	LogLevelError = "ERROR"
)

// Logger provides structured logging
type Logger struct {
	serviceName string
}

// NewLogger creates a new structured logger
func NewLogger(serviceName string) *Logger {
	return &Logger{
		serviceName: serviceName,
	}
}

// extractRequestID gets the Lambda request ID from context
func extractRequestID(ctx context.Context) string {
	lc, ok := lambdacontext.FromContext(ctx)
	if ok {
		return lc.AwsRequestID
	}
	return ""
}

// Log writes a structured log entry
func (l *Logger) Log(ctx context.Context, level, message string, metadata map[string]interface{}) {
	entry := StructuredLog{
		Level:     level,
		Message:   message,
		RequestID: extractRequestID(ctx),
		Timestamp: getTimestamp(),
		Metadata:  metadata,
	}

	if l.serviceName != "" {
		if entry.Metadata == nil {
			entry.Metadata = make(map[string]interface{})
		}
		entry.Metadata["service"] = l.serviceName
	}

	// Add correlation ID if present in context
	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		if id, ok := correlationID.(string); ok {
			entry.CorrelationID = id
		}
	}

	// Output as JSON
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}

	// Write to stdout (CloudWatch will capture this)
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, message string, metadata map[string]interface{}) {
	l.Log(ctx, LogLevelDebug, message, metadata)
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, message string, metadata map[string]interface{}) {
	l.Log(ctx, LogLevelInfo, message, metadata)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, message string, metadata map[string]interface{}) {
	l.Log(ctx, LogLevelWarn, message, metadata)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, message string, metadata map[string]interface{}) {
	l.Log(ctx, LogLevelError, message, metadata)
}

// LogMetric logs a metric event (for CloudWatch Metrics)
func LogMetric(ctx context.Context, metricName string, metadata map[string]interface{}) {
	// CloudWatch Embedded Metric Format
	emf := map[string]interface{}{
		"_aws": map[string]interface{}{
			"Timestamp": getTimestamp(),
			"CloudWatchMetrics": []map[string]interface{}{
				{
					"Namespace":  "Streamer",
					"Dimensions": [][]string{{"Service"}},
					"Metrics": []map[string]interface{}{
						{
							"Name": metricName,
							"Unit": "Count",
						},
					},
				},
			},
		},
		"Service":  os.Getenv("SERVICE_NAME"),
		metricName: 1,
	}

	// Add metadata
	for k, v := range metadata {
		emf[k] = v
	}

	// Output EMF
	data, err := json.Marshal(emf)
	if err != nil {
		log.Printf("Failed to marshal EMF: %v", err)
		return
	}

	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

// getTimestamp returns current Unix timestamp in milliseconds
func getTimestamp() int64 {
	return time.Now().UnixNano() / 1e6
}
