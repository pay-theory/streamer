# Integration Guide - Week 2

## Quick Start Integration

Based on the completed work from Week 1, here's how to integrate the components:

## For Team 2: Using Team 1's Storage Layer

### 1. Import the Storage Package
```go
import (
    "github.com/pay-theory/streamer/internal/store"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)
```

### 2. Initialize Storage Components
```go
// In your router initialization
cfg, err := config.LoadDefaultConfig(ctx)
if err != nil {
    return nil, err
}

dynamoClient := dynamodb.NewFromConfig(cfg)

// Create storage instances
connectionStore := store.NewConnectionStore(dynamoClient, "")
requestQueue := store.NewRequestQueue(dynamoClient, "")
```

### 3. Update Router to Use RequestQueue
```go
// In router.go, update the RequestStore interface to match
type RequestStore interface {
    Enqueue(ctx context.Context, request *Request) error
}

// Create an adapter if needed
type requestQueueAdapter struct {
    queue *store.RequestQueue
}

func (a *requestQueueAdapter) Enqueue(ctx context.Context, request *Request) error {
    // Convert router.Request to store.AsyncRequest
    asyncReq := &store.AsyncRequest{
        RequestID:    request.ID,
        ConnectionID: request.ConnectionID,
        Action:       request.Action,
        Status:       store.StatusPending,
        Payload:      request.Payload,
        CreatedAt:    request.CreatedAt,
        Metadata:     request.Metadata,
    }
    return a.queue.Enqueue(ctx, asyncReq)
}
```

## For Team 1: Implementing ConnectionManager

### 1. Use Team 2's Interfaces
```go
// Implement the ConnectionManager interface that Team 2 expects
package connection

import (
    "github.com/pay-theory/streamer/internal/store"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

type Manager struct {
    store       *store.ConnectionStore
    apiGateway  *apigatewaymanagementapi.Client
    endpoint    string
}

func (m *Manager) Send(ctx context.Context, connectionID string, message interface{}) error {
    // Check if connection exists and is active
    conn, err := m.store.Get(ctx, connectionID)
    if err != nil {
        return fmt.Errorf("connection not found: %w", err)
    }
    
    // Marshal message
    data, err := json.Marshal(message)
    if err != nil {
        return err
    }
    
    // Send via API Gateway Management API
    _, err = m.apiGateway.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
        ConnectionId: &connectionID,
        Data:         data,
    })
    
    return err
}
```

## Shared Types & Constants

### 1. Create a Shared Types Package
```go
// pkg/types/types.go
package types

// Message types for WebSocket communication
const (
    MessageTypeRequest       = "request"
    MessageTypeResponse      = "response"
    MessageTypeAcknowledgment = "acknowledgment"
    MessageTypeProgress      = "progress"
    MessageTypeError         = "error"
    MessageTypeComplete      = "complete"
)

// Standard error codes
const (
    ErrCodeValidation    = "VALIDATION_ERROR"
    ErrCodeInternalError = "INTERNAL_ERROR"
    ErrCodeUnauthorized  = "UNAUTHORIZED"
    ErrCodeNotFound      = "NOT_FOUND"
)
```

### 2. WebSocket Message Format
```go
// Standard WebSocket message structure
type WebSocketMessage struct {
    Type      string                 `json:"type"`
    RequestID string                 `json:"request_id,omitempty"`
    Data      interface{}            `json:"data,omitempty"`
    Error     *Error                 `json:"error,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
```

## Integration Test Example

### End-to-End Flow Test
```go
// tests/integration/e2e_test.go
func TestEndToEndFlow(t *testing.T) {
    ctx := context.Background()
    
    // 1. Initialize all components
    connStore := store.NewConnectionStore(dynamoClient, testTablePrefix)
    reqQueue := store.NewRequestQueue(dynamoClient, testTablePrefix)
    connManager := connection.NewManager(connStore, apiGatewayClient)
    
    router := streamer.NewRouter(
        &requestQueueAdapter{queue: reqQueue},
        connManager,
    )
    
    // 2. Register a test handler
    router.Handle("test_action", streamer.NewEchoHandler())
    
    // 3. Simulate connection
    conn := &store.Connection{
        ConnectionID: "test-conn-123",
        UserID:       "user-456",
        TenantID:     "tenant-789",
    }
    err := connStore.Save(ctx, conn)
    require.NoError(t, err)
    
    // 4. Send request through router
    event := events.APIGatewayWebsocketProxyRequest{
        RequestContext: events.APIGatewayWebsocketProxyRequestContext{
            ConnectionID: conn.ConnectionID,
        },
        Body: `{"action":"test_action","payload":{"message":"hello"}}`,
    }
    
    err = router.Route(ctx, event)
    require.NoError(t, err)
    
    // 5. Verify response was sent
    // (Would need to mock or capture WebSocket sends)
}
```

## Common Integration Issues & Solutions

### 1. Type Mismatches
**Problem**: Router Request vs AsyncRequest types
**Solution**: Create adapters or update interfaces to use common types

### 2. Missing Dependencies
**Problem**: ConnectionManager not available for Team 2
**Solution**: Create a mock implementation for testing:
```go
type mockConnectionManager struct {
    sentMessages []sentMessage
}

type sentMessage struct {
    connectionID string
    message      interface{}
}

func (m *mockConnectionManager) Send(ctx context.Context, connectionID string, message interface{}) error {
    m.sentMessages = append(m.sentMessages, sentMessage{connectionID, message})
    return nil
}
```

### 3. Configuration Differences
**Problem**: Different AWS clients or table names
**Solution**: Use environment variables and configuration struct:
```go
type Config struct {
    TablePrefix string
    AWSRegion   string
    WSEndpoint  string
}
```

## Testing Strategy

### 1. Unit Tests (Isolated)
- Team 1: Test storage layer with mocked DynamoDB
- Team 2: Test router with mocked storage and connection manager

### 2. Integration Tests (Connected)
- Test storage + router integration
- Test connection manager + WebSocket sends
- Test async processor + progress reporting

### 3. End-to-End Tests (Full Flow)
- Deploy to test environment
- Use real WebSocket connections
- Verify complete request lifecycle

## Next Steps

1. **Monday AM**: Teams sync on interface details
2. **Monday PM**: Begin integration work
3. **Tuesday**: Share ConnectionManager implementation
4. **Wednesday**: First integrated test
5. **Thursday**: Performance testing
6. **Friday**: Demo integrated system 