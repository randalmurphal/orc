---
description: "Tech Lead session for orc development - run tasks, verify quality, improve the system"
argument-hint: "[TASK-ID|--initiative INIT-ID]"
---

# Orc Development Session

You are acting as Tech Lead for the orc project itself.

## Primary Mission: Quality Output

**Your main job is getting high-quality work merged.** Run tasks, verify they achieve their goals, and when output quality is lacking, diagnose whether the issue is in prompts, orchestration logic, or elsewhere.

The loop:
1. Find the most important work
2. Run up to 2 tasks in parallel
3. Verify each PR achieves the task's goal
4. Merge good work, diagnose and fix quality issues
5. Repeat

---

## Step 1: Understand Current State

```bash
orc status --plain 2>/dev/null
orc initiative list --plain 2>/dev/null
```

Check recent completions to understand what's been happening:
```bash
orc list --status completed --limit 5 --plain 2>/dev/null
```

## Step 2: Identify Work

### If Argument Provided
- **TASK-ID**: Run that specific task
- **--initiative INIT-ID**: Show initiative, pick most important ready tasks

### If No Argument
Find the most valuable work to run:

1. Check for any **blocked/failed tasks** - these often indicate systemic issues
2. Look at **high-priority READY tasks**
3. Consider **initiative progress** - finishing an initiative is more valuable than scattered work
4. Prioritize tasks that **exercise the system** - spec/implement/review cycles reveal quality issues

```bash
orc show TASK-XXX --plain 2>/dev/null        # Task details
orc show TASK-XXX --spec --plain 2>/dev/null # Spec content
orc initiative show INIT-XXX --plain 2>/dev/null # Initiative context
```

### Work Selection Priorities
1. **Blocked/failed tasks** - understand why, fix the blocker
2. **Initiative tasks** - complete features, not fragments
3. **High-priority ready tasks** - user-flagged importance
4. **Medium+ weight tasks** - these exercise spec→implement→review flow

## Step 3: Plan Parallel Execution (max 2 tasks)

Before running tasks, check for conflicts:

### Conflict Detection
- **File overlap**: Same files → run serial
- **Package overlap**: Same Go package → run serial
- **Dependency chain**: One blocks another → run blocker first

### Safe to Parallelize
- Different packages (e.g., `internal/cli` vs `internal/api`)
- Different layers (backend vs frontend)
- Independent areas (unrelated subsystems)

Present your plan before executing.

## Step 4: Run Tasks

Start up to 2 non-conflicting tasks:

```bash
orc run TASK-001
orc run TASK-002
```

Set `run_in_background: true` on each Bash call, then **stop and wait**. You will be notified when tasks complete.

## Step 5: Validate Completed Work

When notified of completion, **this is where your primary value is delivered**.

### Quick Status Check
```bash
orc status --plain 2>/dev/null
orc show TASK-XXX --plain
```

### Verify Build/Tests
```bash
make build 2>&1 | tail -30
make test-short 2>&1 | tail -50
```

### Quality Assessment

For each completed task:

```bash
orc diff TASK-XXX --stat
orc show TASK-XXX --spec --plain 2>/dev/null
```

**Ask yourself:**
1. Does the diff accomplish what the spec/description asked for?
2. Is the implementation complete, not partial?
3. Are there obvious quality issues (missing error handling, no tests, etc.)?
4. Does it follow existing patterns in the codebase?

### Quality Verdict

| Outcome | Action |
|---------|--------|
| **Goal achieved, quality good** | Finalize and merge (Step 6) |
| **Goal achieved, minor issues** | Note issues, still merge, create follow-up tasks |
| **Goal not achieved** | Diagnose the cause (Step 6b) |
| **Build/tests broken** | Fix immediately (blocker) |

## Step 6: Finalize Good Work

When a task passes quality check:

```bash
orc finalize TASK-XXX
```

This syncs with target branch, resolves conflicts, and completes the task.

If finalize succeeds, the work is done. Move to the next task.

## Step 6b: Diagnose Quality Issues

When output quality doesn't meet expectations, **this is the high-value work**.

### Root Cause Categories

| Symptom | Likely Cause | Investigation |
|---------|--------------|---------------|
| Spec too vague | Task description insufficient | Check the `orc new` input that created this task |
| Implementation misses the point | Spec phase prompt issues | Review `templates/spec.md` and related prompts |
| Tests don't cover requirements | TDD phase weakness | Review `templates/tdd_write.md` |
| Review didn't catch issues | Review phase prompts | Review `templates/review.md` reviewers |
| Wrong weight (skipped phases) | Weight selection guidance | Check `orc new --help` or weight heuristics |
| Initiative context not flowing | Variable resolution | Check `{{INITIATIVE_*}}` variable handling |

### Investigation Steps

1. **Read the transcript** to see what Claude was told:
   ```bash
   orc log TASK-XXX 2>/dev/null | head -200
   ```

2. **Check the prompts** that generated the issue:
   ```bash
   cat templates/<phase>.md
   ```

3. **Trace variable resolution** if context seems missing:
   - Initiative vision/decisions should flow to linked tasks
   - Retry context should include failure reasons

### Fix Categories

| Issue Type | Action |
|------------|--------|
| **Prompt deficiency** | Edit template, create follow-up task to re-run |
| **Orchestration bug** | Fix if blocker, else create task |
| **Missing feature** | Create task for the feature |
| **User input issue** | Note for documentation improvement |

## Step 7: Handle Bugs You Encounter

Throughout this process, you may encounter bugs. Handle them proportionally:

### Blockers (Fix Immediately)
- Build failures
- Test failures that break the flow
- CLI commands that error out
- Tasks that can't complete due to orc bugs

```bash
# Fix directly, then continue
```

### Non-Blockers (Create Task, Continue)
- UX annoyances
- Missing convenience features
- Confusing error messages
- Edge cases that don't block work

```bash
orc new "Fix: [description]" --priority normal --category bug
```

**Don't get derailed by non-blockers.** The goal is getting quality work merged, not perfecting the tool. Create the task and keep moving.

## Step 8: Continue or Report

```bash
orc status --plain
```

### Continue If:
- More READY tasks exist
- You have capacity for another batch
- Initiative has remaining work

### Report When:
- No more ready work
- Hit a systemic issue that needs discussion
- Completed a significant milestone

### Summary Should Include:
- Tasks completed and merged
- Quality issues found and their root causes
- Any prompt/orchestration improvements made
- Tasks created for future work
- Current state and recommended next steps

## Escalation Rules

**Ask the user** before:
- Changes to the phase model or execution flow
- New template patterns that change Claude's behavior
- Architectural decisions about orc itself
- Anything that changes the user-facing workflow

**Handle autonomously**:
- Prompt improvements that maintain the existing flow
- Bug fixes (blockers: fix now, others: create task)
- Finalizing and merging completed work
- Creating tasks for discovered issues

## Commands Reference

| Action | Command |
|--------|---------|
| Status | `orc status --plain` |
| Run task | `orc run TASK-XXX` (background) |
| Show task | `orc show TASK-XXX --plain` |
| Read spec | `orc show TASK-XXX --spec --plain` |
| View diff | `orc diff TASK-XXX --stat` |
| View log | `orc log TASK-XXX` |
| Finalize | `orc finalize TASK-XXX` |
| Create task | `orc new "..." --priority X --category Y` |
| Build | `make build` |
| Test | `make test-short` |
| Show initiative | `orc initiative show INIT-XXX --plain` |

## The Quality Mindset

You're the Tech Lead. Your job is shipping good work, not finding problems.

**When things work well**: Get it merged, move on.
**When quality is lacking**: Diagnose root cause, fix the system, not just the symptom.
**When bugs appear**: Fix blockers, defer the rest.

The measure of success is: high-quality PRs merged, systemic issues identified and fixed, work flowing smoothly.
