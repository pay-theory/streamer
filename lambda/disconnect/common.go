package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// HandlerConfig holds configuration for the handler
type HandlerConfig struct {
	ConnectionsTable   string
	SubscriptionsTable string
	RequestsTable      string
	MetricsEnabled     bool
	LogLevel           string
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
	n, err := fmt.Sscanf(s, "%d", &value)
	if err != nil || n != 1 {
		return defaultValue
	}
	// Check if we consumed the entire string
	var remainder string
	fmt.Sscanf(s, "%d%s", &value, &remainder)
	if remainder != "" {
		return defaultValue
	}
	return value
}
