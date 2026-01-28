#!/bin/bash

set -e

WORKTREE_DIR="/home/randy/repos/orc/.orc/worktrees/orc-TASK-613"
WEB_DIR="$WORKTREE_DIR/web"

echo "=== Agents Page QA Verification ==="
echo ""

# Check if dev server is running
echo "Checking if dev server is running on http://localhost:5173..."
if timeout 3 bash -c 'cat < /dev/null > /dev/tcp/localhost/5173' 2>/dev/null; then
    echo "✓ Dev server is running"
else
    echo "✗ Dev server is NOT running"
    echo ""
    echo "Please start the dev server first:"
    echo "  cd $WEB_DIR && npm run dev"
    echo ""
    echo "Or from worktree root:"
    echo "  cd $WORKTREE_DIR && make web-dev"
    exit 1
fi

echo ""
echo "Running QA verification..."
echo ""

# Run the verification script
cd "$WEB_DIR"
node verify-agents-qa.mjs

exit_code=$?

if [ $exit_code -eq 0 ]; then
    echo ""
    echo "========================================="
    echo "  ALL ORIGINAL ISSUES HAVE BEEN FIXED!"
    echo "========================================="
else
    echo ""
    echo "========================================="
    echo "  SOME ISSUES STILL PRESENT"
    echo "========================================="
fi

exit $exit_code
