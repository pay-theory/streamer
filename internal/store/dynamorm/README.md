# DynamORM Implementation for Streamer

This directory contains the DynamORM v1.0.9 implementation for the Streamer project's data storage layer.

## Overview

DynamORM provides a cleaner, more maintainable interface for DynamoDB operations compared to direct AWS SDK usage. This implementation maintains backward compatibility while offering improved developer experience.

## Files

- `models.go` - DynamORM model definitions with PK/SK pattern
- `connection_store.go` - ConnectionStore interface implementation using DynamORM
- `factory.go` - Factory pattern for creating DynamORM stores
- `connection_store_test.go` - Unit tests for the connection store

## Key Features

### 1. PK/SK Pattern
DynamORM uses a composite key pattern for better query flexibility:
- PK: `CONN#<ConnectionID>`
- SK: `METADATA`

This allows for future extensibility and efficient queries.

### 2. Automatic Index Management
Indexes are defined using struct tags:
```go
UserID   string `dynamorm:"user_id" dynamorm-index:"user-index,pk"`
TenantID string `dynamorm:"tenant_id" dynamorm-index:"tenant-index,pk"`
```

### 3. Simplified Operations
Compare the difference:

**Before (AWS SDK)**:
```go
input := &dynamodb.PutItemInput{
    TableName: aws.String(s.tableName),
    Item:      item,
}
_, err = s.client.PutItem(ctx, input)
```

**After (DynamORM)**:
```go
err := s.db.Model(dynamormConn).Create()
```

## Usage

### Enable DynamORM in Lambda Functions

Set the environment variable:
```bash
USE_DYNAMORM=true
```

### Initialize in Code
```go
// Create DynamORM factory
factory, err := dynamorm.NewStoreFactory(session.Config{
    Region: "us-east-1",
})

// Get connection store
connStore := factory.ConnectionStore()
```

## Migration Status

- ✅ Connection Store implemented
- ⏳ Request Queue (TODO)
- ⏳ Subscription Store (TODO)

## Testing

Run tests with:
```bash
go test ./internal/store/dynamorm/...
```

## Performance Considerations

- DynamORM is optimized for Lambda cold starts
- Connection pooling is handled automatically
- Built-in retry logic with exponential backoff

## Next Steps

1. Implement RequestQueue and SubscriptionStore
2. Add integration tests with DynamoDB Local
3. Create migration scripts for production data
4. Update deployment infrastructure 