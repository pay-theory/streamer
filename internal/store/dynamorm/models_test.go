package dynamorm_test

import (
	"testing"
	"time"

	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/stretchr/testify/assert"
)

// TestConnection_TableName tests the TableName method
func TestConnection_TableName(t *testing.T) {
	conn := &dynamorm.Connection{}
	assert.Equal(t, store.ConnectionsTable, conn.TableName())
}

// TestConnection_SetKeys tests the SetKeys method
func TestConnection_SetKeys(t *testing.T) {
	tests := []struct {
		name         string
		connectionID string
		expectedPK   string
		expectedSK   string
	}{
		{
			name:         "valid connection ID",
			connectionID: "conn123",
			expectedPK:   "CONN#conn123",
			expectedSK:   "METADATA",
		},
		{
			name:         "empty connection ID",
			connectionID: "",
			expectedPK:   "CONN#",
			expectedSK:   "METADATA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &dynamorm.Connection{
				ConnectionID: tt.connectionID,
			}
			conn.SetKeys()
			assert.Equal(t, tt.expectedPK, conn.PK)
			assert.Equal(t, tt.expectedSK, conn.SK)
		})
	}
}

// TestConnection_ToStoreModel tests the ToStoreModel method
func TestConnection_ToStoreModel(t *testing.T) {
	now := time.Now()
	conn := &dynamorm.Connection{
		PK:           "CONN#conn123",
		SK:           "CONN#conn123",
		ConnectionID: "conn123",
		UserID:       "user123",
		TenantID:     "tenant123",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  now,
		LastPing:     now.Add(5 * time.Minute),
		Metadata:     map[string]string{"client": "web"},
		TTL:          now.Add(24 * time.Hour).Unix(),
	}

	storeModel := conn.ToStoreModel()

	assert.Equal(t, "conn123", storeModel.ConnectionID)
	assert.Equal(t, "user123", storeModel.UserID)
	assert.Equal(t, "tenant123", storeModel.TenantID)
	assert.Equal(t, "wss://example.com/ws", storeModel.Endpoint)
	assert.Equal(t, now, storeModel.ConnectedAt)
	assert.Equal(t, now.Add(5*time.Minute), storeModel.LastPing)
	assert.Equal(t, map[string]string{"client": "web"}, storeModel.Metadata)
	assert.Equal(t, now.Add(24*time.Hour).Unix(), storeModel.TTL)
}

// TestConnection_FromStoreModel tests the FromStoreModel method
func TestConnection_FromStoreModel(t *testing.T) {
	now := time.Now()
	storeConn := &store.Connection{
		ConnectionID: "conn123",
		UserID:       "user123",
		TenantID:     "tenant123",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  now,
		LastPing:     now.Add(5 * time.Minute),
		Metadata:     map[string]string{"client": "web", "version": "1.0"},
		TTL:          now.Add(24 * time.Hour).Unix(),
	}

	conn := &dynamorm.Connection{}
	conn.FromStoreModel(storeConn)

	assert.Equal(t, "CONN#conn123", conn.PK)
	assert.Equal(t, "METADATA", conn.SK)
	assert.Equal(t, "conn123", conn.ConnectionID)
	assert.Equal(t, "user123", conn.UserID)
	assert.Equal(t, "tenant123", conn.TenantID)
	assert.Equal(t, "wss://example.com/ws", conn.Endpoint)
	assert.Equal(t, now, conn.ConnectedAt)
	assert.Equal(t, now.Add(5*time.Minute), conn.LastPing)
	assert.Equal(t, map[string]string{"client": "web", "version": "1.0"}, conn.Metadata)
	assert.Equal(t, now.Add(24*time.Hour).Unix(), conn.TTL)
}

// TestAsyncRequest_TableName tests the TableName method
func TestAsyncRequest_TableName(t *testing.T) {
	req := &dynamorm.AsyncRequest{}
	assert.Equal(t, store.RequestsTable, req.TableName())
}

// TestAsyncRequest_SetKeys tests the SetKeys method
func TestAsyncRequest_SetKeys(t *testing.T) {
	tests := []struct {
		name       string
		requestID  string
		status     store.RequestStatus
		expectedPK string
		expectedSK string
	}{
		{
			name:       "pending request",
			requestID:  "req123",
			status:     store.StatusPending,
			expectedPK: "REQ#req123",
			expectedSK: "STATUS#PENDING",
		},
		{
			name:       "processing request",
			requestID:  "req456",
			status:     store.StatusProcessing,
			expectedPK: "REQ#req456",
			expectedSK: "STATUS#PROCESSING",
		},
		{
			name:       "completed request",
			requestID:  "req789",
			status:     store.StatusCompleted,
			expectedPK: "REQ#req789",
			expectedSK: "STATUS#COMPLETED",
		},
		{
			name:       "failed request",
			requestID:  "req000",
			status:     store.StatusFailed,
			expectedPK: "REQ#req000",
			expectedSK: "STATUS#FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &dynamorm.AsyncRequest{
				RequestID: tt.requestID,
				Status:    tt.status,
			}
			req.SetKeys()
			assert.Equal(t, tt.expectedPK, req.PK)
			assert.Equal(t, tt.expectedSK, req.SK)
		})
	}
}

// TestAsyncRequest_ToStoreModel tests the ToStoreModel method
func TestAsyncRequest_ToStoreModel(t *testing.T) {
	now := time.Now()
	processingStarted := now.Add(5 * time.Minute)
	processingEnded := now.Add(10 * time.Minute)

	req := &dynamorm.AsyncRequest{
		PK:                "REQ#req123",
		SK:                "STATUS#PROCESSING",
		RequestID:         "req123",
		ConnectionID:      "conn123",
		Status:            store.StatusProcessing,
		CreatedAt:         now,
		ProcessingStarted: &processingStarted,
		ProcessingEnded:   &processingEnded,
		Action:            "process_data",
		Payload:           map[string]interface{}{"data": "test"},
		Result:            map[string]interface{}{"output": "success"},
		Error:             "",
		Progress:          50.0,
		ProgressMessage:   "Halfway done",
		ProgressDetails:   map[string]interface{}{"step": 5},
		UserID:            "user123",
		TenantID:          "tenant123",
		MaxRetries:        3,
		RetryCount:        1,
		TTL:               now.Add(7 * 24 * time.Hour).Unix(),
	}

	storeModel := req.ToStoreModel()

	assert.Equal(t, "req123", storeModel.RequestID)
	assert.Equal(t, "conn123", storeModel.ConnectionID)
	assert.Equal(t, store.StatusProcessing, storeModel.Status)
	assert.Equal(t, now, storeModel.CreatedAt)
	assert.Equal(t, &processingStarted, storeModel.ProcessingStarted)
	assert.Equal(t, &processingEnded, storeModel.ProcessingEnded)
	assert.Equal(t, "process_data", storeModel.Action)
	assert.Equal(t, map[string]interface{}{"data": "test"}, storeModel.Payload)
	assert.Equal(t, map[string]interface{}{"output": "success"}, storeModel.Result)
	assert.Equal(t, "", storeModel.Error)
	assert.Equal(t, 50.0, storeModel.Progress)
	assert.Equal(t, "Halfway done", storeModel.ProgressMessage)
	assert.Equal(t, map[string]interface{}{"step": 5}, storeModel.ProgressDetails)
	assert.Equal(t, "user123", storeModel.UserID)
	assert.Equal(t, "tenant123", storeModel.TenantID)
	assert.Equal(t, 3, storeModel.MaxRetries)
	assert.Equal(t, 1, storeModel.RetryCount)
	assert.Equal(t, now.Add(7*24*time.Hour).Unix(), storeModel.TTL)
}

// TestAsyncRequest_FromStoreModel tests the FromStoreModel method
func TestAsyncRequest_FromStoreModel(t *testing.T) {
	now := time.Now()
	processingStarted := now.Add(5 * time.Minute)

	storeReq := &store.AsyncRequest{
		RequestID:         "req123",
		ConnectionID:      "conn123",
		Status:            store.StatusProcessing,
		CreatedAt:         now,
		ProcessingStarted: &processingStarted,
		ProcessingEnded:   nil,
		Action:            "process_data",
		Payload:           map[string]interface{}{"data": "test", "count": 100},
		Result:            nil,
		Error:             "",
		Progress:          25.0,
		ProgressMessage:   "Processing...",
		ProgressDetails:   map[string]interface{}{"step": 2, "total": 8},
		UserID:            "user123",
		TenantID:          "tenant123",
		MaxRetries:        5,
		RetryCount:        0,
		TTL:               now.Add(7 * 24 * time.Hour).Unix(),
	}

	req := &dynamorm.AsyncRequest{}
	req.FromStoreModel(storeReq)

	assert.Equal(t, "REQ#req123", req.PK)
	assert.Equal(t, "STATUS#PROCESSING", req.SK)
	assert.Equal(t, "req123", req.RequestID)
	assert.Equal(t, "conn123", req.ConnectionID)
	assert.Equal(t, store.StatusProcessing, req.Status)
	assert.Equal(t, now, req.CreatedAt)
	assert.Equal(t, &processingStarted, req.ProcessingStarted)
	assert.Nil(t, req.ProcessingEnded)
	assert.Equal(t, "process_data", req.Action)
	assert.Equal(t, map[string]interface{}{"data": "test", "count": 100}, req.Payload)
	assert.Nil(t, req.Result)
	assert.Equal(t, "", req.Error)
	assert.Equal(t, 25.0, req.Progress)
	assert.Equal(t, "Processing...", req.ProgressMessage)
	assert.Equal(t, map[string]interface{}{"step": 2, "total": 8}, req.ProgressDetails)
	assert.Equal(t, "user123", req.UserID)
	assert.Equal(t, "tenant123", req.TenantID)
	assert.Equal(t, 5, req.MaxRetries)
	assert.Equal(t, 0, req.RetryCount)
	assert.Equal(t, now.Add(7*24*time.Hour).Unix(), req.TTL)
}

// TestSubscription_TableName tests the TableName method
func TestSubscription_TableName(t *testing.T) {
	sub := &dynamorm.Subscription{}
	assert.Equal(t, store.SubscriptionsTable, sub.TableName())
}

// TestSubscription_SetKeys tests the SetKeys method
func TestSubscription_SetKeys(t *testing.T) {
	tests := []struct {
		name          string
		connectionID  string
		requestID     string
		expectedPK    string
		expectedSK    string
		expectedSubID string
	}{
		{
			name:          "valid subscription",
			connectionID:  "conn123",
			requestID:     "req123",
			expectedPK:    "CONN#conn123",
			expectedSK:    "SUB#req123",
			expectedSubID: "conn123#req123",
		},
		{
			name:          "empty request ID",
			connectionID:  "conn456",
			requestID:     "",
			expectedPK:    "CONN#conn456",
			expectedSK:    "SUB#",
			expectedSubID: "conn456#",
		},
		{
			name:          "empty connection ID",
			connectionID:  "",
			requestID:     "req789",
			expectedPK:    "CONN#",
			expectedSK:    "SUB#req789",
			expectedSubID: "#req789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &dynamorm.Subscription{
				ConnectionID: tt.connectionID,
				RequestID:    tt.requestID,
			}
			sub.SetKeys()
			assert.Equal(t, tt.expectedPK, sub.PK)
			assert.Equal(t, tt.expectedSK, sub.SK)
			assert.Equal(t, tt.expectedSubID, sub.SubscriptionID)
		})
	}
}

// TestSubscription_ToStoreModel tests the ToStoreModel method
func TestSubscription_ToStoreModel(t *testing.T) {
	now := time.Now()
	sub := &dynamorm.Subscription{
		PK:             "CONN#conn123",
		SK:             "SUB#req123",
		SubscriptionID: "conn123#req123",
		ConnectionID:   "conn123",
		RequestID:      "req123",
		EventTypes:     []string{"progress", "completed", "failed"},
		CreatedAt:      now,
		TTL:            now.Add(30 * 24 * time.Hour).Unix(),
	}

	storeModel := sub.ToStoreModel()

	assert.Equal(t, "conn123#req123", storeModel.SubscriptionID)
	assert.Equal(t, "conn123", storeModel.ConnectionID)
	assert.Equal(t, "req123", storeModel.RequestID)
	assert.Equal(t, []string{"progress", "completed", "failed"}, storeModel.EventTypes)
	assert.Equal(t, now, storeModel.CreatedAt)
	assert.Equal(t, now.Add(30*24*time.Hour).Unix(), storeModel.TTL)
}

// TestSubscription_FromStoreModel tests the FromStoreModel method
func TestSubscription_FromStoreModel(t *testing.T) {
	now := time.Now()
	storeSub := &store.Subscription{
		SubscriptionID: "conn456#req456",
		ConnectionID:   "conn456",
		RequestID:      "req456",
		EventTypes:     []string{"started", "progress"},
		CreatedAt:      now,
		TTL:            now.Add(30 * 24 * time.Hour).Unix(),
	}

	sub := &dynamorm.Subscription{}
	sub.FromStoreModel(storeSub)

	assert.Equal(t, "CONN#conn456", sub.PK)
	assert.Equal(t, "SUB#req456", sub.SK)
	assert.Equal(t, "conn456#req456", sub.SubscriptionID)
	assert.Equal(t, "conn456", sub.ConnectionID)
	assert.Equal(t, "req456", sub.RequestID)
	assert.Equal(t, []string{"started", "progress"}, sub.EventTypes)
	assert.Equal(t, now, sub.CreatedAt)
	assert.Equal(t, now.Add(30*24*time.Hour).Unix(), sub.TTL)
}
