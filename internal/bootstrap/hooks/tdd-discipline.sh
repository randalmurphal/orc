#!/bin/bash

# TDD Discipline Hook
# Enforces TDD-first workflow by blocking non-test file writes during tdd_write phase.
# This is a PreToolUse hook that checks the current phase from the orc database.
#
# Required environment variables (set by orc executor):
#   ORC_TASK_ID  - Current task ID
#   ORC_DB_PATH  - Path to project database (.orc/orc.db)
#
# This hook is auto-installed by `orc init` to .claude/hooks/tdd-discipline.sh

set -euo pipefail

# Read hook input from stdin
HOOK_INPUT=$(cat)

# Parse tool name from hook input
TOOL_NAME=$(echo "$HOOK_INPUT" | jq -r '.tool_name // empty')

# Only check file modification tools
case "$TOOL_NAME" in
    Write|Edit|MultiEdit) ;;
    *) exit 0 ;;  # Allow all other tools
esac

# Check if orc context is available
if [[ -z "${ORC_TASK_ID:-}" ]] || [[ -z "${ORC_DB_PATH:-}" ]]; then
    exit 0  # No orc context, allow operation
fi

# Check if database exists
if [[ ! -f "$ORC_DB_PATH" ]]; then
    exit 0  # No database, allow operation
fi

# Check if sqlite3 is available
if ! command -v sqlite3 &> /dev/null; then
    exit 0  # No sqlite3, allow operation
fi

# Validate task ID format (prevent SQL injection)
if [[ ! "$ORC_TASK_ID" =~ ^TASK-[0-9]+$ ]]; then
    exit 0  # Invalid format, allow operation (defensive)
fi

# Query current phase for the task
CURRENT_PHASE=$(sqlite3 "$ORC_DB_PATH" "SELECT current_phase FROM tasks WHERE id = '$ORC_TASK_ID';" 2>/dev/null || echo "")

# Only restrict during tdd_write phase
if [[ "$CURRENT_PHASE" != "tdd_write" ]]; then
    exit 0  # Not TDD phase, allow all operations
fi

# Get file path from tool input
FILE_PATH=$(echo "$HOOK_INPUT" | jq -r '.tool_input.file_path // empty')
if [[ -z "$FILE_PATH" ]]; then
    exit 0  # No file path, allow operation
fi

# Test file patterns (comprehensive coverage for all common test conventions)
# Returns true if the file is a test file or test infrastructure.
# IMPORTANT: This must match the Go implementation in tdd_patterns.go
is_test_file() {
    local file="$1"
    local basename
    basename=$(basename "$file")

    # Test files by naming convention
    # Go, Python, TS, JS, Rust, Ruby: *_test.*
    # JavaScript/TypeScript: *.test.*, *.spec.*
    # Ruby: *_spec.rb
    # Python: test_*.py
    if [[ "$basename" =~ _test\.(go|py|ts|js|tsx|jsx|rs|rb)$ ]] || \
       [[ "$basename" =~ \.test\.(ts|js|tsx|jsx|mjs|cjs)$ ]] || \
       [[ "$basename" =~ \.spec\.(ts|js|tsx|jsx|mjs|cjs|rb)$ ]] || \
       [[ "$basename" =~ _spec\.rb$ ]] || \
       [[ "$basename" =~ ^test_.*\.py$ ]]; then
        return 0
    fi

    # Test directories (match at start of path or after /)
    # tests/, test/, __tests__/, spec/, e2e/, integration/
    if [[ "$file" =~ (^|/)tests?/ ]] || \
       [[ "$file" =~ (^|/)__tests__/ ]] || \
       [[ "$file" =~ (^|/)spec/ ]] || \
       [[ "$file" =~ (^|/)e2e/ ]] || \
       [[ "$file" =~ (^|/)integration/ ]]; then
        return 0
    fi

    # Test infrastructure and configuration
    # Python: conftest.py, pytest.ini
    # JavaScript: jest.config.*, vitest.config.*, playwright.config.*, cypress.config.*
    # Setup files: setupTests.ts, setupTest.js
    if [[ "$basename" == "conftest.py" ]] || \
       [[ "$basename" == "pytest.ini" ]] || \
       [[ "$basename" =~ ^(jest|vitest|playwright|cypress)\.config\. ]] || \
       [[ "$basename" =~ ^setupTests?\.(ts|js|tsx|jsx)$ ]]; then
        return 0
    fi

    # Test data and fixtures directories (match at start of path or after /)
    # fixtures/, testdata/, mocks/, stubs/, fakes/
    if [[ "$file" =~ (^|/)fixtures?/ ]] || \
       [[ "$file" =~ (^|/)testdata/ ]] || \
       [[ "$file" =~ (^|/)mocks?/ ]] || \
       [[ "$file" =~ (^|/)stubs?/ ]] || \
       [[ "$file" =~ (^|/)fakes?/ ]]; then
        return 0
    fi

    # Mock/stub/fake files by convention
    # *.mock.ts, *.stub.go, *.fake.py
    if [[ "$basename" =~ \.(mock|stub|fake)\.(ts|js|go|py|tsx|jsx)$ ]]; then
        return 0
    fi

    return 1
}

# Check if file is a test file
if is_test_file "$FILE_PATH"; then
    exit 0  # Test file, allow operation
fi

# Not a test file during tdd_write phase - block the operation
# Output JSON response for PreToolUse hook
jq -n \
    --arg file "$FILE_PATH" \
    '{
        "decision": "block",
        "reason": ("TDD discipline: During tdd_write phase, only test files can be modified. Blocked: " + $file + "\n\nWrite your tests first, then run `orc run` to continue to the implement phase.")
    }'
