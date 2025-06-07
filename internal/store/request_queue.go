package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// requestQueue implements RequestQueue using DynamoDB
type requestQueue struct {
	client    *dynamodb.Client
	tableName string
}

// NewRequestQueue creates a new DynamoDB-backed request queue
func NewRequestQueue(client *dynamodb.Client, tableName string) RequestQueue {
	if tableName == "" {
		tableName = RequestsTable
	}
	return &requestQueue{
		client:    client,
		tableName: tableName,
	}
}

// Enqueue adds a new request to the queue
func (q *requestQueue) Enqueue(ctx context.Context, req *AsyncRequest) error {
	if err := q.validateRequest(req); err != nil {
		return err
	}

	// Set defaults
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	if req.Status == "" {
		req.Status = StatusPending
	}
	if req.TTL == 0 {
		// Default TTL of 7 days
		req.TTL = time.Now().Add(7 * 24 * time.Hour).Unix()
	}
	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}

	// Marshal request to DynamoDB attribute values
	item, err := attributevalue.MarshalMap(req)
	if err != nil {
		return NewStoreError("Enqueue", q.tableName, req.RequestID, fmt.Errorf("failed to marshal request: %w", err))
	}

	// Put item with condition check to prevent overwrites
	input := &dynamodb.PutItemInput{
		TableName:           aws.String(q.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(RequestID)"),
	}

	_, err = q.client.PutItem(ctx, input)
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return NewStoreError("Enqueue", q.tableName, req.RequestID, ErrAlreadyExists)
		}
		return NewStoreError("Enqueue", q.tableName, req.RequestID, fmt.Errorf("failed to enqueue request: %w", err))
	}

	return nil
}

// Dequeue retrieves and marks requests for processing
func (q *requestQueue) Dequeue(ctx context.Context, limit int) ([]*AsyncRequest, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // DynamoDB query limit
	}

	// Query for pending requests
	input := &dynamodb.QueryInput{
		TableName:              aws.String(q.tableName),
		IndexName:              aws.String("StatusIndex"),
		KeyConditionExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "Status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(StatusPending)},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(true), // Oldest first
	}

	result, err := q.client.Query(ctx, input)
	if err != nil {
		return nil, NewStoreError("Dequeue", q.tableName, "", fmt.Errorf("failed to query pending requests: %w", err))
	}

	var requests []*AsyncRequest
	for _, item := range result.Items {
		var req AsyncRequest
		if err := attributevalue.UnmarshalMap(item, &req); err != nil {
			continue // Skip invalid items
		}

		// Try to mark as processing
		if err := q.UpdateStatus(ctx, req.RequestID, StatusProcessing, "Request dequeued for processing"); err != nil {
			// Another worker might have grabbed it
			continue
		}

		requests = append(requests, &req)
	}

	return requests, nil
}

// UpdateStatus updates the status of a request
func (q *requestQueue) UpdateStatus(ctx context.Context, requestID string, status RequestStatus, message string) error {
	if requestID == "" {
		return NewValidationError("requestID", "cannot be empty")
	}

	updateExpr := "SET #status = :status"
	exprAttrNames := map[string]string{
		"#status": "Status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: string(status)},
	}

	// Add message if provided
	if message != "" {
		updateExpr += ", ProgressMessage = :message"
		exprAttrValues[":message"] = &types.AttributeValueMemberS{Value: message}
	}

	// Update processing timestamps
	now := time.Now()
	switch status {
	case StatusProcessing:
		updateExpr += ", ProcessingStarted = :now"
		exprAttrValues[":now"] = &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)}
	case StatusCompleted, StatusFailed:
		updateExpr += ", ProcessingEnded = :now"
		exprAttrValues[":now"] = &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)}
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(q.tableName),
		Key: map[string]types.AttributeValue{
			"RequestID": &types.AttributeValueMemberS{Value: requestID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrValues,
		ConditionExpression:       aws.String("attribute_exists(RequestID)"),
	}

	_, err := q.client.UpdateItem(ctx, input)
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return NewStoreError("UpdateStatus", q.tableName, requestID, ErrNotFound)
		}
		return NewStoreError("UpdateStatus", q.tableName, requestID, fmt.Errorf("failed to update status: %w", err))
	}

	return nil
}

// UpdateProgress updates the progress of a request
func (q *requestQueue) UpdateProgress(ctx context.Context, requestID string, progress float64, message string, details map[string]interface{}) error {
	if requestID == "" {
		return NewValidationError("requestID", "cannot be empty")
	}
	if progress < 0 || progress > 1 {
		return NewValidationError("progress", "must be between 0 and 1")
	}

	updateExpr := "SET Progress = :progress"
	exprAttrValues := map[string]types.AttributeValue{
		":progress":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", progress)},
		":processing": &types.AttributeValueMemberS{Value: string(StatusProcessing)},
	}

	if message != "" {
		updateExpr += ", ProgressMessage = :message"
		exprAttrValues[":message"] = &types.AttributeValueMemberS{Value: message}
	}

	if details != nil {
		detailsAV, err := attributevalue.MarshalMap(details)
		if err == nil {
			updateExpr += ", ProgressDetails = :details"
			exprAttrValues[":details"] = &types.AttributeValueMemberM{Value: detailsAV}
		}
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(q.tableName),
		Key: map[string]types.AttributeValue{
			"RequestID": &types.AttributeValueMemberS{Value: requestID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
		ConditionExpression:       aws.String("attribute_exists(RequestID) AND #status = :processing"),
		ExpressionAttributeNames: map[string]string{
			"#status": "Status",
		},
	}

	_, err := q.client.UpdateItem(ctx, input)
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return NewStoreError("UpdateProgress", q.tableName, requestID, ErrRequestNotPending)
		}
		return NewStoreError("UpdateProgress", q.tableName, requestID, fmt.Errorf("failed to update progress: %w", err))
	}

	return nil
}

// CompleteRequest marks a request as completed with results
func (q *requestQueue) CompleteRequest(ctx context.Context, requestID string, result map[string]interface{}) error {
	if requestID == "" {
		return NewValidationError("requestID", "cannot be empty")
	}

	updateExpr := "SET #status = :status, ProcessingEnded = :now, Progress = :progress"
	exprAttrNames := map[string]string{
		"#status": "Status",
	}
	exprAttrValues := map[string]types.AttributeValue{
		":status":   &types.AttributeValueMemberS{Value: string(StatusCompleted)},
		":now":      &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339Nano)},
		":progress": &types.AttributeValueMemberN{Value: "1.0"},
	}

	if result != nil {
		resultAV, err := attributevalue.MarshalMap(result)
		if err != nil {
			return NewStoreError("CompleteRequest", q.tableName, requestID, fmt.Errorf("failed to marshal result: %w", err))
		}
		updateExpr += ", #result = :result"
		exprAttrNames["#result"] = "Result"
		exprAttrValues[":result"] = &types.AttributeValueMemberM{Value: resultAV}
	}

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(q.tableName),
		Key:                       map[string]types.AttributeValue{"RequestID": &types.AttributeValueMemberS{Value: requestID}},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprAttrNames,
		ExpressionAttributeValues: exprAttrValues,
		ConditionExpression:       aws.String("attribute_exists(RequestID)"),
	}

	_, err := q.client.UpdateItem(ctx, input)
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return NewStoreError("CompleteRequest", q.tableName, requestID, ErrNotFound)
		}
		return NewStoreError("CompleteRequest", q.tableName, requestID, fmt.Errorf("failed to complete request: %w", err))
	}

	return nil
}

// FailRequest marks a request as failed with an error
func (q *requestQueue) FailRequest(ctx context.Context, requestID string, errMsg string) error {
	if requestID == "" {
		return NewValidationError("requestID", "cannot be empty")
	}
	if errMsg == "" {
		return NewValidationError("error", "cannot be empty")
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(q.tableName),
		Key: map[string]types.AttributeValue{
			"RequestID": &types.AttributeValueMemberS{Value: requestID},
		},
		UpdateExpression: aws.String("SET #status = :status, ProcessingEnded = :now, #error = :error"),
		ExpressionAttributeNames: map[string]string{
			"#status": "Status",
			"#error":  "Error",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(StatusFailed)},
			":now":    &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339Nano)},
			":error":  &types.AttributeValueMemberS{Value: errMsg},
		},
		ConditionExpression: aws.String("attribute_exists(RequestID)"),
	}

	_, err := q.client.UpdateItem(ctx, input)
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return NewStoreError("FailRequest", q.tableName, requestID, ErrNotFound)
		}
		return NewStoreError("FailRequest", q.tableName, requestID, fmt.Errorf("failed to fail request: %w", err))
	}

	return nil
}

// GetByConnection retrieves all requests for a connection
func (q *requestQueue) GetByConnection(ctx context.Context, connectionID string, limit int) ([]*AsyncRequest, error) {
	if connectionID == "" {
		return nil, NewValidationError("connectionID", "cannot be empty")
	}
	if limit <= 0 {
		limit = 100
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(q.tableName),
		IndexName:              aws.String("ConnectionIndex"),
		KeyConditionExpression: aws.String("ConnectionID = :connectionID"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":connectionID": &types.AttributeValueMemberS{Value: connectionID},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(false), // Newest first
	}

	return q.queryRequests(ctx, input)
}

// GetByStatus retrieves requests by status
func (q *requestQueue) GetByStatus(ctx context.Context, status RequestStatus, limit int) ([]*AsyncRequest, error) {
	if limit <= 0 {
		limit = 100
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(q.tableName),
		IndexName:              aws.String("StatusIndex"),
		KeyConditionExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "Status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(status)},
		},
		Limit:            aws.Int32(int32(limit)),
		ScanIndexForward: aws.Bool(true), // Oldest first for processing
	}

	return q.queryRequests(ctx, input)
}

// Get retrieves a specific request
func (q *requestQueue) Get(ctx context.Context, requestID string) (*AsyncRequest, error) {
	if requestID == "" {
		return nil, NewValidationError("requestID", "cannot be empty")
	}

	input := &dynamodb.GetItemInput{
		TableName: aws.String(q.tableName),
		Key: map[string]types.AttributeValue{
			"RequestID": &types.AttributeValueMemberS{Value: requestID},
		},
	}

	result, err := q.client.GetItem(ctx, input)
	if err != nil {
		return nil, NewStoreError("Get", q.tableName, requestID, fmt.Errorf("failed to get request: %w", err))
	}

	if result.Item == nil {
		return nil, NewStoreError("Get", q.tableName, requestID, ErrNotFound)
	}

	var req AsyncRequest
	err = attributevalue.UnmarshalMap(result.Item, &req)
	if err != nil {
		return nil, NewStoreError("Get", q.tableName, requestID, fmt.Errorf("failed to unmarshal request: %w", err))
	}

	return &req, nil
}

// Delete removes a request
func (q *requestQueue) Delete(ctx context.Context, requestID string) error {
	if requestID == "" {
		return NewValidationError("requestID", "cannot be empty")
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(q.tableName),
		Key: map[string]types.AttributeValue{
			"RequestID": &types.AttributeValueMemberS{Value: requestID},
		},
	}

	_, err := q.client.DeleteItem(ctx, input)
	if err != nil {
		return NewStoreError("Delete", q.tableName, requestID, fmt.Errorf("failed to delete request: %w", err))
	}

	return nil
}

// queryRequests executes a query and returns requests
func (q *requestQueue) queryRequests(ctx context.Context, input *dynamodb.QueryInput) ([]*AsyncRequest, error) {
	var requests []*AsyncRequest

	// Use paginator for large result sets
	paginator := dynamodb.NewQueryPaginator(q.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query requests: %w", err)
		}

		for _, item := range page.Items {
			var req AsyncRequest
			if err := attributevalue.UnmarshalMap(item, &req); err != nil {
				// Skip invalid items
				continue
			}
			requests = append(requests, &req)
		}

		// If we've reached the limit, stop paginating
		if input.Limit != nil && len(requests) >= int(*input.Limit) {
			break
		}
	}

	return requests, nil
}

// validateRequest validates a request before enqueueing
func (q *requestQueue) validateRequest(req *AsyncRequest) error {
	if req == nil {
		return NewValidationError("request", "cannot be nil")
	}
	if req.ConnectionID == "" {
		return NewValidationError("ConnectionID", "cannot be empty")
	}
	if req.Action == "" {
		return NewValidationError("Action", "cannot be empty")
	}
	return nil
}
