#!/bin/bash

# Orc Stop Hook
# Implements ralph-style iteration loops for orc tasks running in worktrees.
# Only activates when CWD is within an orc worktree (.orc/worktrees/orc-*).
#
# This hook is auto-installed by `orc init` to .claude/hooks/orc-stop.sh

set -euo pipefail

# Read hook input from stdin (advanced stop hook API)
HOOK_INPUT=$(cat)

# Check if we're in an orc worktree
# Supports both new global location (~/.orc/worktrees/*/orc-*) and legacy (.orc/worktrees/orc-*)
CWD=$(pwd)
if [[ "$CWD" =~ \.orc/worktrees/orc- ]] || [[ "$CWD" =~ "$HOME/.orc/worktrees/".*/orc- ]]; then
    : # In an orc worktree, continue
else
    # Not an orc worktree - allow normal exit
    exit 0
fi

# Check for ralph state in this worktree
RALPH_STATE_FILE=".claude/orc-ralph.local.md"

if [[ ! -f "$RALPH_STATE_FILE" ]]; then
    # No active loop - allow exit
    exit 0
fi

# Parse markdown frontmatter (YAML between ---) and extract values
FRONTMATTER=$(sed -n '/^---$/,/^---$/{ /^---$/d; p; }' "$RALPH_STATE_FILE")
ITERATION=$(echo "$FRONTMATTER" | grep '^iteration:' | sed 's/iteration: *//')
MAX_ITERATIONS=$(echo "$FRONTMATTER" | grep '^max_iterations:' | sed 's/max_iterations: *//')
# Extract completion_promise and strip surrounding quotes if present
COMPLETION_PROMISE=$(echo "$FRONTMATTER" | grep '^completion_promise:' | sed 's/completion_promise: *//' | sed 's/^"\(.*\)"$/\1/')

# Validate numeric fields before arithmetic operations
if [[ ! "$ITERATION" =~ ^[0-9]+$ ]]; then
    echo "orc: Ralph loop state corrupted (invalid iteration: '$ITERATION')" >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

if [[ ! "$MAX_ITERATIONS" =~ ^[0-9]+$ ]]; then
    echo "orc: Ralph loop state corrupted (invalid max_iterations: '$MAX_ITERATIONS')" >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

# Check if max iterations reached
if [[ $MAX_ITERATIONS -gt 0 ]] && [[ $ITERATION -ge $MAX_ITERATIONS ]]; then
    echo "orc: Max iterations ($MAX_ITERATIONS) reached for this phase."
    rm "$RALPH_STATE_FILE"
    exit 0
fi

# Get transcript path from hook input
TRANSCRIPT_PATH=$(echo "$HOOK_INPUT" | jq -r '.transcript_path')

if [[ ! -f "$TRANSCRIPT_PATH" ]]; then
    echo "orc: Transcript file not found, stopping loop." >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

# Read last assistant message from transcript (JSONL format - one JSON per line)
if ! grep -q '"role":"assistant"' "$TRANSCRIPT_PATH"; then
    echo "orc: No assistant messages found in transcript." >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

# Extract last assistant message
LAST_LINE=$(grep '"role":"assistant"' "$TRANSCRIPT_PATH" | tail -1)
if [[ -z "$LAST_LINE" ]]; then
    echo "orc: Failed to extract last assistant message." >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

# Parse JSON with error handling
LAST_OUTPUT=$(echo "$LAST_LINE" | jq -r '
  .message.content |
  map(select(.type == "text")) |
  map(.text) |
  join("\n")
' 2>&1)

if [[ $? -ne 0 ]]; then
    echo "orc: Failed to parse assistant message JSON: $LAST_OUTPUT" >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

if [[ -z "$LAST_OUTPUT" ]]; then
    echo "orc: Assistant message contained no text content." >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

# Check for completion promise (only if set)
if [[ "$COMPLETION_PROMISE" != "null" ]] && [[ -n "$COMPLETION_PROMISE" ]]; then
    # Extract text from <promise> tags using Perl for multiline support
    PROMISE_TEXT=$(echo "$LAST_OUTPUT" | perl -0777 -pe 's/.*?<promise>(.*?)<\/promise>.*/$1/s; s/^\s+|\s+$//g; s/\s+/ /g' 2>/dev/null || echo "")

    # Use = for literal string comparison (not pattern matching)
    if [[ -n "$PROMISE_TEXT" ]] && [[ "$PROMISE_TEXT" = "$COMPLETION_PROMISE" ]]; then
        echo "orc: Phase complete (<promise>$COMPLETION_PROMISE</promise> detected)."
        rm "$RALPH_STATE_FILE"
        exit 0
    fi
fi

# Not complete - continue loop with SAME PROMPT
NEXT_ITERATION=$((ITERATION + 1))

# Extract prompt (everything after the closing ---)
# Skip first --- line, skip until second --- line, then print everything after
PROMPT_TEXT=$(awk '/^---$/{i++; next} i>=2' "$RALPH_STATE_FILE")

if [[ -z "$PROMPT_TEXT" ]]; then
    echo "orc: State file corrupted (no prompt text found)." >&2
    rm "$RALPH_STATE_FILE"
    exit 0
fi

# Update iteration in frontmatter
TEMP_FILE="${RALPH_STATE_FILE}.tmp.$$"
sed "s/^iteration: .*/iteration: $NEXT_ITERATION/" "$RALPH_STATE_FILE" > "$TEMP_FILE"
mv "$TEMP_FILE" "$RALPH_STATE_FILE"

# Build system message with iteration count and completion promise info
if [[ "$COMPLETION_PROMISE" != "null" ]] && [[ -n "$COMPLETION_PROMISE" ]]; then
    SYSTEM_MSG="orc iteration $NEXT_ITERATION/$MAX_ITERATIONS | Complete when: <promise>$COMPLETION_PROMISE</promise>"
else
    SYSTEM_MSG="orc iteration $NEXT_ITERATION/$MAX_ITERATIONS | No completion promise set"
fi

# Output JSON to block the stop and feed prompt back
jq -n \
    --arg prompt "$PROMPT_TEXT" \
    --arg msg "$SYSTEM_MSG" \
    '{
        "decision": "block",
        "reason": $prompt,
        "systemMessage": $msg
    }'

exit 0
