//go:build !lift
// +build !lift

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/lambda/shared"
)

// JWTVerifierInterface defines the interface for JWT verification
type JWTVerifierInterface interface {
	Verify(token string) (*Claims, error)
}

// HandlerConfig is defined in common.go

// Handler handles WebSocket $connect requests
type Handler struct {
	store       store.ConnectionStore
	config      *HandlerConfig
	jwtVerifier JWTVerifierInterface
	logger      *shared.Logger
	metrics     shared.MetricsPublisher
}

// NewHandler creates a new connect handler
func NewHandler(store store.ConnectionStore, config *HandlerConfig, metrics shared.MetricsPublisher) *Handler {
	verifier, err := NewJWTVerifier(config.JWTPublicKey, config.JWTIssuer)
	if err != nil {
		log.Fatalf("Failed to create JWT verifier: %v", err)
	}

	return &Handler{
		store:       store,
		config:      config,
		jwtVerifier: verifier,
		logger:      shared.NewLogger("connect-handler"),
		metrics:     metrics,
	}
}

// NewHandlerWithVerifier creates a new connect handler with a custom JWT verifier (for testing)
func NewHandlerWithVerifier(store store.ConnectionStore, config *HandlerConfig, metrics shared.MetricsPublisher, verifier JWTVerifierInterface) *Handler {
	return &Handler{
		store:       store,
		config:      config,
		jwtVerifier: verifier,
		logger:      shared.NewLogger("connect-handler"),
		metrics:     metrics,
	}
}

// Handle processes the WebSocket $connect event
func (h *Handler) Handle(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	start := time.Now()
	connectionID := event.RequestContext.ConnectionID

	// Start X-Ray tracing
	traceData := shared.TraceSegment{
		ConnectionID: connectionID,
		Action:       "connect",
	}
	ctx, seg := shared.StartSubsegment(ctx, "HandleConnect", traceData)
	defer func() {
		shared.EndSubsegment(seg, nil)
	}()

	// Log the connection attempt with structured logging
	h.logger.Info(ctx, "Connection attempt", map[string]interface{}{
		"connection_id": connectionID,
		"source_ip":     event.RequestContext.Identity.SourceIP,
		"user_agent":    event.Headers["User-Agent"],
	})

	// Extract JWT from query string
	token := event.QueryStringParameters["Authorization"]
	if token == "" {
		// Try headers as fallback
		token = event.Headers["Authorization"]
	}

	if token == "" {
		h.logger.Warn(ctx, "Missing authorization token", map[string]interface{}{
			"connection_id": connectionID,
		})

		// Publish metric for auth failure
		h.metrics.PublishMetric(ctx, "", shared.CommonMetrics.AuthenticationFailed, 1, types.StandardUnitCount,
			shared.MetricsDimensions{}.Environment(os.Getenv("ENVIRONMENT")),
			shared.MetricsDimensions{}.ErrorType("missing_token"))

		return unauthorizedResponse("Missing authorization token")
	}

	// Validate JWT
	ctx2, jwtSeg := shared.StartSubsegment(ctx, "ValidateJWT", shared.TraceSegment{})
	claims, err := h.jwtVerifier.Verify(token)
	shared.EndSubsegment(jwtSeg, err)

	if err != nil {
		h.logger.Error(ctx2, "JWT validation failed", map[string]interface{}{
			"connection_id": connectionID,
			"error":         err.Error(),
		})

		// Publish metric for auth failure
		h.metrics.PublishMetric(ctx, "", shared.CommonMetrics.AuthenticationFailed, 1, types.StandardUnitCount,
			shared.MetricsDimensions{}.Environment(os.Getenv("ENVIRONMENT")),
			shared.MetricsDimensions{}.ErrorType("invalid_jwt"))

		return unauthorizedResponse(fmt.Sprintf("Invalid token: %v", err))
	}

	// Update trace data with user info
	traceData.UserID = claims.Subject
	traceData.TenantID = claims.TenantID
	shared.AddTraceAnnotation(ctx, "user_id", claims.Subject)
	shared.AddTraceAnnotation(ctx, "tenant_id", claims.TenantID)

	// Validate tenant if restrictions are configured
	if len(h.config.AllowedTenants) > 0 {
		allowed := false
		for _, tenant := range h.config.AllowedTenants {
			if claims.TenantID == tenant {
				allowed = true
				break
			}
		}
		if !allowed {
			h.logger.Warn(ctx, "Tenant not allowed", map[string]interface{}{
				"connection_id": connectionID,
				"tenant_id":     claims.TenantID,
			})

			// Publish metric for auth failure
			h.metrics.PublishMetric(ctx, "", shared.CommonMetrics.AuthenticationFailed, 1, types.StandardUnitCount,
				shared.MetricsDimensions{}.Environment(os.Getenv("ENVIRONMENT")),
				shared.MetricsDimensions{}.ErrorType("tenant_not_allowed"),
				shared.MetricsDimensions{}.TenantID(claims.TenantID))

			return unauthorizedResponse("Tenant not allowed")
		}
	}

	// Create connection record
	connection := &store.Connection{
		ConnectionID: event.RequestContext.ConnectionID,
		UserID:       claims.Subject,
		TenantID:     claims.TenantID,
		Endpoint:     fmt.Sprintf("%s/%s", event.RequestContext.DomainName, event.RequestContext.Stage),
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
		Metadata: map[string]string{
			"user_agent":  event.Headers["User-Agent"],
			"ip_address":  event.RequestContext.Identity.SourceIP,
			"permissions": jsonStringify(claims.Permissions),
		},
		TTL: time.Now().Add(24 * time.Hour).Unix(),
	}

	// Save connection to store
	ctx3, saveSeg := shared.StartSubsegment(ctx, "SaveConnection", shared.TraceSegment{})
	err = h.store.Save(ctx3, connection)
	shared.EndSubsegment(saveSeg, err)

	if err != nil {
		h.logger.Error(ctx, "Failed to save connection", map[string]interface{}{
			"connection_id": connectionID,
			"error":         err.Error(),
		})
		return internalErrorResponse("Failed to establish connection")
	}

	// Log successful connection
	h.logger.Info(ctx, "Connection established", map[string]interface{}{
		"connection_id": connectionID,
		"user_id":       connection.UserID,
		"tenant_id":     connection.TenantID,
	})

	// Publish success metrics
	h.metrics.PublishMetric(ctx, "", shared.CommonMetrics.ConnectionEstablished, 1, types.StandardUnitCount,
		shared.MetricsDimensions{}.Environment(os.Getenv("ENVIRONMENT")),
		shared.MetricsDimensions{}.TenantID(connection.TenantID))

	// Track connection latency
	latency := time.Since(start)
	h.metrics.PublishLatency(ctx, "", shared.CommonMetrics.ProcessingLatency, latency,
		shared.MetricsDimensions{}.Environment(os.Getenv("ENVIRONMENT")),
		shared.MetricsDimensions{}.Action("connect"))

	// Return success response
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message":"Connected successfully"}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// Helper functions for responses
func unauthorizedResponse(message string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"error": message,
		"code":  "UNAUTHORIZED",
	})

	return events.APIGatewayProxyResponse{
		StatusCode: 401,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

func internalErrorResponse(message string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"error": message,
		"code":  "INTERNAL_ERROR",
	})

	return events.APIGatewayProxyResponse{
		StatusCode: 500,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// jsonStringify is defined in common.go
