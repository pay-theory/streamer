# External Processor Example for Penny Team

Now you can build processors in your own module using Streamer's public models:

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/dynamorm"
    "github.com/pay-theory/streamer/pkg/models"  // PUBLIC MODELS!
)

func handler(ctx context.Context, event events.DynamoDBEvent) error {
    for _, record := range event.Records {
        if record.EventName != "INSERT" && record.EventName != "MODIFY" {
            continue
        }

        // Parse using public models
        asyncReq, err := parseAsyncRequest(record)
        if err != nil {
            log.Printf("Failed to parse: %v", err)
            continue
        }

        if asyncReq.Status != models.StatusPending {
            continue
        }

        // Process your async request here
        processYourRequest(ctx, asyncReq)
    }
    return nil
}

func parseAsyncRequest(record events.DynamoDBEventRecord) (*models.AsyncRequest, error) {
    image := record.Change.NewImage
    if image == nil {
        return nil, nil
    }

    // Use DynamORM's stream support with public models
    var asyncReq models.AsyncRequest
    if err := dynamorm.UnmarshalStreamImage(image, &asyncReq); err != nil {
        return nil, fmt.Errorf("failed to unmarshal: %w", err)
    }

    return &asyncReq, nil
}

func processYourRequest(ctx context.Context, req *models.AsyncRequest) {
    // Your processing logic here
    log.Printf("Processing request %s with action %s", req.RequestID, req.Action)
}

func main() {
    lambda.Start(handler)
}
```

## In your go.mod:

```
module your-penny-processor

go 1.23

require (
    github.com/aws/aws-lambda-go v1.49.0
    github.com/pay-theory/dynamorm v1.0.26
    github.com/pay-theory/streamer v1.0.10  // Use the version with public models
)
```

## Key Changes:

1. **Public Models**: `github.com/pay-theory/streamer/pkg/models` is now public
2. **No Internal Imports**: You don't need `internal/store/dynamorm` anymore
3. **Same Functionality**: All the same model types and methods are available

The models are now **publicly importable** from any external module!