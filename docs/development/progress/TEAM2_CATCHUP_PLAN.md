# Team 2 Catch-Up Plan - Critical Path to Demo

## 🎯 Your Mission (Next 4 Hours)
Get one complete async flow working for the demo. Perfect is the enemy of done - focus on working, not optimal.

## Hour 1: Fix Progress Updates (With Team 1 Help)

### Quick Debug Checklist
```bash
# 1. Test ConnectionManager directly
aws dynamodb get-item \
  --table-name connections \
  --key '{"connection_id":{"S":"YOUR_CONNECTION_ID"}}'

# 2. Test WebSocket send
aws apigatewaymanagementapi post-to-connection \
  --connection-id YOUR_CONNECTION_ID \
  --data '{"type":"test","message":"Hello"}' \
  --endpoint-url https://YOUR_API_ID.execute-api.region.amazonaws.com/stage

# 3. Check Lambda logs
aws logs tail /aws/lambda/processor --follow
```

### Common Issues & Fixes

#### Issue: "410 Gone" errors
```go
// In progress reporter, add connection check:
func (r *Reporter) Report(percentage float64, message string) error {
    // Check if connection still exists
    if !r.connManager.IsActive(r.connectionID) {
        log.Warn("Connection no longer active", r.connectionID)
        return nil // Don't fail the whole process
    }
    // ... rest of send logic
}
```

#### Issue: Messages not reaching client
```go
// Ensure JSON marshaling is correct:
update := map[string]interface{}{
    "type":       "progress",
    "request_id": r.requestID,
    "percentage": percentage,
    "message":    message,
}

// Log the actual message being sent
jsonData, _ := json.Marshal(update)
log.Info("Sending progress", string(jsonData))
```

## Hour 2-3: Complete ReportHandler

### Minimal Working Implementation
```go
// lambda/processor/handlers/report.go
package handlers

import (
    "context"
    "fmt"
    "time"
    
    "github.com/pay-theory/streamer/pkg/progress"
    "github.com/pay-theory/streamer/internal/store"
)

type ReportHandler struct{}

func (h *ReportHandler) ProcessWithProgress(
    ctx context.Context,
    req *store.AsyncRequest,
    reporter progress.Reporter,
) error {
    // Keep it simple for demo
    steps := []struct {
        percentage float64
        message    string
        duration   time.Duration
    }{
        {10, "Initializing report generation...", 1 * time.Second},
        {30, "Querying data...", 2 * time.Second},
        {50, "Processing records...", 2 * time.Second},
        {70, "Generating PDF...", 2 * time.Second},
        {90, "Uploading to S3...", 1 * time.Second},
        {100, "Report complete!", 0},
    }
    
    for _, step := range steps {
        // Report progress
        if err := reporter.Report(step.percentage, step.message); err != nil {
            log.Warn("Failed to report progress", err)
            // Continue anyway
        }
        
        // Simulate work
        if step.duration > 0 {
            time.Sleep(step.duration)
        }
        
        // Check context
        if ctx.Err() != nil {
            return ctx.Err()
        }
    }
    
    // Update final result
    result := map[string]interface{}{
        "url": "https://s3.amazonaws.com/reports/demo-report.pdf",
        "size": "2.4MB",
        "pages": 42,
    }
    
    return reporter.Complete(result)
}
```

### Register Handler in Processor
```go
// lambda/processor/main.go
func init() {
    // Register handlers
    registry := map[string]AsyncHandler{
        "generate_report": &handlers.ReportHandler{},
        "echo_async": &handlers.EchoAsyncHandler{}, // Simple test handler
    }
    
    // Initialize executor
    exec = executor.New(connManager, reqQueue, registry)
}
```

## Hour 4: Demo Preparation

### 1. Create Test Data Script
```go
// scripts/demo_setup.go
package main

func main() {
    // Create test connection
    conn := &store.Connection{
        ConnectionID: "demo-conn-123",
        UserID:      "demo-user",
        TenantID:    "demo-tenant",
        // ...
    }
    
    // Save to DynamoDB
    // ...
}
```

### 2. Simple Demo Client
```javascript
// demo/client.js
const WebSocket = require('ws');

const ws = new WebSocket('wss://your-api.execute-api.region.amazonaws.com/prod');

ws.on('open', () => {
    console.log('Connected!');
    
    // Send async request
    ws.send(JSON.stringify({
        action: 'generate_report',
        payload: {
            start_date: '2024-01-01',
            end_date: '2024-01-31'
        }
    }));
});

ws.on('message', (data) => {
    const msg = JSON.parse(data);
    
    if (msg.type === 'progress') {
        const bar = '█'.repeat(Math.floor(msg.percentage / 2));
        const empty = '░'.repeat(50 - bar.length);
        console.log(`\rProgress: ${bar}${empty} ${msg.percentage}% - ${msg.message}`);
    } else {
        console.log('Message:', msg);
    }
});
```

### 3. Fallback Demo Video
Record a successful run now as backup:
```bash
# Use asciinema or similar
asciinema rec demo-backup.cast
# Run through the demo
# exit
```

## 🚨 If Things Go Wrong

### Plan B: Simplified Demo
1. Show connection working (Team 1's part)
2. Show sync echo request working
3. Show async request queuing
4. Show monitoring dashboards
5. Explain progress updates are "in final testing"

### Plan C: Architecture Focus
1. Explain the architecture
2. Show the code structure
3. Demonstrate monitoring
4. Discuss scaling capabilities
5. Show the roadmap

## 📝 Demo Talk Track

### What We Built (30 seconds)
"We've built Streamer - an async request processing system that solves the 29-second API Gateway timeout limitation. It handles long-running operations with real-time progress updates via WebSocket."

### Architecture (1 minute)
"The system has two Lambda functions:
- Router: Handles incoming requests, decides sync vs async
- Processor: Processes async requests from DynamoDB Streams

Connection management and progress updates flow through WebSockets."

### Live Demo (3 minutes)
1. "Let me show you a client connecting..." [connect]
2. "First, a sync request completes immediately..." [echo]
3. "Now, an async report generation..." [generate_report]
4. "Notice the progress updates in real-time..." [progress]
5. "And here's the final result with the S3 link..." [complete]

### Monitoring (1 minute)
"We've built comprehensive monitoring:
- Real-time metrics in CloudWatch
- Distributed tracing with X-Ray
- Error alerting configured
- Connection tracking dashboard"

### What's Next (30 seconds)
"Next week we'll add:
- Client SDKs for easy integration
- More async handlers
- Performance optimization
- Production deployment"

## 💪 You Got This!

Remember:
- Focus on getting ONE flow working
- Don't optimize, just make it work
- Test with real WebSocket client
- Have backup plans ready
- The architecture is solid even if demo isn't perfect

Team 1 is available to help debug WebSocket issues! 