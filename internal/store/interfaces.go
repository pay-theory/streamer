package store

import (
	"context"
	"time"
)

// ConnectionStore manages WebSocket connections in DynamoDB
type ConnectionStore interface {
	// Save creates or updates a connection
	Save(ctx context.Context, conn *Connection) error

	// Get retrieves a connection by ID
	Get(ctx context.Context, connectionID string) (*Connection, error)

	// Delete removes a connection
	Delete(ctx context.Context, connectionID string) error

	// ListByUser returns all connections for a user
	ListByUser(ctx context.Context, userID string) ([]*Connection, error)

	// ListByTenant returns all connections for a tenant
	ListByTenant(ctx context.Context, tenantID string) ([]*Connection, error)

	// UpdateLastPing updates the last ping timestamp
	UpdateLastPing(ctx context.Context, connectionID string) error

	// DeleteStale removes connections older than the specified time
	DeleteStale(ctx context.Context, before time.Time) error
}

// RequestQueue manages async requests in DynamoDB
type RequestQueue interface {
	// Enqueue adds a new request to the queue
	Enqueue(ctx context.Context, req *AsyncRequest) error

	// Dequeue retrieves and marks requests for processing
	// This is mainly for testing - in production, DynamoDB Streams handle this
	Dequeue(ctx context.Context, limit int) ([]*AsyncRequest, error)

	// UpdateStatus updates the status of a request
	UpdateStatus(ctx context.Context, requestID string, status RequestStatus, message string) error

	// UpdateProgress updates the progress of a request
	UpdateProgress(ctx context.Context, requestID string, progress float64, message string, details map[string]interface{}) error

	// CompleteRequest marks a request as completed with results
	CompleteRequest(ctx context.Context, requestID string, result map[string]interface{}) error

	// FailRequest marks a request as failed with an error
	FailRequest(ctx context.Context, requestID string, errMsg string) error

	// GetByConnection retrieves all requests for a connection
	GetByConnection(ctx context.Context, connectionID string, limit int) ([]*AsyncRequest, error)

	// GetByStatus retrieves requests by status
	GetByStatus(ctx context.Context, status RequestStatus, limit int) ([]*AsyncRequest, error)

	// Get retrieves a specific request
	Get(ctx context.Context, requestID string) (*AsyncRequest, error)

	// Delete removes a request
	Delete(ctx context.Context, requestID string) error
}

// SubscriptionStore manages real-time update subscriptions
type SubscriptionStore interface {
	// Subscribe creates a subscription for progress updates
	Subscribe(ctx context.Context, sub *Subscription) error

	// Unsubscribe removes a subscription
	Unsubscribe(ctx context.Context, connectionID, requestID string) error

	// GetByConnection returns all subscriptions for a connection
	GetByConnection(ctx context.Context, connectionID string) ([]*Subscription, error)

	// GetByRequest returns all subscriptions for a request
	GetByRequest(ctx context.Context, requestID string) ([]*Subscription, error)

	// DeleteByConnection removes all subscriptions for a connection
	DeleteByConnection(ctx context.Context, connectionID string) error
}
