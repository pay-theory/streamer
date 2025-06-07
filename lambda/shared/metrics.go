package shared

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// MetricsPublisher defines the CloudWatch metrics interface
type MetricsPublisher interface {
	PublishMetric(ctx context.Context, namespace, metricName string, value float64, unit types.StandardUnit, dimensions ...types.Dimension) error
	PublishLatency(ctx context.Context, namespace, metricName string, duration time.Duration, dimensions ...types.Dimension) error
}

// CloudWatchMetrics implements MetricsPublisher using CloudWatch
type CloudWatchMetrics struct {
	client    *cloudwatch.Client
	namespace string
}

// NewCloudWatchMetrics creates a new CloudWatch metrics client
func NewCloudWatchMetrics(cfg aws.Config, namespace string) *CloudWatchMetrics {
	return &CloudWatchMetrics{
		client:    cloudwatch.NewFromConfig(cfg),
		namespace: namespace,
	}
}

// PublishMetric publishes a single metric to CloudWatch
func (c *CloudWatchMetrics) PublishMetric(ctx context.Context, namespace, metricName string, value float64, unit types.StandardUnit, dimensions ...types.Dimension) error {
	if namespace == "" {
		namespace = c.namespace
	}

	input := &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(namespace),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(value),
				Unit:       unit,
				Timestamp:  aws.Time(time.Now()),
				Dimensions: dimensions,
			},
		},
	}

	_, err := c.client.PutMetricData(ctx, input)
	return err
}

// PublishLatency publishes a latency metric in milliseconds
func (c *CloudWatchMetrics) PublishLatency(ctx context.Context, namespace, metricName string, duration time.Duration, dimensions ...types.Dimension) error {
	return c.PublishMetric(ctx, namespace, metricName, float64(duration.Milliseconds()), types.StandardUnitMilliseconds, dimensions...)
}

// MetricsDimensions provides common dimension builders
type MetricsDimensions struct{}

// Function returns a dimension for Lambda function name
func (MetricsDimensions) Function(name string) types.Dimension {
	return types.Dimension{
		Name:  aws.String("FunctionName"),
		Value: aws.String(name),
	}
}

// Environment returns a dimension for environment
func (MetricsDimensions) Environment(env string) types.Dimension {
	return types.Dimension{
		Name:  aws.String("Environment"),
		Value: aws.String(env),
	}
}

// Action returns a dimension for action type
func (MetricsDimensions) Action(action string) types.Dimension {
	return types.Dimension{
		Name:  aws.String("Action"),
		Value: aws.String(action),
	}
}

// ErrorType returns a dimension for error type
func (MetricsDimensions) ErrorType(errType string) types.Dimension {
	return types.Dimension{
		Name:  aws.String("ErrorType"),
		Value: aws.String(errType),
	}
}

// TenantID returns a dimension for tenant
func (MetricsDimensions) TenantID(tenantID string) types.Dimension {
	return types.Dimension{
		Name:  aws.String("TenantID"),
		Value: aws.String(tenantID),
	}
}

// CommonMetrics provides metric name constants
var CommonMetrics = struct {
	ConnectionEstablished string
	ConnectionClosed      string
	MessageSent           string
	MessageFailed         string
	AuthenticationFailed  string
	ConnectionDuration    string
	MessageSize           string
	ProcessingLatency     string
}{
	ConnectionEstablished: "ConnectionEstablished",
	ConnectionClosed:      "ConnectionClosed",
	MessageSent:           "MessageSent",
	MessageFailed:         "MessageFailed",
	AuthenticationFailed:  "AuthenticationFailed",
	ConnectionDuration:    "ConnectionDuration",
	MessageSize:           "MessageSize",
	ProcessingLatency:     "ProcessingLatency",
}

// CloudWatchAlarmConfig represents alarm configuration
type CloudWatchAlarmConfig struct {
	AlarmName          string
	MetricName         string
	Namespace          string
	Statistic          types.Statistic
	Period             int32
	EvaluationPeriods  int32
	Threshold          float64
	ComparisonOperator types.ComparisonOperator
	AlarmDescription   string
	Dimensions         []types.Dimension
	SNSTopic           string
}

// CreateAlarm creates a CloudWatch alarm
func CreateAlarm(ctx context.Context, cwClient *cloudwatch.Client, config CloudWatchAlarmConfig) error {
	input := &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(config.AlarmName),
		MetricName:         aws.String(config.MetricName),
		Namespace:          aws.String(config.Namespace),
		Statistic:          config.Statistic,
		Period:             aws.Int32(config.Period),
		EvaluationPeriods:  aws.Int32(config.EvaluationPeriods),
		Threshold:          aws.Float64(config.Threshold),
		ComparisonOperator: config.ComparisonOperator,
		AlarmDescription:   aws.String(config.AlarmDescription),
		Dimensions:         config.Dimensions,
		ActionsEnabled:     aws.Bool(true),
	}

	if config.SNSTopic != "" {
		input.AlarmActions = []string{config.SNSTopic}
	}

	_, err := cwClient.PutMetricAlarm(ctx, input)
	return err
}

// StandardAlarms provides common alarm configurations
var StandardAlarms = []CloudWatchAlarmConfig{
	{
		AlarmName:          "streamer-high-connection-failures",
		MetricName:         CommonMetrics.AuthenticationFailed,
		Namespace:          "Streamer",
		Statistic:          types.StatisticSum,
		Period:             300, // 5 minutes
		EvaluationPeriods:  2,
		Threshold:          10,
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		AlarmDescription:   "Alert when authentication failures exceed threshold",
	},
	{
		AlarmName:          "streamer-high-message-failures",
		MetricName:         CommonMetrics.MessageFailed,
		Namespace:          "Streamer",
		Statistic:          types.StatisticSum,
		Period:             300,
		EvaluationPeriods:  2,
		Threshold:          50,
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		AlarmDescription:   "Alert when message failures exceed threshold",
	},
	{
		AlarmName:          "streamer-high-processing-latency",
		MetricName:         CommonMetrics.ProcessingLatency,
		Namespace:          "Streamer",
		Statistic:          types.StatisticAverage,
		Period:             300,
		EvaluationPeriods:  3,
		Threshold:          1000, // 1 second
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		AlarmDescription:   "Alert when processing latency is too high",
	},
}
