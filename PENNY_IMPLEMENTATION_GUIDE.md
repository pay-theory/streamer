# Streamer Implementation Guide for Penny Team

## Overview

Streamer provides WebSocket-based async request processing with real-time progress updates. It's built on DynamORM v1.0.24+ and follows single-model architecture principles.

## Key Architecture Points

### 1. DynamORM Integration
- **Single Model Architecture**: Models in `pkg/models` serve both business logic and database operations
- **No AWS SDK**: Never use AWS SDK's `attributevalue` package - DynamORM provides all marshaling
- **Stream Processing**: Use `dynamorm.UnmarshalStreamImage()` for DynamoDB streams

### 2. Core Components

```
streamer/
├── lambda/
│   ├── router/          # WebSocket message router
│   ├── processor/       # Async request processor
│   ├── connect/         # WebSocket connection handler
│   └── disconnect/      # WebSocket disconnection handler
├── internal/
│   └── store/
│       └── dynamorm/    # Internal DynamORM implementation
└── pkg/
    └── models/         # Public models (Connection, AsyncRequest, Subscription)
└── pkg/
    ├── streamer/        # Core interfaces and router
    └── connection/      # WebSocket connection management
```

## Implementation Steps

### Step 1: Set Up DynamoDB Tables

Create three tables following DynamORM patterns:

```go
// streamer_connections - WebSocket connections
type Connection struct {
    PK           string `dynamorm:"pk"`           // CONN#<ConnectionID>
    SK           string `dynamorm:"sk"`           // METADATA
    ConnectionID string `dynamorm:"connection_id"`
    UserID       string `dynamorm:"user_id"`
    TenantID     string `dynamorm:"tenant_id"`
    // ... other fields
}

// streamer_requests - Async requests
type AsyncRequest struct {
    PK           string `dynamorm:"pk"`           // REQ#<RequestID>
    SK           string `dynamorm:"sk"`           // STATUS#<Status>
    RequestID    string `dynamorm:"request_id"`
    ConnectionID string `dynamorm:"connection_id"`
    Status       string `dynamorm:"status"`
    // ... other fields
}

// streamer_subscriptions - Real-time subscriptions
type Subscription struct {
    PK             string `dynamorm:"pk"`           // CONN#<ConnectionID>
    SK             string `dynamorm:"sk"`           // SUB#<RequestID>
    SubscriptionID string `dynamorm:"subscription_id"`
    // ... other fields
}
```

### Step 2: Deploy Lambda Functions

1. **Router Lambda** - Handles incoming WebSocket messages
   ```bash
   cd lambda/router
   GOOS=linux go build -o bootstrap
   zip router.zip bootstrap
   ```

2. **Processor Lambda** - Processes async requests from DynamoDB Streams
   ```bash
   cd lambda/processor
   GOOS=linux go build -o bootstrap
   zip processor.zip bootstrap
   ```

3. **Connect/Disconnect Lambdas** - Manage WebSocket lifecycle
   ```bash
   cd lambda/connect
   GOOS=linux go build -o bootstrap
   zip connect.zip bootstrap
   ```

### Step 3: Configure API Gateway WebSocket

1. Create WebSocket API
2. Set routes:
   - `$connect` → Connect Lambda
   - `$disconnect` → Disconnect Lambda
   - `$default` → Router Lambda
3. Deploy to stage and note the WebSocket endpoint URL

### Step 4: Set Up DynamoDB Streams

Enable streams on `streamer_requests` table:
- Stream view type: NEW_AND_OLD_IMAGES
- Point to Processor Lambda as trigger

### Step 5: Implement Handlers

Create handlers implementing the `streamer.Handler` interface:

```go
type MyHandler struct{}

func (h *MyHandler) Validate(ctx context.Context, payload map[string]interface{}) error {
    // Validate request payload
    return nil
}

func (h *MyHandler) EstimatedDuration() time.Duration {
    // Return expected processing time
    // < 5s = sync, >= 5s = async
    return 10 * time.Second
}

func (h *MyHandler) Process(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
    // Process the request
    return map[string]interface{}{
        "result": "success",
        "data": processedData,
    }, nil
}
```

For progress updates, implement `streamer.HandlerWithProgress`:

```go
func (h *MyHandler) ProcessWithProgress(ctx context.Context, payload map[string]interface{}, reporter progress.Reporter) (map[string]interface{}, error) {
    // Report progress during processing
    reporter.Update(0.25, "Processing phase 1", nil)
    // ... do work ...
    reporter.Update(0.50, "Processing phase 2", nil)
    // ... do work ...
    return result, nil
}
```

### Step 6: Parse DynamoDB Streams

In your processor Lambda, use DynamORM's native stream support:

```go
func parseAsyncRequest(record events.DynamoDBEventRecord) (*models.AsyncRequest, error) {
    image := record.Change.NewImage
    if image == nil {
        return nil, nil
    }

    // Use DynamORM's native stream support (v1.0.24+)
    var asyncReq models.AsyncRequest
    if err := dynamorm.UnmarshalStreamImage(image, &asyncReq); err != nil {
        return nil, fmt.Errorf("failed to unmarshal AsyncRequest: %w", err)
    }

    return &asyncReq, nil
}
```

## Testing

### Local Testing

1. Use the demo client:
   ```bash
   cd demo
   npm install
   export JWT_SECRET="your-secret"
   export USER_ID="test-user"
   export TENANT_ID="test-tenant"
   npm run demo
   ```

2. Test WebSocket messages:
   ```json
   {
     "action": "your_handler_name",
     "payload": {
       "data": "test"
     }
   }
   ```

### Integration Testing

See `tests/integration/` for examples of testing with real DynamoDB tables.

## Common Pitfalls to Avoid

1. **Never use AWS SDK attributevalue**
   ```go
   // WRONG
   attributevalue.UnmarshalMap(image, &model)
   
   // CORRECT
   dynamorm.UnmarshalStreamImage(image, &model)
   ```

2. **Don't use Save() - it doesn't exist in DynamORM**
   ```go
   // WRONG
   db.Save(ctx, model)
   
   // CORRECT
   db.Model(model).Create()
   db.Model(model).Update("field1", "field2")
   ```

3. **Always set composite keys before operations**
   ```go
   asyncReq.SetKeys() // Sets PK and SK based on model state
   db.Model(asyncReq).Update("status", "progress")
   ```

4. **Use proper imports for model packages**
   ```go
   import (
       "github.com/pay-theory/dynamorm"
       "github.com/pay-theory/streamer/pkg/models"
   )
   ```

## Environment Variables

Required for all Lambdas:
- `AWS_REGION` - AWS region
- `DYNAMODB_ENDPOINT` - (optional) for local testing

Required for Processor:
- `WEBSOCKET_ENDPOINT` - API Gateway WebSocket endpoint URL

## Monitoring

1. CloudWatch Logs - All lambdas log to CloudWatch
2. X-Ray - Tracing is enabled by default
3. DynamoDB Streams metrics - Monitor for processing delays

## Support

For issues or questions:
1. Check the examples in `examples/processor/` and `examples/router/`
2. Review integration tests in `tests/integration/`
3. Ensure you're using DynamORM v1.0.24+ and Lift v1.0.54+

## Quick Start Checklist

- [ ] DynamoDB tables created with proper indexes
- [ ] Lambda functions deployed
- [ ] API Gateway WebSocket configured
- [ ] DynamoDB Streams enabled on requests table
- [ ] Environment variables set
- [ ] Handlers implemented and registered
- [ ] JWT authentication configured (if needed)
- [ ] Demo client tested successfully