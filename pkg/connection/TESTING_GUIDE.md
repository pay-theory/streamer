# Connection Package Testing Guide

## Overview

The `connection` package provides premade mocks to make testing easier across the entire codebase. Since different packages define their own minimal `ConnectionManager` interfaces, we provide flexible mocks that work with any interface.

## Available Mocks

### 1. SendOnlyMock
**Best for**: Packages that only need the `Send` method (e.g., `pkg/streamer`)

```go
import "github.com/pay-theory/streamer/pkg/connection"

// Create mock
mock := connection.NewSendOnlyMock()

// Use with router
router := streamer.NewRouter(store, mock)

// Verify messages
messages := mock.GetMessages("conn-123")
assert.Len(t, messages, 1)
```

### 2. ProgressReporterMock
**Best for**: Packages that need `Send` + `IsActive` (e.g., `pkg/progress`)

```go
// Create mock
mock := connection.NewProgressReporterMock()

// Set up connections
mock.SetActive("conn-123", true)
mock.SetActive("conn-456", false)

// Use with progress reporter
reporter := progress.NewReporter("req-123", "conn-123", mock)

// Custom behavior
mock.IsActiveFunc = func(ctx context.Context, connID string) bool {
    return connID != "blocked"
}
```

### 3. RecordingMock
**Best for**: Tests that need to verify call sequences

```go
mock := connection.NewRecordingMock()

// Perform operations
mock.IsActive(ctx, "conn-1")
mock.Send(ctx, "conn-1", "message")

// Verify calls
calls := mock.GetIsActiveCalls()
assert.Equal(t, []string{"conn-1"}, calls)
```

### 4. FailingMock
**Best for**: Error handling tests

```go
// Always returns errors
mock := connection.NewFailingMock(errors.New("network error"))

// Test error handling
err := mock.Send(ctx, "any", "message")
assert.Error(t, err)
```

### 5. Full Mocks (in mocks.go)
**Best for**: Testing the actual connection package or needing all methods

```go
// Manual mock with function fields
mock := connection.NewMockConnectionManager()
mock.SendFunc = func(ctx context.Context, connID string, msg interface{}) error {
    // Custom logic
    return nil
}

// Testify-based mock
mock := new(connection.MockConnectionManagerTestify)
mock.On("Send", ctx, "conn-123", msg).Return(nil)
```

## Usage Patterns

### Pattern 1: Basic Message Verification
```go
func TestMessageSending(t *testing.T) {
    // Create mock
    mock := connection.NewSendOnlyMock()
    
    // Use in your code
    service := NewMyService(mock)
    service.NotifyUser("user-123", "Hello")
    
    // Verify
    messages := mock.GetMessages("user-123")
    assert.Len(t, messages, 1)
    assert.Equal(t, "Hello", messages[0])
}
```

### Pattern 2: Custom Send Behavior
```go
func TestConditionalSending(t *testing.T) {
    mock := connection.NewSendOnlyMock()
    
    // Configure behavior
    mock.SendFunc = func(ctx context.Context, connID string, msg interface{}) error {
        if connID == "banned" {
            return errors.New("user banned")
        }
        // Store message (default behavior)
        mock.Messages[connID] = append(mock.Messages[connID], msg)
        return nil
    }
    
    // Test banned user
    err := mock.Send(ctx, "banned", "test")
    assert.Error(t, err)
}
```

### Pattern 3: Connection State Testing
```go
func TestConnectionStates(t *testing.T) {
    mock := connection.NewProgressReporterMock()
    
    // Setup connections
    mock.SetActive("active-conn", true)
    mock.SetActive("inactive-conn", false)
    
    reporter := progress.NewReporter("req-123", "inactive-conn", mock)
    reporter.Report(50, "Progress") // Won't send to inactive connection
    
    // Verify no message sent
    assert.Empty(t, mock.GetMessages("inactive-conn"))
}
```

### Pattern 4: Parallel Testing
```go
func TestConcurrentAccess(t *testing.T) {
    mock := connection.NewSendOnlyMock()
    
    // All mocks are thread-safe
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            mock.Send(ctx, "conn", fmt.Sprintf("msg-%d", n))
        }(i)
    }
    wg.Wait()
    
    messages := mock.GetMessages("conn")
    assert.Len(t, messages, 100)
}
```

## Mock Selection Guide

| Package | Needs | Recommended Mock |
|---------|-------|------------------|
| `pkg/streamer` | Send only | `SendOnlyMock` |
| `pkg/progress` | Send + IsActive | `ProgressReporterMock` |
| `lambda/router` | Send only | `SendOnlyMock` |
| `lambda/processor` | Send + IsActive | `ProgressReporterMock` |
| Error testing | Failures | `FailingMock` |
| Complex scenarios | All methods | `MockConnectionManager` |

## Best Practices

1. **Use the simplest mock that fits**: Don't use `MockConnectionManager` if `SendOnlyMock` is sufficient

2. **Reset between tests**: 
   ```go
   mock.Reset() // Clears all messages
   ```

3. **Leverage default behavior**: Mocks store messages by default, override only when needed

4. **Thread-safe by default**: All mocks use mutexes for concurrent access

5. **Type assertions for verification**:
   ```go
   msg := mock.GetMessages("conn")[0].(map[string]interface{})
   assert.Equal(t, "progress", msg["type"])
   ```

## Migration from Custom Mocks

If you have existing custom mocks:

```go
// Old custom mock
type myMock struct {
    messages []interface{}
}
func (m *myMock) Send(ctx context.Context, connID string, msg interface{}) error {
    m.messages = append(m.messages, msg)
    return nil
}

// Replace with:
mock := connection.NewSendOnlyMock()
// Works the same way!
```

## Advanced Usage

### Combining with Testify
```go
type MySuite struct {
    suite.Suite
    connMock *connection.SendOnlyMock
}

func (s *MySuite) SetupTest() {
    s.connMock = connection.NewSendOnlyMock()
}

func (s *MySuite) TearDownTest() {
    s.connMock.Reset()
}
```

### Mock Chaining
```go
// Start with recording mock
mock := connection.NewRecordingMock()

// Later switch behavior
mock.IsActiveFunc = func(ctx context.Context, connID string) bool {
    return false // All connections inactive
}
```

## Troubleshooting

**Q: My test compiles but the mock doesn't capture messages**
A: Make sure you're not overriding `SendFunc` without storing messages:
```go
// Wrong
mock.SendFunc = func(...) error {
    return nil // Messages not stored!
}

// Right
mock.SendFunc = func(ctx context.Context, connID string, msg interface{}) error {
    // Custom logic...
    mock.Messages[connID] = append(mock.Messages[connID], msg)
    return nil
}
```

**Q: How do I test timeout scenarios?**
A: Use context cancellation:
```go
mock.SendFunc = func(ctx context.Context, connID string, msg interface{}) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(100 * time.Millisecond):
        return nil
    }
}
``` 