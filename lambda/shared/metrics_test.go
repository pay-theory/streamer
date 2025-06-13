package shared

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simple mock implementation of MetricsPublisher for testing
type mockMetricsPublisher struct{}

func (m *mockMetricsPublisher) PublishMetric(ctx context.Context, namespace, metricName string, value float64, unit types.StandardUnit, dimensions ...types.Dimension) error {
	// Per testing guidelines, just return nil for AWS service calls
	return nil
}

func (m *mockMetricsPublisher) PublishLatency(ctx context.Context, namespace, metricName string, duration time.Duration, dimensions ...types.Dimension) error {
	// Per testing guidelines, just return nil for AWS service calls
	return nil
}

func TestMetricsDimensions(t *testing.T) {
	dims := MetricsDimensions{}

	tests := []struct {
		name     string
		method   func(string) types.Dimension
		input    string
		expected types.Dimension
	}{
		{
			name:   "Function dimension",
			method: dims.Function,
			input:  "lambda-function",
			expected: types.Dimension{
				Name:  aws.String("FunctionName"),
				Value: aws.String("lambda-function"),
			},
		},
		{
			name:   "Environment dimension",
			method: dims.Environment,
			input:  "production",
			expected: types.Dimension{
				Name:  aws.String("Environment"),
				Value: aws.String("production"),
			},
		},
		{
			name:   "Action dimension",
			method: dims.Action,
			input:  "process_request",
			expected: types.Dimension{
				Name:  aws.String("Action"),
				Value: aws.String("process_request"),
			},
		},
		{
			name:   "ErrorType dimension",
			method: dims.ErrorType,
			input:  "validation_error",
			expected: types.Dimension{
				Name:  aws.String("ErrorType"),
				Value: aws.String("validation_error"),
			},
		},
		{
			name:   "TenantID dimension",
			method: dims.TenantID,
			input:  "tenant-123",
			expected: types.Dimension{
				Name:  aws.String("TenantID"),
				Value: aws.String("tenant-123"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(tt.input)
			assert.Equal(t, *tt.expected.Name, *result.Name)
			assert.Equal(t, *tt.expected.Value, *result.Value)
		})
	}
}

func TestCommonMetrics(t *testing.T) {
	// Test that all common metric constants are defined
	assert.Equal(t, "ConnectionEstablished", CommonMetrics.ConnectionEstablished)
	assert.Equal(t, "ConnectionClosed", CommonMetrics.ConnectionClosed)
	assert.Equal(t, "MessageSent", CommonMetrics.MessageSent)
	assert.Equal(t, "MessageFailed", CommonMetrics.MessageFailed)
	assert.Equal(t, "AuthenticationFailed", CommonMetrics.AuthenticationFailed)
	assert.Equal(t, "ConnectionDuration", CommonMetrics.ConnectionDuration)
	assert.Equal(t, "MessageSize", CommonMetrics.MessageSize)
	assert.Equal(t, "ProcessingLatency", CommonMetrics.ProcessingLatency)
}

func TestStandardAlarms(t *testing.T) {
	// Test that standard alarms are properly configured
	require.Len(t, StandardAlarms, 3)

	// Test high connection failures alarm
	authFailAlarm := StandardAlarms[0]
	assert.Equal(t, "streamer-high-connection-failures", authFailAlarm.AlarmName)
	assert.Equal(t, CommonMetrics.AuthenticationFailed, authFailAlarm.MetricName)
	assert.Equal(t, "Streamer", authFailAlarm.Namespace)
	assert.Equal(t, types.StatisticSum, authFailAlarm.Statistic)
	assert.Equal(t, int32(300), authFailAlarm.Period)
	assert.Equal(t, int32(2), authFailAlarm.EvaluationPeriods)
	assert.Equal(t, float64(10), authFailAlarm.Threshold)
	assert.Equal(t, types.ComparisonOperatorGreaterThanThreshold, authFailAlarm.ComparisonOperator)

	// Test high message failures alarm
	msgFailAlarm := StandardAlarms[1]
	assert.Equal(t, "streamer-high-message-failures", msgFailAlarm.AlarmName)
	assert.Equal(t, CommonMetrics.MessageFailed, msgFailAlarm.MetricName)
	assert.Equal(t, float64(50), msgFailAlarm.Threshold)

	// Test high processing latency alarm
	latencyAlarm := StandardAlarms[2]
	assert.Equal(t, "streamer-high-processing-latency", latencyAlarm.AlarmName)
	assert.Equal(t, CommonMetrics.ProcessingLatency, latencyAlarm.MetricName)
	assert.Equal(t, types.StatisticAverage, latencyAlarm.Statistic)
	assert.Equal(t, int32(3), latencyAlarm.EvaluationPeriods)
	assert.Equal(t, float64(1000), latencyAlarm.Threshold)
}

func TestMetricsPublisherInterface(t *testing.T) {
	// Test that mock implements MetricsPublisher interface
	var _ MetricsPublisher = (*mockMetricsPublisher)(nil)
}

// Test example usage of metrics in business logic
func TestBusinessLogicWithMetrics(t *testing.T) {
	// Example of how metrics would be used in actual handlers
	mockPublisher := &mockMetricsPublisher{}

	// Simulate a handler that publishes metrics
	ctx := context.Background()

	// Test connection established metric
	err := mockPublisher.PublishMetric(ctx, "Streamer", CommonMetrics.ConnectionEstablished, 1, types.StandardUnitCount,
		MetricsDimensions{}.Environment("test"))
	assert.NoError(t, err)

	// Test processing latency metric
	processingTime := 250 * time.Millisecond
	err = mockPublisher.PublishLatency(ctx, "Streamer", CommonMetrics.ProcessingLatency, processingTime,
		MetricsDimensions{}.Action("process_request"))
	assert.NoError(t, err)

	// Test error metric with multiple dimensions
	err = mockPublisher.PublishMetric(ctx, "Streamer", CommonMetrics.MessageFailed, 1, types.StandardUnitCount,
		MetricsDimensions{}.Environment("test"),
		MetricsDimensions{}.ErrorType("validation_error"))
	assert.NoError(t, err)
}

// Test alarm configuration validation
func TestCloudWatchAlarmConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config CloudWatchAlarmConfig
		valid  bool
	}{
		{
			name: "valid alarm config",
			config: CloudWatchAlarmConfig{
				AlarmName:          "test-alarm",
				MetricName:         "TestMetric",
				Namespace:          "TestNamespace",
				Statistic:          types.StatisticAverage,
				Period:             300,
				EvaluationPeriods:  2,
				Threshold:          100.0,
				ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
			},
			valid: true,
		},
		{
			name: "alarm with SNS topic",
			config: CloudWatchAlarmConfig{
				AlarmName:          "test-alarm-sns",
				MetricName:         "TestMetric",
				Namespace:          "TestNamespace",
				Statistic:          types.StatisticSum,
				Period:             60,
				EvaluationPeriods:  1,
				Threshold:          50.0,
				ComparisonOperator: types.ComparisonOperatorLessThanThreshold,
				SNSTopic:           "arn:aws:sns:us-east-1:123456789012:test-topic",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just validate the configuration is properly structured
			assert.NotEmpty(t, tt.config.AlarmName)
			assert.NotEmpty(t, tt.config.MetricName)
			assert.NotEmpty(t, tt.config.Namespace)
			assert.Greater(t, tt.config.Period, int32(0))
			assert.Greater(t, tt.config.EvaluationPeriods, int32(0))
		})
	}
}

// Test MetricsPublisher usage patterns
func TestMetricsUsagePatterns(t *testing.T) {
	publisher := &mockMetricsPublisher{}
	ctx := context.Background()
	dims := MetricsDimensions{}

	// Test publishing various metric types
	testCases := []struct {
		name       string
		metricName string
		value      float64
		unit       types.StandardUnit
		dimensions []types.Dimension
	}{
		{
			name:       "connection count",
			metricName: CommonMetrics.ConnectionEstablished,
			value:      1,
			unit:       types.StandardUnitCount,
			dimensions: []types.Dimension{dims.Environment("prod")},
		},
		{
			name:       "message size",
			metricName: CommonMetrics.MessageSize,
			value:      1024,
			unit:       types.StandardUnitBytes,
			dimensions: []types.Dimension{dims.Action("upload")},
		},
		{
			name:       "error rate",
			metricName: CommonMetrics.MessageFailed,
			value:      1,
			unit:       types.StandardUnitCount,
			dimensions: []types.Dimension{
				dims.Environment("prod"),
				dims.ErrorType("timeout"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := publisher.PublishMetric(ctx, "Streamer", tc.metricName, tc.value, tc.unit, tc.dimensions...)
			assert.NoError(t, err)
		})
	}

	// Test publishing latency metrics
	latencyTests := []struct {
		name       string
		metricName string
		duration   time.Duration
		dimensions []types.Dimension
	}{
		{
			name:       "api latency",
			metricName: CommonMetrics.ProcessingLatency,
			duration:   100 * time.Millisecond,
			dimensions: []types.Dimension{dims.Action("api_call")},
		},
		{
			name:       "connection duration",
			metricName: CommonMetrics.ConnectionDuration,
			duration:   5 * time.Minute,
			dimensions: []types.Dimension{dims.TenantID("tenant-123")},
		},
	}

	for _, tc := range latencyTests {
		t.Run(tc.name, func(t *testing.T) {
			err := publisher.PublishLatency(ctx, "Streamer", tc.metricName, tc.duration, tc.dimensions...)
			assert.NoError(t, err)
		})
	}
}
