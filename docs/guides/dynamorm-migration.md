# DynamORM Migration Guide for Streamer

This guide documents the complete migration of the Streamer project from direct AWS SDK DynamoDB usage to DynamORM v1.0.9.

## Overview

DynamORM provides a more developer-friendly interface for DynamoDB operations while maintaining high performance. Since Streamer is preparing for its first integration with no existing users, we performed a complete migration rather than a gradual one.

## Key Benefits

1. **Simplified API**: Less boilerplate code for common operations
2. **Type Safety**: Better compile-time checks
3. **Built-in Best Practices**: Automatic PK/SK pattern implementation
4. **Lambda Optimized**: Designed for serverless environments

## Migration Steps

### 1. Add DynamORM Dependency

```bash
go get github.com/pay-theory/dynamorm@v1.0.9
```

### 2. Update Models

The models have been updated to use DynamORM tags and implement the PK/SK pattern:

```go
// Before: internal/store/models.go
type Connection struct {
    ConnectionID string `dynamodbav:"ConnectionID"`
    UserID       string `dynamodbav:"UserID"`
    // ...
}

// After: internal/store/dynamorm/models.go
type Connection struct {
    PK string `dynamorm:"pk"`
    SK string `dynamorm:"sk"`
    
    ConnectionID string `dynamorm:"connection_id"`
    UserID       string `dynamorm:"user_id" dynamorm-index:"user-index,pk"`
    // ...
}
```

### 3. Update Lambda Handlers

All Lambda handlers have been updated to use DynamORM:

```go
// lambda/connect/main.go - DynamORM version
func main() {
    // Load configuration
    cfg := &HandlerConfig{
        TableName:      getEnv("CONNECTIONS_TABLE", "streamer_connections"),
        JWTPublicKey:   getEnv("JWT_PUBLIC_KEY", ""),
        JWTIssuer:      getEnv("JWT_ISSUER", ""),
        AllowedTenants: getEnvSlice("ALLOWED_TENANTS", []string{}),
        LogLevel:       getEnv("LOG_LEVEL", "INFO"),
    }

    // Initialize AWS SDK for CloudWatch metrics
    awsCfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        log.Fatalf("Failed to load AWS config: %v", err)
    }

    // Initialize DynamORM
    dynamormConfig := session.Config{
        Region: awsCfg.Region,
    }
    
    factory, err := dynamormStore.NewStoreFactory(dynamormConfig)
    if err != nil {
        log.Fatalf("Failed to create DynamORM factory: %v", err)
    }

    // Get connection store from factory
    connStore := factory.ConnectionStore()

    // Create CloudWatch metrics client
    metricsNamespace := getEnv("METRICS_NAMESPACE", "Streamer")
    metrics := shared.NewCloudWatchMetrics(awsCfg, metricsNamespace)

    // Create handler
    handler := NewHandler(connStore, cfg, metrics)

    // Start Lambda runtime
    lambda.Start(handler.Handle)
}
```

### 4. Environment Variables

The following environment variables are used:

```bash
# AWS Region (automatically detected from Lambda environment)
AWS_REGION=us-east-1

# Table names
CONNECTIONS_TABLE=streamer_connections
REQUESTS_TABLE=streamer_requests
SUBSCRIPTIONS_TABLE=streamer_subscriptions
```

### 5. Table Schema Changes

DynamORM uses a PK/SK pattern for better query flexibility. The migration maintains backward compatibility:

#### Original Schema
- Primary Key: `ConnectionID`
- GSI: UserIndex (UserID), TenantIndex (TenantID)

#### DynamORM Schema
- Primary Key: `pk` (CONN#<ConnectionID>), `sk` (METADATA)
- GSI: user-index (user_id), tenant-index (tenant_id)

### 6. Testing

Since this is a complete migration:

1. **Unit Tests**: All test files use DynamORM models
2. **Integration Tests**: Test with DynamoDB Local using DynamORM
3. **Deployment**: Deploy all Lambda functions with DynamORM

## Performance Considerations

DynamORM is optimized for Lambda environments:
- Connection pooling is handled automatically
- Minimal cold start impact
- Built-in retry logic

## Common Issues and Solutions

### Issue: Type Assertion Errors
**Solution**: DynamORM returns `core.ExtendedDB` which needs to be type-asserted to `*dynamorm.DB`.

### Issue: Index Names Different
**Solution**: Update your queries to use the new index names (e.g., "user-index" instead of "UserIndex").

### Issue: Missing TTL
**Solution**: DynamORM handles TTL automatically if configured in the table schema.

## Migration Status

- ✅ **Connection Store**: Fully migrated to DynamORM
- ✅ **Lambda Handlers**: All handlers updated (connect, disconnect, router)
- ✅ **Models**: DynamORM models with PK/SK pattern implemented
- ✅ **Request Queue**: Basic implementation completed (needs testing)
- ⏳ **Subscription Store**: Implementation pending
- ✅ **Factory Pattern**: Centralized store creation with DynamORM

## Next Steps

1. Implement RequestQueue and SubscriptionStore interfaces with DynamORM
2. Update deployment infrastructure (Pulumi/CDK) to create DynamORM-compatible tables
3. Add integration tests with DynamoDB Local
4. Deploy to development environment for testing

## DynamORM v1.0.9 API Reference

### Query Methods

```go
// Get single item by composite key
db.Model(&Item{}).
    Where("pk", "=", pkValue).
    Where("sk", "=", skValue).
    First(&item)

// Query with GSI
db.Model(&Item{}).
    Index("user-index").
    Where("user_id", "=", userID).
    All(&items)

// Scan with filter
db.Model(&Item{}).
    Where("created_at", "<", yesterday).
    Scan(&items)
```

### Valid Operators

- `"="` - Equality
- `"!="` - Not equal  
- `">"`, `"<"`, `">=", "<="` - Comparisons
- `"BEGINS_WITH"` - String prefix
- `"BETWEEN"` - Range queries
- `"IN"` - Value in list
- `"EXISTS"`, `"NOT_EXISTS"` - Attribute existence

## Resources

- [DynamORM Documentation](https://github.com/pay-theory/dynamorm)
- [AWS DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [Lambda Optimization Guide](https://docs.aws.amazon.com/lambda/latest/dg/best-practices.html) 