package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// TestConnectionStore is the main test suite for ConnectionStore
func TestConnectionStore(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test client
	client := createTestDynamoDBClient(t)
	store := NewConnectionStore(client, "test_connections")

	// Create test table
	createTestConnectionTable(t, client)
	defer deleteTestTable(t, client, "test_connections")

	// Run test cases
	t.Run("Save", func(t *testing.T) {
		testConnectionSave(t, store)
	})
	t.Run("Get", func(t *testing.T) {
		testConnectionGet(t, store)
	})
	t.Run("Delete", func(t *testing.T) {
		testConnectionDelete(t, store)
	})
	t.Run("ListByUser", func(t *testing.T) {
		testConnectionListByUser(t, store)
	})
	t.Run("ListByTenant", func(t *testing.T) {
		testConnectionListByTenant(t, store)
	})
	t.Run("UpdateLastPing", func(t *testing.T) {
		testConnectionUpdateLastPing(t, store)
	})
	t.Run("DeleteStale", func(t *testing.T) {
		testConnectionDeleteStale(t, store)
	})
}

func testConnectionSave(t *testing.T, store ConnectionStore) {
	ctx := context.Background()

	tests := []struct {
		name    string
		conn    *Connection
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid connection",
			conn: &Connection{
				ConnectionID: uuid.New().String(),
				UserID:       "user123",
				TenantID:     "tenant456",
				Endpoint:     "wss://example.com/ws",
				ConnectedAt:  time.Now(),
				LastPing:     time.Now(),
				Metadata: map[string]string{
					"client": "web",
				},
			},
			wantErr: false,
		},
		{
			name:    "nil connection",
			conn:    nil,
			wantErr: true,
			errMsg:  "cannot be nil",
		},
		{
			name: "missing connection ID",
			conn: &Connection{
				UserID:   "user123",
				TenantID: "tenant456",
				Endpoint: "wss://example.com/ws",
			},
			wantErr: true,
			errMsg:  "ConnectionID",
		},
		{
			name: "missing user ID",
			conn: &Connection{
				ConnectionID: uuid.New().String(),
				TenantID:     "tenant456",
				Endpoint:     "wss://example.com/ws",
			},
			wantErr: true,
			errMsg:  "UserID",
		},
		{
			name: "missing tenant ID",
			conn: &Connection{
				ConnectionID: uuid.New().String(),
				UserID:       "user123",
				Endpoint:     "wss://example.com/ws",
			},
			wantErr: true,
			errMsg:  "TenantID",
		},
		{
			name: "missing endpoint",
			conn: &Connection{
				ConnectionID: uuid.New().String(),
				UserID:       "user123",
				TenantID:     "tenant456",
			},
			wantErr: true,
			errMsg:  "Endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Save(ctx, tt.conn)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func testConnectionGet(t *testing.T, store ConnectionStore) {
	ctx := context.Background()

	// Save a test connection
	conn := &Connection{
		ConnectionID: uuid.New().String(),
		UserID:       "user123",
		TenantID:     "tenant456",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
		Metadata: map[string]string{
			"client": "web",
		},
	}
	if err := store.Save(ctx, conn); err != nil {
		t.Fatalf("failed to save test connection: %v", err)
	}

	tests := []struct {
		name         string
		connectionID string
		wantErr      bool
		errType      error
	}{
		{
			name:         "existing connection",
			connectionID: conn.ConnectionID,
			wantErr:      false,
		},
		{
			name:         "non-existent connection",
			connectionID: uuid.New().String(),
			wantErr:      true,
			errType:      ErrNotFound,
		},
		{
			name:         "empty connection ID",
			connectionID: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Get(ctx, tt.connectionID)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errType != nil && !IsNotFound(err) {
					t.Errorf("expected ErrNotFound, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got.ConnectionID != conn.ConnectionID {
					t.Errorf("connection ID mismatch: got %s, want %s", got.ConnectionID, conn.ConnectionID)
				}
				if got.UserID != conn.UserID {
					t.Errorf("user ID mismatch: got %s, want %s", got.UserID, conn.UserID)
				}
			}
		})
	}
}

func testConnectionDelete(t *testing.T, store ConnectionStore) {
	ctx := context.Background()

	// Save a test connection
	conn := &Connection{
		ConnectionID: uuid.New().String(),
		UserID:       "user123",
		TenantID:     "tenant456",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
	}
	if err := store.Save(ctx, conn); err != nil {
		t.Fatalf("failed to save test connection: %v", err)
	}

	// Delete the connection
	if err := store.Delete(ctx, conn.ConnectionID); err != nil {
		t.Errorf("failed to delete connection: %v", err)
	}

	// Verify it's deleted
	_, err := store.Get(ctx, conn.ConnectionID)
	if !IsNotFound(err) {
		t.Error("expected connection to be deleted")
	}

	// Delete non-existent connection should not error
	if err := store.Delete(ctx, uuid.New().String()); err != nil {
		t.Errorf("delete non-existent connection should not error: %v", err)
	}
}

func testConnectionListByUser(t *testing.T, store ConnectionStore) {
	ctx := context.Background()
	userID := "user-" + uuid.New().String()

	// Save multiple connections for the same user
	var connections []*Connection
	for i := 0; i < 3; i++ {
		conn := &Connection{
			ConnectionID: uuid.New().String(),
			UserID:       userID,
			TenantID:     "tenant456",
			Endpoint:     "wss://example.com/ws",
			ConnectedAt:  time.Now(),
			LastPing:     time.Now(),
		}
		if err := store.Save(ctx, conn); err != nil {
			t.Fatalf("failed to save connection %d: %v", i, err)
		}
		connections = append(connections, conn)
	}

	// Save a connection for a different user
	otherConn := &Connection{
		ConnectionID: uuid.New().String(),
		UserID:       "other-user",
		TenantID:     "tenant456",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
	}
	if err := store.Save(ctx, otherConn); err != nil {
		t.Fatalf("failed to save other user connection: %v", err)
	}

	// List connections for the user
	got, err := store.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("failed to list connections: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("expected 3 connections, got %d", len(got))
	}

	// Verify all connections belong to the user
	for _, conn := range got {
		if conn.UserID != userID {
			t.Errorf("unexpected user ID: got %s, want %s", conn.UserID, userID)
		}
	}
}

func testConnectionListByTenant(t *testing.T, store ConnectionStore) {
	ctx := context.Background()
	tenantID := "tenant-" + uuid.New().String()

	// Save multiple connections for the same tenant
	var connections []*Connection
	for i := 0; i < 3; i++ {
		conn := &Connection{
			ConnectionID: uuid.New().String(),
			UserID:       "user" + uuid.New().String(),
			TenantID:     tenantID,
			Endpoint:     "wss://example.com/ws",
			ConnectedAt:  time.Now(),
			LastPing:     time.Now(),
		}
		if err := store.Save(ctx, conn); err != nil {
			t.Fatalf("failed to save connection %d: %v", i, err)
		}
		connections = append(connections, conn)
	}

	// List connections for the tenant
	got, err := store.ListByTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("failed to list connections: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("expected 3 connections, got %d", len(got))
	}

	// Verify all connections belong to the tenant
	for _, conn := range got {
		if conn.TenantID != tenantID {
			t.Errorf("unexpected tenant ID: got %s, want %s", conn.TenantID, tenantID)
		}
	}
}

func testConnectionUpdateLastPing(t *testing.T, store ConnectionStore) {
	ctx := context.Background()

	// Save a test connection
	conn := &Connection{
		ConnectionID: uuid.New().String(),
		UserID:       "user123",
		TenantID:     "tenant456",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  time.Now(),
		LastPing:     time.Now().Add(-1 * time.Hour),
	}
	if err := store.Save(ctx, conn); err != nil {
		t.Fatalf("failed to save test connection: %v", err)
	}

	// Update last ping
	if err := store.UpdateLastPing(ctx, conn.ConnectionID); err != nil {
		t.Errorf("failed to update last ping: %v", err)
	}

	// Verify the update
	updated, err := store.Get(ctx, conn.ConnectionID)
	if err != nil {
		t.Fatalf("failed to get updated connection: %v", err)
	}

	if updated.LastPing.Before(conn.LastPing) || updated.LastPing.Equal(conn.LastPing) {
		t.Error("last ping was not updated")
	}

	// Update non-existent connection
	err = store.UpdateLastPing(ctx, uuid.New().String())
	if !IsNotFound(err) {
		t.Error("expected ErrNotFound for non-existent connection")
	}
}

func testConnectionDeleteStale(t *testing.T, store ConnectionStore) {
	ctx := context.Background()

	// Save old and new connections
	oldConn := &Connection{
		ConnectionID: uuid.New().String(),
		UserID:       "user123",
		TenantID:     "tenant456",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  time.Now().Add(-2 * time.Hour),
		LastPing:     time.Now().Add(-2 * time.Hour),
	}
	if err := store.Save(ctx, oldConn); err != nil {
		t.Fatalf("failed to save old connection: %v", err)
	}

	newConn := &Connection{
		ConnectionID: uuid.New().String(),
		UserID:       "user456",
		TenantID:     "tenant456",
		Endpoint:     "wss://example.com/ws",
		ConnectedAt:  time.Now(),
		LastPing:     time.Now(),
	}
	if err := store.Save(ctx, newConn); err != nil {
		t.Fatalf("failed to save new connection: %v", err)
	}

	// Delete stale connections
	if err := store.DeleteStale(ctx, time.Now().Add(-1*time.Hour)); err != nil {
		t.Errorf("failed to delete stale connections: %v", err)
	}

	// Verify old connection is deleted
	_, err := store.Get(ctx, oldConn.ConnectionID)
	if !IsNotFound(err) {
		t.Error("expected old connection to be deleted")
	}

	// Verify new connection still exists
	_, err = store.Get(ctx, newConn.ConnectionID)
	if err != nil {
		t.Error("new connection should not be deleted")
	}
}

// Helper functions

func createTestDynamoDBClient(t *testing.T) *dynamodb.Client {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://localhost:8000"}, nil
			})),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	return dynamodb.NewFromConfig(cfg)
}

func createTestConnectionTable(t *testing.T, client *dynamodb.Client) {
	input := &dynamodb.CreateTableInput{
		TableName: aws.String("test_connections"),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("ConnectionID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("UserID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("TenantID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("ConnectionID"),
				KeyType:       types.KeyTypeHash,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("UserIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("UserID"),
						KeyType:       types.KeyTypeHash,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
			{
				IndexName: aws.String("TenantIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("TenantID"),
						KeyType:       types.KeyTypeHash,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}

	_, err := client.CreateTable(context.Background(), input)
	if err != nil {
		// Ignore if table already exists
		if !contains(err.Error(), "ResourceInUseException") {
			t.Fatalf("failed to create test table: %v", err)
		}
	}

	// Wait for table to be active
	waiter := dynamodb.NewTableExistsWaiter(client)
	err = waiter.Wait(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String("test_connections"),
	}, 30*time.Second)
	if err != nil {
		t.Fatalf("failed waiting for table: %v", err)
	}
}

func deleteTestTable(t *testing.T, client *dynamodb.Client, tableName string) {
	_, err := client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil && !contains(err.Error(), "ResourceNotFoundException") {
		t.Logf("failed to delete test table: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || strings.Contains(s, substr))
}
