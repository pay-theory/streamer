# Team 2: Application Layer & Developer Experience - AI Assistant Prompts for Week 2

## Your Mission This Week
You are completing the router integration, building the async processor, and implementing real-time progress updates. Your router from Week 1 needs to connect with Team 1's storage layer and new ConnectionManager.

## Context from Week 1
- ✅ You've built 80% of the router system with handler interfaces
- ✅ Sync/async decision logic is implemented
- ✅ Your code is in `pkg/streamer/` with good examples

## Monday: Integration Adapters & Planning

### Primary Task
```
Create adapters to integrate your router with Team 1's storage layer.

ANALYZE FIRST:
1. Read internal/store/models.go for AsyncRequest structure
2. Check internal/store/interfaces.go for RequestQueue methods
3. Review internal/store/errors.go for error types
4. Compare with your pkg/streamer/streamer.go Request type

CREATE pkg/streamer/adapters.go:

package streamer

import (
    "github.com/pay-theory/streamer/internal/store"
    "time"
)

// Adapter to use Team 1's RequestQueue with your router
type requestQueueAdapter struct {
    queue store.RequestQueue
}

func NewRequestQueueAdapter(queue store.RequestQueue) RequestStore {
    return &requestQueueAdapter{queue: queue}
}

func (a *requestQueueAdapter) Enqueue(ctx context.Context, req *Request) error {
    // Convert your Request to store.AsyncRequest
    asyncReq := &store.AsyncRequest{
        RequestID:    req.ID,
        ConnectionID: req.ConnectionID,
        Action:       req.Action,
        Status:       store.StatusPending,
        Payload:      req.Payload,
        CreatedAt:    req.CreatedAt,
        Metadata:     req.Metadata,
        TTL:          time.Now().Add(7 * 24 * time.Hour).Unix(),
    }
    
    return a.queue.Enqueue(ctx, asyncReq)
}

Also implement:
- Error mapping between packages
- Helper to convert AsyncRequest back to Request
- Unit tests verifying conversions work correctly

CREATE pkg/types/messages.go (coordinate with Team 1):
- Define standard WebSocket message types
- Error codes and formats
- Progress update structure
```

## Tuesday: Router Lambda Implementation

### Morning: Lambda Function Structure
```
Create the router Lambda function while waiting for ConnectionManager.

CREATE lambda/router/main.go:

package main

import (
    "context"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/pay-theory/streamer/pkg/streamer"
    "github.com/pay-theory/streamer/internal/store"
)

var router *streamer.DefaultRouter

func init() {
    ctx := context.Background()
    cfg, _ := config.LoadDefaultConfig(ctx)
    dynamoClient := dynamodb.NewFromConfig(cfg)
    
    // Initialize storage
    connStore := store.NewConnectionStore(dynamoClient, "")
    reqQueue := store.NewRequestQueue(dynamoClient, "")
    
    // Create adapter
    queueAdapter := streamer.NewRequestQueueAdapter(reqQueue)
    
    // Mock ConnectionManager for now
    connManager := &mockConnectionManager{}
    
    // Create router
    router = streamer.NewRouter(queueAdapter, connManager)
    router.SetAsyncThreshold(5 * time.Second)
    
    // Register handlers
    registerHandlers(router)
}

func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Route the request
    err := router.Route(ctx, event)
    if err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: 500,
            Body:       "Internal Server Error",
        }, nil
    }
    
    return events.APIGatewayProxyResponse{
        StatusCode: 200,
    }, nil
}

func main() {
    lambda.Start(handler)
}

CREATE lambda/router/handlers.go:
- Register all your production handlers
- Include examples like ReportHandler, DataProcessingHandler
- Each handler should define its EstimatedDuration
```

### Afternoon: ConnectionManager Integration
```
Replace mock with Team 1's real ConnectionManager.

UPDATE lambda/router/main.go:
1. Import "github.com/pay-theory/streamer/pkg/connection"
2. Initialize real ConnectionManager:
   connManager := connection.NewManager(connStore, apiGatewayClient)
3. Remove mock implementation

TEST integration:
- Router receives WebSocket message
- Sync requests process immediately  
- Async requests queue to DynamoDB
- Responses sent via ConnectionManager

CREATE lambda/router/handlers/report.go as example:

type ReportHandler struct {
    s3Client *s3.Client
}

func (h *ReportHandler) EstimatedDuration() time.Duration {
    return 2 * time.Minute // Async processing
}

func (h *ReportHandler) Validate(req *streamer.Request) error {
    var params ReportParams
    if err := json.Unmarshal(req.Payload, &params); err != nil {
        return fmt.Errorf("invalid payload: %w", err)
    }
    
    if params.StartDate.After(params.EndDate) {
        return errors.New("start date must be before end date")
    }
    
    return nil
}

func (h *ReportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    // This will be called by async processor, not here
    return nil, errors.New("use ProcessWithProgress for async handlers")
}
```

## Wednesday: Async Processor Foundation

### Morning: DynamoDB Streams Processor
```
Build the Lambda that processes async requests from DynamoDB Streams.

CREATE lambda/processor/main.go:

package main

import (
    "context"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-lambda-go/events"
    "github.com/pay-theory/streamer/lambda/processor/executor"
)

var exec *executor.AsyncExecutor

func init() {
    // Initialize executor with dependencies
    exec = executor.New(
        connectionManager,
        requestQueue,
        handlerRegistry,
    )
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
    for _, record := range event.Records {
        if record.EventName == "INSERT" {
            // Parse AsyncRequest from stream
            asyncReq, err := parseAsyncRequest(record)
            if err != nil {
                log.Error("Failed to parse request", err)
                continue
            }
            
            // Process with timeout handling
            processCtx, cancel := context.WithTimeout(ctx, 14*time.Minute)
            err = exec.ProcessRequest(processCtx, asyncReq)
            cancel()
            
            if err != nil {
                log.Error("Failed to process request", err)
                // Update status to failed
            }
        }
    }
    return nil
}

CREATE lambda/processor/executor/executor.go:
- Handler registry for async handlers
- Progress reporter implementation
- Timeout and error handling
- Status updates to DynamoDB
```

### Afternoon: Progress Reporting
```
Implement real-time progress updates via WebSocket.

CREATE pkg/progress/reporter.go:

type Reporter struct {
    requestID    string
    connectionID string
    connManager  connection.ConnectionManager
    queue        store.RequestQueue
    lastUpdate   time.Time
    mu           sync.Mutex
}

func (r *Reporter) Report(percentage float64, message string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Rate limit updates (max 10 per second)
    if time.Since(r.lastUpdate) < 100*time.Millisecond && percentage < 100 {
        return nil
    }
    
    update := map[string]interface{}{
        "type":       "progress",
        "request_id": r.requestID,
        "percentage": percentage,
        "message":    message,
        "timestamp":  time.Now().Unix(),
    }
    
    // Send via WebSocket
    err := r.connManager.Send(context.Background(), r.connectionID, update)
    if err != nil {
        log.Warn("Failed to send progress update", err)
    }
    
    // Update request metadata
    r.queue.UpdateMetadata(context.Background(), r.requestID, map[string]interface{}{
        "last_progress": percentage,
        "last_message":  message,
        "last_update":   time.Now(),
    })
    
    r.lastUpdate = time.Now()
    return nil
}

Integrate into async handlers:
- Pass reporter to ProcessWithProgress method
- Update progress at meaningful intervals
- Always send 100% on completion
```

## Thursday: Advanced Features & Testing

### Morning: Async Handler Implementation
```
Create production async handlers with progress reporting.

UPDATE lambda/processor/handlers/registry.go:

var Registry = map[string]AsyncHandler{
    "generate_report": &ReportHandler{},
    "process_data":    &DataProcessor{},
    "bulk_operation":  &BulkHandler{},
}

IMPLEMENT ReportHandler with progress:

func (h *ReportHandler) ProcessWithProgress(
    ctx context.Context, 
    req *store.AsyncRequest,
    reporter progress.Reporter,
) (*Result, error) {
    // Parse parameters
    var params ReportParams
    json.Unmarshal(req.Payload, &params)
    
    // Step 1: Query data (30%)
    reporter.Report(0, "Starting report generation...")
    data, err := h.queryData(ctx, params)
    if err != nil {
        return nil, fmt.Errorf("failed to query data: %w", err)
    }
    reporter.Report(30, fmt.Sprintf("Queried %d records", len(data)))
    
    // Step 2: Process data (60%)
    reporter.Report(30, "Processing data...")
    processed, err := h.processData(ctx, data)
    if err != nil {
        return nil, fmt.Errorf("failed to process: %w", err)
    }
    reporter.Report(60, "Data processing complete")
    
    // Step 3: Generate report (90%)
    reporter.Report(60, "Generating report file...")
    reportURL, err := h.generateReport(ctx, processed)
    if err != nil {
        return nil, fmt.Errorf("failed to generate: %w", err)
    }
    reporter.Report(90, "Uploading to S3...")
    
    // Complete
    reporter.Report(100, "Report ready!")
    
    return &Result{
        Success: true,
        Data: map[string]interface{}{
            "url": reportURL,
            "records": len(data),
            "size": processed.Size,
        },
    }, nil
}

Add error handling:
- Graceful degradation on progress send failures
- Timeout handling (save partial progress)
- Retry logic for transient failures
```

### Afternoon: Integration Testing
```
Create comprehensive integration tests.

CREATE tests/integration/async_flow_test.go:

func TestAsyncRequestFlow(t *testing.T) {
    ctx := context.Background()
    
    // Setup
    testEnv := setupTestEnvironment(t)
    defer testEnv.Cleanup()
    
    // Connect client
    conn := testEnv.Connect(t, validJWT)
    
    // Send async request
    request := map[string]interface{}{
        "action": "generate_report",
        "payload": map[string]interface{}{
            "start_date": "2024-01-01",
            "end_date":   "2024-01-31",
            "format":     "pdf",
        },
    }
    
    // Capture progress updates
    var progressUpdates []interface{}
    conn.OnMessage(func(msg interface{}) {
        if m, ok := msg.(map[string]interface{}); ok {
            if m["type"] == "progress" {
                progressUpdates = append(progressUpdates, m)
            }
        }
    })
    
    // Send request
    resp := conn.SendRequest(request)
    assert.Equal(t, "acknowledgment", resp["type"])
    
    // Wait for completion
    result := conn.WaitForCompletion(resp["request_id"], 30*time.Second)
    
    // Verify progress updates
    assert.True(t, len(progressUpdates) >= 4)
    assert.Equal(t, 100.0, progressUpdates[len(progressUpdates)-1]["percentage"])
    
    // Verify result
    assert.True(t, result["success"])
    assert.Contains(t, result["data"], "url")
}

Performance assertions:
- Acknowledgment within 100ms
- First progress update within 1s
- Completion within expected time
```

## Friday: Optimization & Demo

### Morning: Progress Batching
```
Optimize progress updates with intelligent batching.

CREATE pkg/progress/batcher.go:

type Batcher struct {
    reporter    Reporter
    updates     chan *Update
    interval    time.Duration
    maxBatch    int
}

func NewBatcher(reporter Reporter) *Batcher {
    b := &Batcher{
        reporter: reporter,
        updates:  make(chan *Update, 100),
        interval: 100 * time.Millisecond,
        maxBatch: 10,
    }
    go b.run()
    return b
}

func (b *Batcher) run() {
    ticker := time.NewTicker(b.interval)
    batch := make([]*Update, 0, b.maxBatch)
    
    for {
        select {
        case update := <-b.updates:
            batch = append(batch, update)
            
            // Send immediately if:
            // 1. Batch is full
            // 2. Update is 100% complete
            // 3. Update is an error
            if len(batch) >= b.maxBatch || 
               update.Percentage >= 100 || 
               update.Error != nil {
                b.flush(batch)
                batch = batch[:0]
            }
            
        case <-ticker.C:
            if len(batch) > 0 {
                b.flush(batch)
                batch = batch[:0]
            }
        }
    }
}

func (b *Batcher) flush(updates []*Update) {
    // Combine updates intelligently
    combined := b.combineUpdates(updates)
    
    // Send via reporter
    for _, update := range combined {
        b.reporter.Report(update.Percentage, update.Message)
    }
}

Benefits:
- Reduces WebSocket message volume
- Smoother progress updates
- Better performance under load
```

### Afternoon: Demo Excellence
```
Prepare an impressive demo of the complete system.

YOUR DEMO SECTIONS:

1. Router Capabilities:
   - Show instant echo response (< 50ms)
   - Demonstrate validation errors
   - Show sync vs async decision

2. Async Processing:
   - Submit report generation request
   - Show acknowledgment response
   - Display real-time progress updates
   - Show final result with S3 link

3. Error Handling:
   - Submit invalid request
   - Show graceful error response
   - Demonstrate timeout handling

4. Performance:
   - Submit 10 concurrent async requests
   - Show all progress updates working
   - Demonstrate no message loss

PREPARE demo helpers:

// Demo client for easy testing
const client = new StreamerClient('wss://demo.example.com/ws');

// Pre-made requests
const demoRequests = {
    echo: {
        action: 'echo',
        payload: { message: 'Hello, Streamer!' }
    },
    
    report: {
        action: 'generate_report',
        payload: {
            start_date: '2024-01-01',
            end_date: '2024-01-31',
            format: 'pdf',
            include_charts: true
        }
    },
    
    invalid: {
        action: 'generate_report',
        payload: {
            // Missing required fields
        }
    }
};

// Progress visualization
function visualizeProgress(updates) {
    console.clear();
    const bar = '█'.repeat(Math.floor(updates.percentage / 2));
    const empty = '░'.repeat(50 - bar.length);
    console.log(`Progress: ${bar}${empty} ${updates.percentage}%`);
    console.log(`Status: ${updates.message}`);
}
```

## Debugging Guides

### Async Request Not Processing
```
1. Verify DynamoDB Streams is enabled on AsyncRequest table
2. Check processor Lambda is triggered by streams
3. Look for parsing errors in processor logs
4. Verify handler is registered for the action
5. Check for timeout issues (14+ minutes)
```

### Progress Updates Not Arriving
```
1. Verify connection is still active
2. Check ConnectionManager.Send errors
3. Look for rate limiting in progress reporter
4. Verify WebSocket message format
5. Check client-side message handling
```

### Integration Issues
```
1. Verify adapter correctly converts types
2. Check error mapping between packages
3. Test with Team 1's actual ConnectionManager
4. Verify DynamoDB permissions for Lambda
```

## Architecture Decisions

1. **Why 5-second async threshold?**
   - API Gateway timeout is 29 seconds
   - Allows buffer for network and processing
   - Good UX for progress indication

2. **Why DynamoDB Streams?**
   - Automatic triggering
   - At-least-once delivery
   - Handles Lambda scaling
   - Built-in retry

3. **Why batch progress updates?**
   - Reduces WebSocket load
   - Better client performance
   - Cost optimization
   - Smoother UX

4. **Why separate async handlers?**
   - Different timeout requirements
   - Progress reporting needs
   - Better testability
   - Clear separation of concerns

Remember: You're building the developer experience. Make it delightful! 