#!/bin/bash
set -e

echo "🧪 TASK-613 Comprehensive QA Test - Iteration 2"
echo "=============================================="
echo ""

# Check if dev server is running
echo "Checking dev server..."
if curl -s -f http://localhost:5173 > /dev/null 2>&1; then
    echo "✅ Dev server is running at http://localhost:5173"
else
    echo "❌ Dev server is not responding"
    echo ""
    echo "Please start the dev server first:"
    echo "  cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web"
    echo "  bun run dev"
    echo ""
    exit 1
fi

echo ""
echo "Running comprehensive E2E tests..."
echo ""

# Run the test
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613
node comprehensive-qa-test.mjs

echo ""
echo "QA test complete!"
echo ""
echo "Results saved to: /tmp/qa-TASK-613/"
echo "View findings: /tmp/qa-TASK-613/qa-findings.json"
