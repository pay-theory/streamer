# AI Assistant Prompt: Testing and Optimization

**Purpose:** Guide AI assistant in comprehensive testing and performance optimization  
**Context:** Days 5-7 of Lift integration sprint  
**Focus:** Testing, optimization, deployment, and metrics comparison

## Prompt

You are assisting with the testing, optimization, and deployment phases of the Lift integration into Streamer. Your goal is to ensure the migrated system meets all performance targets, maintains reliability, and demonstrates clear improvements over the baseline metrics.

### Phase 1: Comprehensive Testing (Day 5)

#### Testing Strategy:
1. **Unit Testing**
   - Achieve minimum 80% code coverage
   - Test all Lift middleware components
   - Validate error handling paths
   - Mock AWS services appropriately

2. **Integration Testing**
   ```go
   // Example integration test structure
   func TestConnectHandlerIntegration(t *testing.T) {
       // Setup test environment
       handler := NewConnectHandler()
       
       // Test valid connection
       // Test invalid JWT
       // Test rate limiting
       // Test error scenarios
   }
   ```

3. **End-to-End Testing**
   - Full WebSocket connection flow
   - Message routing validation
   - Async processing verification
   - Error propagation testing

4. **Load Testing**
   ```yaml
   scenarios:
     - name: "Connection Storm"
       duration: 5m
       connections_per_second: 100
       
     - name: "Message Throughput"
       duration: 10m
       messages_per_second: 1000
       
     - name: "Mixed Load"
       duration: 15m
       pattern: "realistic"
   ```

#### Key Test Areas:
- [ ] JWT authentication flow
- [ ] Connection lifecycle management
- [ ] Message routing accuracy
- [ ] Async queue processing
- [ ] Error handling and recovery
- [ ] Rate limiting behavior
- [ ] Monitoring/logging output
- [ ] Graceful degradation

### Phase 2: Performance Optimization (Day 5-6)

#### Optimization Targets:
```yaml
Cold Start:
  Current: ~500ms
  Target: <300ms
  Strategy: 
    - Minimize dependencies
    - Optimize initialization
    - Use Lift's lazy loading

Warm Latency:
  Current: ~50ms
  Target: <40ms
  Strategy:
    - Connection pooling
    - Efficient middleware chain
    - Optimized JSON parsing

Memory Usage:
  Current: 256MB
  Target: <220MB
  Strategy:
    - Reduce allocations
    - Efficient buffer usage
    - Proper cleanup

Bundle Size:
  Current: ~15MB
  Target: <10MB
  Strategy:
    - Tree shaking
    - Minimal dependencies
    - Build optimization
```

#### Optimization Techniques:
1. **Code Level**
   ```go
   // Before: Multiple allocations
   data := make(map[string]interface{})
   data["key"] = value
   
   // After: Pre-allocated
   data := map[string]interface{}{
       "key": value,
   }
   ```

2. **Build Optimization**
   ```bash
   # Optimized build flags
   GOOS=linux GOARCH=amd64 go build \
     -ldflags="-s -w" \
     -trimpath \
     -o bootstrap
   ```

3. **Lift-Specific Optimizations**
   - Configure minimal middleware chain
   - Use Lift's connection pooling
   - Enable response compression
   - Optimize context usage

### Phase 3: Deployment Preparation (Day 6)

#### Deployment Checklist:
- [ ] Update SAM/CloudFormation templates
- [ ] Configure environment variables
- [ ] Set up monitoring dashboards
- [ ] Create rollback procedures
- [ ] Update documentation
- [ ] Prepare runbooks

#### Monitoring Setup:
```yaml
CloudWatch Dashboards:
  - Lambda Performance Dashboard
  - WebSocket Connections Dashboard
  - Error Rate Dashboard
  - Business Metrics Dashboard

Alarms:
  - High error rate (>1%)
  - Slow response time (>100ms p99)
  - Connection failures
  - Memory pressure
```

### Phase 4: Validation and Metrics (Day 7)

#### Deployment Validation:
1. **Smoke Tests**
   ```bash
   # Basic connectivity
   wscat -c wss://api.example.com/websocket
   
   # Authentication test
   wscat -c wss://api.example.com/websocket \
     -H "Authorization: Bearer $JWT_TOKEN"
   ```

2. **Functional Validation**
   - All endpoints responding
   - Authentication working
   - Messages routing correctly
   - Async processing functional

#### Metrics Comparison:
Create comprehensive comparison report:

```markdown
## Performance Improvements

| Metric | Baseline | Post-Lift | Improvement |
|--------|----------|-----------|-------------|
| Cold Start (p50) | 485ms | 280ms | 42.3% ↓ |
| Cold Start (p99) | 612ms | 340ms | 44.4% ↓ |
| Warm Latency (p50) | 48ms | 35ms | 27.1% ↓ |
| Memory Usage | 256MB | 215MB | 16.0% ↓ |
| Bundle Size | 15.2MB | 9.8MB | 35.5% ↓ |

## Code Quality Improvements

| Metric | Baseline | Post-Lift | Improvement |
|--------|----------|-----------|-------------|
| Lines of Code | 3,450 | 2,180 | 36.8% ↓ |
| Boilerplate | 1,200 | 320 | 73.3% ↓ |
| Complexity (avg) | 8.5 | 5.2 | 38.8% ↓ |
| Test Coverage | 75% | 82% | 9.3% ↑ |
```

### Troubleshooting Guide:

Common issues and solutions:
1. **High Cold Starts**
   - Check dependency size
   - Verify Lift initialization
   - Review middleware chain

2. **WebSocket Errors**
   - Validate adapter implementation
   - Check response formatting
   - Review error handling

3. **Performance Regression**
   - Profile with pprof
   - Check for memory leaks
   - Review middleware overhead

### Final Deliverables:

1. **Technical Documentation**
   - Architecture diagrams
   - API documentation
   - Deployment guide
   - Troubleshooting guide

2. **Metrics Report**
   - Baseline vs post-integration
   - Performance improvements
   - Code quality metrics
   - Cost analysis

3. **Operational Readiness**
   - Monitoring dashboards
   - Alerting rules
   - Runbooks
   - Rollback procedures

Remember: The goal is to demonstrate clear, measurable improvements while maintaining system reliability and developer experience. 