#!/bin/bash
# Debug timeout issue on Initiatives page

set -e

cd "$(dirname "$0")"

echo "╔═══════════════════════════════════════════════════════╗"
echo "║  Starting Timeout Investigation                       ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# Check if server is responding
echo "Checking if dev server is running..."
if curl -s --head http://localhost:5173 > /dev/null; then
    echo "✓ Server is responding at http://localhost:5173"
else
    echo "✗ Server is NOT responding"
    echo ""
    echo "Please start the dev server first:"
    echo "  cd web && bun run dev"
    echo ""
    exit 1
fi

echo ""
echo "Running debug script..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

cd web
node debug-timeout.mjs 2>&1 | tee ../debug-output.txt

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✓ Investigation complete!"
echo ""
echo "Output saved to: debug-output.txt"
echo "Screenshots saved to: qa-screenshots-debug/"
echo ""
