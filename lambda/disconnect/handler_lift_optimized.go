//go:build lift && optimized
// +build lift,optimized

package main

import (
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/lambda/shared"
)

// DisconnectHandlerOptimized handles WebSocket $disconnect requests using Lift's built-in features
type DisconnectHandlerOptimized struct {
	connStore     store.ConnectionStore
	subStore      SubscriptionStore
	requestStore  RequestStore
	config        *HandlerConfig
	metricsLogger *MetricsLogger
	metrics       shared.MetricsPublisher
}

// NewDisconnectHandlerOptimized creates a new optimized Lift-based disconnect handler
func NewDisconnectHandlerOptimized(connStore store.ConnectionStore, subStore SubscriptionStore, requestStore RequestStore, config *HandlerConfig, metrics shared.MetricsPublisher) *DisconnectHandlerOptimized {
	return &DisconnectHandlerOptimized{
		connStore:     connStore,
		subStore:      subStore,
		requestStore:  requestStore,
		config:        config,
		metricsLogger: NewMetricsLogger(config.MetricsEnabled),
		metrics:       metrics,
	}
}

// HandleDisconnect processes the WebSocket $disconnect event using Lift's built-in features
// Metrics and tracing are handled automatically by Lift
func (h *DisconnectHandlerOptimized) HandleDisconnect(ctx *lift.Context) error {
	// Get WebSocket context
	wsCtx, err := ctx.AsWebSocket()
	if err != nil {
		return ctx.Status(500).JSON(map[string]string{
			"error": "Invalid WebSocket context",
			"code":  "INTERNAL_ERROR",
		})
	}

	connectionID := wsCtx.ConnectionID()
	startTime := time.Now()

	// Log the disconnection attempt
	ctx.Logger.Info("Processing disconnect", map[string]interface{}{
		"connection_id": connectionID,
	})

	// Initialize metrics
	metrics := &DisconnectMetrics{
		ConnectionID:     connectionID,
		DisconnectTime:   time.Now(),
		DisconnectReason: "client_disconnect",
	}

	// Get connection details before deletion for metrics
	conn, err := h.connStore.Get(ctx.Request.Context(), connectionID)
	if err != nil {
		// Connection might already be deleted
		ctx.Logger.Warn("Connection not found during disconnect", map[string]interface{}{
			"connection_id": connectionID,
			"error":         err.Error(),
		})
		metrics.ConnectionNotFound = true
	} else {
		// Calculate connection duration
		metrics.UserID = conn.UserID
		metrics.TenantID = conn.TenantID
		metrics.ConnectedAt = conn.ConnectedAt
		metrics.DurationSeconds = int64(time.Since(conn.ConnectedAt).Seconds())

		// Extract message counts from metadata if available
		if messagesStr, ok := conn.Metadata["messages_sent"]; ok {
			metrics.MessagesSent = parseIntOrDefault(messagesStr, 0)
		}
		if messagesStr, ok := conn.Metadata["messages_received"]; ok {
			metrics.MessagesReceived = parseIntOrDefault(messagesStr, 0)
		}
	}

	// Delete the connection record
	if err := h.connStore.Delete(ctx.Request.Context(), connectionID); err != nil {
		ctx.Logger.Error("Failed to delete connection", map[string]interface{}{
			"connection_id": connectionID,
			"error":         err.Error(),
		})
		metrics.DeleteError = err.Error()
		// Don't return error - connection is already closed on API Gateway side
	}

	// Clean up subscriptions if store is available
	if h.subStore != nil {
		// Count subscriptions before deletion for metrics
		subCount, err := h.subStore.CountByConnection(ctx.Request.Context(), connectionID)
		if err == nil {
			metrics.SubscriptionsCancelled = subCount
		}

		// Delete all subscriptions for this connection
		if err := h.subStore.DeleteByConnection(ctx.Request.Context(), connectionID); err != nil {
			ctx.Logger.Error("Failed to delete subscriptions", map[string]interface{}{
				"connection_id": connectionID,
				"error":         err.Error(),
			})
			metrics.SubscriptionError = err.Error()
		}
	}

	// Cancel any in-progress async requests
	if h.requestStore != nil {
		cancelledCount, err := h.requestStore.CancelByConnection(ctx.Request.Context(), connectionID)
		if err != nil {
			ctx.Logger.Error("Failed to cancel requests", map[string]interface{}{
				"connection_id": connectionID,
				"error":         err.Error(),
			})
			metrics.RequestError = err.Error()
		} else {
			metrics.RequestsCancelled = cancelledCount
		}
	}

	// Calculate cleanup duration
	metrics.CleanupDurationMs = int64(time.Since(startTime).Milliseconds())

	// Log metrics
	h.metricsLogger.LogDisconnect(ctx.Request.Context(), metrics)

	// Log structured summary
	ctx.Logger.Info("Disconnect completed", map[string]interface{}{
		"connection_id":           connectionID,
		"user_id":                 metrics.UserID,
		"tenant_id":               metrics.TenantID,
		"duration_seconds":        metrics.DurationSeconds,
		"messages_sent":           metrics.MessagesSent,
		"messages_received":       metrics.MessagesReceived,
		"subscriptions_cancelled": metrics.SubscriptionsCancelled,
		"requests_cancelled":      metrics.RequestsCancelled,
		"cleanup_duration_ms":     metrics.CleanupDurationMs,
	})

	// Publish CloudWatch metrics
	environment := os.Getenv("ENVIRONMENT")

	// Connection closed metric
	h.metrics.PublishMetric(ctx.Request.Context(), "", shared.CommonMetrics.ConnectionClosed, 1, types.StandardUnitCount,
		shared.MetricsDimensions{}.Environment(environment),
		shared.MetricsDimensions{}.TenantID(metrics.TenantID))

	// Connection duration metric (if we found the connection)
	if !metrics.ConnectionNotFound && metrics.DurationSeconds > 0 {
		h.metrics.PublishMetric(ctx.Request.Context(), "", shared.CommonMetrics.ConnectionDuration, float64(metrics.DurationSeconds), types.StandardUnitSeconds,
			shared.MetricsDimensions{}.Environment(environment),
			shared.MetricsDimensions{}.TenantID(metrics.TenantID))
	}

	// Always return success - the connection is already closed
	return ctx.Status(200).JSON(map[string]string{
		"message": "Disconnected successfully",
	})
}
