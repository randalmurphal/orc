# Orc v1.0 - Ralph Wiggum Build Prompt

## Objective

Build the complete orc v1.0 orchestrator with all P0 and P1 features implemented, tested, and verified.

---

## Library Integration Map

**Use existing capabilities from sibling libraries. Don't reinvent.**

### From llmkit (../llmkit)

| Feature | Package | Use In Orc |
|---------|---------|------------|
| Claude CLI wrapper | `claude.Client` | Already using in executor |
| Session management | `claude/session` | **Leverage for Session Interop** - has token/cost tracking |
| Template rendering | `template.Engine` | Use for prompt rendering instead of custom |
| Response parsing | `parser.IsPhaseComplete()` | Already using for phase detection |
| Token budgeting | `tokens.Budget` | Use for context window management |
| Cost tracking | `model.CostTracker` | **Use for Cost Tracking feature** |
| Model selection | `model.Selector` | Use for phase-appropriate model selection |
| Escalation chains | `model.EscalationChain` | Use for retry with model fallback |

### From devflow (../devflow)

| Feature | Package | Use In Orc |
|---------|---------|------------|
| Git operations | `git.Context` | Already wrapped in internal/git |
| Worktree isolation | `git.CreateWorktree()` | Already using |
| Branch naming | `git.BranchNamer` | Use for task branch names |
| PR creation | `pr.Provider` | **Use for completion actions** (PR/merge) |
| Transcript storage | `transcript.FileStore` | Use for phase transcript persistence |
| Artifact storage | `artifact.Manager` | Use for test results, lint output |

### From flowgraph (../flowgraph)

| Feature | Package | Use In Orc |
|---------|---------|------------|
| Graph execution | `flowgraph.CompiledGraph` | Already using in executor |
| Checkpointing | `checkpoint.SQLiteStore` | **Consider for task state** instead of YAML |
| Event bus | `event.Bus` | Consider for real-time updates |
| Signals | `signal.Signal` | Consider for human gates |
| Observability | `metrics`, `tracing` | Use for phase telemetry |

### Orc-Specific (Must Build)

| Feature | Why Orc-Specific |
|---------|------------------|
| Task YAML persistence | Orc's task/plan/state model |
| Weight classification | Orc's trivial→greenfield mapping |
| Automation profiles | Orc's auto/fast/safe/strict gates |
| Init wizard | Orc's interactive setup flow |
| Web dashboard | Orc's Svelte frontend |
| Multi-project registry | Orc's cross-project support |

---

## Current State Check

Before each iteration:
1. Run `git status` to see changes
2. Run `make test` to check test status
3. Run `make e2e` to check E2E test status
4. Review TODO.md for remaining items
5. Check `.stuck.md` and `.blocked.md` if they exist

---

## Feature Priority

### P0 (Must Complete First)
1. Error Message Standards
2. Session Interoperability
3. Init Wizard
4. Task Enhancement Flow

### P1 (Complete After P0)
5. Cost Tracking
6. Task Templates
7. Web Dashboard
8. Project Detection
9. Keyboard Shortcuts

### P2 (If Time Permits)
10. TUI Watch Mode
11. Cross-Project Resources

---

## Completion Criteria

**ALL of the following must be true to output `<promise>COMPLETE</promise>`**

---

### 1. Error Message Standards

#### Backend Implementation
- [ ] `internal/errors/errors.go` exists with `OrcError` type
- [ ] OrcError has: Code, What, Why, Fix, DocsURL, Cause fields
- [ ] Error constructors exist for all error codes:
  - `ErrNotInitialized`, `ErrAlreadyInitialized`
  - `ErrTaskNotFound`, `ErrTaskInvalidState`, `ErrTaskRunning`
  - `ErrClaudeUnavailable`, `ErrClaudeTimeout`, `ErrPhaseStuck`, `ErrMaxRetries`
  - `ErrConfigInvalid`, `ErrConfigMissing`
  - `ErrGitDirty`, `ErrGitBranchExists`
- [ ] CLI uses `printError()` for all error output
- [ ] API returns JSON error responses with code/message/context/fix

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestOrcErrorFormat` - UserMessage() format correct
  - `TestOrcErrorJSON` - API JSON serialization
  - `TestErrNotInitializedError` - constructor correctness
  - `TestErrTaskNotFoundError` - ID interpolation
  - `TestErrClaudeTimeoutError` - duration formatting
  - `TestErrorCodeUniqueness` - no duplicate codes
- [ ] Integration tests:
  - CLI prints friendly errors for each code
  - API returns correct HTTP status per category
  - `--debug` shows stack traces
- [ ] E2E tests (Playwright MCP):
  - Error card appears on UI errors
  - Error buttons (View Transcript, Rewind) work

---

### 2. Session Interoperability

**Leverage:** `llmkit/claude/session` package already provides session management.

#### Backend Implementation (Wire Up llmkit)
- [ ] Use `session.SessionManager` from llmkit for multi-session tracking
- [ ] Use `session.Session` for bidirectional Claude communication
- [ ] Session metadata (ID, tokens, cost) comes from `session.OutputMessage`
- [ ] Wrap llmkit session in orc's state.yaml for persistence
- [ ] `orc session TASK-ID` command shows session info from llmkit
- [ ] `orc resume TASK-ID` uses llmkit's session resume capability
- [ ] `orc attach TASK-ID` attaches to running llmkit session
- [ ] `GET /api/tasks/:id/session` returns session data

#### Orc-Specific Additions
- [ ] Map llmkit session state to orc task state
- [ ] Graceful pause triggers session.Close() with state preservation
- [ ] Store session context in state.yaml for cross-process resume

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestSessionManagerIntegration` - llmkit session wiring
  - `TestGracefulPause` - state.yaml updated with session data
  - `TestResumeFromState` - reconstruct llmkit session from state
  - `TestConcurrentAccessDetection` - running session detection
- [ ] Integration tests:
  - Pause API preserves session via llmkit
  - Resume constructs continuation prompt
  - WebSocket publishes session events
- [ ] E2E tests (Playwright MCP):
  - Pause task, verify session ID displayed
  - Resume button continues execution
  - Copy session ID works

---

### 3. Init Wizard

#### Backend Implementation
- [ ] `internal/wizard/wizard.go` - interactive prompts
- [ ] `internal/detect/detect.go` - project detection
- [ ] Arrow key navigation for selections
- [ ] Profile selection (auto, safe, strict, custom)
- [ ] Completion action selection (PR, merge, none)
- [ ] Model selection
- [ ] Skill installation with recommendations
- [ ] CLAUDE.md section generation (idempotent)
- [ ] `--quick` flag for non-interactive
- [ ] `--advanced` flag for Claude session setup
- [ ] Project registered in global registry

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestDetectGoProject` - go.mod parsing
  - `TestDetectPythonProject` - pyproject.toml parsing
  - `TestDetectNodeProject` - package.json parsing
  - `TestFrameworkDetection` - Gin, Next.js, FastAPI
  - `TestCLAUDEMDSectionIdempotent` - regeneration
  - `TestSkillRecommendationByLanguage`
- [ ] Integration tests:
  - `orc init` creates correct files
  - `orc init --quick` completes without prompts
  - `orc init --force` overwrites existing
  - Skills written to `.claude/skills/`
- [ ] E2E tests (Playwright MCP):
  - After init, project appears in dropdown
  - Config page shows detected settings

---

### 4. Task Enhancement Flow

#### Backend Implementation
- [ ] `internal/enhance/enhance.go` - enhancement logic
- [ ] `templates/prompts/enhance.md` - enhancement prompt
- [ ] Three modes: Quick (--weight), Standard (auto), Interactive (-i)
- [ ] Enhancement analyzes codebase for scope
- [ ] Weight classification from analysis
- [ ] Enhanced description saved to task.yaml
- [ ] Enhancement session ID stored
- [ ] `POST /api/tasks` supports mode: enhanced|quick|interactive
- [ ] `GET /api/tasks/:id/enhancement` returns status/analysis

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestWeightClassification` - --weight flag parsing
  - `TestEnhancementPromptRendering` - variable substitution
  - `TestEnhancementYAMLParsing` - parse Claude output
  - `TestSkipEnhancementWithQuickFlag`
- [ ] Integration tests:
  - Enhancement flow with mock Claude response
  - State persistence after enhancement
  - API endpoints for all three modes
- [ ] E2E tests (Playwright MCP):
  - Create task with "Enhanced" mode
  - Enhancement progress UI appears
  - Accept/Edit/Cancel buttons work

---

### 5. Cost Tracking

**Leverage:** `llmkit/model.CostTracker` already has per-model pricing and aggregation.

#### Backend Implementation (Wire Up llmkit)
- [ ] Use `model.CostTracker` from llmkit for token/cost aggregation
- [ ] CostTracker already has 2025 pricing (Opus: $15/$75, Sonnet: $3/$15, Haiku: $0.25/$1.25)
- [ ] Use `tracker.Record(model, input, output)` after each Claude call
- [ ] Use `tracker.EstimatedCost()` for task cost display
- [ ] Use `tracker.Summary()` for period aggregation

#### Orc-Specific Additions
- [ ] Persist cost data to state.yaml per phase/task
- [ ] `~/.orc/pricing.yaml` for price overrides (optional, llmkit has defaults)
- [ ] `orc show TASK-ID` displays tokens/cost from tracker
- [ ] `orc cost` aggregates across tasks by period
- [ ] `GET /api/tasks/:id/tokens` endpoint
- [ ] `GET /api/cost/summary?period=week` endpoint
- [ ] Budget alerts via config threshold check

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestCostTrackerIntegration` - llmkit tracker wiring
  - `TestPersistCostToState` - state.yaml tokens section
  - `TestAggregateCostAcrossTasks` - multi-task summary
  - `TestBudgetAlert_ThresholdTriggered`
- [ ] Integration tests:
  - Task execution records tokens via llmkit
  - state.yaml contains tokens section
  - API returns token data from tracker
- [ ] E2E tests (Playwright MCP):
  - Dashboard shows token widget
  - Task card displays token count and cost
  - Task detail tokens tab shows breakdown

---

### 6. Task Templates

**Leverage:** `llmkit/template.Engine` for Handlebars-style variable substitution.

#### Backend Implementation (Wire Up llmkit)
- [ ] Use `template.Engine` from llmkit for variable rendering
- [ ] Engine supports: `{{var}}`, `{{#if x}}`, `{{#each items}}`, helpers
- [ ] Use `template.Parse()` to extract required variables from template

#### Orc-Specific Additions
- [ ] `internal/template/store.go` - template storage/retrieval
- [ ] Template storage in `.orc/templates/` and `~/.orc/templates/`
- [ ] Template YAML format with metadata + prompt content
- [ ] `orc template save TASK-ID --name X`
- [ ] `orc template list`
- [ ] `orc template show X`
- [ ] `orc template delete X`
- [ ] `orc new --template X "title"`
- [ ] Built-in templates: bugfix, feature, refactor, migration, spike
- [ ] Resolution order: project > global > builtin

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestTemplateEngineIntegration` - llmkit engine wiring
  - `TestParseTemplateYAML` - orc template format
  - `TestTemplateResolutionOrder` - project > global > builtin
  - `TestSaveTemplateFromTask`
- [ ] Integration tests:
  - CLI template commands work
  - API endpoints work
  - Global vs project templates
- [ ] E2E tests (Playwright MCP):
  - New task modal shows template dropdown
  - Template variables appear as form fields
  - Templates page lists all templates

---

### 7. Web Dashboard

#### Backend Implementation
- [ ] `GET /api/dashboard/stats` endpoint
- [ ] Returns: running, blocked, paused, today counts, tokens/cost

#### Frontend Implementation
- [ ] Dashboard is default home page
- [ ] Quick Stats widget (Running, Blocked, Today, Tokens)
- [ ] Active Tasks section with expanded cards
- [ ] Recent Activity feed (last 5 completed/failed)
- [ ] Quick Actions bar
- [ ] WebSocket integration for real-time updates
- [ ] Toast notification system
- [ ] Notification center
- [ ] Responsive mobile layout

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestFormatRelativeTime`
  - `TestDashboardStatsAggregation`
  - `TestNotificationQueue`
- [ ] Integration tests:
  - `GET /api/dashboard/stats` accuracy
  - WebSocket broadcasts events
- [ ] E2E tests (Playwright MCP):
  - Dashboard loads within 500ms
  - Quick stats display correct counts
  - Clicking stat card navigates to filtered list
  - Active tasks show phase progress
  - Real-time update when task completes
  - Toast notification appears on completion
  - Mobile layout stacks cards

---

### 8. Project Detection

#### Backend Implementation
- [ ] `internal/detect/language.go` - language detection
- [ ] `internal/detect/framework.go` - framework detection
- [ ] `internal/detect/tools.go` - tool detection
- [ ] Detect: Go, TypeScript, Python, Rust
- [ ] Detect: Gin, Cobra, Next.js, React, FastAPI, etc.
- [ ] Set test/lint/build commands
- [ ] Recommend skills by language/framework
- [ ] `POST /api/projects/:id/detect` endpoint
- [ ] `POST /api/skills/install` endpoint

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestDetectLanguage_Go/TypeScript/Python/Rust`
  - `TestDetectGoFramework_Gin/Cobra`
  - `TestDetectJSFramework_React/Next`
  - `TestDetectTool_Docker/GitHubActions`
  - `TestParseGoVersion`
- [ ] Integration tests:
  - Detection saves to config
  - Skill recommendations work
- [ ] E2E tests (Playwright MCP):
  - Project info displayed in settings
  - Detected commands shown in config

---

### 9. Keyboard Shortcuts

#### Frontend Implementation
- [ ] `web/src/lib/shortcuts.ts` - ShortcutManager
- [ ] Global shortcuts: `⌘K`, `n`, `g d`, `g t`, `g s`, `/`, `?`, `Esc`
- [ ] Task list: `j/k` navigate, `Enter` open, `r` run, `p` pause
- [ ] Task detail: `r`, `p`, `c`, `t`, `[`, `]`, `Backspace`
- [ ] Modal: `Esc`, `Enter`, `Tab`, `Shift+Tab`
- [ ] Visual selection indicator on tasks
- [ ] Shortcut hints on buttons
- [ ] `?` shows help modal

#### Tests (80%+ coverage)
- [ ] Unit tests:
  - `TestNormalizeKey` - key combinations
  - `TestShortcutManager_Register/Unregister`
  - `TestShortcutManager_ScopeChange`
  - `TestSequentialShortcut` - g then d
- [ ] Integration tests:
  - Shortcuts ignored in input fields
  - Modal shortcuts work when modal open
- [ ] E2E tests (Playwright MCP):
  - Press `?` opens help modal
  - `Cmd+K` opens command palette
  - `j/k` navigation in task list
  - `g d` navigates to dashboard
  - Selected task has visual indicator

---

## E2E Testing Protocol (Playwright MCP)

Use these MCP tools for E2E verification:

```
1. mcp__playwright__browser_navigate - Go to page
2. mcp__playwright__browser_snapshot - Capture state
3. mcp__playwright__browser_click - Click element
4. mcp__playwright__browser_type - Type text
5. mcp__playwright__browser_fill_form - Fill forms
6. mcp__playwright__browser_wait_for - Wait for conditions
7. mcp__playwright__browser_network_requests - Verify API calls
```

### E2E Test Flow

1. **Setup**: Ensure API on :8080, frontend on :5173
2. **Navigate**: Go to page under test
3. **Verify State**: Capture snapshot, verify elements
4. **Interact**: Click, type, fill forms
5. **Verify Results**: Snapshot after action, check changes

### Critical User Flows to Test

1. **Task Creation**: New Task → Fill title → Submit → Verify in list
2. **Task Execution**: Select task → Run → Verify streaming → Wait completion
3. **Dashboard**: Load → Verify stats → Click stat → Verify filter
4. **Keyboard Navigation**: Press j/k → Verify selection moves
5. **Template Usage**: New Task → Select template → Verify fields

---

## Self-Correction Rules

### If Test Fails
1. Read error message carefully
2. Check `git log` for recent changes
3. Run single test with verbose output
4. Fix specific failure
5. Re-run full suite
6. Continue when passing

### If Stuck for 3+ Iterations
1. Write analysis to `.stuck.md`:
   - What's failing
   - What was tried
   - What might work
2. Try alternative approach
3. If still stuck, move to next task

### If Blocked on External Dependency
1. Document in `.blocked.md`:
   - What's needed
   - Why it's blocked
   - Workaround if any
2. Continue with other tasks
3. Return when unblocked

---

## Recovery Protocols

### After Pause/Resume
1. Run `git status` to see working state
2. Run `make test` to check current health
3. Review TODO.md for progress tracker
4. Check `.stuck.md` and `.blocked.md` if present
5. Continue from where you left off

---

## Progress Tracking

Maintain `TODO.md` with current status:
```markdown
# Orc v1.0 Progress

## Current Focus
- [ ] Working on: [current feature]

## P0 Features
- [x] Error Standards (100%)
- [ ] Session Interop (75%)
- [ ] Init Wizard (0%)
- [ ] Task Enhancement (0%)

## P1 Features
- [ ] Cost Tracking (0%)
...

## Last Updated
2026-01-10 14:30:00

## Notes
[Any context for next iteration]
```

Update this file:
- At start of each session
- After completing each major checkbox
- Before outputting completion

---

## Code Quality Requirements

### Go Backend
- [ ] `go test ./... -race -cover` passes
- [ ] No race conditions
- [ ] Error handling with context wrapping
- [ ] Consistent naming conventions
- [ ] No TODO comments without ticket

### TypeScript Frontend
- [ ] `npm run lint` passes
- [ ] Svelte 5 runes: `$state`, `$derived`, `$effect`
- [ ] Type safety (no untyped `any`)
- [ ] Component composition
- [ ] Proper error states

### API Design
- [ ] RESTful endpoints
- [ ] Consistent JSON error responses
- [ ] Proper HTTP status codes
- [ ] CORS configured

### Security
- [ ] Input validation
- [ ] No secrets in code
- [ ] Sanitized error messages

---

## Documentation Requirements

- [ ] CLAUDE.md updated with new features
- [ ] README accurate
- [ ] API endpoints documented
- [ ] All specs have testing sections

---

## Iteration Protocol

Each iteration:
1. Review current state (git status, test results)
2. Identify highest priority incomplete item
3. Implement/fix it
4. Run relevant tests
5. If tests pass, commit and continue
6. If tests fail, fix before continuing
7. After major features, run E2E tests

---

## When Complete

When ALL criteria are checked:

1. Create `.ralph-complete` file as a persistent marker
2. Output the completion tag:

```xml
<promise>COMPLETE</promise>
```

---

## File Markers

- `.ralph-complete` - Create when ALL done
- `.stuck.md` - Write when stuck on same error 3+ times
- `.blocked.md` - Write when blocked on external dependency
- `TODO.md` - Track remaining items

---

## Quick Reference

### Commands
```bash
make test           # Run Go tests
make e2e            # Run Playwright E2E
make serve          # Start API server
make web-dev        # Start frontend
make coverage       # Generate coverage report
```

### Test Coverage Target
- **Core paths**: 80%+ coverage
- **Error handling**: 100% coverage
- **New code**: Must have tests

### Commit Pattern
```bash
git commit -m "[orc] FEATURE: description"
```

---

## Runtime Expectations

### Estimated Iterations by Feature
| Feature | Estimated Iterations |
|---------|---------------------|
| Error Standards | 3-5 |
| Session Interop | 5-8 |
| Init Wizard | 8-12 |
| Task Enhancement | 5-8 |
| Cost Tracking | 4-6 |
| Task Templates | 5-8 |
| Web Dashboard | 10-15 |
| Project Detection | 4-6 |
| Keyboard Shortcuts | 6-10 |
| **Total P0+P1** | **50-78** |

### Expected Duration
- Estimated active time: 4-6 hours
- With stuck/blocked states: 1-2 days wall clock

---

## Final Verification Checklist

Before outputting `<promise>COMPLETE</promise>`, verify:

```bash
# 1. All tests pass
make test && make e2e

# 2. Coverage meets target
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# 3. No linting errors
npm run lint --prefix web

# 4. No uncommitted changes
git status

# 5. All completion criteria checked
grep -c "\[x\]" ralph_prompt.md
grep -c "\[ \]" ralph_prompt.md  # Should be 0

# 6. Documentation updated
cat CLAUDE.md | head -50  # Verify new features listed
```

If all pass, create completion:
```bash
touch .ralph-complete
echo "Orc v1.0 complete at $(date)" >> .ralph-complete
```

Then output:
```xml
<promise>COMPLETE</promise>
```
