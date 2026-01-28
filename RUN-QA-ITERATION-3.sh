#!/bin/bash

# QA Iteration 3 - Complete Test Runner
# This script runs comprehensive E2E testing of the Settings page

set -e

WORKDIR="/home/randy/repos/orc/.orc/worktrees/orc-TASK-616"
cd "$WORKDIR/web"

echo ""
echo "========================================"
echo "QA ITERATION 3"
echo "Settings Page Comprehensive Testing"
echo "========================================"
echo ""

# Check if server is running
echo "Checking if dev server is running..."
if curl -s http://localhost:5173 > /dev/null 2>&1; then
    echo "✓ Dev server is running at http://localhost:5173"
else
    echo "✗ ERROR: Dev server is NOT running"
    echo ""
    echo "Please start it first with:"
    echo "  cd $WORKDIR/web"
    echo "  bun run dev"
    echo ""
    exit 1
fi

echo ""

# Check if Playwright is available
echo "Checking Playwright installation..."
if node -e "require('@playwright/test')" 2>/dev/null; then
    echo "✓ Playwright is installed"
else
    echo "✗ ERROR: Playwright is not installed"
    echo ""
    echo "Please install it with:"
    echo "  cd $WORKDIR/web"
    echo "  npm install"
    echo ""
    exit 1
fi

echo ""

# Run the simplified QA test
echo "Running QA tests..."
echo ""

node qa-iter3-simple.mjs

EXIT_CODE=$?

echo ""
echo "========================================"
echo "QA TESTING COMPLETE"
echo "========================================"
echo ""

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ All tests passed - all previous issues are FIXED!"
else
    echo "⚠️  Some issues are still present - see report above"
fi

echo ""
echo "Next steps:"
echo "  1. Review screenshots in: web/qa-screenshots-iter3/"
echo "  2. Review JSON report: web/qa-iteration3-report.json"
echo "  3. Create final QA report"
echo ""

exit $EXIT_CODE
