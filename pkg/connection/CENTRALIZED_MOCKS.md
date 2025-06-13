# Centralized API Gateway Interfaces and Mocks

## Overview

All API Gateway-related interfaces and mocks have been centralized in the `pkg/connection` package to avoid conflicts and provide a consistent testing approach.

## Architecture

### 1. Interface Definition (`interfaces.go`)

```go
// APIGatewayClient defines the interface for API Gateway operations
type APIGatewayClient interface {
    PostToConnection(ctx context.Context, connectionID string, data []byte) error
    DeleteConnection(ctx context.Context, connectionID string) error
    GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error)
}
```

This interface abstracts AWS SDK operations, making the code testable and decoupled from AWS.

### 2. Production Adapter (`api_gateway_adapter.go`)

```go
// AWSAPIGatewayAdapter wraps the real AWS SDK client
type AWSAPIGatewayAdapter struct {
    client *apigatewaymanagementapi.Client
}
```

This adapter implements the `APIGatewayClient` interface using the actual AWS SDK.

### 3. Mock Implementations (`mocks.go`)

We provide two types of mocks for different testing scenarios:

#### MockAPIGatewayClient (Testify-based)
- Uses testify/mock framework
- Good for simple test cases
- Supports expectations and assertions

```go
mock := NewMockAPIGatewayClient()
mock.On("PostToConnection", ctx, "conn123", data).Return(nil)
```

#### TestableAPIGatewayClient (Manual mock)
- More control over behavior
- Can simulate latency, errors, and complex scenarios
- Maintains state (connections, messages)

```go
client := NewTestableAPIGatewayClient()
client.AddConnection("conn123", "127.0.0.1")
client.SetLatency(100 * time.Millisecond)
client.SimulateGoneError("conn456")
```

### 4. Error Types (`interfaces.go`)

Custom error types that implement the `APIError` interface:
- `GoneError` (410)
- `ForbiddenError` (403)
- `PayloadTooLargeError` (413)
- `ThrottlingError` (429)
- `InternalServerError` (500)

## Usage Examples

### Production Code

```go
// Create AWS client
awsClient := apigatewaymanagementapi.NewFromConfig(cfg)

// Wrap with adapter
adapter := connection.NewAWSAPIGatewayAdapter(awsClient)

// Use with ConnectionManager
manager := connection.NewManager(store, adapter, endpoint)
```

### Test Code

```go
// Option 1: Testify mock
mock := NewMockAPIGatewayClient()
mock.On("PostToConnection", ctx, "conn123", []byte("test")).Return(nil)
manager := connection.NewManager(store, mock, endpoint)

// Option 2: Testable client
client := NewTestableAPIGatewayClient()
client.AddConnection("conn123", "127.0.0.1")
manager := connection.NewManager(store, client, endpoint)

// Send message and verify
err := manager.Send(ctx, "conn123", message)
messages := client.GetMessages("conn123")
assert.Len(t, messages, 1)
```

## Benefits

1. **No Duplicate Types**: All mocks are in one place (`mocks.go`)
2. **Clear Interfaces**: Well-defined contracts between components
3. **Flexible Testing**: Multiple mock types for different scenarios
4. **Type Safety**: Interface compliance checks ensure mocks stay in sync
5. **AWS SDK Isolation**: Production code is decoupled from AWS SDK types

## Migration Guide

If you have existing code using the AWS SDK directly:

1. Replace `*apigatewaymanagementapi.Client` with `APIGatewayClient` interface
2. Wrap AWS clients with `NewAWSAPIGatewayAdapter()`
3. Update tests to use mocks from `mocks.go`
4. Remove any duplicate mock definitions

## Interface Compliance

All mocks are verified to implement the correct interfaces:

```go
var (
    _ ConnectionManager = (*MockConnectionManager)(nil)
    _ APIGatewayClient  = (*MockAPIGatewayClient)(nil)
    _ APIGatewayClient  = (*TestableAPIGatewayClient)(nil)
)
```

## Testing Utilities (`testing.go`)

In addition to the full mocks in `mocks.go`, we provide simplified mocks in `testing.go` that work across package boundaries:

### Package-Agnostic Mocks

1. **SendOnlyMock** - Implements only `Send` method
   - Works with `pkg/streamer`'s ConnectionManager interface
   - Simple message recording
   - Thread-safe

2. **ProgressReporterMock** - Implements `Send` + `IsActive`
   - Works with `pkg/progress`'s ConnectionManager interface
   - Connection state management
   - Customizable behavior

3. **RecordingMock** - Tracks all method calls
   - Good for verifying interaction sequences
   - Records both Send and IsActive calls

4. **FailingMock** - Always returns errors
   - Perfect for error handling tests
   - Configurable error messages

### Why Two Sets of Mocks?

- **`mocks.go`**: Full implementations of our complete interfaces (APIGatewayClient, ConnectionManager)
- **`testing.go`**: Minimal mocks that work with any package's ConnectionManager interface

This dual approach ensures:
- Maximum flexibility for consumers
- No need to import our full interface just for testing
- Easy migration from custom mocks
- Type safety across package boundaries

See [TESTING_GUIDE.md](TESTING_GUIDE.md) for detailed usage examples. 