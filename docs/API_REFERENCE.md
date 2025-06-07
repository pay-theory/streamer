# Streamer API Reference

## WebSocket Connection

### Connecting

**Endpoint:** `wss://{api-gateway-url}/production`

**Query Parameters:**
- `Authorization` (required): JWT token for authentication

**Example:**
```javascript
const ws = new WebSocket('wss://api.example.com/production?Authorization=eyJhbGc...');
```

**Connection Response:**
- Success: Connection established, no explicit message
- Failure: Connection closed with error code

### JWT Token Requirements

**Required Claims:**
```json
{
  "sub": "user-123",          // User ID (required)
  "tenant_id": "tenant-456",  // Tenant ID (required)
  "permissions": ["read", "write"], // Permissions array
  "exp": 1234567890,          // Expiration timestamp
  "iat": 1234567890,          // Issued at timestamp
  "iss": "your-issuer"        // Token issuer
}
```

## Message Format

### Request Message

All requests to the WebSocket API must follow this format:

```json
{
  "id": "unique-request-id",    // Client-generated request ID
  "action": "handler_name",     // Handler to invoke
  "payload": {                  // Handler-specific payload
    // ... handler parameters
  },
  "metadata": {                 // Optional request metadata
    "client_version": "1.0.0",
    "trace_id": "xyz"
  }
}
```

### Response Messages

The system sends different types of messages back to the client:

#### 1. Sync Response (Immediate)

```json
{
  "type": "response",
  "response": {
    "request_id": "unique-request-id",
    "status": "success",
    "data": {
      // Handler-specific response data
    },
    "metadata": {
      "processing_time": 0.234
    }
  }
}
```

#### 2. Queued Response (Async)

```json
{
  "type": "response",
  "response": {
    "request_id": "unique-request-id",
    "status": "queued",
    "message": "Request queued for processing",
    "estimated_duration": 120  // seconds
  }
}
```

#### 3. Progress Update

```json
{
  "type": "progress",
  "progress": {
    "request_id": "unique-request-id",
    "progress": 45.5,           // Percentage (0-100)
    "message": "Processing batch 3/7",
    "metadata": {
      "current_step": "data_processing",
      "records_processed": 15000,
      "records_total": 30000
    },
    "timestamp": "2024-01-20T10:30:45Z"
  }
}
```

#### 4. Final Result (Async)

```json
{
  "type": "result",
  "result": {
    "request_id": "unique-request-id",
    "success": true,
    "data": {
      // Handler-specific result data
    },
    "metadata": {
      "total_processing_time": 45.678,
      "handler": "generate_report"
    }
  }
}
```

#### 5. Error Response

```json
{
  "type": "error",
  "error": {
    "request_id": "unique-request-id",
    "code": "VALIDATION_ERROR",
    "message": "Invalid date format",
    "details": {
      "field": "start_date",
      "expected": "YYYY-MM-DD",
      "received": "01/20/2024"
    }
  }
}
```

## Built-in Handlers

### 1. Generate Report

**Action:** `generate_report`

**Description:** Generates various types of reports with progress tracking.

**Estimated Duration:** 2 minutes

**Request Payload:**
```json
{
  "start_date": "2024-01-01",    // Required, format: YYYY-MM-DD
  "end_date": "2024-01-31",      // Required, format: YYYY-MM-DD
  "format": "pdf",               // Required: pdf, csv, excel
  "report_type": "monthly",      // Required: monthly, quarterly, annual, custom
  "include_charts": true,        // Optional, default: false
  "filters": {                   // Optional filters
    "category": "electronics",
    "region": "north-america"
  }
}
```

**Success Response:**
```json
{
  "url": "https://reports.s3.amazonaws.com/report-xyz.pdf",
  "records": 12500,
  "size_bytes": 2457600,
  "format": "pdf",
  "generated_at": "2024-01-20T10:45:00Z",
  "expires_at": "2024-01-27T10:45:00Z",
  "stats": {
    "total_value": 1234567.89,
    "categories_count": 5
  }
}
```

**Progress Updates:**
- 0-30%: Querying data from multiple sources
- 30-60%: Processing and aggregating data
- 60-90%: Generating report file
- 90-100%: Uploading and finalizing

### 2. Process Data

**Action:** `process_data`

**Description:** Runs ML pipelines on data with multiple processing stages.

**Estimated Duration:** 5 minutes

**Request Payload:**
```json
{
  "pipeline": "classification",   // Required: classification, regression, clustering, anomaly
  "data_source": {               // Required
    "type": "file",              // file, query, stream
    "path": "/data/input.csv",   // For file type
    "query": "SELECT ...",       // For query type
    "stream_id": "stream-123"    // For stream type
  },
  "output": {                    // Required
    "format": "json",            // json, csv, parquet
    "destination": "s3://bucket/path/"  // Optional
  },
  "options": {                   // Optional pipeline-specific options
    "confidence_threshold": 0.8,
    "max_iterations": 100
  }
}
```

**Success Response:**
```json
{
  "summary": {
    "total_records": 50000,
    "processing_time": 287.5,
    "model_version": "v2.3.1",
    "pipeline": "classification",
    "metrics": {
      "accuracy": 0.945,
      "precision": 0.932,
      "recall": 0.928,
      "f1_score": 0.930
    }
  },
  "data": [
    // Sample of results (when format is json)
  ],
  "output_location": "s3://bucket/path/results.json"
}
```

**Progress Updates:**
- 0-20%: Data ingestion
- 20-40%: Data preprocessing
- 40-60%: Feature engineering
- 60-85%: Model processing
- 85-100%: Post-processing and output

### 3. Health Check

**Action:** `health`

**Description:** Simple health check endpoint for monitoring.

**Estimated Duration:** 10ms (always synchronous)

**Request Payload:** None required

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-20T10:30:00Z",
  "version": "1.2.3"
}
```

## Error Codes

| Code | Description | Retry |
|------|-------------|-------|
| `VALIDATION_ERROR` | Request payload validation failed | No |
| `NOT_FOUND` | Requested resource not found | No |
| `UNAUTHORIZED` | Authentication or permission issue | No |
| `INTERNAL_ERROR` | Server-side processing error | Yes |
| `TIMEOUT` | Request processing timeout | Yes |
| `RATE_LIMITED` | Too many requests | Yes (with backoff) |
| `INVALID_ACTION` | Unknown handler action | No |

## Rate Limits

- **Connection Rate:** 100 connections per minute per IP
- **Message Rate:** 100 messages per second per connection
- **Concurrent Requests:** 10 per connection
- **Payload Size:** 1MB maximum

## Best Practices

### 1. Request ID Generation

Always use unique request IDs:
```javascript
const requestId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
```

### 2. Progress Monitoring

Subscribe to progress updates for async operations:
```javascript
ws.on('message', (data) => {
  const msg = JSON.parse(data);
  if (msg.type === 'progress' && msg.progress.request_id === myRequestId) {
    updateProgressBar(msg.progress.progress);
    updateStatusText(msg.progress.message);
  }
});
```

### 3. Error Handling

Implement comprehensive error handling:
```javascript
ws.on('message', (data) => {
  const msg = JSON.parse(data);
  if (msg.type === 'error') {
    if (msg.error.code === 'VALIDATION_ERROR') {
      // Show validation error to user
    } else if (msg.error.code === 'TIMEOUT') {
      // Retry with exponential backoff
    }
  }
});
```

### 4. Connection Management

Implement reconnection logic:
```javascript
let reconnectAttempts = 0;

ws.on('close', () => {
  if (reconnectAttempts < 5) {
    setTimeout(() => {
      reconnectAttempts++;
      connectWebSocket();
    }, Math.pow(2, reconnectAttempts) * 1000);
  }
});
```

## SDK Examples

### JavaScript/TypeScript

```typescript
import { StreamerClient } from '@pay-theory/streamer-client';

const client = new StreamerClient({
  endpoint: 'wss://api.example.com/production',
  token: 'your-jwt-token',
  onProgress: (progress) => {
    console.log(`${progress.progress}% - ${progress.message}`);
  }
});

// Sync request
const healthResult = await client.request('health', {});

// Async request with progress
const reportResult = await client.request('generate_report', {
  start_date: '2024-01-01',
  end_date: '2024-01-31',
  format: 'pdf',
  report_type: 'monthly'
});
```

### Python

```python
from streamer_client import StreamerClient

client = StreamerClient(
    endpoint='wss://api.example.com/production',
    token='your-jwt-token'
)

# Async request with progress callback
def on_progress(progress):
    print(f"{progress['progress']}% - {progress['message']}")

result = await client.request(
    action='process_data',
    payload={
        'pipeline': 'classification',
        'data_source': {'type': 'file', 'path': '/data/input.csv'},
        'output': {'format': 'json'}
    },
    on_progress=on_progress
)
```

## Troubleshooting

### Connection Issues

**Problem:** Connection immediately closes
- Check JWT token is valid and not expired
- Verify token contains required claims
- Ensure WebSocket endpoint URL is correct

**Problem:** Connection drops frequently
- Implement heartbeat/ping messages
- Check for network timeouts
- Monitor CloudWatch logs for errors

### Processing Issues

**Problem:** Requests always timeout
- Check handler estimated duration
- Verify Lambda timeout settings
- Monitor DynamoDB throttling

**Problem:** No progress updates received
- Ensure connection is still active
- Check subscription to request
- Verify DynamoDB Streams is enabled

### Performance Issues

**Problem:** Slow response times
- Monitor Lambda cold starts
- Check DynamoDB read/write capacity
- Analyze handler processing logic

## Changelog

### Version 1.2.0 (Current)
- Added progress batching for efficiency
- Improved retry logic with exponential backoff
- Enhanced validation error messages

### Version 1.1.0
- Added ML pipeline support
- Implemented metadata in progress updates
- Added connection TTL

### Version 1.0.0
- Initial release
- Basic async/sync routing
- Progress reporting
- Report generation handler 