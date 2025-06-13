# AI Assistant 2: Lambda Handlers & Business Logic Testing

## Context
You are tasked with improving unit test coverage for a Go-based WebSocket streaming service. The project includes several Lambda functions and business logic components that need comprehensive testing.

## Project Overview
- **Language**: Go
- **Architecture**: Serverless Lambda functions for WebSocket handling
- **Framework**: AWS Lambda with API Gateway WebSocket
- **Testing**: Standard Go testing with testify
- **Key Features**: Real-time streaming, async request processing, progress tracking

## Current Test Coverage

### ✅ Already Well-Tested
```
pkg/types/*                   - 100% coverage (DONE)
pkg/streamer/*                - 97.8% coverage (DONE)
pkg/progress/*                - 97.5% coverage (but has failing tests)
lambda/processor/executor/*   - 90.1% coverage (DONE)
lambda/connect/*              - 71.7% coverage (good enough)
lambda/disconnect/*           - 73.8% coverage (reference)
lambda/processor/handlers/*   - 62.3% coverage (acceptable)
```

### 🟡 Needs Improvement
```
lambda/shared/*               - 54.7% coverage (needs ~30% more)
```

### ❌ Build/Test Failures to Fix
```
lambda/router/*               - Build fails (missing WEBSOCKET_ENDPOINT env)
pkg/progress/*                - Batcher test failing (timing issue)
```

## Your Tasks

### 1. Lambda Connect Handler (`lambda/connect/`)
Create unit tests for WebSocket connection establishment:
- **Handler Logic** (skip complex JWT validation)
  - Connection validation
  - Storage of connection metadata
  - Response formatting
  - Error handling
  
- **Mock JWT Verification**
  ```go
  type mockJWTVerifier struct {
      mock.Mock
  }
  
  func (m *mockJWTVerifier) Verify(token string) (*Claims, error) {
      args := m.Called(token)
      if args.Get(0) == nil {
          return nil, args.Error(1)
      }
      return args.Get(0).(*Claims), args.Error(1)
  }
  ```

### 2. Lambda Router Handler (`lambda/router/`)
Test message routing functionality:
- **Route Registration**
  - Handler registration
  - Route matching
  - Default route handling
  
- **Message Processing**
  - Request validation
  - Handler invocation
  - Response formatting
  - Async vs sync decision making
  
- **Error Scenarios**
  - Unknown routes
  - Invalid payloads
  - Handler failures

### 3. Shared Utilities (`lambda/shared/`)
Test common functionality:
- **Auth Module**
  - Token parsing (mock crypto operations)
  - Claims extraction
  - Permission checking
  
- **Logging Module**
  - Structured logging
  - Context propagation
  - Log levels
  
- **Metrics Module**
  - Metric publishing
  - Dimension handling
  - Error handling
  
- **Tracing Module**
  - Segment creation (mock X-Ray)
  - Annotation handling
  - Error recording

### 4. Message Types (`pkg/types/`)
Test type definitions and factories:
- **Message Creation**
  ```go
  func TestNewMessage(t *testing.T) {
      tests := []struct {
          name     string
          msgType  string
          payload  interface{}
          wantErr  bool
      }{
          {
              name:    "valid request message",
              msgType: "request",
              payload: map[string]interface{}{"action": "test"},
              wantErr: false,
          },
          // More test cases
      }
  }
  ```
  
- **Message Validation**
  - Type validation
  - Required fields
  - Payload structure

### 5. Core Streaming Logic (`pkg/streamer/`)
Improve from 21.1% to 80%+:
- **Router Tests**
  - Handler registration
  - Middleware chain
  - Request routing
  
- **Adapter Tests**
  - Queue adapter
  - Connection adapter
  - Error mapping
  
- **Error Handling**
  - Error types
  - Error wrapping
  - Client vs server errors

## AWS Services Testing Guidelines

### ⚠️ Important: What NOT to Test

**Skip testing these AWS service interactions:**
- ❌ **CloudWatch Metrics** - Just mock to return nil
- ❌ **CloudWatch Logs** - Skip log testing
- ❌ **X-Ray Tracing** - Mock with environment variables
- ❌ **Lambda Context/Runtime** - Focus on handler logic
- ❌ **SQS/SNS/EventBridge** - Skip messaging infrastructure

**Why?** These services don't provide concrete types for testing and don't contain business logic.

### ✅ What TO Test

**Focus your testing on:**
- ✅ **Handler Business Logic** - Request validation, processing, response formatting
- ✅ **API Gateway Types** - Connection management, message sending (concrete types)
- ✅ **Error Handling** - Business errors, validation failures
- ✅ **Data Transformations** - Message formatting, type conversions
- ✅ **Routing Logic** - Route matching, handler selection

### Simplified Testing Pattern

```go
// Example: Lambda handler with metrics
func (h *Handler) Handle(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) error {
    // Just mock this to return nil - DON'T test metric logic
    h.metrics.PublishMetric("RequestReceived", 1)
    
    // DO test this validation logic
    if request.Body == "" {
        return &types.ValidationError{Message: "empty body"}
    }
    
    // DO test this business logic
    message, err := h.parseMessage(request.Body)
    if err != nil {
        return fmt.Errorf("parse error: %w", err)
    }
    
    // Just mock this to return nil
    h.tracer.AddAnnotation("messageType", message.Type)
    
    // DO test routing logic
    return h.router.Route(ctx, message)
}

// In your tests:
func TestHandler(t *testing.T) {
    mockMetrics := &mockMetricsPublisher{}
    mockMetrics.On("PublishMetric", mock.Anything, mock.Anything).Return(nil)
    
    mockTracer := &mockTracer{}
    mockTracer.On("AddAnnotation", mock.Anything, mock.Anything).Return(nil)
    
    // Focus test assertions on business logic, not AWS service calls
}
```

### Quick Mock Templates

```go
// Metrics - always returns nil
type mockMetrics struct{ mock.Mock }
func (m *mockMetrics) PublishMetric(name string, value float64) error {
    return nil // Don't even use mock.Called()
}

// X-Ray - use environment variable
func TestMain(m *testing.M) {
    os.Setenv("_X_AMZN_TRACE_ID", "Root=1-mock-trace-id")
    os.Exit(m.Run())
}
```

## Testing Patterns

### Lambda Handler Testing Pattern
```go
func TestHandler_Handle(t *testing.T) {
    tests := []struct {
        name           string
        event          events.APIGatewayWebsocketProxyRequest
        setupMocks     func(*mockStore, *mockMetrics)
        expectedStatus int
        expectedBody   string
    }{
        {
            name: "successful connection",
            event: events.APIGatewayWebsocketProxyRequest{
                RequestContext: events.APIGatewayWebsocketProxyRequestContext{
                    ConnectionID: "test-123",
                },
                QueryStringParameters: map[string]string{
                    "Authorization": "Bearer mock-token",
                },
            },
            setupMocks: func(store *mockStore, metrics *mockMetrics) {
                store.On("Save", mock.Anything, mock.Anything).Return(nil)
                metrics.On("PublishMetric", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
            },
            expectedStatus: 200,
            expectedBody:   `{"message":"Connected successfully"}`,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Mock External Dependencies
```go
// Mock AWS X-Ray
os.Setenv("_X_AMZN_TRACE_ID", "Root=1-5e13aa28-12345678901234567890abcd")

// Mock CloudWatch Metrics
type mockMetricsPublisher struct {
    mock.Mock
}

// Mock Store interfaces
type mockConnectionStore struct {
    mock.Mock
}
```

## File Structure Example
```
lambda/connect/
├── handler_test.go
├── auth_test.go
└── test_helpers_test.go

lambda/router/
├── handler_test.go
├── routes_test.go
└── middleware_test.go

lambda/shared/
├── auth_test.go
├── logging_test.go
├── metrics_test.go
└── tracing_test.go

pkg/types/
├── messages_test.go
├── errors_test.go
└── validation_test.go
```

## Testing Guidelines

### Avoid Complex Setup
- Use simple mock tokens instead of real JWT signing
- Mock AWS services rather than using real clients
- Use in-memory stores for testing

### Focus on Business Logic
```go
// Don't test AWS Lambda runtime
// DO test your handler logic
func (h *Handler) processConnection(ctx context.Context, connectionID string, claims *Claims) error {
    // This is what you should test
}
```

### Table-Driven Tests
- Group similar test cases
- Use descriptive test names
- Test edge cases and errors

### Performance Considerations
- Keep tests fast (< 100ms per test)
- Avoid time.Sleep() - use time mocking
- Parallel test execution where safe

## Coverage Goals
- **Minimum**: 80% coverage per package
- **Focus Areas**: 
  - Error handling paths
  - Business logic branches
  - Edge cases
- **Skip**: Generated code, simple getters/setters

## Common Mocks to Create
```go
// JWT Verifier
type JWTVerifier interface {
    Verify(token string) (*Claims, error)
}

// Message Publisher
type Publisher interface {
    Send(ctx context.Context, connectionID string, message interface{}) error
}

// Queue Interface
type Queue interface {
    Enqueue(ctx context.Context, message *Message) error
    Dequeue(ctx context.Context) (*Message, error)
}
```

## Notes
- Reference the well-tested `lambda/disconnect` package for patterns
- Mock complex external dependencies (JWT, AWS services)
- Test handlers in isolation from AWS Lambda runtime
- Use context for cancellation testing
- Consider using `github.com/stretchr/testify/suite` for complex test setups

## Next Steps

### Priority 1: Fix Lambda Router Build (lambda/router/)
**Current**: Build failing → **Target**: 80%+ coverage

1. **Fix environment variable issue**:
   ```go
   // Add to test file or TestMain
   func TestMain(m *testing.M) {
       os.Setenv("WEBSOCKET_ENDPOINT", "wss://mock.execute-api.us-east-1.amazonaws.com/dev")
       os.Setenv("AWS_REGION", "us-east-1")
       os.Setenv("_X_AMZN_TRACE_ID", "Root=1-mock-trace")
       
       code := m.Run()
       os.Exit(code)
   }
   ```

2. **Test routing functionality**:
   - Route registration and matching
   - Message dispatch logic
   - Sync vs async decision making
   - Error handling
   - **Skip**: Route metrics, performance tracking

### Priority 2: Fix Progress Package Tests (pkg/progress/)
**Current**: 97.5% coverage but failing → **Target**: All tests passing

1. **Fix batcher shutdown test**:
   ```go
   // The issue is likely timing-related
   // Ensure proper synchronization
   func TestBatcherShutdown(t *testing.T) {
       // Add proper wait/sync mechanisms
       // Use channels or sync.WaitGroup
   }
   ```

2. **Common patterns for timing-sensitive tests**:
   ```go
   // Use channels for synchronization
   done := make(chan struct{})
   go func() {
       // Do work
       close(done)
   }()
   
   select {
   case <-done:
       // Success
   case <-time.After(time.Second):
       t.Fatal("timeout")
   }
   ```

### Priority 3: Improve Lambda Shared (lambda/shared/)
**Current**: 54.7% coverage → **Target**: 85%+

1. **Identify uncovered code**:
   ```bash
   go test -coverprofile=coverage.out ./lambda/shared/...
   go tool cover -html=coverage.out -o shared_coverage.html
   open shared_coverage.html
   ```

2. **Focus on business logic**:
   - Auth validation logic (not JWT crypto)
   - Logging formatters
   - Error handling utilities
   - **Skip**: AWS service calls, just mock them

3. **Simple mock patterns**:
   ```go
   // For metrics - just return nil
   type mockMetrics struct{}
   func (m *mockMetrics) PublishMetric(name string, value float64) error {
       return nil
   }
   ```

### Priority 4: Minor Touch-ups

1. **lambda/processor/handlers** (62.3% → 70%):
   - Add a few more error cases
   - Test edge conditions

2. **lambda/connect** (71.7% → 75%):
   - Minor improvements if easy wins available

### Execution Timeline
- **Day 1**: Fix lambda/router build and add tests
- **Day 2**: Fix pkg/progress test failures + improve lambda/shared
- **Optional**: Minor improvements to other packages

### Key Testing Strategies

1. **Environment setup template**:
   ```go
   func TestMain(m *testing.M) {
       // Required for all Lambda tests
       os.Setenv("AWS_REGION", "us-east-1")
       os.Setenv("_X_AMZN_TRACE_ID", "Root=1-mock-trace")
       os.Setenv("WEBSOCKET_ENDPOINT", "wss://mock.endpoint.com")
       
       code := m.Run()
       os.Exit(code)
   }
   ```

2. **Skip AWS service testing**:
   ```go
   // Don't test these - just mock to return nil:
   // - CloudWatch metrics
   // - X-Ray tracing
   // - CloudWatch logs
   ```

### Success Metrics
- lambda/router building and tested (80%+)
- All pkg/progress tests passing
- lambda/shared at 85%+ coverage
- Overall project coverage above 75%
- Zero build failures 