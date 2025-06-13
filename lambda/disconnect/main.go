//go:build !lift
// +build !lift

package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/pay-theory/dynamorm/pkg/session"
	dynamormStore "github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/lambda/shared"
)

func main() {
	// Load configuration from environment
	cfg := &HandlerConfig{
		ConnectionsTable:   getEnv("CONNECTIONS_TABLE", "streamer_connections"),
		SubscriptionsTable: getEnv("SUBSCRIPTIONS_TABLE", "streamer_subscriptions"),
		RequestsTable:      getEnv("REQUESTS_TABLE", "streamer_requests"),
		MetricsEnabled:     getEnvBool("METRICS_ENABLED", true),
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

	// Get stores from factory
	connStore := factory.ConnectionStore()
	// Note: SubscriptionStore and RequestStore would be initialized here when implemented

	// Create CloudWatch metrics client
	metricsNamespace := getEnv("METRICS_NAMESPACE", "Streamer")
	metrics := shared.NewCloudWatchMetrics(awsCfg, metricsNamespace)

	// Create handler
	handler := NewHandler(connStore, nil, nil, cfg, metrics) // nil for subscription/request stores for now

	// Start Lambda runtime
	lambda.Start(handler.Handle)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}
