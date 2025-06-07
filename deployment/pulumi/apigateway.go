package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/apigatewayv2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func createAPIGateway(ctx *pulumi.Context, environment string, lambdaFunctions map[string]*lambda.Function) (*apigatewayv2.Api, error) {
	// Create WebSocket API
	api, err := apigatewayv2.NewApi(ctx, "websocket-api", &apigatewayv2.ApiArgs{
		Name:                     pulumi.Sprintf("streamer-%s", environment),
		ProtocolType:             pulumi.String("WEBSOCKET"),
		RouteSelectionExpression: pulumi.String("$request.body.action"),
		Description:              pulumi.Sprintf("Streamer WebSocket API for %s", environment),

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Create Lambda integrations
	connectIntegration, err := apigatewayv2.NewIntegration(ctx, "connect-integration", &apigatewayv2.IntegrationArgs{
		ApiId:                api.ID(),
		IntegrationType:      pulumi.String("AWS_PROXY"),
		IntegrationUri:       lambdaFunctions["connect"].InvokeArn,
		IntegrationMethod:    pulumi.String("POST"),
		PayloadFormatVersion: pulumi.String("1.0"),
	})
	if err != nil {
		return nil, err
	}

	disconnectIntegration, err := apigatewayv2.NewIntegration(ctx, "disconnect-integration", &apigatewayv2.IntegrationArgs{
		ApiId:                api.ID(),
		IntegrationType:      pulumi.String("AWS_PROXY"),
		IntegrationUri:       lambdaFunctions["disconnect"].InvokeArn,
		IntegrationMethod:    pulumi.String("POST"),
		PayloadFormatVersion: pulumi.String("1.0"),
	})
	if err != nil {
		return nil, err
	}

	defaultIntegration, err := apigatewayv2.NewIntegration(ctx, "default-integration", &apigatewayv2.IntegrationArgs{
		ApiId:                api.ID(),
		IntegrationType:      pulumi.String("AWS_PROXY"),
		IntegrationUri:       lambdaFunctions["router"].InvokeArn,
		IntegrationMethod:    pulumi.String("POST"),
		PayloadFormatVersion: pulumi.String("1.0"),
	})
	if err != nil {
		return nil, err
	}

	// Create routes
	_, err = apigatewayv2.NewRoute(ctx, "connect-route", &apigatewayv2.RouteArgs{
		ApiId:             api.ID(),
		RouteKey:          pulumi.String("$connect"),
		AuthorizationType: pulumi.String("NONE"),
		Target:            pulumi.Sprintf("integrations/%s", connectIntegration.ID()),
	})
	if err != nil {
		return nil, err
	}

	_, err = apigatewayv2.NewRoute(ctx, "disconnect-route", &apigatewayv2.RouteArgs{
		ApiId:             api.ID(),
		RouteKey:          pulumi.String("$disconnect"),
		AuthorizationType: pulumi.String("NONE"),
		Target:            pulumi.Sprintf("integrations/%s", disconnectIntegration.ID()),
	})
	if err != nil {
		return nil, err
	}

	_, err = apigatewayv2.NewRoute(ctx, "default-route", &apigatewayv2.RouteArgs{
		ApiId:             api.ID(),
		RouteKey:          pulumi.String("$default"),
		AuthorizationType: pulumi.String("NONE"),
		Target:            pulumi.Sprintf("integrations/%s", defaultIntegration.ID()),
	})
	if err != nil {
		return nil, err
	}

	// Create deployment
	deployment, err := apigatewayv2.NewDeployment(ctx, "api-deployment", &apigatewayv2.DeploymentArgs{
		ApiId:       api.ID(),
		Description: pulumi.Sprintf("Deployment for %s environment", environment),
	}, pulumi.DependsOn([]pulumi.Resource{
		api,
		connectIntegration,
		disconnectIntegration,
		defaultIntegration,
	}))
	if err != nil {
		return nil, err
	}

	// Create stage
	stageName := environment
	if environment == "production" {
		stageName = "prod"
	}

	stage, err := apigatewayv2.NewStage(ctx, "api-stage", &apigatewayv2.StageArgs{
		ApiId:        api.ID(),
		DeploymentId: deployment.ID(),
		Name:         pulumi.String(stageName),

		DefaultRouteSettings: &apigatewayv2.StageDefaultRouteSettingsArgs{
			ThrottleRateLimit:      pulumi.Float64(1000),
			ThrottleBurstLimit:     pulumi.Int(2000),
			DataTraceEnabled:       pulumi.Bool(environment != "production"),
			DetailedMetricsEnabled: pulumi.Bool(true),
			LoggingLevel:           pulumi.String(getAPIGatewayLogLevel(environment)),
		},

		Tags: pulumi.StringMap{
			"Environment": pulumi.String(environment),
			"Service":     pulumi.String("streamer"),
		},
	})
	if err != nil {
		return nil, err
	}

	// Grant Lambda permission to be invoked by API Gateway
	err = grantAPIGatewayPermissions(ctx, api, stage, lambdaFunctions)
	if err != nil {
		return nil, err
	}

	return api, nil
}

func grantAPIGatewayPermissions(ctx *pulumi.Context, api *apigatewayv2.Api, stage *apigatewayv2.Stage, functions map[string]*lambda.Function) error {
	// Grant permissions for each Lambda function
	for name, function := range functions {
		sourceArn := pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/*",
			api.ID().ToStringOutput().ApplyT(func(id string) string {
				// Extract region from API ID
				// API ID format: xxxxx.execute-api.region.amazonaws.com
				return "us-east-1" // You might want to make this configurable
			}),
			pulumi.String("*"), // Account ID will be replaced
			api.ID(),
		)

		_, err := lambda.NewPermission(ctx, name+"-api-permission", &lambda.PermissionArgs{
			Action:    pulumi.String("lambda:InvokeFunction"),
			Function:  function.Name,
			Principal: pulumi.String("apigateway.amazonaws.com"),
			SourceArn: sourceArn,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func getAPIGatewayLogLevel(environment string) string {
	switch environment {
	case "dev":
		return "INFO"
	case "staging":
		return "ERROR"
	case "production":
		return "ERROR"
	default:
		return "ERROR"
	}
}
