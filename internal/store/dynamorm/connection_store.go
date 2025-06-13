package dynamorm

import (
	"context"
	"fmt"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
	"github.com/pay-theory/streamer/internal/store"
)

// connectionStore implements ConnectionStore using DynamORM
type connectionStore struct {
	db core.DB
}

// NewConnectionStore creates a new DynamORM-backed connection store
func NewConnectionStore(db core.DB) store.ConnectionStore {
	return &connectionStore{
		db: db,
	}
}

// Save creates or updates a connection
func (s *connectionStore) Save(ctx context.Context, conn *store.Connection) error {
	if err := s.validateConnection(conn); err != nil {
		return err
	}

	// Set TTL to 24 hours from now if not set
	if conn.TTL == 0 {
		conn.TTL = time.Now().Add(24 * time.Hour).Unix()
	}

	// Convert to DynamORM model
	dynamormConn := &Connection{}
	dynamormConn.FromStoreModel(conn)

	// Create or update the connection
	if err := s.db.Model(dynamormConn).Create(); err != nil {
		return store.NewStoreError("Save", dynamormConn.TableName(), conn.ConnectionID, fmt.Errorf("failed to save connection: %w", err))
	}

	return nil
}

// Get retrieves a connection by ID
func (s *connectionStore) Get(ctx context.Context, connectionID string) (*store.Connection, error) {
	if connectionID == "" {
		return nil, store.NewValidationError("connectionID", "cannot be empty")
	}

	// Create model with keys
	conn := &Connection{ConnectionID: connectionID}
	conn.SetKeys()

	// Find the connection using composite key
	if err := s.db.Model(conn).
		Where("pk", "=", conn.PK).
		Where("sk", "=", conn.SK).
		First(conn); err != nil {
		if err.Error() == "item not found" {
			return nil, store.NewStoreError("Get", conn.TableName(), connectionID, store.ErrNotFound)
		}
		return nil, store.NewStoreError("Get", conn.TableName(), connectionID, fmt.Errorf("failed to get connection: %w", err))
	}

	return conn.ToStoreModel(), nil
}

// Delete removes a connection
func (s *connectionStore) Delete(ctx context.Context, connectionID string) error {
	if connectionID == "" {
		return store.NewValidationError("connectionID", "cannot be empty")
	}

	// Create model with keys
	conn := &Connection{ConnectionID: connectionID}
	conn.SetKeys()

	// Delete the connection
	if err := s.db.Model(conn).Delete(); err != nil {
		return store.NewStoreError("Delete", conn.TableName(), connectionID, fmt.Errorf("failed to delete connection: %w", err))
	}

	return nil
}

// ListByUser returns all connections for a user
func (s *connectionStore) ListByUser(ctx context.Context, userID string) ([]*store.Connection, error) {
	if userID == "" {
		return nil, store.NewValidationError("userID", "cannot be empty")
	}

	var connections []Connection

	// Query using the user index with v1.0.9 API
	if err := s.db.Model(&Connection{}).
		Index("user-index").
		Where("user_id", "=", userID).
		All(&connections); err != nil {
		return nil, store.NewStoreError("ListByUser", store.ConnectionsTable, userID, fmt.Errorf("failed to list connections by user: %w", err))
	}

	// Convert to store models
	result := make([]*store.Connection, len(connections))
	for i := range connections {
		result[i] = connections[i].ToStoreModel()
	}

	return result, nil
}

// ListByTenant returns all connections for a tenant
func (s *connectionStore) ListByTenant(ctx context.Context, tenantID string) ([]*store.Connection, error) {
	if tenantID == "" {
		return nil, store.NewValidationError("tenantID", "cannot be empty")
	}

	var connections []Connection

	// Query using the tenant index with v1.0.9 API
	if err := s.db.Model(&Connection{}).
		Index("tenant-index").
		Where("tenant_id", "=", tenantID).
		All(&connections); err != nil {
		return nil, store.NewStoreError("ListByTenant", store.ConnectionsTable, tenantID, fmt.Errorf("failed to list connections by tenant: %w", err))
	}

	// Convert to store models
	result := make([]*store.Connection, len(connections))
	for i := range connections {
		result[i] = connections[i].ToStoreModel()
	}

	return result, nil
}

// UpdateLastPing updates the last ping timestamp
func (s *connectionStore) UpdateLastPing(ctx context.Context, connectionID string) error {
	if connectionID == "" {
		return store.NewValidationError("connectionID", "cannot be empty")
	}

	// Create model with keys
	conn := &Connection{ConnectionID: connectionID}
	conn.SetKeys()

	now := time.Now()

	// Update using DynamORM's update builder
	err := s.db.Model(conn).
		UpdateBuilder().
		Set("last_ping", now).
		Set("ttl", now.Add(24*time.Hour).Unix()).
		Execute()

	if err != nil {
		if err.Error() == "item not found" {
			return store.NewStoreError("UpdateLastPing", conn.TableName(), connectionID, store.ErrNotFound)
		}
		return store.NewStoreError("UpdateLastPing", conn.TableName(), connectionID, fmt.Errorf("failed to update last ping: %w", err))
	}

	return nil
}

// DeleteStale removes connections older than the specified time
func (s *connectionStore) DeleteStale(ctx context.Context, before time.Time) error {
	// In production, this would be handled by DynamoDB TTL
	// This method is primarily for testing and manual cleanup

	var connections []Connection

	// Scan for old connections using v1.0.9 API
	if err := s.db.Model(&Connection{}).
		Where("last_ping", "<", before).
		Scan(&connections); err != nil {
		return store.NewStoreError("DeleteStale", store.ConnectionsTable, "", fmt.Errorf("failed to scan stale connections: %w", err))
	}

	// Delete each stale connection
	deleteCount := 0
	for i := range connections {
		if err := s.Delete(ctx, connections[i].ConnectionID); err != nil {
			// Log error but continue with other deletions
			fmt.Printf("Failed to delete stale connection %s: %v\n", connections[i].ConnectionID, err)
		} else {
			deleteCount++
		}
	}

	return nil
}

// validateConnection validates a connection before saving
func (s *connectionStore) validateConnection(conn *store.Connection) error {
	if conn == nil {
		return store.NewValidationError("connection", "cannot be nil")
	}
	if conn.ConnectionID == "" {
		return store.NewValidationError("ConnectionID", "cannot be empty")
	}
	if conn.UserID == "" {
		return store.NewValidationError("UserID", "cannot be empty")
	}
	if conn.TenantID == "" {
		return store.NewValidationError("TenantID", "cannot be empty")
	}
	if conn.Endpoint == "" {
		return store.NewValidationError("Endpoint", "cannot be empty")
	}
	return nil
}
