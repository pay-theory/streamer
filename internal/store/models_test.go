package store

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRequestStatusConstants tests that all request status constants are defined
func TestRequestStatusConstants(t *testing.T) {
	// Verify all status constants
	assert.Equal(t, RequestStatus("PENDING"), StatusPending)
	assert.Equal(t, RequestStatus("PROCESSING"), StatusProcessing)
	assert.Equal(t, RequestStatus("COMPLETED"), StatusCompleted)
	assert.Equal(t, RequestStatus("FAILED"), StatusFailed)
	assert.Equal(t, RequestStatus("CANCELLED"), StatusCancelled)
	assert.Equal(t, RequestStatus("RETRYING"), StatusRetrying)

	// Ensure they are all unique
	statuses := []RequestStatus{
		StatusPending,
		StatusProcessing,
		StatusCompleted,
		StatusFailed,
		StatusCancelled,
		StatusRetrying,
	}

	seen := make(map[RequestStatus]bool)
	for _, status := range statuses {
		assert.False(t, seen[status], "Duplicate status: %s", status)
		seen[status] = true
	}
}

// TestTableNameConstants tests that table name constants are defined
func TestTableNameConstants(t *testing.T) {
	assert.Equal(t, "streamer_connections", ConnectionsTable)
	assert.Equal(t, "streamer_requests", RequestsTable)
	assert.Equal(t, "streamer_subscriptions", SubscriptionsTable)
}

// TestConnectionStruct tests the Connection struct
func TestConnectionStruct(t *testing.T) {
	now := time.Now()
	conn := Connection{
		ConnectionID: "conn123",
		UserID:       "user456",
		TenantID:     "tenant789",
		Endpoint:     "wss://example.com",
		ConnectedAt:  now,
		LastPing:     now.Add(5 * time.Minute),
		Metadata: map[string]string{
			"source": "web",
			"region": "us-east-1",
		},
		TTL: now.Add(24 * time.Hour).Unix(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(conn)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var decoded Connection
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, conn.ConnectionID, decoded.ConnectionID)
	assert.Equal(t, conn.UserID, decoded.UserID)
	assert.Equal(t, conn.TenantID, decoded.TenantID)
	assert.Equal(t, conn.Endpoint, decoded.Endpoint)
	assert.Equal(t, conn.Metadata, decoded.Metadata)
	assert.Equal(t, conn.TTL, decoded.TTL)
}

// TestAsyncRequestStruct tests the AsyncRequest struct
func TestAsyncRequestStruct(t *testing.T) {
	now := time.Now()
	processingStart := now.Add(1 * time.Minute)
	processingEnd := now.Add(2 * time.Minute)

	req := AsyncRequest{
		RequestID:    "req123",
		ConnectionID: "conn456",
		Status:       StatusProcessing,
		CreatedAt:    now,
		Action:       "process.data",
		Payload: map[string]interface{}{
			"input": "test data",
			"count": float64(100),
		},
		ProcessingStarted: &processingStart,
		ProcessingEnded:   &processingEnd,
		Result: map[string]interface{}{
			"output":  "processed",
			"records": float64(50),
		},
		Error:           "",
		Progress:        75.5,
		ProgressMessage: "Processing records",
		ProgressDetails: map[string]interface{}{
			"current": float64(75),
			"total":   float64(100),
		},
		RetryCount: 1,
		MaxRetries: 3,
		RetryAfter: now.Add(5 * time.Minute),
		UserID:     "user789",
		TenantID:   "tenant123",
		TTL:        now.Add(48 * time.Hour).Unix(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(req)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var decoded AsyncRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, req.RequestID, decoded.RequestID)
	assert.Equal(t, req.ConnectionID, decoded.ConnectionID)
	assert.Equal(t, req.Status, decoded.Status)
	assert.Equal(t, req.Action, decoded.Action)
	assert.Equal(t, req.Payload, decoded.Payload)
	assert.Equal(t, req.Result, decoded.Result)
	assert.Equal(t, req.Progress, decoded.Progress)
	assert.Equal(t, req.ProgressMessage, decoded.ProgressMessage)
	assert.Equal(t, req.RetryCount, decoded.RetryCount)
	assert.Equal(t, req.MaxRetries, decoded.MaxRetries)
}

// TestSubscriptionStruct tests the Subscription struct
func TestSubscriptionStruct(t *testing.T) {
	now := time.Now()
	sub := Subscription{
		SubscriptionID: "conn123#req456",
		ConnectionID:   "conn123",
		RequestID:      "req456",
		EventTypes:     []string{"progress", "completed", "failed"},
		CreatedAt:      now,
		TTL:            now.Add(24 * time.Hour).Unix(),
	}

	// Test JSON marshaling
	data, err := json.Marshal(sub)
	assert.NoError(t, err)

	// Test JSON unmarshaling
	var decoded Subscription
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, sub.SubscriptionID, decoded.SubscriptionID)
	assert.Equal(t, sub.ConnectionID, decoded.ConnectionID)
	assert.Equal(t, sub.RequestID, decoded.RequestID)
	assert.ElementsMatch(t, sub.EventTypes, decoded.EventTypes)
	assert.Equal(t, sub.TTL, decoded.TTL)
}

// TestStructTags tests that all structs have proper tags
func TestStructTags(t *testing.T) {
	// Test Connection tags
	connType := reflect.TypeOf(Connection{})
	testStructTags(t, connType, map[string]struct {
		dynamodb string
		json     string
	}{
		"ConnectionID": {dynamodb: "ConnectionID", json: "connectionId"},
		"UserID":       {dynamodb: "UserID", json: "userId"},
		"TenantID":     {dynamodb: "TenantID", json: "tenantId"},
		"Endpoint":     {dynamodb: "Endpoint", json: "endpoint"},
		"ConnectedAt":  {dynamodb: "ConnectedAt", json: "connectedAt"},
		"LastPing":     {dynamodb: "LastPing", json: "lastPing"},
		"Metadata":     {dynamodb: "Metadata,omitempty", json: "metadata,omitempty"},
		"TTL":          {dynamodb: "TTL,omitempty", json: "ttl,omitempty"},
	})

	// Test AsyncRequest tags
	reqType := reflect.TypeOf(AsyncRequest{})
	testStructTags(t, reqType, map[string]struct {
		dynamodb string
		json     string
	}{
		"RequestID":         {dynamodb: "RequestID", json: "requestId"},
		"ConnectionID":      {dynamodb: "ConnectionID", json: "connectionId"},
		"Status":            {dynamodb: "Status", json: "status"},
		"CreatedAt":         {dynamodb: "CreatedAt", json: "createdAt"},
		"Action":            {dynamodb: "Action", json: "action"},
		"Payload":           {dynamodb: "Payload,omitempty", json: "payload,omitempty"},
		"ProcessingStarted": {dynamodb: "ProcessingStarted,omitempty", json: "processingStarted,omitempty"},
		"ProcessingEnded":   {dynamodb: "ProcessingEnded,omitempty", json: "processingEnded,omitempty"},
		"Result":            {dynamodb: "Result,omitempty", json: "result,omitempty"},
		"Error":             {dynamodb: "Error,omitempty", json: "error,omitempty"},
		"Progress":          {dynamodb: "Progress", json: "progress"},
		"ProgressMessage":   {dynamodb: "ProgressMessage,omitempty", json: "progressMessage,omitempty"},
		"ProgressDetails":   {dynamodb: "ProgressDetails,omitempty", json: "progressDetails,omitempty"},
		"RetryCount":        {dynamodb: "RetryCount", json: "retryCount"},
		"MaxRetries":        {dynamodb: "MaxRetries", json: "maxRetries"},
		"RetryAfter":        {dynamodb: "RetryAfter,omitempty", json: "retryAfter,omitempty"},
		"UserID":            {dynamodb: "UserID", json: "userId"},
		"TenantID":          {dynamodb: "TenantID", json: "tenantId"},
		"TTL":               {dynamodb: "TTL,omitempty", json: "ttl,omitempty"},
	})

	// Test Subscription tags
	subType := reflect.TypeOf(Subscription{})
	testStructTags(t, subType, map[string]struct {
		dynamodb string
		json     string
	}{
		"SubscriptionID": {dynamodb: "SubscriptionID", json: "subscriptionId"},
		"ConnectionID":   {dynamodb: "ConnectionID", json: "connectionId"},
		"RequestID":      {dynamodb: "RequestID", json: "requestId"},
		"EventTypes":     {dynamodb: "EventTypes,stringset", json: "eventTypes"},
		"CreatedAt":      {dynamodb: "CreatedAt", json: "createdAt"},
		"TTL":            {dynamodb: "TTL,omitempty", json: "ttl,omitempty"},
	})
}

// testStructTags is a helper function to test struct tags
func testStructTags(t *testing.T, structType reflect.Type, expectedTags map[string]struct {
	dynamodb string
	json     string
}) {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		expected, exists := expectedTags[field.Name]
		if !exists {
			t.Errorf("Unexpected field %s in struct %s", field.Name, structType.Name())
			continue
		}

		// Check DynamoDB tag
		dynamoTag := field.Tag.Get("dynamodbav")
		assert.Equal(t, expected.dynamodb, dynamoTag, "Field %s has incorrect dynamodbav tag", field.Name)

		// Check JSON tag
		jsonTag := field.Tag.Get("json")
		assert.Equal(t, expected.json, jsonTag, "Field %s has incorrect json tag", field.Name)
	}

	// Ensure all expected fields exist
	for fieldName := range expectedTags {
		_, found := structType.FieldByName(fieldName)
		assert.True(t, found, "Expected field %s not found in struct %s", fieldName, structType.Name())
	}
}

// TestEmptyStructs tests structs with zero values
func TestEmptyStructs(t *testing.T) {
	// Test empty Connection
	var conn Connection
	data, err := json.Marshal(conn)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"connectionId":""`)
	assert.NotContains(t, string(data), `"metadata"`) // omitempty

	// Test empty AsyncRequest
	var req AsyncRequest
	data, err = json.Marshal(req)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"requestId":""`)
	assert.Contains(t, string(data), `"progress":0`)
	assert.NotContains(t, string(data), `"payload"`) // omitempty

	// Test empty Subscription
	var sub Subscription
	data, err = json.Marshal(sub)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"subscriptionId":""`)
}

// TestRequestStatusString tests RequestStatus as a string type
func TestRequestStatusString(t *testing.T) {
	// Test that RequestStatus can be used as a string
	status := StatusProcessing
	str := string(status)
	assert.Equal(t, "PROCESSING", str)

	// Test creating RequestStatus from string
	var status2 RequestStatus = "CUSTOM_STATUS"
	assert.Equal(t, RequestStatus("CUSTOM_STATUS"), status2)

	// Test in switch statement
	testStatus := func(s RequestStatus) string {
		switch s {
		case StatusPending:
			return "pending"
		case StatusProcessing:
			return "processing"
		case StatusCompleted:
			return "completed"
		case StatusFailed:
			return "failed"
		case StatusCancelled:
			return "cancelled"
		case StatusRetrying:
			return "retrying"
		default:
			return "unknown"
		}
	}

	assert.Equal(t, "pending", testStatus(StatusPending))
	assert.Equal(t, "processing", testStatus(StatusProcessing))
	assert.Equal(t, "completed", testStatus(StatusCompleted))
	assert.Equal(t, "failed", testStatus(StatusFailed))
	assert.Equal(t, "cancelled", testStatus(StatusCancelled))
	assert.Equal(t, "retrying", testStatus(StatusRetrying))
	assert.Equal(t, "unknown", testStatus("UNKNOWN"))
}
