// Example: Streamer Processor with DynamORM
// This demonstrates proper DynamORM patterns for async request processing
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
	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/session"

	storedynamorm "github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/lambda/processor/executor"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
)

var (
	exec   *executor.AsyncExecutor
	logger *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "[EXAMPLE] ", log.LstdFlags)

	// Initialize AWS config
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create DynamORM factory with proper configuration
	dynamormConfig := session.Config{
		Region: cfg.Region,
	}

	storeFactory, err := storedynamorm.NewStoreFactory(dynamormConfig)
	if err != nil {
		logger.Fatalf("Failed to create DynamORM store factory: %v", err)
	}

	// Create connection manager for WebSocket notifications
	apiGatewayEndpoint := os.Getenv("WEBSOCKET_ENDPOINT")
	if apiGatewayEndpoint == "" {
		logger.Fatal("WEBSOCKET_ENDPOINT environment variable is required")
	}

	apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &apiGatewayEndpoint
	})

	apiGatewayAdapter := connection.NewAWSAPIGatewayAdapter(apiGatewayClient)
	connManager := connection.NewManager(storeFactory.ConnectionStore(), apiGatewayAdapter, apiGatewayEndpoint)
	connManager.SetLogger(logger.Printf)

	// Create AsyncExecutor with DynamORM database instance
	exec = executor.New(connManager, storeFactory.DB(), logger)

	// Register example handlers
	registerHandlers(exec)

	logger.Println("Example processor initialized with DynamORM")
}

// registerHandlers demonstrates registering different types of handlers
func registerHandlers(exec *executor.AsyncExecutor) {
	// Simple async handler
	exec.RegisterHandler("example_simple", NewSimpleHandler())
	
	// Handler with progress reporting
	exec.RegisterHandler("example_progress", NewProgressHandler())
	
	// Knowledge base handler (example from earlier discussion)
	exec.RegisterHandler("knowledge_query", NewKnowledgeBaseHandler())

	logger.Printf("Registered example handlers")
}

// DynamoDB stream handler using proper DynamORM parsing
func handler(ctx context.Context, event events.DynamoDBEvent) error {
	logger.Printf("Processing %d stream records", len(event.Records))

	for _, record := range event.Records {
		// Only process INSERT and MODIFY events
		if record.EventName != "INSERT" && record.EventName != "MODIFY" {
			continue
		}

		// Parse using DynamORM's SafeMarshaler (not broken JSON approach)
		asyncReq, err := parseAsyncRequest(record)
		if err != nil {
			logger.Printf("Failed to parse AsyncRequest: %v", err)
			continue
		}

		if asyncReq == nil {
			continue
		}

		// Only process pending requests
		if asyncReq.Status != storedynamorm.StatusPending {
			logger.Printf("Skipping request %s with status %s", asyncReq.RequestID, asyncReq.Status)
			continue
		}

		// Process with retry logic
		processCtx, cancel := context.WithTimeout(ctx, 14*time.Minute)
		err = exec.ProcessWithRetry(processCtx, asyncReq)
		cancel()

		if err != nil {
			logger.Printf("Failed to process request %s: %v", asyncReq.RequestID, err)
		}
	}

	return nil
}

// parseAsyncRequest demonstrates proper DynamORM stream parsing
func parseAsyncRequest(record events.DynamoDBEventRecord) (*storedynamorm.AsyncRequest, error) {
	image := record.Change.NewImage
	if image == nil {
		return nil, nil
	}

	// Use DynamORM's native stream support
	var asyncReq storedynamorm.AsyncRequest
	if err := dynamorm.UnmarshalStreamImage(image, &asyncReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AsyncRequest: %w", err)
	}

	return &asyncReq, nil
}

// Example Handlers

// SimpleHandler demonstrates basic async processing
type SimpleHandler struct{}

func NewSimpleHandler() *SimpleHandler {
	return &SimpleHandler{}
}

func (h *SimpleHandler) EstimatedDuration() time.Duration {
	return 10 * time.Second // Will trigger async processing
}

func (h *SimpleHandler) Validate(req *streamer.Request) error {
	if req.Payload == nil {
		return fmt.Errorf("payload is required")
	}
	return nil
}

func (h *SimpleHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	var input map[string]interface{}
	if err := json.Unmarshal(req.Payload, &input); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}

	// Simulate work
	time.Sleep(5 * time.Second)

	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"processed": true,
			"input":     input,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// ProgressHandler demonstrates handler with progress reporting
type ProgressHandler struct{}

func NewProgressHandler() *ProgressHandler {
	return &ProgressHandler{}
}

func (h *ProgressHandler) EstimatedDuration() time.Duration {
	return 30 * time.Second
}

func (h *ProgressHandler) Validate(req *streamer.Request) error {
	return nil // Accept any payload for this example
}

func (h *ProgressHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return nil, fmt.Errorf("use ProcessWithProgress for this handler")
}

func (h *ProgressHandler) ProcessWithProgress(
	ctx context.Context,
	req *streamer.Request,
	reporter streamer.ProgressReporter,
) (*streamer.Result, error) {
	
	// Multi-step processing with progress updates
	steps := []struct {
		percentage float64
		message    string
		duration   time.Duration
	}{
		{0, "Initializing...", 2 * time.Second},
		{25, "Loading data...", 3 * time.Second},
		{50, "Processing...", 5 * time.Second},
		{75, "Finalizing...", 2 * time.Second},
		{100, "Complete!", 0},
	}

	for _, step := range steps {
		// Report progress
		if err := reporter.Report(step.percentage, step.message); err != nil {
			logger.Printf("Failed to report progress: %v", err)
		}

		// Simulate work
		if step.duration > 0 {
			select {
			case <-time.After(step.duration):
				// Continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"result":    "Processing completed successfully",
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// KnowledgeBaseHandler demonstrates a real-world async handler
type KnowledgeBaseHandler struct{}

func NewKnowledgeBaseHandler() *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{}
}

func (h *KnowledgeBaseHandler) EstimatedDuration() time.Duration {
	return 15 * time.Second
}

func (h *KnowledgeBaseHandler) Validate(req *streamer.Request) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload format: %w", err)
	}

	if _, ok := payload["query"]; !ok {
		return fmt.Errorf("query field is required")
	}

	if _, ok := payload["knowledge_base_id"]; !ok {
		return fmt.Errorf("knowledge_base_id field is required")
	}

	return nil
}

func (h *KnowledgeBaseHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return nil, fmt.Errorf("use ProcessWithProgress for knowledge base queries")
}

func (h *KnowledgeBaseHandler) ProcessWithProgress(
	ctx context.Context,
	req *streamer.Request,
	reporter streamer.ProgressReporter,
) (*streamer.Result, error) {
	
	var payload map[string]interface{}
	json.Unmarshal(req.Payload, &payload)
	
	query := payload["query"].(string)
	knowledgeBaseID := payload["knowledge_base_id"].(string)

	// Set metadata for tracking
	reporter.SetMetadata("knowledge_base_id", knowledgeBaseID)
	reporter.SetMetadata("query_length", len(query))

	// Step 1: Initialize
	reporter.Report(10, "Initializing knowledge base connection...")
	time.Sleep(1 * time.Second)

	// Step 2: Query
	reporter.Report(30, "Executing knowledge base query...")
	time.Sleep(3 * time.Second)

	// Step 3: Process results
	reporter.Report(70, "Processing and ranking results...")
	time.Sleep(2 * time.Second)

	// Step 4: Complete
	reporter.Report(100, "Knowledge base query complete!")

	// Simulate results
	results := map[string]interface{}{
		"query":             query,
		"knowledge_base_id": knowledgeBaseID,
		"results": []map[string]interface{}{
			{
				"content":    "Example knowledge base result 1",
				"confidence": 0.95,
				"source":     "doc1.pdf",
			},
			{
				"content":    "Example knowledge base result 2", 
				"confidence": 0.87,
				"source":     "doc2.pdf",
			},
		},
		"result_count":     2,
		"confidence_score": 0.91,
		"processed_at":     time.Now().Format(time.RFC3339),
	}

	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data:      results,
		Metadata: map[string]string{
			"knowledge_base_id": knowledgeBaseID,
			"query_type":        "retrieve",
		},
	}, nil
}

func main() {
	lambda.Start(handler)
}