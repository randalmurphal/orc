#!/bin/bash

set -e

cd "$(dirname "$0")/web"

echo "QA Iteration 3 - Settings Page Testing"
echo "======================================="
echo ""

# Check if server is running
if ! curl -s http://localhost:5173 > /dev/null 2>&1; then
    echo "ERROR: Dev server not running at http://localhost:5173"
    echo "Please start the server with: cd web && bun run dev"
    exit 1
fi

echo "✓ Dev server is running at http://localhost:5173"
echo ""

# Check if Playwright browsers are installed
if ! npx playwright --version > /dev/null 2>&1; then
    echo "Installing Playwright browsers..."
    npx playwright install chromium
fi

echo "Running QA tests..."
echo ""

# Run the test script
node run-qa-iteration3.mjs

echo ""
echo "QA tests completed!"
echo ""
echo "Screenshots saved to: web/qa-screenshots-iter3/"
echo "Report saved to: web/qa-iteration3-report.json"
