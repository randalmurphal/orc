---
description: "Initialize orc in project or start new spec/task"
argument-hint: "[DESCRIPTION] [--quick]"
---

# Orc Init Command

Initialize orc orchestration in this project or start a new task.

## Check Current State

First, check if orc is initialized and what tasks exist:

```bash
# Check if .orc directory exists
if [ -d ".orc" ]; then
  echo "Orc is initialized. Checking for active tasks..."
  orc status 2>/dev/null || echo "No active tasks."
else
  echo "Orc not initialized. Run: orc init"
fi
```

## Workflow

### If orc is not initialized:
1. Run `orc init` to initialize the project
2. This creates `.orc/` directory, config, and database

### If orc is initialized and no arguments provided:
Guide the user through creating a new spec:
1. Ask what they want to build
2. Explore the codebase to understand patterns
3. Consider architectural implications
4. Create spike code in `/tmp` if needed to validate assumptions
5. Draft a spec with task breakdown
6. Create tasks using `orc new "task title"`

### If --quick flag or simple description provided:
1. Create a single task directly: `orc new "<description>"`
2. Start execution: `orc run TASK-XXX`

## Context Injection

When working on orc tasks, always check:
- `.orc/tasks/` for existing tasks
- `.orc/config.yaml` for project configuration
- Current git branch and worktree status

## Output

After initialization or task creation, provide next steps:
- For new project: "Run `/orc:continue` to start working on tasks"
- For new task: "Task TASK-XXX created. Run `orc run TASK-XXX` or `/orc:continue`"
