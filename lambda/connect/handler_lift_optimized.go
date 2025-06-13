//go:build lift && optimized
// +build lift,optimized

package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/lambda/shared"
)

// ConnectHandlerOptimized handles WebSocket $connect requests using Lift framework
// with full middleware stack including JWT, observability, and WebSocket metrics
type ConnectHandlerOptimized struct {
	store   store.ConnectionStore
	config  *HandlerConfig
	metrics shared.MetricsPublisher
}

// NewConnectHandlerOptimized creates a new optimized Lift-based connect handler
func NewConnectHandlerOptimized(store store.ConnectionStore, config *HandlerConfig, metrics shared.MetricsPublisher) *ConnectHandlerOptimized {
	return &ConnectHandlerOptimized{
		store:   store,
		config:  config,
		metrics: metrics,
	}
}

// HandleConnect processes the WebSocket $connect event
// JWT validation is handled by Lift's JWT middleware stack!
func (h *ConnectHandlerOptimized) HandleConnect(ctx *lift.Context) error {
	connectionID := ctx.Request.Metadata["connectionId"].(string)

	// JWT is validated by middleware! Use the correct context methods
	userID := ctx.UserID()     // Direct access from "sub" claim
	tenantID := ctx.TenantID() // Direct access from "tenant_id" claim

	if userID == "" {
		// This shouldn't happen if JWT middleware is configured correctly
		log.Printf("No user ID found for connection %s", connectionID)
		return ctx.Status(401).JSON(map[string]string{
			"error": "Authentication required",
			"code":  "UNAUTHORIZED",
		})
	}

	// Get additional claims via security context
	secCtx := lift.WithSecurity(ctx)
	principal := secCtx.GetPrincipal()

	var roles []string
	var scopes []string
	if principal != nil {
		roles = principal.Roles
		scopes = principal.Scopes
	}

	// Validate tenant if restrictions are configured
	if len(h.config.AllowedTenants) > 0 {
		allowed := false
		for _, tenant := range h.config.AllowedTenants {
			if tenantID == tenant {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Printf("Tenant not allowed for connection %s: %s", connectionID, tenantID)

			// Metrics are handled by middleware, but we can still publish custom business metrics
			h.publishAuthFailureMetric(context.Background(), "tenant_not_allowed", tenantID)

			return ctx.Status(401).JSON(map[string]string{
				"error": "Tenant not allowed",
				"code":  "UNAUTHORIZED",
			})
		}
	}

	// Get WebSocket context for management endpoint
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": "Failed to get WebSocket context",
			"code":  "INTERNAL_ERROR",
		})
	}

	// Get source IP from headers
	sourceIP := ctx.Header("X-Forwarded-For")
	if sourceIP == "" {
		sourceIP = ctx.Header("X-Real-IP")
	}

	// Create connection record with authenticated user info
	connection := &store.Connection{
		ConnectionID: connectionID,
		UserID:       userID,
		TenantID:     tenantID,
		Endpoint:     wsCtx.ManagementEndpoint(),
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
		Metadata: map[string]string{
			"user_agent":    ctx.Header("User-Agent"),
			"ip_address":    sourceIP,
			"roles":         jsonStringify(roles),
			"scopes":        jsonStringify(scopes),
			"stage":         wsCtx.Stage(),
			"authenticated": "true",
		},
		TTL: time.Now().Add(24 * time.Hour).Unix(),
	}

	// Save connection to store
	err = h.store.Save(context.Background(), connection)
	if err != nil {
		log.Printf("Failed to save connection %s: %v", connectionID, err)
		return ctx.Status(500).JSON(map[string]string{
			"error": "Failed to establish connection",
			"code":  "INTERNAL_ERROR",
		})
	}

	// Log successful connection (enhanced logging handled by middleware)
	log.Printf("Connection established: connectionId=%s, userId=%s, tenantId=%s",
		connectionID, connection.UserID, connection.TenantID)

	// Success metrics are handled by middleware automatically!
	// We only publish custom business metrics
	h.metrics.PublishMetric(context.Background(), "", shared.CommonMetrics.ConnectionEstablished, 1, types.StandardUnitCount,
		shared.MetricsDimensions{}.Environment(os.Getenv("ENVIRONMENT")),
		shared.MetricsDimensions{}.TenantID(connection.TenantID))

	// Store connection metadata in context for potential downstream use
	ctx.Set("userId", userID)
	ctx.Set("tenantId", tenantID)
	ctx.Set("authenticated", true)

	// Return success response
	return ctx.Status(200).JSON(map[string]interface{}{
		"message":      "Connected successfully",
		"connectionId": connectionID,
		"userId":       userID,
		"tenantId":     tenantID,
		"roles":        roles,
		"scopes":       scopes,
	})
}

// publishAuthFailureMetric publishes authentication failure metrics
// (Most auth failures are now handled by JWT middleware, this is for custom business logic)
func (h *ConnectHandlerOptimized) publishAuthFailureMetric(ctx context.Context, errorType, tenantID string) {
	dimensions := []types.Dimension{
		shared.MetricsDimensions{}.Environment(os.Getenv("ENVIRONMENT")),
		shared.MetricsDimensions{}.ErrorType(errorType),
	}

	if tenantID != "" {
		dimensions = append(dimensions, shared.MetricsDimensions{}.TenantID(tenantID))
	}

	h.metrics.PublishMetric(ctx, "", shared.CommonMetrics.AuthenticationFailed, 1, types.StandardUnitCount, dimensions...)
}
