// Example: Streamer Router with DynamORM
// This demonstrates proper DynamORM patterns for WebSocket routing
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
	"github.com/pay-theory/dynamorm/pkg/session"

	"github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
)

var (
	router streamer.Router
	logger *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "[ROUTER] ", log.LstdFlags)

	// Initialize AWS config
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create DynamORM factory
	dynamormConfig := session.Config{
		Region: cfg.Region,
	}

	storeFactory, err := dynamorm.NewStoreFactory(dynamormConfig)
	if err != nil {
		logger.Fatalf("Failed to create DynamORM store factory: %v", err)
	}

	// Create API Gateway client for WebSocket management
	apiGatewayEndpoint := os.Getenv("WEBSOCKET_ENDPOINT")
	if apiGatewayEndpoint == "" {
		logger.Fatal("WEBSOCKET_ENDPOINT environment variable is required")
	}

	apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &apiGatewayEndpoint
	})

	// Create connection manager
	apiGatewayAdapter := connection.NewAWSAPIGatewayAdapter(apiGatewayClient)
	connManager := connection.NewManager(storeFactory.ConnectionStore(), apiGatewayAdapter, apiGatewayEndpoint)
	connManager.SetLogger(logger.Printf)

	// Create request queue adapter for the router
	queueAdapter := streamer.NewRequestQueueAdapter(storeFactory.RequestQueue())

	// Create Streamer router
	router = streamer.NewRouter(queueAdapter, connManager)
	router.SetAsyncThreshold(5 * time.Second) // Requests >5s go async

	// Register handlers
	registerRouterHandlers(router)

	logger.Println("Router Lambda initialized with DynamORM")
}

// registerRouterHandlers demonstrates registering handlers with the router
func registerRouterHandlers(router streamer.Router) {
	// Fast sync handlers (processed immediately)
	router.Handle("ping", NewPingHandler())
	router.Handle("echo", streamer.NewEchoHandler())
	router.Handle("health", NewHealthHandler())

	// Slow async handlers (queued to DynamoDB for processor)
	router.Handle("generate_report", NewReportHandler())
	router.Handle("knowledge_query", NewKnowledgeBaseHandler())
	router.Handle("process_data", NewDataProcessingHandler())

	logger.Printf("Registered router handlers")
}

// WebSocket Lambda handler
func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error {
	// Let Streamer's router handle everything
	// It will automatically:
	// 1. Route sync requests (<5s) to immediate processing
	// 2. Queue async requests (>5s) to DynamoDB for the processor
	// 3. Handle WebSocket lifecycle ($connect, $disconnect)
	// 4. Manage progress updates and notifications
	return router.Route(ctx, event)
}

// Example Handlers

// PingHandler - Fast sync handler
type PingHandler struct{}

func NewPingHandler() *PingHandler {
	return &PingHandler{}
}

func (h *PingHandler) EstimatedDuration() time.Duration {
	return 100 * time.Millisecond // Fast - will be processed sync
}

func (h *PingHandler) Validate(req *streamer.Request) error {
	return nil // No validation needed for ping
}

func (h *PingHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"pong":      true,
			"timestamp": time.Now().Unix(),
		},
	}, nil
}

// HealthHandler - Fast sync handler
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) EstimatedDuration() time.Duration {
	return 200 * time.Millisecond // Fast - will be processed sync
}

func (h *HealthHandler) Validate(req *streamer.Request) error {
	return nil
}

func (h *HealthHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	return &streamer.Result{
		RequestID: req.ID,
		Success:   true,
		Data: map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"service":   "streamer-router",
		},
	}, nil
}

// ReportHandler - Slow async handler
type ReportHandler struct{}

func NewReportHandler() *ReportHandler {
	return &ReportHandler{}
}

func (h *ReportHandler) EstimatedDuration() time.Duration {
	return 2 * time.Minute // Slow - will be queued for async processing
}

func (h *ReportHandler) Validate(req *streamer.Request) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if _, ok := payload["start_date"]; !ok {
		return fmt.Errorf("start_date is required")
	}

	if _, ok := payload["end_date"]; !ok {
		return fmt.Errorf("end_date is required")
	}

	return nil
}

func (h *ReportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	// This won't be called for async handlers
	// The router will queue this to DynamoDB and return "queued" response
	return nil, fmt.Errorf("this handler should be processed async")
}

// KnowledgeBaseHandler - Slow async handler  
type KnowledgeBaseHandler struct{}

func NewKnowledgeBaseHandler() *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{}
}

func (h *KnowledgeBaseHandler) EstimatedDuration() time.Duration {
	return 15 * time.Second // Slow - will be queued for async processing
}

func (h *KnowledgeBaseHandler) Validate(req *streamer.Request) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if _, ok := payload["query"]; !ok {
		return fmt.Errorf("query is required")
	}

	if _, ok := payload["knowledge_base_id"]; !ok {
		return fmt.Errorf("knowledge_base_id is required")
	}

	return nil
}

func (h *KnowledgeBaseHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	// This won't be called for async handlers
	return nil, fmt.Errorf("this handler should be processed async")
}

// DataProcessingHandler - Slow async handler
type DataProcessingHandler struct{}

func NewDataProcessingHandler() *DataProcessingHandler {
	return &DataProcessingHandler{}
}

func (h *DataProcessingHandler) EstimatedDuration() time.Duration {
	return 5 * time.Minute // Very slow - will be queued for async processing
}

func (h *DataProcessingHandler) Validate(req *streamer.Request) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if _, ok := payload["dataset_id"]; !ok {
		return fmt.Errorf("dataset_id is required")
	}

	return nil
}

func (h *DataProcessingHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
	// This won't be called for async handlers
	return nil, fmt.Errorf("this handler should be processed async")
}

func main() {
	lambda.Start(handler)
}