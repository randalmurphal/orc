# Orc - Claude Code Task Orchestrator

AI-powered task orchestration with phased execution, git worktree isolation, and multi-round review.

## ⚠️ Code Quality: Non-Negotiable Rules

**These rules override any default behavior. Violations cause bugs that waste hours.**

### 1. ONE Way to Do Things
Before writing new code, check for existing patterns. If similar code exists, consolidate into ONE shared function/interface. Don't create parallel implementations.

| Situation | Action |
|-----------|--------|
| Need schema-constrained LLM call | Use `llmutil.ExecuteWithSchema[T]()` - the ONLY way |
| Need phase completion parsing | Use `CheckPhaseCompletionJSON()` - returns error, handle it |
| Similar logic in 2+ places | Extract to shared function with parameters |

### 2. NO Fallbacks, NO Silent Failures
Every error MUST be handled explicitly. Never swallow errors or return "success" on failure.

| ❌ NEVER | ✅ ALWAYS |
|----------|-----------|
| `if err != nil { return defaultValue }` | `if err != nil { return err }` |
| Silent continue on parse failure | Return error, let caller decide |
| Fallback to alternative field | Error if expected field missing |
| Ignore function return values | Check and propagate all errors |

**Example - JSON schema handling:**
- Asked for `structured_output` → MUST get it or ERROR
- Parse failure → ERROR (not silent continue)
- Empty response → ERROR (not fallback to `result`)

### 3. Remove Code Completely
When removing functionality, DELETE it. Don't deprecate, don't keep as "legacy fallback", don't comment out.

| ❌ NEVER | ✅ ALWAYS |
|----------|-----------|
| `// Deprecated: use NewFunc` | Delete old function |
| `if useLegacy { oldCode() }` | Remove old code path |
| Keep "just in case" code | Delete it, git has history |

**Exception:** Only keep legacy code if explicitly specified for migration period with removal date.

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

## Project Structure

| Path | Purpose | Details |
|------|---------|---------|
| `cmd/orc/` | CLI entry point | - |
| `internal/` | Core packages | See `internal/CLAUDE.md` |
| `templates/` | Phase prompts | See `templates/CLAUDE.md` |
| `web/` | React 19 frontend | See `web/CLAUDE.md` |
| `docs/` | Architecture, specs, ADRs | See `docs/CLAUDE.md` |

**Key packages:** `api/` (REST + WebSocket), `cli/` (Cobra), `executor/` (phase engine), `workflow/` (workflow definitions), `task/` (task model), `storage/` (database backend), `git/` (worktrees), `db/` (SQLite + GlobalDB/ProjectDB), `project/` (multi-project registry), `gate/` (quality gates: auto/human/AI/skip), `events/` (real-time event publishing), `trigger/` (lifecycle event triggers), `jira/` (Jira Cloud import)

## Task Model

### What Makes Tasks Succeed

**For non-trivial tasks, orc REQUIRES a specification with:**

| Section | Purpose | Validation |
|---------|---------|------------|
| **Intent** | Why this work matters, what problem it solves | Must have meaningful content |
| **Success Criteria** | Testable conditions proving the work is done | Must have specific, verifiable items |
| **Testing** | How to verify the implementation works | Must define test types and acceptance criteria |

The spec phase generates these from your task description. **Vague input → vague spec → poor results.**

Run `orc new --help` for detailed guidance on creating tasks that execute well.

### Weight Classification (Determines Required Phases)

| Weight | Phases | Spec? | When to Use |
|--------|--------|-------|-------------|
| trivial | implement | NO | One-liner fixes, typos |
| small | tiny_spec → implement → review | YES | Bug fixes, isolated changes |
| medium | spec → tdd_write → implement → review → docs | YES | Features needing thought |
| large | spec → tdd_write → breakdown → implement → review → docs | YES | Complex multi-file features |

Key phases:
- **spec/tiny_spec**: Generates Success Criteria + Testing requirements (foundation for quality)
- **tdd_write**: Writes failing tests BEFORE implementation (context isolation)
- **breakdown**: Decomposes large tasks into checkboxed implementation steps
- **review**: Multi-agent code review with 6 specialized reviewers + no-op detection + success criteria verification

### Task Completion Flow

1. **Task completes** → PR created or reused on hosting provider (GitHub or GitLab) if `completion.action: pr`
2. **Review PR** → Manual review opportunity
3. **`orc finalize TASK-XXX`** → Syncs with target branch, resolves conflicts, optionally enables auto-merge

**PR creation is idempotent:** If an open PR already exists on the task branch (from a previous run), it's reused and updated rather than creating a duplicate. See `executor/CLAUDE.md` for the full flow.

**Note:** Auto-merge and CI polling are **disabled by default**. Set `completion.pr.auto_merge: true` and `completion.ci.wait_for_ci: true` to enable. GitHub auto-merge requires GraphQL (not supported); GitLab auto-merge is fully supported via `MergeWhenPipelineSucceeds`.

⚠️ **Common mistake**: Under-weighting tasks. A "medium" task run as "small" skips the spec phase, causing Claude to guess requirements.

### Task Properties

| Property | Values | Purpose |
|----------|--------|---------|
| Queue | `active`, `backlog` | Current work vs "someday" |
| Priority | `critical`, `high`, `normal`, `low` | Urgency |
| Category | `feature`, `bug`, `refactor`, `chore`, `docs`, `test` | Affects how Claude approaches work |
| Initiative | Initiative ID | Groups tasks with shared vision/decisions |
| Description | Free text | **Flows into every phase prompt** - be specific! |
| Branch Name | Custom branch name | Override default `orc/TASK-XXX` naming |
| Target Branch | Branch name | PR target (default: repo default branch) |
| PR Options | Draft, labels, reviewers | Control PR creation behavior |

### Dependencies

Tasks support `blocked_by` (must complete first) and `related_to` (informational). CLI: `orc new "Part 2" --blocked-by TASK-001`. Initiatives also support `blocked_by` for ordering.

### Initiatives (Shared Context)

When tasks are part of a larger feature, link them to an initiative:

```bash
orc initiative new "User Auth" -V "JWT-based auth with refresh tokens"
orc initiative decide INIT-001 "Use bcrypt for passwords" -r "Industry standard"
orc new "Login endpoint" -i INIT-001 -w medium
```

The initiative's **Vision** and **Decisions** flow into every linked task's prompts, keeping Claude aligned across multiple tasks.

### Completion Detection

Phases complete when Claude outputs JSON with `{"status": "complete", ...}`. Blocked phases output `{"status": "blocked", "reason": "..."}`. Failed phases trigger retry from earlier phase with `{{RETRY_CONTEXT}}`.

## Configuration

**Hierarchy:** Defaults -> `/etc/orc/` -> `~/.orc/` -> `.orc/` -> `ORC_*` env

### Automation Profiles

| Profile | Behavior | PR Approval |
|---------|----------|-------------|
| `auto` | Fully automated (default) | AI auto-approves |
| `fast` | Speed over safety | AI auto-approves |
| `safe` | AI reviews, human merge | Human required |
| `strict` | Human gates throughout | Human required |

See `docs/specs/CONFIG_HIERARCHY.md` for all options.

### Jira Integration

Import Jira Cloud issues as orc tasks via `orc import jira`. Configure in `.orc/config.yaml` under `jira:` (URL, email, token env var, custom field mappings, default projects, mapping overrides). Epics map to initiatives by default. See `orc import jira --help` for setup.

### Constitution

Project-level principles injected into all phase prompts via `{{CONSTITUTION_CONTENT}}`.
Stored at `.orc/CONSTITUTION.md` (git-tracked):

```bash
orc constitution show                        # View current
orc constitution set --file myprinciples.md  # Set from file
orc constitution delete                      # Remove
```

## Multi-Project Support

Orc supports multiple projects from a single installation. Data is split between a global database and per-project databases.

### Architecture

| Component | Location | Contents |
|-----------|----------|----------|
| **GlobalDB** | `~/.orc/orc.db` | Project registry, cost tracking, budgets, workflows, agents, phase templates |
| **ProjectDB** | `<project>/.orc/orc.db` | Tasks, initiatives, transcripts, phases, events, FTS |
| **ProjectCache** | `internal/api/project_cache.go` | LRU cache for project DB connections (thread-safe) |

### How It Works

- All 13 API services accept `project_id` in request messages, routed via `getBackend(projectID)` (`internal/api/*_server.go`)
- CLI resolves project via: `--project` flag > `ORC_PROJECT` env > cwd detection (`internal/cli/project_context.go`)
- Frontend passes `projectId` from store to all API calls; project picker at `web/src/pages/ProjectPickerPage.tsx`
- Projects auto-register on `orc init`; registry at `~/.orc/projects.json` (`internal/project/`)

## File Layout

```
~/.orc/                          # Global: config, orc.db (GlobalDB), projects.json, token pool
<project>/.orc/                  # Per-project: orc.db (ProjectDB), config, prompts, worktrees
.claude/                         # Claude Code settings, hooks, skills
```

Task data stored in per-project SQLite (`<project>/.orc/orc.db`). Use `orc export --all-tasks --all` for full backup to `.orc/exports/`.

## Commands

**Always run `orc <command> --help` for detailed usage with quality guidance.**

### Core Workflow

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `orc new "title"` | Create task with full control | `-w weight`, `-d description`, `-i initiative`, `--branch`, `--target-branch`, `--pr-draft` |
| `orc run TASK-ID` | Execute task phases | `--profile`, `--auto-skip`, `--stream`, `--skip-gates` |
| `orc status` | Dashboard: what needs attention | `--watch`, `--all` |

### Task Management

| Command | Purpose |
|---------|---------|
| `orc show TASK-ID` | View task details, spec, state (`--gates` for gate history) |
| `orc deps TASK-ID` | Show dependencies (`--tree`, `--graph`) |
| `orc log TASK-ID` | View Claude transcripts (`--follow` for streaming) |
| `orc resume TASK-ID` | Continue paused/failed/orphaned task |
| `orc approve TASK-ID` | Approve blocked gate |
| `orc resolve TASK-ID` | Mark failed task as resolved |
| `orc gates list` | Show gate config for all workflow phases |
| `orc gates show <phase>` | Detailed gate config for a phase |

### Initiatives

| Command | Purpose |
|---------|---------|
| `orc initiative new "title"` | Create initiative with `--vision` |
| `orc initiative decide ID "decision"` | Record decision with `--rationale`, `--supersedes N` |
| `orc initiative edit ID` | Edit properties (`--title`, `--status`, `--vision`, `--priority`, `--add-blocked-by`) |
| `orc initiative link ID TASK...` | Batch link tasks |
| `orc initiative run ID` | Run all ready tasks in order |

Run `orc initiative --help` for full subcommand list.

### Project Management

| Command | Purpose |
|---------|---------|
| `orc projects` | List registered projects |
| `orc projects add .` | Register current directory |
| `orc projects remove ID` | Unregister a project |
| `orc projects default ID` | Set default project |

**Global flag:** `--project/-P` or `ORC_PROJECT` env var selects the active project for any command.

### Data Portability

| Command | Purpose |
|---------|---------|
| `orc export --all-tasks` | Full backup (tar.gz) to `.orc/exports/` |
| `orc export --all-tasks --initiatives` | Include initiatives |
| `orc export --all-tasks --minimal` | Smaller backup (no transcripts) |
| `orc import` | Restore from `.orc/exports/` (auto-detect format) |
| `orc import --dry-run` | Preview import without changes |

**Import behavior:** Newer `updated_at` wins (local preserved on tie). Running tasks become "interrupted" for safe resume. Use `--force` to always overwrite.

### Jira Import

| Command | Purpose |
|---------|---------|
| `orc import jira` | Import Jira Cloud issues as orc tasks |
| `orc import jira --dry-run` | Preview import without saving |
| `orc import jira --project PROJ` | Import from specific project(s) |
| `orc import jira --jql "..."` | Filter issues with JQL query |

Auth: `--url`/`--email`/`--token` flags, `ORC_JIRA_*` env vars, or `jira:` config section. Run `orc import jira --help` for full setup guide.

### Key Insight: Help Text = Documentation

Each command's `--help` contains detailed guidance on:
- What makes the command succeed
- Common mistakes to avoid
- How data flows through the system
- Quality tips for best results

**When in doubt, run `--help` first.**

## Key Patterns

**Error handling:** Always wrap with context
```go
return fmt.Errorf("load task %s: %w", id, err)
```

**Git commits:** After every phase: `[orc] TASK-001: implement - completed`

## Dependencies

Go modules: `llmkit` (Claude wrapper), `flowgraph` (execution), `devflow` (git ops). For local dev: `make setup` creates `go.work`.

### ⚠️ llmkit Sync Requirement

When adding llmkit features that orc depends on:

1. **Tag and push llmkit first** - `git tag vX.Y.Z && git push origin vX.Y.Z`
2. **Update orc's go.mod** - `GOWORK=off go get github.com/randalmurphal/llmkit@vX.Y.Z`
3. **Run GOWORK=off tests** - `make test-short` (uses published deps, not local)

**Why:** `go.work` masks version drift. Code works locally but fails in CI/production. The `GOWORK=off` test catches this.

## Web UI

Start: `make build && orc serve` (production) or `make dev-full` (hot reload).

Features: Project picker, live task board, WebSocket updates, initiative filtering, keyboard shortcuts (`Shift+Alt` modifier), settings editor, visual workflow editor (React Flow). All API calls include `projectId`.

See `web/CLAUDE.md` for component library and architecture.

## Testing

```bash
make test       # Backend (Go)
make web-test   # Frontend (Vitest)
make e2e        # E2E (Playwright)
```

**E2E tests run in isolated sandbox** (`/tmp`), not production. Import from `./fixtures` for automatic sandbox selection.

## Documentation Reference

| Topic | Location |
|-------|----------|
| API Endpoints | `docs/API_REFERENCE.md` |
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phase Model | `docs/architecture/PHASE_MODEL.md` |
| Gates & Approvals | `docs/architecture/GATES.md` |
| Config | `docs/specs/CONFIG_HIERARCHY.md` |
| File Formats | `docs/specs/FILE_FORMATS.md` |
| Constitution | `.orc/CONSTITUTION.md` |
| Web Components | `web/CLAUDE.md` |

<!-- orc:begin -->
## Orc Orchestration

This project uses [orc](https://github.com/randalmurphal/orc) for task orchestration.

### When to Use Orc

Use orc when:
- **Multi-step work**: Features, refactors, or fixes requiring multiple phases
- **Parallel tasks**: Running multiple independent tasks simultaneously
- **Complex changes**: Work that benefits from spec → implement → test → review flow
- **Tracked progress**: When you need visibility into what's done/remaining

**Key principle**: Delegate implementation to `orc run`. Don't implement tasks directly - create them and let orc execute them.

### Workflow

1. `orc new "task title"` - Create a task
2. `orc run TASK-XXX` - Execute it (runs in background)
3. Validate results when complete
4. `orc status` - Check what's next

### Slash Commands

| Command | Purpose |
|---------|---------|
| `/orc:continue` | Tech Lead session - run tasks, validate, keep moving |
| `/orc:status` | Show progress and next steps |
| `/orc:init` | Initialize project or create spec |
| `/orc:review` | Multi-round code review |
| `/orc:qa` | E2E tests and documentation |

### CLI Commands

```bash
orc status           # View active tasks
orc new "title"      # Create task
orc run TASK-001     # Execute task
orc show TASK-001    # Task details
orc diff TASK-001    # What changed
```

See `.orc/` for configuration and task details.

<!-- orc:end -->
