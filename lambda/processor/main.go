package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/lambda/processor/executor"
	"github.com/pay-theory/streamer/lambda/processor/handlers"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
)

var (
	exec             *executor.AsyncExecutor
	logger           *log.Logger
	connectionsTable string
	requestsTable    string
)

func init() {
	logger = log.New(os.Stdout, "[PROCESSOR] ", log.LstdFlags|log.Lshortfile)

	// Get table names from environment
	connectionsTable = os.Getenv("CONNECTIONS_TABLE")
	if connectionsTable == "" {
		connectionsTable = "streamer_connections"
	}

	requestsTable = os.Getenv("REQUESTS_TABLE")
	if requestsTable == "" {
		requestsTable = "streamer_requests"
	}

	// Initialize AWS config
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatalf("Failed to load AWS config: %v", err)
	}

	// Initialize DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Initialize storage components
	requestQueue := store.NewRequestQueue(dynamoClient, requestsTable)
	connectionStore := store.NewConnectionStore(dynamoClient, connectionsTable)

	// Initialize API Gateway Management API client
	apiGatewayEndpoint := os.Getenv("WEBSOCKET_ENDPOINT")
	if apiGatewayEndpoint == "" {
		logger.Fatal("WEBSOCKET_ENDPOINT environment variable is required")
	}

	apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &apiGatewayEndpoint
	})

	// Create real ConnectionManager from Team 1
	connManager := connection.NewManager(connectionStore, apiGatewayClient, apiGatewayEndpoint)
	connManager.SetLogger(logger.Printf)

	// Create executor
	exec = executor.New(connManager, requestQueue, logger)

	// Register async handlers
	if err := registerAsyncHandlers(exec); err != nil {
		logger.Fatalf("Failed to register handlers: %v", err)
	}

	logger.Println("Processor Lambda initialized successfully")
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	logger.Printf("Processing %d stream records", len(event.Records))

	for _, record := range event.Records {
		// Only process INSERT and MODIFY events for async requests
		if record.EventName != "INSERT" && record.EventName != "MODIFY" {
			continue
		}

		// Parse the AsyncRequest from DynamoDB stream
		asyncReq, err := parseAsyncRequest(record)
		if err != nil {
			logger.Printf("Failed to parse AsyncRequest: %v", err)
			continue
		}

		// Skip if not in PENDING status
		if asyncReq.Status != store.StatusPending {
			logger.Printf("Skipping request %s with status %s", asyncReq.RequestID, asyncReq.Status)
			continue
		}

		// Create context with timeout (Lambda max is 15 minutes, leave 1 minute buffer)
		processCtx, cancel := context.WithTimeout(ctx, 14*time.Minute)

		// Process the request with retry logic
		err = exec.ProcessWithRetry(processCtx, asyncReq)

		cancel()

		if err != nil {
			logger.Printf("Failed to process request %s: %v", asyncReq.RequestID, err)
			// The error is logged but we don't return it to avoid reprocessing
			// The request will be marked as failed in the ProcessWithRetry function
		}
	}

	return nil
}

// parseAsyncRequest converts a DynamoDB stream record to an AsyncRequest
func parseAsyncRequest(record events.DynamoDBEventRecord) (*store.AsyncRequest, error) {
	// For INSERT events, use NewImage; for MODIFY events, use NewImage as well
	image := record.Change.NewImage
	if image == nil {
		return nil, nil
	}

	// Convert DynamoDB event attribute values to a regular map
	// This is necessary because Lambda events use a different type than SDK v2
	imageMap := make(map[string]interface{})
	for k, v := range image {
		var val interface{}
		jsonBytes, err := v.MarshalJSON()
		if err != nil {
			logger.Printf("Failed to marshal attribute %s: %v", k, err)
			continue
		}
		if err := json.Unmarshal(jsonBytes, &val); err != nil {
			logger.Printf("Failed to unmarshal attribute %s: %v", k, err)
			continue
		}
		imageMap[k] = val
	}

	// Now convert to AsyncRequest struct
	jsonBytes, err := json.Marshal(imageMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal image map: %w", err)
	}

	var asyncReq store.AsyncRequest
	if err := json.Unmarshal(jsonBytes, &asyncReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AsyncRequest: %w", err)
	}

	return &asyncReq, nil
}

func registerAsyncHandlers(exec *executor.AsyncExecutor) error {
	// Register handlers from the router package
	exec.RegisterHandler("delay", streamer.NewDelayHandler(30*time.Second))

	// Register production async handlers from handlers package
	exec.RegisterHandler("generate_report", handlers.NewReportAsyncHandler())
	exec.RegisterHandler("process_data", handlers.NewDataProcessorHandler())
	exec.RegisterHandler("bulk_operation", NewBulkHandlerWithProgress()) // Keep existing for now
	exec.RegisterHandler("echo_async", handlers.NewEchoAsyncHandler())   // Simple test handler

	logger.Printf("Registered %d async handlers", 5)
	return nil
}

func main() {
	lambda.Start(handler)
}
