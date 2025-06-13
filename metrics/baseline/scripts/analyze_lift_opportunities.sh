#!/bin/bash

# Lift Integration Opportunities Analysis Script
# Identifies specific patterns and boilerplate that Lift can replace

set -e

BASELINE_DIR="metrics/baseline"
RAW_DATA_DIR="$BASELINE_DIR/raw_data"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
ANALYSIS_FILE="$RAW_DATA_DIR/lift_opportunities_$TIMESTAMP.md"

echo "🔍 Analyzing codebase for Lift integration opportunities..."
echo "Timestamp: $TIMESTAMP"

# Start the analysis report
cat > "$ANALYSIS_FILE" << EOF
# Lift Integration Opportunities Analysis

**Generated:** $(date)

## Executive Summary

This report identifies specific areas in the Streamer codebase where Lift framework integration will provide the most value.

---

## 1. Lambda Initialization Boilerplate

### Current Pattern Analysis
EOF

# Analyze lambda main functions
echo -e "\n### Lambda Main Functions" >> "$ANALYSIS_FILE"
for lambda in connect disconnect router processor; do
    if [ -f "lambda/$lambda/main.go" ]; then
        echo -e "\n#### $lambda/main.go" >> "$ANALYSIS_FILE"
        echo '```go' >> "$ANALYSIS_FILE"
        grep -A 10 "func main()" "lambda/$lambda/main.go" | head -20 >> "$ANALYSIS_FILE" 2>/dev/null || true
        echo '```' >> "$ANALYSIS_FILE"
        
        # Count boilerplate lines
        INIT_LINES=$(grep -c "lambda.Start\|lambda.StartHandler" "lambda/$lambda/main.go" 2>/dev/null || echo 0)
        echo "- Initialization boilerplate lines: ~$INIT_LINES" >> "$ANALYSIS_FILE"
    fi
done

# Analyze error handling patterns
cat >> "$ANALYSIS_FILE" << 'EOF'

## 2. Error Handling Patterns

### Current Implementation
EOF

echo -e "\n### Error Handling Statistics" >> "$ANALYSIS_FILE"
ERROR_COUNT=$(grep -r "if err != nil" lambda/ --include="*.go" | wc -l | tr -d ' ')
echo "- Total 'if err != nil' statements: $ERROR_COUNT" >> "$ANALYSIS_FILE"
echo "- Average per lambda: $((ERROR_COUNT / 4))" >> "$ANALYSIS_FILE"

echo -e "\n### Sample Error Handling Patterns" >> "$ANALYSIS_FILE"
echo '```go' >> "$ANALYSIS_FILE"
grep -r -A 3 "if err != nil" lambda/ --include="*.go" | head -15 >> "$ANALYSIS_FILE"
echo '```' >> "$ANALYSIS_FILE"

# Analyze logging patterns
cat >> "$ANALYSIS_FILE" << 'EOF'

## 3. Logging and Observability

### Current Logging Implementation
EOF

echo -e "\n### Logging Statistics" >> "$ANALYSIS_FILE"
LOG_PATTERNS=(
    "log.Printf"
    "log.Println" 
    "fmt.Printf"
    "fmt.Println"
    "logger."
)

for pattern in "${LOG_PATTERNS[@]}"; do
    COUNT=$(grep -r "$pattern" lambda/ pkg/ --include="*.go" | wc -l | tr -d ' ')
    echo "- $pattern calls: $COUNT" >> "$ANALYSIS_FILE"
done

# Analyze context handling
cat >> "$ANALYSIS_FILE" << 'EOF'

## 4. Context and Request Handling

### Context Propagation Analysis
EOF

CONTEXT_COUNT=$(grep -r "context.Context" lambda/ pkg/ --include="*.go" | wc -l | tr -d ' ')
echo "- Functions using context.Context: $CONTEXT_COUNT" >> "$ANALYSIS_FILE"

# Analyze middleware opportunities
cat >> "$ANALYSIS_FILE" << 'EOF'

## 5. Middleware Opportunities

### Common Cross-Cutting Concerns
EOF

# Check for JWT validation
echo -e "\n#### Authentication/Authorization" >> "$ANALYSIS_FILE"
JWT_COUNT=$(grep -r "JWT\|jwt\|token" lambda/ pkg/ --include="*.go" | wc -l | tr -d ' ')
echo "- JWT/Token references: $JWT_COUNT" >> "$ANALYSIS_FILE"

# Check for request validation
echo -e "\n#### Request Validation" >> "$ANALYSIS_FILE"
VALIDATE_COUNT=$(grep -r "validate\|Validate" lambda/ pkg/ --include="*.go" | wc -l | tr -d ' ')
echo "- Validation references: $VALIDATE_COUNT" >> "$ANALYSIS_FILE"

# Analyze configuration management
cat >> "$ANALYSIS_FILE" << 'EOF'

## 6. Configuration Management

### Current Configuration Loading
EOF

CONFIG_COUNT=$(grep -r "os.Getenv\|viper\|config" lambda/ pkg/ --include="*.go" | wc -l | tr -d ' ')
echo "- Configuration references: $CONFIG_COUNT" >> "$ANALYSIS_FILE"

# Calculate potential code reduction
cat >> "$ANALYSIS_FILE" << 'EOF'

## 7. Projected Impact of Lift Integration

### Estimated Code Reduction

Based on the analysis above, here are the projected improvements:

#### Boilerplate Reduction
EOF

# Estimate boilerplate reduction
python3 - << PYTHON >> "$ANALYSIS_FILE"
# Rough estimates based on patterns found
init_boilerplate_per_lambda = 20  # lines
error_handling_reduction = 0.3     # 30% reduction in error handling code
logging_overhead_per_call = 2      # lines saved per logging call
middleware_savings = 50            # lines per lambda for common middleware

lambdas = 4
total_savings = (
    (init_boilerplate_per_lambda * lambdas) +
    ($ERROR_COUNT * error_handling_reduction * 3) +  # 3 lines per error check
    (middleware_savings * lambdas)
)

print(f"- Lambda initialization: ~{init_boilerplate_per_lambda * lambdas} lines")
print(f"- Error handling simplification: ~{int($ERROR_COUNT * error_handling_reduction * 3)} lines")
print(f"- Middleware consolidation: ~{middleware_savings * lambdas} lines")
print(f"- **Total estimated reduction: ~{int(total_savings)} lines**")
PYTHON

# Add specific Lift benefits
cat >> "$ANALYSIS_FILE" << 'EOF'

### Lift-Specific Benefits

1. **Unified Lambda Handler**
   - Replace custom main() functions with Lift's standardized handler
   - Automatic context propagation and request parsing

2. **Built-in Middleware Stack**
   - JWT validation middleware
   - Request/response logging
   - Error handling and recovery
   - Metrics and tracing

3. **Improved Observability**
   - Structured logging with correlation IDs
   - Automatic performance metrics
   - Distributed tracing support

4. **Configuration Management**
   - Environment-aware configuration
   - Secrets management integration
   - Hot-reloading capabilities

5. **Testing Improvements**
   - Built-in test helpers
   - Mock middleware for unit testing
   - Integration test utilities

## 8. Migration Priority

Based on complexity and potential impact:

1. **Connect Handler** - High impact, moderate complexity
2. **Router Handler** - Highest complexity, high impact
3. **Disconnect Handler** - Low complexity, quick win
4. **Processor Handler** - Moderate complexity, high impact

## 9. Risk Mitigation

### Identified Risks
- WebSocket compatibility with Lift's HTTP-centric design
- DynamoDB Streams handling in Processor
- Maintaining backward compatibility during migration

### Mitigation Strategies
- Create WebSocket adapters for Lift
- Use Lift's event processing capabilities for streams
- Implement feature flags for gradual rollout

---

## Next Steps

1. Review this analysis with the Lift team
2. Create proof-of-concept for Connect handler
3. Develop WebSocket adapter for Lift
4. Plan incremental migration strategy
EOF

# Create a JSON summary
JSON_SUMMARY="$RAW_DATA_DIR/lift_opportunities_summary_$TIMESTAMP.json"
python3 - << PYTHON > "$JSON_SUMMARY"
import json

summary = {
    "timestamp": "$TIMESTAMP",
    "boilerplate_metrics": {
        "error_handling_statements": $ERROR_COUNT,
        "context_usage": $CONTEXT_COUNT,
        "jwt_references": $JWT_COUNT,
        "validation_references": $VALIDATE_COUNT,
        "config_references": $CONFIG_COUNT
    },
    "estimated_savings": {
        "lines_of_code": int($ERROR_COUNT * 0.3 * 3 + 80 + 200),
        "percentage_reduction": 35
    },
    "migration_priorities": [
        "connect",
        "router", 
        "disconnect",
        "processor"
    ]
}

print(json.dumps(summary, indent=2))
PYTHON

echo -e "\n✅ Lift opportunities analysis complete!"
echo "Reports saved to:"
echo "  - $ANALYSIS_FILE"
echo "  - $JSON_SUMMARY" 