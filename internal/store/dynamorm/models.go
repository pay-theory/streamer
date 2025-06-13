package dynamorm

import (
	"fmt"
	"time"

	"github.com/pay-theory/streamer/internal/store"
)

// Connection represents a WebSocket connection with DynamORM
type Connection struct {
	// DynamORM composite key pattern
	PK string `dynamorm:"pk"`
	SK string `dynamorm:"sk"`

	// Connection data
	ConnectionID string    `dynamorm:"connection_id"`
	UserID       string    `dynamorm:"user_id" dynamorm-index:"user-index,pk"`
	TenantID     string    `dynamorm:"tenant_id" dynamorm-index:"tenant-index,pk"`
	Endpoint     string    `dynamorm:"endpoint"`
	ConnectedAt  time.Time `dynamorm:"connected_at"`
	LastPing     time.Time `dynamorm:"last_ping"`

	// Metadata for storing additional information
	Metadata map[string]string `dynamorm:"metadata,omitempty"`

	// TTL for automatic cleanup
	TTL int64 `dynamorm:"ttl,omitempty"`
}

// TableName returns the DynamoDB table name
func (c *Connection) TableName() string {
	return store.ConnectionsTable
}

// SetKeys sets the composite keys for the connection
func (c *Connection) SetKeys() {
	c.PK = fmt.Sprintf("CONN#%s", c.ConnectionID)
	c.SK = "METADATA"
}

// ToStoreModel converts to the store.Connection model
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

// FromStoreModel converts from the store.Connection model
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

// AsyncRequest represents a queued async request with DynamORM
type AsyncRequest struct {
	// DynamORM composite key pattern
	PK string `dynamorm:"pk"`
	SK string `dynamorm:"sk"`

	// Request data
	RequestID    string                 `dynamorm:"request_id"`
	ConnectionID string                 `dynamorm:"connection_id" dynamorm-index:"connection-index,pk"`
	Status       store.RequestStatus    `dynamorm:"status" dynamorm-index:"status-index,sk"`
	CreatedAt    time.Time              `dynamorm:"created_at"`
	Action       string                 `dynamorm:"action"`
	Payload      map[string]interface{} `dynamorm:"payload,omitempty"`

	// Processing information
	ProcessingStarted *time.Time `dynamorm:"processing_started,omitempty"`
	ProcessingEnded   *time.Time `dynamorm:"processing_ended,omitempty"`

	// Result or error
	Result map[string]interface{} `dynamorm:"result,omitempty"`
	Error  string                 `dynamorm:"error,omitempty"`

	// Progress tracking
	Progress        float64                `dynamorm:"progress"`
	ProgressMessage string                 `dynamorm:"progress_message,omitempty"`
	ProgressDetails map[string]interface{} `dynamorm:"progress_details,omitempty"`

	// Retry information
	RetryCount int       `dynamorm:"retry_count"`
	MaxRetries int       `dynamorm:"max_retries"`
	RetryAfter time.Time `dynamorm:"retry_after,omitempty"`

	// User and tenant for querying
	UserID   string `dynamorm:"user_id" dynamorm-index:"user-index,sk"`
	TenantID string `dynamorm:"tenant_id" dynamorm-index:"tenant-index,sk"`

	// TTL for automatic cleanup
	TTL int64 `dynamorm:"ttl,omitempty"`
}

// TableName returns the DynamoDB table name
func (r *AsyncRequest) TableName() string {
	return store.RequestsTable
}

// SetKeys sets the composite keys for the request
func (r *AsyncRequest) SetKeys() {
	r.PK = fmt.Sprintf("REQ#%s", r.RequestID)
	r.SK = fmt.Sprintf("STATUS#%s", r.Status)
}

// ToStoreModel converts to the store.AsyncRequest model
func (r *AsyncRequest) ToStoreModel() *store.AsyncRequest {
	return &store.AsyncRequest{
		RequestID:         r.RequestID,
		ConnectionID:      r.ConnectionID,
		Status:            r.Status,
		CreatedAt:         r.CreatedAt,
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
		TTL:               r.TTL,
	}
}

// FromStoreModel converts from the store.AsyncRequest model
func (r *AsyncRequest) FromStoreModel(req *store.AsyncRequest) {
	r.RequestID = req.RequestID
	r.ConnectionID = req.ConnectionID
	r.Status = req.Status
	r.CreatedAt = req.CreatedAt
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
	r.TTL = req.TTL
	r.SetKeys()
}

// Subscription represents a real-time update subscription with DynamORM
type Subscription struct {
	// DynamORM composite key pattern
	PK string `dynamorm:"pk"`
	SK string `dynamorm:"sk"`

	// Subscription data
	SubscriptionID string    `dynamorm:"subscription_id"`
	ConnectionID   string    `dynamorm:"connection_id" dynamorm-index:"connection-index,pk"`
	RequestID      string    `dynamorm:"request_id" dynamorm-index:"request-index,pk"`
	EventTypes     []string  `dynamorm:"event_types,stringset"`
	CreatedAt      time.Time `dynamorm:"created_at"`

	// TTL for automatic cleanup
	TTL int64 `dynamorm:"ttl,omitempty"`
}

// TableName returns the DynamoDB table name
func (s *Subscription) TableName() string {
	return store.SubscriptionsTable
}

// SetKeys sets the composite keys for the subscription
func (s *Subscription) SetKeys() {
	s.PK = fmt.Sprintf("CONN#%s", s.ConnectionID)
	s.SK = fmt.Sprintf("SUB#%s", s.RequestID)
	s.SubscriptionID = fmt.Sprintf("%s#%s", s.ConnectionID, s.RequestID)
}

// ToStoreModel converts to the store.Subscription model
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

// FromStoreModel converts from the store.Subscription model
func (s *Subscription) FromStoreModel(sub *store.Subscription) {
	s.SubscriptionID = sub.SubscriptionID
	s.ConnectionID = sub.ConnectionID
	s.RequestID = sub.RequestID
	s.EventTypes = sub.EventTypes
	s.CreatedAt = sub.CreatedAt
	s.TTL = sub.TTL
	s.SetKeys()
}
