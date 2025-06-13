# AI Assistant Prompt: Lambda Migration to Lift

**Purpose:** Guide AI assistant in migrating Streamer lambda functions to use Lift framework  
**Context:** Days 2-4 of Lift integration sprint  
**Focus:** Connect, Disconnect, Router, and Processor handlers

## Prompt

You are helping migrate the Streamer WebSocket lambda functions to use the Pay Theory Lift framework. You have access to the Lift integration guide and example code. Your goal is to systematically migrate each lambda while maintaining all existing functionality and improving performance.

### Migration Principles:

1. **Preserve Business Logic** - Keep all existing functionality intact
2. **Leverage Lift Features** - Use Lift's middleware, error handling, and observability
3. **Improve Code Quality** - Reduce boilerplate and complexity
4. **Maintain Testability** - Ensure all code remains testable

### For Each Lambda Migration:

#### 1. Analysis Phase
- Review the current lambda implementation
- Identify boilerplate code that Lift can eliminate
- Map current functionality to Lift patterns
- Plan the migration approach

#### 2. Implementation Phase
- Create new Lift-based handler structure
- Migrate authentication/authorization to Lift middleware
- Implement business logic using Lift patterns
- Add comprehensive error handling
- Integrate observability (logging, metrics, tracing)

#### 3. Testing Phase
- Write unit tests for the new implementation
- Create integration tests
- Validate all existing functionality works
- Ensure performance improvements

### Lambda-Specific Guidance:

#### Connect Handler
```go
// Current pattern to migrate:
func handler(ctx context.Context, request events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Manual JWT validation
    // Manual error handling
    // Manual logging
}

// Target Lift pattern:
func NewConnectHandler() *lift.Handler {
    return lift.New(
        lift.WithMiddleware(
            middleware.JWT(),
            middleware.Logging(),
            middleware.Metrics(),
        ),
    ).Handle(connectHandler)
}
```

Focus areas:
- JWT validation via Lift middleware
- Connection storage with proper error handling
- WebSocket response formatting
- Correlation ID propagation

#### Disconnect Handler
Focus areas:
- Graceful cleanup of connections
- Subscription removal
- Error recovery
- Audit logging

#### Router Handler
Focus areas:
- Message type validation
- Async queue integration
- Request/response correlation
- Rate limiting consideration

#### Processor Handler
Focus areas:
- DynamoDB Streams batch processing
- WebSocket notification delivery
- Error handling and retries
- Dead letter queue integration

### Code Quality Checklist:

- [ ] Removed all boilerplate initialization code
- [ ] Implemented proper error handling with Lift patterns
- [ ] Added structured logging with correlation IDs
- [ ] Integrated metrics collection
- [ ] Enabled distributed tracing
- [ ] Simplified context propagation
- [ ] Reduced cyclomatic complexity
- [ ] Maintained or improved test coverage

### Common Patterns to Apply:

```go
// Error handling
if err != nil {
    return lift.Error(ctx, err).
        WithStatus(http.StatusBadRequest).
        WithDetail("Failed to process request")
}

// Logging with context
lift.Logger(ctx).
    With("connectionId", connectionID).
    Info("Connection established")

// Metrics
lift.Metrics(ctx).
    Counter("websocket.connections").
    Inc()
```

### Testing Requirements:

1. **Unit Tests**
   - Test each middleware component
   - Test business logic in isolation
   - Mock AWS services appropriately

2. **Integration Tests**
   - Test full request/response flow
   - Validate WebSocket interactions
   - Test error scenarios

3. **Performance Tests**
   - Measure cold start improvement
   - Validate latency reduction
   - Check memory usage

### Migration Validation:

Before considering a lambda migrated:
1. All tests pass (unit and integration)
2. Code coverage maintained or improved
3. Performance metrics show improvement
4. No functionality regression
5. Documentation updated

Remember: The goal is not just to make it work with Lift, but to leverage Lift's features to create cleaner, more maintainable, and more performant code. 