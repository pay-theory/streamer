package main

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func createIAMRoles(ctx *pulumi.Context, environment string, tables map[string]*dynamodb.Table, logGroups map[string]*cloudwatch.LogGroup, jwtSecretArn pulumi.StringOutput, region string) (map[string]*iam.Role, error) {
	roles := make(map[string]*iam.Role)

	// Lambda execution trust policy
	lambdaTrustPolicy := `{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {
				"Service": "lambda.amazonaws.com"
			},
			"Action": "sts:AssumeRole"
		}]
	}`

	// Create base Lambda execution role
	baseLambdaRole, err := iam.NewRole(ctx, "lambda-base-role", &iam.RoleArgs{
		Name:             pulumi.Sprintf("streamer-%s-lambda-base", environment),
		AssumeRolePolicy: pulumi.String(lambdaTrustPolicy),
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Attach basic Lambda execution policy
	_, err = iam.NewRolePolicyAttachment(ctx, "lambda-basic-execution", &iam.RolePolicyAttachmentArgs{
		Role:      baseLambdaRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
	})
	if err != nil {
		return nil, err
	}

	// Create X-Ray tracing policy attachment
	_, err = iam.NewRolePolicyAttachment(ctx, "lambda-xray", &iam.RolePolicyAttachmentArgs{
		Role:      baseLambdaRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AWSXRayDaemonWriteAccess"),
	})
	if err != nil {
		return nil, err
	}

	// Create CloudWatch metrics policy
	metricsPolicy := iam.RolePolicyArgs{
		Role: baseLambdaRole.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"cloudwatch:PutMetricData"
				],
				"Resource": "*"
			}]
		}`),
	}
	_, err = iam.NewRolePolicy(ctx, "lambda-metrics", &metricsPolicy)
	if err != nil {
		return nil, err
	}

	// Connect Lambda specific role and policies
	connectRole, err := createConnectLambdaRole(ctx, environment, baseLambdaRole, tables["connections"], jwtSecretArn)
	if err != nil {
		return nil, err
	}
	roles["connect"] = connectRole

	// Disconnect Lambda specific role and policies
	disconnectRole, err := createDisconnectLambdaRole(ctx, environment, baseLambdaRole, tables)
	if err != nil {
		return nil, err
	}
	roles["disconnect"] = disconnectRole

	// Router Lambda specific role and policies
	routerRole, err := createRouterLambdaRole(ctx, environment, baseLambdaRole, tables, region)
	if err != nil {
		return nil, err
	}
	roles["router"] = routerRole

	// Processor Lambda specific role and policies
	processorRole, err := createProcessorLambdaRole(ctx, environment, baseLambdaRole, tables)
	if err != nil {
		return nil, err
	}
	roles["processor"] = processorRole

	return roles, nil
}

func createConnectLambdaRole(ctx *pulumi.Context, environment string, baseRole *iam.Role, connectionsTable *dynamodb.Table, jwtSecretArn pulumi.StringOutput) (*iam.Role, error) {
	role, err := iam.NewRole(ctx, "connect-lambda-role", &iam.RoleArgs{
		Name:             pulumi.Sprintf("streamer-%s-connect", environment),
		AssumeRolePolicy: baseRole.AssumeRolePolicy,
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("connect"),
		},
	})
	if err != nil {
		return nil, err
	}

	// DynamoDB policy for connections table
	dynamoPolicy := pulumi.All(connectionsTable.Arn).ApplyT(func(args []interface{}) (string, error) {
		tableArn := args[0].(string)
		policy := map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []interface{}{
				map[string]interface{}{
					"Effect": "Allow",
					"Action": []string{
						"dynamodb:PutItem",
						"dynamodb:GetItem",
					},
					"Resource": tableArn,
				},
			},
		}
		policyJSON, err := json.Marshal(policy)
		return string(policyJSON), err
	}).(pulumi.StringOutput)

	_, err = iam.NewRolePolicy(ctx, "connect-dynamo-policy", &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: dynamoPolicy,
	})
	if err != nil {
		return nil, err
	}

	// Secrets Manager policy for JWT public key
	secretsPolicy := pulumi.All(jwtSecretArn).ApplyT(func(args []interface{}) (string, error) {
		secretArn := args[0].(string)
		policy := map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []interface{}{
				map[string]interface{}{
					"Effect": "Allow",
					"Action": []string{
						"secretsmanager:GetSecretValue",
					},
					"Resource": secretArn,
				},
			},
		}
		policyJSON, err := json.Marshal(policy)
		return string(policyJSON), err
	}).(pulumi.StringOutput)

	_, err = iam.NewRolePolicy(ctx, "connect-secrets-policy", &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: secretsPolicy,
	})
	if err != nil {
		return nil, err
	}

	// Attach basic execution and X-Ray policies
	attachBasicPolicies(ctx, "connect", role)

	return role, nil
}

func createDisconnectLambdaRole(ctx *pulumi.Context, environment string, baseRole *iam.Role, tables map[string]*dynamodb.Table) (*iam.Role, error) {
	role, err := iam.NewRole(ctx, "disconnect-lambda-role", &iam.RoleArgs{
		Name:             pulumi.Sprintf("streamer-%s-disconnect", environment),
		AssumeRolePolicy: baseRole.AssumeRolePolicy,
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("disconnect"),
		},
	})
	if err != nil {
		return nil, err
	}

	// DynamoDB policy for all tables
	tableArns := []pulumi.StringOutput{
		tables["connections"].Arn,
		tables["subscriptions"].Arn,
		tables["requests"].Arn,
	}

	dynamoPolicy := pulumi.All(tableArns...).ApplyT(func(args []interface{}) (string, error) {
		resources := make([]string, len(args))
		for i, arn := range args {
			resources[i] = arn.(string)
		}

		policy := map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []interface{}{
				map[string]interface{}{
					"Effect": "Allow",
					"Action": []string{
						"dynamodb:DeleteItem",
						"dynamodb:GetItem",
						"dynamodb:Query",
					},
					"Resource": resources,
				},
			},
		}
		policyJSON, err := json.Marshal(policy)
		return string(policyJSON), err
	}).(pulumi.StringOutput)

	_, err = iam.NewRolePolicy(ctx, "disconnect-dynamo-policy", &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: dynamoPolicy,
	})
	if err != nil {
		return nil, err
	}

	attachBasicPolicies(ctx, "disconnect", role)
	return role, nil
}

func createRouterLambdaRole(ctx *pulumi.Context, environment string, baseRole *iam.Role, tables map[string]*dynamodb.Table, region string) (*iam.Role, error) {
	role, err := iam.NewRole(ctx, "router-lambda-role", &iam.RoleArgs{
		Name:             pulumi.Sprintf("streamer-%s-router", environment),
		AssumeRolePolicy: baseRole.AssumeRolePolicy,
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("router"),
		},
	})
	if err != nil {
		return nil, err
	}

	// DynamoDB policy
	tableArns := []pulumi.StringOutput{
		tables["connections"].Arn,
		tables["subscriptions"].Arn,
		tables["requests"].Arn,
	}

	dynamoPolicy := pulumi.All(tableArns...).ApplyT(func(args []interface{}) (string, error) {
		resources := make([]string, len(args))
		for i, arn := range args {
			resources[i] = arn.(string)
			// Add index ARNs
			resources = append(resources, fmt.Sprintf("%s/index/*", arn.(string)))
		}

		policy := map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []interface{}{
				map[string]interface{}{
					"Effect": "Allow",
					"Action": []string{
						"dynamodb:GetItem",
						"dynamodb:Query",
						"dynamodb:PutItem",
						"dynamodb:UpdateItem",
						"dynamodb:DeleteItem",
						"dynamodb:BatchGetItem",
						"dynamodb:BatchWriteItem",
					},
					"Resource": resources,
				},
			},
		}
		policyJSON, err := json.Marshal(policy)
		return string(policyJSON), err
	}).(pulumi.StringOutput)

	_, err = iam.NewRolePolicy(ctx, "router-dynamo-policy", &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: dynamoPolicy,
	})
	if err != nil {
		return nil, err
	}

	// API Gateway Management API policy
	apiGatewayPolicy := pulumi.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Action": [
				"execute-api:ManageConnections"
			],
			"Resource": "arn:aws:execute-api:%s:*:*/@connections/*"
		}]
	}`, region)

	_, err = iam.NewRolePolicy(ctx, "router-api-gateway-policy", &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: apiGatewayPolicy,
	})
	if err != nil {
		return nil, err
	}

	// SQS policy for async processing
	sqsPolicy := pulumi.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Action": [
				"sqs:SendMessage",
				"sqs:GetQueueUrl"
			],
			"Resource": "arn:aws:sqs:%s:*:streamer-%s-*"
		}]
	}`, region, environment)

	_, err = iam.NewRolePolicy(ctx, "router-sqs-policy", &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: sqsPolicy,
	})
	if err != nil {
		return nil, err
	}

	attachBasicPolicies(ctx, "router", role)
	return role, nil
}

func createProcessorLambdaRole(ctx *pulumi.Context, environment string, baseRole *iam.Role, tables map[string]*dynamodb.Table) (*iam.Role, error) {
	role, err := iam.NewRole(ctx, "processor-lambda-role", &iam.RoleArgs{
		Name:             pulumi.Sprintf("streamer-%s-processor", environment),
		AssumeRolePolicy: baseRole.AssumeRolePolicy,
		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
			"Function":    pulumi.String("processor"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Similar to router but includes SQS receive permissions
	// Implementation details omitted for brevity
	attachBasicPolicies(ctx, "processor", role)
	return role, nil
}

func attachBasicPolicies(ctx *pulumi.Context, functionName string, role *iam.Role) error {
	// Basic Lambda execution
	_, err := iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-basic-execution", functionName), &iam.RolePolicyAttachmentArgs{
		Role:      role.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
	})
	if err != nil {
		return err
	}

	// X-Ray tracing
	_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-xray", functionName), &iam.RolePolicyAttachmentArgs{
		Role:      role.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AWSXRayDaemonWriteAccess"),
	})
	if err != nil {
		return err
	}

	// CloudWatch metrics
	metricsPolicy := pulumi.String(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Action": [
				"cloudwatch:PutMetricData"
			],
			"Resource": "*"
		}]
	}`)

	_, err = iam.NewRolePolicy(ctx, fmt.Sprintf("%s-metrics", functionName), &iam.RolePolicyArgs{
		Role:   role.Name,
		Policy: metricsPolicy,
	})

	return err
}
