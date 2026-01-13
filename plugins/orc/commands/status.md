---
description: "Show current task status and next steps"
---

# Orc Status Command

Display the current state of orc tasks and recommend next actions.

## Get Status

```bash
# Show running and recent tasks
orc status 2>/dev/null || echo "Orc not initialized. Run: orc init"
```

## Information to Display

### Active Tasks
For each running/paused task, show:
- Task ID and title
- Current phase
- Phase status (running, paused, blocked)
- Iteration count if in ralph loop

### Blocked Tasks
For blocked tasks:
- What's blocking (from state.yaml)
- User questions requiring answers
- Failed gates

### Recent Completed
Last 3 completed tasks with:
- Task ID and title
- Completion time
- Final status

## Recommendations

Based on status, suggest:

**No tasks**:
- "No active tasks. Run `/orc:init` to create one."

**Paused task**:
- "Task TASK-XXX is paused. Run `/orc:continue` to resume."

**Blocked task**:
- "Task TASK-XXX is blocked: [reason]. [Specific action to unblock]"

**Running task**:
- "Task TASK-XXX is running in phase [phase]. Check progress with `/orc:continue`"

**Review needed**:
- "Task TASK-XXX needs review. Run `/orc:review` to start."

## Format Output

Present status in a clear table format:

```
| Task      | Phase     | Status  | Progress |
|-----------|-----------|---------|----------|
| TASK-001  | implement | running | 3/10     |
| TASK-002  | review    | blocked | waiting  |
```

Then provide actionable next steps.
