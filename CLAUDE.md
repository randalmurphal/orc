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
| `web/` | React 19 frontend | See `web/CLAUDE.md` |
| `web-svelte-archive/` | Svelte 5 frontend (archived) | Previous implementation |
| `docs/` | Architecture, specs, ADRs | See `docs/CLAUDE.md` |

**Key packages:** `api/` (REST + WebSocket), `cli/` (Cobra), `executor/` (phase engine), `task/` (task model), `storage/` (database backend), `git/` (worktrees), `db/` (SQLite)

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
| `diagnostics.resource_tracking.enabled` | Enable process/memory tracking | `docs/guides/TROUBLESHOOTING.md` |
| `diagnostics.resource_tracking.memory_threshold_mb` | Memory growth warning threshold | `docs/guides/TROUBLESHOOTING.md` |

**All config:** `orc config docs` or `docs/specs/CONFIG_HIERARCHY.md`

## File Layout

```
~/.orc/                          # Global
├── orc.db                       # Global database (projects, cost, templates)
├── config.yaml                  # Global configuration
├── projects.yaml                # Project registry
└── token-pool/                  # OAuth token pool

.orc/                            # Project
├── orc.db                       # Project database (tasks, states, plans, specs, initiatives)
├── config.yaml                  # Project configuration
├── prompts/                     # Phase prompt overrides (files)
└── worktrees/TASK-001/          # Isolated worktrees per task
    └── test-results/            # Playwright test results
        ├── report.json, screenshots/, traces/

.claude/                         # Claude Code
├── settings.json, hooks/, skills/

.mcp.json                        # MCP server configuration (auto-generated for UI tasks)
```

**Note:** Task data (tasks, plans, states, specs, initiatives) is stored in SQLite (`orc.db`), not YAML files. Use `orc show TASK-001 --format yaml` for human-readable export.

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

### Running the Server

**Production/Built mode:**
```bash
make build      # Builds frontend + backend to bin/orc
orc serve &     # Start server (port shown in output, default :8080)
```

**Development mode** (hot reload):
```bash
make serve      # API only :8080
make web-dev    # Frontend :5173 (separate terminal)
```

If port 8080 is in use, the frontend can run standalone with `make web-dev` while pointing to a different API port via environment variable.

**Live refresh:** Task board auto-updates when tasks are created/modified/deleted via CLI or API. Database operations publish events over WebSocket.

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

### Component Library

**Button Primitive**: Unified button component used across all pages. Features variants (primary, secondary, danger, ghost, success), sizes (sm, md, lg), icon support (leftIcon, rightIcon, iconOnly), and loading state. Provides consistent accessibility (focus-visible, aria-disabled, aria-busy). Icon-only buttons require `aria-label`.

**Radix UI Primitives**: Accessible components with automatic keyboard navigation and ARIA attributes.

| Component | Package | Usage |
|-----------|---------|-------|
| DropdownMenu | `@radix-ui/react-dropdown-menu` | TaskCard quick menu, ExportDropdown (action menus) |
| Select | `@radix-ui/react-select` | InitiativeDropdown, ViewModeDropdown, DependencyDropdown |
| Tabs | `@radix-ui/react-tabs` | TabNav in task detail |
| Tooltip | `@radix-ui/react-tooltip` | Replace native `title` attributes |
| Dialog | `@radix-ui/react-dialog` | Modal.tsx (focus trap, ESC close) |

**Radix Patterns**:
- Portal to `document.body` by default (avoids z-index issues)
- Style via `data-*` attributes: `data-state="open|closed"`, `data-highlighted`, `data-disabled`
- Trigger uses `asChild` to wrap existing Button components
- Select requires string values (map `null` to internal constants like `'__all__'`)
- Global animations in `index.css` respect `prefers-reduced-motion`

**Keyboard Navigation** (automatic via Radix):

| Component | Keys |
|-----------|------|
| DropdownMenu/Select | Arrow keys, Enter, Escape, Home/End, typeahead |
| Tabs | Arrow left/right, Home/End, Tab to panel |
| Dialog | Escape closes, Tab cycles within focus trap |
| Tooltip | Focus shows, blur hides |

**E2E Validation**: Three test files (`ui-primitives.spec.ts`, `radix-a11y.spec.ts`, `axe-audit.spec.ts`) validate behavior, keyboard accessibility, and WCAG 2.1 AA compliance.

See `web/CLAUDE.md` for component architecture and full API documentation.

## Documentation Reference

| Topic | Location |
|-------|----------|
| API Endpoints | `docs/API_REFERENCE.md` |
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phase Model | `docs/architecture/PHASE_MODEL.md` |
| Executor | `docs/architecture/EXECUTOR.md` |
| Branch Targeting | `docs/architecture/BRANCH_TARGETING.md` |
| Config | `docs/specs/CONFIG_HIERARCHY.md` |
| CLI Spec | `docs/specs/CLI.md` |
| File Formats | `docs/specs/FILE_FORMATS.md` |
| Troubleshooting | `docs/guides/TROUBLESHOOTING.md` |
| Radix UI Adoption | `docs/decisions/ADR-008-radix-adoption.md` |
| Web UI Components | `web/CLAUDE.md` |

## Testing

```bash
make test       # Backend (Go)
make web-test   # Frontend (Vitest)
make e2e        # E2E (Playwright)

# Visual regression tests
cd web && npx playwright test --project=visual                    # Compare against baselines
cd web && npx playwright test --project=visual --update-snapshots # Capture new baselines
```

**⚠️ CRITICAL: E2E Test Sandbox Isolation**

E2E tests run against an **ISOLATED SANDBOX PROJECT** in `/tmp`, NOT the real orc project. Tests perform real actions (drag-drop, clicks, API calls) that modify task statuses. Running against production data WILL corrupt real task states.

Test files MUST import from `./fixtures` (not `@playwright/test`) to get automatic sandbox selection. See `web/CLAUDE.md` for details.

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
| Initiative database storage | Initiatives stored in SQLite (`initiatives`, `initiative_tasks`, `initiative_decisions`, `initiative_dependencies` tables); `initiative.Save()` writes directly to database; CLI operations auto-commit changes | TASK-097 |
| CLAUDE.md auto-merge | During git sync, conflicts in knowledge section (within `orc:knowledge:begin/end` markers) are auto-resolved if purely additive (both sides add new table rows); rows combined and sorted by TASK-XXX source ID; complex conflicts (overlapping edits) fall back to manual resolution | TASK-096 |
| Task database storage | Tasks stored in SQLite (`tasks`, `phases`, `plans`, `specs` tables); all task operations write directly to database; config changes and prompt files still tracked in git | TASK-153 |
| CI wait and auto-merge | After finalize, poll `gh pr checks` until CI passes (30s interval, 10m timeout), then merge via GitHub REST API (`PUT /repos/.../pulls/.../merge`); bypasses GitHub auto-merge feature (no branch protection needed); `auto`/`fast` profiles only | TASK-151 |
| Config and prompt auto-commit | Configuration files (`.orc/config.yaml`) and prompt templates (`.orc/prompts/`) auto-commit to git on modification; task and initiative data stored in SQLite (not git-tracked) | TASK-193 |
| WebSocket E2E event injection | Use Playwright's `routeWebSocket` to intercept connections and inject events via `ws.send()`; captures real WebSocket, forwards messages bidirectionally, allows test-initiated events; framework-agnostic approach for testing real-time UI updates | TASK-157 |
| Visual regression baselines | Separate Playwright project (`visual`) with 1440x900 @2x viewport, disabled animations, masked dynamic content (timestamps, tokens); use `--update-snapshots` to regenerate after intentional UI changes; baselines in `web/e2e/__snapshots__/` | TASK-159 |
| Keyboard shortcut E2E testing | Test multi-key sequences (g+d, g+t) with sequential `page.keyboard.press()` calls; test Shift+Alt modifiers; verify input field awareness (shortcuts disabled when typing); use `.selected` class for task navigation; 13 tests in `web/e2e/keyboard-shortcuts.spec.ts` | TASK-160 |
| Finalize workflow E2E testing | Test finalize modal states (not started, running, completed, failed) via WebSocket event injection; covers button visibility on completed tasks, modal content, progress bar with step labels, success/failure results, retry option; 10 tests in `web/e2e/finalize.spec.ts` | TASK-161 |
| Sync on start for stale worktrees | Before execution starts, sync task branch with target to catch conflicts from parallel tasks; `sync_on_start: true` (default) rebases onto latest target so implement phase sees current code; disable if you need isolation from concurrent changes | TASK-194 |
| Resource tracking for orphan detection | Executor snapshots processes before/after task; compares to detect orphaned MCP processes (playwright, chromium); logs warnings with process details and memory growth; configure via `diagnostics.resource_tracking` in config | TASK-197 |
| E2E sandbox isolation | E2E tests MUST run against isolated sandbox project in `/tmp`, not production; `global-setup.ts` creates sandbox with test tasks/initiatives, `global-teardown.ts` removes it; test files import from `./fixtures` (not `@playwright/test`) to auto-select sandbox; tests that bypass fixtures will corrupt real task data | TASK-201 |
| React migration complete | Frontend migrated from Svelte 5 to React 19; archived Svelte codebase at `web-svelte-archive/`, moved React to `web/`; E2E tests use framework-agnostic selectors (role, text, CSS classes) | TASK-180 |
| Resolve with worktree cleanup | `orc resolve` detects worktree state (dirty, rebase/merge in progress, conflicts) and offers `--cleanup` to abort git ops and discard changes; `--force` skips checks; worktree state recorded in task metadata for audit | TASK-221 |
| Multi-table DB transactions | Operations spanning multiple tables (task+dependencies, state+phases, initiative+decisions+tasks) wrapped in `RunInTx()` for atomicity; transaction-aware functions (`SaveTaskTx`, `SavePhaseTx`, etc.) use `TxOps` context; rollback on any error ensures consistency | TASK-223 |
| Button primitive for board components | All board buttons (TaskCard, QueuedColumn, Swimlane, ViewModeDropdown) use Button component with `variant="ghost" size="sm"` for consistency; preserve existing CSS classes via `className` prop for backwards compatibility; icon-only buttons require `aria-label` | TASK-207 |
| Button primitive migration | Dashboard and layout components migrated from raw `<button>` to unified Button component; preserve existing CSS class names via `className` prop; use `variant="ghost"` for minimal styling, `variant="primary"` for primary actions | TASK-209 |
| FinalizeTracker cleanup | `finalizeTracker` (in-memory map of finalize operations) auto-cleans completed/failed entries after 5 min retention; `startCleanup()` runs in background goroutine triggered by `Server.StartContext()`; running/pending entries never cleaned (active operations); prevents unbounded memory growth from completed finalize operations | TASK-225 |
| Initiative batch loading | `LoadAllInitiatives()` uses batch queries (`GetAllInitiativeDecisions`, `GetAllInitiativeTaskRefs`, etc.) to fetch all related data in 4 queries instead of N+1 per initiative; batch methods return maps keyed by initiative ID; `GetAllInitiativeTaskRefs()` JOINs tasks table to include title/status | TASK-234 |
| WorkerPool self-cleanup | Workers remove themselves from `WorkerPool.workers` map immediately on completion/failure via defer in `run()`; frees capacity without waiting for next orchestrator tick; handlers check `GetWorker()` first (idempotent - worker may have self-cleaned) | TASK-238 |
| Finalize goroutine cancellation | `runFinalizeAsync` goroutines use contexts derived from `Server.serverCtx`; `finalizeTracker.cancels` map stores cancel functions; server shutdown calls `cancelAll()` to terminate running operations; goroutines check `ctx.Err()` at key points to exit cleanly | TASK-226 |
| Scheduler map cleanup | `Scheduler.MarkCompleted()` cleans up `taskDeps` immediately and prunes `completed` entries no longer needed by queued/running tasks; `RemoveTask()` available for failed tasks that won't be retried; prevents unbounded memory growth in long-running orchestrator | TASK-239 |
| Context propagation in DatabaseBackend | `SaveTaskCtx`, `SaveStateCtx`, `SaveInitiativeCtx` methods propagate context through `RunInTx` to `TxOps`; enables request cancellation and timeouts for database operations; non-context methods use `context.Background()` for backward compatibility | TASK-240 |
| Worker.run() iterative phase loop | `Worker.run()` uses `for` loop (not recursion) to execute phases sequentially; eliminates stack growth risk for tasks with many phases; loop continues until all phases complete, context cancelled, or error occurs | TASK-230 |
| Git mutex for compound operations | `Git` struct has mutex protecting compound operations (worktree creation, checkpoint, rebase, conflict detection); individual git commands are process-atomic and don't need locking; worktree instances get independent mutexes for parallel execution | TASK-235 |
| Finalize tracker atomic tryStart | `finalizeTracker.tryStart()` atomically checks and sets finalize state to prevent TOCTOU race; returns existing state if pending/running, allows replacement if completed/failed (retry); eliminates race where concurrent requests both pass the check and both start finalize | TASK-236 |
| Worker process group cleanup | Workers set `Setpgid: true` on commands to create process groups; `Stop()` kills entire process group via negative PID (`syscall.Kill(-pid, SIGKILL)`); prevents orphaned MCP processes (playwright, chromium) after worker shutdown; platform-specific files: `worker_unix.go` (real), `worker_windows.go` (no-op) | TASK-222 |
| TaskCard Radix DropdownMenu | TaskCard quick menu uses `@radix-ui/react-dropdown-menu` for accessible menu; trigger wraps Button with `DropdownMenu.Trigger asChild`; content portals via `DropdownMenu.Portal`; uses `data-highlighted` for keyboard focus, `onCloseAutoFocus` prevents refocus; CSS class `.quick-menu-dropdown` for styling | TASK-212 |
| Test worker limits for parallel tasks | Playwright (4 workers) and Vitest (4 threads) limit parallelism to prevent OOM when multiple orc tasks run tests concurrently; without limits, 3 parallel tasks on 16 cores could spawn 48 browser/test processes | TASK-253 |
| Filter dropdown Radix migration | InitiativeDropdown, ViewModeDropdown, DependencyDropdown use Radix Select; ExportDropdown uses Radix DropdownMenu (action menu pattern); null values mapped to internal string constants since Radix Select requires strings; provides keyboard nav, typeahead, Home/End, ARIA | TASK-213 |
| TabNav Radix Tabs migration | TabNav uses `@radix-ui/react-tabs` with render prop pattern for tab content; Tabs.Root wraps entire component, Tabs.List contains Tabs.Trigger elements, Tabs.Content wraps the children render prop result; provides arrow key navigation, Home/End, focus management, automatic ARIA; CSS uses `data-state='active'` for active tab styling | TASK-214 |
| Radix Tooltip component | `Tooltip` wraps `@radix-ui/react-tooltip` with consistent styling; `TooltipProvider` at App.tsx root provides global delay config (300ms); supports rich content (JSX), controlled mode, placement, arrow; portals to `document.body`; use instead of native `title` for accessibility and consistent styling | TASK-215 |
| UI primitives E2E testing | Three test files validate Radix integration: `ui-primitives.spec.ts` (22 tests: Button, DropdownMenu, Select, Tabs, Tooltip), `radix-a11y.spec.ts` (17 tests: keyboard navigation, focus trap, ARIA attributes), `axe-audit.spec.ts` (8 tests: WCAG 2.1 AA compliance via axe-core); selector strategy prioritizes role/aria-label over CSS classes | TASK-216 |
| Branch resolution 5-level hierarchy | `ResolveTargetBranch()` resolves target branch with priority: task override → initiative branch → developer staging → project config → "main"; each level can override lower levels; source tracking via `ResolveBranchSource()` for debugging | branch-targeting |
| Branch lazy creation | Target branches (initiative, staging) created on first task run via `EnsureTargetBranchExists()` in setup; default branches (main, master, develop) never auto-created; prevents orphan branches from unused initiatives | branch-targeting |
| Branch registry tracking | All orc-managed branches tracked in `branches` table with type (initiative/staging/task), owner_id, status (active/merged/stale/orphaned), timestamps; enables `orc branches list/cleanup` for lifecycle management | branch-targeting |
| Initiative branch auto-merge | When all initiative tasks complete and initiative has `BranchBase`, auto-merge to target branch; `auto`/`fast` profiles auto-merge after CI, `safe`/`strict` leave PR for human review; tracks `MergeStatus` (none/pending/merged/failed) | branch-targeting |
| Developer staging workflow | Personal staging branches via `developer.staging_branch` + `staging_enabled` in personal config; `orc staging status/sync/enable/disable` commands; staging takes precedence over project default but yields to initiative branches | branch-targeting |

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
| PR labels in config don't exist on repo | Orc warns and creates PR without labels (graceful degradation) | TASK-015 |
| `go:embed` fails without static dir | Run `make test` (creates placeholder) or `mkdir -p internal/api/static` | TASK-016 |
| Tests fail with `go.work` | Use `GOWORK=off go test` or `make test` | TASK-016 |
| Raw `InputTokens` appears misleadingly low | Use `EffectiveInputTokens()` which adds cached tokens to get actual context size | TASK-010 |
| Task stuck in "running" after crash | Use `orc resume TASK-XXX` (auto-detects orphaned state) or `--force` to override | TASK-046 |
| Failed task can't be resumed | Fixed: `orc resume` now supports failed tasks, resuming from last incomplete phase | TASK-025 |
| Setup errors (worktree creation) failed silently | Fixed: Errors now always display even in quiet mode, task status set to failed | TASK-044 |
| Web UI shows "No project selected" | Select a project via `Shift+Alt+P` - server can run from any directory | TASK-005 |
| Auto-merge fails with worktree error | Fixed: Uses GitHub REST API for merge instead of `gh pr merge` CLI which tried to checkout target branch locally | TASK-196 |
| Finished tasks still blocked dependents | Fixed: `GetIncompleteBlockers()` now uses `isDone()` helper to recognize both `completed` and `finished` statuses as done | TASK-199 |
| Re-running completed task fails to push | Fixed: Push now detects non-fast-forward errors (diverged remote) and automatically retries with `--force-with-lease` | TASK-198 |
| Sync fails with '0 files in conflict' error | Fixed: `RebaseWithConflictCheck()` now only returns `ErrMergeConflict` when actual conflicts detected; other rebase failures (dirty tree, rebase in progress) return the raw error | TASK-201 |
| PRPoller.Stop() panics on double call | Fixed: `Stop()` now uses `sync.Once` to guard channel close; safe to call multiple times from concurrent shutdown paths | TASK-231 |
| Total time shows 292 years | Fixed: `State.Elapsed()` method checks for zero `StartedAt` before calling `time.Since()`; returns 0 for uninitialized states instead of Unix epoch difference | TASK-243 |
| Running tasks falsely flagged as orphaned | Fixed: `SaveState()` now persists `ExecutionInfo` (PID, hostname, heartbeat) to database; `LoadState()` restores it; orphan detection works correctly across orc restarts | TASK-242 |
| SaveTask overwrites executor fields | Fixed: `SaveTask()` now preserves executor fields (PID, hostname, heartbeat) when updating task metadata; prevents false orphan detection when CLI/API updates a running task | TASK-249 |
| detectConflictsViaMerge left worktree in merge state | Fixed: Cleanup (`merge --abort` + `reset --hard`) now uses `defer` to guarantee execution even on error/panic; idempotent cleanup is safe even if merge wasn't started | TASK-229 |
| Resume blocked by dirty worktree state | Fixed: `SetupWorktree` now calls `cleanWorktreeState` when reusing existing worktree; aborts in-progress rebase/merge and discards uncommitted changes; enables resume after task failure without manual cleanup | TASK-247 |
| Template variables not substituted in prompts | Fixed: Flowgraph executor's `renderTemplate()` now includes all variables from standard `RenderTemplate()`: `{{TASK_CATEGORY}}`, `{{INITIATIVE_CONTEXT}}`, `{{REQUIRES_UI_TESTING}}`, `{{SCREENSHOT_DIR}}`, `{{TEST_RESULTS}}`, `{{COVERAGE_THRESHOLD}}`, `{{REVIEW_FINDINGS}}` | TASK-278 |
| Date shows '12/31/1' instead of '12/31/2001' | Fixed: `toLocaleDateString()` without options can produce abbreviated years; use explicit options `{ year: 'numeric', month: 'numeric', day: 'numeric' }` to ensure 4-digit year display; also add null/invalid date guards | TASK-255 |
| Dashboard initiative progress shows 0/0 | Fixed: `DashboardInitiatives` was calculating progress from `initiative.tasks` (unpopulated by API) while Sidebar used `getInitiativeProgress(tasks)` from task store; now both use task store for consistent counts | TASK-276 |
| View mode dropdown disabled on clean URL | Fixed: `swimlaneDisabled` was using store value `currentInitiativeId` which includes localStorage-persisted state; changed to use URL param `searchParams.get('initiative')` as source of truth; clean URL (`/board`) now enables dropdown even with stale localStorage filter | TASK-275 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| Sync at completion (default) | Balance safety vs overhead; phase-level sync adds latency for marginal benefit | TASK-019 |

<!-- orc:knowledge:end -->
