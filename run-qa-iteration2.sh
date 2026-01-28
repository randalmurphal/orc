#!/bin/bash
#
# QA Testing Script - Settings Page Iteration 2
# Tests bug fixes from Iteration 1 and performs comprehensive edge case testing
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB_DIR="$SCRIPT_DIR/web"
RESULTS_DIR="/tmp/qa-TASK-616-iteration2"

echo "╔═══════════════════════════════════════════════════════════╗"
echo "║        QA Testing - Settings Page (Iteration 2)          ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

# Ensure screenshot directory exists
mkdir -p "$RESULTS_DIR"

echo "📁 Test results will be saved to: $RESULTS_DIR"
echo ""

# Check if servers are running
echo "🔍 Checking if servers are running..."
if ! curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
    echo "⚠️  API server not running on :8080"
    echo "   Start it with: cd .. && ./bin/orc serve"
    exit 1
fi

if ! curl -s http://localhost:5173 > /dev/null 2>&1; then
    echo "⚠️  Frontend not running on :5173"
    echo "   Start it with: cd web && bun run dev"
    exit 1
fi

echo "✓ API server running on :8080"
echo "✓ Frontend running on :5173"
echo ""

# Navigate to web directory
cd "$WEB_DIR"

echo "🧪 Running QA tests..."
echo ""

# Run the iteration 2 test suite
bunx playwright test qa-iteration2-verification.spec.ts \
    --reporter=list \
    --reporter=html

EXIT_CODE=$?

echo ""
echo "═══════════════════════════════════════════════════════════"

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ All tests PASSED"
else
    echo "❌ Some tests FAILED"
fi

echo ""
echo "📊 Results:"
echo "   - Screenshots: $RESULTS_DIR"
echo "   - HTML Report: $WEB_DIR/playwright-report/index.html"
echo "   - JSON Report: $WEB_DIR/test-results/results.json"
echo ""
echo "📝 To view HTML report:"
echo "   cd $WEB_DIR && bunx playwright show-report"
echo ""
echo "═══════════════════════════════════════════════════════════"

exit $EXIT_CODE
