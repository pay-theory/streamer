package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func createLambdaFunctions(ctx *pulumi.Context, environment string, roles map[string]*iam.Role, tables map[string]*dynamodb.Table, logGroups map[string]*cloudwatch.LogGroup, jwtSecretArn pulumi.StringOutput) (map[string]*lambda.Function, error) {
	functions := make(map[string]*lambda.Function)

	cfg := config.New(ctx, "")

	// Lambda configuration
	memorySize := cfg.GetInt("lambdaMemorySize")
	if memorySize == 0 {
		memorySize = 3008 // 2 vCPUs worth of processing power
	}

	timeout := cfg.GetInt("lambdaTimeout")
	if timeout == 0 {
		timeout = 30
	}

	reservedConcurrency := cfg.GetInt("reservedConcurrency")
	if reservedConcurrency == 0 {
		reservedConcurrency = 100
	}

	// Connect Lambda
	connectFunc, err := lambda.NewFunction(ctx, "connect", &lambda.FunctionArgs{
		Name:        pulumi.Sprintf("streamer-connect-%s", environment),
		Description: pulumi.String("WebSocket connection handler"),
		Runtime:     pulumi.String("go1.x"),
		Handler:     pulumi.String("main"),
		Role:        roles["connect"].Arn,
		MemorySize:  pulumi.Int(memorySize),
		Timeout:     pulumi.Int(timeout),

		Environment: &lambda.FunctionEnvironmentArgs{
			Variables: pulumi.All(tables["connections"].Name, jwtSecretArn).ApplyT(
				func(args []interface{}) pulumi.StringMap {
					vars := pulumi.StringMap{
						"CONNECTIONS_TABLE": pulumi.String(args[0].(string)),
						"JWT_SECRET_ARN":    pulumi.String(args[1].(string)),
						"JWT_ISSUER":        pulumi.String(cfg.Get("jwtIssuer")),
						"ENVIRONMENT":       pulumi.String(environment),
						"LOG_LEVEL":         pulumi.String(getLogLevel(environment)),
						"METRICS_NAMESPACE": pulumi.String("Streamer"),
					}

					// Add allowed tenants if configured
					allowedTenants := cfg.Get("allowedTenants")
					if allowedTenants != "" {
						vars["ALLOWED_TENANTS"] = pulumi.String(allowedTenants)
					}

					return vars
				},
			).(pulumi.StringMapOutput),
		},

		TracingConfig: &lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},

		ReservedConcurrentExecutions: pulumi.Int(reservedConcurrency),

		// Code will be uploaded later
		Code: pulumi.NewFileArchive("../../lambda/connect/deployment.zip"),

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("connect"),
		},
	})
	if err != nil {
		return nil, err
	}
	functions["connect"] = connectFunc

	// Log group subscription for Connect function
	err = createLogGroupSubscription(ctx, "connect", connectFunc, logGroups["connect"])
	if err != nil {
		return nil, err
	}

	// Disconnect Lambda
	disconnectFunc, err := lambda.NewFunction(ctx, "disconnect", &lambda.FunctionArgs{
		Name:        pulumi.Sprintf("streamer-disconnect-%s", environment),
		Description: pulumi.String("WebSocket disconnection handler"),
		Runtime:     pulumi.String("go1.x"),
		Handler:     pulumi.String("main"),
		Role:        roles["disconnect"].Arn,
		MemorySize:  pulumi.Int(memorySize),
		Timeout:     pulumi.Int(timeout),

		Environment: &lambda.FunctionEnvironmentArgs{
			Variables: pulumi.All(
				tables["connections"].Name,
				tables["subscriptions"].Name,
				tables["requests"].Name,
			).ApplyT(
				func(args []interface{}) pulumi.StringMap {
					return pulumi.StringMap{
						"CONNECTIONS_TABLE":   pulumi.String(args[0].(string)),
						"SUBSCRIPTIONS_TABLE": pulumi.String(args[1].(string)),
						"REQUESTS_TABLE":      pulumi.String(args[2].(string)),
						"ENVIRONMENT":         pulumi.String(environment),
						"LOG_LEVEL":           pulumi.String(getLogLevel(environment)),
						"METRICS_NAMESPACE":   pulumi.String("Streamer"),
						"METRICS_ENABLED":     pulumi.String("true"),
					}
				},
			).(pulumi.StringMapOutput),
		},

		TracingConfig: &lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},

		ReservedConcurrentExecutions: pulumi.Int(reservedConcurrency),

		Code: pulumi.NewFileArchive("../../lambda/disconnect/deployment.zip"),

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("disconnect"),
		},
	})
	if err != nil {
		return nil, err
	}
	functions["disconnect"] = disconnectFunc

	// Router Lambda
	routerFunc, err := lambda.NewFunction(ctx, "router", &lambda.FunctionArgs{
		Name:        pulumi.Sprintf("streamer-router-%s", environment),
		Description: pulumi.String("WebSocket message router"),
		Runtime:     pulumi.String("go1.x"),
		Handler:     pulumi.String("main"),
		Role:        roles["router"].Arn,
		MemorySize:  pulumi.Int(memorySize * 2), // More memory for router
		Timeout:     pulumi.Int(timeout),

		Environment: &lambda.FunctionEnvironmentArgs{
			Variables: pulumi.All(
				tables["connections"].Name,
				tables["subscriptions"].Name,
				tables["requests"].Name,
			).ApplyT(
				func(args []interface{}) pulumi.StringMap {
					return pulumi.StringMap{
						"CONNECTIONS_TABLE":   pulumi.String(args[0].(string)),
						"SUBSCRIPTIONS_TABLE": pulumi.String(args[1].(string)),
						"REQUESTS_TABLE":      pulumi.String(args[2].(string)),
						"ENVIRONMENT":         pulumi.String(environment),
						"LOG_LEVEL":           pulumi.String(getLogLevel(environment)),
						"METRICS_NAMESPACE":   pulumi.String("Streamer"),
					}
				},
			).(pulumi.StringMapOutput),
		},

		TracingConfig: &lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},

		ReservedConcurrentExecutions: pulumi.Int(reservedConcurrency * 2), // More concurrency for router

		Code: pulumi.NewFileArchive("../../lambda/router/deployment.zip"),

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("router"),
		},
	})
	if err != nil {
		return nil, err
	}
	functions["router"] = routerFunc

	// Processor Lambda (for async processing)
	processorFunc, err := lambda.NewFunction(ctx, "processor", &lambda.FunctionArgs{
		Name:        pulumi.Sprintf("streamer-processor-%s", environment),
		Description: pulumi.String("Async message processor"),
		Runtime:     pulumi.String("go1.x"),
		Handler:     pulumi.String("main"),
		Role:        roles["processor"].Arn,
		MemorySize:  pulumi.Int(memorySize),
		Timeout:     pulumi.Int(300), // 5 minutes for async processing

		Environment: &lambda.FunctionEnvironmentArgs{
			Variables: pulumi.All(
				tables["connections"].Name,
				tables["subscriptions"].Name,
				tables["requests"].Name,
			).ApplyT(
				func(args []interface{}) pulumi.StringMap {
					return pulumi.StringMap{
						"CONNECTIONS_TABLE":   pulumi.String(args[0].(string)),
						"SUBSCRIPTIONS_TABLE": pulumi.String(args[1].(string)),
						"REQUESTS_TABLE":      pulumi.String(args[2].(string)),
						"ENVIRONMENT":         pulumi.String(environment),
						"LOG_LEVEL":           pulumi.String(getLogLevel(environment)),
						"METRICS_NAMESPACE":   pulumi.String("Streamer"),
					}
				},
			).(pulumi.StringMapOutput),
		},

		TracingConfig: &lambda.FunctionTracingConfigArgs{
			Mode: pulumi.String("Active"),
		},

		Code: pulumi.NewFileArchive("../../lambda/processor/deployment.zip"),

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("processor"),
		},
	})
	if err != nil {
		return nil, err
	}
	functions["processor"] = processorFunc

	return functions, nil
}

func getLogLevel(environment string) string {
	switch environment {
	case "dev":
		return "DEBUG"
	case "staging":
		return "INFO"
	case "production":
		return "WARN"
	default:
		return "INFO"
	}
}

func createLogGroupSubscription(ctx *pulumi.Context, name string, function *lambda.Function, logGroup *cloudwatch.LogGroup) error {
	// Allow Lambda to write to CloudWatch Logs
	_, err := lambda.NewPermission(ctx, fmt.Sprintf("%s-logs-permission", name), &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		Function:  function.Name,
		Principal: pulumi.String("logs.amazonaws.com"),
		SourceArn: logGroup.Arn,
	})
	return err
}
