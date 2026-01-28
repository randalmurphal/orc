#!/bin/bash
set -e

echo "Starting QA testing for Agents page..."

cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-613/web

# Create output directory
mkdir -p /tmp/qa-TASK-613

# Run the test script using node with the local node_modules
node ../test-agents-page.mjs

echo "Test complete! Check /tmp/qa-TASK-613/ for screenshots and findings."
