---
description: "Continue current task from where it left off"
argument-hint: "[TASK-ID]"
---

# Orc Continue Command

Resume work on the current or specified orc task.

## Check Current State

```bash
# Get active/paused tasks
orc status 2>/dev/null
```

## Workflow

### Find the task to continue:
1. If TASK-ID provided, use that task
2. Otherwise, find the most recent in-progress or paused task
3. If no tasks exist, suggest `/orc:init` to create one

### Load task context:
Read the task files to understand current state:
- `.orc/tasks/TASK-XXX/task.yaml` - Task definition
- `.orc/tasks/TASK-XXX/plan.yaml` - Phase sequence
- `.orc/tasks/TASK-XXX/state.yaml` - Execution state (current phase, iterations)
- `.orc/tasks/TASK-XXX/spec.md` - Task specification (if exists)

### Determine current phase:
From state.yaml, identify:
- Current phase (implement, test, review, qa, etc.)
- Phase status (running, paused, blocked)
- Iteration count

### Resume work:
Based on current phase:

**implement**: Continue coding the feature
- Check what's already done in the codebase
- Review any previous transcripts for context
- Continue implementation

**test**: Write and run tests
- Check existing test coverage
- Add missing tests
- Run test suite

**review**: Address review feedback
- Check review findings in state
- Fix identified issues
- Prepare for re-review

**qa**: Complete QA tasks
- Run e2e tests
- Generate documentation
- Validate all requirements met

## Completion Detection

When a phase is complete, output:
```xml
<phase_complete>true</phase_complete>
```

If blocked on something, output:
```xml
<phase_blocked>reason: description of what's blocking</phase_blocked>
```

## Context Files

Always check these files for context:
- `.orc/tasks/TASK-XXX/transcripts/` - Previous conversation logs
- Git history for task branch
- `.orc/config.yaml` - Project settings
