#!/bin/bash
# Live QA Test Runner for TASK-614
# Starts dev server if needed and runs Playwright verification

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "════════════════════════════════════════════════════════════════"
echo "  Live QA Testing: Initiatives Page"
echo "  Task: TASK-614"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Function to check if server is responding
check_server() {
    curl -s -o /dev/null -w "%{http_code}" http://localhost:5173 2>/dev/null
}

# Function to wait for server with timeout
wait_for_server() {
    local timeout=60
    local elapsed=0

    echo "⏳ Waiting for dev server to be ready..."
    while [ $elapsed -lt $timeout ]; do
        if [ "$(check_server)" = "200" ]; then
            echo "✓ Dev server is ready!"
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
        echo -n "."
    done

    echo ""
    echo "✗ Server didn't start within $timeout seconds"
    return 1
}

# Check if server is already running
if [ "$(check_server)" = "200" ]; then
    echo "✓ Dev server already running at http://localhost:5173"
    SERVER_WAS_RUNNING=true
else
    echo "⚠ Dev server not running. Starting it now..."
    SERVER_WAS_RUNNING=false

    # Start dev server in background
    cd web
    echo "  Running: npm run dev"
    npm run dev > /tmp/orc-dev-server.log 2>&1 &
    DEV_SERVER_PID=$!
    cd ..

    echo "  Process ID: $DEV_SERVER_PID"
    echo "  Log file: /tmp/orc-dev-server.log"
    echo ""

    # Wait for server to be ready
    if ! wait_for_server; then
        echo ""
        echo "Last 20 lines of server log:"
        tail -20 /tmp/orc-dev-server.log
        exit 1
    fi
fi

echo ""

# Check if playwright is installed
if ! cd web && npx playwright --version &> /dev/null; then
    echo "📦 Installing Playwright browsers..."
    npx playwright install chromium
fi
cd "$SCRIPT_DIR"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  Running QA Verification Script"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Create screenshot directory
mkdir -p /tmp/qa-TASK-614

# Run verification script
if node verify-qa-findings.mjs; then
    TEST_EXIT_CODE=0
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "  ✓ QA Verification Complete"
    echo "════════════════════════════════════════════════════════════════"
else
    TEST_EXIT_CODE=$?
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "  ✗ QA Verification Failed (exit code: $TEST_EXIT_CODE)"
    echo "════════════════════════════════════════════════════════════════"
fi

echo ""
echo "📸 Screenshots saved to: /tmp/qa-TASK-614/"
echo "📋 Reference design: $SCRIPT_DIR/example_ui/initiatives-dashboard.png"

# Cleanup: Stop dev server if we started it
if [ "$SERVER_WAS_RUNNING" = false ] && [ -n "$DEV_SERVER_PID" ]; then
    echo ""
    echo "🛑 Stopping dev server (PID: $DEV_SERVER_PID)..."
    kill $DEV_SERVER_PID 2>/dev/null || true
    # Also kill any child processes
    pkill -P $DEV_SERVER_PID 2>/dev/null || true
fi

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  Test Complete"
echo "════════════════════════════════════════════════════════════════"

exit $TEST_EXIT_CODE
