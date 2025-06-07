# Connection Manager Package

The `connection` package provides WebSocket connection management for the Streamer system. It handles sending messages to connected clients via AWS API Gateway Management API.

## Overview

The ConnectionManager is responsible for:
- Sending messages to individual WebSocket connections
- Broadcasting messages to multiple connections
- Checking connection health
- Automatic cleanup of stale connections
- Connection caching for performance

## Usage

### Basic Setup

```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
    "github.com/pay-theory/streamer/internal/store"
    "github.com/pay-theory/streamer/pkg/connection"
)

// Initialize AWS config
cfg, err := config.LoadDefaultConfig(ctx)
if err != nil {
    return err
}

// Create API Gateway Management API client
// The endpoint should be your WebSocket API endpoint
endpoint := "https://your-api-id.execute-api.region.amazonaws.com/stage"
apiClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
    o.EndpointResolver = apigatewaymanagementapi.EndpointResolverFunc(
        func(region string, options apigatewaymanagementapi.EndpointResolverOptions) (aws.Endpoint, error) {
            return aws.Endpoint{
                URL: endpoint,
            }, nil
        })
})

// Create connection manager
connManager := connection.NewManager(connectionStore, apiClient, endpoint)
```

### Sending Messages

```go
// Send to a single connection
message := map[string]interface{}{
    "type": "progress",
    "request_id": "req_123",
    "percentage": 45.5,
    "message": "Processing...",
}

err := connManager.Send(ctx, connectionID, message)
if err != nil {
    if connection.IsConnectionGone(err) {
        // Connection no longer exists
        log.Printf("Connection %s is gone", connectionID)
    } else {
        // Other error
        return err
    }
}
```

### Broadcasting Messages

```go
// Broadcast to multiple connections
connectionIDs := []string{"conn1", "conn2", "conn3"}
notification := map[string]interface{}{
    "type": "announcement",
    "message": "System update in 5 minutes",
}

err := connManager.Broadcast(ctx, connectionIDs, notification)
if err != nil {
    // Broadcast error contains details about failed connections
    var broadcastErr *connection.BroadcastError
    if errors.As(err, &broadcastErr) {
        log.Printf("Broadcast failed for %d connections", len(broadcastErr.Errors))
    }
}
```

### Checking Connection Status

```go
// Check if a connection is active
if connManager.IsActive(ctx, connectionID) {
    // Connection is active
} else {
    // Connection is inactive or gone
}
```

## Error Handling

The package provides specific error types:

- `ConnectionGoneError`: Indicates the WebSocket connection no longer exists (410 Gone)
- `BroadcastError`: Contains errors from failed broadcast attempts

Use the helper function to check for specific errors:

```go
if connection.IsConnectionGone(err) {
    // Handle gone connection
}
```

## Performance Considerations

1. **Connection Caching**: The manager caches connection status for 30 seconds to reduce DynamoDB lookups
2. **Parallel Broadcasts**: Broadcasts are sent in parallel with a concurrency limit of 50
3. **Automatic Cleanup**: Gone connections are automatically removed from DynamoDB
4. **Async Operations**: Last ping updates and cleanup operations run asynchronously

## Testing

The package includes comprehensive unit tests with mocked dependencies:

```go
// Create mocks for testing
mockStore := new(MockConnectionStore)
mockAPI := new(MockAPIGatewayClient)

// Set up expectations
mockStore.On("Get", mock.Anything, "conn-123").Return(conn, nil)
mockAPI.On("PostToConnection", mock.Anything, mock.Anything).Return(output, nil)

// Create manager with mocks
manager := NewManager(mockStore, mockAPI, endpoint)
```

## Integration with Team 2

Team 2 can use this ConnectionManager in their router and async processor:

### In Router Lambda
```go
// When sending sync responses
response := map[string]interface{}{
    "type": "response",
    "request_id": request.ID,
    "data": result,
}
err = connManager.Send(ctx, request.ConnectionID, response)
```

### In Async Processor
```go
// When sending progress updates
progressReporter := &progressReporter{
    connManager: connManager,
    connectionID: request.ConnectionID,
    requestID: request.ID,
}

// Use progress reporter during processing
progressReporter.Report(25.0, "Processing started")
```

## Environment Variables

- `WEBSOCKET_ENDPOINT`: The WebSocket API endpoint URL
- `AWS_REGION`: AWS region for the services

## Dependencies

- AWS SDK v2 for Go
- API Gateway Management API client
- Internal store package (from Team 1) 