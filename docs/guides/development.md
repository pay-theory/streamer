# Development Guide

## Prerequisites

- Go 1.21 or higher
- AWS CLI configured
- Docker (for local DynamoDB)
- Make

## Local Development Setup

### 1. Clone Repository

```bash
git clone https://github.com/pay-theory/streamer.git
cd streamer
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Start Local DynamoDB

```bash
docker run -p 8000:8000 amazon/dynamodb-local
```

### 4. Create Local Tables

```bash
make local-tables
```

### 5. Set Environment Variables

Create `.env.local`:

```bash
# DynamoDB
DYNAMODB_ENDPOINT=http://localhost:8000
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=local
AWS_SECRET_ACCESS_KEY=local

# Tables
CONNECTIONS_TABLE=streamer-connections-local
REQUESTS_TABLE=streamer-requests-local

# JWT (for testing)
JWT_PUBLIC_KEY="-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----"

# WebSocket
WEBSOCKET_ENDPOINT=ws://localhost:8080
```

### 6. Run Tests

```bash
make test
```

## Project Structure

```
streamer/
├── pkg/                      # Public packages
│   ├── streamer/            # Core interfaces
│   │   ├── router.go        # Router implementation
│   │   ├── handler.go       # Handler interfaces
│   │   └── types.go         # Common types
│   ├── connection/          # Connection management
│   │   ├── manager.go       # Connection manager
│   │   ├── store.go         # DynamoDB storage
│   │   └── types.go         # Connection types
│   └── progress/            # Progress reporting
│       ├── reporter.go      # Progress reporter
│       └── types.go         # Progress types
├── internal/                # Private packages
│   ├── store/              # Storage layer
│   │   ├── connection.go   # Connection model
│   │   ├── request.go      # Request model
│   │   └── dynamodb.go     # DynamoDB client
│   └── auth/               # Authentication
│       ├── jwt.go          # JWT validation
│       └── claims.go       # JWT claims
├── lambda/                  # Lambda functions
│   ├── connect/            # $connect handler
│   ├── disconnect/         # $disconnect handler
│   ├── router/             # Message router
│   └── processor/          # Async processor
├── examples/               # Example implementations
├── tests/                  # Integration tests
└── deployment/             # Infrastructure code
```

## Adding a New Handler

### 1. Define Handler Type

```go
// handlers/report.go
package handlers

import (
    "context"
    "time"
    
    "github.com/pay-theory/streamer/pkg/streamer"
    "github.com/pay-theory/streamer/pkg/progress"
    "github.com/pay-theory/streamer/internal/store"
)

type ReportHandler struct {
    reportService ReportService
}

// Implement Handler interface
func (h *ReportHandler) Action() string {
    return "generate_report"
}

func (h *ReportHandler) EstimatedDuration() time.Duration {
    return 2 * time.Minute // Will be processed async
}
```

### 2. Implement Sync Processing (< 5s)

```go
func (h *ReportHandler) Process(
    ctx context.Context,
    req *streamer.Request,
) (*streamer.Response, error) {
    // For quick operations
    params := &ReportParams{}
    if err := req.UnmarshalPayload(params); err != nil {
        return nil, err
    }
    
    // Quick validation or cached response
    if cached := h.checkCache(params); cached != nil {
        return &streamer.Response{
            Success: true,
            Data: map[string]interface{}{
                "cached": true,
                "url": cached.URL,
            },
        }, nil
    }
    
    // If not cached, it will fall through to async
    return nil, streamer.ErrAsyncRequired
}
```

### 3. Implement Async Processing (> 5s)

```go
func (h *ReportHandler) ProcessWithProgress(
    ctx context.Context,
    req *store.AsyncRequest,
    reporter progress.Reporter,
) error {
    params := &ReportParams{}
    if err := json.Unmarshal(req.Payload, params); err != nil {
        return err
    }
    
    // Report progress throughout
    reporter.Report(10, "Validating parameters...")
    if err := h.validate(params); err != nil {
        return err
    }
    
    reporter.Report(20, "Querying data...")
    data, err := h.queryData(ctx, params)
    if err != nil {
        return err
    }
    
    reporter.Report(50, "Generating report...")
    report, err := h.generateReport(ctx, data)
    if err != nil {
        return err
    }
    
    reporter.Report(80, "Uploading to S3...")
    url, err := h.uploadReport(ctx, report)
    if err != nil {
        return err
    }
    
    reporter.Report(100, "Complete!")
    
    // Set final result
    return reporter.Complete(map[string]interface{}{
        "url": url,
        "size": report.Size,
        "pages": report.Pages,
    })
}
```

### 4. Register Handler

```go
// lambda/router/main.go
func init() {
    router := streamer.NewRouter(
        streamer.WithDynamoDB(db),
        streamer.WithHandlers(
            &handlers.EchoHandler{},
            &handlers.ReportHandler{
                reportService: services.NewReportService(),
            },
            // Add more handlers here
        ),
    )
}
```

## Testing

### Unit Tests

```go
// handlers/report_test.go
func TestReportHandler_Process(t *testing.T) {
    handler := &ReportHandler{
        reportService: &mockReportService{},
    }
    
    req := &streamer.Request{
        Action: "generate_report",
        Payload: map[string]interface{}{
            "start_date": "2024-01-01",
            "end_date": "2024-12-31",
        },
    }
    
    resp, err := handler.Process(context.Background(), req)
    assert.NoError(t, err)
    assert.True(t, resp.Success)
}
```

### Integration Tests

```go
// tests/integration/report_test.go
func TestReportGeneration_EndToEnd(t *testing.T) {
    // Start local environment
    env := tests.NewLocalEnvironment(t)
    defer env.Cleanup()
    
    // Create WebSocket client
    client := env.NewClient("test-user")
    defer client.Close()
    
    // Send request
    result, err := client.Request("generate_report", map[string]interface{}{
        "start_date": "2024-01-01",
        "end_date": "2024-12-31",
    }, &streamer.RequestOptions{
        OnProgress: func(p progress.Update) {
            t.Logf("Progress: %d%% - %s", p.Percentage, p.Message)
        },
    })
    
    assert.NoError(t, err)
    assert.NotEmpty(t, result["url"])
}
```

### Load Tests

```go
// tests/load/concurrent_test.go
func TestConcurrentRequests(t *testing.T) {
    env := tests.NewLocalEnvironment(t)
    defer env.Cleanup()
    
    const numClients = 100
    const requestsPerClient = 10
    
    var wg sync.WaitGroup
    errors := make(chan error, numClients*requestsPerClient)
    
    for i := 0; i < numClients; i++ {
        wg.Add(1)
        go func(clientID int) {
            defer wg.Done()
            
            client := env.NewClient(fmt.Sprintf("user-%d", clientID))
            defer client.Close()
            
            for j := 0; j < requestsPerClient; j++ {
                _, err := client.Request("echo", map[string]interface{}{
                    "message": fmt.Sprintf("msg-%d-%d", clientID, j),
                })
                if err != nil {
                    errors <- err
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    var errorCount int
    for err := range errors {
        t.Errorf("Request failed: %v", err)
        errorCount++
    }
    
    assert.Equal(t, 0, errorCount)
}
```

## Local Development Tools

### Mock WebSocket Server

```go
// cmd/mock-server/main.go
package main

import (
    "github.com/pay-theory/streamer/tests/mock"
)

func main() {
    server := mock.NewWebSocketServer(mock.Config{
        Port: 8080,
        Handlers: map[string]mock.Handler{
            "echo": mock.EchoHandler,
            "delay": mock.DelayHandler(5 * time.Second),
        },
    })
    
    log.Fatal(server.Start())
}
```

### CLI Client

```bash
# Install CLI
go install ./cmd/streamer-cli

# Send request
streamer-cli request \
  --url ws://localhost:8080 \
  --action echo \
  --payload '{"message":"Hello"}'

# With progress tracking
streamer-cli request \
  --url ws://localhost:8080 \
  --action generate_report \
  --payload '{"start_date":"2024-01-01"}' \
  --follow
```

## Debugging

### Enable Debug Logging

```go
// Set environment variable
os.Setenv("LOG_LEVEL", "DEBUG")

// Or in code
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
```

### Trace Requests

```go
// Add trace ID to context
ctx = context.WithValue(ctx, "trace_id", uuid.New().String())

// Log with trace ID
logger.With("trace_id", ctx.Value("trace_id")).Info("Processing request")
```

### Local X-Ray

```bash
# Run X-Ray daemon locally
docker run -p 2000:2000 amazon/aws-xray-daemon -o

# Set environment variable
export AWS_XRAY_DAEMON_ADDRESS=127.0.0.1:2000
```

## Performance Profiling

### CPU Profiling

```go
import _ "net/http/pprof"

func init() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}

// Profile: go tool pprof http://localhost:6060/debug/pprof/profile
```

### Memory Profiling

```go
// Capture heap profile
runtime.GC()
f, _ := os.Create("mem.prof")
defer f.Close()
runtime.pprof.WriteHeapProfile(f)

// Analyze: go tool pprof mem.prof
```

### Benchmarks

```go
// handlers/report_bench_test.go
func BenchmarkReportHandler_Process(b *testing.B) {
    handler := &ReportHandler{}
    req := &streamer.Request{
        Action: "generate_report",
        Payload: testPayload,
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = handler.Process(context.Background(), req)
    }
}

// Run: go test -bench=. -benchmem
```

## Code Style Guide

### General Guidelines

1. Follow standard Go conventions
2. Use meaningful variable names
3. Keep functions focused and small
4. Add comments for exported types and functions
5. Handle errors explicitly

### Error Handling

```go
// Define custom errors
var (
    ErrInvalidParams = errors.New("invalid parameters")
    ErrNotFound = errors.New("resource not found")
)

// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to generate report: %w", err)
}

// Check error types
if errors.Is(err, ErrNotFound) {
    return nil, streamer.NewError(404, "Report not found")
}
```

### Testing Patterns

```go
// Table-driven tests
func TestValidateParams(t *testing.T) {
    tests := []struct {
        name    string
        params  ReportParams
        wantErr bool
    }{
        {
            name: "valid params",
            params: ReportParams{
                StartDate: "2024-01-01",
                EndDate: "2024-12-31",
            },
            wantErr: false,
        },
        {
            name: "invalid date range",
            params: ReportParams{
                StartDate: "2024-12-31",
                EndDate: "2024-01-01",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateParams(tt.params)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateParams() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Contributing

### Workflow

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests first (TDD)
4. Implement feature
5. Run tests (`make test`)
6. Commit changes (`git commit -m 'Add amazing feature'`)
7. Push branch (`git push origin feature/amazing-feature`)
8. Open Pull Request

### PR Checklist

- [ ] Tests pass locally
- [ ] Code follows style guide
- [ ] Documentation updated
- [ ] Changelog entry added
- [ ] No security vulnerabilities
- [ ] Performance impact considered

## Resources

- [Go Documentation](https://go.dev/doc/)
- [AWS Lambda Best Practices](https://docs.aws.amazon.com/lambda/latest/dg/best-practices.html)
- [DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [WebSocket API Documentation](https://docs.aws.amazon.com/apigateway/latest/developerguide/websocket-api.html) 