# Orc QA Validation - Ralph Loop Prompt

## Objective

Validate the orc orchestrator by **actually using it** as a new user would. Find and fix all UX issues, platform design issues, and bugs through real usage - not code review.

**Core Loop**: Use orc to orchestrate real tasks on a test project. Every failure, confusion, or friction point is a bug to fix.

---

## Ralph Loop Invocation

Start this prompt with the ralph-loop plugin:

```bash
/ralph-loop --completion-promise 'QA VALIDATION COMPLETE' --max-iterations 50 < ralph_prompt.md
```

Or manually:
```bash
cd ~/repos/orc
cat ralph_prompt.md | claude --print -
```

**Completion Promise**: `QA VALIDATION COMPLETE`

To exit the loop, output ONLY when genuinely true:
```xml
<promise>QA VALIDATION COMPLETE</promise>
```

**CRITICAL**: Do NOT output the promise tag unless ALL completion criteria are met. The loop is designed to continue until genuine completion. Do not lie to escape.

---

## The Ralph Methodology

**Core Insight**: The prompt never changes - the filesystem does. Each iteration reads the same instructions but operates on evolved state.

```
┌─────────────────────────────────────────┐
│           ralph_prompt.md               │
│  - Stable goals                         │
│  - Fixed completion criteria            │
│  - Self-correction rules                │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│         Ralph Loop Iteration            │
│                                         │
│  1. Read prompt (this file)             │
│  2. Check filesystem state              │
│  3. Continue from where you left off    │
│  4. Make progress                       │
│  5. Exit → Loop feeds prompt again      │
│                                         │
│  State lives in:                        │
│  - .orc-qa/PROGRESS.md                  │
│  - .orc-qa/phase*.md (issues)           │
│  - Git commits (checkpoints)            │
└─────────────────────────────────────────┘
```

---

## Test Project Setup

Use `~/repos/forex-platform` as the test project for orc validation.

### Before Each Session

```bash
# 1. Ensure orc binary is current
cd ~/repos/orc && go build -o bin/orc ./cmd/orc

# 2. Check test project state
cd ~/repos/forex-platform
ls -la .orc/ 2>/dev/null || echo "Not initialized"

# 3. If testing fresh install, clean up
rm -rf .orc/ .claude/settings.json 2>/dev/null
```

### Fresh Start Protocol

When testing the "new user experience":
```bash
cd ~/repos/forex-platform
rm -rf .orc/
~/repos/orc/bin/orc init
```

---

## Validation Phases

Complete these in order. Each phase must pass before proceeding.

### Phase 1: Installation & Init

**Test Cases:**
- [ ] `orc init` creates `.orc/` directory structure
- [ ] `orc init` detects Go project (go.mod present)
- [ ] `orc init` creates valid `config.yaml`
- [ ] `orc init --quick` works without prompts
- [ ] `orc setup` launches interactive Claude session
- [ ] Running `orc init` twice warns about existing initialization
- [ ] Project appears in global registry (`~/.orc/projects.yaml`)
- [ ] `orc projects` lists the initialized project

**Log Issues To:** `.orc-qa/phase1-init.md`

---

### Phase 2: Task Creation

**Test Cases:**
- [ ] `orc new "Add health check endpoint"` creates task
- [ ] Task ID format is correct (TASK-NNN)
- [ ] Task files created: `task.yaml`, `plan.yaml`
- [ ] Weight classification runs (or uses default)
- [ ] `orc new --weight trivial "Quick fix"` bypasses classification
- [ ] `orc list` shows created tasks
- [ ] `orc show TASK-001` displays task details
- [ ] `orc delete TASK-001` removes task cleanly
- [ ] Creating task with same title works (unique IDs)

**Log Issues To:** `.orc-qa/phase2-tasks.md`

---

### Phase 3: Task Execution

**This is the critical path. Actually run tasks and observe behavior.**

**Test Cases:**
- [ ] `orc run TASK-001` starts execution
- [ ] Progress output shows phase transitions
- [ ] Transcript files created in `transcripts/`
- [ ] `state.yaml` updates during execution
- [ ] `orc pause TASK-001` stops execution gracefully
- [ ] `orc resume TASK-001` continues from pause point
- [ ] `orc stop TASK-001` terminates immediately
- [ ] Git branch created with task prefix
- [ ] Commits made at phase completion
- [ ] `orc status` shows running task
- [ ] `orc log TASK-001` shows transcripts

**Execution Scenarios to Test:**
1. **Trivial task** - Single phase, completes quickly
2. **Small task** - implement + test phases
3. **Task that fails** - Verify retry behavior
4. **Task that gets stuck** - Verify stuck detection

**Log Issues To:** `.orc-qa/phase3-execution.md`

---

### Phase 4: Web UI

**Start the servers and test the dashboard.**

```bash
# Terminal 1: API server
cd ~/repos/orc && ./bin/orc serve

# Terminal 2: Frontend (if dev mode)
cd ~/repos/orc/web && bun dev
```

**Test Cases:**
- [ ] Dashboard loads at `http://localhost:5173` (or configured port)
- [ ] Project dropdown shows test project
- [ ] Task list displays existing tasks
- [ ] Task creation modal works
- [ ] Task details page shows phases/transcripts
- [ ] Run button starts task execution
- [ ] Pause/Resume buttons work
- [ ] WebSocket updates show real-time progress
- [ ] Keyboard shortcuts work (Cmd+K, j/k, etc.)
- [ ] Mobile responsive layout functions

**Log Issues To:** `.orc-qa/phase4-webui.md`

---

### Phase 5: Advanced Features

**Test Cases:**
- [ ] `orc config show` displays configuration
- [ ] `orc config profile safe` changes automation level
- [ ] `orc template list` shows templates
- [ ] `orc initiative new "Feature X"` creates initiative
- [ ] `orc initiative add-task INIT-001 TASK-001` links task
- [ ] `orc cost` shows token usage
- [ ] `orc export TASK-001` produces valid YAML
- [ ] `orc import exported.yaml` recreates task
- [ ] `orc diff TASK-001` shows git changes
- [ ] `orc rewind TASK-001 --to implement` resets state

**Log Issues To:** `.orc-qa/phase5-advanced.md`

---

### Phase 6: Error Handling

**Deliberately trigger errors and verify UX.**

**Test Cases:**
- [ ] `orc run NONEXISTENT` shows helpful error
- [ ] Running without init shows "run orc init first"
- [ ] Invalid config.yaml shows clear parse error
- [ ] Network timeout shows retry suggestions
- [ ] Disk full scenario handled gracefully
- [ ] Concurrent run attempts detected
- [ ] Ctrl+C during execution cleans up properly

**Log Issues To:** `.orc-qa/phase6-errors.md`

---

## Issue Logging Format

Create `.orc-qa/` directory in the orc repo to track issues:

```markdown
# Phase N: [Name] Issues

## Issue: [Short Title]

**Severity**: Critical | High | Medium | Low
**Status**: Open | Fixed | Won't Fix

**Steps to Reproduce:**
1. ...
2. ...
3. ...

**Expected Behavior:**
...

**Actual Behavior:**
...

**Error Output:**
```
<paste error>
```

**Fix Applied:**
- [ ] Code change: `path/to/file.go:line`
- [ ] Commit: `abc123`

---
```

---

## Fix Protocol

When you find an issue:

1. **Log it** - Add to appropriate phase file in `.orc-qa/`
2. **Categorize severity**:
   - **Critical**: Blocks core functionality
   - **High**: Major UX friction, data loss risk
   - **Medium**: Annoying but workaround exists
   - **Low**: Cosmetic, minor polish
3. **Fix Critical/High immediately** before continuing
4. **Batch Medium/Low** for later unless trivial

### After Each Fix

```bash
# Rebuild
cd ~/repos/orc && go build -o bin/orc ./cmd/orc

# Verify fix
[reproduce the issue - should now work]

# Commit
git add . && git commit -m "[orc] Fix: [description]"
```

---

## Completion Criteria

**ALL must be true before outputting `<promise>QA VALIDATION COMPLETE</promise>`:**

### Functional Completeness
- [ ] All Phase 1-6 test cases pass
- [ ] Zero Critical issues open
- [ ] Zero High issues open
- [ ] All Medium issues logged (fix optional)

### Clean Run Verification
- [ ] Fresh `orc init` works on forex-platform
- [ ] Create task, run it, completes successfully
- [ ] Web UI displays task correctly
- [ ] No unexpected errors in any flow

### Code Quality
- [ ] `go test ./...` passes
- [ ] No panics during any test scenario
- [ ] All fixes committed with descriptive messages

### Documentation
- [ ] CLAUDE.md reflects any new behaviors
- [ ] Error messages are actionable
- [ ] `.orc-qa/SUMMARY.md` lists all issues found/fixed

---

## Self-Correction Rules

### If a Test Fails
1. Log the issue immediately
2. Assess severity
3. If Critical/High: fix before continuing
4. If Medium/Low: log and continue

### If Stuck on Same Issue 3+ Times
1. Write detailed analysis to `.orc-qa/.stuck.md`
2. Try alternative approach
3. If still stuck, skip and document as "Needs Review"

### If Blocked by External Factor
1. Document in `.orc-qa/.blocked.md`
2. Continue with other test phases
3. Return when unblocked

### DO NOT LIE TO EXIT
Even if you:
- Think you're stuck
- Believe the task is impossible
- Have been running too long
- Want to exit for any reason

You MUST NOT output `<promise>QA VALIDATION COMPLETE</promise>` unless ALL completion criteria are genuinely met. The loop continues until true completion.

---

## Progress Tracking

Maintain `.orc-qa/PROGRESS.md`:

```markdown
# QA Validation Progress

## Current Phase: [N]
## Current Test: [description]

## Phase Status
| Phase | Status | Issues Found | Fixed |
|-------|--------|--------------|-------|
| 1. Init | Complete | 3 | 3 |
| 2. Tasks | In Progress | 1 | 0 |
| 3. Execution | Pending | - | - |
| 4. Web UI | Pending | - | - |
| 5. Advanced | Pending | - | - |
| 6. Errors | Pending | - | - |

## Last Updated
[timestamp]

## Notes
[context for next iteration]
```

---

## Iteration Protocol

Each iteration:

1. **Check state**: Read `.orc-qa/PROGRESS.md`
2. **Resume from checkpoint**: Continue current phase/test
3. **Execute test**: Run the actual orc command
4. **Observe**: Note any friction, errors, confusion
5. **Log issues**: Add to phase file if problems found
6. **Fix if needed**: Critical/High issues fixed immediately
7. **Update progress**: Mark test complete, move to next
8. **Commit**: Save progress to git

---

## Test Data

### Sample Tasks for Testing

```bash
# Trivial - should complete in one phase
orc new --weight trivial "Add TODO comment to main.go"

# Small - implement + test
orc new --weight small "Add health check endpoint at /healthz"

# Medium - full workflow
orc new "Implement rate limiting middleware"

# Likely to fail (tests error handling)
orc new "Integrate with nonexistent-api.example.com"
```

### Expected Behaviors

| Task Type | Expected Phases | Typical Duration |
|-----------|-----------------|------------------|
| Trivial | implement | < 2 min |
| Small | implement, test | 3-5 min |
| Medium | implement, test, docs | 5-10 min |
| Large | spec, implement, test, docs, validate | 15-30 min |

---

## When Complete

When ALL completion criteria are genuinely met:

1. Create summary:
```bash
echo "QA Validation Complete - $(date)" > .orc-qa/COMPLETE
```

2. Generate final report:
```bash
cat .orc-qa/SUMMARY.md
```

3. Output completion promise (ONLY if truly complete):
```xml
<promise>QA VALIDATION COMPLETE</promise>
```

---

## Quick Reference

### Orc Commands
```bash
orc init                    # Initialize project
orc new "title"             # Create task
orc run TASK-001            # Execute task
orc pause TASK-001          # Pause execution
orc resume TASK-001         # Resume execution
orc status                  # Show running tasks
orc list                    # List all tasks
orc show TASK-001           # Task details
orc log TASK-001            # View transcripts
orc delete TASK-001         # Remove task
orc serve                   # Start API server
```

### Key Paths
```
~/repos/orc/               # Orc source
~/repos/orc/bin/orc        # Built binary
~/repos/forex-platform/    # Test project
~/.orc/                    # Global config
.orc/                      # Project config
.orc-qa/                   # QA issue tracking
```

### Build & Test
```bash
cd ~/repos/orc
go build -o bin/orc ./cmd/orc
go test ./...
```

---

## Recovery After Pause

If resuming after interruption:

1. Read this prompt (you just did)
2. Check `.orc-qa/PROGRESS.md` for current state
3. Run `git status` to see uncommitted changes
4. Continue from where you left off

The filesystem IS the state. Pick up and continue.
