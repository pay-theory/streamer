package dynamorm

import (
	"context"
	"fmt"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
	"github.com/pay-theory/streamer/internal/store"
)

// requestQueue implements RequestQueue using DynamORM
type requestQueue struct {
	db core.DB
}

// NewRequestQueue creates a new DynamORM-backed request queue
func NewRequestQueue(db core.DB) store.RequestQueue {
	return &requestQueue{
		db: db,
	}
}

// Enqueue adds a new request to the queue
func (q *requestQueue) Enqueue(ctx context.Context, req *store.AsyncRequest) error {
	if err := q.validateRequest(req); err != nil {
		return err
	}

	// Set default values
	if req.Status == "" {
		req.Status = store.StatusPending
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	if req.TTL == 0 {
		req.TTL = time.Now().Add(7 * 24 * time.Hour).Unix() // 7 days TTL
	}

	// Convert to DynamORM model
	dynamormReq := &AsyncRequest{}
	dynamormReq.FromStoreModel(req)

	// Create the request
	if err := q.db.Model(dynamormReq).Create(); err != nil {
		return store.NewStoreError("Enqueue", dynamormReq.TableName(), req.RequestID, fmt.Errorf("failed to enqueue request: %w", err))
	}

	return nil
}

// Get retrieves a specific request
func (q *requestQueue) Get(ctx context.Context, requestID string) (*store.AsyncRequest, error) {
	if requestID == "" {
		return nil, store.NewValidationError("requestID", "cannot be empty")
	}

	// Create model with keys
	req := &AsyncRequest{RequestID: requestID}
	req.SetKeys()

	// Query for the request (need to handle multiple statuses)
	var requests []AsyncRequest
	if err := q.db.Model(&AsyncRequest{}).
		Where("pk", "=", req.PK).
		All(&requests); err != nil {
		return nil, store.NewStoreError("Get", req.TableName(), requestID, fmt.Errorf("failed to get request: %w", err))
	}

	if len(requests) == 0 {
		return nil, store.NewStoreError("Get", req.TableName(), requestID, store.ErrNotFound)
	}

	// Return the first (should be only) result
	return requests[0].ToStoreModel(), nil
}

// UpdateStatus updates the status of a request
func (q *requestQueue) UpdateStatus(ctx context.Context, requestID string, status store.RequestStatus, message string) error {
	if requestID == "" {
		return store.NewValidationError("requestID", "cannot be empty")
	}

	// Get the current request to find its current status
	current, err := q.Get(ctx, requestID)
	if err != nil {
		return err
	}

	// Delete old status entry
	oldReq := &AsyncRequest{RequestID: requestID, Status: current.Status}
	oldReq.SetKeys()
	if err := q.db.Model(oldReq).Delete(); err != nil {
		// Log but don't fail
	}

	// Create new status entry
	newReq := &AsyncRequest{}
	newReq.FromStoreModel(current)
	newReq.Status = status
	newReq.SetKeys()

	if err := q.db.Model(newReq).Create(); err != nil {
		return store.NewStoreError("UpdateStatus", newReq.TableName(), requestID, fmt.Errorf("failed to update status: %w", err))
	}

	return nil
}

// UpdateProgress updates the progress of a request
func (q *requestQueue) UpdateProgress(ctx context.Context, requestID string, progress float64, message string, details map[string]interface{}) error {
	if requestID == "" {
		return store.NewValidationError("requestID", "cannot be empty")
	}

	// Get current request
	current, err := q.Get(ctx, requestID)
	if err != nil {
		return err
	}

	req := &AsyncRequest{RequestID: requestID, Status: current.Status}
	req.SetKeys()

	// Update progress fields
	err = q.db.Model(req).
		UpdateBuilder().
		Set("progress", progress).
		Set("progress_message", message).
		Set("progress_details", details).
		Execute()

	if err != nil {
		return store.NewStoreError("UpdateProgress", req.TableName(), requestID, fmt.Errorf("failed to update progress: %w", err))
	}

	return nil
}

// CompleteRequest marks a request as completed with results
func (q *requestQueue) CompleteRequest(ctx context.Context, requestID string, result map[string]interface{}) error {
	now := time.Now()

	// Get current request
	current, err := q.Get(ctx, requestID)
	if err != nil {
		return err
	}

	// Update status to completed
	current.Status = store.StatusCompleted
	current.ProcessingEnded = &now
	current.Result = result
	current.Progress = 100

	// Delete old entry and create new one with updated status
	return q.UpdateStatus(ctx, requestID, store.StatusCompleted, "Request completed successfully")
}

// FailRequest marks a request as failed with an error
func (q *requestQueue) FailRequest(ctx context.Context, requestID string, errMsg string) error {
	now := time.Now()

	// Get current request
	current, err := q.Get(ctx, requestID)
	if err != nil {
		return err
	}

	// Update status to failed
	current.Status = store.StatusFailed
	current.ProcessingEnded = &now
	current.Error = errMsg

	// Delete old entry and create new one with updated status
	return q.UpdateStatus(ctx, requestID, store.StatusFailed, errMsg)
}

// GetByConnection retrieves all requests for a connection
func (q *requestQueue) GetByConnection(ctx context.Context, connectionID string, limit int) ([]*store.AsyncRequest, error) {
	if connectionID == "" {
		return nil, store.NewValidationError("connectionID", "cannot be empty")
	}

	var requests []AsyncRequest

	// Query using the connection index with v1.0.9 API
	query := q.db.Model(&AsyncRequest{}).
		Index("connection-index").
		Where("connection_id", "=", connectionID)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.All(&requests); err != nil {
		return nil, store.NewStoreError("GetByConnection", store.RequestsTable, connectionID, fmt.Errorf("failed to get requests by connection: %w", err))
	}

	// Convert to store models
	result := make([]*store.AsyncRequest, len(requests))
	for i := range requests {
		result[i] = requests[i].ToStoreModel()
	}

	return result, nil
}

// GetByStatus retrieves requests by status
func (q *requestQueue) GetByStatus(ctx context.Context, status store.RequestStatus, limit int) ([]*store.AsyncRequest, error) {
	var requests []AsyncRequest

	// Query using the status index with v1.0.9 API
	query := q.db.Model(&AsyncRequest{}).
		Index("status-index").
		Where("status", "=", status)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.All(&requests); err != nil {
		return nil, store.NewStoreError("GetByStatus", store.RequestsTable, string(status), fmt.Errorf("failed to get requests by status: %w", err))
	}

	// Convert to store models
	result := make([]*store.AsyncRequest, len(requests))
	for i := range requests {
		result[i] = requests[i].ToStoreModel()
	}

	return result, nil
}

// Dequeue retrieves and marks requests for processing
func (q *requestQueue) Dequeue(ctx context.Context, limit int) ([]*store.AsyncRequest, error) {
	// Get pending requests
	requests, err := q.GetByStatus(ctx, store.StatusPending, limit)
	if err != nil {
		return nil, err
	}

	// Mark each as processing
	for _, req := range requests {
		if err := q.UpdateStatus(ctx, req.RequestID, store.StatusProcessing, "Dequeued for processing"); err != nil {
			// Log error but continue
		}
	}

	return requests, nil
}

// Delete removes a request
func (q *requestQueue) Delete(ctx context.Context, requestID string) error {
	if requestID == "" {
		return store.NewValidationError("requestID", "cannot be empty")
	}

	// Get the request to find its status
	current, err := q.Get(ctx, requestID)
	if err != nil {
		return err
	}

	// Create model with keys
	req := &AsyncRequest{RequestID: requestID, Status: current.Status}
	req.SetKeys()

	// Delete the request
	if err := q.db.Model(req).Delete(); err != nil {
		return store.NewStoreError("Delete", req.TableName(), requestID, fmt.Errorf("failed to delete request: %w", err))
	}

	return nil
}

// validateRequest validates a request before saving
func (q *requestQueue) validateRequest(req *store.AsyncRequest) error {
	if req == nil {
		return store.NewValidationError("request", "cannot be nil")
	}
	if req.RequestID == "" {
		return store.NewValidationError("RequestID", "cannot be empty")
	}
	if req.ConnectionID == "" {
		return store.NewValidationError("ConnectionID", "cannot be empty")
	}
	if req.Action == "" {
		return store.NewValidationError("Action", "cannot be empty")
	}
	if req.UserID == "" {
		return store.NewValidationError("UserID", "cannot be empty")
	}
	if req.TenantID == "" {
		return store.NewValidationError("TenantID", "cannot be empty")
	}
	return nil
}
