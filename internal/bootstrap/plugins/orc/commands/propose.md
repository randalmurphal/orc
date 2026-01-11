---
description: "Propose a sub-task for later review"
argument-hint: "TITLE [--description DESC] [--parent TASK-ID]"
---

# Orc Propose Command

Queue a sub-task for review after the current task completes.

## When to Use

During implementation, you may discover:
- Related work that should be done separately
- Refactoring opportunities
- Technical debt to address
- Follow-up features

Instead of scope creep, use `/orc:propose` to queue these for later.

## Usage

Propose a sub-task with title and optional description:

```bash
# Simple proposal
orc propose "Refactor auth module for better testability"

# With description
orc propose "Add rate limiting to API" --description "The API endpoints should have rate limiting to prevent abuse. Consider using token bucket algorithm."

# Linked to parent task
orc propose "Add logging to new feature" --parent TASK-001
```

## What Happens

1. Sub-task is queued in the database
2. Marked as "pending" approval
3. Linked to parent task (if specified)
4. Shown to user after parent task merges
5. User approves/rejects
6. Approved tasks become real tasks

## Queue Status

Check pending sub-tasks:
```bash
orc subtasks list
```

## Configuration

Check `.orc/config.yaml`:
```yaml
subtasks:
  allow_creation: true    # Agents can propose
  auto_approve: false     # Require human approval
  max_pending: 10         # Max queued per task
```

## Output

After proposing:
```xml
<subtask_proposed>
  <id>ST-001</id>
  <title>Refactor auth module for better testability</title>
  <parent>TASK-001</parent>
  <status>pending</status>
</subtask_proposed>
```

Continue with your current work - the sub-task will be reviewed later.

## Best Practices

- Keep proposals focused and specific
- Include enough context for later review
- Don't propose trivial tasks (just do them)
- Link to parent task for context
- Estimate rough effort in description
