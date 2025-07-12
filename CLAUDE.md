# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Streamer is a production-ready Go library for async request processing in AWS Lambda with WebSocket support. It solves the API Gateway 29-second timeout limitation by implementing an async request/response pattern with real-time progress updates. The system is built for Pay Theory's serverless infrastructure using DynamORM and Lift frameworks.

## Common Development Commands

### Building and Testing
```bash
# Run all tests
make test

# Run unit tests only
make test-short

# Run integration tests
make test-integration

# Run tests with coverage
make test-coverage

# Build the project
make build

# Build Lambda deployment packages
make build-lambdas
```

### Code Quality
```bash
# Format code
make fmt

# Run linters (requires golangci-lint)
make lint

# Security scan (requires gosec)
make security
```

### Development Environment
```bash
# Start local DynamoDB
make docker-dynamo

# Create DynamoDB tables locally
make create-tables

# Stop local DynamoDB
make stop-dynamo
```

### Specific Testing
```bash
# Test storage layer only
make test-store

# Test with storage coverage
make test-store-coverage
```

## Architecture Overview

The system implements a distributed async processing pattern with four core components:

1. **Connection Manager** (`pkg/connection/`) - Manages WebSocket connections with DynamoDB persistence
2. **Request Router** (`lambda/router/`) - Fast Lambda that acknowledges requests and queues async work
3. **Async Processor** (`lambda/processor/`) - Long-running Lambda triggered by DynamoDB Streams
4. **Progress Reporting** (`pkg/progress/`) - Real-time progress updates via WebSocket

### Key Interfaces for Development

**Handler Interface** - Main interface for implementing custom request handlers:
```go
type Handler interface {
    Validate(request *Request) error
    EstimatedDuration() time.Duration
    Process(ctx context.Context, request *Request) (*Result, error)
}
```

**HandlerWithProgress** - For handlers needing progress reporting:
```go
type HandlerWithProgress interface {
    Handler
    ProcessWithProgress(ctx context.Context, request *Request, reporter ProgressReporter) (*Result, error)
}
```

### Data Flow
- Sync requests (<5s): Router → Handler → Response
- Async requests (>5s): Router → DynamoDB Queue → Processor → Progress Updates → Client

## Key Technologies

- **Go 1.23.10** with AWS Lambda runtime
- **DynamORM** - Type-safe DynamoDB ORM (Pay Theory internal)
- **Lift** - Pay Theory's serverless optimization framework
- **AWS Services**: Lambda, API Gateway WebSockets, DynamoDB, CloudWatch, X-Ray
- **Pulumi** for Infrastructure as Code

## Project Structure

```
pkg/                    # Public packages
├── streamer/          # Core router and handler interfaces  
├── connection/        # WebSocket connection management
├── progress/          # Progress reporting system
├── subscription/      # Real-time update subscriptions
└── types/            # Shared message types

internal/              # Private packages
└── store/            # DynamoDB storage layer with interfaces

lambda/                # Lambda function handlers
├── connect/          # WebSocket $connect handler
├── disconnect/       # WebSocket $disconnect handler  
├── router/           # Request router and dispatcher
├── processor/        # Async request processor
└── shared/           # Common Lambda utilities

deployment/           # Pulumi Infrastructure as Code
tests/                # Integration and performance tests
```

## Development Notes

- Target 80% unit test coverage
- Use DynamORM for all DynamoDB operations
- Leverage Lift optimizations for Lambda builds (see build tags)
- Follow Pay Theory's modular, security-first design principles
- Single table design pattern for DynamoDB efficiency
- JWT authentication for WebSocket connections
- Progress updates use WebSocket message batching for efficiency

## Build Tags

The project uses build tags for optimization:
- `lift` - Enable Lift framework optimizations
- `optimized` - Additional performance optimizations  
- `lambda.norpc` - Disable RPC for Lambda environment

## Authentication

WebSocket connections require JWT tokens with proper validation. Connection metadata includes TenantID for multi-tenant isolation.

## Monitoring

The system includes comprehensive observability:
- CloudWatch metrics for connection counts, processing times, queue depth
- X-Ray tracing for request flows
- Structured JSON logging throughout
- TTL-based automatic cleanup for DynamoDB records

## DynamORM Single-Model Architecture

Streamer now follows proper DynamORM conventions with single models serving both business logic and database operations.

### 1. Creating DynamORM Components

Use the DynamORM factory pattern to get the database instance:

```go
import (
    "github.com/pay-theory/dynamorm/pkg/session"
    "github.com/pay-theory/streamer/internal/store/dynamorm"
    "github.com/pay-theory/streamer/pkg/models"
)

// Create DynamORM configuration
dynamormConfig := session.Config{
    Region: "us-east-1", // your AWS region
}

// Create the store factory
factory, err := dynamorm.NewStoreFactory(dynamormConfig)
if err != nil {
    log.Fatalf("Failed to create DynamORM factory: %v", err)
}

// Get the DynamORM database instance
db := factory.DB()
```

### 2. Setting Up AsyncExecutor with DynamORM

The AsyncExecutor now works directly with DynamORM models:

```go
import (
    "github.com/pay-theory/streamer/lambda/processor/executor"
    "github.com/pay-theory/streamer/pkg/connection"
    "github.com/pay-theory/streamer/pkg/streamer"
)

// Create connection manager using public API
apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, ...)
apiGatewayAdapter := connection.NewAWSAPIGatewayAdapter(apiGatewayClient)
connManager := connection.NewManager(connectionStore, apiGatewayAdapter, endpoint)

// Create AsyncExecutor with DynamORM database instance
exec := executor.New(connManager, db, logger)

// Register handlers
exec.RegisterHandler("action_name", yourHandler)

// Process requests (typically called from DynamoDB stream handler)
err := exec.ProcessWithRetry(ctx, asyncRequest)
```

### 3. DynamoDB Stream Event Handling

**Proper DynamORM approach** - Use DynamORM's SafeMarshaler:

```go
import (
    "github.com/aws/aws-lambda-go/events"
    "github.com/pay-theory/dynamorm/pkg/marshal"
    "github.com/pay-theory/streamer/pkg/models"
)

func parseAsyncRequest(record events.DynamoDBEventRecord) (*models.AsyncRequest, error) {
    image := record.Change.NewImage
    if image == nil {
        return nil, nil
    }

    // Use DynamORM's SafeMarshaler for proper conversion
    marshaler := marshal.NewSafeMarshaler()
    
    var asyncReq models.AsyncRequest
    if err := marshaler.UnmarshalItem(image, &asyncReq); err != nil {
        return nil, fmt.Errorf("failed to unmarshal AsyncRequest using DynamORM marshaler: %w", err)
    }

    return &asyncReq, nil
}

// In your Lambda handler
func handler(ctx context.Context, event events.DynamoDBEvent) error {
    for _, record := range event.Records {
        if record.EventName != "INSERT" && record.EventName != "MODIFY" {
            continue
        }

        asyncReq, err := parseAsyncRequest(record)
        if err != nil || asyncReq == nil {
            continue
        }

        if asyncReq.Status != models.StatusPending {
            continue
        }

        // Process with AsyncExecutor
        err = exec.ProcessWithRetry(ctx, asyncReq)
        // Handle errors...
    }
    return nil
}
```

### 4. Handler Implementation Examples

**Basic Handler:**
```go
type CustomHandler struct{}

func (h *CustomHandler) EstimatedDuration() time.Duration {
    return 30 * time.Second // >5s = async processing
}

func (h *CustomHandler) Validate(req *streamer.Request) error {
    // Validation logic
    return nil
}

func (h *CustomHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    // Processing logic
    return &streamer.Result{Success: true, Data: result}, nil
}
```

**Handler with Progress:**
```go
func (h *CustomHandler) ProcessWithProgress(
    ctx context.Context, 
    req *streamer.Request, 
    reporter streamer.ProgressReporter,
) (*streamer.Result, error) {
    reporter.Report(0, "Starting...")
    // ... work ...
    reporter.Report(50, "Half done...")
    // ... more work ...
    reporter.Report(100, "Complete!")
    return &streamer.Result{Success: true, Data: result}, nil
}
```

## Key Design Principles

- **Single-model architecture**: DynamORM models serve both business logic and database operations
- **Use DynamORM's SafeMarshaler** for stream parsing instead of raw AWS SDK calls
- **AsyncExecutor works directly with DynamORM models** for database operations
- **Progress reporting is batched** for efficiency (200ms intervals)
- **Proper struct tags**: Use `dynamorm:"attr:field_name"` for business fields, `dynamorm:"pk"/"sk"` for keys
- **Handler registration** happens in AsyncExecutor, not Router (Router is for sync requests)