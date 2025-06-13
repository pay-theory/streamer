#!/bin/bash

# Master Baseline Collection Script
# Orchestrates all baseline metrics collection for Streamer project

set -e

echo "🚀 Streamer Baseline Metrics Collection"
echo "======================================"
echo "Date: $(date)"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to check and install tools
check_tool() {
    local tool=$1
    local install_cmd=$2
    local go_install=$3
    
    if command -v "$tool" &> /dev/null; then
        echo -e "${GREEN}✓${NC} $tool is installed"
    else
        echo -e "${YELLOW}⚠${NC}  $tool not found"
        if [ "$go_install" = "true" ]; then
            echo "Installing $tool via go install..."
            eval "$install_cmd"
        else
            echo -e "${RED}Please install $tool manually:${NC}"
            echo "  $install_cmd"
            return 1
        fi
    fi
}

echo "📋 Checking required tools..."
echo ""

# Check for required tools
TOOLS_OK=true

# Check standard tools
check_tool "bc" "brew install bc" || TOOLS_OK=false
check_tool "python3" "brew install python3" || TOOLS_OK=false

# Check/install Go tools
check_tool "gocloc" "go install github.com/hhatto/gocloc/cmd/gocloc@latest" true
check_tool "gocyclo" "go install github.com/fzipp/gocyclo/cmd/gocyclo@latest" true

echo ""

if [ "$TOOLS_OK" = false ]; then
    echo -e "${RED}❌ Some tools are missing. Please install them and run again.${NC}"
    exit 1
fi

# Make all scripts executable
echo "🔧 Making scripts executable..."
chmod +x metrics/baseline/scripts/*.sh

# Create results directory
RESULTS_DIR="metrics/baseline/results_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo ""
echo "📊 Starting baseline collection..."
echo "Results will be saved to: $RESULTS_DIR"
echo ""

# Run each collection script
echo "════════════════════════════════════════"
echo "1. Collecting Code Metrics"
echo "════════════════════════════════════════"
./metrics/baseline/scripts/collect_code_metrics.sh

echo ""
echo "════════════════════════════════════════"
echo "2. Collecting Performance Benchmarks"
echo "════════════════════════════════════════"
./metrics/baseline/scripts/collect_performance_benchmarks.sh

echo ""
echo "════════════════════════════════════════"
echo "3. Analyzing Lift Integration Opportunities"
echo "════════════════════════════════════════"
./metrics/baseline/scripts/analyze_lift_opportunities.sh

# Copy latest results to the results directory
echo ""
echo "📁 Organizing results..."
cp -r metrics/baseline/raw_data/* "$RESULTS_DIR/" 2>/dev/null || true

# Generate consolidated baseline report
echo ""
echo "📑 Generating consolidated baseline report..."

cat > "$RESULTS_DIR/baseline_report.md" << 'EOF'
# Streamer Baseline Metrics Report

**Generated:** $(date)  
**Purpose:** Pre-Lift integration baseline for comparison

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Code Quality Metrics](#code-quality-metrics)
3. [Performance Benchmarks](#performance-benchmarks)
4. [Lift Integration Opportunities](#lift-integration-opportunities)
5. [Recommendations](#recommendations)

---

## Executive Summary

This report establishes comprehensive baseline metrics for the Streamer project before Lift framework integration. All metrics were collected through local analysis without requiring AWS deployment.

### Key Findings

EOF

# Add summary data to report
python3 - << PYTHON >> "$RESULTS_DIR/baseline_report.md"
import json
import glob
import os

# Find the latest JSON files
raw_data_dir = "metrics/baseline/raw_data"
latest_files = {}

for pattern in ["metrics_summary_*.json", "performance_benchmarks_*.json", "lift_opportunities_summary_*.json"]:
    files = glob.glob(os.path.join(raw_data_dir, pattern))
    if files:
        latest_files[pattern.split('_')[0]] = max(files, key=os.path.getmtime)

# Load and summarize data
if "metrics" in latest_files:
    with open(latest_files["metrics"]) as f:
        metrics = json.load(f)
        if "test_metrics" in metrics and "coverage_percentage" in metrics["test_metrics"]:
            print(f"- **Test Coverage:** {metrics['test_metrics']['coverage_percentage']:.1f}%")
        if "dependency_metrics" in metrics:
            print(f"- **Dependencies:** {metrics['dependency_metrics']['direct']} direct, {metrics['dependency_metrics']['indirect']} indirect")
        if "bundle_sizes" in metrics:
            avg_size = sum(metrics['bundle_sizes'].values()) / len(metrics['bundle_sizes']) if metrics['bundle_sizes'] else 0
            print(f"- **Average Lambda Size:** {avg_size:.2f}MB")

if "performance" in latest_files:
    with open(latest_files["performance"]) as f:
        perf = json.load(f)
        if "build_times" in perf:
            avg_cold = sum(l["cold"] for l in perf["build_times"].values()) / 4
            print(f"- **Average Cold Build Time:** {avg_cold:.2f}s")

if "lift" in latest_files:
    with open(latest_files["lift"]) as f:
        lift = json.load(f)
        if "estimated_savings" in lift:
            print(f"- **Estimated Code Reduction with Lift:** {lift['estimated_savings']['percentage_reduction']}%")

print("\n## Code Quality Metrics\n")
print("*See raw_data/cloc_metrics_*.txt for detailed breakdown*\n")
print("## Performance Benchmarks\n")
print("*See raw_data/performance_benchmarks_*.txt for detailed results*\n")
print("## Lift Integration Opportunities\n")
print("*See raw_data/lift_opportunities_*.md for comprehensive analysis*\n")
PYTHON

cat >> "$RESULTS_DIR/baseline_report.md" << 'EOF'

## Recommendations

Based on the baseline analysis:

1. **Prioritize Connect Handler** for initial Lift migration (moderate complexity, high impact)
2. **Focus on middleware consolidation** to achieve quick wins in code reduction
3. **Implement comprehensive testing** during migration to maintain coverage levels
4. **Create WebSocket adapters** early to mitigate compatibility risks

## Next Steps

1. Share this baseline report with the Lift team
2. Set up development environment with Lift dependencies
3. Begin Day 1 tasks per the integration plan
4. Use these metrics to measure post-integration improvements

---

*For raw data and detailed metrics, see the accompanying files in this directory.*
EOF

# Create a summary dashboard
cat > "$RESULTS_DIR/dashboard.txt" << 'EOF'
╔══════════════════════════════════════════════════════════╗
║          STREAMER BASELINE METRICS DASHBOARD             ║
╠══════════════════════════════════════════════════════════╣
║                                                          ║
║  📊 Code Metrics                                         ║
║  ├─ Lines of Code: Check cloc_metrics_*.txt             ║
║  ├─ Complexity: Check complexity_metrics_*.txt          ║
║  └─ Coverage: Check coverage_summary_*.txt              ║
║                                                          ║
║  ⚡ Performance                                          ║
║  ├─ Build Times: Check performance_benchmarks_*.txt     ║
║  ├─ Test Speed: Check performance_benchmarks_*.json     ║
║  └─ Bundle Sizes: Check bundle_sizes_*.txt              ║
║                                                          ║
║  🎯 Lift Opportunities                                   ║
║  ├─ Analysis: Check lift_opportunities_*.md             ║
║  └─ Summary: Check lift_opportunities_summary_*.json    ║
║                                                          ║
╚══════════════════════════════════════════════════════════╝
EOF

echo ""
echo -e "${GREEN}✅ Baseline collection complete!${NC}"
echo ""
echo "📊 Results saved to: $RESULTS_DIR"
echo ""
echo "Key files generated:"
echo "  - baseline_report.md: Consolidated report"
echo "  - dashboard.txt: Quick reference dashboard"
echo "  - Raw data files: Detailed metrics"
echo ""
echo "Next steps:"
echo "  1. Review the baseline_report.md"
echo "  2. Share results with the Lift team"
echo "  3. Begin Day 1 of Lift integration" 