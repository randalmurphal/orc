#!/bin/bash
#
# Block Background Ampersand Hook for Claude Code
#
# Blocks Bash commands that end with a bare `&` for backgrounding.
# Claude should use `run_in_background: true` parameter instead, which:
# - Returns a task ID for tracking
# - Allows checking output with TaskOutput tool
# - Properly manages the background process
#
# Exit codes:
# - 0: Allow tool execution
# - 2: Block tool execution (stderr message shown to Claude)

# Read JSON from stdin
input=$(cat)

# Extract tool name
tool_name=$(echo "$input" | jq -r '.tool_name // empty')

# Only check Bash tool
if [[ "$tool_name" != "Bash" ]]; then
    exit 0
fi

# Extract command
command=$(echo "$input" | jq -r '.tool_input.command // empty')

if [[ -z "$command" ]]; then
    exit 0
fi

# Check if command ends with bare `&` (not `&&` or `|&`)
# Trim trailing whitespace first, then check
trimmed=$(echo "$command" | sed 's/[[:space:]]*$//')

# Match: ends with & but not && or |&
if [[ "$trimmed" =~ \&$ ]] && [[ ! "$trimmed" =~ \&\&$ ]] && [[ ! "$trimmed" =~ \|\&$ ]]; then
    cat >&2 << 'EOF'
BLOCKED: Don't use `&` to background. Use `"run_in_background": true` instead.
EOF
    exit 2
fi

exit 0
