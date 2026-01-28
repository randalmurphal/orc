#!/bin/bash

# Check if server is running
echo "Checking if dev server is running..."
if curl -s http://localhost:5173 > /dev/null 2>&1; then
    echo "✓ Dev server is running"
    echo ""
    cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web
    node run-qa-iteration3.mjs
else
    echo "✗ Dev server is NOT running at http://localhost:5173"
    echo ""
    echo "Please start the dev server first with:"
    echo "  cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web"
    echo "  bun run dev"
    echo ""
    echo "Then run this script again."
    exit 1
fi
