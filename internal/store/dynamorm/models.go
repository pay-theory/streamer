package dynamorm

import (
	"fmt"
	"time"

	"github.com/pay-theory/streamer/internal/store"
)

// Connection represents a WebSocket connection - single model serving both business and database needs
type Connection struct {
	// DynamORM composite key pattern
	PK string `dynamorm:"PK"`
	SK string `dynamorm:"SK"`

	// Connection data with proper attribute mapping
	ConnectionID string    `dynamorm:"connection_id" json:"connectionId"`
	UserID       string    `dynamorm:"user_id" json:"userId" dynamorm-index:"user-index,pk"`
	TenantID     string    `dynamorm:"tenant_id" json:"tenantId" dynamorm-index:"tenant-index,pk"`
	Endpoint     string    `dynamorm:"endpoint" json:"endpoint"`
	ConnectedAt  time.Time `dynamorm:"connected_at" json:"connectedAt"`
	LastPing     time.Time `dynamorm:"last_ping" json:"lastPing"`

	// Metadata for storing additional information
	Metadata map[string]string `dynamorm:"metadata,omitempty" json:"metadata,omitempty"`

	// DynamORM managed fields
	CreatedAt time.Time `dynamorm:"created_at" json:"createdAt"`
	UpdatedAt time.Time `dynamorm:"updated_at" json:"updatedAt"`
	Version   int       `dynamorm:"version" json:"version"`

	// TTL for automatic cleanup
	TTL int64 `dynamorm:"ttl,omitempty" json:"ttl,omitempty"`
}

// TableName returns the DynamoDB table name
func (c *Connection) TableName() string {
	return "streamer_connections"
}

// SetKeys sets the composite keys for the connection
func (c *Connection) SetKeys() {
	c.PK = fmt.Sprintf("CONN#%s", c.ConnectionID)
	c.SK = "METADATA"
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

// AsyncRequest represents a queued async request - single model serving both business and database needs
type AsyncRequest struct {
	// DynamORM composite key pattern
	PK string `dynamorm:"PK"`
	SK string `dynamorm:"SK"`

	// Request data with proper attribute mapping
	RequestID    string                 `dynamorm:"request_id" json:"requestId"`
	ConnectionID string                 `dynamorm:"connection_id" json:"connectionId" dynamorm-index:"connection-index,pk"`
	Status       RequestStatus          `dynamorm:"status" json:"status" dynamorm-index:"status-index,sk"`
	Action       string                 `dynamorm:"action" json:"action"`
	Payload      map[string]interface{} `dynamorm:"payload,omitempty" json:"payload,omitempty"`

	// Processing information
	ProcessingStarted *time.Time `dynamorm:"processing_started,omitempty" json:"processingStarted,omitempty"`
	ProcessingEnded   *time.Time `dynamorm:"processing_ended,omitempty" json:"processingEnded,omitempty"`

	// Result or error
	Result map[string]interface{} `dynamorm:"result,omitempty" json:"result,omitempty"`
	Error  string                 `dynamorm:"error,omitempty" json:"error,omitempty"`

	// Progress tracking
	Progress        float64                `dynamorm:"progress" json:"progress"`
	ProgressMessage string                 `dynamorm:"progress_message,omitempty" json:"progressMessage,omitempty"`
	ProgressDetails map[string]interface{} `dynamorm:"progress_details,omitempty" json:"progressDetails,omitempty"`

	// Retry information
	RetryCount int       `dynamorm:"retry_count" json:"retryCount"`
	MaxRetries int       `dynamorm:"max_retries" json:"maxRetries"`
	RetryAfter time.Time `dynamorm:"retry_after,omitempty" json:"retryAfter,omitempty"`

	// User and tenant for querying
	UserID   string `dynamorm:"user_id" json:"userId" dynamorm-index:"user-index,sk"`
	TenantID string `dynamorm:"tenant_id" json:"tenantId" dynamorm-index:"tenant-index,sk"`

	// DynamORM managed fields
	CreatedAt time.Time `dynamorm:"created_at" json:"createdAt"`
	UpdatedAt time.Time `dynamorm:"updated_at" json:"updatedAt"`
	Version   int       `dynamorm:"version" json:"version"`

	// TTL for automatic cleanup
	TTL int64 `dynamorm:"ttl,omitempty" json:"ttl,omitempty"`
}

// TableName returns the DynamoDB table name
func (r *AsyncRequest) TableName() string {
	return "streamer_requests"
}

// SetKeys sets the composite keys for the request
func (r *AsyncRequest) SetKeys() {
	r.PK = fmt.Sprintf("REQ#%s", r.RequestID)
	r.SK = fmt.Sprintf("STATUS#%s", r.Status)
}

// Subscription represents a real-time update subscription - single model serving both business and database needs
type Subscription struct {
	// DynamORM composite key pattern
	PK string `dynamorm:"PK"`
	SK string `dynamorm:"SK"`

	// Subscription data with proper attribute mapping
	SubscriptionID string   `dynamorm:"subscription_id" json:"subscriptionId"`
	ConnectionID   string   `dynamorm:"connection_id" json:"connectionId" dynamorm-index:"connection-index,pk"`
	RequestID      string   `dynamorm:"request_id" json:"requestId" dynamorm-index:"request-index,pk"`
	EventTypes     []string `dynamorm:"event_types,stringset" json:"eventTypes"`

	// DynamORM managed fields
	CreatedAt time.Time `dynamorm:"created_at" json:"createdAt"`
	UpdatedAt time.Time `dynamorm:"updated_at" json:"updatedAt"`
	Version   int       `dynamorm:"version" json:"version"`

	// TTL for automatic cleanup
	TTL int64 `dynamorm:"ttl,omitempty" json:"ttl,omitempty"`
}

// TableName returns the DynamoDB table name
func (s *Subscription) TableName() string {
	return "streamer_subscriptions"
}

// SetKeys sets the composite keys for the subscription
func (s *Subscription) SetKeys() {
	s.PK = fmt.Sprintf("CONN#%s", s.ConnectionID)
	s.SK = fmt.Sprintf("SUB#%s", s.RequestID)
	s.SubscriptionID = fmt.Sprintf("%s#%s", s.ConnectionID, s.RequestID)
}

// ToStoreModel converts DynamORM Subscription to store.Subscription
func (s *Subscription) ToStoreModel() *store.Subscription {
	return &store.Subscription{
		SubscriptionID: s.SubscriptionID,
		ConnectionID:   s.ConnectionID,
		RequestID:      s.RequestID,
		EventTypes:     s.EventTypes,
		CreatedAt:      s.CreatedAt,
		TTL:            s.TTL,
	}
}

// FromStoreModel converts store.Subscription to DynamORM Subscription
func (s *Subscription) FromStoreModel(sub *store.Subscription) {
	s.SubscriptionID = sub.SubscriptionID
	s.ConnectionID = sub.ConnectionID
	s.RequestID = sub.RequestID
	s.EventTypes = sub.EventTypes
	s.CreatedAt = sub.CreatedAt
	s.TTL = sub.TTL
	s.SetKeys()
}

// Conversion methods between DynamORM models and store models

// ToStoreModel converts DynamORM Connection to store.Connection
func (c *Connection) ToStoreModel() *store.Connection {
	return &store.Connection{
		ConnectionID: c.ConnectionID,
		UserID:       c.UserID,
		TenantID:     c.TenantID,
		Endpoint:     c.Endpoint,
		ConnectedAt:  c.ConnectedAt,
		LastPing:     c.LastPing,
		Metadata:     c.Metadata,
		TTL:          c.TTL,
	}
}

// FromStoreModel converts store.Connection to DynamORM Connection
func (c *Connection) FromStoreModel(conn *store.Connection) {
	c.ConnectionID = conn.ConnectionID
	c.UserID = conn.UserID
	c.TenantID = conn.TenantID
	c.Endpoint = conn.Endpoint
	c.ConnectedAt = conn.ConnectedAt
	c.LastPing = conn.LastPing
	c.Metadata = conn.Metadata
	c.TTL = conn.TTL
	c.SetKeys()
}

// ToStoreModel converts DynamORM AsyncRequest to store.AsyncRequest
func (r *AsyncRequest) ToStoreModel() *store.AsyncRequest {
	return &store.AsyncRequest{
		RequestID:         r.RequestID,
		ConnectionID:      r.ConnectionID,
		Status:            store.RequestStatus(r.Status),
		Action:            r.Action,
		Payload:           r.Payload,
		ProcessingStarted: r.ProcessingStarted,
		ProcessingEnded:   r.ProcessingEnded,
		Result:            r.Result,
		Error:             r.Error,
		Progress:          r.Progress,
		ProgressMessage:   r.ProgressMessage,
		ProgressDetails:   r.ProgressDetails,
		RetryCount:        r.RetryCount,
		MaxRetries:        r.MaxRetries,
		RetryAfter:        r.RetryAfter,
		UserID:            r.UserID,
		TenantID:          r.TenantID,
		CreatedAt:         r.CreatedAt,
		TTL:               r.TTL,
	}
}

// FromStoreModel converts store.AsyncRequest to DynamORM AsyncRequest
func (r *AsyncRequest) FromStoreModel(req *store.AsyncRequest) {
	r.RequestID = req.RequestID
	r.ConnectionID = req.ConnectionID
	r.Status = RequestStatus(req.Status)
	r.Action = req.Action
	r.Payload = req.Payload
	r.ProcessingStarted = req.ProcessingStarted
	r.ProcessingEnded = req.ProcessingEnded
	r.Result = req.Result
	r.Error = req.Error
	r.Progress = req.Progress
	r.ProgressMessage = req.ProgressMessage
	r.ProgressDetails = req.ProgressDetails
	r.RetryCount = req.RetryCount
	r.MaxRetries = req.MaxRetries
	r.RetryAfter = req.RetryAfter
	r.UserID = req.UserID
	r.TenantID = req.TenantID
	r.CreatedAt = req.CreatedAt
	r.TTL = req.TTL
	r.SetKeys()
}
