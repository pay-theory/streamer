package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/lambda/shared"
)

var (
	connectionStore store.ConnectionStore
	jwtSecret       string
	tablePrefix     string
)

func init() {
	// Load configuration
	jwtSecret = os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	tablePrefix = os.Getenv("TABLE_PREFIX")
	if tablePrefix == "" {
		tablePrefix = "streamer_"
	}

	// Initialize AWS clients
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Initialize connection store
	dynamoClient := dynamodb.NewFromConfig(cfg)
	connectionStore = store.NewConnectionStore(dynamoClient, tablePrefix)
}

func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Log the connection attempt
	log.Printf("Connection attempt from: %s", event.RequestContext.ConnectionID)

	// Extract JWT token from query parameters
	token := event.QueryStringParameters["Authorization"]
	if token == "" {
		// Also check headers as fallback
		token = event.Headers["Authorization"]
	}

	if token == "" {
		log.Printf("No authorization token provided for connection %s", event.RequestContext.ConnectionID)
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       `{"error": "Authorization required"}`,
		}, nil
	}

	// Validate JWT and extract claims
	claims, err := shared.ValidateJWT(token, jwtSecret)
	if err != nil {
		log.Printf("JWT validation failed for connection %s: %v", event.RequestContext.ConnectionID, err)
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       fmt.Sprintf(`{"error": "Invalid token: %s"}`, err.Error()),
		}, nil
	}

	// Extract user and tenant information from claims
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		log.Printf("Missing user_id in JWT claims for connection %s", event.RequestContext.ConnectionID)
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       `{"error": "Invalid token: missing user_id"}`,
		}, nil
	}

	tenantID, ok := claims["tenant_id"].(string)
	if !ok || tenantID == "" {
		log.Printf("Missing tenant_id in JWT claims for connection %s", event.RequestContext.ConnectionID)
		return events.APIGatewayProxyResponse{
			StatusCode: 401,
			Body:       `{"error": "Invalid token: missing tenant_id"}`,
		}, nil
	}

	// Extract additional metadata
	metadata := make(map[string]string)
	if userAgent := event.Headers["User-Agent"]; userAgent != "" {
		metadata["user_agent"] = userAgent
	}
	if ip := event.RequestContext.Identity.SourceIP; ip != "" {
		metadata["source_ip"] = ip
	}
	if permissions, ok := claims["permissions"].([]interface{}); ok {
		permBytes, _ := json.Marshal(permissions)
		metadata["permissions"] = string(permBytes)
	}

	// Create connection record
	now := time.Now()
	connection := &store.Connection{
		ConnectionID: event.RequestContext.ConnectionID,
		UserID:       userID,
		TenantID:     tenantID,
		Endpoint:     fmt.Sprintf("https://%s/%s", event.RequestContext.DomainName, event.RequestContext.Stage),
		ConnectedAt:  now,
		LastPing:     now,
		Metadata:     metadata,
		TTL:          now.Add(24 * time.Hour).Unix(), // 24-hour TTL
	}

	// Save connection to DynamoDB
	if err := connectionStore.Save(ctx, connection); err != nil {
		log.Printf("Failed to save connection %s: %v", event.RequestContext.ConnectionID, err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       `{"error": "Failed to establish connection"}`,
		}, nil
	}

	// Log successful connection
	log.Printf("Connection established - ID: %s, User: %s, Tenant: %s",
		event.RequestContext.ConnectionID, userID, tenantID)

	// Add structured logging for monitoring
	shared.LogMetric(ctx, "ConnectionEstablished", map[string]interface{}{
		"connection_id": event.RequestContext.ConnectionID,
		"user_id":       userID,
		"tenant_id":     tenantID,
		"timestamp":     now.Unix(),
	})

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message": "Connected successfully"}`,
	}, nil
}

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

	// Initialize AWS SDK
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(awsCfg)

	// Create connection store
	connStore := store.NewConnectionStore(dynamoClient, cfg.TableName)

	// Create CloudWatch metrics client
	metricsNamespace := getEnv("METRICS_NAMESPACE", "Streamer")
	metrics := shared.NewCloudWatchMetrics(awsCfg, metricsNamespace)

	// Create handler
	handler := NewHandler(connStore, cfg, metrics)

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
