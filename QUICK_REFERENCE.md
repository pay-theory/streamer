# Streamer Quick Reference

## Core Concepts

### Request Flow
```
Client → WebSocket → Router (sync/async?) → Queue → Processor → Result → Client
                          ↓                                          ↑
                     Immediate ACK ─────────────────────────────────┘
```

### Key Components
- **Router**: Receives requests, decides sync/async
- **Queue**: DynamoDB table for async requests  
- **Processor**: Lambda triggered by DynamoDB Streams
- **Notifier**: Sends progress updates via WebSocket

## Basic Usage

### 1. Define a Handler

```go
type ReportHandler struct {
    s3Client *s3.Client
}

// For sync/async decision
func (h *ReportHandler) ShouldQueue(req streamer.Request) bool {
    // Queue if report will take > 5 seconds
    return req.Payload["recordCount"].(int) > 1000
}

// For sync processing
func (h *ReportHandler) Process(ctx context.Context, req streamer.Request) (streamer.Response, error) {
    // Quick operations only
    return streamer.Response{
        Success: true,
        Data: map[string]interface{}{
            "status": "completed",
        },
    }, nil
}

// For async processing
func (h *ReportHandler) ProcessAsync(ctx context.Context, req streamer.AsyncRequest) (interface{}, error) {
    reporter := streamer.GetReporter(ctx)
    
    // Report progress
    reporter.Progress(0.5, "Processing data...")
    
    // Do work...
    
    return map[string]interface{}{
        "url": "s3://bucket/report.pdf",
    }, nil
}
```

### 2. Register Handler

```go
// In Lambda init
router := streamer.NewRouter(
    streamer.WithDynamoDB(db),
    streamer.WithTable("requests"),
)

router.Register("generate_report", &ReportHandler{})
```

### 3. Client Usage

```typescript
const client = new StreamerClient('wss://api.example.com/ws');

const result = await client.request('generate_report', {
    startDate: '2024-01-01',
    endDate: '2024-12-31'
}, {
    onProgress: (progress, message) => {
        console.log(`${progress * 100}%: ${message}`);
    }
});
```

## Data Models

### Connection
```go
type Connection struct {
    ConnectionID string    `dynamorm:"pk"`
    UserID       string    `dynamorm:"index:user-connections,pk"`
    TenantID     string    `dynamorm:"index:tenant-connections,pk"`
    Endpoint     string    
    ConnectedAt  time.Time 
    TTL          int64     `dynamorm:"ttl"`
}
```

### AsyncRequest
```go
type AsyncRequest struct {
    RequestID    string                 `dynamorm:"pk"`
    ConnectionID string                 `dynamorm:"index:connection-requests,pk"`
    Status       string                 // queued, processing, completed, failed
    Action       string                 
    Payload      map[string]interface{} `dynamorm:"json"`
    Result       map[string]interface{} `dynamorm:"json"`
    Progress     float64               
    TTL          int64                 `dynamorm:"ttl"`
}
```

## Message Protocol

### Request
```json
{
    "id": "req-123",
    "action": "generate_report",
    "payload": {
        "startDate": "2024-01-01"
    }
}
```

### Progress Update
```json
{
    "id": "req-123",
    "type": "progress",
    "progress": 0.75,
    "message": "Generating PDF..."
}
```

### Result
```json
{
    "id": "req-123",
    "type": "result",
    "success": true,
    "data": {
        "url": "https://..."
    }
}
```

## Lambda Functions

### Connect ($connect)
```go
func HandleConnect(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error {
    // 1. Validate auth
    userID, tenantID := validateJWT(event.Headers["Authorization"])
    
    // 2. Store connection
    conn := &Connection{
        ConnectionID: event.RequestContext.ConnectionID,
        UserID:       userID,
        TenantID:     tenantID,
        Endpoint:     getCallbackURL(event),
        TTL:          time.Now().Add(24*time.Hour).Unix(),
    }
    
    return connectionManager.Connect(ctx, conn)
}
```

### Router ($default)
```go
func HandleMessage(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error {
    var msg Message
    json.Unmarshal([]byte(event.Body), &msg)
    
    return router.Route(ctx, event)
}
```

### Processor (DynamoDB Streams)
```go
func HandleStream(ctx context.Context, event events.DynamoDBEvent) error {
    return processor.ProcessStream(ctx, event)
}
```

## Common Patterns

### Progress Reporting
```go
func processLargeDataset(ctx context.Context, data []Record) error {
    reporter := streamer.GetReporter(ctx)
    total := len(data)
    
    for i, record := range data {
        // Process record...
        
        // Report progress every 100 records
        if i%100 == 0 {
            progress := float64(i) / float64(total)
            reporter.Progress(progress, fmt.Sprintf("Processed %d/%d", i, total))
        }
    }
    
    return nil
}
```

### Error Handling
```go
func (h *Handler) ProcessAsync(ctx context.Context, req AsyncRequest) (interface{}, error) {
    // Retryable error
    if err := externalAPI.Call(); err != nil {
        return nil, streamer.RetryableError(err)
    }
    
    // Non-retryable error
    if err := validate(req); err != nil {
        return nil, streamer.FatalError(err)
    }
    
    return result, nil
}
```

### Multi-Tenant
```go
// Get tenant-specific data
conn, _ := connectionManager.Get(ctx, connectionID)
tenantData := getTenantData(conn.TenantID)

// Broadcast to tenant
connectionManager.Broadcast(ctx, BroadcastOptions{
    TenantID: tenantID,
    Message:  announcement,
})
```

## Configuration

### Environment Variables
```bash
# DynamoDB Tables
CONNECTIONS_TABLE=streamer-connections
REQUESTS_TABLE=streamer-requests
SUBSCRIPTIONS_TABLE=streamer-subscriptions

# Lambda Configuration
LAMBDA_TIMEOUT_BUFFER=1s
MAX_RETRIES=3
RETRY_DELAY=5s

# WebSocket
WEBSOCKET_API_URL=wss://api.example.com/ws
CONNECTION_TTL=24h
```

### Lambda Memory Settings
- **Router**: 256MB (fast response)
- **Processor**: 3008MB (CPU intensive)
- **Notifier**: 512MB (I/O bound)

## Debugging

### CloudWatch Logs
```go
// Structured logging
log.Printf("Processing request: %+v", map[string]interface{}{
    "requestId":    req.RequestID,
    "connectionId": req.ConnectionID,
    "action":       req.Action,
    "status":       req.Status,
})
```

### X-Ray Tracing
```go
// Automatic with environment variable
AWS_XRAY_TRACE_ID=1-5e1b4f87-7c1b2a3d4e5f6a7b8c9d0e1f
```

### Metrics
```go
// Custom CloudWatch metrics
metrics.PutMetric("RequestProcessed", 1, "Count")
metrics.PutMetric("ProcessingTime", duration.Milliseconds(), "Milliseconds")
```

## Error Codes

| Code | Description | Retry |
|------|-------------|-------|
| `INVALID_REQUEST` | Malformed request | No |
| `UNAUTHORIZED` | Auth failed | No |
| `RATE_LIMITED` | Too many requests | Yes |
| `TIMEOUT` | Processing timeout | Yes |
| `INTERNAL_ERROR` | Server error | Yes |

## Performance Tips

1. **Pre-register models** in Lambda init
2. **Reuse connections** across invocations
3. **Batch operations** when possible
4. **Use projections** to reduce data transfer
5. **Set appropriate TTLs** for cleanup
6. **Monitor cold starts** and optimize

## Testing

### Unit Test
```go
func TestHandler_Process(t *testing.T) {
    handler := &ReportHandler{}
    req := streamer.Request{
        Action: "generate_report",
        Payload: map[string]interface{}{
            "type": "monthly",
        },
    }
    
    resp, err := handler.Process(context.Background(), req)
    assert.NoError(t, err)
    assert.True(t, resp.Success)
}
```

### Integration Test
```go
func TestEndToEnd(t *testing.T) {
    // Setup
    router := setupTestRouter()
    processor := setupTestProcessor()
    
    // Send request
    event := createWebSocketEvent("test_action", payload)
    err := router.Route(context.Background(), event)
    assert.NoError(t, err)
    
    // Process
    streamEvent := waitForStreamEvent()
    err = processor.ProcessStream(context.Background(), streamEvent)
    assert.NoError(t, err)
    
    // Verify result
    result := getProcessedRequest(requestID)
    assert.Equal(t, "completed", result.Status)
} 