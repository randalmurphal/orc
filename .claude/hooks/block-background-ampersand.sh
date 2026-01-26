#!/bin/bash
#
# Block Background Ampersand Hook for Claude Code
#
# Blocks Bash commands that use `&` for backgrounding ANYWHERE in the command.
# Claude should use `run_in_background: true` parameter instead, which:
# - Returns a task ID for tracking
# - Allows checking output with TaskOutput tool
# - Properly manages the background process
#
# Allowed patterns (not backgrounding):
# - && (logical AND)
# - |& (pipe stderr)
# - &> &>> (redirect both streams)
# - >& (redirect stdout)
# - <& (duplicate input fd)
# - & inside quotes (commit messages, echo, etc.)
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

# Strip quoted content to avoid false positives on strings like "foo & bar"
# This handles the common cases; pathological nesting may slip through
strip_quotes() {
    local str="$1"
    # Remove escaped quotes and ampersands first
    str=$(echo "$str" | sed -E "s/\\\\[\"'&]//g")
    # Remove single-quoted strings (no escaping inside single quotes)
    str=$(echo "$str" | sed -E "s/'[^']*'//g")
    # Remove double-quoted strings
    str=$(echo "$str" | sed -E 's/"[^"]*"//g')
    echo "$str"
}

# Strip quoted content first
unquoted=$(strip_quotes "$command")

# Remove all safe & patterns, then check if any bare & remains
# Safe patterns: && |& &> &>> >& <&
# Order matters: check longer patterns first
safe_removed=$(echo "$unquoted" | sed -E '
    s/&>>//g
    s/&&//g
    s/\|&//g
    s/&>//g
    s/>&//g
    s/<&//g
')

# If any & remains after removing safe patterns, it's a backgrounding &
if [[ "$safe_removed" == *"&"* ]]; then
    cat >&2 << 'EOF'
BLOCKED: Don't use `&` to background processes. Use `"run_in_background": true` parameter instead.

This catches ALL backgrounding, including:
- `cmd &` (trailing)
- `cmd1 & cmd2` (mid-command)
- `(subshell &)` (in subshells)
EOF
    exit 2
fi

exit 0
