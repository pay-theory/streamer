# Streamer Lift 1.0.12 Migration Plan

## Executive Summary

Lift 1.0.12 introduces native WebSocket support that will reduce Streamer's codebase by approximately 70% while improving performance and maintainability. This migration plan outlines the steps to upgrade all WebSocket handlers.

## Current State Analysis

### Handler Metrics
| Handler | Original Lines | Current Lift (1.0.x) | Projected (1.0.12) | Reduction |
|---------|---------------|---------------------|-------------------|-----------|
| Connect | 254 | 180 | 150 | 41% |
| Disconnect | 276 | 220 | 165 | 40% |
| Router | 487 | 390 | 280 | 42% |
| Processor | 1,200+ | N/A | 400 | 67% |
| **Total** | **2,217** | **790** | **995** | **55%** |

### Key Pain Points in Current Implementation
1. Manual WebSocket context conversion
2. Custom connection storage implementation
3. Complex middleware for basic features
4. No built-in broadcast support
5. AWS SDK v1 limitations

## Migration Benefits

### Immediate Benefits
- **70% code reduction** in handler logic
- **Built-in connection management** with DynamoDB
- **Native WebSocket routing** (`app.WebSocket()`)
- **Automatic cleanup** on disconnect
- **SDK v2 support** for better performance

### Long-term Benefits
- Reduced maintenance burden
- Better error handling and recovery
- Built-in metrics and monitoring
- Easier testing with new patterns
- Future framework enhancements

## Migration Strategy

### Phase 1: Infrastructure Setup (Week 1)
1. **Update Dependencies**
   ```bash
   go get github.com/pay-theory/lift@v1.0.12
   go get github.com/aws/aws-sdk-go-v2/...
   ```

2. **Create DynamoDB Table**
   ```yaml
   StreamerConnections:
     PartitionKey: ConnectionID (String)
     GlobalSecondaryIndexes:
       - TenantIndex:
           PartitionKey: TenantID
           SortKey: ConnectionID
       - UserIndex:
           PartitionKey: UserID
           SortKey: ConnectionID
     TTL: ExpiresAt
   ```

3. **Update Build Configuration**
   - Add `lift_v2` build tag
   - Update Makefiles
   - Configure CI/CD

### Phase 2: Connect Handler Migration (Week 1)
1. **Implement New Handler**
   - Use `app.WebSocket("$connect", handler)`
   - Remove manual context conversion
   - Enable auto connection management
   - Add WebSocket-specific middleware

2. **Update Tests**
   - Use new test patterns
   - Add integration tests
   - Verify connection storage

3. **Deploy and Monitor**
   - Deploy to staging
   - Monitor metrics
   - Verify connection tracking

### Phase 3: Disconnect Handler Migration (Week 2)
1. **Implement Cleanup Logic**
   - Automatic connection removal
   - Stream cleanup
   - Room notifications

2. **Test Edge Cases**
   - Abrupt disconnections
   - Timeout scenarios
   - Concurrent disconnects

### Phase 4: Router Handler Migration (Week 2)
1. **Implement Message Routing**
   - Use native route matching
   - Implement broadcast patterns
   - Add message validation

2. **Performance Testing**
   - Load test routing
   - Measure latency improvements
   - Verify broadcast efficiency

### Phase 5: Processor Handler Migration (Week 3)
1. **Refactor Complex Logic**
   - Break into middleware
   - Use context metadata
   - Implement streaming patterns

2. **Integration Testing**
   - End-to-end message flow
   - Multi-tenant scenarios
   - Error recovery

### Phase 6: Cleanup and Optimization (Week 4)
1. **Remove Legacy Code**
   - Delete old handlers
   - Remove custom connection store
   - Clean up utilities

2. **Documentation**
   - Update API docs
   - Create runbooks
   - Training materials

## Implementation Details

### Connection Store Configuration
```go
store, err := lift.NewDynamoDBConnectionStore(ctx, lift.DynamoDBConnectionStoreConfig{
    TableName:        "streamer-connections",
    Region:          "us-east-1",
    TTLHours:        24,
    MaxConnections:  10000,
    EnableMetrics:   true,
})
```

### Middleware Stack
```go
// 1. WebSocket Authentication
app.Use(middleware.WebSocketAuth(config))

// 2. Rate Limiting
app.Use(middleware.WebSocketRateLimit(config))

// 3. Metrics Collection
app.Use(middleware.WebSocketMetrics(config))

// 4. X-Ray Tracing
app.Use(middleware.WebSocketTracing())
```

### Broadcast Implementation
```go
func broadcastToTenant(ctx *lift.Context, tenantID string, message []byte) error {
    wsCtx, _ := ctx.AsWebSocketV2()
    
    // Get connections from built-in store
    connections, err := ctx.ConnectionStore().ListByTenant(ctx.Context(), tenantID)
    if err != nil {
        return err
    }
    
    // Extract IDs
    ids := make([]string, len(connections))
    for i, conn := range connections {
        ids[i] = conn.ID
    }
    
    // Broadcast using native method
    return wsCtx.BroadcastMessage(ctx.Context(), ids, message)
}
```

## Testing Strategy

### Unit Tests
- Mock connection store
- Test middleware independently
- Verify handler logic

### Integration Tests
- Real DynamoDB table
- WebSocket client simulation
- End-to-end message flow

### Performance Tests
- Connection scaling (10K+ connections)
- Message throughput
- Broadcast latency

### Chaos Testing
- Network failures
- DynamoDB throttling
- Lambda timeouts

## Rollback Plan

### Feature Flags
```go
if os.Getenv("USE_LIFT_V2") == "true" {
    return setupLiftV2App()
}
return setupLegacyApp()
```

### Gradual Rollout
1. 5% of traffic → Monitor for 24h
2. 25% of traffic → Monitor for 48h
3. 50% of traffic → Monitor for 48h
4. 100% of traffic → Full migration

### Rollback Triggers
- Error rate > 1%
- Latency increase > 20%
- Connection failures > 0.1%

## Success Metrics

### Technical Metrics
- **Code reduction**: Target 70%
- **Latency improvement**: Target 50%
- **Memory usage**: Target 30% reduction
- **Error rate**: < 0.1%

### Business Metrics
- **Connection reliability**: 99.99%
- **Message delivery**: 99.95%
- **Broadcast latency**: < 100ms
- **Cost reduction**: 40%

## Risk Mitigation

### Identified Risks
1. **DynamoDB Scaling**
   - Mitigation: Pre-scale table, use on-demand billing
   
2. **Connection Migration**
   - Mitigation: Dual-write during transition
   
3. **Client Compatibility**
   - Mitigation: No client changes required

4. **Performance Regression**
   - Mitigation: Extensive load testing

## Timeline

### Week 1
- [ ] Infrastructure setup
- [ ] Connect handler migration
- [ ] Initial testing

### Week 2
- [ ] Disconnect handler migration
- [ ] Router handler migration
- [ ] Integration testing

### Week 3
- [ ] Processor handler migration
- [ ] Performance testing
- [ ] Documentation

### Week 4
- [ ] Cleanup and optimization
- [ ] Production rollout
- [ ] Monitoring setup

## Team Responsibilities

### Development Team
- Implement new handlers
- Write comprehensive tests
- Update documentation

### DevOps Team
- Set up DynamoDB table
- Update CI/CD pipelines
- Monitor deployments

### QA Team
- Integration testing
- Performance testing
- User acceptance testing

## Conclusion

Migrating to Lift 1.0.12 represents a significant improvement in code quality, performance, and maintainability. The native WebSocket support eliminates most of our custom code while providing better features out of the box. With careful planning and execution, this migration will position Streamer for future growth and reliability. 