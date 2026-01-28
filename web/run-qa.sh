#!/bin/bash
set -e

echo "🧪 Starting QA Test for Agents Page"
echo "===================================="
echo ""

# Check if dev server is running
echo "Checking dev server..."
if curl -s -f http://localhost:5173 > /dev/null 2>&1; then
    echo "✓ Dev server is running at http://localhost:5173"
else
    echo "❌ Dev server is not responding"
    echo "Please start the dev server with: bun run dev"
    exit 1
fi

echo ""
echo "Running Playwright QA tests..."
echo ""

# Run the test
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web
node qa-agents-test.mjs

echo ""
echo "QA test complete!"
