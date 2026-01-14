# Orc - Claude Code Task Orchestrator

AI-powered task orchestration with phased execution, git worktree isolation, and multi-round review.

## Quick Start

```bash
# Install
go install github.com/randalmurphal/orc/cmd/orc@latest

# Development
make setup && make build    # Build to bin/orc
make test                   # Run tests
make dev-full               # API (:8080) + frontend (:5173)

# Run
./bin/orc init && ./bin/orc new "task description" && ./bin/orc run TASK-001
```

## Claude Code Plugin

The orc plugin for Claude Code lives in a separate lightweight repo to avoid cloning the full codebase:

**Repo:** [randalmurphal/orc-claude-plugin](https://github.com/randalmurphal/orc-claude-plugin)

Install in Claude Code (run once):
```
/plugin marketplace add randalmurphal/orc-claude-plugin
/plugin install orc@orc
```

Commands: `/orc:init`, `/orc:status`, `/orc:continue`, `/orc:review`, `/orc:qa`, `/orc:propose`

## Project Structure

| Path | Purpose | Details |
|------|---------|---------|
| `cmd/orc/` | CLI entry point | - |
| `internal/` | Core packages | See `internal/CLAUDE.md` |
| `templates/` | Phase prompts | See `templates/CLAUDE.md` |
| `web/` | Svelte 5 frontend (current) | See `web/CLAUDE.md` |
| `web-react/` | React 19 frontend (migration) | See `web-react/CLAUDE.md` |
| `docs/` | Architecture, specs, ADRs | See `docs/CLAUDE.md` |

**Key packages:** `api/` (REST + WebSocket), `cli/` (Cobra), `executor/` (phase engine), `task/` (YAML persistence), `git/` (worktrees), `db/` (SQLite), `watcher/` (live refresh)

## Task Execution Model

### Task Organization

Tasks support queue, priority, category, and initiative organization:

| Property | Values | Purpose |
|----------|--------|---------|
| Queue | `active`, `backlog` | Separates current work from "someday" items |
| Priority | `critical`, `high`, `normal`, `low` | Urgency within a queue |
| Category | `feature`, `bug`, `refactor`, `chore`, `docs`, `test` | Type of work for organization and filtering |
| Initiative | Initiative ID (e.g., `INIT-001`) | Groups related tasks under an initiative |

**Queues:**
- **Active** (default): Current work shown on the board
- **Backlog**: Deferred tasks, collapsed by default in each column

**Priorities:**
- **Critical**: Urgent, needs immediate attention (pulsing indicator)
- **High**: Important, should be done soon
- **Normal**: Default priority
- **Low**: Can wait

**Categories:**
- **feature**: New functionality or capability (default)
- **bug**: Bug fix or error correction
- **refactor**: Code restructuring without behavior change
- **chore**: Maintenance tasks (dependencies, cleanup, config)
- **docs**: Documentation changes
- **test**: Test-related changes

**Initiatives:**
- Tasks can optionally belong to an initiative (a group of related tasks)
- Set via `orc new --initiative INIT-001` or `orc edit TASK-001 --initiative INIT-001`
- Unlink via `orc edit TASK-001 --initiative ""`
- Bidirectional sync: setting initiative_id auto-adds task to initiative's task list
- Initiatives can depend on other initiatives via `blocked_by` (see below)

Tasks are sorted within each column by: **running status first** (running tasks always appear at the top), then by priority. Higher priority tasks appear before lower priority tasks.

### Task Dependencies

Tasks support dependency relationships for ordering and tracking:

| Field | Stored | Purpose |
|-------|--------|---------|
| `blocked_by` | Yes | Task IDs that must complete before this task |
| `blocks` | No | Tasks waiting on this task (computed inverse) |
| `related_to` | Yes | Related task IDs (informational) |
| `referenced_by` | No | Tasks mentioning this task ID (auto-detected) |

**CLI usage:**
```bash
orc new "Part 2" --blocked-by TASK-001,TASK-002
orc edit TASK-003 --add-blocker TASK-004
orc edit TASK-003 --remove-blocker TASK-001
```

**Validation:** Self-references rejected, circular dependencies detected, non-existent IDs rejected.

### Initiative Dependencies

Initiatives can depend on other initiatives completing first:

| Field | Stored | Purpose |
|-------|--------|---------|
| `blocked_by` | Yes | Initiative IDs that must complete before this initiative |
| `blocks` | No | Initiatives waiting on this initiative (computed inverse) |

**CLI usage:**
```bash
orc initiative new "React Migration" --blocked-by INIT-001
orc initiative edit INIT-002 --add-blocker INIT-003
orc initiative edit INIT-002 --remove-blocker INIT-001
```

**Example:** 'React Migration' initiative can't start until 'Build System Upgrade' completes:
```
INIT-001: Build System Upgrade → INIT-002: React Migration → INIT-003: Component Library
```

**Blocking rules:**
- Initiative is blocked if ANY blocking initiative is not `completed`
- Can activate a blocked initiative (plan for future work)
- `orc initiative run` warns if initiative is blocked, use `--force` to override

### Weight Classification

| Weight | Phases | Use Case |
|--------|--------|----------|
| trivial | implement | One-liner fix |
| small | implement → test | Bug fix, small feature |
| medium | implement → test → docs | Feature with tests |
| large | spec → implement → test → docs → validate → finalize | Complex feature |
| greenfield | research → spec → implement → test → docs → validate → finalize | New system |

### Completion Detection

Phases complete when Claude outputs:
```xml
<phase_complete>true</phase_complete>
```

Phases block when:
```xml
<phase_blocked>reason: ...</phase_blocked>
```

### Cross-Phase Retry

Failed phases trigger automatic retry from earlier phase:
- `test` fails → retry from `implement`
- `validate` fails → retry from `implement`

Retry phase receives `{{RETRY_CONTEXT}}` with failure details.

### Artifact Detection

When running a task, orc detects existing artifacts and offers to skip phases:

```
📄 spec.md already exists. Skip spec phase? [Y/n]:
```

| Phase | Auto-Skippable | Artifacts Detected |
|-------|----------------|-------------------|
| spec | Yes | `spec.md` with valid content |
| research | Yes | `artifacts/research.md` or in spec |
| docs | Yes | `artifacts/docs.md` |
| implement | No | Never (too complex) |
| test | No | Must re-run against current code |
| validate | No | Must verify current state |
| finalize | No | Must sync with latest target branch |

Use `--auto-skip` to skip automatically without prompting. Skip reasons are recorded in `state.yaml`.

## Configuration

**Hierarchy** (later overrides earlier): Defaults → `/etc/orc/` → `~/.orc/` → `.orc/` → `ORC_*` env

### Automation Profiles

| Profile | Behavior | PR Approval |
|---------|----------|-------------|
| `auto` | Fully automated (default) | AI auto-approves after verifying CI |
| `fast` | Minimal gates, speed over safety | AI auto-approves after verifying CI |
| `safe` | AI reviews, human for merge | Human approval required |
| `strict` | Human gates on spec/review/merge | Human approval required |

```bash
orc run TASK-001 --profile safe
orc config profile strict
```

### Key Config Options

| Option | Purpose | Docs |
|--------|---------|------|
| `storage.mode` | hybrid/files/database | `docs/specs/DATABASE_ABSTRACTION.md` |
| `worktree.enabled` | Git worktree isolation | `docs/architecture/GIT_INTEGRATION.md` |
| `pool.enabled` | OAuth token rotation | - |
| `team.mode` | local/shared_db | `docs/specs/TEAM_ARCHITECTURE.md` |
| `completion.action` | pr/merge/none | - |
| `completion.pr.auto_approve` | AI-assisted PR approval (auto/fast only) | `docs/architecture/EXECUTOR.md` |
| `completion.sync.strategy` | Branch sync timing | `docs/architecture/GIT_INTEGRATION.md` |
| `completion.sync.sync_on_start` | Sync branch before execution (default: true) | `docs/architecture/GIT_INTEGRATION.md` |
| `completion.finalize.enabled` | Enable finalize phase | `docs/architecture/PHASE_MODEL.md` |
| `completion.finalize.auto_trigger_on_approval` | Auto-trigger finalize on PR approval | `docs/architecture/EXECUTOR.md` |
| `completion.finalize.sync.strategy` | Finalize sync: merge/rebase | `docs/architecture/GIT_INTEGRATION.md` |
| `completion.ci.wait_for_ci` | Wait for CI before merge (auto/fast only) | `docs/architecture/EXECUTOR.md` |
| `completion.ci.ci_timeout` | Max time to wait for CI (default: 10m) | `docs/architecture/EXECUTOR.md` |
| `completion.ci.merge_on_ci_pass` | Auto-merge when CI passes (auto/fast only) | `docs/architecture/EXECUTOR.md` |
| `completion.ci.merge_method` | Merge method: squash/merge/rebase | `docs/architecture/GIT_INTEGRATION.md` |
| `artifact_skip.enabled` | Detect existing artifacts | `docs/architecture/PHASE_MODEL.md` |
| `artifact_skip.auto_skip` | Skip without prompting | `docs/architecture/PHASE_MODEL.md` |
| `tasks.disable_auto_commit` | Disable auto-commit for all .orc/ file mutations | `docs/architecture/GIT_INTEGRATION.md` |
| `timeouts.turn_max` | Max time per API turn | `docs/architecture/EXECUTOR.md` |
| `timeouts.heartbeat_interval` | Progress dots interval | `docs/architecture/EXECUTOR.md` |

**All config:** `orc config docs` or `docs/specs/CONFIG_HIERARCHY.md`

## File Layout

```
~/.orc/                          # Global
├── orc.db, config.yaml, projects.yaml, token-pool/

.orc/                            # Project
├── orc.db, config.yaml
├── prompts/                     # Phase prompt overrides
├── worktrees/                   # Isolated worktrees
└── tasks/TASK-001/
    ├── task.yaml, plan.yaml, state.yaml, spec.md
    ├── transcripts/
    ├── attachments/             # Task attachments (images, files)
    └── test-results/            # Playwright test results
        ├── report.json, index.html
        ├── screenshots/
        └── traces/

.claude/                         # Claude Code
├── settings.json, hooks/, skills/

.mcp.json                        # MCP server configuration (auto-generated for UI tasks)
```

## Commands

| Command | Purpose |
|---------|---------|
| `orc go` | Main entry (interactive/headless/quick) |
| `orc init` | Initialize project (<500ms) |
| `orc new "title"` | Create task, classify weight (`-c bug` for category, `-a file` for attachments) |
| `orc run TASK-ID` | Execute phases |
| `orc plan TASK-ID` | Interactive spec creation |
| `orc status` | Show running/blocked/ready/paused |
| `orc deps [TASK-ID]` | Show dependencies (`--tree`, `--graph`) |
| `orc log TASK-ID --follow` | Stream transcript |
| `orc knowledge status` | Knowledge queue stats |

**Full CLI:** `internal/cli/CLAUDE.md` | **Pool:** `orc pool --help` | **Initiatives:** `orc initiative --help` | **Deps:** `orc deps --help`

## Key Patterns

**Error handling:** Always wrap with context
```go
return fmt.Errorf("load task %s: %w", id, err)
```

**Phase execution:** flowgraph with Ralph-style loop
```go
graph := flowgraph.NewGraph[PhaseState]()
graph.AddConditionalEdge("check", routerFunc)
```

**Git commits:** After every phase
```
[orc] TASK-001: implement - completed
```

## Dependencies

Go modules: `llmkit` (Claude wrapper), `flowgraph` (execution), `devflow` (git ops)

For local dev: `make setup` creates `go.work` for sibling directories.

## Web UI

```bash
make serve      # API :8080
make web-dev    # Frontend :5173
```

**Live refresh:** Task board auto-updates when tasks are created/modified/deleted via CLI or filesystem. File watcher monitors `.orc/tasks/` and broadcasts events over WebSocket.

**Project selection:** The server can run from any directory. Project selection persists in URL (`?project=xxx`) and localStorage, surviving page refresh. Use `Shift+Alt+P` to switch projects.

**Initiative filtering:** Tasks can be filtered by initiative using either:
- **Sidebar**: Collapsible Initiatives section with initiative list and progress counts
- **Filter bar**: Initiative dropdown in the Tasks and Board page headers

Both sync to the same state. Options include "All initiatives" (no filter), "Unassigned" (tasks without an initiative), and specific initiatives with task counts. Selection persists in URL (`?initiative=INIT-001`) and localStorage. Click "All initiatives" to clear the filter.

**Board view modes:** The board supports two view modes via dropdown toggle:
- **Flat** (default): Traditional kanban with all tasks in columns
- **By Initiative**: Swimlane view grouping tasks by initiative with collapsible rows, progress bars, and cross-swimlane drag-drop for reassigning tasks

**Keyboard shortcuts:** Uses `Shift+Alt` modifier (⇧⌥ on Mac) to avoid browser conflicts. `Shift+Alt+K` (palette), `Shift+Alt+N` (new task), `g t` (tasks), `j/k` (navigate). Press `?` for full list.

**Settings management:** All settings are editable through the UI:
- Claude Code settings (global `~/.claude/settings.json` + project `.claude/settings.json`) via `/preferences`
- Orc config (`.orc/config.yaml`) via `/environment/orchestrator/automation`

**Task dependencies:** Task detail page shows a collapsible Dependencies sidebar displaying blocked_by, blocks, related_to, and referenced_by relationships with status indicators. Add/remove blockers and related tasks inline.

**Task finalize workflow:** Done column shows different visual states for completed/finalizing/finished tasks:
- **Completed**: Shows finalize button - click to open FinalizeModal and start branch sync
- **Finalizing**: Shows progress bar with step label, pulsing border animation
- **Finished**: Shows merged commit SHA and target branch in green section

**Initiative detail:** Click an initiative in the sidebar or navigate to `/initiatives/:id` to view/manage initiative tasks and decisions. Features include progress tracking, task linking, decision recording, and status management.

See `web/CLAUDE.md` for component architecture.

## Documentation Reference

| Topic | Location |
|-------|----------|
| API Endpoints | `docs/API_REFERENCE.md` |
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phase Model | `docs/architecture/PHASE_MODEL.md` |
| Executor | `docs/architecture/EXECUTOR.md` |
| Config | `docs/specs/CONFIG_HIERARCHY.md` |
| CLI Spec | `docs/specs/CLI.md` |
| File Formats | `docs/specs/FILE_FORMATS.md` |
| Troubleshooting | `docs/guides/TROUBLESHOOTING.md` |

## Testing

```bash
make test       # Backend (Go)
make web-test   # Frontend (Vitest)
make e2e        # E2E (Playwright)

# Visual regression tests
cd web && bunx playwright test --project=visual                    # Compare against baselines
cd web && bunx playwright test --project=visual --update-snapshots # Capture new baselines
```

**Visual regression baselines** are stored in `web/e2e/__snapshots__/` covering Dashboard, Board, Task Detail, and Modal states. See `web/CLAUDE.md` for details.

## UI Testing with Playwright MCP

Tasks involving UI changes automatically get Playwright MCP tools for E2E testing.

### Auto-Detection

When a task's title or description contains UI-related keywords (`button`, `form`, `page`, `modal`, `component`, etc.), orc:
1. Sets `requires_ui_testing: true` in task.yaml
2. Configures Playwright MCP server in `.mcp.json`
3. Creates screenshot directory at `.orc/tasks/{id}/test-results/screenshots/`

### Playwright MCP Tools

| Tool | Purpose |
|------|---------|
| `browser_navigate` | Load pages/routes |
| `browser_snapshot` | Capture accessibility tree (preferred for state verification) |
| `browser_click` | Click elements by ref from snapshot |
| `browser_type` | Type text into inputs |
| `browser_fill_form` | Fill multiple form fields |
| `browser_take_screenshot` | Visual verification |
| `browser_console_messages` | Check for JavaScript errors |
| `browser_network_requests` | Verify API calls |

### Test Results

Results are stored in `.orc/tasks/{id}/test-results/`:
- `report.json` - Structured test report
- `screenshots/` - Test screenshots
- `traces/` - Playwright traces
- `index.html` - HTML report (if generated)

See `docs/specs/FILE_FORMATS.md` for full format specification.

<!-- orc:begin -->
## Orc Orchestration

This project uses [orc](https://github.com/randalmurphal/orc) for task orchestration.

### Slash Commands

| Command | Purpose |
|---------|---------|
| `/orc:init` | Initialize project or create spec |
| `/orc:continue` | Resume current task |
| `/orc:status` | Show progress and next steps |
| `/orc:review` | Multi-round code review |
| `/orc:qa` | E2E tests and documentation |
| `/orc:propose` | Create sub-task for later |

### Task Files

Task specifications and state are stored in `.orc/tasks/`:

```
.orc/tasks/TASK-001/
├── task.yaml      # Task metadata
├── spec.md        # Task specification
├── plan.yaml      # Phase sequence
├── state.yaml     # Execution state
└── attachments/   # Images, files (for screenshots, etc.)
```

### CLI Commands

```bash
orc status           # View active tasks
orc run TASK-001     # Execute task
orc pause TASK-001   # Pause execution
orc resume TASK-001  # Continue task
```

See `.orc/` for configuration and task details.

<!-- orc:end -->

<!-- orc:knowledge:begin -->
## Project Knowledge

Patterns, gotchas, and decisions learned during development.

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Branch sync before completion | Task branches rebase onto target before PR to catch conflicts early | TASK-019 |
| Executor PID tracking | Track executor PID + heartbeat in state.yaml to detect orphaned tasks (running but executor dead) | TASK-046 |
| Atomic status+phase updates | Set `current_phase` atomically with `status=running` to avoid UI timing issues (task shows in wrong column) | TASK-057 |
| Plan regeneration on weight change | When task weight changes, plan.yaml auto-regenerates with new phases; completed/skipped phases preserved if they exist in both plans | TASK-003 |
| Artifact detection for phase skip | Before running phases, check if artifacts exist (spec.md, research.md, docs.md) and offer to skip; use `--auto-skip` for non-interactive mode | TASK-004 |
| Project selection persistence | URL param (`?project=xxx`) takes precedence over localStorage; enables shareable links and browser back/forward navigation | TASK-009 |
| Running task visual indicator | Running tasks show pulsing border + gradient background; placed in column matching `current_phase` from state.yaml | TASK-006 |
| Running task sort priority | Running tasks sort to top of their column before priority sorting; ensures active work is always visible | TASK-028 |
| Live transcript modal | Click running task to open LiveTranscriptModal with streaming output, token tracking, and connection status; uses WebSocket `transcript` events for real-time updates | TASK-012 |
| Worktree-aware project root | `config.FindProjectRoot()` resolves main repo with `.orc/tasks` when running from worktree; uses git common-dir to find main repo | TASK-025 |
| Initiative-task bidirectional sync | Setting `initiative_id` on a task auto-adds it to the initiative's task list; deleting a task removes it from its initiative | TASK-060 |
| Initiative sidebar filtering | Sidebar Initiatives section filters Board/Tasks; URL param (`?initiative=xxx`) takes precedence over localStorage; selection pushes to browser history for back/forward navigation | TASK-061 |
| Initiative filter dropdown | InitiativeDropdown component in filter bars syncs with sidebar; includes "All initiatives", "Unassigned" (tasks with no initiative_id), and initiative list with task counts; uses UNASSIGNED_INITIATIVE constant for special filtering | TASK-062 |
| Editable settings via UI | All settings (Claude Code global/project, orc config) editable through web UI; separate API endpoints for global (`PUT /api/settings/global`) vs project (`PUT /api/settings`) scope | TASK-033 |
| Browser-safe keyboard shortcuts | Web UI uses `Shift+Alt` modifier (⇧⌥ on Mac) for global shortcuts instead of Cmd/Ctrl to avoid browser conflicts with Cmd+K, Cmd+N, etc. | TASK-037 |
| Task dependency validation | `blocked_by` and `related_to` fields validated on create/update: references must exist, no self-references, circular deps rejected; computed fields (`blocks`, `referenced_by`) populated on load | TASK-070 |
| Blocking enforcement on run | CLI and API check `blocked_by` for incomplete blockers before running; CLI prompts in interactive mode, refuses in quiet mode without `--force`; API returns 409 Conflict with blocker details, accepts `?force=true` to override | TASK-071 |
| Dependency visualization CLI | `orc deps` shows dependencies with multiple views: standard (single task), `--tree` (recursive), `--graph` (ASCII flow chart); `orc status` shows BLOCKED/READY sections for dependency-aware task overview | TASK-077 |
| Finalize phase with escalation | Finalize phase syncs branch with target, resolves conflicts via AI, runs tests, and assesses risk; escalates to implement phase if >10 conflicts or >5 test failures persist | TASK-089 |
| Initiative-to-initiative dependencies | Initiatives support `blocked_by` field for ordering; `blocks` computed on load; `orc initiative list/show` displays blocked status; `orc initiative run --force` overrides blocking | TASK-075 |
| Initiative detail page | `/initiatives/:id` route manages tasks and decisions within an initiative; supports task linking/unlinking, decision recording with rationale, status management (draft/active/completed/archived), and progress tracking | TASK-066 |
| Initiative dependency graph | Graph tab in initiative detail shows visual DAG of task dependencies; uses Kahn's algorithm for topological layout; interactive zoom/pan, click-to-navigate, PNG export; API: `GET /api/initiatives/:id/dependency-graph` | TASK-076 |
| PR status polling | Background poller (60s interval, 30s rate limit) tracks PR status via GitHub API; status derived from PR state + reviews (changes_requested > approved > pending_review); stores in task.yaml `pr` field | TASK-090 |
| Board swimlane view | Optional "By Initiative" view groups tasks into horizontal swimlanes; toggle persists in localStorage; disabled when initiative filter active; cross-swimlane drag-drop changes task initiative with confirmation | TASK-065 |
| Auto-trigger finalize on approval | In `auto` profile, finalize phase auto-triggers when PR is approved; controlled by `completion.finalize.auto_trigger_on_approval`; respects 30s rate limit, skips trivial tasks | TASK-091 |
| Finalize UI components | FinalizeModal for progress/results; TaskCard shows finalize button (completed), progress bar (finalizing), merge info (finished); WebSocket `finalize` events for real-time updates | TASK-094 |
| Auto-approve PRs in auto mode | In `auto`/`fast` profiles, PRs are auto-approved after verifying CI passes; uses `gh pr review --approve` with summary comment; `safe`/`strict` profiles require human approval | TASK-099 |
| Initiative auto-commit | Initiative files auto-commit to git and sync to DB on create/modify via CLI; uses `initiative.CommitAndSync()` after each save; watcher monitors `.orc/initiatives/` for external edits | TASK-097 |
| Initiative hybrid storage | Initiatives use YAML as source of truth with DB cache; `initiative_dependencies` table tracks blocked_by; recovery via `RebuildDBIndex()` from YAML or `RecoverFromDB()` from database | TASK-097 |
| CLAUDE.md auto-merge | During git sync, conflicts in knowledge section (within `orc:knowledge:begin/end` markers) are auto-resolved if purely additive (both sides add new table rows); rows combined and sorted by TASK-XXX source ID; complex conflicts (overlapping edits) fall back to manual resolution | TASK-096 |
| Task auto-commit | Task files auto-commit to git on create/modify via CLI; uses `task.CommitAndSync()` after each save; commit messages follow format `[orc] task TASK-001: action - Title`; disable via `tasks.disable_auto_commit` config | TASK-153 |
| CI wait and auto-merge | After finalize, poll `gh pr checks` until CI passes (30s interval, 10m timeout), then merge via `gh pr merge --squash`; bypasses GitHub auto-merge feature (no branch protection needed); `auto`/`fast` profiles only | TASK-151 |
| Comprehensive auto-commit | ALL .orc/ file mutations auto-commit to git: task lifecycle (status, state, phase transitions), initiative operations (status, linking, decisions), API/UI changes (config, prompts, projects), PR status updates; `state.CommitTaskState()` and `state.CommitPhaseTransition()` for executor; `autoCommit*()` helpers in API handlers; disable via `tasks.disable_auto_commit` config | TASK-193 |
| WebSocket E2E event injection | Use Playwright's `routeWebSocket` to intercept connections and inject events via `ws.send()`; captures real WebSocket, forwards messages bidirectionally, allows test-initiated events; framework-agnostic approach for testing real-time UI updates | TASK-157 |
| Visual regression baselines | Separate Playwright project (`visual`) with 1440x900 @2x viewport, disabled animations, masked dynamic content (timestamps, tokens); use `--update-snapshots` to regenerate after intentional UI changes; baselines in `web/e2e/__snapshots__/` | TASK-159 |
| Keyboard shortcut E2E testing | Test multi-key sequences (g+d, g+t) with sequential `page.keyboard.press()` calls; test Shift+Alt modifiers; verify input field awareness (shortcuts disabled when typing); use `.selected` class for task navigation; 13 tests in `web/e2e/keyboard-shortcuts.spec.ts` | TASK-160 |
| Finalize workflow E2E testing | Test finalize modal states (not started, running, completed, failed) via WebSocket event injection; covers button visibility on completed tasks, modal content, progress bar with step labels, success/failure results, retry option; 10 tests in `web/e2e/finalize.spec.ts` | TASK-161 |

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
| PR labels in config don't exist on repo | Orc warns and creates PR without labels (graceful degradation) | TASK-015 |
| `go:embed` fails without static dir | Run `make test` (creates placeholder) or `mkdir -p internal/api/static` | TASK-016 |
| Tests fail with `go.work` | Use `GOWORK=off go test` or `make test` | TASK-016 |
| Raw `InputTokens` appears misleadingly low | Use `EffectiveInputTokens()` which adds cached tokens to get actual context size | TASK-010 |
| Task stuck in "running" after crash | Use `orc resume TASK-XXX` (auto-detects orphaned state) or `--force` to override | TASK-046 |
| Failed task can't be resumed | Fixed: `orc resume` now supports failed tasks, resuming from last incomplete phase | TASK-025 |
| Spurious "Task deleted" toast notifications | Fixed: Watcher now verifies deletions with debounce to filter false positives from git ops/atomic saves | TASK-053 |
| Setup errors (worktree creation) failed silently | Fixed: Errors now always display even in quiet mode, task status set to failed | TASK-044 |
| Web UI shows "No project selected" | Select a project via `Shift+Alt+P` - server can run from any directory | TASK-005 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| Sync at completion (default) | Balance safety vs overhead; phase-level sync adds latency for marginal benefit | TASK-019 |

<!-- orc:knowledge:end -->
