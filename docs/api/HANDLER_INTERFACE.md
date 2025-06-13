# Handler Interface API Reference

This document provides complete API reference for Streamer's handler interfaces, types, and methods.

## Core Interfaces

### Handler Interface

The base interface that all handlers must implement.

```go
type Handler interface {
    // EstimatedDuration returns how long this handler is expected to take
    EstimatedDuration() time.Duration
    
    // Validate checks if the incoming request is valid
    Validate(req *Request) error
    
    // Process handles synchronous requests (< 5 seconds)
    Process(ctx context.Context, req *Request) (*Result, error)
}
```

### HandlerWithProgress Interface

Extended interface for handlers that support async processing with progress updates.

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

## Core Types

### Request

Represents an incoming WebSocket request.

```go
type Request struct {
    // Unique request identifier
    ID string `json:"id"`
    
    // WebSocket connection ID (set by router)
    ConnectionID string `json:"-"`
    
    // User and tenant context (set by auth middleware)
    UserID   string `json:"-"`
    TenantID string `json:"-"`
    
    // Action to perform
    Action string `json:"action"`
    
    // Request payload (JSON bytes)
    Payload []byte `json:"payload"`
    
    // Additional metadata
    Metadata map[string]interface{} `json:"-"`
    
    // Request timestamp
    Timestamp time.Time `json:"-"`
}
```

#### Methods

```go
// UnmarshalPayload unmarshals the payload into the provided struct
func (r *Request) UnmarshalPayload(v interface{}) error

// GetMetadata retrieves metadata value by key
func (r *Request) GetMetadata(key string) (interface{}, bool)

// SetMetadata sets a metadata value
func (r *Request) SetMetadata(key string, value interface{})
```

### Result

Represents the result of processing a request.

```go
type Result struct {
    // Request ID this result corresponds to
    RequestID string `json:"request_id"`
    
    // Whether the operation succeeded
    Success bool `json:"success"`
    
    // Result data (will be JSON marshaled)
    Data interface{} `json:"data,omitempty"`
    
    // Error information if Success is false
    Error *Error `json:"error,omitempty"`
    
    // Processing metadata
    Metadata map[string]interface{} `json:"metadata,omitempty"`
    
    // Processing duration
    Duration time.Duration `json:"-"`
}
```

### Error

Structured error information.

```go
type Error struct {
    // Error code (see error codes reference)
    Code string `json:"code"`
    
    // Human-readable error message
    Message string `json:"message"`
    
    // Additional error details
    Details map[string]interface{} `json:"details,omitempty"`
    
    // Retry information
    Retry *RetryInfo `json:"retry,omitempty"`
}
```

### RetryInfo

Information about retry behavior for failed requests.

```go
type RetryInfo struct {
    // Whether this error is retryable
    Retryable bool `json:"retryable"`
    
    // When to retry (if retryable)
    After time.Time `json:"after,omitempty"`
    
    // Maximum number of retries
    MaxTries int `json:"max_tries,omitempty"`
    
    // Current retry attempt
    Attempt int `json:"attempt,omitempty"`
}
```

## Handler Implementation Examples

### Simple Sync Handler

```go
type EchoHandler struct{}

func (h *EchoHandler) EstimatedDuration() time.Duration {
    return 100 * time.Millisecond // Fast, will be processed synchronously
}

func (h *EchoHandler) Validate(req *streamer.Request) error {
    if len(req.Payload) == 0 {
        return streamer.NewError(streamer.ErrCodeValidation, "payload required")
    }
    return nil
}

func (h *EchoHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    var payload map[string]interface{}
    if err := req.UnmarshalPayload(&payload); err != nil {
        return nil, streamer.NewError(streamer.ErrCodeValidation, "invalid JSON payload")
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data:      payload,
    }, nil
}
```

### Async Handler with Progress

```go
type ReportHandler struct{}

func (h *ReportHandler) EstimatedDuration() time.Duration {
    return 2 * time.Minute // Long-running, will be processed asynchronously
}

func (h *ReportHandler) Validate(req *streamer.Request) error {
    var params ReportParams
    if err := req.UnmarshalPayload(&params); err != nil {
        return streamer.NewError(streamer.ErrCodeValidation, "invalid payload format")
    }
    
    if params.StartDate.IsZero() || params.EndDate.IsZero() {
        return streamer.NewError(streamer.ErrCodeValidation, "start_date and end_date required")
    }
    
    return nil
}

func (h *ReportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    return nil, streamer.NewError(streamer.ErrCodeInternalError, "use ProcessWithProgress for async handlers")
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
    
    // Step 1: Initialize
    if err := reporter.Report(10, "Initializing report generation..."); err != nil {
        return nil, err
    }
    
    // Step 2: Gather data
    if err := reporter.Report(30, "Gathering data from database..."); err != nil {
        return nil, err
    }
    data, err := h.gatherData(ctx, params)
    if err != nil {
        return nil, err
    }
    
    // Step 3: Process data
    if err := reporter.Report(60, "Processing and analyzing data..."); err != nil {
        return nil, err
    }
    processedData, err := h.processData(ctx, data)
    if err != nil {
        return nil, err
    }
    
    // Step 4: Generate report
    if err := reporter.Report(90, "Generating final report..."); err != nil {
        return nil, err
    }
    reportURL, err := h.generateReport(ctx, processedData)
    if err != nil {
        return nil, err
    }
    
    // Complete
    return reporter.Complete(map[string]interface{}{
        "report_url": reportURL,
        "size_bytes": len(processedData),
        "generated_at": time.Now(),
    })
}
```

## Router Interface

The router manages handler registration and request routing.

```go
type Router interface {
    // Handle registers a handler for an action
    Handle(action string, handler Handler) error
    
    // Route processes an incoming WebSocket request
    Route(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error
    
    // SetAsyncThreshold sets the duration threshold for async processing
    SetAsyncThreshold(threshold time.Duration)
    
    // SetMiddleware adds middleware to the processing chain
    SetMiddleware(middleware ...Middleware)
    
    // SetFallback sets a fallback handler for unknown actions
    SetFallback(handler Handler)
}
```

### DefaultRouter

The default router implementation.

```go
type DefaultRouter struct {
    handlers        map[string]Handler
    asyncThreshold  time.Duration
    middleware      []Middleware
    fallbackHandler Handler
    requestQueue    RequestQueue
    connManager     ConnectionManager
}

// NewRouter creates a new router instance
func NewRouter(requestQueue RequestQueue, connManager ConnectionManager) *DefaultRouter
```

## Middleware

Middleware allows you to wrap handlers with additional functionality.

```go
type Middleware func(Handler) Handler

// Example: Logging middleware
func LoggingMiddleware(logger func(string, ...interface{})) Middleware {
    return func(next Handler) Handler {
        return HandlerFunc{
            EstimatedDurationFunc: next.EstimatedDuration,
            ValidateFunc: next.Validate,
            ProcessFunc: func(ctx context.Context, req *Request) (*Result, error) {
                start := time.Now()
                logger("Processing request %s for action %s", req.ID, req.Action)
                
                result, err := next.Process(ctx, req)
                
                duration := time.Since(start)
                if err != nil {
                    logger("Request %s failed after %v: %v", req.ID, duration, err)
                } else {
                    logger("Request %s completed after %v", req.ID, duration)
                }
                
                return result, err
            },
        }
    }
}
```

## Helper Functions

### Creating Handlers

```go
// NewHandlerFunc creates a handler from functions
func NewHandlerFunc(
    processFunc func(context.Context, *Request) (*Result, error),
    duration time.Duration,
    validateFunc func(*Request) error,
) Handler

// SimpleHandler creates a basic handler for simple operations
func SimpleHandler(action string, processFunc func(context.Context, *Request) (*Result, error)) Handler
```

### Error Creation

```go
// NewError creates a new structured error
func NewError(code, message string) *Error

// NewErrorWithDetails creates an error with additional details
func NewErrorWithDetails(code, message string, details map[string]interface{}) *Error

// NewRetryableError creates an error that can be retried
func NewRetryableError(code, message string, retryAfter time.Duration) *Error
```

## Standard Error Codes

| Code | Description | Retryable |
|------|-------------|-----------|
| `VALIDATION_ERROR` | Request validation failed | No |
| `INVALID_ACTION` | Unknown action requested | No |
| `NOT_FOUND` | Resource not found | No |
| `UNAUTHORIZED` | Authentication failed | No |
| `FORBIDDEN` | Access denied | No |
| `RATE_LIMITED` | Rate limit exceeded | Yes |
| `INTERNAL_ERROR` | Internal server error | Yes |
| `TIMEOUT` | Operation timed out | Yes |
| `SERVICE_UNAVAILABLE` | Service temporarily unavailable | Yes |
| `PROCESSING_FAILED` | Handler processing failed | Depends |

## Best Practices

### Handler Design

1. **Keep EstimatedDuration Accurate**: This determines sync vs async processing
2. **Validate Early**: Perform all validation in the `Validate` method
3. **Handle Context Cancellation**: Check `ctx.Done()` in long-running operations
4. **Use Structured Errors**: Provide meaningful error codes and details
5. **Report Progress Regularly**: For async handlers, update progress every few seconds

### Error Handling

1. **Use Appropriate Error Codes**: Choose the most specific error code
2. **Provide Helpful Messages**: Include actionable information in error messages
3. **Set Retry Information**: Indicate if and when operations can be retried
4. **Log Errors**: Use structured logging for debugging

### Performance

1. **Minimize Validation Logic**: Keep validation fast and simple
2. **Use Connection Pooling**: Reuse database connections where possible
3. **Batch Operations**: Group related operations together
4. **Monitor Duration**: Ensure EstimatedDuration matches actual performance

This API reference provides the foundation for building robust, scalable handlers with Streamer. 