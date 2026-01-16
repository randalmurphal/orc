# Executor Package

Phase execution engine with Ralph-style iteration loops and weight-based executor strategies.

## File Structure

| File | Purpose |
|------|---------|
| `executor.go` | Main orchestrator, `getPhaseExecutor()` |
| `task_execution.go` | `ExecuteTask()`, `ResumeFromPhase()` |
| `phase.go` | `ExecutePhase()`, session/flowgraph dispatch |
| `phase_executor.go` | `PhaseExecutor` interface, `ResolveModelSetting()` |

### Executor Types

| File | Strategy | Weight |
|------|----------|--------|
| `trivial.go` | Fire-and-forget | trivial |
| `standard.go` | Session per phase | small/medium |
| `full.go` | Persistent session | large/greenfield |
| `finalize.go` | Branch sync, conflict resolution | large/greenfield |

### Support Modules

| File | Purpose |
|------|---------|
| `template.go` | `BuildTemplateVars()`, `RenderTemplate()` |
| `flowgraph_nodes.go` | Flowgraph nodes, `renderTemplate()` |
| `session_adapter.go` | LLM session wrapper |
| `completion.go` | `<phase_complete>` detection |
| `ci_merge.go` | CI polling and auto-merge |
| `resource_tracker.go` | Orphan process detection |

## Architecture

```
Executor.ExecuteTask()
├── setupWorktree()           # Isolate in git worktree
├── loadPlan()                # Get phases for weight
├── for each phase:
│   ├── evaluateGate()        # Check conditions
│   ├── getPhaseExecutor()    # Select by weight
│   │   └── ResolveModelSetting()  # Get model + thinking
│   ├── ExecutePhase()        # Run
│   └── checkpoint()          # Git commit
└── cleanupWorktreeForTask()  # Remove worktree (if configured)
```

## Executor Strategies

| Executor | Session | Checkpoints | Max Iters |
|----------|---------|-------------|-----------|
| Trivial | None | None | 5 |
| Standard | Per-phase | On complete | 20 |
| Full | Persistent | Per-iteration | 30-50 |
| Finalize | Per-phase | On complete | 10 |

## Model Configuration

Per-phase, per-weight model selection (`config.go`, `phase_executor.go`):

```
ResolveModelSetting(weight, phase)
├── config.OrcConfig.Models[weight][phase]  # Phase-specific
├── config.OrcConfig.Models.Default         # Global default
└── config.Model                            # Legacy fallback
```

**Default matrix:**
- Decision phases (spec, review, validate): opus + thinking
- Execution phases (implement, test, docs): sonnet

**Extended thinking:** When `modelSetting.Thinking == true`, prepend `ultrathink\n\n` to prompt text.

## Template Variables

⚠️ **CRITICAL**: Two rendering paths MUST stay in sync:
- `template.go:RenderTemplate()` - Session-based executors
- `flowgraph_nodes.go:renderTemplate()` - Flowgraph execution

Both call `processReviewConditionals()` for `{{#if REVIEW_ROUND_N}}` blocks.

Key variables: `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}`, `{{TASK_CATEGORY}}`, `{{SPEC_CONTENT}}`, `{{DESIGN_CONTENT}}`, `{{RETRY_CONTEXT}}`, `{{WORKTREE_PATH}}`, `{{TASK_BRANCH}}`, `{{TARGET_BRANCH}}`, `{{INITIATIVE_CONTEXT}}`, `{{REQUIRES_UI_TESTING}}`, `{{SCREENSHOT_DIR}}`, `{{REVIEW_ROUND}}`, `{{REVIEW_FINDINGS}}`, `{{VERIFICATION_RESULTS}}`

## Session Configuration

Sessions need user source for agents in headless mode:

```go
session.WithSettingSources([]string{"project", "local", "user"})
```

Sources: `project` (.claude/), `local` (worktree .claude/), `user` (~/.claude/)

## Completion Detection

```xml
<phase_complete>true</phase_complete>   <!-- Success -->
<phase_blocked>reason: ...</phase_blocked>  <!-- Needs help -->
```

## FinalizeExecutor

Steps: fetchTarget → checkDivergence → syncWithTarget → resolveConflicts → runTests → assessRisk

**Escalation:** >10 conflicts or >5 test failures → retry from implement phase

See `docs/architecture/FINALIZE.md` for detailed flow.

## CI Merger

`ci_merge.go` handles CI polling and auto-merge after finalize.

**Profiles:** `auto`/`fast` auto-merge on CI pass; `safe`/`strict` require human approval.

## Resource Tracker

`resource_tracker.go` detects orphaned MCP processes after task execution.

## Testing

```bash
go test ./internal/executor/... -v
```

| Test File | Coverage |
|-----------|----------|
| `executor_test.go` | Integration |
| `template_test.go` | Variable substitution |
| `finalize_test.go` | Sync, risk assessment |
| `ci_merge_test.go` | CI polling, merge |

## Common Gotchas

1. **Raw InputTokens misleading** - Use `EffectiveInputTokens()`
2. **Ultrathink in system prompt** - Doesn't work; must be user message
3. **Template not substituted** - Check BOTH `template.go` AND `flowgraph_nodes.go`
4. **User agents unavailable** - Need `WithSettingSources` with "user"
5. **Worktree cleanup by path** - Use `CleanupWorktreeAtPath(e.worktreePath)` not `CleanupWorktree(taskID)` to handle initiative-prefixed worktrees correctly
