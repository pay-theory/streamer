//go:build lift && optimized
// +build lift,optimized

package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/lambda/router"
	"github.com/pay-theory/streamer/pkg/connection"
	"github.com/pay-theory/streamer/pkg/streamer"
)

// RouterHandlerOptimized handles WebSocket message routing using Lift framework
// with optimized patterns but using the ACTUAL Lift API
type RouterHandlerOptimized struct {
	router router.Router
	config *HandlerConfig
}

// NewRouterHandlerOptimized creates a new optimized Lift-based router handler
func NewRouterHandlerOptimized(router router.Router, config *HandlerConfig) *RouterHandlerOptimized {
	return &RouterHandlerOptimized{
		router: router,
		config: config,
	}
}

// HandleMessage processes WebSocket messages
func (h *RouterHandlerOptimized) HandleMessage(ctx *lift.Context) error {
	// Get WebSocket context
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": "Invalid WebSocket context",
			"code":  "INTERNAL_ERROR",
		})
	}

	connectionID := wsCtx.ConnectionID()

	// Get message body from request - Lift handles validation automatically
	body := ctx.Request.Request.Body

	// Parse message structure - basic validation is handled by Lift
	var message map[string]interface{}
	if err := json.Unmarshal(body, &message); err != nil {
		return ctx.Status(400).JSON(map[string]string{
			"error": "Invalid JSON",
			"code":  "INVALID_JSON",
		})
	}

	// Check required fields
	action, ok := message["action"].(string)
	if !ok || action == "" {
		return ctx.Status(400).JSON(map[string]string{
			"error": "Missing required field: action",
			"code":  "MISSING_ACTION",
		})
	}

	// CORRECTED: Get user/tenant context from connection metadata or context values
	// since ctx.UserID() and ctx.TenantID() don't exist
	requestCtx := ctx.Request.Context()

	// Try to get user info from context values (set by connect handler or middleware)
	if userID, ok := ctx.Get("userId").(string); ok && userID != "" {
		requestCtx = context.WithValue(requestCtx, "userId", userID)
	}
	if tenantID, ok := ctx.Get("tenantId").(string); ok && tenantID != "" {
		requestCtx = context.WithValue(requestCtx, "tenantId", tenantID)
	}

	// Log the message
	ctx.Logger.Info("Processing message", map[string]interface{}{
		"connection_id": connectionID,
		"action":        action,
		"message_size":  len(body),
		"has_payload":   message["payload"] != nil,
	})

	// Create the WebSocket event for the router (compatibility layer)
	event := createWebSocketEvent(connectionID, action, body)

	// Route the request using the Streamer router
	err = h.router.Route(requestCtx, event)
	if err != nil {
		ctx.Logger.Error("Failed to route message", map[string]interface{}{
			"connection_id": connectionID,
			"action":        action,
			"error":         err.Error(),
		})

		return ctx.Status(500).JSON(map[string]string{
			"error": "Failed to process message",
			"code":  "ROUTING_ERROR",
		})
	}

	// Return success (the router handles the actual response via WebSocket)
	return ctx.Status(200).JSON(map[string]string{
		"status": "processed",
	})
}

// createWebSocketEvent creates a compatible WebSocket event for the Streamer router
func createWebSocketEvent(connectionID, action string, body []byte) events.APIGatewayWebsocketProxyRequest {
	return events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: connectionID,
			RouteKey:     action,
		},
		Body: string(body),
	}
}

// isAsyncAction checks if an action should be processed asynchronously
func isAsyncAction(action string) bool {
	asyncActions := map[string]bool{
		"generate_report": true,
		"process_data":    true,
		"bulk_operation":  true,
	}
	return asyncActions[action]
}

// CreateStreamerRouter creates and configures the Streamer router for use with optimized Lift
func CreateStreamerRouter(
	connStore store.ConnectionStore,
	reqQueue store.RequestQueue,
	apiGatewayClient *apigatewaymanagementapi.Client,
	wsEndpoint string,
	logger *log.Logger,
) (*streamer.DefaultRouter, error) {
	// Create adapters for Streamer compatibility
	queueAdapter := streamer.NewRequestQueueAdapter(reqQueue)

	// Wrap the AWS SDK client with the adapter
	apiGatewayAdapter := connection.NewAWSAPIGatewayAdapter(apiGatewayClient)

	// Create ConnectionManager
	connManager := connection.NewManager(connStore, apiGatewayAdapter, wsEndpoint)
	connManager.SetLogger(logger.Printf)

	// Create router
	router := streamer.NewRouter(queueAdapter, connManager)
	router.SetAsyncThreshold(5 * time.Second)

	// Apply minimal Streamer middleware (validation/metrics handled by Lift)
	router.SetMiddleware(
		streamer.LoggingMiddleware(logger.Printf),
	)

	// Register handlers
	if err := registerHandlers(router); err != nil {
		return nil, err
	}

	return router, nil
}
