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

// HandlerConfig holds configuration for the handler
type HandlerConfig struct {
	ConnectionsTable   string
	SubscriptionsTable string
	RequestsTable      string
	MetricsEnabled     bool
	LogLevel           string
}

// Handler handles WebSocket $disconnect requests
type Handler struct {
	connStore     store.ConnectionStore
	subStore      SubscriptionStore // Interface for future implementation
	requestStore  RequestStore      // Interface for future implementation
	config        *HandlerConfig
	metricsLogger *MetricsLogger
	logger        *shared.Logger
	metrics       shared.MetricsPublisher
}

// SubscriptionStore interface for future implementation
type SubscriptionStore interface {
	DeleteByConnection(ctx context.Context, connectionID string) error
	CountByConnection(ctx context.Context, connectionID string) (int, error)
}

// RequestStore interface for future implementation
type RequestStore interface {
	CancelByConnection(ctx context.Context, connectionID string) (int, error)
}

// NewHandler creates a new disconnect handler
func NewHandler(connStore store.ConnectionStore, subStore SubscriptionStore, requestStore RequestStore, config *HandlerConfig, metrics shared.MetricsPublisher) *Handler {
	return &Handler{
		connStore:     connStore,
		subStore:      subStore,
		requestStore:  requestStore,
		config:        config,
		metricsLogger: NewMetricsLogger(config.MetricsEnabled),
		logger:        shared.NewLogger("disconnect-handler"),
		metrics:       metrics,
	}
}

// Handle processes the WebSocket $disconnect event
func (h *Handler) Handle(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionID := event.RequestContext.ConnectionID
	startTime := time.Now()

	// Start X-Ray tracing
	traceData := shared.TraceSegment{
		ConnectionID: connectionID,
		Action:       "disconnect",
	}
	ctx, seg := shared.StartSubsegment(ctx, "HandleDisconnect", traceData)
	defer func() {
		shared.EndSubsegment(seg, nil)
	}()

	// Log the disconnection attempt with structured logging
	h.logger.Info(ctx, "Processing disconnect", map[string]interface{}{
		"connection_id": connectionID,
	})

	// Initialize metrics
	metrics := &DisconnectMetrics{
		ConnectionID:     connectionID,
		DisconnectTime:   time.Now(),
		DisconnectReason: "client_disconnect", // Default reason
	}

	// Get connection details before deletion for metrics
	conn, err := h.connStore.Get(ctx, connectionID)
	if err != nil {
		// Connection might already be deleted
		log.Printf("Connection %s not found during disconnect: %v", connectionID, err)
		metrics.ConnectionNotFound = true
	} else {
		// Calculate connection duration
		metrics.UserID = conn.UserID
		metrics.TenantID = conn.TenantID
		metrics.ConnectedAt = conn.ConnectedAt
		metrics.DurationSeconds = int64(time.Since(conn.ConnectedAt).Seconds())

		// Extract message counts from metadata if available
		if messagesStr, ok := conn.Metadata["messages_sent"]; ok {
			// Parse message count - this would be updated by the message processor
			metrics.MessagesSent = parseIntOrDefault(messagesStr, 0)
		}
		if messagesStr, ok := conn.Metadata["messages_received"]; ok {
			metrics.MessagesReceived = parseIntOrDefault(messagesStr, 0)
		}
	}

	// Delete the connection record
	if err := h.connStore.Delete(ctx, connectionID); err != nil {
		log.Printf("Failed to delete connection %s: %v", connectionID, err)
		metrics.DeleteError = err.Error()
		// Don't return error - connection is already closed on API Gateway side
	}

	// Clean up subscriptions if store is available
	if h.subStore != nil {
		// Count subscriptions before deletion for metrics
		subCount, err := h.subStore.CountByConnection(ctx, connectionID)
		if err == nil {
			metrics.SubscriptionsCancelled = subCount
		}

		// Delete all subscriptions for this connection
		if err := h.subStore.DeleteByConnection(ctx, connectionID); err != nil {
			log.Printf("Failed to delete subscriptions for connection %s: %v", connectionID, err)
			metrics.SubscriptionError = err.Error()
			// Don't return error - continue cleanup
		}
	}

	// Cancel any in-progress async requests
	if h.requestStore != nil {
		cancelledCount, err := h.requestStore.CancelByConnection(ctx, connectionID)
		if err != nil {
			log.Printf("Failed to cancel requests for connection %s: %v", connectionID, err)
			metrics.RequestError = err.Error()
		} else {
			metrics.RequestsCancelled = cancelledCount
		}
	}

	// Calculate cleanup duration
	metrics.CleanupDurationMs = int64(time.Since(startTime).Milliseconds())

	// Log metrics
	h.metricsLogger.LogDisconnect(ctx, metrics)

	// Log structured summary
	h.logger.Info(ctx, "Disconnect completed", map[string]interface{}{
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

	// Update trace data with user info
	if metrics.UserID != "" {
		shared.AddTraceAnnotation(ctx, "user_id", metrics.UserID)
		shared.AddTraceAnnotation(ctx, "tenant_id", metrics.TenantID)
	}

	// Publish CloudWatch metrics
	environment := os.Getenv("ENVIRONMENT")

	// Connection closed metric
	h.metrics.PublishMetric(ctx, "", shared.CommonMetrics.ConnectionClosed, 1, types.StandardUnitCount,
		shared.MetricsDimensions{}.Environment(environment),
		shared.MetricsDimensions{}.TenantID(metrics.TenantID))

	// Connection duration metric (if we found the connection)
	if !metrics.ConnectionNotFound && metrics.DurationSeconds > 0 {
		h.metrics.PublishMetric(ctx, "", shared.CommonMetrics.ConnectionDuration, float64(metrics.DurationSeconds), types.StandardUnitSeconds,
			shared.MetricsDimensions{}.Environment(environment),
			shared.MetricsDimensions{}.TenantID(metrics.TenantID))
	}

	// Cleanup latency metric
	h.metrics.PublishLatency(ctx, "", shared.CommonMetrics.ProcessingLatency, time.Since(startTime),
		shared.MetricsDimensions{}.Environment(environment),
		shared.MetricsDimensions{}.Action("disconnect"))

	// Always return success - the connection is already closed
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message":"Disconnected successfully"}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// DisconnectMetrics holds metrics for a disconnect event
type DisconnectMetrics struct {
	ConnectionID           string
	UserID                 string
	TenantID               string
	ConnectedAt            time.Time
	DisconnectTime         time.Time
	DisconnectReason       string
	DurationSeconds        int64
	MessagesSent           int
	MessagesReceived       int
	SubscriptionsCancelled int
	RequestsCancelled      int
	CleanupDurationMs      int64
	ConnectionNotFound     bool
	DeleteError            string
	SubscriptionError      string
	RequestError           string
}

// MetricsLogger handles metrics logging
type MetricsLogger struct {
	enabled bool
}

// NewMetricsLogger creates a new metrics logger
func NewMetricsLogger(enabled bool) *MetricsLogger {
	return &MetricsLogger{enabled: enabled}
}

// LogDisconnect logs disconnect metrics
func (m *MetricsLogger) LogDisconnect(ctx context.Context, metrics *DisconnectMetrics) {
	if !m.enabled {
		return
	}

	// Format as JSON for CloudWatch Insights
	data := map[string]interface{}{
		"event_type":              "connection_disconnected",
		"connection_id":           metrics.ConnectionID,
		"user_id":                 metrics.UserID,
		"tenant_id":               metrics.TenantID,
		"disconnect_reason":       metrics.DisconnectReason,
		"duration_seconds":        metrics.DurationSeconds,
		"messages_sent":           metrics.MessagesSent,
		"messages_received":       metrics.MessagesReceived,
		"subscriptions_cancelled": metrics.SubscriptionsCancelled,
		"requests_cancelled":      metrics.RequestsCancelled,
		"cleanup_duration_ms":     metrics.CleanupDurationMs,
		"connection_not_found":    metrics.ConnectionNotFound,
		"has_delete_error":        metrics.DeleteError != "",
		"has_subscription_error":  metrics.SubscriptionError != "",
		"has_request_error":       metrics.RequestError != "",
		"timestamp":               time.Now().UTC().Format(time.RFC3339),
	}

	// Log as structured JSON
	jsonData, _ := json.Marshal(data)
	log.Printf("METRICS: %s", string(jsonData))
}

// Helper function to parse int with default
func parseIntOrDefault(s string, defaultValue int) int {
	var value int
	if _, err := fmt.Sscanf(s, "%d", &value); err != nil {
		return defaultValue
	}
	return value
}
