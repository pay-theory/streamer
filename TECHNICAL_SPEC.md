# Streamer Technical Specification

## Core Interfaces

### 1. Router Interface

The router handles incoming WebSocket messages and decides whether to process synchronously or queue for async processing.

```go
// pkg/streamer/router.go
package streamer

import (
    "context"
    "github.com/aws/aws-lambda-go/events"
)

// Router handles incoming WebSocket messages
type Router interface {
    // Route processes an incoming WebSocket message
    Route(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error
    
    // Register adds a handler for an action
    Register(action string, handler Handler) error
    
    // SetFallback sets the default handler for unknown actions
    SetFallback(handler Handler) error
}

// Handler processes a specific action
type Handler interface {
    // Validate checks if the request is valid
    Validate(ctx context.Context, req Request) error
    
    // ShouldQueue determines if this request needs async processing
    ShouldQueue(req Request) bool
    
    // Process handles synchronous requests
    Process(ctx context.Context, req Request) (Response, error)
    
    // PrepareAsync prepares a request for async processing
    PrepareAsync(ctx context.Context, req Request) (AsyncRequest, error)
}

// Request represents an incoming request
type Request struct {
    ID           string                 `json:"id"`
    ConnectionID string                 `json:"-"`
    TenantID     string                 `json:"-"`
    Action       string                 `json:"action"`
    Payload      map[string]interface{} `json:"payload"`
    Metadata     RequestMetadata        `json:"-"`
}

// Response for synchronous requests
type Response struct {
    Success bool                   `json:"success"`
    Data    map[string]interface{} `json:"data,omitempty"`
    Error   *Error                 `json:"error,omitempty"`
}
```

### 2. Processor Interface

The processor handles queued requests from DynamoDB Streams.

```go
// pkg/streamer/processor.go
package streamer

import (
    "context"
    "github.com/aws/aws-lambda-go/events"
)

// Processor handles async request processing
type Processor interface {
    // ProcessStream handles DynamoDB stream events
    ProcessStream(ctx context.Context, event events.DynamoDBEvent) error
    
    // Register adds an async handler
    Register(action string, handler AsyncHandler) error
}

// AsyncHandler processes long-running requests
type AsyncHandler interface {
    // Process executes the async operation
    Process(ctx context.Context, req AsyncRequest) (AsyncResult, error)
    
    // Timeout returns the maximum processing duration
    Timeout() time.Duration
    
    // RetryPolicy returns retry configuration
    RetryPolicy() RetryPolicy
}

// AsyncRequest represents a queued request
type AsyncRequest struct {
    RequestID     string                 `dynamorm:"pk"`
    ConnectionID  string                 `dynamorm:"index:connection-requests,pk"`
    TenantID      string                 `dynamorm:"index:tenant-requests,pk"`
    Status        RequestStatus          `dynamorm:"index:status-time,pk"`
    CreatedAt     time.Time              `dynamorm:"index:status-time,sk"`
    Action        string                 
    Payload       map[string]interface{} `dynamorm:"json"`
    Result        map[string]interface{} `dynamorm:"json"`
    Error         string                 
    ProcessedAt   time.Time              
    TTL           int64                  `dynamorm:"ttl"`
    RetryCount    int                    
    Priority      int                    
}

// ProgressReporter allows handlers to report progress
type ProgressReporter interface {
    // Report sends a progress update
    Report(progress float64, message string, details ...map[string]interface{}) error
    
    // ReportError reports a non-fatal error
    ReportError(err error) error
    
    // SetCheckpoint saves progress state
    SetCheckpoint(checkpoint interface{}) error
}
```

### 3. Connection Manager Interface

Manages WebSocket connections with DynamoDB persistence.

```go
// pkg/connection/manager.go
package connection

import (
    "context"
    "time"
)

// Manager handles WebSocket connections
type Manager interface {
    // Connection lifecycle
    Connect(ctx context.Context, conn Connection) error
    Disconnect(ctx context.Context, connectionID string) error
    UpdateActivity(ctx context.Context, connectionID string) error
    
    // Retrieval
    Get(ctx context.Context, connectionID string) (*Connection, error)
    GetByUser(ctx context.Context, userID string) ([]*Connection, error)
    GetByTenant(ctx context.Context, tenantID string) ([]*Connection, error)
    
    // Messaging
    Send(ctx context.Context, connectionID string, message interface{}) error
    SendBatch(ctx context.Context, messages map[string]interface{}) error
    Broadcast(ctx context.Context, filter Filter, message interface{}) error
    
    // Maintenance
    PruneStale(ctx context.Context, before time.Time) error
}

// Connection represents a WebSocket connection
type Connection struct {
    ConnectionID string            `dynamorm:"pk"`
    UserID       string            `dynamorm:"index:user-connections,pk"`
    TenantID     string            `dynamorm:"index:tenant-connections,pk"`
    Endpoint     string            
    ConnectedAt  time.Time         
    LastActivity time.Time         
    Metadata     map[string]string `dynamorm:"json"`
    TTL          int64             `dynamorm:"ttl"`
}

// Filter for connection queries
type Filter struct {
    TenantID   string
    UserIDs    []string
    Metadata   map[string]string
    ActiveOnly bool
}
```

### 4. Queue Interface

Manages async request queuing with priority support.

```go
// pkg/queue/queue.go
package queue

import (
    "context"
    "time"
)

// Queue manages async requests
type Queue interface {
    // Enqueue adds a request to the queue
    Enqueue(ctx context.Context, req QueueRequest) error
    
    // Dequeue retrieves requests for processing (used in tests)
    Dequeue(ctx context.Context, limit int) ([]*QueueRequest, error)
    
    // Get retrieves a specific request
    Get(ctx context.Context, requestID string) (*QueueRequest, error)
    
    // Update modifies request status
    Update(ctx context.Context, requestID string, update StatusUpdate) error
    
    // Delete removes a completed request
    Delete(ctx context.Context, requestID string) error
    
    // Query finds requests matching criteria
    Query(ctx context.Context, query Query) ([]*QueueRequest, error)
}

// QueueRequest represents a queued request
type QueueRequest struct {
    AsyncRequest
    Priority      int       
    ProcessAfter  time.Time 
    MaxRetries    int       
    RetryDelay    time.Duration
}

// StatusUpdate for request state changes
type StatusUpdate struct {
    Status   RequestStatus
    Result   map[string]interface{}
    Error    string
    Progress float64
}
```

## Data Models (DynamORM)

### Table: streamer_connections

```go
// internal/store/models.go
type Connection struct {
    // Partition key
    ConnectionID string `dynamorm:"pk"`
    
    // Global secondary indexes
    UserID   string `dynamorm:"index:gsi-user,pk"`
    TenantID string `dynamorm:"index:gsi-tenant,pk"`
    
    // Connection details
    Endpoint     string    // API Gateway callback URL
    ConnectedAt  time.Time 
    LastActivity time.Time 
    IPAddress    string
    UserAgent    string
    
    // Metadata
    Metadata map[string]string `dynamorm:"json"`
    
    // TTL for automatic cleanup
    TTL int64 `dynamorm:"ttl"`
}
```

### Table: streamer_requests

```go
type AsyncRequest struct {
    // Partition key
    RequestID string `dynamorm:"pk"`
    
    // Global secondary indexes
    ConnectionID string    `dynamorm:"index:gsi-connection,pk"`
    Status       string    `dynamorm:"index:gsi-status,pk"`
    CreatedAt    time.Time `dynamorm:"index:gsi-status,sk"`
    Priority     int       `dynamorm:"index:gsi-priority,pk"`
    ProcessAfter time.Time `dynamorm:"index:gsi-priority,sk"`
    
    // Request details
    Action   string
    Payload  map[string]interface{} `dynamorm:"json"`
    TenantID string
    UserID   string
    
    // Processing state
    ProcessingStarted time.Time
    ProcessingEnded   time.Time
    Progress          float64
    Result            map[string]interface{} `dynamorm:"json"`
    Error             string
    
    // Retry handling
    RetryCount int
    MaxRetries int
    RetryAfter time.Time
    
    // TTL
    TTL int64 `dynamorm:"ttl"`
}
```

### Table: streamer_subscriptions

```go
type Subscription struct {
    // Composite primary key
    SubscriptionID string `dynamorm:"pk,composite:connection_id,request_id"`
    
    // Global secondary indexes
    ConnectionID string   `dynamorm:"index:gsi-connection,pk"`
    RequestID    string   `dynamorm:"index:gsi-request,pk"`
    
    // Subscription details
    EventTypes []string  `dynamorm:"set"` // progress, complete, error
    CreatedAt  time.Time
    
    // TTL
    TTL int64 `dynamorm:"ttl"`
}
```

## Message Protocol

### WebSocket Message Format

All messages follow this structure:

```json
{
    "id": "unique-request-id",
    "type": "request|response|progress|error",
    "action": "action-name",
    "payload": {},
    "metadata": {
        "timestamp": "2024-01-01T00:00:00Z",
        "version": "1.0"
    }
}
```

### Message Types

#### 1. Request (Client → Server)
```json
{
    "id": "req-123",
    "type": "request",
    "action": "generate_report",
    "payload": {
        "dateRange": {"start": "2024-01-01", "end": "2024-12-31"},
        "format": "pdf"
    }
}
```

#### 2. Acknowledgment (Server → Client)
```json
{
    "id": "req-123",
    "type": "ack",
    "status": "queued",
    "message": "Request queued for processing"
}
```

#### 3. Progress Update (Server → Client)
```json
{
    "id": "req-123",
    "type": "progress",
    "progress": 0.75,
    "message": "Processing data...",
    "details": {
        "recordsProcessed": 7500,
        "totalRecords": 10000
    }
}
```

#### 4. Result (Server → Client)
```json
{
    "id": "req-123",
    "type": "result",
    "success": true,
    "data": {
        "url": "https://example.com/report.pdf",
        "size": 1048576
    }
}
```

## Lambda Functions

### 1. Connect Handler

Handles WebSocket $connect route.

```go
// lambda/connect/handler.go
func HandleConnect(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    // 1. Validate authorization
    userID, tenantID, err := validateAuth(event.Headers)
    if err != nil {
        return forbiddenResponse(), nil
    }
    
    // 2. Create connection record
    conn := &Connection{
        ConnectionID: event.RequestContext.ConnectionID,
        UserID:       userID,
        TenantID:     tenantID,
        Endpoint:     getCallbackURL(event),
        ConnectedAt:  time.Now(),
        TTL:          time.Now().Add(24 * time.Hour).Unix(),
    }
    
    // 3. Store in DynamoDB
    if err := connectionManager.Connect(ctx, conn); err != nil {
        return errorResponse(err), nil
    }
    
    return successResponse(), nil
}
```

### 2. Router Handler

Routes messages to appropriate handlers.

```go
// lambda/router/handler.go
func HandleMessage(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    // 1. Parse message
    var msg Message
    if err := json.Unmarshal([]byte(event.Body), &msg); err != nil {
        return sendError(event.RequestContext.ConnectionID, "Invalid message format")
    }
    
    // 2. Get handler
    handler, exists := handlers[msg.Action]
    if !exists {
        return sendError(event.RequestContext.ConnectionID, "Unknown action")
    }
    
    // 3. Create request
    req := Request{
        ID:           msg.ID,
        ConnectionID: event.RequestContext.ConnectionID,
        Action:       msg.Action,
        Payload:      msg.Payload,
    }
    
    // 4. Validate
    if err := handler.Validate(ctx, req); err != nil {
        return sendError(event.RequestContext.ConnectionID, err.Error())
    }
    
    // 5. Check if async needed
    if handler.ShouldQueue(req) {
        // Queue for async processing
        asyncReq, err := handler.PrepareAsync(ctx, req)
        if err != nil {
            return sendError(event.RequestContext.ConnectionID, err.Error())
        }
        
        if err := queue.Enqueue(ctx, asyncReq); err != nil {
            return sendError(event.RequestContext.ConnectionID, "Failed to queue request")
        }
        
        // Send acknowledgment
        return sendAck(event.RequestContext.ConnectionID, req.ID, "queued")
    }
    
    // 6. Process synchronously
    resp, err := handler.Process(ctx, req)
    if err != nil {
        return sendError(event.RequestContext.ConnectionID, err.Error())
    }
    
    return sendResponse(event.RequestContext.ConnectionID, req.ID, resp)
}
```

### 3. Processor Handler

Processes queued requests from DynamoDB Streams.

```go
// lambda/processor/handler.go
func HandleStream(ctx context.Context, event events.DynamoDBEvent) error {
    for _, record := range event.Records {
        if record.EventName != "INSERT" && record.EventName != "MODIFY" {
            continue
        }
        
        // Parse request
        var req AsyncRequest
        if err := unmarshalStreamImage(record.Change.NewImage, &req); err != nil {
            log.Printf("Failed to unmarshal: %v", err)
            continue
        }
        
        // Only process queued requests
        if req.Status != StatusQueued {
            continue
        }
        
        // Process in goroutine for parallelism
        go processRequest(ctx, req)
    }
    
    return nil
}

func processRequest(ctx context.Context, req AsyncRequest) {
    // 1. Get handler
    handler, exists := asyncHandlers[req.Action]
    if !exists {
        failRequest(req, "Unknown action")
        return
    }
    
    // 2. Create progress reporter
    reporter := NewProgressReporter(req.RequestID, req.ConnectionID)
    ctx = WithProgressReporter(ctx, reporter)
    
    // 3. Update status to processing
    updateStatus(req.RequestID, StatusProcessing)
    
    // 4. Process with timeout
    processCtx, cancel := context.WithTimeout(ctx, handler.Timeout())
    defer cancel()
    
    result, err := handler.Process(processCtx, req)
    
    // 5. Handle result
    if err != nil {
        if req.RetryCount < req.MaxRetries {
            retryRequest(req, err)
        } else {
            failRequest(req, err.Error())
        }
        return
    }
    
    // 6. Complete request
    completeRequest(req, result)
}
```

## Security

### Authentication Flow

1. **WebSocket Connection**:
   - JWT token in query parameters or headers
   - Validate on $connect
   - Store user/tenant in connection record

2. **Per-Request Authorization**:
   - Validate user has permission for action
   - Check tenant isolation
   - Rate limiting per user/tenant

### Data Encryption

- **At Rest**: DynamoDB encryption enabled
- **In Transit**: TLS 1.2+ for WebSocket
- **Sensitive Fields**: Application-level encryption for PII

### Rate Limiting

```go
type RateLimiter interface {
    // Check if request allowed
    Allow(ctx context.Context, key string, cost int) (bool, error)
    
    // Get current usage
    Usage(ctx context.Context, key string) (current, limit int, err error)
}

// Implementation using DynamoDB
type DynamoRateLimiter struct {
    table      string
    windowSize time.Duration
    limits     map[string]int // tier -> limit mapping
}
```

## Performance Optimizations

### 1. Connection Pooling

```go
// Reuse API Gateway Management API clients
var (
    clientCache = make(map[string]*apigatewaymanagementapi.Client)
    clientMu    sync.RWMutex
)

func getClient(endpoint string) *apigatewaymanagementapi.Client {
    clientMu.RLock()
    client, exists := clientCache[endpoint]
    clientMu.RUnlock()
    
    if exists {
        return client
    }
    
    // Create new client
    clientMu.Lock()
    defer clientMu.Unlock()
    
    // Double-check
    if client, exists := clientCache[endpoint]; exists {
        return client
    }
    
    client = apigatewaymanagementapi.New(...)
    clientCache[endpoint] = client
    
    return client
}
```

### 2. Batch Operations

```go
// Batch WebSocket sends
func (m *Manager) SendBatch(ctx context.Context, messages map[string]interface{}) error {
    // Group by endpoint for efficiency
    grouped := make(map[string][]sendRequest)
    
    for connID, msg := range messages {
        conn, err := m.Get(ctx, connID)
        if err != nil {
            continue
        }
        
        grouped[conn.Endpoint] = append(grouped[conn.Endpoint], sendRequest{
            ConnectionID: connID,
            Data:        msg,
        })
    }
    
    // Send in parallel
    var wg sync.WaitGroup
    errors := make(chan error, len(grouped))
    
    for endpoint, requests := range grouped {
        wg.Add(1)
        go func(ep string, reqs []sendRequest) {
            defer wg.Done()
            
            client := getClient(ep)
            for _, req := range reqs {
                if err := send(client, req); err != nil {
                    errors <- err
                }
            }
        }(endpoint, requests)
    }
    
    wg.Wait()
    close(errors)
    
    // Collect errors
    var errs []error
    for err := range errors {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("batch send had %d errors", len(errs))
    }
    
    return nil
}
```

### 3. DynamoDB Optimizations

- **Batch writes** for multiple updates
- **Projection expressions** to reduce data transfer
- **Sparse indexes** for optional attributes
- **Parallel scans** for large queries

## Testing Strategy

### 1. Unit Tests

```go
// connection_test.go
func TestConnectionManager_Connect(t *testing.T) {
    // Setup
    db := newMockDynamoDB()
    manager := NewConnectionManager(db)
    
    // Test
    conn := &Connection{
        ConnectionID: "test-123",
        UserID:       "user-123",
        TenantID:     "tenant-123",
    }
    
    err := manager.Connect(context.Background(), conn)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, 1, db.PutItemCallCount())
}
```

### 2. Integration Tests

```go
// flow_test.go
func TestAsyncRequestFlow(t *testing.T) {
    // Setup local DynamoDB
    db := setupLocalDynamoDB(t)
    
    // Initialize components
    router := NewRouter(db)
    processor := NewProcessor(db)
    
    // Register handler
    router.Register("test", &testHandler{})
    processor.Register("test", &testAsyncHandler{})
    
    // Simulate request flow
    event := createWebSocketEvent("test", map[string]interface{}{
        "data": "test-data",
    })
    
    // Route request
    err := router.Route(context.Background(), event)
    assert.NoError(t, err)
    
    // Verify queued
    req := getQueuedRequest(t, db)
    assert.Equal(t, "queued", req.Status)
    
    // Process via stream
    streamEvent := createStreamEvent(req)
    err = processor.ProcessStream(context.Background(), streamEvent)
    assert.NoError(t, err)
    
    // Verify completed
    req = getRequest(t, db, req.RequestID)
    assert.Equal(t, "completed", req.Status)
}
```

### 3. Load Tests

```javascript
// k6/websocket_load.js
import ws from 'k6/ws';
import { check } from 'k6';

export let options = {
    stages: [
        { duration: '30s', target: 100 },  // Ramp up
        { duration: '2m', target: 1000 },  // Sustain
        { duration: '30s', target: 0 },    // Ramp down
    ],
};

export default function() {
    const url = `wss://${__ENV.API_URL}/ws`;
    
    ws.connect(url, {}, function(socket) {
        socket.on('open', () => {
            // Send async request
            socket.send(JSON.stringify({
                id: `req-${__VU}-${__ITER}`,
                action: 'generate_report',
                payload: {
                    type: 'load-test',
                },
            }));
        });
        
        socket.on('message', (data) => {
            const msg = JSON.parse(data);
            check(msg, {
                'request acknowledged': (m) => m.type === 'ack',
                'no errors': (m) => m.type !== 'error',
            });
        });
        
        socket.setTimeout(() => {
            socket.close();
        }, 30000);
    });
}
```

## Deployment

### CloudFormation/SAM Template

```yaml
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31

Globals:
  Function:
    Runtime: go1.x
    MemorySize: 256
    Timeout: 30
    Environment:
      Variables:
        CONNECTIONS_TABLE: !Ref ConnectionsTable
        REQUESTS_TABLE: !Ref RequestsTable

Resources:
  # DynamoDB Tables
  ConnectionsTable:
    Type: AWS::DynamoDB::Table
    Properties:
      BillingMode: PAY_PER_REQUEST
      AttributeDefinitions:
        - AttributeName: ConnectionID
          AttributeType: S
        - AttributeName: UserID
          AttributeType: S
        - AttributeName: TenantID
          AttributeType: S
      KeySchema:
        - AttributeName: ConnectionID
          KeyType: HASH
      GlobalSecondaryIndexes:
        - IndexName: gsi-user
          KeySchema:
            - AttributeName: UserID
              KeyType: HASH
          Projection:
            ProjectionType: ALL
        - IndexName: gsi-tenant
          KeySchema:
            - AttributeName: TenantID
              KeyType: HASH
          Projection:
            ProjectionType: ALL
      TimeToLiveSpecification:
        AttributeName: TTL
        Enabled: true
        
  RequestsTable:
    Type: AWS::DynamoDB::Table
    Properties:
      BillingMode: PAY_PER_REQUEST
      StreamSpecification:
        StreamViewType: NEW_AND_OLD_IMAGES
      # ... attributes and indexes ...
      
  # Lambda Functions
  ConnectFunction:
    Type: AWS::Serverless::Function
    Properties:
      CodeUri: lambda/connect/
      Handler: main
      Events:
        Connect:
          Type: Api
          Properties:
            Path: /$connect
            Method: POST
            
  RouterFunction:
    Type: AWS::Serverless::Function
    Properties:
      CodeUri: lambda/router/
      Handler: main
      Events:
        Default:
          Type: Api
          Properties:
            Path: /$default
            Method: POST
            
  ProcessorFunction:
    Type: AWS::Serverless::Function
    Properties:
      CodeUri: lambda/processor/
      Handler: main
      MemorySize: 3008
      Timeout: 900
      Events:
        Stream:
          Type: DynamoDB
          Properties:
            Stream: !GetAtt RequestsTable.StreamArn
            StartingPosition: LATEST
            MaximumBatchingWindowInSeconds: 10
``` 