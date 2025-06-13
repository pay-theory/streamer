package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout captures stdout output during test execution
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger("test-service")
	assert.NotNil(t, logger)
	assert.Equal(t, "test-service", logger.serviceName)
}

func TestLogger_Log(t *testing.T) {
	logger := NewLogger("test-service")

	tests := []struct {
		name     string
		ctx      context.Context
		level    string
		message  string
		metadata map[string]interface{}
		validate func(t *testing.T, log StructuredLog)
	}{
		{
			name:    "basic log entry",
			ctx:     context.Background(),
			level:   LogLevelInfo,
			message: "Test message",
			metadata: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
			validate: func(t *testing.T, log StructuredLog) {
				assert.Equal(t, LogLevelInfo, log.Level)
				assert.Equal(t, "Test message", log.Message)
				assert.Equal(t, "test-service", log.Metadata["service"])
				assert.Equal(t, "value1", log.Metadata["key1"])
				assert.Equal(t, float64(123), log.Metadata["key2"]) // JSON unmarshals numbers as float64
			},
		},
		{
			name:     "log without metadata",
			ctx:      context.Background(),
			level:    LogLevelWarn,
			message:  "Warning message",
			metadata: nil,
			validate: func(t *testing.T, log StructuredLog) {
				assert.Equal(t, LogLevelWarn, log.Level)
				assert.Equal(t, "Warning message", log.Message)
				assert.Equal(t, "test-service", log.Metadata["service"])
			},
		},
		{
			name: "log with lambda context",
			ctx: lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{
				AwsRequestID: "test-request-123",
			}),
			level:   LogLevelError,
			message: "Error message",
			metadata: map[string]interface{}{
				"error": "Something went wrong",
			},
			validate: func(t *testing.T, log StructuredLog) {
				assert.Equal(t, LogLevelError, log.Level)
				assert.Equal(t, "test-request-123", log.RequestID)
			},
		},
		{
			name:    "log with correlation ID",
			ctx:     context.WithValue(context.Background(), "correlation_id", "corr-123"),
			level:   LogLevelDebug,
			message: "Debug message",
			metadata: map[string]interface{}{
				"debug": true,
			},
			validate: func(t *testing.T, log StructuredLog) {
				assert.Equal(t, LogLevelDebug, log.Level)
				assert.Equal(t, "corr-123", log.CorrelationID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				logger.Log(tt.ctx, tt.level, tt.message, tt.metadata)
			})

			// Parse the JSON output
			var log StructuredLog
			err := json.Unmarshal([]byte(strings.TrimSpace(output)), &log)
			require.NoError(t, err)

			// Validate basic fields
			assert.Equal(t, tt.level, log.Level)
			assert.Equal(t, tt.message, log.Message)
			assert.Greater(t, log.Timestamp, int64(0))

			// Run custom validation
			if tt.validate != nil {
				tt.validate(t, log)
			}
		})
	}
}

func TestLogger_ConvenienceMethods(t *testing.T) {
	logger := NewLogger("test-service")
	ctx := context.Background()

	tests := []struct {
		name     string
		method   func(context.Context, string, map[string]interface{})
		expected string
	}{
		{
			name:     "Debug",
			method:   logger.Debug,
			expected: LogLevelDebug,
		},
		{
			name:     "Info",
			method:   logger.Info,
			expected: LogLevelInfo,
		},
		{
			name:     "Warn",
			method:   logger.Warn,
			expected: LogLevelWarn,
		},
		{
			name:     "Error",
			method:   logger.Error,
			expected: LogLevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				tt.method(ctx, "Test message", map[string]interface{}{"test": true})
			})

			var log StructuredLog
			err := json.Unmarshal([]byte(strings.TrimSpace(output)), &log)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, log.Level)
			assert.Equal(t, "Test message", log.Message)
			assert.Equal(t, true, log.Metadata["test"])
		})
	}
}

func TestLogMetric(t *testing.T) {
	ctx := context.Background()
	oldServiceName := os.Getenv("SERVICE_NAME")
	defer os.Setenv("SERVICE_NAME", oldServiceName)

	tests := []struct {
		name        string
		serviceName string
		metricName  string
		metadata    map[string]interface{}
		validate    func(t *testing.T, emf map[string]interface{})
	}{
		{
			name:        "basic metric",
			serviceName: "test-service",
			metricName:  "TestMetric",
			metadata: map[string]interface{}{
				"dimension1": "value1",
				"dimension2": 123,
			},
			validate: func(t *testing.T, emf map[string]interface{}) {
				assert.Equal(t, "test-service", emf["Service"])
				assert.Equal(t, float64(1), emf["TestMetric"])
				assert.Equal(t, "value1", emf["dimension1"])
				assert.Equal(t, float64(123), emf["dimension2"])

				// Check CloudWatch EMF structure
				aws, ok := emf["_aws"].(map[string]interface{})
				require.True(t, ok)

				metrics, ok := aws["CloudWatchMetrics"].([]interface{})
				require.True(t, ok)
				require.Len(t, metrics, 1)

				metric := metrics[0].(map[string]interface{})
				assert.Equal(t, "Streamer", metric["Namespace"])
			},
		},
		{
			name:        "metric without service name",
			serviceName: "",
			metricName:  "NoServiceMetric",
			metadata:    nil,
			validate: func(t *testing.T, emf map[string]interface{}) {
				assert.Equal(t, "", emf["Service"])
				assert.Equal(t, float64(1), emf["NoServiceMetric"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SERVICE_NAME", tt.serviceName)

			output := captureStdout(func() {
				LogMetric(ctx, tt.metricName, tt.metadata)
			})

			// Parse the JSON output
			var emf map[string]interface{}
			err := json.Unmarshal([]byte(strings.TrimSpace(output)), &emf)
			require.NoError(t, err)

			if tt.validate != nil {
				tt.validate(t, emf)
			}
		})
	}
}

func TestExtractRequestID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "no lambda context",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name: "with lambda context",
			ctx: lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{
				AwsRequestID: "lambda-request-123",
			}),
			expected: "lambda-request-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestID := extractRequestID(tt.ctx)
			assert.Equal(t, tt.expected, requestID)
		})
	}
}

func TestGetTimestamp(t *testing.T) {
	before := time.Now().UnixNano() / 1e6
	timestamp := getTimestamp()
	after := time.Now().UnixNano() / 1e6

	assert.GreaterOrEqual(t, timestamp, before)
	assert.LessOrEqual(t, timestamp, after)
}

func TestLoggerWithEmptyServiceName(t *testing.T) {
	logger := NewLogger("")

	output := captureStdout(func() {
		logger.Info(context.Background(), "Test", nil)
	})

	var log StructuredLog
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &log)
	require.NoError(t, err)

	// Service should not be in metadata when service name is empty
	_, hasService := log.Metadata["service"]
	assert.False(t, hasService)
}

func TestLoggerWithComplexMetadata(t *testing.T) {
	logger := NewLogger("complex-service")

	metadata := map[string]interface{}{
		"string": "value",
		"number": 42,
		"float":  3.14,
		"bool":   true,
		"nil":    nil,
		"array":  []string{"a", "b", "c"},
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	output := captureStdout(func() {
		logger.Info(context.Background(), "Complex metadata test", metadata)
	})

	var log StructuredLog
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &log)
	require.NoError(t, err)

	assert.Equal(t, "value", log.Metadata["string"])
	assert.Equal(t, float64(42), log.Metadata["number"])
	assert.Equal(t, 3.14, log.Metadata["float"])
	assert.Equal(t, true, log.Metadata["bool"])
	assert.Nil(t, log.Metadata["nil"])

	array, ok := log.Metadata["array"].([]interface{})
	require.True(t, ok)
	assert.Len(t, array, 3)

	nested, ok := log.Metadata["nested"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "value", nested["key"])
}

func TestCorrelationIDTypes(t *testing.T) {
	logger := NewLogger("test")

	tests := []struct {
		name          string
		correlationID interface{}
		expected      string
	}{
		{
			name:          "string correlation ID",
			correlationID: "corr-123",
			expected:      "corr-123",
		},
		{
			name:          "non-string correlation ID",
			correlationID: 123,
			expected:      "",
		},
		{
			name:          "nil correlation ID",
			correlationID: nil,
			expected:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), "correlation_id", tt.correlationID)

			output := captureStdout(func() {
				logger.Info(ctx, "Test", nil)
			})

			var log StructuredLog
			err := json.Unmarshal([]byte(strings.TrimSpace(output)), &log)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, log.CorrelationID)
		})
	}
}
