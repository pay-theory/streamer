package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/pay-theory/dynamorm/pkg/session"

	"github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/lambda/processor/executor"
	"github.com/pay-theory/streamer/lambda/processor/handlers"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
)

var (
	exec   *executor.AsyncExecutor
	logger *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "[PROCESSOR] ", log.LstdFlags|log.Lshortfile)

	// Initialize AWS config
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatalf("Failed to load AWS config: %v", err)
	}

	// Initialize DynamORM factory
	dynamormConfig := session.Config{
		Region: cfg.Region,
	}

	storeFactory, err := dynamorm.NewStoreFactory(dynamormConfig)
	if err != nil {
		logger.Fatalf("Failed to create DynamORM store factory: %v", err)
	}

	// Get storage components from factory
	connectionStore := storeFactory.ConnectionStore()

	// Initialize API Gateway Management API client
	apiGatewayEndpoint := os.Getenv("WEBSOCKET_ENDPOINT")
	if apiGatewayEndpoint == "" {
		logger.Fatal("WEBSOCKET_ENDPOINT environment variable is required")
	}

	apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &apiGatewayEndpoint
	})

	// Wrap the AWS SDK client with the adapter
	apiGatewayAdapter := connection.NewAWSAPIGatewayAdapter(apiGatewayClient)

	// Create real ConnectionManager from Team 1
	connManager := connection.NewManager(connectionStore, apiGatewayAdapter, apiGatewayEndpoint)
	connManager.SetLogger(logger.Printf)

	// Create executor using DynamORM DB directly
	exec = executor.New(connManager, storeFactory.DB(), logger)

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
		if asyncReq.Status != dynamorm.StatusPending {
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

// parseAsyncRequest converts a DynamoDB stream record to an AsyncRequest using DynamORM's SafeMarshaler
func parseAsyncRequest(record events.DynamoDBEventRecord) (*dynamorm.AsyncRequest, error) {
	// For INSERT events, use NewImage; for MODIFY events, use NewImage as well
	image := record.Change.NewImage
	if image == nil {
		return nil, nil
	}

	// Convert events.DynamoDBAttributeValue to types.AttributeValue
	attribs := make(map[string]types.AttributeValue)
	for k, v := range image {
		attribs[k] = convertDynamoDBAttributeValue(v)
	}

	// Use AWS SDK attributevalue marshaler as fallback until DynamORM API is clarified
	var asyncReq dynamorm.AsyncRequest
	if err := attributevalue.UnmarshalMap(attribs, &asyncReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AsyncRequest: %w", err)
	}

	return &asyncReq, nil
}

// convertDynamoDBAttributeValue converts events.DynamoDBAttributeValue to types.AttributeValue
func convertDynamoDBAttributeValue(eventVal events.DynamoDBAttributeValue) types.AttributeValue {
	if eventVal.IsNull() {
		return &types.AttributeValueMemberNULL{Value: true}
	}
	if s := eventVal.String(); s != "" {
		return &types.AttributeValueMemberS{Value: s}
	}
	if n := eventVal.Number(); n != "" {
		return &types.AttributeValueMemberN{Value: n}
	}
	if b := eventVal.Binary(); b != nil {
		return &types.AttributeValueMemberB{Value: b}
	}
	if ss := eventVal.StringSet(); ss != nil {
		return &types.AttributeValueMemberSS{Value: ss}
	}
	if ns := eventVal.NumberSet(); ns != nil {
		return &types.AttributeValueMemberNS{Value: ns}
	}
	if bs := eventVal.BinarySet(); bs != nil {
		return &types.AttributeValueMemberBS{Value: bs}
	}
	if m := eventVal.Map(); m != nil {
		converted := make(map[string]types.AttributeValue)
		for k, v := range m {
			converted[k] = convertDynamoDBAttributeValue(v)
		}
		return &types.AttributeValueMemberM{Value: converted}
	}
	if l := eventVal.List(); l != nil {
		converted := make([]types.AttributeValue, len(l))
		for i, v := range l {
			converted[i] = convertDynamoDBAttributeValue(v)
		}
		return &types.AttributeValueMemberL{Value: converted}
	}
	if eventVal.DataType() == events.DataTypeBoolean {
		b := eventVal.Boolean()
		return &types.AttributeValueMemberBOOL{Value: b}
	}
	return &types.AttributeValueMemberNULL{Value: true}
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
