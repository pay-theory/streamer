# AI Assistant Prompt: Baseline Metrics Collection

**Purpose:** Guide AI assistant in helping collect comprehensive baseline metrics before Lift integration  
**Context:** Day 0 of Lift integration sprint  
**Duration:** 4 hours

## Prompt

You are assisting with collecting baseline metrics for the Streamer project before integrating the Lift framework. Your goal is to help establish comprehensive performance and code quality baselines that will be used to measure the impact of the Lift integration.

### Your Tasks:

1. **Performance Metrics Collection**
   - Help create scripts to measure cold start times for all 4 lambda functions (connect, disconnect, router, processor)
   - Guide the setup of load tests to measure warm performance
   - Assist in extracting CloudWatch metrics for the past 7 days
   - Document p50, p90, and p99 latencies

2. **Code Metrics Analysis**
   - Use cloc to analyze lines of code, separating boilerplate from business logic
   - Run gocyclo to measure cyclomatic complexity
   - Analyze the dependency tree and count direct vs indirect dependencies
   - Measure current test coverage percentage
   - Calculate bundle sizes for each lambda

3. **Documentation**
   - Create a structured baseline report in `streamer/metrics/baseline/`
   - Include all raw data and calculated metrics
   - Highlight areas that could benefit most from Lift integration

### Key Metrics to Focus On:

```yaml
Performance:
  - Cold Start Times (ms)
  - Warm Latency (ms)
  - Memory Usage (MB)
  - Bundle Size (MB)

Code Quality:
  - Total Lines of Code
  - Boilerplate Percentage
  - Cyclomatic Complexity
  - Test Coverage Percentage
  - Number of Dependencies
```

### Expected Deliverables:

1. `metrics/baseline/performance_metrics.json` - Raw performance data
2. `metrics/baseline/code_metrics.json` - Code analysis results
3. `metrics/baseline/baseline_report.md` - Comprehensive baseline report
4. `metrics/baseline/improvement_targets.md` - Specific targets for Lift integration

### Important Considerations:

- Ensure all measurements are reproducible
- Document the exact commands and tools used
- Take multiple samples for performance metrics to ensure accuracy
- Identify current pain points that Lift can address
- Create visualizations where helpful (charts, graphs)

### Sample Commands to Use:

```bash
# Performance testing
aws lambda invoke --function-name streamer-connect --payload '{}' response.json

# Code analysis
cloc streamer/lambda --json --exclude-dir=vendor
gocyclo -over 10 streamer/lambda
go test ./... -coverprofile=coverage.out

# Bundle size
cd lambda/connect && GOOS=linux go build -o bootstrap && zip -r connect.zip bootstrap && ls -lh connect.zip
```

Remember to compare these baselines with post-integration metrics to demonstrate the value of Lift. 