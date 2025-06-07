package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/kms"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/secretsmanager"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get configuration
		cfg := config.New(ctx, "")
		environment := cfg.Require("environment")

		// Get AWS configuration
		awsCfg := config.New(ctx, "aws")
		region := awsCfg.Require("region")

		// Create KMS key for encryption
		kmsKey, err := kms.NewKey(ctx, "streamer-key", &kms.KeyArgs{
			Description:          pulumi.Sprintf("KMS key for Streamer %s", environment),
			DeletionWindowInDays: pulumi.Int(10),
			EnableKeyRotation:    pulumi.Bool(true),
			Tags: pulumi.StringMap{
				"Name":        pulumi.Sprintf("streamer-%s", environment),
				"Environment": pulumi.String(environment),
			},
		})
		if err != nil {
			return err
		}

		// Create KMS alias
		_, err = kms.NewAlias(ctx, "streamer-alias", &kms.AliasArgs{
			Name:        pulumi.Sprintf("alias/streamer-%s", environment),
			TargetKeyId: kmsKey.ID(),
		})
		if err != nil {
			return err
		}

		// Create secrets for JWT keys
		jwtPublicKeySecret, err := secretsmanager.NewSecret(ctx, "jwt-public-key", &secretsmanager.SecretArgs{
			Name:     pulumi.Sprintf("streamer/%s/jwt-public-key", environment),
			KmsKeyId: kmsKey.ID(),
		})
		if err != nil {
			return err
		}

		jwtPrivateKeySecret, err := secretsmanager.NewSecret(ctx, "jwt-private-key", &secretsmanager.SecretArgs{
			Name:     pulumi.Sprintf("streamer/%s/jwt-private-key", environment),
			KmsKeyId: kmsKey.ID(),
		})
		if err != nil {
			return err
		}

		// Create DynamoDB tables
		tables, err := createDynamoDBTables(ctx, environment, kmsKey.Arn)
		if err != nil {
			return err
		}

		// Create CloudWatch Log Groups
		logGroups, err := createLogGroups(ctx, environment, kmsKey.Arn)
		if err != nil {
			return err
		}

		// Create IAM roles and policies
		roles, err := createIAMRoles(ctx, environment, tables, logGroups, jwtPublicKeySecret.Arn, region)
		if err != nil {
			return err
		}

		// Create Lambda functions
		lambdaFunctions, err := createLambdaFunctions(ctx, environment, roles, tables, logGroups, jwtPublicKeySecret.Arn)
		if err != nil {
			return err
		}

		// Create API Gateway
		api, err := createAPIGateway(ctx, environment, lambdaFunctions)
		if err != nil {
			return err
		}

		// Create CloudWatch Alarms
		err = createCloudWatchAlarms(ctx, environment, lambdaFunctions)
		if err != nil {
			return err
		}

		// Export outputs
		ctx.Export("apiEndpoint", api.ApiEndpoint)
		ctx.Export("connectFunctionArn", lambdaFunctions["connect"].Arn)
		ctx.Export("disconnectFunctionArn", lambdaFunctions["disconnect"].Arn)
		ctx.Export("routerFunctionArn", lambdaFunctions["router"].Arn)
		ctx.Export("processorFunctionArn", lambdaFunctions["processor"].Arn)
		ctx.Export("connectionsTableName", tables["connections"].Name)
		ctx.Export("subscriptionsTableName", tables["subscriptions"].Name)
		ctx.Export("requestsTableName", tables["requests"].Name)

		return nil
	})
}

func createDynamoDBTables(ctx *pulumi.Context, environment string, kmsKeyArn pulumi.StringOutput) (map[string]*dynamodb.Table, error) {
	tables := make(map[string]*dynamodb.Table)

	// Connections table
	connectionsTable, err := dynamodb.NewTable(ctx, "connections", &dynamodb.TableArgs{
		Name:        pulumi.Sprintf("streamer-%s-connections", environment),
		BillingMode: pulumi.String("PAY_PER_REQUEST"),
		HashKey:     pulumi.String("connectionId"),

		Attributes: dynamodb.TableAttributeArray{
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("connectionId"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("userId"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("tenantId"),
				Type: pulumi.String("S"),
			},
		},

		GlobalSecondaryIndexes: dynamodb.TableGlobalSecondaryIndexArray{
			&dynamodb.TableGlobalSecondaryIndexArgs{
				Name:           pulumi.String("userId-index"),
				HashKey:        pulumi.String("userId"),
				ProjectionType: pulumi.String("ALL"),
			},
			&dynamodb.TableGlobalSecondaryIndexArgs{
				Name:           pulumi.String("tenantId-index"),
				HashKey:        pulumi.String("tenantId"),
				ProjectionType: pulumi.String("ALL"),
			},
		},

		Ttl: &dynamodb.TableTtlArgs{
			AttributeName: pulumi.String("ttl"),
			Enabled:       pulumi.Bool(true),
		},

		ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{
			Enabled:   pulumi.Bool(true),
			KmsKeyArn: kmsKeyArn,
		},

		PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{
			Enabled: pulumi.Bool(true),
		},

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
		},
	})
	if err != nil {
		return nil, err
	}
	tables["connections"] = connectionsTable

	// Subscriptions table
	subscriptionsTable, err := dynamodb.NewTable(ctx, "subscriptions", &dynamodb.TableArgs{
		Name:        pulumi.Sprintf("streamer-%s-subscriptions", environment),
		BillingMode: pulumi.String("PAY_PER_REQUEST"),
		HashKey:     pulumi.String("subscriptionId"),
		RangeKey:    pulumi.String("connectionId"),

		Attributes: dynamodb.TableAttributeArray{
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("subscriptionId"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("connectionId"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("topic"),
				Type: pulumi.String("S"),
			},
		},

		GlobalSecondaryIndexes: dynamodb.TableGlobalSecondaryIndexArray{
			&dynamodb.TableGlobalSecondaryIndexArgs{
				Name:           pulumi.String("connectionId-index"),
				HashKey:        pulumi.String("connectionId"),
				ProjectionType: pulumi.String("ALL"),
			},
			&dynamodb.TableGlobalSecondaryIndexArgs{
				Name:           pulumi.String("topic-index"),
				HashKey:        pulumi.String("topic"),
				ProjectionType: pulumi.String("ALL"),
			},
		},

		ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{
			Enabled:   pulumi.Bool(true),
			KmsKeyArn: kmsKeyArn,
		},

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
		},
	})
	if err != nil {
		return nil, err
	}
	tables["subscriptions"] = subscriptionsTable

	// Requests table
	requestsTable, err := dynamodb.NewTable(ctx, "requests", &dynamodb.TableArgs{
		Name:        pulumi.Sprintf("streamer-%s-requests", environment),
		BillingMode: pulumi.String("PAY_PER_REQUEST"),
		HashKey:     pulumi.String("requestId"),

		Attributes: dynamodb.TableAttributeArray{
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("requestId"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("connectionId"),
				Type: pulumi.String("S"),
			},
		},

		GlobalSecondaryIndexes: dynamodb.TableGlobalSecondaryIndexArray{
			&dynamodb.TableGlobalSecondaryIndexArgs{
				Name:           pulumi.String("connectionId-index"),
				HashKey:        pulumi.String("connectionId"),
				ProjectionType: pulumi.String("ALL"),
			},
		},

		Ttl: &dynamodb.TableTtlArgs{
			AttributeName: pulumi.String("ttl"),
			Enabled:       pulumi.Bool(true),
		},

		ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{
			Enabled:   pulumi.Bool(true),
			KmsKeyArn: kmsKeyArn,
		},

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
		},
	})
	if err != nil {
		return nil, err
	}
	tables["requests"] = requestsTable

	return tables, nil
}

func createLogGroups(ctx *pulumi.Context, environment string, kmsKeyArn pulumi.StringOutput) (map[string]*cloudwatch.LogGroup, error) {
	logGroups := make(map[string]*cloudwatch.LogGroup)

	logRetention := 30
	if environment == "production" {
		logRetention = 90
	} else if environment == "dev" {
		logRetention = 7
	}

	// API Gateway logs
	apiGatewayLogs, err := cloudwatch.NewLogGroup(ctx, "api-gateway-logs", &cloudwatch.LogGroupArgs{
		Name:            pulumi.Sprintf("/aws/apigateway/streamer-%s", environment),
		RetentionInDays: pulumi.Int(logRetention),
		KmsKeyId:        kmsKeyArn,
	})
	if err != nil {
		return nil, err
	}
	logGroups["api-gateway"] = apiGatewayLogs

	// Lambda function logs
	lambdaNames := []string{"connect", "disconnect", "router", "processor"}
	for _, name := range lambdaNames {
		logGroup, err := cloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-logs", name), &cloudwatch.LogGroupArgs{
			Name:            pulumi.Sprintf("/aws/lambda/streamer-%s-%s", name, environment),
			RetentionInDays: pulumi.Int(logRetention),
			KmsKeyId:        kmsKeyArn,
		})
		if err != nil {
			return nil, err
		}
		logGroups[name] = logGroup
	}

	return logGroups, nil
}
