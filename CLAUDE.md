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

**Key packages:** `api/` (REST + WebSocket), `cli/` (Cobra), `executor/` (phase engine), `workflow/` (workflow definitions), `task/` (task model), `storage/` (database backend), `git/` (worktrees), `db/` (SQLite)

## Task Model

Tasks have weight (trivial/small/medium/large) determining phase workflow. All tasks require a spec (tiny_spec or spec phase). See `orc new --help` for guidance.

**Weight determines phases:**
- **trivial**: tiny_spec → implement
- **small**: tiny_spec → implement → review
- **medium**: spec → tdd_write → implement → review → docs
- **large**: spec → tdd_write → breakdown → implement → review → docs → validate

**Key insight**: Vague task description → vague spec → poor results. Be specific in task descriptions.

**Task properties**: Queue (active/backlog), Priority, Category (feature/bug/refactor/chore/docs/test), Initiative (group related tasks), Dependencies (blocked_by, related_to).

**Completion flow**: Task completes → PR created → Review → `orc finalize TASK-XXX` → Auto-merge (if configured).

See `docs/architecture/PHASE_MODEL.md` for phase details.

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

### Constitution

Project-level principles injected into all phase prompts via `{{CONSTITUTION_CONTENT}}`:

```bash
orc constitution set --file INVARIANTS.md   # Set from file
orc constitution show                        # View current
orc constitution delete                      # Remove
```

## File Layout

```
~/.orc/                          # Global config, database, token pool
.orc/                            # Project database, config, prompts, worktrees
.claude/                         # Claude Code settings, hooks, skills
```

Task data stored in SQLite (`orc.db`). Use `orc export --all-tasks --all` for full backup to `.orc/exports/`.

## Commands

**Run `orc --help` or `orc <command> --help` for detailed usage.** Help text includes quality guidance and common mistakes.

**Core**: `new` (create), `run` (execute), `go` (create+run), `status` (dashboard), `resume` (continue paused/failed)

**Management**: `show`, `deps`, `log`, `approve`, `resolve`, `finalize`

**Initiatives**: `initiative new/decide/link/run` - group related tasks with shared vision

**Data**: `export/import` - portable tar.gz backups with auto-migration

See `internal/cli/COMMANDS.md` for complete reference.

## Key Patterns

**Error handling**: `fmt.Errorf("load task %s: %w", id, err)`
**Git commits**: `[orc] TASK-001: implement - completed`
**Dependencies**: `llmkit`, `flowgraph`, `devflow`. Local dev: `make setup`

## Web UI

`make dev-full` (dev) or `orc serve` (prod). React 19, WebSocket live updates. See `web/CLAUDE.md`.

## Testing

`make test` (Go), `make web-test` (Vitest), `make e2e` (Playwright). E2E uses isolated `/tmp` sandbox.

## Documentation Reference

| Topic | Location |
|-------|----------|
| API Endpoints | `docs/API_REFERENCE.md` |
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phase Model | `docs/architecture/PHASE_MODEL.md` |
| Config | `docs/specs/CONFIG_HIERARCHY.md` |
| File Formats | `docs/specs/FILE_FORMATS.md` |
| Invariants | `docs/INVARIANTS.md` |
| Web Components | `web/CLAUDE.md` |

<!-- orc:begin -->
## Orc Orchestration

This project uses orc for task orchestration. Use for multi-step work, parallel tasks, or when you need spec → implement → test → review flow.

**Workflow**: `orc new "title"` → `orc run TASK-XXX` → validate → `orc status`

**Slash commands**: `/orc:continue` (Tech Lead session), `/orc:status`, `/orc:init`, `/orc:review`, `/orc:qa`

See `.orc/` for configuration.
<!-- orc:end -->

## Project Knowledge

See [docs/knowledge/PROJECT_KNOWLEDGE.md](docs/knowledge/PROJECT_KNOWLEDGE.md) for patterns, gotchas, and decisions learned during development.

<!-- orc:knowledge:target:docs/knowledge/PROJECT_KNOWLEDGE.md -->
