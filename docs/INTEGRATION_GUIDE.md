# Streamer Integration Guide

> **For Development Teams**: Complete guide to integrating Streamer for async request processing

## Overview

Streamer enables your applications to handle long-running operations (>5 seconds) by providing an async request/response pattern with real-time progress updates via WebSocket. This guide will help your team integrate Streamer quickly and effectively.

## Integration Scenarios

### Scenario 1: Adding Async Processing to Existing APIs

**Use Case**: You have REST APIs that sometimes timeout due to long processing times.

**Solution**: Implement Streamer handlers for long-running operations while keeping short operations synchronous.

### Scenario 2: Real-time Progress Updates

**Use Case**: Users need to see progress for operations like file processing, report generation, or data imports.

**Solution**: Use Streamer's progress reporting to provide real-time updates.

### Scenario 3: Batch Processing

**Use Case**: Processing large datasets that exceed Lambda timeout limits.

**Solution**: Break work into chunks and use progress reporting to track completion.

## Step-by-Step Integration

### 1. Identify Long-Running Operations

Audit your existing operations to identify candidates for async processing:
- File uploads/processing
- Report generation
- Data exports
- Complex calculations
- External API calls with retries

### 2. Design Your Handlers

For each long-running operation, create a handler:

```go
type DataExportHandler struct {
    db       Database
    s3Client S3Client
}

func (h *DataExportHandler) EstimatedDuration() time.Duration {
    return 5 * time.Minute // This triggers async processing
}

func (h *DataExportHandler) Validate(req *streamer.Request) error {
    var params ExportParams
    if err := json.Unmarshal(req.Payload, &params); err != nil {
        return streamer.NewError(streamer.ErrCodeValidation, "Invalid payload")
    }
    
    if params.StartDate.After(params.EndDate) {
        return streamer.NewError(streamer.ErrCodeValidation, "Invalid date range")
    }
    
    return nil
}

func (h *DataExportHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    var params ExportParams
    json.Unmarshal(req.Payload, &params)
    
    // Step 1: Query data
    reporter.Report(10, "Querying database...")
    data, err := h.db.QueryRange(ctx, params.StartDate, params.EndDate)
    if err != nil {
        return nil, err
    }
    
    // Step 2: Process in chunks
    totalRecords := len(data)
    processedRecords := 0
    
    for i, chunk := range chunkData(data, 1000) {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
            // Process chunk
            processedData := h.processChunk(chunk)
            processedRecords += len(chunk)
            
            // Report progress
            percentage := float64(processedRecords) / float64(totalRecords) * 80
            reporter.Report(percentage+10, fmt.Sprintf("Processed %d/%d records", processedRecords, totalRecords))
        }
    }
    
    // Step 3: Upload to S3
    reporter.Report(90, "Uploading export file...")
    url, err := h.s3Client.Upload(ctx, processedData)
    if err != nil {
        return nil, err
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]interface{}{
            "export_url":     url,
            "total_records":  totalRecords,
            "file_size":      len(processedData),
            "expires_at":     time.Now().Add(24 * time.Hour),
        },
    }, nil
}
```

### 3. Set Up Infrastructure

Use the provided Pulumi templates or deploy manually:

```bash
# Quick deployment
cd deployment/pulumi
cp Pulumi.dev.yaml.template Pulumi.yourenv.yaml
# Edit configuration
pulumi up -s yourenv
```

### 4. Register Handlers

In your Lambda router function:

```go
func init() {
    router = streamer.NewRouter(requestQueue, connectionManager)
    
    // Register your handlers
    router.Handle("export_data", &DataExportHandler{
        db:       database.New(),
        s3Client: s3.New(),
    })
    
    router.Handle("generate_report", &ReportHandler{})
    router.Handle("process_upload", &UploadHandler{})
}
```

### 5. Client Integration

#### Frontend (JavaScript/React)

```javascript
import { StreamerClient } from '@your-org/streamer-client';

const client = new StreamerClient(process.env.REACT_APP_WEBSOCKET_URL, {
    token: authToken,
    onConnect: () => console.log('Connected to Streamer'),
    onDisconnect: () => console.log('Disconnected from Streamer'),
});

// Async operation with progress
const handleExport = async () => {
    try {
        const result = await client.request('export_data', {
            start_date: '2024-01-01',
            end_date: '2024-12-31',
            format: 'csv'
        }, {
            onProgress: (progress) => {
                setProgress(progress.percentage);
                setStatusMessage(progress.message);
            }
        });
        
        // Operation completed
        setDownloadUrl(result.export_url);
    } catch (error) {
        setError(error.message);
    }
};
```

#### Backend Integration (Go)

```go
import "github.com/pay-theory/streamer/pkg/client"

func exportDataEndpoint(w http.ResponseWriter, r *http.Request) {
    // For operations you want to handle via Streamer
    client := streamer.NewClient(websocketURL, &streamer.ClientConfig{
        Token: getAuthToken(r),
    })
    
    result, err := client.RequestWithProgress(r.Context(), "export_data", params, func(progress client.Progress) {
        // Optional: Store progress in database for UI polling
        updateProgressInDB(requestID, progress)
    })
    
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}
```

## Testing Your Integration

### 1. Unit Tests

```go
func TestDataExportHandler(t *testing.T) {
    handler := &DataExportHandler{
        db:       &MockDB{},
        s3Client: &MockS3{},
    }
    
    req := &streamer.Request{
        ID:      "test-123",
        Action:  "export_data",
        Payload: []byte(`{"start_date":"2024-01-01","end_date":"2024-12-31"}`),
    }
    
    reporter := &MockProgressReporter{}
    
    result, err := handler.ProcessWithProgress(context.Background(), req, reporter)
    
    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.Contains(t, result.Data, "export_url")
}
```

### 2. Integration Tests

```bash
# Test WebSocket connection
wscat -c "wss://your-api.execute-api.region.amazonaws.com/prod?Authorization=Bearer $TOKEN"

# Send test request
{"action": "export_data", "id": "test-123", "payload": {"start_date": "2024-01-01", "end_date": "2024-01-31"}}
```

## Monitoring & Observability

### Key Metrics to Track

1. **Request Volume**: Number of async requests per minute
2. **Processing Time**: Average duration by handler type
3. **Error Rates**: Failed requests by error type
4. **Connection Health**: Active WebSocket connections
5. **Progress Reporting**: Frequency of progress updates

### CloudWatch Dashboards

```json
{
  "widgets": [
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["Streamer", "AsyncRequests", "Handler", "export_data"],
          [".", "ProcessingDuration", "Handler", "export_data"],
          [".", "ErrorRate", "Handler", "export_data"]
        ],
        "period": 300,
        "stat": "Average",
        "region": "us-east-1",
        "title": "Data Export Handler Performance"
      }
    }
  ]
}
```

## Common Integration Patterns

### Pattern 1: Hybrid Sync/Async

Some operations might be fast or slow depending on input:

```go
func (h *SmartHandler) EstimatedDuration() time.Duration {
    // Always assume async - router will decide based on actual processing
    return 10 * time.Second
}

func (h *SmartHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    // Quick operations can still complete synchronously
    if h.isQuickOperation(req) {
        return h.processQuickly(ctx, req)
    }
    
    // Complex operations should not be handled here
    return nil, streamer.NewError(streamer.ErrCodeInternalError, "Use ProcessWithProgress for complex operations")
}
```

### Pattern 2: Chunked Processing

For very large datasets:

```go
func (h *BatchHandler) ProcessWithProgress(ctx context.Context, req *streamer.Request, reporter progress.Reporter) (*streamer.Result, error) {
    chunks := h.createChunks(req.Payload)
    results := make([]interface{}, 0, len(chunks))
    
    for i, chunk := range chunks {
        result, err := h.processChunk(ctx, chunk)
        if err != nil {
            return nil, err
        }
        
        results = append(results, result)
        
        // Report progress
        percentage := float64(i+1) / float64(len(chunks)) * 100
        reporter.Report(percentage, fmt.Sprintf("Processed chunk %d/%d", i+1, len(chunks)))
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data:      map[string]interface{}{"results": results},
    }, nil
}
```

### Pattern 3: External Service Integration

When calling external services:

```go
func (h *ExternalAPIHandler) ProcessWithProgress(ctx context.Context, req *streamer.Request, reporter progress.Reporter) (*streamer.Result, error) {
    reporter.Report(10, "Initiating external API call...")
    
    // Make external API call with retries
    var result ExternalAPIResponse
    for attempt := 1; attempt <= 3; attempt++ {
        resp, err := h.externalClient.CallAPI(ctx, req.Payload)
        if err == nil {
            result = resp
            break
        }
        
        if attempt < 3 {
            reporter.Report(20+float64(attempt)*10, fmt.Sprintf("Retrying... (attempt %d/3)", attempt+1))
            time.Sleep(time.Duration(attempt) * time.Second)
        } else {
            return nil, fmt.Errorf("external API failed after 3 attempts: %w", err)
        }
    }
    
    reporter.Report(90, "Processing external API response...")
    processedResult := h.processExternalResponse(result)
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data:      processedResult,
    }, nil
}
```

## Error Handling Best Practices

### 1. Use Appropriate Error Codes

```go
// Validation errors
if invalid {
    return nil, streamer.NewError(streamer.ErrCodeValidation, "Field X is required")
}

// External service errors
if serviceDown {
    return nil, streamer.NewError(streamer.ErrCodeServiceUnavailable, "External service temporarily unavailable").
        WithRetry(5 * time.Minute)
}

// Rate limiting
if rateLimited {
    return nil, streamer.NewError(streamer.ErrCodeRateLimited, "Rate limit exceeded").
        WithRetry(1 * time.Minute)
}
```

### 2. Provide Actionable Error Messages

```go
// Bad
return nil, streamer.NewError(streamer.ErrCodeValidation, "Invalid input")

// Good
return nil, streamer.NewError(streamer.ErrCodeValidation, "Date range cannot exceed 1 year. Provided range: 18 months")
```

## Performance Optimization

### 1. Connection Pooling

```go
// Initialize connection pools in handler constructors
func NewDataHandler() *DataHandler {
    return &DataHandler{
        dbPool: database.NewPool(maxConnections),
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
            },
        },
    }
}
```

### 2. Context Awareness

```go
func (h *Handler) ProcessWithProgress(ctx context.Context, req *streamer.Request, reporter progress.Reporter) (*streamer.Result, error) {
    for i, item := range items {
        select {
        case <-ctx.Done():
            // Graceful shutdown
            return nil, ctx.Err()
        default:
            // Continue processing
            if err := h.processItem(ctx, item); err != nil {
                return nil, err
            }
            
            reporter.Report(float64(i)/float64(len(items))*100, "Processing...")
        }
    }
}
```

### 3. Memory Management

```go
// Process large datasets in chunks to avoid memory issues
func (h *Handler) processLargeDataset(data []Record) error {
    const chunkSize = 1000
    
    for i := 0; i < len(data); i += chunkSize {
        end := i + chunkSize
        if end > len(data) {
            end = len(data)
        }
        
        chunk := data[i:end]
        if err := h.processChunk(chunk); err != nil {
            return err
        }
        
        // Allow garbage collection
        runtime.GC()
    }
    
    return nil
}
```

## Security Considerations

### 1. Input Validation

```go
func (h *Handler) Validate(req *streamer.Request) error {
    var params RequestParams
    if err := json.Unmarshal(req.Payload, &params); err != nil {
        return streamer.NewError(streamer.ErrCodeValidation, "Invalid JSON payload")
    }
    
    // Validate file size limits
    if params.FileSize > maxFileSize {
        return streamer.NewError(streamer.ErrCodeValidation, 
            fmt.Sprintf("File too large. Max size: %d bytes", maxFileSize))
    }
    
    // Validate allowed file types
    if !isAllowedFileType(params.FileType) {
        return streamer.NewError(streamer.ErrCodeValidation, "File type not allowed")
    }
    
    return nil
}
```

### 2. Authorization

```go
func (h *Handler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    // Extract user context from request metadata
    userID := req.Metadata["user_id"]
    tenantID := req.Metadata["tenant_id"]
    
    // Check permissions
    if !h.authService.HasPermission(userID, "export_data", tenantID) {
        return nil, streamer.NewError(streamer.ErrCodeUnauthorized, "Insufficient permissions")
    }
    
    // Continue with processing...
}
```

## Troubleshooting Guide

### Common Issues

| Issue | Symptom | Solution |
|-------|---------|----------|
| No progress updates | Client receives ack but no progress | Check DynamoDB Streams configuration |
| High latency | Slow response times | Review Lambda memory allocation |
| Connection drops | WebSocket disconnects | Check API Gateway timeout settings |
| Memory errors | Lambda OOM | Process data in smaller chunks |

### Debug Commands

```bash
# Check Lambda logs
aws logs tail /aws/lambda/streamer-processor-prod --follow

# Monitor DynamoDB streams
aws dynamodb describe-stream --stream-arn $STREAM_ARN

# Test WebSocket connection
wscat -c "wss://your-endpoint" -H "Authorization: Bearer $TOKEN"
```

## Next Steps

1. **Start Small**: Begin with one long-running operation
2. **Monitor Performance**: Set up CloudWatch dashboards
3. **Gather Feedback**: Track user experience with async operations
4. **Iterate**: Optimize based on real usage patterns
5. **Scale Up**: Add more handlers as needed

## Support Resources

- [API Reference](./api/HANDLER_INTERFACE.md)
- [Deployment Guide](./deployment/README.md)
- [Architecture Overview](./ARCHITECTURE.md)
- [Example Implementations](../examples/)

For technical support, create an issue in the repository or contact the Streamer team. 