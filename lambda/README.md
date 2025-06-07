# Streamer Lambda Functions

This directory contains the AWS Lambda functions for the Streamer WebSocket system.

## Functions

### 1. Connect Handler (`connect/`)
Handles WebSocket connection establishment with JWT authentication.

**Responsibilities:**
- Validates JWT tokens from query parameters
- Extracts user and tenant information
- Creates connection records in DynamoDB
- Sets 24-hour TTL on connections

**Environment Variables:**
- `JWT_SECRET`: Secret for JWT validation (required)
- `TABLE_PREFIX`: DynamoDB table name prefix (default: "streamer_")

### 2. Disconnect Handler (`disconnect/`)
Handles WebSocket disconnection and cleanup.

**Responsibilities:**
- Removes connection records from DynamoDB
- Cleans up any active subscriptions
- Logs connection metrics for monitoring

**Environment Variables:**
- `TABLE_PREFIX`: DynamoDB table name prefix (default: "streamer_")

### 3. Router Handler (`router/`) - *Team 2 Implementation*
Routes incoming WebSocket messages to appropriate handlers.

### 4. Processor Handler (`processor/`) - *Team 2 Implementation*
Processes async requests from DynamoDB Streams.

## Deployment

### Prerequisites
- AWS SAM CLI installed
- Go 1.21+ installed
- AWS credentials configured
- S3 bucket for SAM deployments

### Quick Deploy
```bash
# Set environment variables
export SAM_BUCKET=your-sam-deployment-bucket
export JWT_SECRET=your-jwt-secret
export TABLE_PREFIX=streamer_

# Deploy with SAM
make deploy
```

### Guided Deploy (First Time)
```bash
make deploy-guided
```

### Build Only
```bash
make build
```

## Local Development

### Testing JWT Authentication
```go
// Generate a test JWT token
token, err := shared.GenerateJWT(
    "user-123",          // userID
    "tenant-456",        // tenantID
    []string{"read"},    // permissions
    "your-jwt-secret",   // secret
    24 * time.Hour,      // duration
)

// Connect with token
wscat -c "wss://your-api.execute-api.region.amazonaws.com/prod?Authorization=$token"
```

### Running Tests
```bash
make test
```

### Viewing Logs
```bash
# Connect function logs
make logs-connect

# Disconnect function logs
make logs-disconnect

# Router function logs
make logs-router
```

## Architecture

```
┌─────────────┐         ┌──────────────┐
│   Client    │────────▶│ API Gateway  │
└─────────────┘         └──────┬───────┘
                               │
                    ┌──────────┴──────────┐
                    │                     │
              ┌─────▼─────┐         ┌─────▼─────┐
              │ $connect  │         │ $default  │
              │  Lambda   │         │  Lambda   │
              └─────┬─────┘         └─────┬─────┘
                    │                     │
                    └─────────┬───────────┘
                              │
                        ┌─────▼─────┐
                        │ DynamoDB  │
                        └───────────┘
```

## Cold Start Optimization

The Lambda functions are optimized for cold starts:
- Minimal dependencies
- Connection pooling for DynamoDB
- Initialization in `init()` function
- Binary compilation with `GOOS=linux GOARCH=amd64`

## Security

### JWT Token Structure
```json
{
  "user_id": "user-123",
  "tenant_id": "tenant-456",
  "permissions": ["read", "write"],
  "exp": 1234567890,
  "iat": 1234567890
}
```

### IAM Permissions
Each Lambda function has minimal required permissions:
- DynamoDB access to specific tables
- CloudWatch Logs for logging
- X-Ray for tracing
- API Gateway Management API (for router)

## Monitoring

### CloudWatch Metrics
The functions emit custom metrics using CloudWatch Embedded Metric Format:
- `ConnectionEstablished`: New WebSocket connections
- `ConnectionDisconnected`: Closed connections
- Connection duration in seconds

### Structured Logging
All logs use structured JSON format:
```json
{
  "level": "INFO",
  "message": "Connection established",
  "request_id": "abc-123",
  "connection_id": "conn-456",
  "user_id": "user-789",
  "tenant_id": "tenant-012",
  "timestamp": 1234567890
}
```

## Troubleshooting

### Common Issues

1. **JWT Validation Fails**
   - Check JWT_SECRET environment variable
   - Verify token expiration
   - Ensure required claims (user_id, tenant_id) are present

2. **Connection Not Saved**
   - Check DynamoDB table exists
   - Verify IAM permissions
   - Check CloudWatch logs for errors

3. **High Cold Start Latency**
   - Consider increasing Lambda memory
   - Enable provisioned concurrency for predictable traffic
   - Review dependencies for optimization

## Integration with Team 2

Team 2 should:
1. Import the ConnectionManager from `pkg/connection`
2. Use it in the router Lambda to send responses
3. Use it in the processor Lambda for progress updates
4. Follow the WebSocket message format defined in the integration guide

## Next Steps

1. Complete router Lambda implementation (Team 2)
2. Add processor Lambda for async requests (Team 2)
3. Implement subscription management
4. Add request cancellation on disconnect
5. Performance testing with 1000+ connections 