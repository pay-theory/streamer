# Streamer Quick Reference

## 🚀 Quick Start

```bash
# Clone and build
git clone https://github.com/pay-theory/streamer.git
cd streamer
make build

# Run tests
make test

# Deploy to AWS
cd deployment/pulumi
pulumi up -s production
```

## 📁 Key Files

```
streamer/
├── pkg/streamer/        # Public API
├── lambda/              # Lambda functions
│   ├── connect/         # WebSocket $connect
│   ├── disconnect/      # WebSocket $disconnect
│   ├── router/          # Request router
│   └── processor/       # Async processor
├── internal/store/      # DynamoDB models
└── deployment/pulumi/   # Infrastructure as Code
```

## 🔧 Common Commands

### Development

```bash
# Run local DynamoDB
docker run -p 8000:8000 amazon/dynamodb-local

# Create local tables
make local-tables

# Run specific tests
go test ./pkg/streamer/... -v

# Build Lambda functions
make lambda-build

# Format code
go fmt ./...

# Lint code
golangci-lint run
```

### Deployment

```bash
# Deploy infrastructure
cd deployment/pulumi
pulumi up -s production

# View Lambda logs
aws logs tail /aws/lambda/streamer-router-prod --follow

# Check DynamoDB table
aws dynamodb scan --table-name streamer-connections-prod
```

## 🌐 API Usage

### WebSocket Connection

```javascript
// Connect with JWT
const ws = new WebSocket('wss://api.example.com/ws?Authorization=JWT_TOKEN');

// Send request
ws.send(JSON.stringify({
  action: 'generate_report',
  payload: { start_date: '2024-01-01' }
}));

// Handle responses
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch(msg.type) {
    case 'progress':
      console.log(`${msg.percentage}% - ${msg.message}`);
      break;
    case 'complete':
      console.log('Result:', msg.result);
      break;
  }
};
```

### Handler Implementation

```go
type MyHandler struct{}

func (h *MyHandler) Action() string {
    return "my_action"
}

func (h *MyHandler) EstimatedDuration() time.Duration {
    return 30 * time.Second // Will be async
}

func (h *MyHandler) ProcessWithProgress(
    ctx context.Context,
    req *store.AsyncRequest,
    reporter progress.Reporter,
) error {
    reporter.Report(50, "Halfway done!")
    // Your logic here
    return reporter.Complete(result)
}
```

## 🔍 Debugging

### Check Lambda Logs

```bash
# Router logs
aws logs tail /aws/lambda/streamer-router-prod --follow

# Processor logs  
aws logs tail /aws/lambda/streamer-processor-prod --follow

# Filter by request ID
aws logs filter-log-events \
  --log-group-name /aws/lambda/streamer-router-prod \
  --filter-pattern '{ $.request_id = "req_123" }'
```

### Query DynamoDB

```bash
# Get connection
aws dynamodb get-item \
  --table-name streamer-connections-prod \
  --key '{"ConnectionID": {"S": "conn_123"}}'

# List user connections
aws dynamodb query \
  --table-name streamer-connections-prod \
  --index-name UserIndex \
  --key-condition-expression "UserID = :uid" \
  --expression-attribute-values '{":uid":{"S":"user_123"}}'
```

### Test WebSocket

```bash
# Install wscat
npm install -g wscat

# Connect
wscat -c wss://api.example.com/ws?Authorization=TOKEN

# Send message
> {"action":"echo","payload":{"message":"Hello"}}
```

## 📊 Monitoring

### Key Metrics

```bash
# Lambda invocations
aws cloudwatch get-metric-statistics \
  --namespace AWS/Lambda \
  --metric-name Invocations \
  --dimensions Name=FunctionName,Value=streamer-router-prod \
  --start-time 2024-01-01T00:00:00Z \
  --end-time 2024-01-02T00:00:00Z \
  --period 3600 \
  --statistics Sum

# WebSocket connections
aws cloudwatch get-metric-statistics \
  --namespace AWS/ApiGateway \
  --metric-name ConnectionCount \
  --dimensions Name=ApiName,Value=streamer-prod \
  --start-time 2024-01-01T00:00:00Z \
  --end-time 2024-01-02T00:00:00Z \
  --period 300 \
  --statistics Maximum
```

### X-Ray Traces

```bash
# Get recent traces
aws xray get-trace-summaries \
  --time-range-type LastHour \
  --query 'TraceSummaries[?ServiceNames[?contains(@, `streamer`)]]'

# Get trace details
aws xray get-traces --trace-ids trace-id-here
```

## 🛠️ Troubleshooting

| Issue | Solution |
|-------|----------|
| WebSocket connection fails | Check JWT token and API Gateway logs |
| No progress updates | Verify connection is active and DynamoDB Streams enabled |
| Lambda timeout | Increase timeout or optimize handler code |
| DynamoDB throttling | Switch to on-demand or increase capacity |
| High Lambda cold starts | Enable provisioned concurrency |

## 📚 Documentation

- [Architecture Guide](docs/ARCHITECTURE.md)
- [API Reference](docs/api/websocket-api.md)  
- [Deployment Guide](docs/deployment/README.md)
- [Development Guide](docs/guides/development.md)


[Read the full story →](docs/development/achievement/STREAMER_100_PERCENT_COMPLETE.md)