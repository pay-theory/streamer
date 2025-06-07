# Streamer

Async request processing and WebSocket management for AWS Lambda, designed to work seamlessly with DynamORM.

## Overview

Streamer solves the critical challenge of handling long-running operations in serverless architectures where API Gateway enforces a 29-second timeout. By implementing an async request/response pattern with real-time updates via WebSocket, Streamer enables Lambda functions to process operations that take minutes or hours while keeping clients informed of progress.

## Core Architecture

### Request Flow

```
Client → WebSocket → Router Lambda (fast) → DynamoDB Queue → Processor Lambda (async)
                           ↓                                          ↓
                     Immediate ACK                             Progress Updates
                           ↓                                          ↓
                        Client ←─────────── WebSocket ←──────────────┘
```

### Key Components

1. **Connection Manager** - Manages WebSocket connections with DynamoDB persistence
2. **Request Router** - Handles incoming messages and queues long-running requests  
3. **Async Processor** - Processes queued requests via DynamoDB Streams
4. **Subscription System** - Delivers real-time updates to connected clients
5. **Progress Reporter** - Provides progress updates during long operations

## Integration with DynamORM

Streamer leverages DynamORM for all data persistence:

```go
// Connection model using DynamORM
type Connection struct {
    ConnectionID string            `dynamorm:"pk"`
    UserID       string            `dynamorm:"index:user-connections,pk"`
    TenantID     string            `dynamorm:"index:tenant-connections,pk"`
    Endpoint     string            `dynamorm:"endpoint"`
    ConnectedAt  time.Time         `dynamorm:"connected_at"`
    LastPing     time.Time         `dynamorm:"last_ping"`
    Metadata     map[string]string `dynamorm:"metadata,json"`
    TTL          int64             `dynamorm:"ttl"` // Auto-cleanup
}

// Async request with DynamoDB Streams
type AsyncRequest struct {
    RequestID    string                 `dynamorm:"pk"`
    ConnectionID string                 `dynamorm:"index:connection-requests,pk"`
    Status       string                 `dynamorm:"index:status-time,pk"`
    CreatedAt    time.Time              `dynamorm:"index:status-time,sk"`
    Action       string                 `dynamorm:"action"`
    Payload      map[string]interface{} `dynamorm:"payload,json"`
    Result       map[string]interface{} `dynamorm:"result,json"`
    TTL          int64                  `dynamorm:"ttl"`
}
```

## Features

### For Developers
- **Simple API** - Send async requests like sync ones
- **Type-safe handlers** - Define strongly-typed request/response schemas
- **Progress tracking** - Built-in progress reporting
- **Error handling** - Automatic retries with exponential backoff
- **Testing support** - Comprehensive mocks and helpers

### For Production
- **Multi-tenant** - Built-in tenant isolation
- **Scalable** - Handles millions of concurrent connections
- **Resilient** - No message loss with DynamoDB persistence  
- **Cost-effective** - Pay only for active processing
- **Observable** - CloudWatch metrics and X-Ray tracing

## Quick Start

### 1. Install Streamer

```bash
go get github.com/pay-theory/streamer
```

### 2. Define Your Handler

```go
// handlers/report.go
type ReportHandler struct {
    s3Client *s3.Client
}

func (h *ReportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    // Extract typed payload
    var params ReportParams
    if err := req.UnmarshalPayload(&params); err != nil {
        return nil, err
    }
    
    // Report progress
    reporter := streamer.GetReporter(ctx)
    
    // Step 1: Query data
    reporter.Progress(0.2, "Querying data...")
    data, err := h.queryData(ctx, params)
    if err != nil {
        return nil, err
    }
    
    // Step 2: Generate report  
    reporter.Progress(0.6, "Generating report...")
    report, err := h.generateReport(ctx, data)
    if err != nil {
        return nil, err
    }
    
    // Step 3: Upload
    reporter.Progress(0.9, "Uploading...")
    url, err := h.uploadToS3(ctx, report)
    if err != nil {
        return nil, err
    }
    
    reporter.Progress(1.0, "Complete!")
    
    return &streamer.Result{
        "url": url,
        "size": report.Size,
    }, nil
}
```

### 3. Set Up Lambda Functions

```go
// lambda/router/main.go
package main

import (
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/streamer"
    "github.com/pay-theory/dynamorm"
)

var router *streamer.Router

func init() {
    // Initialize DynamORM
    db, _ := dynamorm.NewLambdaOptimized()
    
    // Create router
    router = streamer.NewRouter(
        streamer.WithDynamoDB(db),
        streamer.WithConnectionTable("connections"),
        streamer.WithRequestTable("requests"),
    )
    
    // Register handlers
    router.Handle("generate_report", &ReportHandler{})
    router.Handle("process_payment", &PaymentHandler{})
}

func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error {
    return router.Route(ctx, event)
}

func main() {
    lambda.Start(handler)
}
```

### 4. Client Usage

```typescript
// TypeScript client
import { StreamerClient } from '@pay-theory/streamer-client';

const client = new StreamerClient('wss://api.example.com/ws');

// Send async request
const result = await client.request('generate_report', {
    dateRange: { start: '2024-01-01', end: '2024-12-31' },
    format: 'pdf'
}, {
    onProgress: (progress, message) => {
        console.log(`${Math.round(progress * 100)}% - ${message}`);
    }
});

console.log('Report URL:', result.url);
```

## Use Cases

### 1. Report Generation
Generate large reports that take minutes to compile without timing out.

### 2. Batch Processing  
Process thousands of records with progress updates.

### 3. AI/ML Operations
Run long-running ML inference or training jobs.

### 4. Data Migrations
Migrate large datasets with progress tracking.

### 5. Video Processing
Transcode videos or generate thumbnails asynchronously.

## Architecture Decisions

### Why DynamoDB for Queue?
- **Durability** - No message loss
- **Streams** - Automatic processing triggers
- **TTL** - Automatic cleanup
- **Querying** - Rich query capabilities
- **Integration** - Seamless with DynamORM

### Why Separate Lambda Functions?
- **Timeout management** - Router stays under 30s
- **Cost optimization** - Different memory/CPU needs
- **Scaling** - Independent scaling policies
- **Isolation** - Failure isolation

### Why WebSockets?
- **Real-time** - Instant progress updates
- **Bidirectional** - Client can cancel operations
- **Efficient** - Single connection for multiple requests
- **Native** - API Gateway WebSocket support

## Roadmap

### Phase 1: Core (Q1 2024)
- [x] Connection management
- [x] Request routing
- [x] Async processing
- [x] Progress reporting
- [ ] Basic client SDKs

### Phase 2: Production (Q2 2024)
- [ ] Multi-tenant support
- [ ] Request prioritization
- [ ] Dead letter queues
- [ ] Monitoring dashboard
- [ ] Advanced retry strategies

### Phase 3: Advanced (Q3 2024)
- [ ] Request chaining/workflows
- [ ] Batch operations
- [ ] Request templates
- [ ] Cost optimization
- [ ] GraphQL subscriptions

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
