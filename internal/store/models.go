package store

import (
	"time"
)

// Connection represents a WebSocket connection
type Connection struct {
	// Primary key
	ConnectionID string `dynamodbav:"ConnectionID" json:"connectionId"`

	// User and tenant information for multi-tenancy
	UserID   string `dynamodbav:"UserID" json:"userId"`
	TenantID string `dynamodbav:"TenantID" json:"tenantId"`

	// WebSocket endpoint for sending messages
	Endpoint string `dynamodbav:"Endpoint" json:"endpoint"`

	// Timestamps
	ConnectedAt time.Time `dynamodbav:"ConnectedAt" json:"connectedAt"`
	LastPing    time.Time `dynamodbav:"LastPing" json:"lastPing"`

	// Metadata for storing additional information
	Metadata map[string]string `dynamodbav:"Metadata,omitempty" json:"metadata,omitempty"`

	// TTL for automatic cleanup
	TTL int64 `dynamodbav:"TTL,omitempty" json:"ttl,omitempty"`
}

// AsyncRequest represents a queued async request
type AsyncRequest struct {
	// Primary key
	RequestID string `dynamodbav:"RequestID" json:"requestId"`

	// Connection that created this request
	ConnectionID string `dynamodbav:"ConnectionID" json:"connectionId"`

	// Status tracking
	Status    RequestStatus `dynamodbav:"Status" json:"status"`
	CreatedAt time.Time     `dynamodbav:"CreatedAt" json:"createdAt"`

	// Request details
	Action  string                 `dynamodbav:"Action" json:"action"`
	Payload map[string]interface{} `dynamodbav:"Payload,omitempty" json:"payload,omitempty"`

	// Processing information
	ProcessingStarted *time.Time `dynamodbav:"ProcessingStarted,omitempty" json:"processingStarted,omitempty"`
	ProcessingEnded   *time.Time `dynamodbav:"ProcessingEnded,omitempty" json:"processingEnded,omitempty"`

	// Result or error
	Result map[string]interface{} `dynamodbav:"Result,omitempty" json:"result,omitempty"`
	Error  string                 `dynamodbav:"Error,omitempty" json:"error,omitempty"`

	// Progress tracking
	Progress        float64                `dynamodbav:"Progress" json:"progress"`
	ProgressMessage string                 `dynamodbav:"ProgressMessage,omitempty" json:"progressMessage,omitempty"`
	ProgressDetails map[string]interface{} `dynamodbav:"ProgressDetails,omitempty" json:"progressDetails,omitempty"`

	// Retry information
	RetryCount int       `dynamodbav:"RetryCount" json:"retryCount"`
	MaxRetries int       `dynamodbav:"MaxRetries" json:"maxRetries"`
	RetryAfter time.Time `dynamodbav:"RetryAfter,omitempty" json:"retryAfter,omitempty"`

	// User and tenant for querying
	UserID   string `dynamodbav:"UserID" json:"userId"`
	TenantID string `dynamodbav:"TenantID" json:"tenantId"`

	// TTL for automatic cleanup
	TTL int64 `dynamodbav:"TTL,omitempty" json:"ttl,omitempty"`
}

// RequestStatus represents the status of an async request
type RequestStatus string

const (
	StatusPending    RequestStatus = "PENDING"
	StatusProcessing RequestStatus = "PROCESSING"
	StatusCompleted  RequestStatus = "COMPLETED"
	StatusFailed     RequestStatus = "FAILED"
	StatusCancelled  RequestStatus = "CANCELLED"
	StatusRetrying   RequestStatus = "RETRYING"
)

// Subscription represents a real-time update subscription
type Subscription struct {
	// Composite key: ConnectionID#RequestID
	SubscriptionID string `dynamodbav:"SubscriptionID" json:"subscriptionId"`

	// Individual components for querying
	ConnectionID string `dynamodbav:"ConnectionID" json:"connectionId"`
	RequestID    string `dynamodbav:"RequestID" json:"requestId"`

	// Subscription details
	EventTypes []string  `dynamodbav:"EventTypes,stringset" json:"eventTypes"`
	CreatedAt  time.Time `dynamodbav:"CreatedAt" json:"createdAt"`

	// TTL for automatic cleanup
	TTL int64 `dynamodbav:"TTL,omitempty" json:"ttl,omitempty"`
}

// TableNames defines the DynamoDB table names
const (
	ConnectionsTable   = "streamer_connections"
	RequestsTable      = "streamer_requests"
	SubscriptionsTable = "streamer_subscriptions"
)
