# WebSocket API Reference

## Overview

The Streamer WebSocket API provides real-time bidirectional communication for async request processing. All messages are JSON-encoded.

## Connection

### Endpoint

```
wss://api.example.com/ws
```

### Authentication

Pass JWT token as query parameter:

```
wss://api.example.com/ws?Authorization=<JWT_TOKEN>
```

### JWT Claims Required

```json
{
  "sub": "user-id",
  "tenant_id": "tenant-id",
  "permissions": ["read", "write"],
  "exp": 1234567890
}
```

## Message Format

### Client → Server

All client messages must follow this structure:

```json
{
  "id": "unique-request-id",     // Optional, generated if not provided
  "action": "action-name",        // Required
  "payload": {                    // Optional, action-specific
    // Action-specific data
  },
  "metadata": {                   // Optional
    "key": "value"
  }
}
```

### Server → Client

Server messages will be one of these types:

#### Acknowledgment

```json
{
  "type": "acknowledgment",
  "request_id": "req_123",
  "status": "queued",
  "message": "Request queued for async processing"
}
```

#### Response (Sync)

```json
{
  "type": "response",
  "request_id": "req_123",
  "success": true,
  "data": {
    // Response data
  }
}
```

#### Progress Update

```json
{
  "type": "progress",
  "request_id": "req_123",
  "percentage": 45.5,
  "message": "Processing batch 2 of 4",
  "metadata": {
    "items_processed": 1250
  }
}
```

#### Completion

```json
{
  "type": "complete",
  "request_id": "req_123",
  "success": true,
  "result": {
    // Final result data
  }
}
```

#### Error

```json
{
  "type": "error",
  "request_id": "req_123",
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input parameters",
    "details": {
      // Additional error context
    }
  }
}
```

## Built-in Actions

### echo

Test action that returns the payload immediately.

**Request:**
```json
{
  "action": "echo",
  "payload": {
    "message": "Hello, World!"
  }
}
```

**Response:**
```json
{
  "type": "response",
  "request_id": "req_123",
  "success": true,
  "data": {
    "echo": {
      "message": "Hello, World!"
    },
    "timestamp": "2024-01-01T12:00:00Z"
  }
}
```

### generate_report (Async)

Generates a report with progress updates.

**Request:**
```json
{
  "action": "generate_report",
  "payload": {
    "start_date": "2024-01-01",
    "end_date": "2024-12-31",
    "format": "pdf",
    "include_charts": true
  }
}
```

**Flow:**
1. Acknowledgment (immediate)
2. Progress updates (multiple)
3. Completion with result

## Error Codes

| Code | Description |
|------|-------------|
| `VALIDATION_ERROR` | Invalid request parameters |
| `UNAUTHORIZED` | Authentication failed or insufficient permissions |
| `NOT_FOUND` | Requested resource not found |
| `INTERNAL_ERROR` | Server-side error |
| `TIMEOUT` | Request processing timeout |
| `RATE_LIMITED` | Too many requests |

## Rate Limiting

- **Connection Rate**: 10 connections per minute per IP
- **Message Rate**: 100 messages per minute per connection
- **Progress Updates**: Automatically throttled to 10 per second

## Best Practices

### Client Implementation

1. **Reconnection Logic**
   ```javascript
   class ReconnectingWebSocket {
     connect() {
       this.ws = new WebSocket(this.url);
       this.ws.onclose = () => {
         setTimeout(() => this.connect(), this.backoff);
         this.backoff = Math.min(this.backoff * 2, 30000);
       };
       this.ws.onopen = () => {
         this.backoff = 1000;
       };
     }
   }
   ```

2. **Request Tracking**
   ```javascript
   const pendingRequests = new Map();
   
   function sendRequest(action, payload) {
     const id = generateUUID();
     const promise = new Promise((resolve, reject) => {
       pendingRequests.set(id, { resolve, reject });
     });
     
     ws.send(JSON.stringify({ id, action, payload }));
     return promise;
   }
   ```

3. **Progress Handling**
   ```javascript
   ws.onmessage = (event) => {
     const message = JSON.parse(event.data);
     
     if (message.type === 'progress') {
       const handler = progressHandlers.get(message.request_id);
       if (handler) {
         handler(message.percentage, message.message);
       }
     }
   };
   ```

### Server Integration

1. **Custom Actions**
   ```go
   router.Handle("custom_action", &CustomHandler{
     EstimatedDuration: 30 * time.Second,
   })
   ```

2. **Progress Reporting**
   ```go
   func (h *Handler) ProcessWithProgress(ctx context.Context, req *Request, reporter ProgressReporter) error {
     reporter.Report(10, "Starting...")
     // Process...
     reporter.Report(100, "Complete!")
     return nil
   }
   ```

## Connection Lifecycle

### Connection Established
1. Client connects with JWT
2. Server validates token
3. Connection record created
4. Ready to receive messages

### During Connection
- Heartbeat every 30 seconds (automatic)
- Messages processed in order
- Progress updates delivered in real-time

### Connection Closed
1. Cleanup connection record
2. Cancel any pending subscriptions
3. Log metrics
4. No impact on async processing

## Examples

### JavaScript/TypeScript Client

```typescript
import { StreamerClient } from '@pay-theory/streamer-client';

const client = new StreamerClient('wss://api.example.com/ws', {
  token: 'your-jwt-token',
  reconnect: true,
  heartbeatInterval: 30000
});

// Sync request
const echoResult = await client.request('echo', { 
  message: 'Hello' 
});

// Async request with progress
const reportResult = await client.request('generate_report', {
  start_date: '2024-01-01',
  end_date: '2024-12-31'
}, {
  onProgress: (progress) => {
    console.log(`${progress.percentage}% - ${progress.message}`);
  }
});
```

### Python Client

```python
from streamer import StreamerClient

client = StreamerClient(
    url="wss://api.example.com/ws",
    token="your-jwt-token"
)

# Async request with progress callback
def on_progress(percentage, message):
    print(f"{percentage}% - {message}")

result = await client.request(
    action="generate_report",
    payload={
        "start_date": "2024-01-01",
        "end_date": "2024-12-31"
    },
    on_progress=on_progress
)
```

## Troubleshooting

### Connection Issues

1. **401 Unauthorized**: Check JWT token expiration and claims
2. **403 Forbidden**: Verify permissions in JWT
3. **Connection Dropped**: Check network and implement reconnection

### Message Issues

1. **No Response**: Verify action name and handler registration
2. **Timeout**: Check if request should be async (>5s)
3. **Missing Progress**: Ensure connection is active

### Performance Issues

1. **Slow Messages**: Check CloudWatch metrics for Lambda cold starts
2. **Delayed Progress**: Review DynamoDB Stream lag
3. **Connection Limits**: Monitor concurrent connection count 