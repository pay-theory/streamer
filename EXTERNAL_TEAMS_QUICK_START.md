# Quick Start Guide for External Teams

## ✅ FIXED: You Can Now Build External Processors!

The fundamental issue has been resolved. External teams can now build processors using Streamer's **public models**.

## What Changed

### Before (Broken)
```go
// ❌ This was broken - can't import internal packages
import "github.com/pay-theory/streamer/internal/store/dynamorm" 
```

### After (Fixed)
```go
// ✅ This works - public models package
import "github.com/pay-theory/streamer/pkg/models"
```

## Quick Example

Create your own processor in your own module:

```go
// your-processor/main.go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/streamer/pkg/models"  // PUBLIC!
)

func handler(ctx context.Context, event events.DynamoDBEvent) error {
    for _, record := range event.Records {
        if record.EventName != "INSERT" && record.EventName != "MODIFY" {
            continue
        }

        // Parse using public models
        asyncReq, err := parseAsyncRequest(record)
        if err != nil {
            log.Printf("Parse error: %v", err)
            continue
        }

        if asyncReq.Status != models.StatusPending {
            continue
        }

        // Process your request
        log.Printf("Processing %s action %s", asyncReq.RequestID, asyncReq.Action)
    }
    return nil
}

func parseAsyncRequest(record events.DynamoDBEventRecord) (*models.AsyncRequest, error) {
    image := record.Change.NewImage
    if image == nil {
        return nil, nil
    }

    var asyncReq models.AsyncRequest
    if err := dynamorm.UnmarshalStreamImage(image, &asyncReq); err != nil {
        return nil, fmt.Errorf("unmarshal failed: %w", err)
    }

    return &asyncReq, nil
}

func main() {
    lambda.Start(handler)
}
```

## Your go.mod

```go
module github.com/your-team/your-processor

go 1.23

require (
    github.com/aws/aws-lambda-go v1.49.0
    github.com/pay-theory/dynamorm v1.0.26
    github.com/pay-theory/streamer v1.0.10  // Latest with public models
)
```

## Available Public Types

From `github.com/pay-theory/streamer/pkg/models`:

- `models.AsyncRequest` - DynamoDB async request model
- `models.Connection` - WebSocket connection model  
- `models.Subscription` - Real-time subscription model
- `models.RequestStatus` - Status type
- Status constants: `models.StatusPending`, `models.StatusProcessing`, etc.

## Key Points

1. **No internal imports needed** - everything is public
2. **Same DynamORM tags** - field mappings work correctly  
3. **Stream parsing works** - `dynamorm.UnmarshalStreamImage()` works with public models
4. **All documentation updated** - guides now show correct imports

## Deployment

1. Build your Lambda: `GOOS=linux go build -o bootstrap main.go`
2. Zip it: `zip processor.zip bootstrap`
3. Deploy to AWS Lambda
4. Set DynamoDB stream trigger pointing to your Lambda
5. Set environment variable: `WEBSOCKET_ENDPOINT=your-api-gateway-endpoint`

## Status

- ✅ **Compiles**: Everything builds successfully
- ✅ **Tests Pass**: All tests updated and passing  
- ✅ **Documentation**: All guides updated to use public models
- ✅ **Stream Parsing**: DynamORM integration works correctly
- ✅ **No Internal Deps**: External teams can build processors independently

**You're good to go!** 🚀