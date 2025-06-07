# Streamer Architecture

## System Overview

Streamer implements a distributed async request processing system optimized for AWS Lambda and API Gateway WebSockets. The architecture separates fast routing from long-running processing to overcome API Gateway's 29-second timeout limitation.

## Core Components

### 1. Connection Manager

Manages WebSocket connections with DynamoDB persistence for durability across Lambda invocations.

```go
// Internal implementation
type connectionManager struct {
    db        *dynamorm.DB
    tableName string
    ttl       time.Duration
}

// Connection lifecycle
Connect    → Store in DynamoDB with TTL
Disconnect → Remove from DynamoDB  
Ping       → Update LastPing timestamp
Prune      → Remove stale connections (TTL expired)
```

**Key Design Decisions:**
- DynamoDB for persistence (survives Lambda cold starts)
- TTL for automatic cleanup of dead connections
- GSI on TenantID for efficient multi-tenant queries
- Connection metadata stored as JSON for flexibility

### 2. Request Router (Lambda 1)

Fast Lambda function that acknowledges requests immediately and queues long-running work.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│   Router    │────▶│  DynamoDB   │
│ (WebSocket) │◀────│  (Lambda)   │     │   Queue     │
└─────────────┘     └─────────────┘     └─────────────┘
      │                                         │
      │                                         ▼
      │                                   ┌───────────┐
      │◀──────────────────────────────────│ Processor │
                                          │ (Lambda)  │
                                          └───────────┘
```

**Router Responsibilities:**
1. Validate incoming requests
2. Check if sync or async processing needed
3. Queue async requests in DynamoDB
4. Return immediate acknowledgment
5. Handle sync requests inline (< 5 seconds)

### 3. Async Processor (Lambda 2)

Long-running Lambda triggered by DynamoDB Streams when requests are queued.

**Processing Flow:**
```
DynamoDB Stream → Lambda → Get Handler → Process → Update Status → Notify Client
                    │                        │
                    ▼                        ▼
                Retry Queue            Progress Updates
```

**Key Features:**
- Automatic retries with exponential backoff
- Progress reporting during processing
- Result storage in DynamoDB
- Client notification via WebSocket

### 4. Data Models

All models use DynamORM for type-safe DynamoDB operations:

```go
// Connection - WebSocket connection state
type Connection struct {
    ConnectionID string            `dynamorm:"pk"`
    UserID       string            `dynamorm:"index:user-connections,pk"`
    TenantID     string            `dynamorm:"index:tenant-connections,pk"`
    Endpoint     string            // API Gateway callback URL
    ConnectedAt  time.Time         
    LastPing     time.Time         
    Metadata     map[string]string `dynamorm:"json"`
    TTL          int64             `dynamorm:"ttl"`
}

// AsyncRequest - Queued request for processing
type AsyncRequest struct {
    RequestID     string                 `dynamorm:"pk"`
    ConnectionID  string                 `dynamorm:"index:connection-requests,pk"`
    TenantID      string                 `dynamorm:"index:tenant-requests,pk"`
    Status        string                 `dynamorm:"index:status-time,pk"`
    CreatedAt     time.Time              `dynamorm:"index:status-time,sk"`
    Action        string                 
    Payload       map[string]interface{} `dynamorm:"json"`
    Result        map[string]interface{} `dynamorm:"json"`
    Error         string                 
    ProcessedAt   time.Time              
    TTL           int64                  `dynamorm:"ttl"`
    RetryCount    int                    
    Priority      int                    `dynamorm:"index:priority-queue,pk"`
    ProcessAfter  time.Time              `dynamorm:"index:priority-queue,sk"`
}

// Subscription - Client subscriptions to request updates
type Subscription struct {
    SubscriptionID string   `dynamorm:"pk"`
    ConnectionID   string   `dynamorm:"index:connection-subs,pk"`
    RequestID      string   `dynamorm:"index:request-subs,pk"`
    EventTypes     []string `dynamorm:"set"`
    CreatedAt      time.Time
    TTL            int64    `dynamorm:"ttl"`
}
```

### 5. Progress Reporting

Progress updates flow from processor to client in real-time:

```
Processor → Progress Update → DynamoDB → Change Stream → Notifier → WebSocket → Client
                    │
                    ▼
              Update Request
               Progress %
```

## Scalability Patterns

### 1. Connection Scaling
- Connections distributed across multiple tables by hash
- Read/write capacity auto-scales with demand
- Connection pruning prevents unbounded growth

### 2. Request Processing Scaling
- DynamoDB Streams shards scale automatically
- Lambda concurrent executions scale to 1000s
- Priority queues for request ordering
- Batch processing for efficiency

### 3. Multi-Tenant Isolation
- Tenant-specific tables or prefixes
- IAM roles for cross-account access
- Request routing by tenant
- Isolated failure domains

## Reliability Patterns

### 1. Message Durability
- All messages persisted to DynamoDB before ACK
- DynamoDB Streams guarantee at-least-once delivery
- Idempotency keys prevent duplicate processing

### 2. Failure Handling
- Automatic retries with exponential backoff
- Dead letter queue for failed requests
- Circuit breakers for downstream services
- Graceful degradation for non-critical features

### 3. Connection Recovery
- Clients auto-reconnect with exponential backoff
- Request state preserved across reconnections
- Progress resumes from last checkpoint

## Security Considerations

### 1. Authentication & Authorization
- JWT validation on connect
- Per-request authorization checks
- Tenant isolation enforcement
- API key rotation support

### 2. Data Protection
- Encryption at rest (DynamoDB)
- Encryption in transit (TLS)
- PII handling compliance
- Audit logging

### 3. Rate Limiting
- Per-connection message limits
- Tenant-level quotas
- Backpressure mechanisms
- DDoS protection

## Performance Optimizations

### 1. Lambda Cold Starts
- Connection pooling
- Lazy initialization
- Pre-warmed containers
- Minimal dependencies

### 2. DynamoDB Optimization
- Batch operations
- Projection expressions
- Sparse indexes
- Caching strategies

### 3. WebSocket Efficiency
- Message batching
- Binary protocol support
- Compression
- Keep-alive tuning

## Monitoring & Observability

### 1. Metrics
- Connection count by tenant
- Request processing time
- Queue depth
- Error rates
- Progress checkpoints

### 2. Logging
- Structured JSON logs
- Request tracing
- Error aggregation
- Performance profiling

### 3. Alerting
- Queue backup alerts
- Error rate thresholds
- Connection limit warnings
- SLA monitoring

## Cost Optimization

### 1. DynamoDB Costs
- On-demand pricing for variable load
- TTL for automatic data cleanup
- Efficient index design
- Batch operations

### 2. Lambda Costs
- Right-sized memory allocation
- Minimal cold starts
- Efficient processing
- ARM Graviton2

### 3. Data Transfer
- Regional deployments
- VPC endpoints
- Compression
- Caching

## Future Considerations

### 1. Alternative Backends
- Redis for hot connection data
- SQS for simple queuing
- EventBridge for event routing
- Kinesis for high-volume streams

### 2. Advanced Features
- Request workflows (Step Functions)
- Scheduled requests
- Request templates
- Batch operations

### 3. Client Libraries
- Official SDKs (JS, Python, Go)
- React hooks
- Mobile SDKs
- CLI tools 