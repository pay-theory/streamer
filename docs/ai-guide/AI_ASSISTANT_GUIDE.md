# Streamer AI Assistant Guide

*A comprehensive reference for AI assistants working with the Streamer async processing framework*

## 🎯 Overview

Streamer is a production-ready async request processing system for AWS Lambda that overcomes API Gateway's 29-second timeout limitation through WebSocket-based real-time communication and DynamoDB Streams processing.

### **Key Concepts**
- **Sync/Async Routing**: Operations <5s run synchronously, >5s run asynchronously
- **Real-time Progress**: WebSocket-based progress updates for long-running operations
- **DynamoDB Streams**: Triggers async processing when requests are queued
- **JWT Authentication**: Secure connection management with tenant isolation
- **Type Safety**: Strongly-typed handlers and messages throughout

## 📚 Core Types & Interfaces

### **Handler Interfaces**

#### Base Handler Interface
```go
type Handler interface {
    // EstimatedDuration determines sync vs async processing (5s threshold)
    EstimatedDuration() time.Duration
    
    // Validate checks if the incoming request is valid
    Validate(req *Request) error
    
    // Process handles synchronous requests (< 5 seconds)
    Process(ctx context.Context, req *Request) (*Result, error)
}
```

#### Handler with Progress Support
```go
type HandlerWithProgress interface {
    Handler
    
    // ProcessWithProgress handles async requests with real-time progress updates
    ProcessWithProgress(
        ctx context.Context, 
        req *Request, 
        reporter progress.Reporter,
    ) (*Result, error)
}
```

### **Core Data Types**

#### Request Structure
```go
type Request struct {
    ID           string            `json:"id"`           // Unique request identifier
    ConnectionID string            `json:"-"`            // WebSocket connection ID
    UserID       string            `json:"-"`            // User context (from JWT)
    TenantID     string            `json:"-"`            // Tenant context (from JWT)
    Action       string            `json:"action"`       // Handler action to invoke
    Payload      []byte            `json:"payload"`      // JSON payload data
    Metadata     map[string]interface{} `json:"-"`       // Additional metadata
    Timestamp    time.Time         `json:"-"`            // Request timestamp
}

// Helper methods
func (r *Request) UnmarshalPayload(v interface{}) error
func (r *Request) GetMetadata(key string) (interface{}, bool)
func (r *Request) SetMetadata(key string, value interface{})
```

#### Result Structure
```go
type Result struct {
    RequestID string                 `json:"request_id"`  // Request ID this result corresponds to
    Success   bool                   `json:"success"`     // Whether operation succeeded
    Data      interface{}            `json:"data,omitempty"` // Result data (JSON marshaled)
    Error     *Error                 `json:"error,omitempty"` // Error information if failed
    Metadata  map[string]interface{} `json:"metadata,omitempty"` // Processing metadata
    Duration  time.Duration          `json:"-"`           // Processing duration
}
```

#### Error Structure
```go
type Error struct {
    Code    string                 `json:"code"`        // Error code (see standard codes below)
    Message string                 `json:"message"`     // Human-readable error message
    Details map[string]interface{} `json:"details,omitempty"` // Additional error details
    Retry   *RetryInfo            `json:"retry,omitempty"` // Retry information
}

type RetryInfo struct {
    Retryable bool      `json:"retryable"`    // Whether this error is retryable
    After     time.Time `json:"after,omitempty"` // When to retry (if retryable)
    MaxTries  int       `json:"max_tries,omitempty"` // Maximum number of retries
    Attempt   int       `json:"attempt,omitempty"` // Current retry attempt
}
```

### **Progress Reporting**

#### Progress Reporter Interface
```go
type Reporter interface {
    // Report sends a progress update (0-100%)
    Report(percentage float64, message string, details ...map[string]interface{}) error
    
    // Complete marks the operation as finished with final result
    Complete(result map[string]interface{}) (*Result, error)
    
    // Fail marks the operation as failed
    Fail(err error) error
}
```

#### Progress Update Structure
```go
type ProgressUpdate struct {
    RequestID  string                 `json:"request_id"`
    Percentage float64                `json:"percentage"`  // 0-100
    Message    string                 `json:"message"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
    Timestamp  time.Time              `json:"timestamp"`
}
```

### **Connection Management**

#### Connection Manager Interface
```go
type ConnectionManager interface {
    // Send a message to a specific connection
    Send(ctx context.Context, connectionID string, message interface{}) error
    
    // Check if a connection is active
    IsActive(ctx context.Context, connectionID string) bool
    
    // Broadcast to multiple connections
    Broadcast(ctx context.Context, connectionIDs []string, message interface{}) error
    
    // Get connection metrics
    GetMetrics() map[string]interface{}
}
```

#### Connection Model (DynamORM)
```go
type Connection struct {
    ConnectionID string            `dynamorm:"pk"`
    UserID       string            `dynamorm:"index:user-connections,pk"`
    TenantID     string            `dynamorm:"index:tenant-connections,pk"`
    Endpoint     string            // API Gateway callback URL
    ConnectedAt  time.Time         
    LastPing     time.Time         
    Metadata     map[string]string `dynamorm:"json"`
    TTL          int64             `dynamorm:"ttl"` // 24 hours
}
```

### **Request Queue (DynamORM)**

#### AsyncRequest Model
```go
type AsyncRequest struct {
    RequestID         string                 `dynamorm:"pk"`
    ConnectionID      string                 `dynamorm:"index:connection-requests,pk"`
    Status            RequestStatus          `dynamorm:"index:status-time,pk"`
    CreatedAt         time.Time              `dynamorm:"index:status-time,sk"`
    Action            string                 
    Payload           map[string]interface{} `dynamorm:"json"`
    
    // Processing state
    ProcessingStarted *time.Time             
    ProcessingEnded   *time.Time             
    Progress          float64                
    ProgressMessage   string                 
    Result            map[string]interface{} `dynamorm:"json"`
    Error             string                 
    
    // Retry handling
    RetryCount        int                    
    MaxRetries        int                    
    RetryAfter        time.Time              
    
    // Multi-tenancy
    UserID            string                 
    TenantID          string                 
    TTL               int64                  `dynamorm:"ttl"` // 7 days
}

// Request statuses
const (
    StatusPending    RequestStatus = "PENDING"
    StatusProcessing RequestStatus = "PROCESSING"
    StatusCompleted  RequestStatus = "COMPLETED"
    StatusFailed     RequestStatus = "FAILED"
)
```

#### Request Queue Interface
```go
type RequestQueue interface {
    // Enqueue adds a request to the queue
    Enqueue(ctx context.Context, req *AsyncRequest) error
    
    // Get retrieves a specific request
    Get(ctx context.Context, requestID string) (*AsyncRequest, error)
    
    // Update request status
    UpdateStatus(ctx context.Context, requestID string, status RequestStatus, message string) error
    
    // Update progress
    UpdateProgress(ctx context.Context, requestID string, progress float64, message string, details map[string]interface{}) error
    
    // Complete request with result
    CompleteRequest(ctx context.Context, requestID string, result map[string]interface{}) error
    
    // Fail request with error
    FailRequest(ctx context.Context, requestID string, errMsg string) error
}
```

## 🧪 Testing Utilities & Mocks

### **Connection Manager Mocks**

#### Simple Send-Only Mock
```go
// For basic message sending tests
mock := connection.NewSendOnlyMock()

// Configure custom behavior
mock.SendFunc = func(ctx context.Context, connID string, msg interface{}) error {
    if connID == "banned" {
        return errors.New("user banned")
    }
    return nil // Default: store message
}

// Verify messages
messages := mock.GetMessages("conn-123")
assert.Len(t, messages, 1)
```

#### Progress Reporter Mock
```go
// For progress reporting tests
mock := connection.NewProgressReporterMock()

// Setup connection states
mock.SetActive("active-conn", true)
mock.SetActive("inactive-conn", false)

// Test progress reporting
reporter := progress.NewReporter("req-123", "active-conn", mock)
reporter.Report(50, "Halfway done!")

// Verify
messages := mock.GetMessages("active-conn")
assert.Len(t, messages, 1)
```

#### Full Connection Manager Mock
```go
// For comprehensive testing
mock := connection.NewMockConnectionManager()

// Configure all methods
mock.SendFunc = func(ctx context.Context, connID string, msg interface{}) error { return nil }
mock.IsActiveFunc = func(ctx context.Context, connID string) bool { return true }
mock.BroadcastFunc = func(ctx context.Context, connIDs []string, msg interface{}) error { return nil }

// Verify call counts
assert.Equal(t, 1, mock.CallCount("Send"))
```

### **Handler Testing Patterns**

#### Mock Handler
```go
type mockHandler struct {
    mock.Mock
}

func (m *mockHandler) EstimatedDuration() time.Duration {
    args := m.Called()
    return args.Get(0).(time.Duration)
}

func (m *mockHandler) Validate(req *streamer.Request) error {
    args := m.Called(req)
    return args.Error(0)
}

func (m *mockHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    args := m.Called(ctx, req)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*streamer.Result), args.Error(1)
}
```

#### Handler with Progress Mock
```go
type mockHandlerWithProgress struct {
    mockHandler
}

func (m *mockHandlerWithProgress) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    args := m.Called(ctx, req, reporter)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*streamer.Result), args.Error(1)
}
```

### **DynamORM Mocks**

#### Request Queue Mock
```go
type mockRequestQueue struct {
    mock.Mock
}

func (m *mockRequestQueue) Enqueue(ctx context.Context, req *store.AsyncRequest) error {
    args := m.Called(ctx, req)
    return args.Error(0)
}

func (m *mockRequestQueue) UpdateStatus(ctx context.Context, requestID string, status store.RequestStatus, message string) error {
    args := m.Called(ctx, requestID, status, message)
    return args.Error(0)
}

func (m *mockRequestQueue) UpdateProgress(ctx context.Context, requestID string, progress float64, message string, details map[string]interface{}) error {
    args := m.Called(ctx, requestID, progress, message, details)
    return args.Error(0)
}
```

### **API Gateway Mocks**

#### Testable API Gateway Client
```go
// For complex error simulation
testAPI := connection.NewTestableAPIGatewayClient()

// Add connections
testAPI.AddConnection("conn-123", &ConnectionInfo{IP: "192.168.1.1"})

// Simulate errors
testAPI.SimulateGoneError("conn-456")           // 410 Gone
testAPI.SimulateThrottling("conn-789", 5)       // 429 Rate Limited
testAPI.SetLatency(100 * time.Millisecond)      // Network latency

// Verify messages
messages := testAPI.GetMessages("conn-123")
assert.Len(t, messages, 1)
```

## 🏗️ Implementation Patterns & Best Practices

### **Handler Implementation Patterns**

#### Simple Sync Handler
```go
type EchoHandler struct{}

func (h *EchoHandler) EstimatedDuration() time.Duration {
    return 100 * time.Millisecond // Fast, will be processed synchronously
}

func (h *EchoHandler) Validate(req *streamer.Request) error {
    if len(req.Payload) == 0 {
        return streamer.NewError("VALIDATION_ERROR", "payload required")
    }
    return nil
}

func (h *EchoHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    var payload map[string]interface{}
    if err := req.UnmarshalPayload(&payload); err != nil {
        return nil, streamer.NewError("VALIDATION_ERROR", "invalid JSON payload")
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data:      payload,
    }, nil
}
```

#### Async Handler with Progress
```go
type ReportHandler struct{}

func (h *ReportHandler) EstimatedDuration() time.Duration {
    return 2 * time.Minute // Long-running, will be processed asynchronously
}

func (h *ReportHandler) Validate(req *streamer.Request) error {
    var params ReportParams
    if err := req.UnmarshalPayload(&params); err != nil {
        return streamer.NewError("VALIDATION_ERROR", "invalid payload format")
    }
    
    if params.StartDate.IsZero() || params.EndDate.IsZero() {
        return streamer.NewError("VALIDATION_ERROR", "start_date and end_date required")
    }
    
    return nil
}

func (h *ReportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    return nil, streamer.NewError("INTERNAL_ERROR", "use ProcessWithProgress for async handlers")
}

func (h *ReportHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    var params ReportParams
    if err := req.UnmarshalPayload(&params); err != nil {
        return nil, err
    }
    
    // Step 1: Initialize (10%)
    if err := reporter.Report(10, "Initializing report generation..."); err != nil {
        return nil, err
    }
    
    // Step 2: Gather data (30%)
    if err := reporter.Report(30, "Gathering data from database..."); err != nil {
        return nil, err
    }
    data, err := h.gatherData(ctx, params)
    if err != nil {
        return nil, err
    }
    
    // Step 3: Process data (60%)
    if err := reporter.Report(60, "Processing and analyzing data..."); err != nil {
        return nil, err
    }
    processedData, err := h.processData(ctx, data)
    if err != nil {
        return nil, err
    }
    
    // Step 4: Generate report (90%)
    if err := reporter.Report(90, "Generating final report..."); err != nil {
        return nil, err
    }
    reportURL, err := h.generateReport(ctx, processedData)
    if err != nil {
        return nil, err
    }
    
    // Complete with results
    return reporter.Complete(map[string]interface{}{
        "report_url": reportURL,
        "size_bytes": len(processedData),
        "pages":      15,
        "generated_at": time.Now(),
    })
}
```

### **Router Configuration Pattern**

```go
func setupRouter() *streamer.DefaultRouter {
    // Initialize dependencies
    requestQueue := dynamorm.NewRequestQueue(db)
    connectionManager := connection.NewManager(connStore, apiGatewayClient, endpoint)
    
    // Create router
    router := streamer.NewRouter(requestQueue, connectionManager)
    router.SetAsyncThreshold(5 * time.Second)
    
    // Register handlers
    router.Handle("echo", &EchoHandler{})
    router.Handle("health", &HealthHandler{})
    router.Handle("generate_report", &ReportHandler{})
    router.Handle("process_data", &DataProcessingHandler{})
    
    // Add middleware
    router.Use(
        LoggingMiddleware(logger),
        MetricsMiddleware(metrics),
        AuthMiddleware(jwtVerifier),
    )
    
    return router
}
```

### **Progress Reporting Patterns**

#### Basic Progress Pattern
```go
func (h *Handler) ProcessWithProgress(ctx context.Context, req *Request, reporter progress.Reporter) (*Result, error) {
    steps := []struct {
        percentage float64
        message    string
        action     func() error
    }{
        {10, "Validating input...", h.validate},
        {30, "Fetching data...", h.fetchData},
        {60, "Processing...", h.process},
        {90, "Finalizing...", h.finalize},
    }
    
    for _, step := range steps {
        if err := reporter.Report(step.percentage, step.message); err != nil {
            return nil, err
        }
        
        if err := step.action(); err != nil {
            return nil, err
        }
    }
    
    return reporter.Complete(result)
}
```

#### Progress with Metadata
```go
func (h *Handler) ProcessWithProgress(ctx context.Context, req *Request, reporter progress.Reporter) (*Result, error) {
    totalItems := 1000
    
    for i := 0; i < totalItems; i++ {
        // Process item
        if err := h.processItem(i); err != nil {
            return nil, err
        }
        
        // Report progress every 10 items
        if i%10 == 0 {
            percentage := float64(i) / float64(totalItems) * 100
            metadata := map[string]interface{}{
                "items_processed": i,
                "items_total":     totalItems,
                "current_item":    i,
            }
            
            if err := reporter.Report(percentage, fmt.Sprintf("Processing item %d/%d", i, totalItems), metadata); err != nil {
                return nil, err
            }
        }
    }
    
    return reporter.Complete(map[string]interface{}{
        "items_processed": totalItems,
        "success_rate":    0.95,
    })
}
```

### **Error Handling Patterns**

#### Standard Error Codes
```go
const (
    ErrCodeValidation     = "VALIDATION_ERROR"     // Request validation failed
    ErrCodeNotFound       = "NOT_FOUND"            // Resource not found
    ErrCodeUnauthorized   = "UNAUTHORIZED"         // Authentication failed
    ErrCodeForbidden      = "FORBIDDEN"            // Access denied
    ErrCodeRateLimited    = "RATE_LIMITED"         // Rate limit exceeded
    ErrCodeTimeout        = "TIMEOUT"              // Operation timed out
    ErrCodeInternalError  = "INTERNAL_ERROR"       // Internal server error
    ErrCodeServiceUnavail = "SERVICE_UNAVAILABLE"  // Service temporarily unavailable
)
```

#### Error Creation Helpers
```go
// Create basic error
func NewError(code, message string) *Error {
    return &Error{
        Code:    code,
        Message: message,
    }
}

// Create error with details
func NewErrorWithDetails(code, message string, details map[string]interface{}) *Error {
    return &Error{
        Code:    code,
        Message: message,
        Details: details,
    }
}

// Create retryable error
func NewRetryableError(code, message string, retryAfter time.Duration) *Error {
    return &Error{
        Code:    code,
        Message: message,
        Retry: &RetryInfo{
            Retryable: true,
            After:     time.Now().Add(retryAfter),
        },
    }
}
```

#### Error Handling in Handlers
```go
func (h *Handler) Validate(req *Request) error {
    var params MyParams
    if err := req.UnmarshalPayload(&params); err != nil {
        return NewErrorWithDetails(ErrCodeValidation, "Invalid payload format", map[string]interface{}{
            "error": err.Error(),
            "field": "payload",
        })
    }
    
    if params.StartDate.After(params.EndDate) {
        return NewError(ErrCodeValidation, "start_date must be before end_date")
    }
    
    return nil
}
```

### **Testing Patterns**

#### Unit Test Pattern
```go
func TestHandler_Process(t *testing.T) {
    tests := []struct {
        name        string
        request     *streamer.Request
        expectError bool
        errorCode   string
    }{
        {
            name: "valid request",
            request: &streamer.Request{
                ID:     "req-123",
                Action: "test",
                Payload: mustMarshal(map[string]interface{}{
                    "param1": "value1",
                }),
            },
            expectError: false,
        },
        {
            name: "invalid payload",
            request: &streamer.Request{
                ID:      "req-456",
                Action:  "test",
                Payload: []byte("invalid json"),
            },
            expectError: true,
            errorCode:   "VALIDATION_ERROR",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            handler := &TestHandler{}
            
            result, err := handler.Process(context.Background(), tt.request)
            
            if tt.expectError {
                assert.Error(t, err)
                if tt.errorCode != "" {
                    var streamerErr *streamer.Error
                    assert.True(t, errors.As(err, &streamerErr))
                    assert.Equal(t, tt.errorCode, streamerErr.Code)
                }
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, result)
                assert.True(t, result.Success)
            }
        })
    }
}
```

#### Integration Test Pattern
```go
func TestAsyncProcessing_EndToEnd(t *testing.T) {
    // Setup test environment
    mockDB := new(dynamocks.MockDB)
    mockQuery := new(dynamocks.MockQuery)
    mockConnMgr := connection.NewMockConnectionManager()
    
    // Setup mocks
    setupDynamoMocks(mockDB, mockQuery)
    mockConnMgr.SendFunc = func(ctx context.Context, connID string, msg interface{}) error {
        return nil
    }
    
    // Create components
    queue := dynamorm.NewRequestQueue(mockDB)
    executor := executor.New(mockConnMgr, queue, logger)
    
    // Register handler
    handler := &TestAsyncHandler{}
    executor.RegisterHandler("test-action", handler)
    
    // Create async request
    asyncReq := &store.AsyncRequest{
        RequestID:    "req-123",
        ConnectionID: "conn-456",
        Action:       "test-action",
        Status:       store.StatusPending,
        Payload:      map[string]interface{}{"data": "test"},
    }
    
    // Process request
    err := executor.ProcessRequest(context.Background(), asyncReq)
    assert.NoError(t, err)
    
    // Verify expectations
    mockDB.AssertExpectations(t)
    mockQuery.AssertExpectations(t)
    assert.Equal(t, 1, mockConnMgr.CallCount("Send"))
}
```

### **Lambda Function Patterns**

#### Connect Handler Pattern
```go
func main() {
    // Initialize dependencies
    cfg := loadConfig()
    connStore := dynamorm.NewConnectionStore(db)
    metrics := shared.NewCloudWatchMetrics(awsCfg, cfg.MetricsNamespace)
    
    // Create handler
    handler := NewHandler(connStore, cfg, metrics)
    
    // Start Lambda runtime
    lambda.Start(handler.Handle)
}

func (h *Handler) Handle(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Extract JWT token
    token := extractToken(event)
    if token == "" {
        return unauthorizedResponse("Missing authorization token")
    }
    
    // Validate JWT
    claims, err := h.jwtVerifier.Verify(token)
    if err != nil {
        return unauthorizedResponse("Invalid token")
    }
    
    // Create connection record
    conn := &store.Connection{
        ConnectionID: event.RequestContext.ConnectionID,
        UserID:       claims.Subject,
        TenantID:     claims.TenantID,
        Endpoint:     buildEndpoint(event),
        ConnectedAt:  time.Now(),
        TTL:          time.Now().Add(24 * time.Hour).Unix(),
    }
    
    // Save to DynamoDB
    if err := h.store.Save(ctx, conn); err != nil {
        return errorResponse("Failed to save connection")
    }
    
    return successResponse("Connected successfully")
}
```

#### Router Handler Pattern
```go
func main() {
    // Initialize components
    router := setupRouter()
    
    // Create Lambda handler
    handler := func(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
        return router.Route(ctx, event)
    }
    
    lambda.Start(handler)
}

func (r *DefaultRouter) Route(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Parse message
    var msg Message
    if err := json.Unmarshal([]byte(event.Body), &msg); err != nil {
        return r.sendError(event.RequestContext.ConnectionID, "Invalid message format")
    }
    
    // Get handler
    handler, exists := r.handlers[msg.Action]
    if !exists {
        return r.sendError(event.RequestContext.ConnectionID, "Unknown action")
    }
    
    // Create request
    req := &Request{
        ID:           msg.ID,
        ConnectionID: event.RequestContext.ConnectionID,
        Action:       msg.Action,
        Payload:      msg.Payload,
    }
    
    // Validate
    if err := handler.Validate(req); err != nil {
        return r.sendError(event.RequestContext.ConnectionID, err.Error())
    }
    
    // Check if async needed
    if handler.EstimatedDuration() > r.asyncThreshold {
        // Queue for async processing
        asyncReq := r.prepareAsyncRequest(req)
        if err := r.requestQueue.Enqueue(ctx, asyncReq); err != nil {
            return r.sendError(event.RequestContext.ConnectionID, "Failed to queue request")
        }
        
        return r.sendAck(event.RequestContext.ConnectionID, req.ID, "queued")
    }
    
    // Process synchronously
    result, err := handler.Process(ctx, req)
    if err != nil {
        return r.sendError(event.RequestContext.ConnectionID, err.Error())
    }
    
    return r.sendResponse(event.RequestContext.ConnectionID, req.ID, result)
}
```

### **DynamORM Usage Patterns**

#### Model Definition Pattern
```go
type MyModel struct {
    PK string `dynamorm:"pk"`                    // Partition key
    SK string `dynamorm:"sk"`                    // Sort key
    
    // Indexed fields
    UserID   string `dynamorm:"user_id" dynamorm-index:"user-index,pk"`
    TenantID string `dynamorm:"tenant_id" dynamorm-index:"tenant-index,pk"`
    Status   string `dynamorm:"status" dynamorm-index:"status-index,pk"`
    
    // Regular fields
    Data      map[string]interface{} `dynamorm:"json"`
    CreatedAt time.Time              `dynamorm:"created_at"`
    TTL       int64                  `dynamorm:"ttl"`
}
```

#### Query Patterns
```go
// Get single item
var item MyModel
err := db.Model(&MyModel{}).
    Where("pk", "=", pkValue).
    Where("sk", "=", skValue).
    First(&item)

// Query with GSI
var items []MyModel
err := db.Model(&MyModel{}).
    Index("user-index").
    Where("user_id", "=", userID).
    All(&items)

// Query with filters
var items []MyModel
err := db.Model(&MyModel{}).
    Where("status", "=", "PENDING").
    Where("created_at", ">", yesterday).
    Limit(100).
    All(&items)

// Update item
err := db.Model(&MyModel{}).
    Where("pk", "=", pkValue).
    Update(map[string]interface{}{
        "status": "COMPLETED",
        "updated_at": time.Now(),
    })
```

## 🚀 Quick Implementation Checklist

### **Creating a New Handler**
- [ ] Implement `Handler` interface (EstimatedDuration, Validate, Process)
- [ ] If async: Implement `HandlerWithProgress` interface
- [ ] Add proper error handling with standard error codes
- [ ] Write unit tests with mocks
- [ ] Register handler in router
- [ ] Add integration tests

### **Testing a Handler**
- [ ] Test validation logic with various inputs
- [ ] Test sync processing path
- [ ] Test async processing with progress updates
- [ ] Test error scenarios
- [ ] Test with connection manager mocks
- [ ] Verify progress reporting

### **Lambda Function Setup**
- [ ] Initialize DynamORM with proper configuration
- [ ] Set up connection manager with API Gateway client
- [ ] Configure JWT verification
- [ ] Add CloudWatch metrics
- [ ] Set up proper error handling
- [ ] Add structured logging

### **Common Gotchas**
- [ ] EstimatedDuration determines sync vs async (5s threshold)
- [ ] Progress updates are batched (200ms intervals)
- [ ] DynamORM uses PK/SK pattern for all models
- [ ] JWT tokens must include user_id and tenant_id claims
- [ ] Connection TTL is 24 hours by default
- [ ] Request TTL is 7 days by default

## 📖 Standard Error Codes Reference

| Code | Description | Retryable | Usage |
|------|-------------|-----------|-------|
| `VALIDATION_ERROR` | Request validation failed | No | Invalid payload, missing fields |
| `INVALID_ACTION` | Unknown action requested | No | Handler not registered |
| `NOT_FOUND` | Resource not found | No | Missing data, invalid IDs |
| `UNAUTHORIZED` | Authentication failed | No | Invalid JWT, missing token |
| `FORBIDDEN` | Access denied | No | Insufficient permissions |
| `RATE_LIMITED` | Rate limit exceeded | Yes | Too many requests |
| `INTERNAL_ERROR` | Internal server error | Yes | Unexpected errors |
| `TIMEOUT` | Operation timed out | Yes | Long-running operations |
| `SERVICE_UNAVAILABLE` | Service temporarily unavailable | Yes | Downstream service issues |

## 🔗 Key Resources

- **Architecture**: `docs/ARCHITECTURE.md` - System design and data flow
- **API Reference**: `docs/api/HANDLER_INTERFACE.md` - Complete API documentation
- **Development Guide**: `docs/guides/development.md` - Local setup and development
- **Testing Guide**: `pkg/connection/TESTING_GUIDE.md` - Testing patterns and mocks
- **DynamORM Migration**: `docs/guides/dynamorm-migration.md` - DynamORM usage patterns

---

*This guide provides comprehensive coverage of Streamer's architecture, types, and patterns. Use it as a reference when implementing handlers, writing tests, or debugging issues.* 