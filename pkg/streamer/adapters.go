package streamer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pay-theory/streamer/internal/store"
)

// RequestQueueAdapter adapts Team 1's RequestQueue to our RequestStore interface
type RequestQueueAdapter struct {
	queue store.RequestQueue
}

// NewRequestQueueAdapter creates a new adapter instance
func NewRequestQueueAdapter(queue store.RequestQueue) RequestStore {
	return &RequestQueueAdapter{queue: queue}
}

// Enqueue converts our Request to store.AsyncRequest and enqueues it
func (a *RequestQueueAdapter) Enqueue(ctx context.Context, request *Request) error {
	// Convert router.Request to store.AsyncRequest
	asyncReq := &store.AsyncRequest{
		RequestID:    request.ID,
		ConnectionID: request.ConnectionID,
		Action:       request.Action,
		Status:       store.StatusPending,
		Payload:      make(map[string]interface{}),
		CreatedAt:    request.CreatedAt,
		Progress:     0,
		RetryCount:   0,
		MaxRetries:   3,                                         // Default retry count
		TTL:          time.Now().Add(7 * 24 * time.Hour).Unix(), // 7 days TTL
	}

	// Convert payload from json.RawMessage to map[string]interface{}
	if len(request.Payload) > 0 {
		if err := json.Unmarshal(request.Payload, &asyncReq.Payload); err != nil {
			return fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	}

	// Add metadata to payload to preserve it
	if len(request.Metadata) > 0 {
		asyncReq.Payload["_metadata"] = request.Metadata
	}

	// Extract UserID and TenantID from request metadata if available
	if userID, ok := request.Metadata["user_id"]; ok {
		asyncReq.UserID = userID
	}
	if tenantID, ok := request.Metadata["tenant_id"]; ok {
		asyncReq.TenantID = tenantID
	}

	// Map error if enqueue fails
	if err := a.queue.Enqueue(ctx, asyncReq); err != nil {
		return mapStoreError(err)
	}

	return nil
}

// ConvertAsyncRequestToRequest converts a store.AsyncRequest back to a Request
func ConvertAsyncRequestToRequest(asyncReq *store.AsyncRequest) (*Request, error) {
	request := &Request{
		ID:           asyncReq.RequestID,
		ConnectionID: asyncReq.ConnectionID,
		Action:       asyncReq.Action,
		CreatedAt:    asyncReq.CreatedAt,
		Metadata:     make(map[string]string),
	}

	// Extract metadata from payload if it exists
	if metadata, ok := asyncReq.Payload["_metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				request.Metadata[k] = str
			}
		}
		// Remove _metadata from payload before marshaling
		delete(asyncReq.Payload, "_metadata")
	}

	// Add user and tenant IDs to metadata if they exist
	if asyncReq.UserID != "" {
		request.Metadata["user_id"] = asyncReq.UserID
	}
	if asyncReq.TenantID != "" {
		request.Metadata["tenant_id"] = asyncReq.TenantID
	}

	// Convert payload back to json.RawMessage
	if len(asyncReq.Payload) > 0 {
		payloadBytes, err := json.Marshal(asyncReq.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		request.Payload = payloadBytes
	}

	return request, nil
}

// mapStoreError maps storage errors to streamer errors
func mapStoreError(err error) error {
	if err == nil {
		return nil
	}

	// Check for specific store errors
	if store.IsNotFound(err) {
		return NewError(ErrCodeNotFound, "Request not found")
	}

	if store.IsAlreadyExists(err) {
		return NewError(ErrCodeValidation, "Request already exists")
	}

	// Check for validation errors
	var validationErr *store.ValidationError
	if errors.As(err, &validationErr) {
		return NewError(ErrCodeValidation, validationErr.Error())
	}

	// Default to internal error
	return NewError(ErrCodeInternalError, fmt.Sprintf("Storage error: %v", err))
}

// GetAsyncRequest retrieves an async request and converts it to Request
func (a *RequestQueueAdapter) GetAsyncRequest(ctx context.Context, requestID string) (*Request, error) {
	asyncReq, err := a.queue.Get(ctx, requestID)
	if err != nil {
		return nil, mapStoreError(err)
	}

	return ConvertAsyncRequestToRequest(asyncReq)
}

// UpdateProgress updates the progress of an async request
func (a *RequestQueueAdapter) UpdateProgress(ctx context.Context, requestID string, progress float64, message string) error {
	details := map[string]interface{}{
		"timestamp": time.Now().Unix(),
	}

	if err := a.queue.UpdateProgress(ctx, requestID, progress, message, details); err != nil {
		return mapStoreError(err)
	}

	return nil
}

// CompleteRequest marks a request as completed
func (a *RequestQueueAdapter) CompleteRequest(ctx context.Context, requestID string, result *Result) error {
	resultMap := map[string]interface{}{
		"success": result.Success,
		"data":    result.Data,
	}

	if result.Error != nil {
		resultMap["error"] = map[string]interface{}{
			"code":    result.Error.Code,
			"message": result.Error.Message,
			"details": result.Error.Details,
		}
	}

	if len(result.Metadata) > 0 {
		resultMap["metadata"] = result.Metadata
	}

	if err := a.queue.CompleteRequest(ctx, requestID, resultMap); err != nil {
		return mapStoreError(err)
	}

	return nil
}

// FailRequest marks a request as failed
func (a *RequestQueueAdapter) FailRequest(ctx context.Context, requestID string, err error) error {
	var errMsg string
	if streamerErr, ok := err.(*Error); ok {
		errMsg = fmt.Sprintf("[%s] %s", streamerErr.Code, streamerErr.Message)
	} else {
		errMsg = err.Error()
	}

	if err := a.queue.FailRequest(ctx, requestID, errMsg); err != nil {
		return mapStoreError(err)
	}

	return nil
}
