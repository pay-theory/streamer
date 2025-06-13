# AI Assistant 1: Storage Layer & Infrastructure Testing

## Context
You are tasked with improving unit test coverage for a Go-based WebSocket streaming service. The project uses AWS services (DynamoDB, API Gateway) and needs comprehensive unit tests for the storage and infrastructure layers.

## Project Overview
- **Language**: Go
- **Architecture**: WebSocket-based streaming service
- **Storage**: DynamoDB
- **Infrastructure**: AWS Lambda, API Gateway
- **Testing Framework**: Standard Go testing with testify

## Current Test Coverage

### ✅ Already Well-Tested
```
internal/store/dynamorm/*     - 84.0% coverage (DONE)
internal/protocol/*           - 98.6% coverage (DONE) 
internal/store/*              - 80.8% coverage (DONE)
```

### 🟡 Needs Improvement
```
pkg/connection/*              - 59.2% coverage (needs 20% more)
```

### ❌ Failing Tests to Fix
```
tests/integration/*           - Nil pointer issues with DynamoDB client
```

## Your Tasks

### 1. DynamoDB Storage Layer (`internal/store/dynamorm/`)
Create comprehensive unit tests for:
- **Connection Storage Operations**
  - `Save()`, `Get()`, `Delete()`, `Update()`
  - `ListByUser()`, `ListByTenant()`
  - `DeleteStale()` with time-based queries
  
- **Request Queue Operations**
  - Queue operations (enqueue, dequeue)
  - Status updates
  - Batch operations
  
- **Factory Methods**
  - Table initialization
  - Configuration validation
  
- **Error Handling**
  - Network failures
  - Conditional check failures
  - Validation errors

### 2. WebSocket Connection Management (`pkg/connection/`)
Add tests for:
- **API Gateway Integration**
  - Connection establishment
  - Message sending/receiving
  - Connection closure
  
- **Connection Manager**
  - Connection tracking
  - Concurrent access handling
  - Connection cleanup
  
- **Error Scenarios**
  - Network interruptions
  - Invalid connection IDs
  - Rate limiting

### 3. Protocol Layer (`internal/protocol/`)
Test:
- **Message Validation**
  - Schema validation
  - Required fields
  - Type checking
  
- **Protocol Compliance**
  - Message format verification
  - Version compatibility

## AWS Services Testing Guidelines

### ⚠️ Important: What NOT to Test

**Skip testing for these AWS services:**
- ❌ **CloudWatch Metrics** - Skip metric publishing calls
- ❌ **CloudWatch Logs** - Skip log streaming  
- ❌ **X-Ray Tracing** - Skip trace segments
- ❌ **IAM/STS** - Skip credential/role testing
- ❌ **Lambda Runtime** - Skip Lambda context/runtime

**Why?** These services don't provide concrete types and testing them adds no value to business logic verification.

### ✅ What TO Test

**Focus on these AWS interactions:**
- ✅ **DynamoDB Operations** - Core storage functionality
- ✅ **API Gateway Management** - WebSocket message sending (concrete types)
- ✅ **Business Logic** - Error handling, validation, data transformation

### Testing Pattern for AWS Services

```go
// DON'T test CloudWatch metrics
func (h *Handler) Handle(ctx context.Context) error {
    // Skip testing this
    h.metrics.PublishMetric("ConnectionCount", 1)
    
    // DO test this business logic
    if err := h.validateConnection(ctx); err != nil {
        return fmt.Errorf("invalid connection: %w", err)
    }
    
    // DO test DynamoDB operations
    return h.store.Save(ctx, connection)
}

// In tests, simply mock metrics to return nil
mockMetrics.On("PublishMetric", mock.Anything, mock.Anything).Return(nil)
```

## Testing Guidelines

### Use Table-Driven Tests
```go
func TestConnectionStore_Save(t *testing.T) {
    tests := []struct {
        name    string
        conn    *store.Connection
        setup   func(*mockDynamoDB)
        wantErr bool
        errType error
    }{
        // Test cases here
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Mock AWS SDK Calls
```go
type mockDynamoDB struct {
    mock.Mock
}

func (m *mockDynamoDB) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
    args := m.Called(ctx, params)
    return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}
```

### Test Both Success and Failure Scenarios
- Happy path testing
- Error conditions
- Edge cases (empty inputs, nil values)
- Concurrent access scenarios

### Coverage Targets
- **Minimum**: 80% coverage per package
- **Preferred**: 90%+ for critical paths
- **Focus on**: Branch coverage, not just line coverage

## File Structure Example
```
internal/store/dynamorm/
├── connection_store_test.go
├── request_queue_test.go
├── factory_test.go
├── models_test.go
└── test_helpers.go

pkg/connection/
├── api_gateway_test.go
├── manager_test.go
├── errors_test.go
└── mock_test.go
```

## Key Interfaces to Mock
```go
// DynamoDB Client
type DynamoDBClient interface {
    PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
    GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
    DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
    Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// API Gateway Management API
type APIGatewayClient interface {
    PostToConnection(context.Context, *apigatewaymanagementapi.PostToConnectionInput, ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error)
}
```

## Notes
- The project uses interfaces for storage operations, making mocking straightforward
- Focus on unit tests, not integration tests
- Tests should run without external dependencies
- Use `testify/mock` and `testify/assert` for cleaner test code
- Consider using `github.com/aws/aws-sdk-go-v2/service/dynamodb/types` for type construction

## Next Steps

### Priority 1: Fix Integration Tests (tests/integration/)
**Current**: Failing → **Target**: All passing

1. **Fix nil DynamoDB client issue**:
   ```go
   // The test is trying to use a nil DynamoDB client
   // Need to properly initialize the mock client
   mockClient := &mockDynamoDBClient{}
   store := &connectionStore{
       client: mockClient,
       // ... other fields
   }
   ```

2. **Common integration test pattern**:
   ```go
   func setupTestStore(t *testing.T) (*connectionStore, *mockDynamoDBClient) {
       mockClient := &mockDynamoDBClient{}
       store := &connectionStore{
           client:    mockClient,
           tableName: "test-table",
       }
       return store, mockClient
   }
   ```

### Priority 2: Complete Connection Management (pkg/connection/)
**Current**: 59.2% coverage → **Target**: 80%+

1. **Identify gaps in coverage**:
   ```bash
   go test -coverprofile=coverage.out ./pkg/connection/...
   go tool cover -html=coverage.out -o connection_coverage.html
   open connection_coverage.html
   ```

2. **Focus on untested areas**:
   - Error handling paths
   - Edge cases for connection state
   - Concurrent connection scenarios
   - Connection timeout handling
   - **Skip**: CloudWatch metrics, Lambda context

3. **Add missing test scenarios**:
   ```go
   // Test connection failures
   func TestConnection_SendMessage_Failed(t *testing.T) {
       // Test various failure modes
   }
   
   // Test concurrent access
   func TestConnection_ConcurrentOperations(t *testing.T) {
       // Test race conditions
   }
   ```

### Priority 3: Minor Improvements (if time permits)

1. **internal/store/dynamorm/** (84.0% → 90%):
   - Add edge case tests
   - Test error scenarios not yet covered

2. **internal/store/** (80.8% → 85%):
   - Cover any remaining untested branches

### Testing Time Savers

#### For Integration Tests
```go
// Simple mock that satisfies the interface
type mockDynamoDBClient struct {
    mock.Mock
}

// Implement only what you need
func (m *mockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
    args := m.Called(ctx, params)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}
```

#### Focus on What Matters
- ✅ Fix failing tests first
- ✅ Complete pkg/connection to 80%
- ✅ Integration test fixes
- ❌ Don't over-optimize already good coverage
- ❌ Skip AWS service mocking complexity

### Execution Timeline
- **Day 1**: Fix all integration test failures
- **Day 2**: Complete pkg/connection coverage to 80%
- **Optional**: Minor improvements to other packages

### Success Metrics
- All integration tests passing
- pkg/connection at 80%+ coverage
- No test failures
- Overall project coverage above 70% 