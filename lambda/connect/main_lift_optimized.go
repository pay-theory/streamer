//go:build lift && optimized
// +build lift,optimized

package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/middleware"
	"github.com/pay-theory/lift/pkg/observability"
	observabilityCloudwatch "github.com/pay-theory/lift/pkg/observability/cloudwatch"
	"github.com/pay-theory/lift/pkg/observability/zap"
	"github.com/pay-theory/lift/pkg/security"
	dynamormStore "github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/pay-theory/streamer/lambda/shared"
)

func main() {
	// Load AWS configuration
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
	metricsNamespace := os.Getenv("METRICS_NAMESPACE")
	if metricsNamespace == "" {
		metricsNamespace = "Streamer"
	}
	metrics := shared.NewCloudWatchMetrics(awsCfg, metricsNamespace)

	// Create handler configuration
	cfg := &HandlerConfig{
		JWTPublicKey:   os.Getenv("JWT_PUBLIC_KEY"),
		JWTIssuer:      os.Getenv("JWT_ISSUER"),
		AllowedTenants: parseAllowedTenants(os.Getenv("ALLOWED_TENANTS")),
	}

	// Validate configuration
	if cfg.JWTPublicKey == "" {
		log.Fatal("JWT_PUBLIC_KEY environment variable is required")
	}

	// Create optimized handler
	handler := NewConnectHandlerOptimized(connStore, cfg, metrics)

	// Create Lift app with WebSocket support
	app := lift.New(lift.WithWebSocketSupport())

	// 1. JWT Authentication using correct API
	jwtConfig := security.JWTConfig{
		SigningMethod:   "RS256",
		PublicKeyPath:   cfg.JWTPublicKey, // Assuming this is a path, not the key content
		Issuer:          cfg.JWTIssuer,
		Audience:        []string{"streamer-api"},
		MaxAge:          time.Hour,
		RequireTenantID: true,
	}
	app.Use(middleware.JWT(jwtConfig))

	// 2. Create observability components
	// Logger setup
	loggerFactory := zap.NewZapLoggerFactory()
	logger, err := loggerFactory.CreateConsoleLogger(observability.LoggerConfig{
		Level:  getEnv("LOG_LEVEL", "info"),
		Format: "json",
	})
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	// Metrics setup - use CloudWatch for production
	metricsConfig := observabilityCloudwatch.CloudWatchMetricsConfig{
		Namespace:     metricsNamespace,
		BufferSize:    1000,
		FlushInterval: 60 * time.Second,
		Dimensions: map[string]string{
			"Environment": getEnv("ENVIRONMENT", "development"),
			"Service":     "streamer-connect",
		},
	}

	cloudwatchClient := cloudwatch.NewFromConfig(awsCfg)
	metricsCollector := observabilityCloudwatch.NewCloudWatchMetrics(cloudwatchClient, metricsConfig)

	// 3. Observability middleware
	app.Use(middleware.ObservabilityMiddleware(middleware.ObservabilityConfig{
		Logger:  logger,
		Metrics: metricsCollector,
	}))

	// 4. WebSocket-specific metrics
	app.Use(middleware.WebSocketMetrics(metricsCollector))

	// 5. WebSocket Authentication (if needed for additional validation)
	wsAuthConfig := middleware.WebSocketAuthConfig{
		JWTConfig: jwtConfig,
		TokenExtractor: func(ctx *lift.Context) string {
			// Extract from query params for WebSocket connections
			token := ctx.Query("Authorization")
			if token == "" {
				token = ctx.Query("token")
			}
			return strings.TrimPrefix(token, "Bearer ")
		},
		OnError: func(ctx *lift.Context, err error) error {
			log.Printf("WebSocket auth failed: %v", err)
			return ctx.Status(401).JSON(map[string]string{
				"error": "Authentication failed: " + err.Error(),
				"code":  "UNAUTHORIZED",
			})
		},
		SkipRoutes: []string{}, // Don't skip any routes for connect
	}
	app.Use(middleware.WebSocketAuth(wsAuthConfig))

	// Register WebSocket connect handler
	app.WebSocket("$connect", handler.HandleConnect)

	// Log startup
	log.Printf("Connect handler (Lift optimized with full middleware stack) started - Region: %s", awsCfg.Region)

	// Start the Lambda handler
	lambda.Start(app.WebSocketHandler())
}

// parseAllowedTenants parses a comma-separated list of tenant IDs
func parseAllowedTenants(tenants string) []string {
	if tenants == "" {
		return []string{}
	}

	result := []string{}
	for _, tenant := range strings.Split(tenants, ",") {
		trimmed := strings.TrimSpace(tenant)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
