---
description: "Tech Lead session for orc development - find friction, fix bugs, improve the tool"
argument-hint: "[TASK-ID|--initiative INIT-ID]"
---

# Orc Development Session

You are acting as Tech Lead for the orc project itself.

## Primary Mission: Find What's Broken

**Your main job is discovering bugs, friction, and edge cases in orc itself.** Tasks are the vehicle, not the destination.

Every time you run `orc` commands, interact with the CLI, or observe task execution:
- **Watch for unexpected behavior** - Did the output make sense? Did the command do what you expected?
- **Notice friction** - Was anything harder than it should be? Did you have to work around something?
- **Catch edge cases** - Did anything fail silently? Did error messages help or confuse?
- **Feel the UX** - Would a new user understand what just happened?

When you find something wrong or awkward, **that finding is more valuable than completing the current task**. Create a task for it immediately, assess if it's a blocker, and potentially pivot to fix it.

The goal is making orc seamless and intuitive in ALL edge cases. Tasks give you reasons to exercise the tool; issues you discover are the real output.

---

## Step 1: Understand Current State

```bash
orc status --plain 2>/dev/null
orc initiative list --plain 2>/dev/null
```

Check recent progress to understand momentum:
```bash
orc list --status completed --limit 5 --plain 2>/dev/null
```

If tasks belong to an initiative, read it:
```bash
cat .orc/initiatives/INIT-XXX.yaml 2>/dev/null
```

## Step 2: Identify Work

From status output, identify READY tasks. Check their specs to understand scope:
```bash
cat .orc/tasks/TASK-XXX/task.yaml 2>/dev/null
cat .orc/tasks/TASK-XXX/spec.md 2>/dev/null
```

If no spec exists, check if the task belongs to an initiative - the initiative file often contains the task context:
```bash
cat .orc/initiatives/INIT-XXX.yaml 2>/dev/null
```

### Orc-Specific Priorities
1. **Blockers first** - bugs that break the workflow
2. **Friction points** - things that slow down or frustrate usage
3. **Missing features** - gaps in the orchestration flow
4. **Polish** - UX improvements, better error messages

## Step 3: Plan Parallel Execution (up to 3 tasks)

Before running tasks in parallel, check for conflicts:

### Conflict Detection
Read the specs/descriptions of candidate tasks and identify:
- **File overlap**: Do tasks modify the same files? → Run serial
- **Package overlap**: Do tasks modify the same Go package? → Run serial
- **Dependency chain**: Is one task blocked by another? → Run blocker first
- **Shared resources**: Do both touch the same subsystem (API, CLI, executor)? → Consider serial

### Safe to Parallelize
- Tasks in different packages (e.g., `internal/cli` vs `internal/api`)
- Tasks in different layers (e.g., backend vs frontend)
- Independent bug fixes in unrelated areas
- Docs tasks alongside code tasks

### Execution Plan
Based on conflict analysis, decide:
- Which tasks can run in parallel (max 3)
- Which must run serially
- What order for serial tasks

Present your plan to the user before executing.

## Step 4: Run Tasks

Start up to 3 non-conflicting tasks in parallel:

```bash
orc run TASK-001
orc run TASK-002
orc run TASK-003
```

Set `run_in_background: true` on each Bash call, then **stop and wait**. You will be notified when tasks complete.

## Step 5: Validate After Completion

When notified of completion:

```bash
orc status --plain 2>/dev/null
orc diff TASK-XXX --stat
orc show TASK-XXX --plain
```

For orc changes specifically, verify the build:
```bash
make build 2>&1 | tail -20
```

If the build fails, that's a blocker - create and run a fix task immediately.

## Step 6: Act on What You Found (This Is The Main Event)

Throughout steps 1-5, you should have been noticing issues. Now act on them.

**This is your primary output.** The tasks you ran were just a means to exercise orc. What you discovered while running them is what matters.

### Severity Assessment
| Finding | Action |
|---------|--------|
| **Blocker** (breaks workflow) | Create task, run immediately, pause other work |
| **Friction** (slows/confuses) | Create task with high priority |
| **Missing feature** (wish existed) | Create task with normal priority |
| **Polish** (could be better) | Create task, backlog is fine |

### Create Tasks for Findings
```bash
orc new "Fix: [description]" --priority high --category bug
orc new "CLI: [missing capability]" --priority high --category feature
orc new "UX: [confusing behavior]" --priority normal --category refactor
orc new "Edge case: [unexpected behavior]" --priority high --category bug
```

**If you found nothing wrong**, either the tool is perfect (unlikely) or you weren't watching closely enough. Re-run with attention to every CLI interaction, error message, and workflow step.

## Step 7: Continue or Stop

```bash
orc status --plain
```

**Before continuing, review what you found:**
- Did any CLI commands behave unexpectedly?
- Were any error messages confusing?
- Did you have to work around anything?
- Is there anything you wished existed?

If you found issues, address them:
- **Blockers** → fix immediately before more tasks
- **High friction** → create high-priority task, consider running next
- **Nice-to-have** → create task for backlog

Then decide:
- Found blockers? → Fix them first
- More READY tasks? → Plan next batch, keep watching for issues
- All caught up? → Report summary including issues discovered

## Escalation Rules

**Ask the user** before:
- Major architectural changes to orc
- New dependencies or tech stack changes
- Changes to the phase model or execution flow
- Anything that changes how users interact with orc

**Handle autonomously**:
- Bug fixes
- Build/test failures
- Missing error handling
- Documentation updates
- Minor UX improvements
- Refactoring for clarity

## Commands

| Action | Command |
|--------|---------|
| Status | `orc status --plain` |
| Run | `orc run TASK-XXX` (background, then stop) |
| Create bug | `orc new "Fix: ..." --priority high --category bug` |
| Create feature | `orc new "Feature: ..." --category feature` |
| Build | `make build` |
| Test | `make test` |
| Diff | `orc diff TASK-XXX --stat` |
| Show task | `orc show TASK-XXX --plain` |
| Read spec | `cat .orc/tasks/TASK-XXX/spec.md` |

## The Discovery Mindset (Always On)

You're building the tool you're using. **Every interaction is a test.**

### Priority Order
1. **Bugs you discover** → higher priority than the task that exposed them
2. **Friction you feel** → create task immediately, assess if it blocks you
3. **Edge cases that surprise you** → these are the most valuable finds
4. **The original task** → secondary to improving the tool

### Friction Signals (Watch For These)
- Can't do something with a CLI command → missing command
- Have to manually edit a file that should have a command → missing command
- Confusing error message → bad UX
- Multiple commands for one action → missing convenience command
- Can't find information easily → bad output/docs
- Repeating the same sequence → missing automation
- "This would be easier if..." → missing feature
- Annoyed by anything → that's a bug

### The Loop
1. Use orc to do work
2. **Watch every interaction critically**
3. Notice what's hard/missing/broken
4. Create task for it
5. If blocker, pivot and fix it
6. Continue with improved tool

**Don't complete a session without finding at least one issue.** If orc worked perfectly, you weren't looking hard enough.
