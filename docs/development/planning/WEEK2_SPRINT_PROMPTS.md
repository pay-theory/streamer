# Week 2 Sprint Prompts

## Team 1 - Sprint 2: Lambda Functions & Connection Management

### Context
Your storage layer is complete and Team 2 needs a ConnectionManager implementation. You'll create the Lambda functions and connection management system.

### Prompt
"Building on your completed storage layer, implement the AWS Lambda functions and connection management system:

1. **ConnectionManager Implementation** (`pkg/connection/`)
   ```go
   type ConnectionManager interface {
       // Send a message to a specific connection
       Send(ctx context.Context, connectionID string, message interface{}) error
       
       // Broadcast to multiple connections
       Broadcast(ctx context.Context, connectionIDs []string, message interface{}) error
       
       // Check if connection is active
       IsActive(ctx context.Context, connectionID string) bool
   }
   ```
   - Use API Gateway Management API for WebSocket communication
   - Integrate with your ConnectionStore for validation
   - Handle connection failures gracefully
   - Add retry logic for transient failures

2. **Connect Lambda Handler** (`lambda/connect/`)
   ```go
   // Extract JWT from query parameters
   // Validate token and extract user/tenant info
   // Create Connection record with TTL
   // Return appropriate status
   ```
   - JWT validation from `Authorization` query parameter
   - Extract claims: userID, tenantID, permissions
   - Set 24-hour TTL for connections
   - Return 401 for auth failures, 200 for success

3. **Disconnect Lambda Handler** (`lambda/disconnect/`)
   ```go
   // Clean up connection record
   // Cancel any active subscriptions
   // Log metrics for monitoring
   ```

4. **Lambda Deployment Configuration**
   - SAM template or Terraform modules
   - IAM roles with minimal permissions
   - Environment variables for configuration
   - Cold start optimization (minimal dependencies)

5. **Integration Requirements**
   - Expose ConnectionManager as a package for Team 2
   - Use structured logging with correlation IDs
   - Add CloudWatch metrics for connection lifecycle
   - Include integration test helpers

Share the ConnectionManager interface implementation with Team 2 by Tuesday EOD."

---

## Team 2 - Sprint 4: Complete Router Integration & Async Processor

### Context
Team 1's storage layer is ready, and you need to integrate it with your router. You'll also start building the async processor.

### Prompt
"Complete the router integration and build the async processing system:

1. **Router Integration** (`lambda/router/`)
   ```go
   // Create Lambda handler that uses your router
   func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
       // Initialize router with dependencies
       // Process the WebSocket event
       // Return appropriate response
   }
   ```
   - Initialize router with Team 1's RequestQueue
   - Use Team 1's ConnectionManager (available Tuesday)
   - Add request correlation IDs
   - Implement graceful error responses

2. **Async Request Processor** (`lambda/processor/`)
   ```go
   // DynamoDB Streams handler
   func handler(ctx context.Context, event events.DynamoDBEvent) error {
       // Parse stream records for new AsyncRequests
       // Look up and execute handlers
       // Report progress via ConnectionManager
       // Update request status in RequestQueue
   }
   ```
   - Process INSERT events from AsyncRequest table
   - Implement handler registry for async handlers
   - Add timeout management (max 15 minutes)
   - Handle batch processing efficiently

3. **Progress Reporter Implementation** (`pkg/progress/`)
   ```go
   type ProgressReporter interface {
       Report(percentage float64, message string) error
       SetMetadata(key string, value interface{}) error
       Complete(result interface{}) error
       Fail(err error) error
   }
   ```
   - Send real-time updates via WebSocket
   - Batch updates for efficiency (every 100ms)
   - Store progress in request metadata
   - Handle connection failures gracefully

4. **WebSocket Message Types**
   ```json
   // Progress Update
   {
       "type": "progress",
       "request_id": "req_123",
       "percentage": 45.5,
       "message": "Processing batch 2 of 4",
       "metadata": { "items_processed": 1250 }
   }
   
   // Completion
   {
       "type": "complete",
       "request_id": "req_123",
       "result": { ... }
   }
   ```

5. **Testing Requirements**
   - Integration tests with Team 1's components
   - Mock DynamoDB Streams events
   - Test progress reporting flow
   - Benchmark async processing throughput

Coordinate with Team 1 on ConnectionManager integration by Tuesday."

---

## 🤝 Joint Integration Tasks

### Monday: Integration Planning
Both teams meet to:
1. Finalize ConnectionManager interface
2. Agree on WebSocket message formats
3. Plan integration test scenarios
4. Set up shared test environment

### Tuesday: ConnectionManager Integration
- Team 1: Deliver ConnectionManager implementation
- Team 2: Integrate into router and processor
- Both: Run first integration tests

### Wednesday: End-to-End Testing
```bash
# Test flow:
1. Client connects (Team 1 Lambda)
2. Client sends sync request (Team 2 Lambda)
3. Client sends async request (Team 2 Lambda)
4. Processor handles async request (Team 2)
5. Client receives progress updates (Both teams)
6. Client disconnects (Team 1 Lambda)
```

### Thursday: Performance Testing
- Load test with 1000 concurrent connections
- Measure message latency (target < 50ms p99)
- Test async request throughput
- Profile Lambda cold starts

### Friday: Demo & Planning
- Demo end-to-end flow
- Review performance metrics
- Plan Week 3 priorities
- Address any integration issues

---

## 📋 Shared Code Structure

### Connection Package (Team 1 owns, Team 2 uses)
```
pkg/connection/
├── manager.go          # ConnectionManager implementation
├── api_gateway.go      # API Gateway Management API client
├── errors.go           # Connection-specific errors
└── manager_test.go     # Unit tests with mocks
```

### Progress Package (Team 2 owns, both use)
```
pkg/progress/
├── reporter.go         # ProgressReporter interface
├── websocket.go        # WebSocket implementation
├── batch.go            # Batch update logic
└── reporter_test.go    # Unit tests
```

### Lambda Shared Utilities
```
lambda/shared/
├── auth.go             # JWT validation (Team 1)
├── context.go          # Request context helpers
├── logging.go          # Structured logging
└── metrics.go          # CloudWatch metrics

```

---

## 🎯 Week 2 Success Criteria

### Team 1
- [ ] ConnectionManager fully implemented
- [ ] Connect/Disconnect Lambdas deployed
- [ ] JWT authentication working
- [ ] Integration tests passing

### Team 2  
- [ ] Router Lambda deployed and working
- [ ] Async processor handling requests
- [ ] Progress reporting functional
- [ ] Integration with Team 1 complete

### Joint
- [ ] End-to-end flow demonstrated
- [ ] Performance targets met
- [ ] No critical bugs
- [ ] Documentation updated 