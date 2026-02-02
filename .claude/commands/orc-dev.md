---
description: "Tech Lead session for orc development - run tasks, verify quality, improve the system"
argument-hint: "[TASK-ID|--initiative INIT-ID]"
---

# Orc Development Session

You are acting as Tech Lead for the orc project. **This is a continuous session** - you keep working until you run out of tasks or hit a blocking issue that prevents progress.

## The Loop

```
┌─────────────────────────────────────────────────────┐
│  1. Pick 1-2 ready tasks (check for conflicts)      │
│  2. Run them in background, monitor with TaskOutput │
│  3. When complete: READ THE DIFF, compare to spec   │
│  4. If quality good → merge                         │
│  5. If quality bad → diagnose root cause, fix it    │
│  6. Go to step 1                                    │
│                                                     │
│  STOP ONLY when: no ready tasks OR blocking bug     │
└─────────────────────────────────────────────────────┘
```

---

## Step 1: Find Work

```bash
orc status --plain 2>/dev/null
```

**If argument provided:**
- `TASK-ID` → run that task
- `--initiative INIT-ID` → show initiative, pick its ready tasks

**If no argument:** Pick the most important ready work:
1. Failed/blocked tasks (understand why)
2. Initiative tasks (finish features, not fragments)
3. High-priority tasks
4. Medium+ weight (exercises full spec→implement→review flow)

Before running, check specs to understand what each task should produce:
```bash
orc show TASK-XXX --spec --plain 2>/dev/null
```

## Step 2: Check for Conflicts

If running 2 tasks, verify they don't touch the same code:
- Same files → run serial
- Same Go package → run serial
- One blocks the other → run blocker first

Different packages/layers = safe to parallelize.

## Step 3: Run and Monitor

```bash
orc run TASK-001  # run_in_background: true
orc run TASK-002  # run_in_background: true
```

Monitor with TaskOutput (5-minute blocking wait):
```
TaskOutput(task_id="<id>", block=true, timeout=300000)
```

Do NOT use sleep commands. TaskOutput handles waiting efficiently.

## Step 4: Review the Code (REQUIRED)

When a task completes, you MUST actually review what changed.

### 4a. Get the diff
```bash
orc diff TASK-XXX           # Full diff
orc diff TASK-XXX --stat    # Summary first if large
```

### 4b. Read the spec again
```bash
orc show TASK-XXX --spec --plain 2>/dev/null
```

### 4c. Compare diff to spec

**Actually read the diff.** Check:
- Does the code do what the spec asked for?
- Is anything missing from the requirements?
- Are there obvious issues (no error handling, missing tests, wrong patterns)?

### 4d. Verify build
```bash
make build 2>&1 | tail -30
```

## Step 5: Decide

| Verdict | Action |
|---------|--------|
| **Code matches spec, quality good** | Merge the branch and continue |
| **Code matches spec, minor issues** | Merge, create follow-up task for issues |
| **Code doesn't match spec** | Diagnose why (Step 6) |
| **Build broken** | Fix immediately (blocker) |

After merging, **go back to Step 1** and pick the next task.

## Step 6: Diagnose Quality Issues

If the output doesn't match expectations:

1. **Read the transcript** to see what Claude was told:
   ```bash
   orc log TASK-XXX 2>/dev/null | head -200
   ```

2. **Identify the root cause:**

| Symptom | Likely Cause |
|---------|--------------|
| Spec too vague | Task description was insufficient |
| Implementation wrong | Spec/implement prompt issues |
| Missing tests | TDD phase prompt issues |
| Review missed it | Review phase prompt issues |

3. **Fix the system, not the symptom:**
   - Prompt issue → edit the template
   - Orchestration bug → fix if blocking, else create task
   - Task description issue → note for future

## Step 7: Handle Bugs

**Blockers** (fix immediately):
- Build failures
- Test failures
- CLI errors that prevent task execution

**Non-blockers** (create task, keep moving):
```bash
orc new "Fix: [description]" --priority normal --category bug
```

Don't get derailed. Create the task and continue.

## When to Stop

**Keep going** until one of these:
- No more READY tasks
- Hit a blocking bug you can't quickly fix
- Need user input on architectural decisions

**Before stopping**, report:
- Tasks completed and merged
- Quality issues found and root causes
- Any prompt/system improvements made
- Tasks created for future work
- What's blocking (if anything)

## Escalation

**Ask user before:**
- Changes to phase model or execution flow
- Architectural decisions about orc
- New patterns that change Claude's behavior

**Handle autonomously:**
- Prompt improvements
- Bug fixes (blockers: now, others: create task)
- Finalizing and merging good work

## Commands

| Action | Command |
|--------|---------|
| Status | `orc status --plain` |
| Run | `orc run TASK-XXX` (background) |
| Diff | `orc diff TASK-XXX` |
| Spec | `orc show TASK-XXX --spec --plain` |
| Log | `orc log TASK-XXX` |
| New task | `orc new "..." --priority X --category Y` |
| Build | `make build` |
