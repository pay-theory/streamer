# API Gateway Mocking Guide for Streamer

## Overview

This guide shows how to use streamer's API Gateway mocking utilities to test WebSocket functionality without AWS dependencies, similar to how DynamORM provides DynamoDB mocking.

## The Interface Mismatch Problem (Now Fixed!)

Previously, there was a mismatch between AWS SDK v2's API Gateway client and streamer's simpler interface:

```go
// AWS SDK expects:
DeleteConnection(context.Context, *apigatewaymanagementapi.DeleteConnectionInput, ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.DeleteConnectionOutput, error)

// Streamer's interface expects:
DeleteConnection(ctx context.Context, connectionID string) error
```

**Solution**: Use the `AWSAPIGatewayAdapter` to bridge the gap:

```go
// Wrap the AWS SDK client
apiGatewayAdapter := connection.NewAWSAPIGatewayAdapter(apiGatewayClient)

// Use the adapter with ConnectionManager
connManager := connection.NewManager(connStore, apiGatewayAdapter, endpoint)
```

## Available Mocks

### 1. MockAPIGatewayClient (Testify-based)

Best for simple test scenarios with testify:

```go
func TestSendMessage(t *testing.T) {
    // Create mock
    mockAPI := connection.NewMockAPIGatewayClient()
    
    // Set up expectations
    mockAPI.On("PostToConnection", mock.Anything, "conn-123", []byte(`{"message":"hello"}`)).
        Return(nil)
    
    // Use in ConnectionManager
    manager := connection.NewManager(mockStore, mockAPI, "wss://test.com")
    
    // Test your code
    err := manager.Send(ctx, "conn-123", map[string]string{"message": "hello"})
    assert.NoError(t, err)
    
    // Verify
    mockAPI.AssertExpectations(t)
}
```

### 2. TestableAPIGatewayClient (Behavior-based)

Best for complex scenarios and error simulation:

```go
func TestErrorHandling(t *testing.T) {
    // Create testable client
    testAPI := connection.NewTestableAPIGatewayClient()
    
    // Add a connection
    testAPI.AddConnection("conn-123", "192.168.1.1")
    
    // Simulate errors
    testAPI.SimulateGoneError("conn-456")           // 410 Gone
    testAPI.SimulateThrottling("conn-789", 5)       // 429 Rate Limited
    testAPI.SetLatency(100 * time.Millisecond)      // Network latency
    
    manager := connection.NewManager(mockStore, testAPI, "wss://test.com")
    
    // Test error handling
    err := manager.Send(ctx, "conn-456", "test")
    assert.True(t, errors.Is(err, connection.ErrConnectionStale))
    
    // Verify messages were stored
    messages := testAPI.GetMessages("conn-123")
    assert.Len(t, messages, 1)
}
```

## Testing Patterns

### Pattern 1: Unit Testing Handlers

```go
func TestWebSocketHandler(t *testing.T) {
    // Setup
    mockStore := store.NewMockConnectionStore()
    mockAPI := connection.NewMockAPIGatewayClient()
    
    // Configure mock behavior
    mockAPI.On("PostToConnection", mock.Anything, mock.Anything, mock.Anything).
        Return(nil)
    
    // Create manager
    manager := connection.NewManager(mockStore, mockAPI, "wss://test.com")
    
    // Create your handler with the manager
    handler := NewMessageHandler(manager)
    
    // Test the handler
    err := handler.Process(ctx, &Request{
        ConnectionID: "conn-123",
        Message:      "Hello",
    })
    
    assert.NoError(t, err)
    mockAPI.AssertExpectations(t)
}
```

### Pattern 2: Integration Testing

```go
func TestRouterIntegration(t *testing.T) {
    // Use TestableAPIGatewayClient for more control
    testAPI := connection.NewTestableAPIGatewayClient()
    
    // Pre-populate connections
    testAPI.AddConnection("user-1", "10.0.0.1")
    testAPI.AddConnection("user-2", "10.0.0.2")
    
    // Create router with mocked dependencies
    router := streamer.NewRouter(mockQueue, 
        connection.NewManager(mockStore, testAPI, "wss://test.com"))
    
    // Test broadcast functionality
    err := router.Broadcast(ctx, []string{"user-1", "user-2"}, 
        &BroadcastMessage{Text: "System update"})
    
    assert.NoError(t, err)
    
    // Verify both users received the message
    assert.Equal(t, 1, testAPI.GetMessageCount("user-1"))
    assert.Equal(t, 1, testAPI.GetMessageCount("user-2"))
}
```

### Pattern 3: Error Scenario Testing

```go
func TestConnectionErrors(t *testing.T) {
    tests := []struct {
        name          string
        setupMock     func(*connection.TestableAPIGatewayClient)
        expectedError error
    }{
        {
            name: "connection gone",
            setupMock: func(api *connection.TestableAPIGatewayClient) {
                api.SimulateGoneError("conn-123")
            },
            expectedError: connection.ErrConnectionStale,
        },
        {
            name: "rate limited",
            setupMock: func(api *connection.TestableAPIGatewayClient) {
                api.SimulateThrottling("conn-123", 5)
            },
            expectedError: connection.ErrThrottled,
        },
        {
            name: "payload too large",
            setupMock: func(api *connection.TestableAPIGatewayClient) {
                api.SetError("conn-123", connection.PayloadTooLargeError{
                    ConnectionID: "conn-123",
                    PayloadSize:  40000,
                    MaxSize:      32768,
                })
            },
            expectedError: connection.ErrPayloadTooLarge,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            testAPI := connection.NewTestableAPIGatewayClient()
            testAPI.AddConnection("conn-123", "127.0.0.1")
            tt.setupMock(testAPI)
            
            manager := connection.NewManager(mockStore, testAPI, "wss://test.com")
            
            err := manager.Send(ctx, "conn-123", "test message")
            assert.True(t, errors.Is(err, tt.expectedError))
        })
    }
}
```

## Best Practices

### 1. Use the Right Mock for Your Needs

- **MockAPIGatewayClient**: Simple testify-based assertions
- **TestableAPIGatewayClient**: Complex scenarios, error simulation, latency testing

### 2. Always Use the Adapter in Production Code

```go
// ✅ Correct
apiGatewayAdapter := connection.NewAWSAPIGatewayAdapter(apiGatewayClient)
manager := connection.NewManager(store, apiGatewayAdapter, endpoint)

// ❌ Wrong (causes interface mismatch)
manager := connection.NewManager(store, apiGatewayClient, endpoint)
```

### 3. Test Message Serialization

```go
func TestMessageSerialization(t *testing.T) {
    testAPI := connection.NewTestableAPIGatewayClient()
    testAPI.AddConnection("conn-123", "127.0.0.1")
    
    manager := connection.NewManager(mockStore, testAPI, "wss://test.com")
    
    // Send structured message
    msg := Message{
        Type: "notification",
        Data: map[string]interface{}{
            "title": "Test",
            "body":  "Hello World",
        },
    }
    
    err := manager.Send(ctx, "conn-123", msg)
    assert.NoError(t, err)
    
    // Verify JSON serialization
    messages := testAPI.GetMessages("conn-123")
    require.Len(t, messages, 1)
    
    var received Message
    err = json.Unmarshal(messages[0], &received)
    assert.NoError(t, err)
    assert.Equal(t, msg.Type, received.Type)
}
```

### 4. Test Concurrent Operations

```go
func TestConcurrentSends(t *testing.T) {
    testAPI := connection.NewTestableAPIGatewayClient()
    
    // Add multiple connections
    for i := 0; i < 10; i++ {
        testAPI.AddConnection(fmt.Sprintf("conn-%d", i), "127.0.0.1")
    }
    
    manager := connection.NewManager(mockStore, testAPI, "wss://test.com")
    
    // Send messages concurrently
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            err := manager.Send(ctx, fmt.Sprintf("conn-%d", id), "test")
            assert.NoError(t, err)
        }(i)
    }
    
    wg.Wait()
    
    // Verify all messages sent
    for i := 0; i < 10; i++ {
        assert.Equal(t, 1, testAPI.GetMessageCount(fmt.Sprintf("conn-%d", i)))
    }
}
```

## Migration from Direct AWS SDK Usage

If you have existing code using the AWS SDK directly:

```go
// Before
func sendMessage(client *apigatewaymanagementapi.Client, connID string, data []byte) error {
    _, err := client.PostToConnection(context.Background(), 
        &apigatewaymanagementapi.PostToConnectionInput{
            ConnectionId: &connID,
            Data:         data,
        })
    return err
}

// After - Production
func sendMessage(manager connection.ConnectionManager, connID string, data interface{}) error {
    return manager.Send(context.Background(), connID, data)
}

// After - Testing
func TestSendMessage(t *testing.T) {
    mockAPI := connection.NewTestableAPIGatewayClient()
    mockAPI.AddConnection("conn-123", "127.0.0.1")
    
    manager := connection.NewManager(mockStore, mockAPI, "wss://test.com")
    
    err := sendMessage(manager, "conn-123", "test data")
    assert.NoError(t, err)
    
    messages := mockAPI.GetMessages("conn-123")
    assert.Len(t, messages, 1)
}
```

## Summary

By using the adapter pattern and provided mocks, streamer offers:

1. **Clean abstraction** over AWS API Gateway Management API
2. **Easy testing** without AWS dependencies
3. **Error simulation** for robust error handling tests
4. **Type safety** with Go interfaces
5. **DynamORM-style mocking** for API Gateway

This approach enables high test coverage while keeping tests fast and deterministic. 