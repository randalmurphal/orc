#!/bin/bash
# Run QA test script for Initiatives page and capture output

set -e

cd "$(dirname "$0")/web"

echo "Running QA test for Initiatives page..."
echo "========================================"
echo ""

# Run the test script
node qa-initiatives-test.mjs 2>&1 | tee ../qa-test-output.txt

echo ""
echo "Test complete! Output saved to qa-test-output.txt"
echo "Screenshots saved to qa-screenshots/"
