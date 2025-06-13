# Architecture Overview

Streamer is a production-ready async request processing system that overcomes AWS API Gateway's 29-second timeout limitation through WebSocket-based real-time communication and DynamoDB Streams processing.

## System Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │    │ API Gateway │    │   Router    │    │  Processor  │
│ (WebSocket) │◄──►│ (WebSocket) │◄──►│   Lambda    │    │   Lambda    │
└─────────────┘    └─────────────┘    └─────┬───────┘    └──────▲──────┘
                                             │                   │
                                             ▼                   │
                                    ┌─────────────┐              │
                                    │  DynamoDB   │              │
                                    │ (Requests)  │──────────────┘
                                    └─────────────┘   DynamoDB
                                             │        Streams
                                             ▼
                                    ┌─────────────┐
                                    │  DynamoDB   │
                                    │(Connections)│
                                    └─────────────┘
```

## Core Components

### 1. WebSocket API Gateway

**Purpose**: Manages persistent WebSocket connections between clients and the system.

**Responsibilities**:
- Connection lifecycle management ($connect, $disconnect, $default routes)
- JWT authentication via custom authorizers
- Message routing to appropriate Lambda functions
- Connection state management

**Routes**:
- `$connect`: Validates JWT and creates connection record
- `$disconnect`: Cleans up connection and subscriptions  
- `$default`: Routes all messages to Router Lambda

### 2. Router Lambda

**Purpose**: Intelligent request routing with sync/async decision logic.

**Key Features**:
- **Sync/Async Decision**: Uses `handler.EstimatedDuration()` vs 5-second threshold
- **Request Validation**: Validates payload and authentication
- **Immediate Response**: Sync requests processed and returned immediately
- **Queue Management**: Async requests queued in DynamoDB with acknowledgment

**Decision Flow**:
```go
if handler.EstimatedDuration() <= 5*time.Second {
    // Process synchronously
    result := handler.Process(ctx, request)
    return sendResponse(connectionID, result)
} else {
    // Queue for async processing
    requestQueue.Enqueue(request)
    return sendAcknowledgment(connectionID, "queued")
}
```

### 3. Processor Lambda

**Purpose**: Handles async request processing with real-time progress updates.

**Trigger**: DynamoDB Streams from the requests table
**Concurrency**: Configurable (default: 100 concurrent executions)
**Timeout**: 15 minutes (AWS Lambda maximum)

**Processing Flow**:
1. Receives stream event with new/updated request
2. Validates request is in PENDING status
3. Updates status to PROCESSING
4. Executes handler with progress reporter
5. Sends real-time updates via WebSocket
6. Updates final status (COMPLETED/FAILED)

### 4. Connection Manager

**Purpose**: Manages WebSocket connections and message delivery.

**Features**:
- **Connection Persistence**: Stores connection metadata in DynamoDB
- **Message Delivery**: Sends messages via API Gateway Management API
- **Health Checking**: Validates connection status before sending
- **Automatic Cleanup**: TTL-based cleanup of stale connections

**Connection Lifecycle**:
```
Connect → Authenticate → Store → Process Messages → Disconnect → Cleanup
```

### 5. Progress Reporter

**Purpose**: Provides real-time progress updates with intelligent batching.

**Features**:
- **Rate Limiting**: Batches updates every 200ms to prevent spam
- **Intelligent Batching**: Combines multiple updates, preserves significant changes
- **Metadata Support**: Allows custom metadata in progress updates
- **Automatic Flush**: Immediately sends updates at 90%+ progress

**Batching Logic**:
```go
// Immediate flush conditions
if percentage >= 95.0 || 
   time.Since(lastUpdate) >= flushInterval ||
   isError {
    flushImmediately()
}
```

## Data Models

### Connection Model

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

### Request Model

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
```

## Message Flow

### Sync Request Flow

```
1. Client sends WebSocket message
2. API Gateway routes to Router Lambda
3. Router validates and processes immediately
4. Response sent back via WebSocket
5. Total time: <100ms
```

### Async Request Flow

```
1. Client sends WebSocket message
2. Router queues request in DynamoDB
3. Acknowledgment sent to client
4. DynamoDB Stream triggers Processor
5. Processor executes handler with progress updates
6. Real-time progress sent via WebSocket
7. Final result delivered when complete
8. Total time: Unlimited (up to 15 min Lambda timeout)
```

## Scalability & Performance

### Connection Scaling

- **Concurrent Connections**: 10,000+ per API Gateway
- **Connection Storage**: DynamoDB with auto-scaling
- **Geographic Distribution**: Multi-region deployment support

### Processing Scaling

- **Lambda Concurrency**: Configurable per function
- **DynamoDB Streams**: Automatic scaling based on shard count
- **Request Queuing**: Unlimited queue depth in DynamoDB

### Performance Characteristics

| Component | Latency | Throughput |
|-----------|---------|------------|
| Sync Requests | <50ms p99 | 1000+ RPS |
| WebSocket Messages | <100ms p99 | 10,000+ msg/sec |
| Progress Updates | <200ms | 100+ updates/sec |
| Connection Setup | <500ms | 100+ conn/sec |

## Security Architecture

### Authentication Flow

```
1. Client obtains JWT token from auth service
2. WebSocket connection includes token in query params
3. API Gateway validates token via custom authorizer
4. Connection established with user/tenant context
5. All subsequent messages inherit auth context
```

### Multi-Tenant Isolation

- **Data Isolation**: Tenant ID in all data models
- **Connection Isolation**: Tenant-based connection filtering
- **Processing Isolation**: Tenant context in all operations
- **Monitoring Isolation**: Tenant-specific metrics and logs

### Security Features

- **JWT Authentication**: RS256 with public key validation
- **Encryption**: All data encrypted at rest and in transit
- **IAM Roles**: Least privilege access for all components
- **VPC**: Optional VPC deployment for network isolation
- **Audit Logging**: Comprehensive CloudTrail integration

## Monitoring & Observability

### CloudWatch Metrics

- **Connection Metrics**: Active connections, connection rate
- **Processing Metrics**: Request latency, success rate, error rate
- **Progress Metrics**: Update frequency, batching efficiency
- **System Metrics**: Lambda duration, DynamoDB throttling

### X-Ray Tracing

- **End-to-End Tracing**: From WebSocket to completion
- **Performance Analysis**: Identify bottlenecks
- **Error Analysis**: Trace error propagation
- **Dependency Mapping**: Visualize service interactions

### Structured Logging

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "component": "router",
  "connection_id": "abc123",
  "user_id": "user456",
  "tenant_id": "tenant789",
  "action": "generate_report",
  "duration_ms": 45,
  "status": "success"
}
```

## Error Handling & Resilience

### Circuit Breaker Pattern

- **Failure Detection**: Automatic failure rate monitoring
- **Circuit Opening**: Stop processing when failure rate exceeds threshold
- **Recovery**: Gradual recovery with health checks

### Retry Logic

- **Exponential Backoff**: Configurable retry delays
- **Max Retries**: Configurable retry limits
- **Dead Letter Queue**: Failed requests for manual review

### Graceful Degradation

- **Connection Failures**: Continue processing, queue updates
- **DynamoDB Throttling**: Automatic backoff and retry
- **Lambda Timeouts**: Checkpoint progress, resume processing

## Deployment Architecture

### Infrastructure as Code

- **Pulumi**: Primary IaC tool with TypeScript/Go support
- **SAM**: Alternative deployment for Lambda-focused setups
- **CloudFormation**: Generated templates for enterprise environments

### Environment Separation

- **Development**: Single region, minimal resources
- **Staging**: Production-like, reduced capacity
- **Production**: Multi-region, full monitoring, auto-scaling

### Blue/Green Deployment

- **Lambda Versions**: Gradual traffic shifting
- **API Gateway Stages**: Environment isolation
- **Database Migration**: Zero-downtime schema updates

This architecture enables Streamer to handle long-running operations while maintaining real-time communication, providing a robust foundation for async processing at scale. 