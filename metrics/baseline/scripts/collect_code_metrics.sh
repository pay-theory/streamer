#!/bin/bash

# Code Metrics Collection Script for Streamer Project
# Focuses on local analysis without requiring AWS deployment

set -e

BASELINE_DIR="metrics/baseline"
RAW_DATA_DIR="$BASELINE_DIR/raw_data"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "🔍 Starting baseline code metrics collection..."
echo "Timestamp: $TIMESTAMP"

# Create output directories
mkdir -p "$RAW_DATA_DIR"

# 1. Lines of Code Analysis
echo -e "\n📊 Analyzing lines of code..."
if command -v gocloc &> /dev/null; then
    # Use gocloc for Go projects
    gocloc lambda/ pkg/ internal/ \
        --output-type=json \
        --exclude-ext=mod,sum \
        --not-match-d="vendor|node_modules|examples" > "$RAW_DATA_DIR/cloc_metrics_$TIMESTAMP.json"
    
    # Also create a human-readable version
    gocloc lambda/ pkg/ internal/ \
        --exclude-ext=mod,sum \
        --not-match-d="vendor|node_modules|examples" > "$RAW_DATA_DIR/cloc_metrics_$TIMESTAMP.txt"
else
    echo "⚠️  gocloc not found. Please install: go install github.com/hhatto/gocloc/cmd/gocloc@latest"
fi

# 2. Cyclomatic Complexity Analysis
echo -e "\n🔄 Analyzing cyclomatic complexity..."
if command -v gocyclo &> /dev/null; then
    gocyclo -over 5 lambda/ pkg/ internal/ > "$RAW_DATA_DIR/complexity_metrics_$TIMESTAMP.txt" || true
    
    # Also capture all complexity scores
    gocyclo lambda/ pkg/ internal/ > "$RAW_DATA_DIR/complexity_all_$TIMESTAMP.txt" || true
else
    echo "⚠️  gocyclo not found. Please install: go install github.com/fzipp/gocyclo/cmd/gocyclo@latest"
fi

# 3. Test Coverage Analysis
echo -e "\n✅ Measuring test coverage..."
go test $(go list ./... | grep -v /examples/) -coverprofile="$RAW_DATA_DIR/coverage_$TIMESTAMP.out" -covermode=atomic > "$RAW_DATA_DIR/test_results_$TIMESTAMP.txt" 2>&1 || true

# Generate coverage report
if [ -f "$RAW_DATA_DIR/coverage_$TIMESTAMP.out" ]; then
    go tool cover -html="$RAW_DATA_DIR/coverage_$TIMESTAMP.out" -o "$RAW_DATA_DIR/coverage_$TIMESTAMP.html"
    
    # Extract coverage percentage
    COVERAGE=$(go tool cover -func="$RAW_DATA_DIR/coverage_$TIMESTAMP.out" | grep total | awk '{print $3}')
    echo "Total coverage: $COVERAGE" > "$RAW_DATA_DIR/coverage_summary_$TIMESTAMP.txt"
fi

# 4. Dependency Analysis
echo -e "\n📦 Analyzing dependencies..."
go list -m all > "$RAW_DATA_DIR/dependencies_$TIMESTAMP.txt"

# Count direct vs indirect dependencies
DIRECT_DEPS=$(grep -v "// indirect" "$RAW_DATA_DIR/dependencies_$TIMESTAMP.txt" | wc -l)
INDIRECT_DEPS=$(grep "// indirect" "$RAW_DATA_DIR/dependencies_$TIMESTAMP.txt" | wc -l)

echo "Direct dependencies: $((DIRECT_DEPS - 1))" > "$RAW_DATA_DIR/dependency_summary_$TIMESTAMP.txt"
echo "Indirect dependencies: $INDIRECT_DEPS" >> "$RAW_DATA_DIR/dependency_summary_$TIMESTAMP.txt"

# 5. Bundle Size Analysis (Build each lambda locally)
echo -e "\n📦 Building lambdas to measure bundle sizes..."
mkdir -p "$RAW_DATA_DIR/builds"

for lambda in connect disconnect router processor; do
    echo "Building $lambda lambda..."
    if [ -d "lambda/$lambda" ]; then
        cd "lambda/$lambda"
        GOOS=linux GOARCH=amd64 go build -o bootstrap
        zip -r "$lambda.zip" bootstrap > /dev/null 2>&1
        
        # Get size in bytes and MB
        SIZE_BYTES=$(stat -f%z "$lambda.zip" 2>/dev/null || stat -c%s "$lambda.zip" 2>/dev/null)
        SIZE_MB=$(echo "scale=2; $SIZE_BYTES / 1048576" | bc)
        
        echo "$lambda: ${SIZE_MB}MB (${SIZE_BYTES} bytes)" >> "../../$RAW_DATA_DIR/bundle_sizes_$TIMESTAMP.txt"
        
        # Move the zip for reference
        mv "$lambda.zip" "../../$RAW_DATA_DIR/builds/"
        rm -f bootstrap
        cd ../..
    fi
done

# 6. Identify Boilerplate Code Patterns
echo -e "\n🔍 Analyzing boilerplate patterns..."
{
    echo "=== Boilerplate Analysis ==="
    echo "Lambda initialization patterns:"
    grep -r "func main()" lambda/ --include="*.go" | wc -l
    
    echo -e "\nError handling patterns:"
    grep -r "if err != nil" lambda/ pkg/ internal/ --include="*.go" | wc -l
    
    echo -e "\nLogging statements:"
    grep -r "log\." lambda/ pkg/ internal/ --include="*.go" | wc -l
    
    echo -e "\nContext handling:"
    grep -r "context\." lambda/ pkg/ internal/ --include="*.go" | wc -l
} > "$RAW_DATA_DIR/boilerplate_analysis_$TIMESTAMP.txt"

# 7. Create summary JSON
echo -e "\n📋 Creating summary..."
python3 - << EOF > "$RAW_DATA_DIR/metrics_summary_$TIMESTAMP.json"
import json
import re
import os

summary = {
    "timestamp": "$TIMESTAMP",
    "code_metrics": {},
    "test_metrics": {},
    "dependency_metrics": {
        "direct": $((DIRECT_DEPS - 1)),
        "indirect": $INDIRECT_DEPS
    },
    "bundle_sizes": {},
    "boilerplate_indicators": {}
}

# Parse gocloc output
if os.path.exists("$RAW_DATA_DIR/cloc_metrics_$TIMESTAMP.json"):
    with open("$RAW_DATA_DIR/cloc_metrics_$TIMESTAMP.json") as f:
        cloc_data = json.load(f)
        if "languages" in cloc_data:
            for lang_data in cloc_data["languages"]:
                if lang_data.get("name") == "Go":
                    summary["code_metrics"] = {
                        "files": lang_data.get("files", 0),
                        "lines_of_code": lang_data.get("code", 0),
                        "comments": lang_data.get("comment", 0),
                        "blank_lines": lang_data.get("blank", 0),
                        "total_lines": lang_data.get("code", 0) + lang_data.get("comment", 0) + lang_data.get("blank", 0)
                    }
                    break

# Parse bundle sizes
if os.path.exists("$RAW_DATA_DIR/bundle_sizes_$TIMESTAMP.txt"):
    with open("$RAW_DATA_DIR/bundle_sizes_$TIMESTAMP.txt") as f:
        for line in f:
            if ":" in line:
                lambda_name, size_info = line.strip().split(":")
                mb_match = re.search(r'(\d+\.\d+)MB', size_info)
                if mb_match:
                    summary["bundle_sizes"][lambda_name] = float(mb_match.group(1))

# Parse coverage
if os.path.exists("$RAW_DATA_DIR/coverage_summary_$TIMESTAMP.txt"):
    with open("$RAW_DATA_DIR/coverage_summary_$TIMESTAMP.txt") as f:
        coverage_line = f.read().strip()
        coverage_match = re.search(r'(\d+\.\d+)%', coverage_line)
        if coverage_match:
            summary["test_metrics"]["coverage_percentage"] = float(coverage_match.group(1))

print(json.dumps(summary, indent=2))
EOF

echo -e "\n✅ Baseline metrics collection complete!"
echo "Raw data saved to: $RAW_DATA_DIR"
echo "Latest summary: $RAW_DATA_DIR/metrics_summary_$TIMESTAMP.json" 