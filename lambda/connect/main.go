//go:build !lift
// +build !lift

package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/pay-theory/dynamorm/pkg/session"
	dynamormStore "github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/lambda/shared"
)

func main() {
	// Load configuration from environment
	cfg := &HandlerConfig{
		TableName:      getEnv("CONNECTIONS_TABLE", "streamer_connections"),
		JWTPublicKey:   getEnv("JWT_PUBLIC_KEY", ""),
		JWTIssuer:      getEnv("JWT_ISSUER", ""),
		AllowedTenants: getEnvSlice("ALLOWED_TENANTS", []string{}),
		LogLevel:       getEnv("LOG_LEVEL", "INFO"),
	}

	// Validate configuration
	if cfg.JWTPublicKey == "" {
		log.Fatal("JWT_PUBLIC_KEY environment variable is required")
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

	// Create handler
	handler := NewHandler(connStore, cfg, metrics)

	// Log startup
	log.Printf("Connect handler started - Table: %s, Region: %s", cfg.TableName, awsCfg.Region)

	// Start Lambda runtime
	lambda.Start(handler.Handle)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	// Simple comma-separated parsing
	var result []string
	for _, v := range strings.Split(value, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}
