#!/bin/bash

WORKTREE_DIR="/home/randy/repos/orc/.orc/worktrees/orc-TASK-613"

echo "Checking if dev server is running on http://localhost:5173..."

if timeout 5 bash -c 'cat < /dev/null > /dev/tcp/localhost/5173' 2>/dev/null; then
    echo "✓ Dev server is running"
    echo ""
    echo "Starting verification test..."
    cd "$WORKTREE_DIR"
    node verify-agents-page.mjs
    exit_code=$?

    if [ $exit_code -eq 0 ]; then
        echo ""
        echo "=== Verification Complete ==="
        echo ""
        echo "View screenshots:"
        echo "  - agents-page-desktop-full.png"
        echo "  - agents-page-mobile-full.png"
        echo ""
        echo "View report:"
        echo "  - verification-report.json"
    fi

    exit $exit_code
else
    echo "✗ Dev server is NOT running on port 5173"
    echo ""
    echo "Please start the dev server first with one of these commands:"
    echo ""
    echo "  Option 1 (from worktree root):"
    echo "    cd $WORKTREE_DIR && make web-dev"
    echo ""
    echo "  Option 2 (from web directory):"
    echo "    cd $WORKTREE_DIR/web && npm run dev"
    echo ""
    echo "  Option 3 (both API + frontend):"
    echo "    cd $WORKTREE_DIR && make dev-full"
    echo ""
    exit 1
fi
