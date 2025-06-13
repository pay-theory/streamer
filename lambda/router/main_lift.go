//go:build lift
// +build lift

package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/pay-theory/lift/pkg/lift"
	dynamormStore "github.com/pay-theory/streamer/internal/store/dynamorm"
)

func main() {
	// Load configuration from environment
	cfg := &HandlerConfig{
		ConnectionsTable:   getEnv("CONNECTIONS_TABLE", "streamer_connections"),
		RequestsTable:      getEnv("REQUESTS_TABLE", "streamer_requests"),
		SubscriptionsTable: getEnv("SUBSCRIPTIONS_TABLE", "streamer_subscriptions"),
		WebSocketEndpoint:  os.Getenv("WEBSOCKET_ENDPOINT"),
		AsyncThreshold:     5 * time.Second,
	}

	if cfg.WebSocketEndpoint == "" {
		log.Fatal("WEBSOCKET_ENDPOINT environment variable is required")
	}

	// Initialize AWS SDK
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Initialize DynamORM
	dynamormConfig := session.Config{
		Region: awsCfg.Region,
	}

	factory, err := dynamormStore.NewStoreFactory(dynamormConfig)
	if err != nil {
		log.Fatalf("Failed to create DynamORM factory: %v", err)
	}

	// Get stores from factory
	connStore := factory.ConnectionStore()
	reqQueue := factory.RequestQueue()

	// Initialize API Gateway Management API client
	apiGatewayClient := apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &cfg.WebSocketEndpoint
	})

	// Create logger
	logger := log.New(os.Stdout, "[ROUTER-LIFT] ", log.LstdFlags|log.Lshortfile)

	// Create Streamer router (using the existing Streamer framework)
	router, err := CreateStreamerRouter(connStore, reqQueue, apiGatewayClient, cfg.WebSocketEndpoint, logger)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	// Create Lift-based handler
	handler := NewRouterHandlerLift(router, cfg)

	// Create Lift app with WebSocket support
	app := lift.New()

	// Add middleware stack
	app.Use(handler.ValidationMiddleware())
	app.Use(handler.MetricsMiddleware())
	app.Use(handler.TracingMiddleware())

	// Register WebSocket message handler
	// The router handles all message events
	app.Handle("MESSAGE", "/message", handler.HandleMessage)

	// Log startup
	logger.Printf("Router Lambda (Lift) initialized - Tables: %s, %s, %s - Region: %s",
		cfg.ConnectionsTable, cfg.RequestsTable, cfg.SubscriptionsTable, awsCfg.Region)

	// Start Lambda runtime with Lift's HandleRequest method
	lambda.Start(app.HandleRequest)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
