#!/bin/bash
# complete-task.sh - Complete an orc task with merge or PR
#
# Usage: complete-task.sh TASK-ID [merge|pr]
#
# Reads config from .orc/config.yaml for:
# - completion.target_branch (default: main)
# - completion.action (default: pr)
# - completion.pr.title
# - completion.pr.labels
# - completion.pr.reviewers
# - completion.pr.auto_merge
# - completion.delete_branch
#
# Requires: yq (for YAML parsing), gh (for GitHub operations)

set -euo pipefail

TASK_ID="${1:-}"
ACTION="${2:-}"

# Validate args
if [[ -z "$TASK_ID" ]]; then
    echo "Usage: complete-task.sh TASK-ID [merge|pr]" >&2
    exit 1
fi

# Find config
CONFIG_FILE=".orc/config.yaml"
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Error: $CONFIG_FILE not found. Run 'orc init' first." >&2
    exit 1
fi

# Read config with defaults
read_config() {
    local key="$1"
    local default="${2:-}"
    local value
    value=$(yq -r "$key // \"$default\"" "$CONFIG_FILE" 2>/dev/null || echo "$default")
    echo "$value"
}

# Config values
TARGET_BRANCH=$(read_config '.completion.target_branch' 'main')
BRANCH_PREFIX=$(read_config '.branch_prefix' 'orc/')
DELETE_BRANCH=$(read_config '.completion.delete_branch' 'true')

# Determine action from config if not specified
if [[ -z "$ACTION" ]]; then
    ACTION=$(read_config '.completion.action' 'pr')
fi

# Task branch name
TASK_BRANCH="${BRANCH_PREFIX}${TASK_ID}"

echo "==> Completing task: $TASK_ID"
echo "    Branch: $TASK_BRANCH"
echo "    Target: $TARGET_BRANCH"
echo "    Action: $ACTION"

# Sync with target branch
echo "==> Syncing with $TARGET_BRANCH..."
git fetch origin "$TARGET_BRANCH"
if ! git rebase "origin/$TARGET_BRANCH"; then
    echo "Error: Rebase failed. Resolve conflicts and try again." >&2
    exit 1
fi

case "$ACTION" in
    merge)
        echo "==> Merging into $TARGET_BRANCH..."
        git checkout "$TARGET_BRANCH"
        git merge --no-ff "$TASK_BRANCH" -m "[orc] Merge $TASK_ID"
        git push origin "$TARGET_BRANCH"

        if [[ "$DELETE_BRANCH" == "true" ]]; then
            echo "==> Deleting task branch..."
            git branch -d "$TASK_BRANCH"
            git push origin --delete "$TASK_BRANCH" 2>/dev/null || true
        fi

        echo "==> Merged successfully"
        ;;

    pr)
        echo "==> Creating pull request..."

        # Push branch first
        git push -u origin "$TASK_BRANCH"

        # Build PR title
        PR_TITLE=$(read_config '.completion.pr.title' '[orc] {{TASK_ID}}')
        PR_TITLE="${PR_TITLE//\{\{TASK_ID\}\}/$TASK_ID}"

        # Build gh command
        GH_ARGS=(pr create --title "$PR_TITLE" --base "$TARGET_BRANCH" --head "$TASK_BRANCH")

        # Add labels
        LABELS=$(read_config '.completion.pr.labels' '[]')
        if [[ "$LABELS" != "[]" && "$LABELS" != "null" ]]; then
            while IFS= read -r label; do
                [[ -n "$label" ]] && GH_ARGS+=(--label "$label")
            done < <(echo "$LABELS" | yq -r '.[]' 2>/dev/null || true)
        fi

        # Add reviewers
        REVIEWERS=$(read_config '.completion.pr.reviewers' '[]')
        if [[ "$REVIEWERS" != "[]" && "$REVIEWERS" != "null" ]]; then
            while IFS= read -r reviewer; do
                [[ -n "$reviewer" ]] && GH_ARGS+=(--reviewer "$reviewer")
            done < <(echo "$REVIEWERS" | yq -r '.[]' 2>/dev/null || true)
        fi

        # Draft mode
        DRAFT=$(read_config '.completion.pr.draft' 'false')
        [[ "$DRAFT" == "true" ]] && GH_ARGS+=(--draft)

        # Create PR body
        PR_BODY="## Task: $TASK_ID

This PR was created by orc orchestration.

### Changes
$(git log "origin/$TARGET_BRANCH..HEAD" --oneline)

---
*Automated by [orc](https://github.com/randalmurphal/orc)*"

        GH_ARGS+=(--body "$PR_BODY")

        # Create PR
        PR_URL=$(gh "${GH_ARGS[@]}")
        echo "==> Created PR: $PR_URL"

        # Enable auto-merge if configured
        AUTO_MERGE=$(read_config '.completion.pr.auto_merge' 'false')
        if [[ "$AUTO_MERGE" == "true" ]]; then
            echo "==> Enabling auto-merge..."
            gh pr merge "$PR_URL" --auto --merge || echo "Warning: Could not enable auto-merge"
        fi
        ;;

    none)
        echo "==> No completion action (none)"
        ;;

    *)
        echo "Error: Unknown action '$ACTION'. Use 'merge', 'pr', or 'none'." >&2
        exit 1
        ;;
esac

echo "==> Task $TASK_ID completion finished"
