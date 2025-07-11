# Streamer Testing Guide for Migrant Team

## 🎯 Overview

This guide shows the migrant team how to leverage Streamer's comprehensive mocking infrastructure for testing WebSocket and async processing functionality without AWS dependencies.

## 📦 Available Mocks

Streamer provides **5 different mock types** for different testing scenarios:

### 1. **SendOnlyMock** - For Basic Message Testing
```go
import "github.com/pay-theory/streamer/pkg/connection"

// Perfect for testing handlers that only send messages
mock := connection.NewSendOnlyMock()

// Use with your handler
handler := NewMigrantHandler(mock)
handler.SendNotification("user-123", "Migration complete")

// Verify messages
messages := mock.GetMessages("user-123")
assert.Len(t, messages, 1)
assert.Equal(t, "Migration complete", messages[0])
```

### 2. **ProgressReporterMock** - For Progress Tracking
```go
// Perfect for async operations with progress updates
mock := connection.NewProgressReporterMock()

// Set connection states
mock.SetActive("active-user", true)
mock.SetActive("disconnected-user", false)

// Use with progress reporter
reporter := progress.NewReporter("migration-123", "active-user", mock)
reporter.Report(50, "Migrating data...")

// Verify progress was sent only to active connections
messages := mock.GetMessages("active-user")
assert.Len(t, messages, 1)
assert.Empty(t, mock.GetMessages("disconnected-user"))
```

### 3. **TestableAPIGatewayClient** - For Complex Scenarios
```go
// Perfect for testing connection management and error scenarios
client := connection.NewTestableAPIGatewayClient()

// Add connections
client.AddConnection("conn-123", "192.168.1.1")
client.AddConnection("conn-456", "192.168.1.2")

// Simulate errors
client.SimulateGoneError("conn-gone")
client.SimulateThrottling("conn-throttled", 5)

// Use with ConnectionManager
manager := connection.NewManager(mockStore, client, "wss://test.com")

// Test error handling
err := manager.Send(ctx, "conn-gone", "test")
assert.True(t, errors.Is(err, connection.ErrConnectionStale))
```

### 4. **RecordingMock** - For Call Sequence Verification
```go
// Perfect for testing interaction patterns
mock := connection.NewRecordingMock()

// Perform operations
mock.IsActive(ctx, "user-1")
mock.Send(ctx, "user-1", "message")
mock.IsActive(ctx, "user-2")

// Verify call sequence
calls := mock.GetIsActiveCalls()
assert.Equal(t, []string{"user-1", "user-2"}, calls)
```

### 5. **FailingMock** - For Error Testing
```go
// Perfect for testing error handling
mock := connection.NewFailingMock(errors.New("network timeout"))

// Test error resilience
handler := NewMigrantHandler(mock)
err := handler.ProcessMigration("user-123")
assert.Error(t, err)
assert.Contains(t, err.Error(), "network timeout")
```

## 🚀 Real-World Testing Patterns for Migrant

### Pattern 1: Testing Migration Progress Updates

```go
func TestMigrationProgressUpdates(t *testing.T) {
    // Setup
    mock := connection.NewProgressReporterMock()
    mock.SetActive("user-123", true)
    
    migrator := NewDataMigrator(mock)
    
    // Execute migration
    err := migrator.MigrateUserData("user-123", MigrationRequest{
        TableName: "user_profiles",
        BatchSize: 100,
    })
    
    assert.NoError(t, err)
    
    // Verify progress updates were sent
    messages := mock.GetMessages("user-123")
    assert.GreaterOrEqual(t, len(messages), 3) // Start, progress, complete
    
    // Verify message structure
    firstMsg := messages[0].(map[string]interface{})
    assert.Equal(t, "progress", firstMsg["type"])
    assert.Equal(t, "user-123", firstMsg["migration_id"])
}
```

### Pattern 2: Testing Batch Migration with Multiple Users

```go
func TestBatchMigration(t *testing.T) {
    mock := connection.NewSendOnlyMock()
    
    // Setup multiple users
    userIDs := []string{"user-1", "user-2", "user-3"}
    migrator := NewBatchMigrator(mock)
    
    // Execute batch migration
    results := migrator.MigrateBatch(userIDs, BatchConfig{
        MaxConcurrent: 2,
        BatchSize:     50,
    })
    
    // Verify all users received notifications
    for _, userID := range userIDs {
        messages := mock.GetMessages(userID)
        assert.GreaterOrEqual(t, len(messages), 1)
        
        // Check completion message
        lastMsg := messages[len(messages)-1].(map[string]interface{})
        assert.Equal(t, "migration_complete", lastMsg["type"])
    }
    
    // Verify success rates
    assert.Equal(t, 3, len(results.Successful))
    assert.Empty(t, results.Failed)
}
```

### Pattern 3: Testing Error Recovery

```go
func TestMigrationErrorRecovery(t *testing.T) {
    // Create mock that fails intermittently
    mock := connection.NewSendOnlyMock()
    callCount := 0
    
    mock.SendFunc = func(ctx context.Context, connID string, msg interface{}) error {
        callCount++
        if callCount <= 2 {
            return errors.New("temporary network error")
        }
        // Store message on success
        mock.Messages[connID] = append(mock.Messages[connID], msg)
        return nil
    }
    
    migrator := NewResilientMigrator(mock, RetryConfig{
        MaxRetries: 3,
        BackoffMs:  100,
    })
    
    // Should succeed after retries
    err := migrator.MigrateWithRetry("user-123", migrationData)
    assert.NoError(t, err)
    
    // Verify message was eventually sent
    messages := mock.GetMessages("user-123")
    assert.Len(t, messages, 1)
}
```

### Pattern 4: Testing WebSocket Connection States

```go
func TestConnectionStateHandling(t *testing.T) {
    client := connection.NewTestableAPIGatewayClient()
    
    // Add various connection states
    client.AddConnection("active-user", "192.168.1.1")
    client.SimulateGoneError("disconnected-user")
    client.SimulateThrottling("throttled-user", 3)
    
    manager := connection.NewManager(mockStore, client, "wss://test.com")
    migrator := NewWebSocketMigrator(manager)
    
    // Test different scenarios
    tests := []struct {
        userID          string
        expectedSuccess bool
        expectedError   error
    }{
        {"active-user", true, nil},
        {"disconnected-user", false, connection.ErrConnectionStale},
        {"throttled-user", false, connection.ErrThrottled},
    }
    
    for _, tt := range tests {
        t.Run(tt.userID, func(t *testing.T) {
            err := migrator.NotifyMigrationStatus(tt.userID, "in_progress")
            
            if tt.expectedSuccess {
                assert.NoError(t, err)
                messages := client.GetMessages(tt.userID)
                assert.Len(t, messages, 1)
            } else {
                assert.Error(t, err)
                assert.True(t, errors.Is(err, tt.expectedError))
            }
        })
    }
}
```

### Pattern 5: Testing Async Request Processing

```go
func TestAsyncMigrationProcessing(t *testing.T) {
    mock := connection.NewProgressReporterMock()
    mock.SetActive("user-123", true)
    
    // Setup async processor
    processor := NewAsyncMigrationProcessor(mock)
    
    // Create migration request
    req := &AsyncMigrationRequest{
        RequestID:    "req-456",
        ConnectionID: "user-123",
        MigrationType: "profile_data",
        Payload: map[string]interface{}{
            "source_table": "legacy_profiles",
            "target_table": "user_profiles",
        },
    }
    
    // Process asynchronously
    result, err := processor.ProcessMigration(ctx, req)
    assert.NoError(t, err)
    assert.True(t, result.Success)
    
    // Verify progress updates
    messages := mock.GetMessages("user-123")
    progressMessages := filterMessagesByType(messages, "progress")
    assert.GreaterOrEqual(t, len(progressMessages), 3)
    
    // Verify completion message
    completionMessages := filterMessagesByType(messages, "complete")
    assert.Len(t, completionMessages, 1)
    
    completion := completionMessages[0].(map[string]interface{})
    assert.Equal(t, "req-456", completion["request_id"])
}
```

## 🔧 Advanced Testing Scenarios

### Testing with Lift Integration

```go
func TestLiftStreamerIntegration(t *testing.T) {
    // Setup Streamer mocks
    mock := connection.NewSendOnlyMock()
    
    // Create Lift app with Streamer handlers
    app := lift.New(lift.WithWebSocketSupport())
    
    // Add middleware and handlers
    migrationHandler := NewLiftMigrationHandler(mock)
    app.WebSocket("migration.start", migrationHandler.StartMigration)
    app.WebSocket("migration.status", migrationHandler.GetStatus)
    
    // Test WebSocket message handling
    ctx := &lift.Context{
        Request: lift.Request{
            Metadata: map[string]interface{}{
                "connectionId": "conn-123",
            },
        },
    }
    ctx.Set("userId", "user-123")
    ctx.Set("tenantId", "tenant-abc")
    
    // Execute handler
    err := migrationHandler.StartMigration(ctx)
    assert.NoError(t, err)
    
    // Verify Streamer was called
    messages := mock.GetMessages("conn-123")
    assert.Len(t, messages, 1)
}
```

### Testing Performance and Concurrency

```go
func TestConcurrentMigrations(t *testing.T) {
    mock := connection.NewSendOnlyMock()
    migrator := NewConcurrentMigrator(mock, 10) // 10 workers
    
    // Start 100 concurrent migrations
    userIDs := make([]string, 100)
    for i := 0; i < 100; i++ {
        userIDs[i] = fmt.Sprintf("user-%d", i)
    }
    
    var wg sync.WaitGroup
    errors := make([]error, 100)
    
    for i, userID := range userIDs {
        wg.Add(1)
        go func(idx int, uid string) {
            defer wg.Done()
            errors[idx] = migrator.MigrateUser(uid)
        }(i, userID)
    }
    
    wg.Wait()
    
    // Verify all succeeded
    for i, err := range errors {
        assert.NoError(t, err, "Migration %d failed", i)
    }
    
    // Verify all users got messages
    for _, userID := range userIDs {
        messages := mock.GetMessages(userID)
        assert.GreaterOrEqual(t, len(messages), 1, 
            "User %s didn't receive messages", userID)
    }
}
```

## 📋 Mock Selection Guide for Migrant

| Use Case | Recommended Mock | Why |
|----------|------------------|-----|
| **Basic Migration Notifications** | `SendOnlyMock` | Simple message sending |
| **Progress Tracking** | `ProgressReporterMock` | Connection state + progress |
| **Error Scenario Testing** | `TestableAPIGatewayClient` | Error simulation |
| **Batch Processing** | `SendOnlyMock` | Multiple user handling |
| **Connection Management** | `TestableAPIGatewayClient` | Full WebSocket lifecycle |
| **Retry Logic Testing** | `FailingMock` | Consistent errors |
| **Integration Testing** | `RecordingMock` | Call sequence verification |

## 🎯 Best Practices for Migrant Team

### 1. **Test Message Structure**
```go
func TestMigrationMessageFormat(t *testing.T) {
    mock := connection.NewSendOnlyMock()
    migrator := NewMigrator(mock)
    
    migrator.SendMigrationUpdate("user-123", MigrationUpdate{
        Type:       "progress",
        Percentage: 75,
        Message:    "Migrating user preferences...",
    })
    
    messages := mock.GetMessages("user-123")
    msg := messages[0].(map[string]interface{})
    
    // Verify required fields
    assert.Equal(t, "progress", msg["type"])
    assert.Equal(t, float64(75), msg["percentage"])
    assert.Contains(t, msg["message"], "preferences")
    assert.NotEmpty(t, msg["timestamp"])
}
```

### 2. **Test Error Propagation**
```go
func TestMigrationErrorHandling(t *testing.T) {
    mock := connection.NewFailingMock(errors.New("database timeout"))
    migrator := NewMigrator(mock)
    
    err := migrator.MigrateUserData("user-123")
    
    // Should handle the error gracefully
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to notify user")
    
    // Should still complete the migration
    assert.True(t, migrator.IsMigrationComplete("user-123"))
}
```

### 3. **Test with Real Data Shapes**
```go
func TestMigrationWithRealDataStructures(t *testing.T) {
    mock := connection.NewSendOnlyMock()
    migrator := NewMigrator(mock)
    
    // Use real migration data structures
    migrationReq := MigrationRequest{
        UserID: "user-123",
        MigrationType: "full_profile",
        SourceSchema: "legacy_v1",
        TargetSchema: "current_v2",
        DataMapping: map[string]string{
            "old_name": "full_name",
            "old_email": "email_address",
        },
    }
    
    result := migrator.ExecuteMigration(migrationReq)
    
    assert.True(t, result.Success)
    
    // Verify notification format matches expected client schema
    messages := mock.GetMessages("user-123")
    notification := messages[len(messages)-1].(MigrationNotification)
    
    assert.Equal(t, "user-123", notification.UserID)
    assert.Equal(t, "full_profile", notification.MigrationType)
    assert.NotZero(t, notification.RecordsMigrated)
}
```

## 🚀 Getting Started Checklist

- [ ] Add Streamer dependency: `go mod edit -require github.com/pay-theory/streamer@v1.0.2`
- [ ] Import mocks: `import "github.com/pay-theory/streamer/pkg/connection"`
- [ ] Choose appropriate mock for each test scenario
- [ ] Write tests for happy path migration flows
- [ ] Add error scenario testing with `FailingMock`
- [ ] Test connection state handling with `TestableAPIGatewayClient`
- [ ] Verify message formats match client expectations
- [ ] Add performance/concurrency tests for batch operations

## 📞 Support

If you need additional mock functionality or have questions about testing patterns, the Streamer library provides extensive documentation in:
- `pkg/connection/TESTING_GUIDE.md`
- `pkg/connection/api_gateway_mocking_guide.md`
- `pkg/connection/CENTRALIZED_MOCKS.md`

**Happy Testing!** 🎉 