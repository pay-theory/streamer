# Streamer Integration Guide for Development Teams

> **Complete guide for teams integrating Streamer into their applications**

## Quick Start for Teams

### What is Streamer?
Streamer solves the 29-second API Gateway timeout problem by providing async request processing with real-time WebSocket progress updates. Perfect for:
- File processing & uploads
- Report generation  
- Data exports/imports
- Long-running calculations
- External API integrations

### Integration Decision Tree

```
Is your operation > 5 seconds? ──No──► Keep using REST APIs
                │
               Yes
                │
Do users need progress updates? ──No──► Use simple async handler
                │
               Yes  
                │
         Use Streamer with progress reporting
```

## Step 1: Identify Integration Points

**Audit your current operations:**

| Operation | Current Duration | User Experience Issue | Streamer Solution |
|-----------|------------------|----------------------|-------------------|
| PDF Report Gen | 15-45 seconds | Timeouts, no feedback | Async + Progress |
| File Upload Processing | 2-20 seconds | Users wait blindly | Progress updates |
| Data Export | 30+ seconds | Frequent failures | Chunked processing |
| External API Calls | Variable | Unreliable | Retry with progress |

## Step 2: Handler Implementation Patterns

### Pattern A: Simple Async Handler
For operations that don't need progress updates:

```go
type SimpleAsyncHandler struct {
    service YourService
}

func (h *SimpleAsyncHandler) EstimatedDuration() time.Duration {
    return 10 * time.Second // >5s triggers async
}

func (h *SimpleAsyncHandler) Validate(req *streamer.Request) error {
    // Validate input early
    return h.service.ValidateInput(req.Payload)
}

func (h *SimpleAsyncHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    result, err := h.service.DoLongOperation(ctx, req.Payload)
    if err != nil {
        return nil, err
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data:      result,
    }, nil
}
```

### Pattern B: Progress-Aware Handler  
For operations where users need feedback:

```go
type ProgressAwareHandler struct {
    processor DataProcessor
}

func (h *ProgressAwareHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    
    // Step 1: Initialize (10%)
    reporter.Report(10, "Starting data processing...")
    data, err := h.processor.LoadData(ctx, req.Payload)
    if err != nil {
        return nil, err
    }
    
    // Step 2: Process in chunks (10-90%)
    totalChunks := len(data) / chunkSize
    for i, chunk := range h.processor.ChunkData(data, chunkSize) {
        // Check for cancellation
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
        
        err := h.processor.ProcessChunk(ctx, chunk)
        if err != nil {
            return nil, err
        }
        
        // Update progress
        progress := 10 + (float64(i+1)/float64(totalChunks))*80
        reporter.Report(progress, fmt.Sprintf("Processed %d/%d chunks", i+1, totalChunks))
    }
    
    // Step 3: Finalize (90-100%)
    reporter.Report(95, "Generating output...")
    output, err := h.processor.GenerateOutput(ctx)
    if err != nil {
        return nil, err
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]interface{}{
            "output_url": output.URL,
            "size":       output.Size,
            "duration":   time.Since(startTime).String(),
        },
    }, nil
}
```

## Step 3: Client Integration

### Frontend (React/JavaScript)

```javascript
// Install the client library
// npm install @your-org/streamer-client

import { StreamerClient } from '@your-org/streamer-client';
import { useState } from 'react';

function AsyncOperationComponent() {
    const [progress, setProgress] = useState(0);
    const [status, setStatus] = useState('');
    const [result, setResult] = useState(null);
    const [loading, setLoading] = useState(false);
    
    const client = new StreamerClient(process.env.REACT_APP_WEBSOCKET_URL, {
        token: authToken
    });
    
    const handleAsyncOperation = async () => {
        setLoading(true);
        setProgress(0);
        
        try {
            const result = await client.request('process_data', {
                input: formData
            }, {
                onProgress: (progressUpdate) => {
                    setProgress(progressUpdate.percentage);
                    setStatus(progressUpdate.message);
                },
                onError: (error) => {
                    console.error('Operation failed:', error);
                    setLoading(false);
                }
            });
            
            setResult(result);
            setStatus('Completed successfully!');
        } catch (error) {
            setStatus(`Error: ${error.message}`);
        } finally {
            setLoading(false);
        }
    };
    
    return (
        <div>
            <button onClick={handleAsyncOperation} disabled={loading}>
                {loading ? 'Processing...' : 'Start Processing'}
            </button>
            
            {loading && (
                <div>
                    <div className="progress-bar">
                        <div 
                            className="progress-fill" 
                            style={{width: `${progress}%`}}
                        />
                    </div>
                    <p>{status}</p>
                </div>
            )}
            
            {result && (
                <div>
                    <h3>Results:</h3>
                    <pre>{JSON.stringify(result, null, 2)}</pre>
                </div>
            )}
        </div>
    );
}
```

### Backend Integration (Go HTTP Handler)

```go
// For existing REST endpoints that need async processing
func (s *Server) processDataEndpoint(w http.ResponseWriter, r *http.Request) {
    // Parse request
    var params ProcessingParams
    if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // Quick validation
    if err := params.Validate(); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // For large/slow operations, delegate to Streamer
    if params.EstimatedSize > largeDataThreshold {
        // Return WebSocket connection info for async processing
        response := map[string]interface{}{
            "async":           true,
            "websocket_url":   s.config.WebSocketURL,
            "action":          "process_data",
            "request_payload": params,
            "message":         "Connect to WebSocket for real-time progress",
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
        return
    }
    
    // Handle small operations synchronously
    result, err := s.processor.ProcessSmallData(r.Context(), params)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}
```

## Step 4: Infrastructure Setup

### Option A: Use Pulumi (Recommended)

```bash
# Copy and customize the Pulumi template
cp deployment/pulumi/Pulumi.dev.yaml.template deployment/pulumi/Pulumi.yourteam.yaml

# Edit configuration
cat > deployment/pulumi/Pulumi.yourteam.yaml << EOF
config:
  aws:region: us-west-2
  streamer:environment: yourteam-prod
  streamer:domain: wss://api-yourteam.yourdomain.com
  streamer:jwtPublicKey: |
    -----BEGIN PUBLIC KEY-----
    YOUR_JWT_PUBLIC_KEY_HERE
    -----END PUBLIC KEY-----
  streamer:connectionsTable: streamer-connections-yourteam
  streamer:requestsTable: streamer-requests-yourteam
EOF

# Deploy
cd deployment/pulumi
pulumi up -s yourteam
```

### Option B: Manual AWS Setup

```bash
# 1. Create DynamoDB tables
aws dynamodb create-table \
  --table-name streamer-connections-yourteam \
  --attribute-definitions \
    AttributeName=ConnectionID,AttributeType=S \
    AttributeName=UserID,AttributeType=S \
    AttributeName=TenantID,AttributeType=S \
  --key-schema AttributeName=ConnectionID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --global-secondary-indexes \
    'IndexName=UserIndex,Keys=[{AttributeName=UserID,KeyType=HASH}],Projection={ProjectionType=ALL}' \
    'IndexName=TenantIndex,Keys=[{AttributeName=TenantID,KeyType=HASH}],Projection={ProjectionType=ALL}'

# 2. Build and deploy Lambda functions
make build-lambda
aws lambda create-function --function-name streamer-router-yourteam \
  --runtime provided.al2 --role $LAMBDA_ROLE_ARN \
  --handler bootstrap --zip-file fileb://bin/router.zip

# 3. Set up API Gateway WebSocket API
# (Use the AWS Console or CLI - see deployment guide)
```

## Step 5: Testing Your Integration

### Unit Tests

```go
func TestYourHandler(t *testing.T) {
    handler := &YourHandler{
        service: &MockService{},
    }
    
    // Test validation
    req := &streamer.Request{
        ID:      "test-123",
        Payload: json.RawMessage(`{"invalid": "payload"}`),
    }
    
    err := handler.Validate(req)
    assert.Error(t, err)
    
    // Test successful processing
    req.Payload = json.RawMessage(`{"valid": "payload"}`)
    
    reporter := &progress.MockReporter{}
    result, err := handler.ProcessWithProgress(context.Background(), req, reporter)
    
    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.True(t, reporter.ProgressWasReported())
}
```

### Integration Tests

```bash
# Test WebSocket connection
wscat -c "wss://your-api.execute-api.region.amazonaws.com/prod" \
  -H "Authorization: Bearer $YOUR_JWT_TOKEN"

# Send test message
{
  "action": "your_handler_action",
  "id": "test-123",
  "payload": {
    "test": "data"
  }
}

# Expected responses:
# 1. Immediate acknowledgment
# 2. Progress updates
# 3. Final result
```

## Monitoring & Alerting

### Essential CloudWatch Dashboards

```json
{
  "widgets": [
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["Streamer", "RequestCount", "Team", "YourTeam"],
          [".", "ErrorRate", "Team", "YourTeam"],
          [".", "AverageProcessingTime", "Team", "YourTeam"],
          [".", "ActiveConnections", "Team", "YourTeam"]
        ],
        "period": 300,
        "stat": "Average",
        "region": "us-west-2",
        "title": "YourTeam Streamer Metrics"
      }
    }
  ]
}
```

### Recommended Alerts  

```bash
# High error rate
aws cloudwatch put-metric-alarm \
  --alarm-name "YourTeam-Streamer-HighErrors" \
  --alarm-description "High error rate for YourTeam Streamer operations" \
  --metric-name ErrorRate \
  --namespace Streamer \
  --statistic Average \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 5.0 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=Team,Value=YourTeam

# Long processing times
aws cloudwatch put-metric-alarm \
  --alarm-name "YourTeam-Streamer-SlowProcessing" \
  --alarm-description "Processing taking too long" \
  --metric-name AverageProcessingTime \
  --namespace Streamer \
  --statistic Average \
  --period 300 \
  --evaluation-periods 3 \
  --threshold 300 \
  --comparison-operator GreaterThanThreshold \
  --dimensions Name=Team,Value=YourTeam
```

## Common Integration Patterns

### Pattern 1: File Processing Pipeline

```go
type FileProcessingHandler struct {
    s3Client   S3Client
    processor  FileProcessor
    notifier   NotificationService
}

func (h *FileProcessingHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    var fileRequest FileProcessingRequest
    json.Unmarshal(req.Payload, &fileRequest)
    
    // Step 1: Download file (0-20%)
    reporter.Report(5, "Downloading file from S3...")
    file, err := h.s3Client.Download(ctx, fileRequest.S3Key)
    if err != nil {
        return nil, fmt.Errorf("failed to download file: %w", err)
    }
    reporter.Report(20, "File downloaded successfully")
    
    // Step 2: Process file (20-80%)
    reporter.Report(25, "Starting file processing...")
    result, err := h.processor.ProcessFile(ctx, file, func(progress float64) {
        // Convert processor progress (0-1) to our range (20-80%)
        overallProgress := 20 + (progress * 60)
        reporter.Report(overallProgress, "Processing file...")
    })
    if err != nil {
        return nil, fmt.Errorf("file processing failed: %w", err)
    }
    
    // Step 3: Upload results (80-95%)
    reporter.Report(85, "Uploading processed file...")
    resultKey, err := h.s3Client.Upload(ctx, result)
    if err != nil {
        return nil, fmt.Errorf("failed to upload result: %w", err)
    }
    
    // Step 4: Send notifications (95-100%)
    reporter.Report(95, "Sending notifications...")
    err = h.notifier.NotifyProcessingComplete(ctx, fileRequest.UserID, resultKey)
    if err != nil {
        // Log error but don't fail the entire operation
        log.Printf("Failed to send notification: %v", err)
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]interface{}{
            "processed_file_key": resultKey,
            "original_file_size": file.Size,
            "processed_file_size": result.Size,
            "processing_duration": result.Duration,
        },
    }, nil
}
```

### Pattern 2: Multi-Stage Data Pipeline

```go
type DataPipelineHandler struct {
    extractor  DataExtractor
    transformer DataTransformer
    loader     DataLoader
}

func (h *DataPipelineHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    var pipeline DataPipelineRequest
    json.Unmarshal(req.Payload, &pipeline)
    
    // Stage 1: Extract (0-30%)
    reporter.Report(5, "Extracting data from sources...")
    extractedData, err := h.extractor.Extract(ctx, pipeline.Sources, func(progress float64) {
        reporter.Report(progress*0.25, "Extracting data...")
    })
    if err != nil {
        return nil, fmt.Errorf("data extraction failed: %w", err)
    }
    reporter.Report(30, fmt.Sprintf("Extracted %d records", len(extractedData)))
    
    // Stage 2: Transform (30-70%)
    reporter.Report(35, "Transforming data...")
    transformedData, err := h.transformer.Transform(ctx, extractedData, pipeline.Rules, func(progress float64) {
        overallProgress := 30 + (progress * 40)
        reporter.Report(overallProgress, "Transforming data...")
    })
    if err != nil {
        return nil, fmt.Errorf("data transformation failed: %w", err)
    }
    reporter.Report(70, fmt.Sprintf("Transformed %d records", len(transformedData)))
    
    // Stage 3: Load (70-100%)
    reporter.Report(75, "Loading data to destination...")
    loadResult, err := h.loader.Load(ctx, transformedData, pipeline.Destination, func(progress float64) {
        overallProgress := 70 + (progress * 30)
        reporter.Report(overallProgress, "Loading data...")
    })
    if err != nil {
        return nil, fmt.Errorf("data loading failed: %w", err)
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]interface{}{
            "records_extracted":  len(extractedData),
            "records_transformed": len(transformedData),
            "records_loaded":     loadResult.Count,
            "destination_id":     loadResult.DestinationID,
            "duration":          loadResult.Duration,
        },
    }, nil
}
```

## Performance Optimization Tips

### 1. Memory Management

```go
// Process large datasets in chunks to avoid OOM
func (h *Handler) processLargeDataset(ctx context.Context, data []Record, reporter progress.Reporter) error {
    const chunkSize = 1000 // Adjust based on memory constraints
    totalChunks := (len(data) + chunkSize - 1) / chunkSize
    
    for i := 0; i < len(data); i += chunkSize {
        end := i + chunkSize
        if end > len(data) {
            end = len(data)
        }
        
        chunk := data[i:end]
        
        if err := h.processChunk(ctx, chunk); err != nil {
            return err
        }
        
        // Report progress
        chunkNum := (i / chunkSize) + 1
        progress := float64(chunkNum) / float64(totalChunks) * 100
        reporter.Report(progress, fmt.Sprintf("Processed chunk %d/%d", chunkNum, totalChunks))
        
        // Force garbage collection for large datasets
        if len(data) > 10000 && chunkNum%10 == 0 {
            runtime.GC()
        }
    }
    
    return nil
}
```

### 2. Context Cancellation

```go
func (h *Handler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    
    // Create a derived context with timeout
    processCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
    defer cancel()
    
    // Check for cancellation at regular intervals
    for i, item := range items {
        select {
        case <-processCtx.Done():
            if processCtx.Err() == context.DeadlineExceeded {
                return nil, streamer.NewError(streamer.ErrCodeTimeout, "Processing exceeded 15 minute limit")
            }
            return nil, processCtx.Err()
        default:
            // Continue processing
        }
        
        if err := h.processItem(processCtx, item); err != nil {
            return nil, err
        }
        
        // Report progress every 10 items or every 5 seconds
        if i%10 == 0 || time.Since(lastProgressReport) > 5*time.Second {
            progress := float64(i+1) / float64(len(items)) * 100
            reporter.Report(progress, fmt.Sprintf("Processed %d/%d items", i+1, len(items)))
            lastProgressReport = time.Now()
        }
    }
    
    return result, nil
}
```

### 3. Connection Pooling

```go
// Initialize shared resources in handler constructor
func NewYourHandler() *YourHandler {
    return &YourHandler{
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:       100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:    90 * time.Second,
            },
        },
        dbPool: database.NewPool(&database.Config{
            MaxOpenConns: 25,
            MaxIdleConns: 5,
            MaxLifetime:  30 * time.Minute,
        }),
    }
}
```

## Troubleshooting Guide

### Issue: No Progress Updates Received

**Symptoms:** Client gets acknowledgment but no progress updates

**Debugging Steps:**
1. Check DynamoDB Streams are enabled on requests table
2. Verify processor Lambda has stream trigger configured
3. Check processor Lambda logs for errors
4. Verify WebSocket connection is still active

```bash
# Check stream configuration
aws dynamodb describe-table --table-name streamer-requests-yourteam | jq '.Table.StreamSpecification'

# Check Lambda event source mapping
aws lambda list-event-source-mappings --function-name streamer-processor-yourteam

# Check recent processor logs
aws logs tail /aws/lambda/streamer-processor-yourteam --follow
```

### Issue: Handler Timing Out

**Symptoms:** Operations fail with timeout errors

**Solutions:**
1. Increase Lambda timeout (max 15 minutes)
2. Break work into smaller chunks
3. Add more frequent progress reports
4. Optimize database queries

```go
// Add timeout monitoring in your handler
func (h *Handler) ProcessWithProgress(ctx context.Context, req *streamer.Request, reporter progress.Reporter) (*streamer.Result, error) {
    start := time.Now()
    
    defer func() {
        duration := time.Since(start)
        if duration > 10*time.Minute {
            log.Printf("WARNING: Handler took %v to process request %s", duration, req.ID)
        }
    }()
    
    // Your processing logic...
}
```

### Issue: High Memory Usage

**Symptoms:** Lambda OOM errors, slow performance

**Solutions:**
1. Process data in smaller chunks
2. Use streaming where possible
3. Implement garbage collection hints
4. Increase Lambda memory allocation

```go
// Monitor memory usage in handler
func (h *Handler) ProcessWithProgress(ctx context.Context, req *streamer.Request, reporter progress.Reporter) (*streamer.Result, error) {
    var m runtime.MemStats
    
    for i, batch := range batches {
        runtime.ReadMemStats(&m)
        if m.Alloc > 100*1024*1024 { // 100MB threshold
            runtime.GC()
            runtime.ReadMemStats(&m)
            log.Printf("Memory after GC: %d MB", m.Alloc/1024/1024)
        }
        
        // Process batch...
    }
}
```

## Team Onboarding Checklist

### Pre-Integration
- [ ] Identify operations > 5 seconds in your application
- [ ] Determine which operations need progress feedback
- [ ] Set up development AWS environment  
- [ ] Review Streamer architecture documentation

### Development Phase
- [ ] Implement handlers for identified operations
- [ ] Write unit tests for handlers
- [ ] Set up local development environment with WebSocket testing
- [ ] Create client-side integration (frontend/backend)
- [ ] Test end-to-end integration flow

### Deployment Phase
- [ ] Deploy Streamer infrastructure to staging
- [ ] Configure monitoring and alerting
- [ ] Load test with realistic data volumes
- [ ] Set up production environment
- [ ] Create runbooks for troubleshooting

### Post-Deployment
- [ ] Monitor performance metrics for first week
- [ ] Gather user feedback on experience
- [ ] Optimize based on real usage patterns
- [ ] Document team-specific patterns and learnings

## Getting Help

1. **Documentation:** Start with [API Reference](./api/HANDLER_INTERFACE.md)
2. **Examples:** Check [examples/](../examples/) directory
3. **Issues:** Search existing GitHub issues
4. **Support:** Contact #streamer-support Slack channel
5. **Architecture Questions:** Schedule review with Streamer team

## Success Metrics

Track these metrics to measure integration success:

| Metric | Target | Measurement |
|--------|--------|-------------|
| Operation Timeout Rate | < 1% | CloudWatch alarms |
| User Experience Rating | > 4.5/5 | User surveys |
| Average Processing Time | Baseline - 20% | Application metrics |
| Error Rate | < 2% | CloudWatch metrics |
| Support Ticket Volume | < 5/month | Ticket system |

---

**Ready to integrate?** Start with Step 1 and reach out to the Streamer team if you need help! 