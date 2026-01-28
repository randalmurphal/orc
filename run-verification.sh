#!/bin/bash

set -e

WORKTREE_DIR="/home/randy/repos/orc/.orc/worktrees/orc-TASK-613"
WEB_DIR="$WORKTREE_DIR/web"

echo "=== Agents Page Verification Test ==="
echo ""

# Check if dev server is already running
if curl -s http://localhost:5173 > /dev/null 2>&1; then
    echo "✓ Dev server already running on port 5173"
else
    echo "✗ Dev server not running on port 5173"
    echo ""
    echo "Please start the dev server first:"
    echo "  cd $WEB_DIR && npm run dev"
    echo ""
    echo "Or run from the worktree root:"
    echo "  cd $WORKTREE_DIR && make dev-web"
    exit 1
fi

echo ""
echo "Running verification test..."
echo ""

cd "$WORKTREE_DIR"
node verify-agents-page.mjs

echo ""
echo "=== Test Complete ==="
echo ""
echo "Screenshots saved:"
echo "  - agents-page-desktop-full.png"
echo "  - agents-page-mobile-full.png"
echo ""
echo "Report saved:"
echo "  - verification-report.json"
