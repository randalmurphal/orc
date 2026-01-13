---
description: "Propose a sub-task for later review"
argument-hint: "TITLE"
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

## How It Works

When you invoke `/orc:propose`, output the sub-task in XML format:

```xml
<subtask_proposal>
  <title>Refactor auth module for better testability</title>
  <description>The auth module has grown complex. Should be refactored to use dependency injection for easier testing.</description>
  <parent_task>TASK-001</parent_task>
</subtask_proposal>
```

The orchestrator will:
1. Parse the XML from your output
2. Queue the sub-task in the database via API
3. Mark it as "pending" approval
4. Link to the parent task
5. Show to user after parent task completes

## Queue Management

Sub-tasks are managed through the orc API:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `POST /api/projects/:id/subtasks` | Create | Queue new sub-task |
| `GET /api/projects/:id/subtasks` | List | List pending sub-tasks |
| `POST /api/subtasks/:id/approve` | Approve | Approve and create task |
| `POST /api/subtasks/:id/reject` | Reject | Reject sub-task |

The web UI provides a visual interface for reviewing and approving queued sub-tasks.

## Configuration

Check `.orc/config.yaml`:
```yaml
subtasks:
  allow_creation: true    # Agents can propose
  auto_approve: false     # Require human approval
  max_pending: 10         # Max queued per task
```

## Output

After outputting the proposal XML, the orchestrator will confirm:
```xml
<subtask_queued>
  <id>ST-001</id>
  <title>Refactor auth module for better testability</title>
  <parent>TASK-001</parent>
  <status>pending</status>
</subtask_queued>
```

Continue with your current work - the sub-task will be reviewed later.

## Best Practices

- Keep proposals focused and specific
- Include enough context for later review
- Don't propose trivial tasks (just do them)
- Always include the parent task ID for context
- Estimate rough effort in the description
