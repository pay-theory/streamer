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
)

// connectionStore implements ConnectionStore using DynamoDB
type connectionStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewConnectionStore creates a new DynamoDB-backed connection store
func NewConnectionStore(client *dynamodb.Client, tableName string) ConnectionStore {
	if tableName == "" {
		tableName = ConnectionsTable
	}
	return &connectionStore{
		client:    client,
		tableName: tableName,
	}
}

// Save creates or updates a connection
func (s *connectionStore) Save(ctx context.Context, conn *Connection) error {
	if err := s.validateConnection(conn); err != nil {
		return err
	}

	// Set TTL to 24 hours from now if not set
	if conn.TTL == 0 {
		conn.TTL = time.Now().Add(24 * time.Hour).Unix()
	}

	// Marshal connection to DynamoDB attribute values
	item, err := attributevalue.MarshalMap(conn)
	if err != nil {
		return NewStoreError("Save", s.tableName, conn.ConnectionID, fmt.Errorf("failed to marshal connection: %w", err))
	}

	// Put item in DynamoDB
	input := &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}

	_, err = s.client.PutItem(ctx, input)
	if err != nil {
		return NewStoreError("Save", s.tableName, conn.ConnectionID, fmt.Errorf("failed to save connection: %w", err))
	}

	return nil
}

// Get retrieves a connection by ID
func (s *connectionStore) Get(ctx context.Context, connectionID string) (*Connection, error) {
	if connectionID == "" {
		return nil, NewValidationError("connectionID", "cannot be empty")
	}

	input := &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"ConnectionID": &types.AttributeValueMemberS{Value: connectionID},
		},
	}

	result, err := s.client.GetItem(ctx, input)
	if err != nil {
		return nil, NewStoreError("Get", s.tableName, connectionID, fmt.Errorf("failed to get connection: %w", err))
	}

	if result.Item == nil {
		return nil, NewStoreError("Get", s.tableName, connectionID, ErrNotFound)
	}

	var conn Connection
	err = attributevalue.UnmarshalMap(result.Item, &conn)
	if err != nil {
		return nil, NewStoreError("Get", s.tableName, connectionID, fmt.Errorf("failed to unmarshal connection: %w", err))
	}

	return &conn, nil
}

// Delete removes a connection
func (s *connectionStore) Delete(ctx context.Context, connectionID string) error {
	if connectionID == "" {
		return NewValidationError("connectionID", "cannot be empty")
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"ConnectionID": &types.AttributeValueMemberS{Value: connectionID},
		},
	}

	_, err := s.client.DeleteItem(ctx, input)
	if err != nil {
		return NewStoreError("Delete", s.tableName, connectionID, fmt.Errorf("failed to delete connection: %w", err))
	}

	return nil
}

// ListByUser returns all connections for a user
func (s *connectionStore) ListByUser(ctx context.Context, userID string) ([]*Connection, error) {
	if userID == "" {
		return nil, NewValidationError("userID", "cannot be empty")
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("UserIndex"),
		KeyConditionExpression: aws.String("UserID = :userID"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":userID": &types.AttributeValueMemberS{Value: userID},
		},
	}

	return s.queryConnections(ctx, input)
}

// ListByTenant returns all connections for a tenant
func (s *connectionStore) ListByTenant(ctx context.Context, tenantID string) ([]*Connection, error) {
	if tenantID == "" {
		return nil, NewValidationError("tenantID", "cannot be empty")
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String("TenantIndex"),
		KeyConditionExpression: aws.String("TenantID = :tenantID"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":tenantID": &types.AttributeValueMemberS{Value: tenantID},
		},
	}

	return s.queryConnections(ctx, input)
}

// UpdateLastPing updates the last ping timestamp
func (s *connectionStore) UpdateLastPing(ctx context.Context, connectionID string) error {
	if connectionID == "" {
		return NewValidationError("connectionID", "cannot be empty")
	}

	now := time.Now()
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"ConnectionID": &types.AttributeValueMemberS{Value: connectionID},
		},
		UpdateExpression: aws.String("SET LastPing = :now, #ttl = :ttl"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "TTL", // TTL is a reserved word in DynamoDB
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339Nano)},
			":ttl": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", now.Add(24*time.Hour).Unix())},
		},
		ConditionExpression: aws.String("attribute_exists(ConnectionID)"),
	}

	_, err := s.client.UpdateItem(ctx, input)
	if err != nil {
		var cfe *types.ConditionalCheckFailedException
		if errors.As(err, &cfe) {
			return NewStoreError("UpdateLastPing", s.tableName, connectionID, ErrNotFound)
		}
		return NewStoreError("UpdateLastPing", s.tableName, connectionID, fmt.Errorf("failed to update last ping: %w", err))
	}

	return nil
}

// DeleteStale removes connections older than the specified time
func (s *connectionStore) DeleteStale(ctx context.Context, before time.Time) error {
	// In production, this would be handled by DynamoDB TTL
	// This method is primarily for testing and manual cleanup

	// Scan for old connections
	input := &dynamodb.ScanInput{
		TableName:        aws.String(s.tableName),
		FilterExpression: aws.String("LastPing < :before"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":before": &types.AttributeValueMemberS{Value: before.Format(time.RFC3339Nano)},
		},
	}

	paginator := dynamodb.NewScanPaginator(s.client, input)

	var deleteCount int
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return NewStoreError("DeleteStale", s.tableName, "", fmt.Errorf("failed to scan stale connections: %w", err))
		}

		// Delete each stale connection
		for _, item := range page.Items {
			if connID, ok := item["ConnectionID"].(*types.AttributeValueMemberS); ok {
				if err := s.Delete(ctx, connID.Value); err != nil {
					// Log error but continue with other deletions
					fmt.Printf("Failed to delete stale connection %s: %v\n", connID.Value, err)
				} else {
					deleteCount++
				}
			}
		}
	}

	return nil
}

// queryConnections executes a query and returns connections
func (s *connectionStore) queryConnections(ctx context.Context, input *dynamodb.QueryInput) ([]*Connection, error) {
	var connections []*Connection

	paginator := dynamodb.NewQueryPaginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query connections: %w", err)
		}

		for _, item := range page.Items {
			var conn Connection
			if err := attributevalue.UnmarshalMap(item, &conn); err != nil {
				// Skip invalid items
				continue
			}
			connections = append(connections, &conn)
		}
	}

	return connections, nil
}

// validateConnection validates a connection before saving
func (s *connectionStore) validateConnection(conn *Connection) error {
	if conn == nil {
		return NewValidationError("connection", "cannot be nil")
	}
	if conn.ConnectionID == "" {
		return NewValidationError("ConnectionID", "cannot be empty")
	}
	if conn.UserID == "" {
		return NewValidationError("UserID", "cannot be empty")
	}
	if conn.TenantID == "" {
		return NewValidationError("TenantID", "cannot be empty")
	}
	if conn.Endpoint == "" {
		return NewValidationError("Endpoint", "cannot be empty")
	}
	return nil
}
