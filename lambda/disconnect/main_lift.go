//go:build lift
// +build lift

package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/pay-theory/lift/pkg/lift"
	dynamormStore "github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/lambda/shared"
)

func main() {
	// Load configuration from environment
	cfg := &HandlerConfig{
		ConnectionsTable:   getEnv("CONNECTIONS_TABLE", "streamer_connections"),
		SubscriptionsTable: getEnv("SUBSCRIPTIONS_TABLE", "streamer_subscriptions"),
		RequestsTable:      getEnv("REQUESTS_TABLE", "streamer_requests"),
		MetricsEnabled:     getEnv("METRICS_ENABLED", "true") == "true",
		LogLevel:           getEnv("LOG_LEVEL", "INFO"),
	}

	// Initialize AWS SDK for CloudWatch metrics
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

	// Get connection store from factory
	connStore := factory.ConnectionStore()

	// Create CloudWatch metrics client
	metricsNamespace := getEnv("METRICS_NAMESPACE", "Streamer")
	metrics := shared.NewCloudWatchMetrics(awsCfg, metricsNamespace)

	// TODO: Initialize subscription and request stores when available
	// For now, we'll pass nil which the handler checks for
	var subStore SubscriptionStore
	var requestStore RequestStore

	// Create optimized Lift-based handler
	handler := NewDisconnectHandlerOptimized(connStore, subStore, requestStore, cfg, metrics)

	// Create Lift app with WebSocket support and built-in middleware
	app := lift.New(lift.WithWebSocketSupport())

	// Register WebSocket disconnect handler
	app.Handle("DISCONNECT", "/disconnect", handler.HandleDisconnect)

	// Log startup
	log.Printf("Disconnect handler (Lift Optimized) started - Tables: %s, %s, %s - Region: %s",
		cfg.ConnectionsTable, cfg.SubscriptionsTable, cfg.RequestsTable, awsCfg.Region)

	// Start Lambda runtime with Lift's HandleRequest method
	lambda.Start(app.HandleRequest)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
