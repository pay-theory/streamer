package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
)

var (
	router             *streamer.DefaultRouter
	logger             *log.Logger
	connectionsTable   string
	requestsTable      string
	subscriptionsTable string
)

func init() {
	logger = log.New(os.Stdout, "[ROUTER] ", log.LstdFlags|log.Lshortfile)

	// Get table names from environment
	connectionsTable = os.Getenv("CONNECTIONS_TABLE")
	if connectionsTable == "" {
		connectionsTable = "streamer_connections"
	}

	requestsTable = os.Getenv("REQUESTS_TABLE")
	if requestsTable == "" {
		requestsTable = "streamer_requests"
	}

	subscriptionsTable = os.Getenv("SUBSCRIPTIONS_TABLE")
	if subscriptionsTable == "" {
		subscriptionsTable = "streamer_subscriptions"
	}

	// Initialize AWS config and DynamoDB client
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatalf("Failed to load AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Initialize storage
	connStore := store.NewConnectionStore(dynamoClient, connectionsTable)
	reqQueue := store.NewRequestQueue(dynamoClient, requestsTable)

	// Create adapter
	queueAdapter := streamer.NewRequestQueueAdapter(reqQueue)

	// Initialize API Gateway Management API client
	apiGatewayEndpoint := os.Getenv("WEBSOCKET_ENDPOINT")
	if apiGatewayEndpoint == "" {
		logger.Fatal("WEBSOCKET_ENDPOINT environment variable is required")
	}

	apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &apiGatewayEndpoint
	})

	// Create real ConnectionManager from Team 1
	connManager := connection.NewManager(connStore, apiGatewayClient, apiGatewayEndpoint)
	connManager.SetLogger(logger.Printf)

	// Create router
	router = streamer.NewRouter(queueAdapter, connManager)
	router.SetAsyncThreshold(5 * time.Second)

	// Apply middleware
	router.SetMiddleware(
		streamer.LoggingMiddleware(logger.Printf),
		validationMiddleware(),
		metricsMiddleware(),
	)

	// Register handlers
	if err := registerHandlers(router); err != nil {
		logger.Fatalf("Failed to register handlers: %v", err)
	}

	logger.Println("Router Lambda initialized successfully")
}

// handler processes incoming WebSocket messages
func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Printf("Processing request from connection: %s, route: %s",
		event.RequestContext.ConnectionID,
		event.RequestContext.RouteKey)

	// Add request metadata
	if event.RequestContext.Authorizer != nil {
		// Extract user/tenant info from authorizer if available
		if authData, ok := event.RequestContext.Authorizer.(map[string]interface{}); ok {
			if userID, ok := authData["userId"].(string); ok {
				ctx = context.WithValue(ctx, "userId", userID)
			}
			if tenantID, ok := authData["tenantId"].(string); ok {
				ctx = context.WithValue(ctx, "tenantId", tenantID)
			}
		}
	}

	// Route the request
	err := router.Route(ctx, event)
	if err != nil {
		logger.Printf("Error routing request: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Internal Server Error",
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

// validationMiddleware adds request validation
func validationMiddleware() streamer.Middleware {
	return func(next streamer.Handler) streamer.Handler {
		return streamer.NewHandlerFunc(
			func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
				// Add user/tenant IDs from context to request metadata
				if userID, ok := ctx.Value("userId").(string); ok {
					req.Metadata["user_id"] = userID
				}
				if tenantID, ok := ctx.Value("tenantId").(string); ok {
					req.Metadata["tenant_id"] = tenantID
				}

				// Validate request size
				if len(req.Payload) > 1024*1024 { // 1MB limit
					return nil, streamer.NewError(streamer.ErrCodeValidation, "Payload too large (max 1MB)")
				}

				return next.Process(ctx, req)
			},
			next.EstimatedDuration(),
			next.Validate,
		)
	}
}

// metricsMiddleware adds basic metrics logging
func metricsMiddleware() streamer.Middleware {
	return func(next streamer.Handler) streamer.Handler {
		return streamer.NewHandlerFunc(
			func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
				start := time.Now()

				result, err := next.Process(ctx, req)

				duration := time.Since(start)
				status := "success"
				if err != nil {
					status = "error"
				}

				logger.Printf("METRICS: action=%s, duration=%v, status=%s",
					req.Action, duration, status)

				return result, err
			},
			next.EstimatedDuration(),
			next.Validate,
		)
	}
}

func main() {
	lambda.Start(handler)
}
