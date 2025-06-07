# Streamer Storage Layer

This package implements the storage layer for the Streamer project using AWS DynamoDB.

## Overview

The storage layer provides interfaces and implementations for managing:
- WebSocket connections
- Async request queuing
- Real-time subscriptions

## Components

### Models (`models.go`)

- **Connection**: Represents a WebSocket connection with user/tenant information
- **AsyncRequest**: Represents a queued request for async processing
- **Subscription**: Represents a real-time update subscription

### Interfaces (`interfaces.go`)

- **ConnectionStore**: Manages WebSocket connections
  - Save, Get, Delete connections
  - Query by user or tenant
  - Update activity timestamps
  - Clean up stale connections

- **RequestQueue**: Manages async requests
  - Enqueue new requests
  - Update status and progress
  - Query by connection or status
  - Complete or fail requests

- **SubscriptionStore**: Manages real-time subscriptions
  - Subscribe/unsubscribe to updates
  - Query by connection or request

### Implementations

- **connectionStore** (`connection_store.go`): DynamoDB implementation of ConnectionStore
- **requestQueue** (`request_queue.go`): DynamoDB implementation of RequestQueue

### Table Definitions (`migrations.go`)

Defines DynamoDB table schemas with:
- Primary keys and indexes
- TTL configuration for automatic cleanup
- Pay-per-request billing mode

#### Tables

1. **streamer_connections**
   - Primary Key: ConnectionID
   - GSI: UserIndex (UserID)
   - GSI: TenantIndex (TenantID)
   - TTL: 24 hours

2. **streamer_requests**
   - Primary Key: RequestID
   - GSI: ConnectionIndex (ConnectionID, CreatedAt)
   - GSI: StatusIndex (Status, CreatedAt)
   - TTL: 7 days

3. **streamer_subscriptions**
   - Primary Key: SubscriptionID
   - GSI: ConnectionIndex (ConnectionID)
   - GSI: RequestIndex (RequestID)
   - TTL: 24 hours

## Usage

```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/streamer/streamer/internal/store"
)

// Create DynamoDB client
cfg, _ := config.LoadDefaultConfig(context.Background())
client := dynamodb.NewFromConfig(cfg)

// Create tables (one-time setup)
err := store.CreateTables(context.Background(), client)

// Create stores
connStore := store.NewConnectionStore(client, "")
requestQueue := store.NewRequestQueue(client, "")

// Save a connection
conn := &store.Connection{
    ConnectionID: "conn-123",
    UserID:       "user-456",
    TenantID:     "tenant-789",
    Endpoint:     "wss://example.com/ws",
    ConnectedAt:  time.Now(),
    LastPing:     time.Now(),
}
err = connStore.Save(context.Background(), conn)

// Enqueue a request
req := &store.AsyncRequest{
    ConnectionID: "conn-123",
    Action:       "generate_report",
    Payload:      map[string]interface{}{"type": "monthly"},
}
err = requestQueue.Enqueue(context.Background(), req)
```

## Testing

The storage layer includes comprehensive unit tests that can be run against a local DynamoDB instance:

```bash
# Start local DynamoDB
docker run -p 8000:8000 amazon/dynamodb-local

# Run tests
go test ./internal/store/...

# Run integration tests
go test ./internal/store/... -run Integration
```

## Error Handling

The storage layer defines custom error types:
- `ErrNotFound`: Item not found in DynamoDB
- `ErrAlreadyExists`: Item already exists
- `ErrInvalidInput`: Validation failed
- `ValidationError`: Field-specific validation errors

Use the helper functions to check error types:
```go
if store.IsNotFound(err) {
    // Handle not found
}
```

## Performance Considerations

- Uses DynamoDB batch operations where possible
- Implements pagination for large result sets
- Optimized indexes for common query patterns
- TTL for automatic cleanup of old data
- Pay-per-request billing for cost efficiency

## Future Improvements

- [ ] Add caching layer for frequently accessed items
- [ ] Implement batch operations for bulk updates
- [ ] Add metrics and monitoring
- [ ] Support for DynamoDB transactions
- [ ] Backup and restore functionality 