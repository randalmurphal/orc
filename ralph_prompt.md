# Orc Exhaustive QA Validation - Ralph Loop

## Completion Promise

```
<promise>EVERY FEATURE WORKS AND ALL TESTS PASS</promise>
```

Output this ONLY when genuinely complete. Do not lie to escape.

---

## Objective

Validate **every single feature** of the orc orchestrator through real usage until the product is production-ready for the solo dev workflow.

**The Solo Dev Workflow:**
1. Developer has a project they're working on
2. Developer creates tasks via `orc new "description"`
3. Orc spawns Claude Code to execute the task phases
4. Claude implements, tests, documents the changes
5. Task completes, code merges, developer moves on

**Two Perspectives to Validate:**
1. **Developer Experience** - Using orc CLI/UI to manage tasks
2. **Agent Experience** - Claude executing inside orc phases

Both must work flawlessly.

---

## Ralph Loop Invocation

```bash
cd ~/repos/orc
/ralph-loop --completion-promise 'EVERY FEATURE WORKS AND ALL TESTS PASS' < ralph_prompt.md
```

---

## State Management

All state lives in the filesystem. Each iteration:

1. Read `.orc-qa/PROGRESS.md` for current position
2. Continue from where you left off
3. Log issues to phase-specific files
4. Update progress after each test
5. Commit fixes with descriptive messages

```
.orc-qa/
├── PROGRESS.md           # Current position, phase status
├── SUMMARY.md            # Final summary (when complete)
├── phase01-cli-core.md   # Issues for phase 1
├── phase02-cli-full.md   # Issues for phase 2
├── ...
├── .stuck.md             # If stuck on same issue 3+ times
└── .blocked.md           # If blocked on external factor
```

---

## Test Project

Use `~/repos/forex-platform` as the test project.

**Reset Protocol** (when testing fresh experience):
```bash
cd ~/repos/forex-platform
rm -rf .orc/
git checkout main
git clean -fd
```

**Rebuild orc after fixes:**
```bash
cd ~/repos/orc
go build -o bin/orc ./cmd/orc
```

---

## Fix Protocol - CRITICAL

When fixing bugs, you MUST consider blast radius:

### Before Any Fix

1. **Identify the issue** - What exactly is broken?
2. **Find the code** - Where is the bug?
3. **Check callers** - Who uses this code?
4. **Check consumers** - What depends on this behavior?
5. **Consider edge cases** - What else might break?

### Making the Fix

1. **Minimal change** - Fix only what's broken
2. **Don't refactor** - No "while I'm here" changes
3. **Preserve contracts** - API signatures, file formats, behaviors
4. **Add tests** - If the bug wasn't caught, add a test
5. **Run existing tests** - `go test ./...` before committing

### After the Fix

1. **Verify the fix** - Reproduce the issue, confirm it's fixed
2. **Run all tests** - Ensure nothing else broke
3. **Test related features** - Manually verify adjacent functionality
4. **Commit atomically** - One fix per commit

### Fix Commit Format

```bash
git commit -m "[orc] Fix: <short description>

- What was broken
- Why it was broken
- How it was fixed
- What was verified

Affects: <list of commands/features impacted>
"
```

---

## PHASE 1: CLI Core Commands

Test the essential developer workflow.

### 1.1 Project Initialization

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Fresh init | `orc init` | Creates .orc/, <500ms | |
| Init output | `orc init` | Shows project ID, config path, next steps | |
| Detect Go project | `orc init` (in Go project) | Config has Go-appropriate settings | |
| Global registry | `cat ~/.orc/projects.yaml` | Project registered | |
| Double init | `orc init` (already initialized) | Error: "already initialized" | |
| Force reinit | `orc init --force` | Reinitializes cleanly | |
| Projects list | `orc projects` | Shows registered project | |

### 1.2 Task Creation

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Create task | `orc new "Add health check"` | Creates TASK-001 | |
| Task files | `ls .orc/tasks/TASK-001/` | task.yaml, plan.yaml exist | |
| With weight | `orc new --weight trivial "Quick fix"` | Skips classification | |
| With description | `orc new -d "Details here" "Title"` | Description in task.yaml | |
| Sequential IDs | Create 3 tasks | TASK-001, TASK-002, TASK-003 | |
| List tasks | `orc list` | Shows all created tasks | |
| Show task | `orc show TASK-001` | Displays full details | |
| Delete task | `orc delete TASK-001` | Removes task and files | |

### 1.3 Task Execution - Trivial

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Run trivial | `orc run TASK-001` (trivial weight) | Completes in single phase | |
| Progress output | Observe terminal | Shows phase transitions | |
| Transcript created | `ls .orc/tasks/TASK-001/transcripts/` | implement-001.md exists | |
| State updated | `cat .orc/tasks/TASK-001/state.yaml` | Status: completed | |
| Git branch | `git branch` | orc/TASK-001 exists | |
| Git commits | `git log --oneline` | [orc] commit present | |

### 1.4 Task Execution - Small

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Run small | `orc run TASK-002` (small weight) | implement → test phases | |
| Both phases complete | Check state.yaml | Both phases completed | |
| Test phase runs | Check transcripts | test-001.md exists | |
| Retry on test fail | Cause test to fail | Retries from implement | |

### 1.5 Pause/Resume/Stop

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Pause running | `orc pause TASK-001` | Execution stops, state saved | |
| Status shows paused | `orc status` | Shows paused task | |
| Resume paused | `orc resume TASK-001` | Continues from checkpoint | |
| Stop running | `orc stop TASK-001` | Immediate termination | |
| Cleanup on stop | Check worktree | Worktree removed | |

### 1.6 View & Status

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Status overview | `orc status` | Shows running, completed counts | |
| View transcripts | `orc log TASK-001` | Displays transcript content | |
| Log specific phase | `orc log TASK-001 --phase implement` | Shows only implement | |
| Show diff | `orc diff TASK-001` | Git diff output | |

---

## PHASE 2: CLI Full Commands

Test all remaining CLI commands.

### 2.1 Configuration

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Show config | `orc config show` | Displays all config | |
| With sources | `orc config show --source` | Shows where each value comes from | |
| Get single | `orc config get model` | Returns model value | |
| Set value | `orc config set max_iterations 50` | Updates user config | |
| Set project | `orc config set --project profile safe` | Updates .orc/config.yaml | |
| Profile change | `orc config set profile strict` | Changes automation level | |

### 2.2 Task Control

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Skip phase | `orc skip TASK-001 --phase test -r "Manual"` | Skips with audit | |
| Rewind to phase | `orc rewind TASK-001 --to implement` | Resets state | |
| Approve gate | `orc approve TASK-001` | Gate passes | |
| Reject gate | `orc reject TASK-001 --reason "Needs fix"` | Gate fails, phase retries | |
| Run with profile | `orc run TASK-001 --profile safe` | Uses safe profile | |
| Dry run | `orc run TASK-001 --dry-run` | Shows plan, no execution | |

### 2.3 Import/Export

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Export task | `orc export TASK-001` | YAML output | |
| Export with transcripts | `orc export TASK-001 --transcripts` | Includes logs | |
| Import task | `orc import exported.yaml` | Creates task from export | |
| Import force | `orc import --force exported.yaml` | Overwrites existing | |

### 2.4 Cleanup

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Cleanup completed | `orc cleanup` | Removes completed branches | |
| Cleanup dry run | `orc cleanup --dry-run` | Preview only | |
| Cleanup all | `orc cleanup --all` | All orc branches | |

### 2.5 Initiatives

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Create initiative | `orc initiative new "Auth System"` | Creates INIT-001 | |
| List initiatives | `orc initiative list` | Shows initiative | |
| Show initiative | `orc initiative show INIT-001` | Displays details | |
| Add task | `orc initiative add-task INIT-001 TASK-001` | Links task | |
| Add decision | `orc initiative decide INIT-001 "Use JWT"` | Records decision | |
| Run initiative | `orc initiative run INIT-001` | Runs tasks in order | |

### 2.6 Templates

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| List templates | `orc template list` | Shows available templates | |
| Show template | `orc template show bugfix` | Displays template | |
| Save as template | `orc template save TASK-001 --name my-template` | Creates template | |
| New from template | `orc new --template bugfix "Fix login"` | Uses template | |

### 2.7 Token Pool

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Pool init | `orc pool init` | Creates ~/.orc/token-pool/ | |
| Pool list | `orc pool list` | Shows accounts | |
| Pool status | `orc pool status` | Shows current account | |
| Pool reset | `orc pool reset` | Clears exhausted flags | |

### 2.8 Cost Tracking

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Show cost | `orc cost` | Displays usage summary | |
| Cost by period | `orc cost --period week` | Weekly breakdown | |
| Task cost | `orc show TASK-001` | Includes token/cost | |

---

## PHASE 3: API Endpoints

Test all REST API endpoints.

### 3.1 Start Server

```bash
cd ~/repos/orc && ./bin/orc serve &
# Wait for startup
sleep 2
```

### 3.2 Task Endpoints

| Test | Method | Endpoint | Expected | Status |
|------|--------|----------|----------|--------|
| List tasks | GET | /api/tasks | 200, task array | |
| Create task | POST | /api/tasks | 201, task object | |
| Get task | GET | /api/tasks/TASK-001 | 200, task details | |
| Delete task | DELETE | /api/tasks/TASK-001 | 204 | |
| Get state | GET | /api/tasks/TASK-001/state | 200, state object | |
| Get plan | GET | /api/tasks/TASK-001/plan | 200, plan object | |
| Get transcripts | GET | /api/tasks/TASK-001/transcripts | 200, transcript list | |
| Run task | POST | /api/tasks/TASK-001/run | 202, started | |
| Pause task | POST | /api/tasks/TASK-001/pause | 200, paused | |
| Resume task | POST | /api/tasks/TASK-001/resume | 202, resumed | |
| Rewind task | POST | /api/tasks/TASK-001/rewind | 200, rewound | |

### 3.3 Project Endpoints

| Test | Method | Endpoint | Expected | Status |
|------|--------|----------|----------|--------|
| List projects | GET | /api/projects | 200, project array | |
| Get project | GET | /api/projects/:id | 200, project details | |
| Project tasks | GET | /api/projects/:id/tasks | 200, task array | |

### 3.4 Initiative Endpoints

| Test | Method | Endpoint | Expected | Status |
|------|--------|----------|----------|--------|
| List initiatives | GET | /api/initiatives | 200, array | |
| Create initiative | POST | /api/initiatives | 201, object | |
| Get initiative | GET | /api/initiatives/:id | 200, details | |
| Update initiative | PUT | /api/initiatives/:id | 200, updated | |
| Delete initiative | DELETE | /api/initiatives/:id | 204 | |
| Initiative tasks | GET | /api/initiatives/:id/tasks | 200, array | |
| Add task | POST | /api/initiatives/:id/tasks | 201 | |
| Add decision | POST | /api/initiatives/:id/decisions | 201 | |
| Ready tasks | GET | /api/initiatives/:id/ready | 200, array | |

### 3.5 Config Endpoints

| Test | Method | Endpoint | Expected | Status |
|------|--------|----------|----------|--------|
| Get prompts | GET | /api/prompts | 200, prompt list | |
| Get prompt | GET | /api/prompts/implement | 200, prompt content | |
| Save prompt | PUT | /api/prompts/implement | 200, saved | |
| Delete prompt | DELETE | /api/prompts/implement | 204 | |
| Get hooks | GET | /api/hooks | 200, hook map | |
| Get skills | GET | /api/skills | 200, skill list | |
| Get settings | GET | /api/settings | 200, settings | |
| Update settings | PUT | /api/settings | 200, updated | |
| Get tools | GET | /api/tools | 200, tool list | |
| Get agents | GET | /api/agents | 200, agent list | |
| Get scripts | GET | /api/scripts | 200, script list | |
| Get CLAUDE.md | GET | /api/claudemd | 200, content | |
| Get MCP | GET | /api/mcp | 200, server list | |
| Get cost | GET | /api/cost/summary | 200, summary | |
| Get config | GET | /api/config | 200, config | |

### 3.6 WebSocket

| Test | Action | Expected | Status |
|------|--------|----------|--------|
| Connect | WS /api/ws | Connection established | |
| Subscribe | `{"type":"subscribe","task_id":"TASK-001"}` | Subscribed response | |
| Receive events | Run task | State/transcript events | |
| Ping/pong | `{"type":"ping"}` | Pong response | |
| Unsubscribe | `{"type":"unsubscribe"}` | Unsubscribed | |

---

## PHASE 4: Web UI

Test the web dashboard.

### 4.1 Setup

```bash
# Terminal 1: API
cd ~/repos/orc && ./bin/orc serve

# Terminal 2: Frontend
cd ~/repos/orc/web && bun dev
```

### 4.2 Dashboard

| Test | Action | Expected | Status |
|------|--------|----------|--------|
| Load dashboard | Navigate to / | Dashboard renders | |
| Quick stats | View stats | Running, Blocked, Today, Tokens | |
| Active tasks | View section | Shows running/paused tasks | |
| Recent activity | View section | Shows completed/failed | |
| Connection status | Check indicator | Shows Live/Connecting/Offline | |
| Real-time update | Run task | Stats update live | |

### 4.3 Task Management

| Test | Action | Expected | Status |
|------|--------|----------|--------|
| Task list | Navigate to /tasks | Lists all tasks | |
| Create task | Click New Task | Modal opens | |
| Submit task | Fill form, submit | Task created, appears in list | |
| Task filters | Filter by status | List filters correctly | |
| Task search | Search by title | Results filter | |
| Task detail | Click task | Detail page loads | |
| Phase timeline | View timeline | Shows all phases | |
| Transcript view | View transcript | Shows Claude output | |
| Run from UI | Click Run | Task starts executing | |
| Pause from UI | Click Pause | Task pauses | |
| Resume from UI | Click Resume | Task resumes | |
| Delete from UI | Click Delete | Confirmation, then removes | |

### 4.4 Settings Pages

| Test | Action | Expected | Status |
|------|--------|----------|--------|
| Settings page | Navigate to /settings | Settings render | |
| AI settings | View AI tab | Model, iterations, timeout | |
| Gate settings | View Gates tab | Gate configuration | |
| Save settings | Modify and save | Changes persist | |
| Source display | Check sources | Shows where values come from | |

### 4.5 Project Management

| Test | Action | Expected | Status |
|------|--------|----------|--------|
| Project dropdown | Click dropdown | Shows registered projects | |
| Switch project | Select different project | Context switches | |
| Project tasks | View tasks | Shows project-specific tasks | |

### 4.6 Keyboard Shortcuts

Uses `Shift+Alt` modifier (⇧⌥ on Mac) for global shortcuts to avoid browser conflicts.

| Test | Shortcut | Expected | Status |
|------|----------|----------|--------|
| Command palette | Shift+Alt+K | Palette opens | |
| New task | Shift+Alt+N | New task modal | |
| Toggle sidebar | Shift+Alt+B | Sidebar toggles | |
| Project switcher | Shift+Alt+P | Project switcher opens | |
| Search focus | / | Search focused | |
| Help | ? | Help modal | |
| Close | Esc | Modal/overlay closes | |
| Navigate tasks | j/k | Selection moves | |
| Open task | Enter | Opens selected task | |
| Go dashboard | g d | Navigates to dashboard | |
| Go tasks | g t | Navigates to tasks | |
| Go environment | g e | Navigates to environment | |

### 4.7 Real-time Updates

| Test | Action | Expected | Status |
|------|--------|----------|--------|
| Task state change | Run task | Status updates live | |
| Phase completion | Complete phase | Timeline updates | |
| Transcript stream | During execution | Lines appear live | |
| Toast notification | Task completes | Toast appears | |
| Reconnection | Disconnect/reconnect | Auto-reconnects | |

### 4.8 Responsive Design

| Test | Viewport | Expected | Status |
|------|----------|----------|--------|
| Mobile layout | 375px width | Stacks correctly | |
| Tablet layout | 768px width | Adjusts layout | |
| Desktop layout | 1200px+ width | Full layout | |
| Sidebar collapse | Mobile | Collapsible | |

---

## PHASE 5: Agent Experience

Test what Claude sees when executing inside orc.

### 5.1 Prompt Rendering

| Test | Expected | Status |
|------|----------|--------|
| Task context in prompt | `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}` rendered | |
| Phase name in prompt | `{{PHASE}}` correct | |
| Iteration in prompt | `{{ITERATION}}` correct | |
| Workspace path | `{{WORKSPACE}}` points to worktree | |
| Project CLAUDE.md | Included in context | |

### 5.2 Phase Execution

| Test | Expected | Status |
|------|----------|--------|
| Implement phase | Claude can write code | |
| Test phase | Claude can run tests | |
| Docs phase | Claude can update docs | |
| Spec phase | Claude can write specs | |
| Completion signal | `<phase_complete>true</phase_complete>` detected | |
| Blocked signal | `<phase_blocked>reason</phase_blocked>` detected | |

### 5.3 Retry Context

| Test | Expected | Status |
|------|----------|--------|
| Test fail → retry | `{{RETRY_CONTEXT}}` has failure info | |
| Retry attempt number | Context shows attempt # | |
| Previous error | Context includes error output | |
| Fix guidance | Agent can fix based on context | |

### 5.4 Worktree Isolation

| Test | Expected | Status |
|------|----------|--------|
| Worktree created | .orc/worktrees/orc-TASK-XXX exists | |
| Correct branch | Worktree on orc/TASK-XXX branch | |
| Isolated changes | Changes don't affect main worktree | |
| Cleanup on complete | Worktree removed after success | |
| Kept on failure | Worktree preserved for debugging | |

### 5.5 Git Operations

| Test | Expected | Status |
|------|----------|--------|
| Branch created | orc/TASK-XXX branch exists | |
| Phase commits | Commit after each phase | |
| Commit format | [orc] TASK-XXX: phase - status | |
| Checkpoint | Can rewind to checkpoint | |

---

## PHASE 6: Automation Profiles

Test each profile's behavior.

### 6.1 Auto Profile (Default)

| Test | Expected | Status |
|------|----------|--------|
| All gates auto | No human intervention needed | |
| Retry enabled | Test failures retry from implement | |
| Merge auto | Merges on completion | |

### 6.2 Fast Profile

| Test | Expected | Status |
|------|----------|--------|
| No gates | Runs straight through | |
| Retry disabled | Failures don't retry | |
| Skip on stuck | Continues past stuck phases | |

### 6.3 Safe Profile

| Test | Expected | Status |
|------|----------|--------|
| Most gates auto | Normal phases proceed | |
| Merge requires human | Pauses before merge | |
| Review by AI | AI reviews before merge | |

### 6.4 Strict Profile

| Test | Expected | Status |
|------|----------|--------|
| Spec requires human | Pauses at spec phase | |
| Design requires human | Pauses at design phase | |
| Merge requires human | Pauses before merge | |
| Retry needs decision | Human decides on retry | |

---

## PHASE 7: Weight & Phase Combinations

Test all weight/phase combinations.

### 7.1 Trivial Weight

| Test | Expected | Status |
|------|----------|--------|
| Single implement phase | No other phases | |
| Max 5 iterations | Stops at 5 | |
| Quick completion | < 5 minutes typical | |
| No docs phase | Skipped | |

### 7.2 Small Weight

| Test | Expected | Status |
|------|----------|--------|
| implement → test | Both phases run | |
| Max 20 iterations | Per phase | |
| Test retry | Fails test → retry from implement | |

### 7.3 Medium Weight

| Test | Expected | Status |
|------|----------|--------|
| Full phase sequence | spec → implement → review → docs → test | |
| Docs phase runs | Documentation updated | |
| Review phase | Claude reviews changes | |

### 7.4 Large Weight

| Test | Expected | Status |
|------|----------|--------|
| Research phase | Research happens first | |
| Design phase | Architecture decisions | |
| Validate phase | E2E validation | |
| Max 50 iterations | Extended limit | |

### 7.5 Greenfield Weight

| Test | Expected | Status |
|------|----------|--------|
| Full phase suite | All phases | |
| Human gates | Some phases need approval | |
| Extended iterations | Up to 100 | |

---

## PHASE 8: Error Handling

Test error conditions and recovery.

### 8.1 User Errors

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Task not found | `orc run NONEXISTENT` | Error with suggestion | |
| Not initialized | `orc run` (no .orc/) | "Run orc init first" | |
| Invalid task ID | `orc show INVALID` | Clear error message | |
| Missing required arg | `orc new` (no title) | Usage help | |

### 8.2 Execution Errors

| Test | Scenario | Expected | Status |
|------|----------|----------|--------|
| Claude timeout | Phase takes too long | Timeout error, retry option | |
| Rate limit | Hit API limit | Token pool switch or wait | |
| Max iterations | Phase won't complete | Stuck detection, skip option | |
| Parse error | Invalid Claude output | Retry iteration | |

### 8.3 System Errors

| Test | Scenario | Expected | Status |
|------|----------|----------|--------|
| Config parse error | Invalid YAML | Clear syntax error | |
| Database error | Corrupted DB | Recovery instructions | |
| Git conflict | Merge conflict | Resolution guidance | |
| Disk full | No space | Graceful error | |

### 8.4 Recovery

| Test | Scenario | Expected | Status |
|------|----------|----------|--------|
| Ctrl+C cleanup | Interrupt running task | State preserved, worktree ok | |
| Resume after crash | Kill -9, then resume | Continues from checkpoint | |
| Rewind after failure | Phase failed badly | Can rewind to earlier phase | |

---

## PHASE 9: Stuck Detection

Test the stuck detection system.

| Test | Scenario | Expected | Status |
|------|----------|----------|--------|
| Detect same error 3x | Same error 3 iterations | Stuck detected | |
| Create .stuck.md | Stuck triggered | Analysis file created | |
| Skip option | Stuck with skip_on_stuck | Skips to next phase | |
| Error normalization | Timestamps stripped | Same error recognized | |
| Resume from stuck | Address issue, resume | Continues normally | |

---

## PHASE 10: Cost & Token Tracking

Test cost tracking accuracy.

| Test | Expected | Status |
|------|----------|--------|
| Token capture | Each iteration logs tokens | |
| Phase aggregation | Phase totals correct | |
| Task aggregation | Task totals correct | |
| Cost calculation | USD estimate reasonable | |
| Cost warnings | Threshold triggers warning | |
| Cost API | /api/cost/summary returns data | |
| Dashboard widget | Tokens displayed in UI | |

---

## PHASE 11: Completion Actions

Test post-completion actions.

### 11.1 PR Creation

| Test | Expected | Status |
|------|----------|--------|
| PR created | GitHub/GitLab PR exists | |
| Title template | `[orc] {{TASK_TITLE}}` rendered | |
| Body template | PR body from template | |
| Labels applied | Configured labels present | |
| Auto-merge | Merges when approved (if configured) | |

### 11.2 Direct Merge

| Test | Expected | Status |
|------|----------|--------|
| Merge to main | Code on main branch | |
| Branch deleted | Task branch removed | |
| Clean history | Squash merge clean | |

### 11.3 No Action

| Test | Expected | Status |
|------|----------|--------|
| Branch preserved | Task branch remains | |
| No PR | No PR created | |
| Manual completion | Developer handles merge | |

---

## PHASE 12: Integration Tests

Run the actual test suites.

| Test | Command | Expected | Status |
|------|---------|----------|--------|
| Go unit tests | `go test ./...` | All pass | |
| Go race tests | `go test -race ./...` | No races | |
| Go coverage | `go test -cover ./...` | Reports coverage | |
| Frontend tests | `cd web && bun test` | All pass | |
| E2E tests | `make e2e` | Playwright passes | |

---

## Completion Criteria

**ALL must be true:**

### Functional
- [ ] Every test in Phases 1-12 passes
- [ ] Zero Critical bugs open
- [ ] Zero High bugs open
- [ ] All Medium bugs logged

### Quality
- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] No panics in any scenario
- [ ] All error messages are actionable

### Documentation
- [ ] CLAUDE.md reflects current behavior
- [ ] All fixes have descriptive commits
- [ ] `.orc-qa/SUMMARY.md` complete

### Clean Run
- [ ] Fresh init on forex-platform works
- [ ] Create task → run → complete → merge works
- [ ] Web UI functional end-to-end
- [ ] No regressions from fixes

---

## Progress Template

Update `.orc-qa/PROGRESS.md` after each test:

```markdown
# QA Progress

## Current Position
- Phase: [N]
- Section: [X.Y]
- Test: [description]

## Phase Status
| Phase | Status | Tests | Pass | Fail | Issues |
|-------|--------|-------|------|------|--------|
| 1. CLI Core | Complete | 25 | 25 | 0 | 2 |
| 2. CLI Full | In Progress | 40 | 30 | 2 | 3 |
| ... | | | | | |

## Open Issues
| ID | Severity | Phase | Description | Status |
|----|----------|-------|-------------|--------|
| 001 | High | 1.3 | Task run fails silently | Fixed |
| 002 | Medium | 2.1 | Config source not shown | Open |

## Last Updated
[timestamp]

## Notes
[context for next iteration]
```

---

## Issue Template

Log issues to `.orc-qa/phaseNN-name.md`:

```markdown
## Issue NNN: [Title]

**Severity**: Critical | High | Medium | Low
**Test**: Phase X.Y - [test name]
**Status**: Open | Fixed | Won't Fix

### Reproduction
```bash
[exact commands]
```

### Expected
[what should happen]

### Actual
[what happens]

### Error Output
```
[paste error]
```

### Analysis
[root cause if known]

### Fix
- File: `path/to/file.go:line`
- Change: [description]
- Commit: [sha]

### Verification
- [ ] Issue no longer reproduces
- [ ] Related tests pass
- [ ] No regressions
```

---

## Self-Correction

### If Test Fails
1. Log to phase file immediately
2. Assess severity (Critical/High = fix now)
3. Find root cause before fixing
4. Make minimal targeted fix
5. Verify fix and run related tests

### If Stuck 3+ Times
1. Write detailed analysis to `.orc-qa/.stuck.md`
2. Try different approach
3. If still stuck, mark as "Needs Review" and continue

### If Blocked
1. Document in `.orc-qa/.blocked.md`
2. Continue with other tests
3. Return when unblocked

### DO NOT LIE
Even if stuck, tired, or wanting to exit - do NOT output the completion promise unless genuinely complete. The loop continues until true completion.

---

## When Complete

When ALL criteria genuinely met:

```bash
# Create summary
cat > .orc-qa/SUMMARY.md << 'EOF'
# QA Validation Summary

## Completion Date
[date]

## Statistics
- Total Tests: [N]
- Passed: [N]
- Issues Found: [N]
- Issues Fixed: [N]

## Issues by Severity
- Critical: [N] (all fixed)
- High: [N] (all fixed)
- Medium: [N] ([N] fixed, [N] deferred)
- Low: [N] ([N] fixed, [N] deferred)

## Key Fixes
1. [description]
2. [description]

## Known Limitations
- [any deferred items]

## Verification
- [ ] All Phase 1-12 tests pass
- [ ] go test passes
- [ ] Clean run verified
EOF

# Mark complete
echo "COMPLETE - $(date)" > .orc-qa/COMPLETE
```

Then output:
```xml
<promise>EVERY FEATURE WORKS AND ALL TESTS PASS</promise>
```
