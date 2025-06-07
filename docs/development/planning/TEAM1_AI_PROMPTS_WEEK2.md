# Team 1: Infrastructure & Core Systems - AI Assistant Prompts for Week 2

## Your Mission This Week
You are building the AWS Lambda infrastructure and connection management system for Streamer. Your work enables real-time WebSocket communication and integrates with the storage layer you built in Week 1.

## Context from Week 1
- ✅ You've completed the storage layer with DynamoDB models and interfaces
- ✅ ConnectionStore and RequestQueue are fully implemented
- ✅ Your code is in `internal/store/` with 80%+ test coverage

## Monday: ConnectionManager & Interface Design

### Primary Task
```
Implement a production-ready WebSocket ConnectionManager that Team 2's router will use.

ANALYZE FIRST:
1. Read pkg/streamer/router.go (lines 30-40) to understand what Team 2 expects
2. Check your internal/store/connection_store.go for available methods
3. Review internal/store/models.go for the Connection type

CREATE pkg/connection/manager.go:

package connection

import (
    "context"
    "github.com/pay-theory/streamer/internal/store"
    "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

type Manager struct {
    store      store.ConnectionStore
    apiGateway *apigatewaymanagementapi.Client
    endpoint   string
}

// Implement these methods:
// - Send(ctx, connectionID, message) error
// - Broadcast(ctx, connectionIDs, message) error  
// - IsActive(ctx, connectionID) bool

Key requirements:
- Validate connection exists before sending
- Handle 410 Gone by removing stale connections
- Retry 5xx errors up to 3 times with exponential backoff
- Log all operations with correlation IDs
- Marshal messages to JSON before sending

Also create:
- pkg/connection/errors.go with ErrConnectionNotFound, ErrConnectionStale
- pkg/connection/retry.go with backoff logic
- Comprehensive unit tests with mocked AWS clients
```

## Tuesday: Lambda Functions Implementation

### Morning: Complete ConnectionManager
```
Finalize the ConnectionManager implementation with production features:

ADD TO pkg/connection/manager.go:
1. Connection pooling for API Gateway client (max 100 connections)
2. Batch support in Broadcast (process in groups of 25)
3. Circuit breaker pattern for failing connections
4. Metrics collection (send latency, error rates)
5. Graceful shutdown handling

DELIVER to Team 2 by noon with:
- Clear usage examples
- Mock implementation for their tests
- Performance characteristics documented
```

### Afternoon: Connect Lambda Handler
```
Build the WebSocket $connect Lambda handler with JWT authentication.

CREATE lambda/connect/ directory with:

main.go:
- Lambda handler setup
- Dependency injection
- Environment variable loading

handler.go:
- Extract JWT from query string "Authorization" parameter
- Validate JWT and extract claims
- Create Connection record with TTL
- Return proper API Gateway response

auth.go:
- JWT validation using RS256
- Public key from JWT_PUBLIC_KEY env var
- Extract: userID (sub), tenantID (tenant_id), permissions[]
- Validate expiration and issuer

Connection record structure:
{
    ConnectionID: event.RequestContext.ConnectionID,
    UserID:      claims.Subject,
    TenantID:    claims.TenantID, 
    Endpoint:    event.RequestContext.DomainName + "/" + event.RequestContext.Stage,
    ConnectedAt: time.Now(),
    LastPing:    time.Now(),
    Metadata: {
        "user_agent": event.Headers["User-Agent"],
        "ip_address": event.RequestContext.Identity.SourceIP,
    },
    TTL: time.Now().Add(24 * time.Hour).Unix(),
}

Test cases to implement:
- Valid JWT → 200 OK
- Expired JWT → 401 Unauthorized
- Invalid signature → 401 Unauthorized
- Missing token → 401 Unauthorized
- DynamoDB error → 500 Internal Server Error
```

## Wednesday: Complete Lambda Infrastructure

### Morning: Disconnect Handler
```
Implement the $disconnect Lambda handler for cleanup.

CREATE lambda/disconnect/handler.go:

Requirements:
1. Delete connection from ConnectionStore
2. Query subscriptions by connection ID (future feature)
3. Cancel active subscriptions
4. Log metrics:
   - Connection duration
   - Total messages sent/received
   - Disconnect reason
5. ALWAYS return 200 OK (even if connection not found)

Include structured logging:
{
    "connection_id": "xxx",
    "user_id": "xxx",
    "duration_seconds": 3600,
    "messages_sent": 150,
    "disconnect_time": "2024-01-01T12:00:00Z"
}
```

### Afternoon: Integration Testing
```
Test your Lambda functions with Team 2's router.

CREATE tests/integration/connection_lifecycle_test.go:

Test the complete flow:
1. Generate valid JWT token
2. Call Connect handler → verify connection created
3. Use ConnectionManager.Send() → verify it works
4. Call Disconnect handler → verify cleanup
5. Try Send again → verify connection not found

Work with Team 2 to debug any integration issues.
```

## Thursday: Performance & Monitoring ✅

### Morning: Optimize ConnectionManager ✅
```
Add production optimizations to handle scale.

ENHANCE pkg/connection/manager.go:

1. Parallel sending for Broadcast: ✅
   - Use worker pool (10 workers)
   - Channel-based job distribution
   - Collect errors without blocking

2. Smart retry with jitter: ✅
   - Base delay: 100ms
   - Max delay: 5 seconds
   - Jitter: ±25% to prevent thundering herd

3. Connection health tracking: ✅
   - Mark connections as unhealthy after 3 failures
   - Skip unhealthy connections in broadcast
   - Periodic health check recovery

4. Performance metrics: ✅
   - Send latency histogram
   - Broadcast throughput
   - Error rate by type
   - Circuit breaker state

Run benchmarks: ✅
- Single Send: target < 10ms p99
- Broadcast 100: target < 50ms total
- Concurrent operations: 1000 goroutines
```

### Afternoon: Lambda Monitoring ✅
```
Add comprehensive monitoring to all Lambda functions.

IMPLEMENT in lambda/shared/monitoring.go:

1. CloudWatch custom metrics: ✅
   - ActiveConnections (gauge)
   - ConnectionsPerMinute (counter)
   - AuthenticationFailures (counter)
   - MessagesSentPerMinute (counter)

2. X-Ray tracing: ✅
   - Trace all AWS SDK calls
   - Custom segments for business logic
   - Annotate with user/tenant IDs

3. Structured logging standard: ✅
   {
       "timestamp": "ISO8601",
       "level": "INFO|WARN|ERROR",
       "correlation_id": "uuid",
       "user_id": "xxx",
       "tenant_id": "xxx",
       "operation": "connect|disconnect|send",
       "duration_ms": 123,
       "error": "error message if any"
   }

4. CloudWatch alarms: ✅
   - Error rate > 1% → Low severity
   - Error rate > 5% → High severity
   - Cold start duration > 1s
   - Concurrent executions > 80% of limit
```

## Friday: Production Readiness ✅

### Morning: Security & Deployment ✅
```
Finalize security and create deployment configuration.

SECURITY CHECKLIST: ✅
1. Lambda execution roles with minimal permissions
2. Secrets in AWS Secrets Manager (not env vars)
3. API Gateway resource policies
4. DynamoDB encryption at rest
5. CloudTrail logging enabled

CREATE deployment/pulumi/: ✅
1. Lambda function definitions
2. IAM roles and policies
3. API Gateway WebSocket API
4. DynamoDB table configuration
5. CloudWatch log groups
6. X-Ray tracing configuration

Environment-specific configs: ✅
- dev: Verbose logging, no alarms
- staging: Full monitoring, low alarm thresholds
- production: Optimized settings, normal alarms
```

### Afternoon: Demo Preparation ✅
```
Prepare your part of the end-to-end demo.

YOUR DEMO SECTIONS: ✅
1. Show connection flow in AWS Console
2. Demonstrate JWT validation (success and failure)
3. Show ConnectionManager sending messages
4. Display CloudWatch metrics dashboard
5. Demonstrate connection cleanup on disconnect

PREPARE: ✅
- Test JWT tokens (valid and invalid)
- CloudWatch dashboard showing key metrics
- X-Ray trace of complete connection flow
- Log insights queries for debugging
- Backup plan if live demo fails

KEY METRICS TO HIGHLIGHT: ✅
- Connection success rate: 99.9%
- Average connection duration: X minutes
- Messages per second capacity: 10,000+
- Cold start impact: < 100ms
```

## Debugging Guides

### WebSocket Connection Failures
```
1. Check API Gateway CloudWatch logs
2. Verify Lambda was invoked
3. Check Lambda logs for errors
4. Verify JWT validation logic
5. Check DynamoDB for connection record
6. Test with wscat or similar tool
```

### Performance Issues
```
1. Check X-Ray traces for slow operations
2. Look for DynamoDB throttling
3. Analyze Lambda cold starts
4. Review connection pool usage
5. Check for retry storms
```

### Integration Problems
```
1. Verify ConnectionManager interface matches Team 2's expectations
2. Check error types are properly exported
3. Verify JSON marshaling of messages
4. Test with Team 2's actual usage patterns
```

## Key Decisions to Document

1. **Why API Gateway Management API?**
   - Native WebSocket support
   - Automatic connection management
   - Built-in scaling

2. **Why 24-hour TTL?**
   - Balance between cleanup and long sessions
   - Matches JWT token expiration
   - Reduces storage costs

3. **Why connection pooling?**
   - Reduces Lambda cold start impact
   - Improves message send latency
   - Better resource utilization

4. **Why circuit breaker pattern?**
   - Prevents cascade failures
   - Faster failure detection
   - Automatic recovery

Remember: Your infrastructure is the foundation. Make it rock solid! 