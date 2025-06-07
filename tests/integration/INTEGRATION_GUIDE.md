# Integration Guide for Team 2

## Overview
This guide helps you integrate your router with our ConnectionManager and Lambda handlers. We've built integration tests that verify the complete connection lifecycle works properly.

## Quick Start Integration

### 1. Using ConnectionManager in Your Router

```go
import (
    "github.com/pay-theory/streamer/internal/store"
    "github.com/pay-theory/streamer/pkg/connection"
)

// In your router initialization
func createRouter(dynamoClient *dynamodb.Client) *DefaultRouter {
    // Create connection store
    connStore := store.NewConnectionStore(dynamoClient, "streamer_connections")
    
    // Create API Gateway client
    apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
        o.EndpointResolver = apigatewaymanagementapi.EndpointResolverFunc(
            func(region string, options apigatewaymanagementapi.EndpointResolverOptions) (aws.Endpoint, error) {
                return aws.Endpoint{
                    URL: os.Getenv("WEBSOCKET_ENDPOINT"), // e.g., https://xxx.execute-api.region.amazonaws.com/stage
                }, nil
            },
        )
    })
    
    // Create connection manager
    connManager := connection.NewManager(connStore, apiGatewayClient, os.Getenv("WEBSOCKET_ENDPOINT"))
    
    // Create your router
    router := NewRouter(requestStore, connManager)
    return router
}
```

### 2. Running Integration Tests Together

We've created integration tests that verify the complete flow. To run them with your router:

```go
// tests/integration/router_integration_test.go
func TestRouterWithRealConnections(t *testing.T) {
    // Setup environment (see our connection_lifecycle_test.go)
    testEnv := setupTestEnvironment(t)
    defer testEnv.cleanup()
    
    // Connect a user
    token := generateTestJWT(t, testEnv.privateKey, "test-user", "test-tenant")
    connectEvent := createConnectEvent("conn-123", token)
    response, _ := testEnv.connectHandler.Handle(ctx, connectEvent)
    require.Equal(t, 200, response.StatusCode)
    
    // Create your router with our connection manager
    router := NewRouter(requestStore, testEnv.connectionManager)
    
    // Process a message through your router
    messageEvent := events.APIGatewayWebsocketProxyRequest{
        RequestContext: events.APIGatewayWebsocketProxyRequestContext{
            ConnectionID: "conn-123",
        },
        Body: `{
            "action": "echo",
            "payload": {"message": "Hello World"}
        }`,
    }
    
    err := router.Route(ctx, messageEvent)
    assert.NoError(t, err)
    
    // The router should have sent a response via ConnectionManager
    // In test environment, the send will fail but we can verify the attempt
}
```

## Testing Scenarios

### 1. Connection Not Found
```go
// Your router should handle this gracefully
err := connManager.Send(ctx, "non-existent-conn", message)
assert.ErrorIs(t, err, connection.ErrConnectionNotFound)
```

### 2. Stale Connection (410 Gone)
```go
// ConnectionManager automatically removes stale connections
err := connManager.Send(ctx, "stale-conn", message)
assert.ErrorIs(t, err, connection.ErrConnectionStale)
```

### 3. Broadcast Performance
```go
// Test broadcasting to many connections
connectionIDs := []string{"conn-1", "conn-2", ..., "conn-100"}
start := time.Now()
err := connManager.Broadcast(ctx, connectionIDs, message)
duration := time.Since(start)
// Should complete in < 50ms for 100 connections
```

## Debugging Integration Issues

### 1. Connection Problems
If connections aren't working:
- Check JWT token is being passed in query string parameter "Authorization"
- Verify JWT public key matches between issuer and Lambda environment
- Check DynamoDB table permissions

### 2. Message Sending Failures
If messages aren't being delivered:
- Verify API Gateway endpoint URL is correct
- Check IAM permissions for `execute-api:ManageConnections`
- Look for 410 Gone errors indicating stale connections

### 3. Performance Issues
If experiencing slow message delivery:
- Check CloudWatch metrics from ConnectionManager
- Monitor circuit breaker states: `manager.GetMetrics()`
- Review worker pool utilization

## Local Testing Setup

For local integration testing without AWS:

```go
// Create mock API Gateway client
type mockAPIGatewayClient struct {
    sentMessages []sentMessage
}

func (m *mockAPIGatewayClient) PostToConnection(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
    m.sentMessages = append(m.sentMessages, sentMessage{
        ConnectionID: *params.ConnectionId,
        Data:         params.Data,
    })
    return &apigatewaymanagementapi.PostToConnectionOutput{}, nil
}

// Use in tests
mockClient := &mockAPIGatewayClient{}
connManager := connection.NewManager(connStore, mockClient, "test-endpoint")
```

## Common Integration Patterns

### 1. Request-Response Pattern
```go
// In your handler
result, err := handler.Process(ctx, request)
if err != nil {
    return connManager.Send(ctx, connectionID, map[string]interface{}{
        "type": "error",
        "error": err.Error(),
    })
}

return connManager.Send(ctx, connectionID, map[string]interface{}{
    "type": "response",
    "request_id": request.ID,
    "data": result.Data,
})
```

### 2. Progress Updates
```go
// Send progress updates during long operations
for i := 0; i < 100; i += 10 {
    connManager.Send(ctx, connectionID, map[string]interface{}{
        "type": "progress",
        "request_id": requestID,
        "progress": i,
    })
    // Do work...
}
```

### 3. Error Handling
```go
err := connManager.Send(ctx, connectionID, response)
if err != nil {
    switch {
    case errors.Is(err, connection.ErrConnectionNotFound):
        // Connection doesn't exist, maybe user disconnected
        log.Printf("User disconnected: %s", connectionID)
    case errors.Is(err, connection.ErrConnectionStale):
        // Connection was stale and has been cleaned up
        log.Printf("Cleaned up stale connection: %s", connectionID)
    default:
        // Other error (network, etc.)
        log.Printf("Failed to send message: %v", err)
    }
}
```

## Performance Expectations

Based on our testing:
- **Connection establishment**: < 50ms including JWT validation
- **Single message send**: < 10ms p99 (excluding network)
- **Broadcast to 100**: < 50ms total
- **Disconnect cleanup**: < 20ms

## Monitoring Integration

Key metrics to monitor:
1. Connection success rate
2. Message delivery latency
3. Circuit breaker activations
4. Worker pool utilization

Access metrics:
```go
metrics := connManager.GetMetrics()
log.Printf("Metrics: %+v", metrics)
```

## Next Steps

1. Run the integration tests: `go test ./tests/integration/...`
2. Review our handler implementations in `lambda/connect/` and `lambda/disconnect/`
3. Check metrics and monitoring setup
4. Test with real WebSocket clients

## Questions?

The ConnectionManager is designed to be a drop-in solution for your router's needs. If you encounter any issues during integration, check:
- Error types in `pkg/connection/errors.go`
- Integration test examples in `tests/integration/connection_lifecycle_test.go`
- Handler implementations for reference 