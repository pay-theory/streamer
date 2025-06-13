# Quick Start Guide

Get up and running with Streamer in 5 minutes! This guide will walk you through creating your first async handler and processing long-running requests via WebSocket.

## Prerequisites

- Go 1.21+
- AWS Account with CLI configured
- Basic understanding of Lambda and WebSockets

## 1. Installation

```bash
go mod init my-streamer-app
go get github.com/pay-theory/streamer
```

## 2. Create Your First Handler

Create `handlers/report.go`:

```go
package handlers

import (
    "context"
    "fmt"
    "time"
    
    "github.com/pay-theory/streamer/pkg/progress"
    "github.com/pay-theory/streamer/pkg/streamer"
)

type ReportHandler struct{}

func NewReportHandler() *ReportHandler {
    return &ReportHandler{}
}

// EstimatedDuration tells the router this will take longer than 5 seconds
func (h *ReportHandler) EstimatedDuration() time.Duration {
    return 2 * time.Minute // This will trigger async processing
}

// Validate checks the incoming request
func (h *ReportHandler) Validate(req *streamer.Request) error {
    if req.Payload == nil {
        return fmt.Errorf("payload is required")
    }
    return nil
}

// Process handles sync requests (won't be called for async)
func (h *ReportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    return nil, fmt.Errorf("use ProcessWithProgress for async handlers")
}

// ProcessWithProgress handles async requests with real-time updates
func (h *ReportHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    
    // Step 1: Initialize
    reporter.Report(10, "Starting report generation...")
    time.Sleep(2 * time.Second) // Simulate work
    
    // Step 2: Gather data
    reporter.Report(30, "Gathering data from database...")
    time.Sleep(3 * time.Second)
    
    // Step 3: Process data
    reporter.Report(60, "Processing and analyzing data...")
    time.Sleep(4 * time.Second)
    
    // Step 4: Generate report
    reporter.Report(90, "Generating final report...")
    time.Sleep(2 * time.Second)
    
    // Complete with results
    return reporter.Complete(map[string]interface{}{
        "report_url": "https://s3.amazonaws.com/reports/report-123.pdf",
        "size_bytes": 1024000,
        "pages": 15,
    })
}
```

## 3. Set Up Lambda Functions

Create `lambda/router/main.go`:

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/pay-theory/streamer/pkg/streamer"
    "my-streamer-app/handlers"
)

var router *streamer.DefaultRouter

func init() {
    // Initialize AWS and Streamer components
    cfg, err := config.LoadDefaultConfig(context.Background())
    if err != nil {
        log.Fatalf("Failed to load AWS config: %v", err)
    }

    // Create router with your stores and connection manager
    router = streamer.NewRouter(requestQueue, connectionManager)
    router.SetAsyncThreshold(5 * time.Second)

    // Register your handlers
    router.Handle("generate_report", handlers.NewReportHandler())
    
    log.Println("Router initialized successfully")
}

func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
    return router.Route(ctx, event)
}

func main() {
    lambda.Start(handler)
}
```

## 4. Client Usage

### JavaScript/TypeScript Client

```javascript
// Connect to WebSocket
const ws = new WebSocket('wss://your-api.execute-api.region.amazonaws.com/prod?Authorization=your-jwt-token');

// Send async request
const request = {
    action: 'generate_report',
    id: 'req-' + Date.now(),
    payload: {
        start_date: '2024-01-01',
        end_date: '2024-12-31',
        format: 'pdf'
    }
};

ws.send(JSON.stringify(request));

// Handle responses
ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    
    switch (message.type) {
        case 'acknowledgment':
            console.log('Request queued:', message.request_id);
            break;
            
        case 'progress':
            console.log(`Progress: ${message.percentage}% - ${message.message}`);
            break;
            
        case 'complete':
            console.log('Report ready:', message.result.report_url);
            break;
            
        case 'error':
            console.error('Error:', message.error);
            break;
    }
};
```

### Go Client

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/pay-theory/streamer/pkg/client"
)

func main() {
    // Create client
    client := client.New("wss://your-api.execute-api.region.amazonaws.com/prod", &client.Config{
        Token: "your-jwt-token",
    })

    // Connect
    if err := client.Connect(context.Background()); err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Send async request with progress tracking
    result, err := client.RequestWithProgress(context.Background(), "generate_report", map[string]interface{}{
        "start_date": "2024-01-01",
        "end_date":   "2024-12-31",
        "format":     "pdf",
    }, func(progress client.Progress) {
        fmt.Printf("Progress: %.0f%% - %s\n", progress.Percentage, progress.Message)
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Report URL: %s\n", result["report_url"])
}
```

## 5. Deploy to AWS

### Using Pulumi (Recommended)

```bash
# Install Pulumi
curl -fsSL https://get.pulumi.com | sh

# Deploy infrastructure
cd deployment/pulumi
pulumi up
```

### Using SAM

```bash
# Build and deploy
cd lambda
make build
sam deploy --guided
```

## 6. Test Your Setup

```bash
# Test WebSocket connection
wscat -c "wss://your-api-id.execute-api.region.amazonaws.com/prod?Authorization=your-jwt-token"

# Send test message
{"action": "generate_report", "id": "test-123", "payload": {"format": "pdf"}}
```

## What Happens Next?

1. **Sync vs Async Decision**: Router checks `EstimatedDuration()` - since it's > 5 seconds, request goes async
2. **Queue**: Request is stored in DynamoDB with status "PENDING"
3. **Acknowledgment**: Client receives immediate confirmation that request is queued
4. **Processing**: DynamoDB Stream triggers Processor Lambda
5. **Progress Updates**: Handler calls `reporter.Report()` which sends real-time updates via WebSocket
6. **Completion**: Final result is sent to client when `reporter.Complete()` is called

## Next Steps

- [Create more handlers](../guides/CREATING_HANDLERS.md)
- [Set up authentication](../guides/AUTHENTICATION.md)
- [Deploy to production](../deployment/PRODUCTION.md)
- [Monitor your system](../guides/MONITORING.md)

## Common Issues

**WebSocket connection fails**: Check JWT token format and API Gateway configuration
**No progress updates**: Verify DynamoDB Streams are enabled and Processor Lambda is deployed
**Timeouts**: Ensure Lambda timeout is set appropriately (15 minutes max)

Need help? Check our [Troubleshooting Guide](../reference/TROUBLESHOOTING.md). 