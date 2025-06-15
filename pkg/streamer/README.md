# Streamer Router Package

The `streamer` package provides the core request routing functionality for the Streamer async request processing system. It handles WebSocket message routing, sync/async decision logic, request validation, and middleware support.

## Overview

The router is designed to:
- Receive WebSocket messages from API Gateway
- Route requests to appropriate handlers based on action
- Automatically decide between sync and async processing
- Validate requests before processing
- Support middleware for cross-cutting concerns
- Handle errors gracefully with structured responses

## Quick Start

```go
import (
    "github.com/streamer/streamer/pkg/streamer"
)

// Create a router
router := streamer.NewRouter(requestStore, connectionManager)

// Register a handler
handler := streamer.NewEchoHandler()
router.Handle("echo", handler)

// Set async threshold (optional, default is 5 seconds)
router.SetAsyncThreshold(10 * time.Second)

// Process WebSocket events
err := router.Route(ctx, websocketEvent)
```

## Core Concepts

### Handlers

Handlers process incoming requests. They must implement the `Handler` interface:

```go
type Handler interface {
    Validate(request *Request) error
    EstimatedDuration() time.Duration
    Process(ctx context.Context, request *Request) (*Result, error)
}
```

### Sync vs Async Processing

The router automatically decides whether to process a request synchronously or asynchronously based on the handler's `EstimatedDuration()`:

- **Sync**: If duration < async threshold, process immediately and return response
- **Async**: If duration >= async threshold, queue the request and return acknowledgment

### Message Format

#### Request Message
```json
{
    "action": "process-data",
    "id": "req-123",
    "payload": {
        "data": "example"
    },
    "metadata": {
        "user_id": "user-456"
    }
}
```

#### Response Types

**Sync Response:**
```json
{
    "type": "response",
    "request_id": "req-123",
    "success": true,
    "data": {
        "result": "processed"
    }
}
```

**Async Acknowledgment:**
```json
{
    "type": "acknowledgment",
    "request_id": "req-123",
    "status": "queued",
    "message": "Request queued for async processing"
}
```

**Error Response:**
```json
{
    "type": "error",
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Invalid payload format",
        "details": {
            "field": "email"
        }
    }
}
```

## Creating Handlers

### Simple Handler

Use `SimpleHandler` for basic handlers:

```go
handler := streamer.SimpleHandler("greet", func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    var payload struct {
        Name string `json:"name"`
    }
    
    if err := json.Unmarshal(req.Payload, &payload); err != nil {
        return nil, err
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]string{
            "message": fmt.Sprintf("Hello, %s!", payload.Name),
        },
    }, nil
})

router.Handle("greet", handler)
```

### Custom Handler with Validation

```go
type CreateUserHandler struct {
    streamer.BaseHandler
    userService UserService
}

func NewCreateUserHandler(userService UserService) *CreateUserHandler {
    h := &CreateUserHandler{
        BaseHandler: streamer.BaseHandler{
            EstimatedDuration: 200 * time.Millisecond,
        },
        userService: userService,
    }
    
    h.Validator = func(req *streamer.Request) error {
        var payload CreateUserPayload
        if err := json.Unmarshal(req.Payload, &payload); err != nil {
            return fmt.Errorf("invalid payload format")
        }
        
        if payload.Email == "" {
            return fmt.Errorf("email is required")
        }
        
        if !isValidEmail(payload.Email) {
            return fmt.Errorf("invalid email format")
        }
        
        return nil
    }
    
    return h
}

func (h *CreateUserHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    var payload CreateUserPayload
    json.Unmarshal(req.Payload, &payload)
    
    user, err := h.userService.Create(ctx, payload)
    if err != nil {
        return nil, err
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data:      user,
    }, nil
}
```

### Handler with Progress Reporting

For async handlers that support progress updates:

```go
type ReportGeneratorHandler struct {
    streamer.BaseHandler
}

func (h *ReportGeneratorHandler) ProcessWithProgress(
    ctx context.Context, 
    req *streamer.Request, 
    reporter streamer.ProgressReporter,
) (*streamer.Result, error) {
    // Parse request
    var params ReportParams
    json.Unmarshal(req.Payload, &params)
    
    // Step 1: Load data (30%)
    reporter.Report(10, "Loading data...")
    data := loadData(params)
    reporter.Report(30, "Data loaded")
    
    // Step 2: Process data (60%)
    reporter.Report(40, "Processing data...")
    processed := processData(data)
    reporter.Report(60, "Data processed")
    
    // Step 3: Generate report (100%)
    reporter.Report(70, "Generating report...")
    report := generateReport(processed)
    reporter.Report(100, "Report complete")
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]string{
            "report_url": report.URL,
        },
    }, nil
}
```

## Middleware

Middleware allows you to add cross-cutting concerns like logging, metrics, and authentication:

```go
// Logging middleware
loggingMiddleware := func(next streamer.Handler) streamer.Handler {
    return streamer.NewHandlerFunc(
        func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
            start := time.Now()
            log.Printf("Processing request: %s, action: %s", req.ID, req.Action)
            
            result, err := next.Process(ctx, req)
            
            duration := time.Since(start)
            if err != nil {
                log.Printf("Request failed: %s, duration: %v, error: %v", req.ID, duration, err)
            } else {
                log.Printf("Request completed: %s, duration: %v", req.ID, duration)
            }
            
            return result, err
        },
        next.EstimatedDuration(),
        next.Validate,
    )
}

// Authentication middleware
authMiddleware := func(next streamer.Handler) streamer.Handler {
    return streamer.NewHandlerFunc(
        func(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
            // Extract auth token from metadata
            token, ok := req.Metadata["auth_token"]
            if !ok {
                return nil, streamer.NewError(
                    streamer.ErrCodeUnauthorized, 
                    "Authentication required",
                )
            }
            
            // Validate token
            userID, err := validateToken(token)
            if err != nil {
                return nil, streamer.NewError(
                    streamer.ErrCodeUnauthorized,
                    "Invalid authentication token",
                )
            }
            
            // Add user ID to context
            ctx = context.WithValue(ctx, "user_id", userID)
            
            return next.Process(ctx, req)
        },
        next.EstimatedDuration(),
        next.Validate,
    )
}

// Apply middleware
router.SetMiddleware(loggingMiddleware, authMiddleware)
```

## Error Handling

Use structured errors for consistent error responses:

```go
// Create an error with details
err := streamer.NewError(
    streamer.ErrCodeValidation,
    "Invalid input data",
).WithDetail("field", "email").WithDetail("reason", "invalid format")

// Common error codes
const (
    ErrCodeValidation      = "VALIDATION_ERROR"
    ErrCodeNotFound        = "NOT_FOUND"
    ErrCodeUnauthorized    = "UNAUTHORIZED"
    ErrCodeInternalError   = "INTERNAL_ERROR"
    ErrCodeTimeout         = "TIMEOUT"
    ErrCodeRateLimited     = "RATE_LIMITED"
    ErrCodeInvalidAction   = "INVALID_ACTION"
)
```

## Testing

The package includes comprehensive test utilities:

```go
// Create mock dependencies
store := &MockRequestStore{}
connManager := NewMockConnectionManager()
router := streamer.NewRouter(store, connManager)

// Register handler
router.Handle("test", handler)

// Create test event
event := events.APIGatewayWebsocketProxyRequest{
    Body: `{"action": "test", "id": "test-123"}`,
    RequestContext: events.APIGatewayWebsocketProxyRequestContext{
        ConnectionID: "conn-123",
    },
}

// Route and verify
err := router.Route(context.Background(), event)

// Check responses
messages := connManager.Messages["conn-123"]
// ... verify messages
```

## Best Practices

1. **Accurate Duration Estimates**: Provide accurate `EstimatedDuration()` to ensure proper sync/async routing
2. **Comprehensive Validation**: Validate requests thoroughly in the `Validate()` method
3. **Structured Errors**: Use the provided error types for consistent error handling
4. **Context Propagation**: Pass context through the entire request lifecycle
5. **Idempotent Handlers**: Design handlers to be idempotent for retry safety
6. **Progress Reporting**: For long operations, implement `ProcessWithProgress`
7. **Middleware Order**: Apply middleware in the correct order (outer to inner)

## Integration with Team 1

The router depends on interfaces that Team 1 will implement:

- `RequestStore`: For queuing async requests to DynamoDB
- `ConnectionManager`: For sending WebSocket messages via API Gateway

These interfaces allow for easy mocking and testing while Team 1 builds the infrastructure. 