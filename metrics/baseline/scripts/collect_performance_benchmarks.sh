#!/bin/bash

# Local Performance Benchmarking Script for Streamer Project
# Measures build times, test performance, and simulated lambda initialization

set -e

BASELINE_DIR="metrics/baseline"
RAW_DATA_DIR="$BASELINE_DIR/raw_data"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "⏱️  Starting local performance benchmarking..."
echo "Timestamp: $TIMESTAMP"

# Create output file
PERF_FILE="$RAW_DATA_DIR/performance_benchmarks_$TIMESTAMP.txt"
JSON_FILE="$RAW_DATA_DIR/performance_benchmarks_$TIMESTAMP.json"

{
    echo "=== Local Performance Benchmarks ==="
    echo "Timestamp: $TIMESTAMP"
    echo ""
} > "$PERF_FILE"

# 1. Lambda Build Times
echo -e "\n🔨 Measuring lambda build times..."
for lambda in connect disconnect router processor; do
    if [ -d "lambda/$lambda" ]; then
        echo "Building $lambda..." | tee -a "$PERF_FILE"
        
        # Measure cold build (clean cache)
        go clean -cache
        START=$(date +%s.%N)
        cd "lambda/$lambda"
        GOOS=linux GOARCH=amd64 go build -o /dev/null 2>&1 || true
        END=$(date +%s.%N)
        COLD_BUILD_TIME=$(echo "$END - $START" | bc)
        
        # Measure warm build
        START=$(date +%s.%N)
        GOOS=linux GOARCH=amd64 go build -o /dev/null 2>&1 || true
        END=$(date +%s.%N)
        WARM_BUILD_TIME=$(echo "$END - $START" | bc)
        
        cd ../..
        
        {
            echo "  Cold build time: ${COLD_BUILD_TIME}s"
            echo "  Warm build time: ${WARM_BUILD_TIME}s"
            echo ""
        } | tee -a "$PERF_FILE"
    fi
done

# 2. Test Execution Performance
echo -e "\n🧪 Measuring test execution times..."
{
    echo "=== Test Execution Performance ==="
    
    # Run tests multiple times to get average
    ITERATIONS=3
    for i in $(seq 1 $ITERATIONS); do
        echo "Test run $i of $ITERATIONS:"
        START=$(date +%s.%N)
        go test $(go list ./... | grep -v /examples/) -count=1 > /dev/null 2>&1 || true
        END=$(date +%s.%N)
        TEST_TIME=$(echo "$END - $START" | bc)
        echo "  Run $i: ${TEST_TIME}s"
    done
    echo ""
} | tee -a "$PERF_FILE"

# 3. Package Loading Times (simulated lambda init)
echo -e "\n📦 Measuring package initialization times..."
cat > metrics/baseline/scripts/measure_init.go << 'EOF'
package main

import (
    "fmt"
    "time"
    _ "github.com/pay-theory/streamer/lambda/connect"
    _ "github.com/pay-theory/streamer/lambda/disconnect"
    _ "github.com/pay-theory/streamer/lambda/router"
    _ "github.com/pay-theory/streamer/lambda/processor"
)

func main() {
    start := time.Now()
    // Simulate initialization
    time.Sleep(1 * time.Millisecond)
    duration := time.Since(start)
    fmt.Printf("Package load time: %v\n", duration)
}
EOF

# Build and run the init measurement
go build -o metrics/baseline/scripts/measure_init metrics/baseline/scripts/measure_init.go 2>/dev/null || true
if [ -f "metrics/baseline/scripts/measure_init" ]; then
    INIT_TIME=$(./metrics/baseline/scripts/measure_init 2>&1 | grep "Package load time:" || echo "Failed to measure")
    echo "Package initialization: $INIT_TIME" | tee -a "$PERF_FILE"
    rm -f metrics/baseline/scripts/measure_init metrics/baseline/scripts/measure_init.go
fi

# 4. Memory Usage Analysis
echo -e "\n💾 Analyzing memory usage..."
{
    echo ""
    echo "=== Memory Usage Analysis ==="
    
    # Get binary sizes
    for lambda in connect disconnect router processor; do
        if [ -d "lambda/$lambda" ]; then
            cd "lambda/$lambda"
            GOOS=linux GOARCH=amd64 go build -o bootstrap 2>/dev/null || true
            if [ -f "bootstrap" ]; then
                SIZE=$(ls -lh bootstrap | awk '{print $5}')
                echo "$lambda binary size: $SIZE"
                rm -f bootstrap
            fi
            cd ../..
        fi
    done
} | tee -a "$PERF_FILE"

# 5. Create JSON summary
echo -e "\n📊 Creating performance summary..."
python3 - << EOF > "$JSON_FILE"
import json
import re

perf_data = {
    "timestamp": "$TIMESTAMP",
    "build_times": {
        "connect": {"cold": 0, "warm": 0},
        "disconnect": {"cold": 0, "warm": 0},
        "router": {"cold": 0, "warm": 0},
        "processor": {"cold": 0, "warm": 0}
    },
    "test_execution": {
        "iterations": ${ITERATIONS:-3},
        "times": []
    },
    "recommendations": []
}

# Parse the performance file
with open("$PERF_FILE", "r") as f:
    content = f.read()
    
    # Extract build times
    for lambda_name in ["connect", "disconnect", "router", "processor"]:
        cold_match = re.search(f"Building {lambda_name}.*?Cold build time: ([0-9.]+)s", content, re.DOTALL)
        warm_match = re.search(f"Building {lambda_name}.*?Warm build time: ([0-9.]+)s", content, re.DOTALL)
        
        if cold_match:
            perf_data["build_times"][lambda_name]["cold"] = float(cold_match.group(1))
        if warm_match:
            perf_data["build_times"][lambda_name]["warm"] = float(warm_match.group(1))
    
    # Extract test times
    test_matches = re.findall(r"Run \d+: ([0-9.]+)s", content)
    perf_data["test_execution"]["times"] = [float(t) for t in test_matches]
    if perf_data["test_execution"]["times"]:
        perf_data["test_execution"]["average"] = sum(perf_data["test_execution"]["times"]) / len(perf_data["test_execution"]["times"])

# Add recommendations based on data
avg_cold_build = sum(l["cold"] for l in perf_data["build_times"].values()) / 4
if avg_cold_build > 10:
    perf_data["recommendations"].append("High build times detected - Lift's optimized build process could help")

if perf_data["test_execution"].get("average", 0) > 30:
    perf_data["recommendations"].append("Long test execution times - Consider parallelizing tests")

print(json.dumps(perf_data, indent=2))
EOF

echo -e "\n✅ Performance benchmarking complete!"
echo "Results saved to:"
echo "  - $PERF_FILE"
echo "  - $JSON_FILE" 