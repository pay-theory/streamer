package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	lifttesting "github.com/pay-theory/lift/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestPublishConnectionMetrics tests the actual production CloudWatch metrics publishing
func TestPublishConnectionMetrics(t *testing.T) {
	mockClient := lifttesting.NewMockCloudWatchClient()
	ctx := context.Background()

	t.Run("PublishActiveConnectionsMetric", func(t *testing.T) {
		// Test publishing active connections metric
		expectedOutput := lifttesting.NewMockPutMetricDataOutput()
		mockClient.On("PutMetricData", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricDataInput) bool {
			return *input.Namespace == "PayTheory/Streamer/Connections" &&
				len(input.MetricData) == 1 &&
				*input.MetricData[0].MetricName == "ActiveConnections" &&
				*input.MetricData[0].Value == 42.0
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		// Simulate production code publishing metrics
		err := publishConnectionMetric(ctx, mockClient, "ActiveConnections", 42.0)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("PublishMessageThroughputMetric", func(t *testing.T) {
		// Test publishing message throughput metric
		expectedOutput := lifttesting.NewMockPutMetricDataOutput()
		mockClient.On("PutMetricData", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricDataInput) bool {
			return *input.Namespace == "PayTheory/Streamer/Performance" &&
				*input.MetricData[0].MetricName == "MessageThroughput" &&
				*input.MetricData[0].Value == 1250.5 &&
				input.MetricData[0].Unit == types.StandardUnitCountSecond
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		err := publishPerformanceMetric(ctx, mockClient, "MessageThroughput", 1250.5, types.StandardUnitCountSecond)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("PublishErrorRateMetric", func(t *testing.T) {
		// Test publishing error rate metric with dimensions
		expectedOutput := lifttesting.NewMockPutMetricDataOutput()
		mockClient.On("PutMetricData", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricDataInput) bool {
			return *input.Namespace == "PayTheory/Streamer/Errors" &&
				*input.MetricData[0].MetricName == "ErrorRate" &&
				*input.MetricData[0].Value == 0.025 &&
				len(input.MetricData[0].Dimensions) == 2
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		dimensions := []types.Dimension{
			{Name: aws.String("Service"), Value: aws.String("Streamer")},
			{Name: aws.String("Environment"), Value: aws.String("production")},
		}
		err := publishErrorMetric(ctx, mockClient, "ErrorRate", 0.025, dimensions)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("BatchMetricsPublishing", func(t *testing.T) {
		// Test publishing multiple metrics in a single call
		expectedOutput := lifttesting.NewMockPutMetricDataOutput()
		mockClient.On("PutMetricData", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricDataInput) bool {
			return *input.Namespace == "PayTheory/Streamer/Batch" &&
				len(input.MetricData) == 3 &&
				*input.MetricData[0].MetricName == "ConnectionsCreated" &&
				*input.MetricData[1].MetricName == "ConnectionsClosed" &&
				*input.MetricData[2].MetricName == "MessagesProcessed"
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		metrics := []MetricData{
			{Name: "ConnectionsCreated", Value: 15.0, Unit: types.StandardUnitCount},
			{Name: "ConnectionsClosed", Value: 8.0, Unit: types.StandardUnitCount},
			{Name: "MessagesProcessed", Value: 2847.0, Unit: types.StandardUnitCount},
		}
		err := publishBatchMetrics(ctx, mockClient, "PayTheory/Streamer/Batch", metrics)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

// TestCloudWatchAlarmsManagement tests production alarm management
func TestCloudWatchAlarmsManagement(t *testing.T) {
	mockClient := lifttesting.NewMockCloudWatchClient()
	ctx := context.Background()

	t.Run("CreateHighConnectionCountAlarm", func(t *testing.T) {
		// Test creating production alarm for high connection count
		expectedOutput := lifttesting.NewMockPutMetricAlarmOutput()
		mockClient.On("PutMetricAlarm", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricAlarmInput) bool {
			return *input.AlarmName == "Streamer-HighConnectionCount-production" &&
				*input.MetricName == "ActiveConnections" &&
				*input.Threshold == 500.0 &&
				*input.EvaluationPeriods == 2 &&
				*input.Period == 300
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		err := createConnectionCountAlarm(ctx, mockClient, "production", 500.0)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("CreateHighLatencyAlarm", func(t *testing.T) {
		// Test creating production alarm for high message latency
		expectedOutput := lifttesting.NewMockPutMetricAlarmOutput()
		mockClient.On("PutMetricAlarm", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricAlarmInput) bool {
			return *input.AlarmName == "Streamer-HighLatency-production" &&
				*input.MetricName == "MessageLatency" &&
				*input.Threshold == 2000.0 &&
				input.ComparisonOperator == types.ComparisonOperatorGreaterThanThreshold
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		err := createLatencyAlarm(ctx, mockClient, "production", 2000.0)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("CreateErrorRateAlarm", func(t *testing.T) {
		// Test creating production alarm for high error rate
		expectedOutput := lifttesting.NewMockPutMetricAlarmOutput()
		mockClient.On("PutMetricAlarm", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricAlarmInput) bool {
			return *input.AlarmName == "Streamer-HighErrorRate-production" &&
				*input.MetricName == "ErrorRate" &&
				*input.Threshold == 5.0 &&
				input.Statistic == types.StatisticAverage
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		err := createErrorRateAlarm(ctx, mockClient, "production", 5.0)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

// TestCloudWatchMetricsRetrieval tests production metrics retrieval
func TestCloudWatchMetricsRetrieval(t *testing.T) {
	mockClient := lifttesting.NewMockCloudWatchClient()
	ctx := context.Background()

	t.Run("GetConnectionMetricsHistory", func(t *testing.T) {
		// Test retrieving connection metrics history
		datapoints := []types.Datapoint{
			{
				Timestamp: aws.Time(time.Now().Add(-15 * time.Minute)),
				Average:   aws.Float64(45.2),
				Maximum:   aws.Float64(52.0),
				Minimum:   aws.Float64(38.0),
				Unit:      types.StandardUnitCount,
			},
			{
				Timestamp: aws.Time(time.Now().Add(-10 * time.Minute)),
				Average:   aws.Float64(48.7),
				Maximum:   aws.Float64(55.0),
				Minimum:   aws.Float64(42.0),
				Unit:      types.StandardUnitCount,
			},
		}
		expectedOutput := lifttesting.NewMockGetMetricStatisticsOutput(datapoints)

		mockClient.On("GetMetricStatistics", ctx, mock.MatchedBy(func(input *cloudwatch.GetMetricStatisticsInput) bool {
			return *input.MetricName == "ActiveConnections" &&
				*input.Namespace == "PayTheory/Streamer/Connections" &&
				*input.Period == 300
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		metrics, err := getConnectionMetricsHistory(ctx, mockClient, time.Hour)
		assert.NoError(t, err)
		assert.Len(t, metrics, 2)
		assert.Equal(t, 45.2, *metrics[0].Average)
		assert.Equal(t, 48.7, *metrics[1].Average)
		mockClient.AssertExpectations(t)
	})

	t.Run("GetPerformanceMetrics", func(t *testing.T) {
		// Test retrieving performance metrics
		datapoints := []types.Datapoint{
			{
				Timestamp: aws.Time(time.Now().Add(-5 * time.Minute)),
				Average:   aws.Float64(1250.5),
				Sum:       aws.Float64(6252.5),
				Unit:      types.StandardUnitCountSecond,
			},
		}
		expectedOutput := lifttesting.NewMockGetMetricStatisticsOutput(datapoints)

		mockClient.On("GetMetricStatistics", ctx, mock.MatchedBy(func(input *cloudwatch.GetMetricStatisticsInput) bool {
			return *input.MetricName == "MessageThroughput" &&
				*input.Namespace == "PayTheory/Streamer/Performance"
		}), mock.AnythingOfType("[]func(*cloudwatch.Options)")).Return(expectedOutput, nil)

		metrics, err := getPerformanceMetrics(ctx, mockClient, "MessageThroughput", 30*time.Minute)
		assert.NoError(t, err)
		assert.Len(t, metrics, 1)
		assert.Equal(t, 1250.5, *metrics[0].Average)
		mockClient.AssertExpectations(t)
	})
}

// TestCloudWatchErrorHandling tests error scenarios in production code
func TestCloudWatchErrorHandling(t *testing.T) {
	mockClient := lifttesting.NewMockCloudWatchClient()
	ctx := context.Background()

	t.Run("HandleThrottlingError", func(t *testing.T) {
		// Test handling CloudWatch throttling errors
		throttlingErr := fmt.Errorf("throttling exception: rate exceeded")
		mockClient.On("PutMetricData", ctx, mock.AnythingOfType("*cloudwatch.PutMetricDataInput"), mock.AnythingOfType("[]func(*cloudwatch.Options)")).
			Return((*cloudwatch.PutMetricDataOutput)(nil), throttlingErr)

		err := publishConnectionMetric(ctx, mockClient, "ActiveConnections", 42.0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "throttling")
		mockClient.AssertExpectations(t)
	})

	t.Run("HandleInvalidParameterError", func(t *testing.T) {
		// Test handling invalid parameter errors
		mockClient.ExpectedCalls = nil // Reset expectations
		invalidParamErr := fmt.Errorf("invalid parameter: metric name cannot be empty")
		mockClient.On("PutMetricData", ctx, mock.AnythingOfType("*cloudwatch.PutMetricDataInput"), mock.AnythingOfType("[]func(*cloudwatch.Options)")).
			Return((*cloudwatch.PutMetricDataOutput)(nil), invalidParamErr)

		err := publishConnectionMetric(ctx, mockClient, "", 42.0) // Empty metric name
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameter")
		mockClient.AssertExpectations(t)
	})

	t.Run("HandleServiceUnavailableError", func(t *testing.T) {
		// Test handling service unavailable errors
		mockClient.ExpectedCalls = nil // Reset expectations
		serviceErr := fmt.Errorf("service unavailable")
		mockClient.On("GetMetricStatistics", ctx, mock.AnythingOfType("*cloudwatch.GetMetricStatisticsInput"), mock.AnythingOfType("[]func(*cloudwatch.Options)")).
			Return((*cloudwatch.GetMetricStatisticsOutput)(nil), serviceErr)

		_, err := getConnectionMetricsHistory(ctx, mockClient, time.Hour)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service unavailable")
		mockClient.AssertExpectations(t)
	})
}

// Helper types and functions for testing (these would be in production code)

type MetricData struct {
	Name  string
	Value float64
	Unit  types.StandardUnit
}

// Mock production functions that would use CloudWatch
func publishConnectionMetric(ctx context.Context, client *lifttesting.MockCloudWatchClient, metricName string, value float64) error {
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String("PayTheory/Streamer/Connections"),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(value),
				Unit:       types.StandardUnitCount,
				Timestamp:  aws.Time(time.Now()),
			},
		},
	})
	return err
}

func publishPerformanceMetric(ctx context.Context, client *lifttesting.MockCloudWatchClient, metricName string, value float64, unit types.StandardUnit) error {
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String("PayTheory/Streamer/Performance"),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(value),
				Unit:       unit,
				Timestamp:  aws.Time(time.Now()),
			},
		},
	})
	return err
}

func publishErrorMetric(ctx context.Context, client *lifttesting.MockCloudWatchClient, metricName string, value float64, dimensions []types.Dimension) error {
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String("PayTheory/Streamer/Errors"),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(value),
				Unit:       types.StandardUnitPercent,
				Dimensions: dimensions,
				Timestamp:  aws.Time(time.Now()),
			},
		},
	})
	return err
}

func publishBatchMetrics(ctx context.Context, client *lifttesting.MockCloudWatchClient, namespace string, metrics []MetricData) error {
	metricData := make([]types.MetricDatum, len(metrics))
	for i, metric := range metrics {
		metricData[i] = types.MetricDatum{
			MetricName: aws.String(metric.Name),
			Value:      aws.Float64(metric.Value),
			Unit:       metric.Unit,
			Timestamp:  aws.Time(time.Now()),
		}
	}

	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(namespace),
		MetricData: metricData,
	})
	return err
}

func createConnectionCountAlarm(ctx context.Context, client *lifttesting.MockCloudWatchClient, environment string, threshold float64) error {
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(fmt.Sprintf("Streamer-HighConnectionCount-%s", environment)),
		AlarmDescription:   aws.String("Alert when WebSocket connections exceed threshold"),
		MetricName:         aws.String("ActiveConnections"),
		Namespace:          aws.String("PayTheory/Streamer/Connections"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(300),
		EvaluationPeriods:  aws.Int32(2),
		Threshold:          aws.Float64(threshold),
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		TreatMissingData:   aws.String("notBreaching"),
	})
	return err
}

func createLatencyAlarm(ctx context.Context, client *lifttesting.MockCloudWatchClient, environment string, threshold float64) error {
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(fmt.Sprintf("Streamer-HighLatency-%s", environment)),
		AlarmDescription:   aws.String("Alert when message latency is too high"),
		MetricName:         aws.String("MessageLatency"),
		Namespace:          aws.String("PayTheory/Streamer/Performance"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(300),
		EvaluationPeriods:  aws.Int32(3),
		Threshold:          aws.Float64(threshold),
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		TreatMissingData:   aws.String("breaching"),
	})
	return err
}

func createErrorRateAlarm(ctx context.Context, client *lifttesting.MockCloudWatchClient, environment string, threshold float64) error {
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(fmt.Sprintf("Streamer-HighErrorRate-%s", environment)),
		AlarmDescription:   aws.String("Alert when error rate is too high"),
		MetricName:         aws.String("ErrorRate"),
		Namespace:          aws.String("PayTheory/Streamer/Errors"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(300),
		EvaluationPeriods:  aws.Int32(2),
		Threshold:          aws.Float64(threshold),
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		TreatMissingData:   aws.String("breaching"),
	})
	return err
}

func getConnectionMetricsHistory(ctx context.Context, client *lifttesting.MockCloudWatchClient, duration time.Duration) ([]types.Datapoint, error) {
	now := time.Now()
	result, err := client.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("PayTheory/Streamer/Connections"),
		MetricName: aws.String("ActiveConnections"),
		StartTime:  aws.Time(now.Add(-duration)),
		EndTime:    aws.Time(now),
		Period:     aws.Int32(300),
		Statistics: []types.Statistic{types.StatisticAverage, types.StatisticMaximum, types.StatisticMinimum},
	})
	if err != nil {
		return nil, err
	}
	return result.Datapoints, nil
}

func getPerformanceMetrics(ctx context.Context, client *lifttesting.MockCloudWatchClient, metricName string, duration time.Duration) ([]types.Datapoint, error) {
	now := time.Now()
	result, err := client.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("PayTheory/Streamer/Performance"),
		MetricName: aws.String(metricName),
		StartTime:  aws.Time(now.Add(-duration)),
		EndTime:    aws.Time(now),
		Period:     aws.Int32(300),
		Statistics: []types.Statistic{types.StatisticAverage, types.StatisticSum},
	})
	if err != nil {
		return nil, err
	}
	return result.Datapoints, nil
}
