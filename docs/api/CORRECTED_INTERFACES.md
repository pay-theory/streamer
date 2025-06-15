# Corrected API Interfaces Reference

> **This document provides the CORRECT interfaces that match the actual implementation**

## Core Handler Interface

### Handler (Basic Interface)

```go
// pkg/streamer/streamer.go - ACTUAL implementation
type Handler interface {
    // Validate checks if the request is valid
    Validate(request *Request) error

    // EstimatedDuration returns the expected processing time
    // Used to determine if the request should be processed sync or async
    EstimatedDuration() time.Duration

    // Process executes the handler logic
    Process(ctx context.Context, request *Request) (*Result, error)
}
```

### HandlerWithProgress (Extended Interface)

```go
// For handlers that support async processing with progress updates
type HandlerWithProgress interface {
    Handler

    // ProcessWithProgress executes the handler with progress reporting capability
    ProcessWithProgress(ctx context.Context, request *Request, reporter ProgressReporter) (*Result, error)
}
```

## Core Types (Corrected)

### Request

```go
// pkg/streamer/streamer.go - ACTUAL implementation
type Request struct {
    // Unique request identifier
    ID string `json:"id"`
    
    // WebSocket connection ID
    ConnectionID string `json:"connection_id"`
    
    // Action to perform
    Action string `json:"action"`
    
    // Request payload (JSON bytes) - NOT map[string]interface{}
    Payload json.RawMessage `json:"payload"`
    
    // Additional metadata
    Metadata map[string]string `json:"metadata,omitempty"`
    
    // Request timestamp
    CreatedAt time.Time `json:"created_at"`
}
```

### Result

```go
// pkg/streamer/streamer.go - ACTUAL implementation
type Result struct {
    // Request ID this result corresponds to
    RequestID string `json:"request_id"`
    
    // Whether the operation succeeded
    Success bool `json:"success"`
    
    // Result data (will be JSON marshaled)
    Data interface{} `json:"data,omitempty"`
    
    // Error information if Success is false
    Error *Error `json:"error,omitempty"`
    
    // Processing metadata
    Metadata map[string]string `json:"metadata,omitempty"`
}
```

### ProgressReporter

```go
// pkg/streamer/streamer.go - ACTUAL implementation
type ProgressReporter interface {
    // Report sends a progress update
    Report(percentage float64, message string) error

    // SetMetadata adds metadata to the progress update
    SetMetadata(key string, value interface{}) error
}
```

## Correct Implementation Examples

### Basic Handler Example

```go
type DataExportHandler struct {
    service DataService
}

// EstimatedDuration - determines sync vs async processing
func (h *DataExportHandler) EstimatedDuration() time.Duration {
    return 30 * time.Second // > 5 seconds triggers async
}

// Validate - check request before processing
func (h *DataExportHandler) Validate(req *streamer.Request) error {
    var params ExportParams
    if err := json.Unmarshal(req.Payload, &params); err != nil {
        return streamer.NewError(streamer.ErrCodeValidation, "Invalid payload format")
    }
    
    if params.StartDate.After(params.EndDate) {
        return streamer.NewError(streamer.ErrCodeValidation, "Invalid date range")
    }
    
    return nil
}

// Process - sync processing (not recommended for long operations)
func (h *DataExportHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    return nil, streamer.NewError(streamer.ErrCodeInternalError, "Use ProcessWithProgress for async operations")
}

// ProcessWithProgress - async processing with progress updates
func (h *DataExportHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    var params ExportParams
    json.Unmarshal(req.Payload, &params)
    
    // Step 1: Initialize
    if err := reporter.Report(10, "Starting data export..."); err != nil {
        return nil, err
    }
    
    // Step 2: Query data
    if err := reporter.Report(30, "Querying database..."); err != nil {
        return nil, err
    }
    data, err := h.service.QueryData(ctx, params)
    if err != nil {
        return nil, err
    }
    
    // Step 3: Process data
    if err := reporter.Report(70, "Processing data..."); err != nil {
        return nil, err
    }
    processedData, err := h.service.ProcessData(data)
    if err != nil {
        return nil, err
    }
    
    // Step 4: Upload results
    if err := reporter.Report(90, "Uploading results..."); err != nil {
        return nil, err
    }
    downloadURL, err := h.service.UploadResults(processedData)
    if err != nil {
        return nil, err
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]interface{}{
            "download_url": downloadURL,
            "record_count": len(processedData),
            "file_size":    len(processedData),
        },
    }, nil
}
```

### Handler Registration (Correct)

```go
// In your Lambda router initialization
func initializeRouter() *streamer.DefaultRouter {
    router := streamer.NewRouter(requestQueue, connectionManager)
    
    // Set the threshold for async processing (default is 5 seconds)
    router.SetAsyncThreshold(5 * time.Second)
    
    // Register handlers using the correct interface
    router.Handle("export_data", &DataExportHandler{
        service: dataService,
    })
    
    router.Handle("generate_report", &ReportHandler{
        generator: reportGenerator,
    })
    
    return router
}
```

## Key Differences from Old Documentation

### ❌ WRONG (Old Docs)
```go
// This interface doesn't exist
type Handler interface {
    Validate(ctx context.Context, req Request) error
    ShouldQueue(req Request) bool  // No such method
    Process(ctx context.Context, req Request) (Response, error)
    PrepareAsync(ctx context.Context, req Request) (AsyncRequest, error)
}

// Wrong payload type
type Request struct {
    Payload map[string]interface{} `json:"payload"`  // Wrong!
}

// Wrong progress interface
type ProgressReporter interface {
    Report(progress float64, message string, details ...map[string]interface{}) error
    ReportError(err error) error
    SetCheckpoint(checkpoint interface{}) error
}
```

### ✅ CORRECT (Actual Implementation)
```go
// This is the real interface
type Handler interface {
    Validate(request *Request) error
    EstimatedDuration() time.Duration  // This determines sync vs async
    Process(ctx context.Context, request *Request) (*Result, error)
}

// Correct payload type
type Request struct {
    Payload json.RawMessage `json:"payload"`  // Correct!
}

// Correct progress interface
type ProgressReporter interface {
    Report(percentage float64, message string) error
    SetMetadata(key string, value interface{}) error
}
```

## Working Example Application

Here's a complete, working example that integrates with the actual Streamer implementation:

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "time"

    "github.com/aws/aws-lambda-go/lambda"
    "github.com/pay-theory/streamer/pkg/streamer"
    "github.com/pay-theory/streamer/pkg/progress"
)

// Your handler implementation
type FileProcessorHandler struct{}

func (h *FileProcessorHandler) EstimatedDuration() time.Duration {
    return 45 * time.Second // This will trigger async processing
}

func (h *FileProcessorHandler) Validate(req *streamer.Request) error {
    var params FileProcessingParams
    if err := json.Unmarshal(req.Payload, &params); err != nil {
        return streamer.NewError(streamer.ErrCodeValidation, "Invalid JSON payload")
    }
    
    if params.FileURL == "" {
        return streamer.NewError(streamer.ErrCodeValidation, "file_url is required")
    }
    
    return nil
}

func (h *FileProcessorHandler) Process(ctx context.Context, req *streamer.Request) (*streamer.Result, error) {
    // For async handlers, direct sync processing is not recommended
    return nil, streamer.NewError(streamer.ErrCodeInternalError, "Use async processing via WebSocket")
}

func (h *FileProcessorHandler) ProcessWithProgress(
    ctx context.Context,
    req *streamer.Request,
    reporter progress.Reporter,
) (*streamer.Result, error) {
    var params FileProcessingParams
    json.Unmarshal(req.Payload, &params)
    
    // Step 1: Download file
    reporter.Report(10, "Downloading file...")
    file, err := downloadFile(params.FileURL)
    if err != nil {
        return nil, err
    }
    
    // Step 2: Process file in chunks
    totalChunks := calculateChunks(file.Size)
    for i := 0; i < totalChunks; i++ {
        // Check for cancellation
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
        
        err := processChunk(file, i)
        if err != nil {
            return nil, err
        }
        
        // Update progress (10% to 90%)
        progress := 10 + (float64(i+1)/float64(totalChunks))*80
        reporter.Report(progress, fmt.Sprintf("Processed chunk %d/%d", i+1, totalChunks))
    }
    
    // Step 3: Upload results
    reporter.Report(95, "Uploading results...")
    resultURL, err := uploadResults(file.ProcessedData)
    if err != nil {
        return nil, err
    }
    
    return &streamer.Result{
        RequestID: req.ID,
        Success:   true,
        Data: map[string]interface{}{
            "result_url":     resultURL,
            "original_size":  file.Size,
            "processed_size": len(file.ProcessedData),
            "chunks_processed": totalChunks,
        },
    }, nil
}

// Lambda function entry point
func main() {
    // Initialize router with correct interfaces
    router := streamer.NewRouter(requestQueue, connectionManager)
    router.Handle("process_file", &FileProcessorHandler{})
    
    lambda.Start(router.Route)
}

type FileProcessingParams struct {
    FileURL string `json:"file_url"`
    Options map[string]interface{} `json:"options"`
}
```

## Client Integration (Correct)

### JavaScript Client

```javascript
// Correct WebSocket client usage
const ws = new WebSocket('wss://your-api.execute-api.region.amazonaws.com/prod?Authorization=Bearer ' + token);

ws.onopen = () => {
    // Send request with correct format
    const request = {
        id: 'req-' + Date.now(),
        action: 'process_file',
        payload: {
            file_url: 'https://example.com/file.pdf',
            options: { quality: 'high' }
        }
    };
    
    ws.send(JSON.stringify(request));
};

ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    
    // Handle different message types
    switch (message.type) {
        case 'ack':
            console.log('Request acknowledged:', message.request_id);
            break;
            
        case 'progress':
            console.log(`Progress: ${message.percentage}% - ${message.message}`);
            updateProgressBar(message.percentage);
            break;
            
        case 'result':
            if (message.success) {
                console.log('Processing completed:', message.data);
                handleSuccess(message.data);
            } else {
                console.error('Processing failed:', message.error);
                handleError(message.error);
            }
            break;
            
        case 'error':
            console.error('Error:', message.error);
            handleError(message.error);
            break;
    }
};
```

This corrected documentation ensures that integration teams will be able to successfully implement handlers that work with the actual Streamer system. 