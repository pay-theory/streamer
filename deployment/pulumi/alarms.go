package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func createCloudWatchAlarms(ctx *pulumi.Context, environment string, lambdaFunctions map[string]*lambda.Function) error {
	// Common alarm settings
	alarmActionsEnabled := environment != "dev"

	// Create high connection failure alarm
	_, err := cloudwatch.NewMetricAlarm(ctx, "high-connection-failures", &cloudwatch.MetricAlarmArgs{
		Name:               pulumi.Sprintf("streamer-%s-high-connection-failures", environment),
		AlarmDescription:   pulumi.String("Alert when authentication failures exceed threshold"),
		MetricName:         pulumi.String("AuthenticationFailed"),
		Namespace:          pulumi.String("Streamer"),
		Statistic:          pulumi.String("Sum"),
		Period:             pulumi.Int(300), // 5 minutes
		EvaluationPeriods:  pulumi.Int(2),
		Threshold:          pulumi.Float64(getThreshold(environment, "auth_failures", 10)),
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		ActionsEnabled:     pulumi.Bool(alarmActionsEnabled),
		TreatMissingData:   pulumi.String("notBreaching"),

		Dimensions: pulumi.StringMap{
			"Environment": pulumi.String(environment),
		},

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Severity":    pulumi.String("high"),
		},
	})
	if err != nil {
		return err
	}

	// Create high message failure alarm
	_, err = cloudwatch.NewMetricAlarm(ctx, "high-message-failures", &cloudwatch.MetricAlarmArgs{
		Name:               pulumi.Sprintf("streamer-%s-high-message-failures", environment),
		AlarmDescription:   pulumi.String("Alert when message failures exceed threshold"),
		MetricName:         pulumi.String("MessageFailed"),
		Namespace:          pulumi.String("Streamer"),
		Statistic:          pulumi.String("Sum"),
		Period:             pulumi.Int(300),
		EvaluationPeriods:  pulumi.Int(2),
		Threshold:          pulumi.Float64(getThreshold(environment, "message_failures", 50)),
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		ActionsEnabled:     pulumi.Bool(alarmActionsEnabled),
		TreatMissingData:   pulumi.String("notBreaching"),

		Dimensions: pulumi.StringMap{
			"Environment": pulumi.String(environment),
		},

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Severity":    pulumi.String("high"),
		},
	})
	if err != nil {
		return err
	}

	// Create high processing latency alarm
	_, err = cloudwatch.NewMetricAlarm(ctx, "high-processing-latency", &cloudwatch.MetricAlarmArgs{
		Name:               pulumi.Sprintf("streamer-%s-high-processing-latency", environment),
		AlarmDescription:   pulumi.String("Alert when processing latency is too high"),
		MetricName:         pulumi.String("ProcessingLatency"),
		Namespace:          pulumi.String("Streamer"),
		Statistic:          pulumi.String("Average"),
		Period:             pulumi.Int(300),
		EvaluationPeriods:  pulumi.Int(3),
		Threshold:          pulumi.Float64(getThreshold(environment, "latency_ms", 1000)), // 1 second
		ComparisonOperator: pulumi.String("GreaterThanThreshold"),
		ActionsEnabled:     pulumi.Bool(alarmActionsEnabled),
		TreatMissingData:   pulumi.String("notBreaching"),

		Dimensions: pulumi.StringMap{
			"Environment": pulumi.String(environment),
		},

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Severity":    pulumi.String("medium"),
		},
	})
	if err != nil {
		return err
	}

	// Create Lambda function-specific alarms
	for name, function := range lambdaFunctions {
		// Error rate alarm
		_, err = cloudwatch.NewMetricAlarm(ctx, fmt.Sprintf("%s-error-rate", name), &cloudwatch.MetricAlarmArgs{
			Name:               pulumi.Sprintf("streamer-%s-%s-error-rate", environment, name),
			AlarmDescription:   pulumi.Sprintf("Alert when %s Lambda error rate is high", name),
			MetricName:         pulumi.String("Errors"),
			Namespace:          pulumi.String("AWS/Lambda"),
			Statistic:          pulumi.String("Average"),
			Period:             pulumi.Int(300),
			EvaluationPeriods:  pulumi.Int(2),
			Threshold:          pulumi.Float64(getThreshold(environment, "error_rate", 0.05)), // 5%
			ComparisonOperator: pulumi.String("GreaterThanThreshold"),
			ActionsEnabled:     pulumi.Bool(alarmActionsEnabled),
			TreatMissingData:   pulumi.String("notBreaching"),

			Dimensions: pulumi.StringMap{
				"FunctionName": function.Name,
			},

			Tags: pulumi.StringMap{
				"Environment": pulumi.String(environment),
				"Service":     pulumi.String("streamer"),
				"Function":    pulumi.String(name),
				"Severity":    pulumi.String("high"),
			},
		})
		if err != nil {
			return err
		}

		// Duration alarm (cold starts)
		_, err = cloudwatch.NewMetricAlarm(ctx, fmt.Sprintf("%s-duration", name), &cloudwatch.MetricAlarmArgs{
			Name:               pulumi.Sprintf("streamer-%s-%s-duration", environment, name),
			AlarmDescription:   pulumi.Sprintf("Alert when %s Lambda duration is high", name),
			MetricName:         pulumi.String("Duration"),
			Namespace:          pulumi.String("AWS/Lambda"),
			Statistic:          pulumi.String("Average"),
			Period:             pulumi.Int(300),
			EvaluationPeriods:  pulumi.Int(3),
			Threshold:          pulumi.Float64(getThreshold(environment, "duration_ms", 3000)), // 3 seconds
			ComparisonOperator: pulumi.String("GreaterThanThreshold"),
			ActionsEnabled:     pulumi.Bool(alarmActionsEnabled),
			TreatMissingData:   pulumi.String("notBreaching"),

			Dimensions: pulumi.StringMap{
				"FunctionName": function.Name,
			},

			Tags: pulumi.StringMap{
				"Environment": pulumi.String(environment),
				"Service":     pulumi.String("streamer"),
				"Function":    pulumi.String(name),
				"Severity":    pulumi.String("medium"),
			},
		})
		if err != nil {
			return err
		}

		// Concurrent executions alarm
		_, err = cloudwatch.NewMetricAlarm(ctx, fmt.Sprintf("%s-concurrent-executions", name), &cloudwatch.MetricAlarmArgs{
			Name:               pulumi.Sprintf("streamer-%s-%s-concurrent-executions", environment, name),
			AlarmDescription:   pulumi.Sprintf("Alert when %s Lambda concurrent executions are high", name),
			MetricName:         pulumi.String("ConcurrentExecutions"),
			Namespace:          pulumi.String("AWS/Lambda"),
			Statistic:          pulumi.String("Maximum"),
			Period:             pulumi.Int(60),
			EvaluationPeriods:  pulumi.Int(5),
			Threshold:          pulumi.Float64(getThreshold(environment, "concurrent_executions", 80)), // 80% of reserved
			ComparisonOperator: pulumi.String("GreaterThanThreshold"),
			ActionsEnabled:     pulumi.Bool(alarmActionsEnabled),
			TreatMissingData:   pulumi.String("notBreaching"),

			Dimensions: pulumi.StringMap{
				"FunctionName": function.Name,
			},

			Tags: pulumi.StringMap{
				"Environment": pulumi.String(environment),
				"Service":     pulumi.String("streamer"),
				"Function":    pulumi.String(name),
				"Severity":    pulumi.String("low"),
			},
		})
		if err != nil {
			return err
		}

		// Throttles alarm
		_, err = cloudwatch.NewMetricAlarm(ctx, fmt.Sprintf("%s-throttles", name), &cloudwatch.MetricAlarmArgs{
			Name:               pulumi.Sprintf("streamer-%s-%s-throttles", environment, name),
			AlarmDescription:   pulumi.Sprintf("Alert when %s Lambda is being throttled", name),
			MetricName:         pulumi.String("Throttles"),
			Namespace:          pulumi.String("AWS/Lambda"),
			Statistic:          pulumi.String("Sum"),
			Period:             pulumi.Int(300),
			EvaluationPeriods:  pulumi.Int(1),
			Threshold:          pulumi.Float64(getThreshold(environment, "throttles", 10)),
			ComparisonOperator: pulumi.String("GreaterThanThreshold"),
			ActionsEnabled:     pulumi.Bool(alarmActionsEnabled),
			TreatMissingData:   pulumi.String("notBreaching"),

			Dimensions: pulumi.StringMap{
				"FunctionName": function.Name,
			},

			Tags: pulumi.StringMap{
				"Environment": pulumi.String(environment),
				"Service":     pulumi.String("streamer"),
				"Function":    pulumi.String(name),
				"Severity":    pulumi.String("high"),
			},
		})
		if err != nil {
			return err
		}
	}

	// DynamoDB alarms would go here
	// API Gateway alarms would go here

	return nil
}

// getThreshold returns environment-specific thresholds
func getThreshold(environment, metric string, defaultValue float64) float64 {
	// You can customize thresholds per environment
	thresholds := map[string]map[string]float64{
		"dev": {
			"auth_failures":         5,
			"message_failures":      20,
			"latency_ms":            2000,
			"error_rate":            0.1,
			"duration_ms":           5000,
			"concurrent_executions": 50,
			"throttles":             5,
		},
		"staging": {
			"auth_failures":         10,
			"message_failures":      50,
			"latency_ms":            1000,
			"error_rate":            0.05,
			"duration_ms":           3000,
			"concurrent_executions": 80,
			"throttles":             10,
		},
		"production": {
			"auth_failures":         20,
			"message_failures":      100,
			"latency_ms":            500,
			"error_rate":            0.01,
			"duration_ms":           1000,
			"concurrent_executions": 90,
			"throttles":             20,
		},
	}

	if envThresholds, ok := thresholds[environment]; ok {
		if value, ok := envThresholds[metric]; ok {
			return value
		}
	}

	return defaultValue
}
