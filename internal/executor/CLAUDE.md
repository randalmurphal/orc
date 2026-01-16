# Executor Package

Phase execution engine with Ralph-style iteration loops and weight-based executor strategies.

## File Structure

| File | Purpose |
|------|---------|
| `executor.go` | Main orchestrator, task lifecycle, `getPhaseExecutor()` |
| `task_execution.go` | `ExecuteTask()`, `ResumeFromPhase()`, gate evaluation |
| `phase.go` | `ExecutePhase()`, session/flowgraph dispatch |
| `phase_executor.go` | `PhaseExecutor` interface, `ExecutorConfig`, `ResolveModelSetting()` |

### Executor Types

| File | Strategy | Weight |
|------|----------|--------|
| `trivial.go` | Fire-and-forget, no session | trivial |
| `standard.go` | Session per phase, iteration loop | small/medium |
| `full.go` | Persistent session, per-iteration checkpointing | large/greenfield |
| `finalize.go` | Branch sync, conflict resolution, risk assessment | large/greenfield |

### Support Modules

| File | Responsibility |
|------|----------------|
| `config.go` | `ExecutorConfig` defaults per weight |
| `template.go` | Prompt variable substitution, `BuildTemplateVars()` |
| `session_adapter.go` | LLM session wrapper, `StreamTurnWithProgress()` |
| `completion.go` | `CheckPhaseCompletion()` - detects `<phase_complete>` |
| `retry.go` | Cross-phase retry context |
| `worktree.go` | Git worktree setup/cleanup |
| `publish.go` | Nil-safe `EventPublisher` |
| `ci_merge.go` | CI polling and auto-merge |
| `resource_tracker.go` | Process/memory tracking for orphan detection |

## Architecture

```
Executor.ExecuteTask()
├── setupWorktree()           # Isolate in git worktree
├── loadPlan()                # Get phase sequence for weight
└── for each phase:
    ├── evaluateGate()        # Check gate conditions
    ├── getPhaseExecutor()    # Select by weight → executor.go:351
    │   └── ResolveModelSetting()  # Get model + thinking for phase
    ├── ExecutePhase()        # Run with selected executor
    └── checkpoint()          # Git commit on completion
```

## Executor Strategies

| Executor | Session | Checkpoints | Max Iters | Best For |
|----------|---------|-------------|-----------|----------|
| Trivial | None | None | 5 | Single-prompt fixes |
| Standard | Per-phase | On complete | 20 | Small/medium tasks |
| Full | Persistent | Per-iteration | 30-50 | Large/greenfield |
| Finalize | Per-phase | On complete | 10 | Branch sync, merge prep |

## Model Configuration

**Location:** `config.go:70-85`, `phase_executor.go:70-85`

Per-phase, per-weight model selection with thinking mode support.

### Resolution Hierarchy

```
ExecutorConfig.ResolveModelSetting(weight, phase)
├── Check config.OrcConfig.Models[weight][phase]  # Phase-specific
├── Fallback to config.OrcConfig.Models.Default   # Global default
└── Fallback to config.Model                      # Legacy field
```

### Default Model Matrix

| Weight | Decision Phases | Execution Phases |
|--------|-----------------|------------------|
| trivial | opus | sonnet |
| small | opus | sonnet |
| medium | opus + thinking | sonnet |
| large | opus + thinking | sonnet |
| greenfield | opus + thinking | sonnet |

**Decision phases** (thinking enabled): spec, design, review, validate, research
**Execution phases** (no thinking): implement, test, docs, finalize

### Extended Thinking (Ultrathink)

**Location:** `standard.go:189-194`, `full.go:224-229`, `trivial.go:125-129`, `finalize.go:577-581`

When `modelSetting.Thinking == true`, inject trigger at prompt start:
```go
if modelSetting.Thinking {
    promptText = "ultrathink\n\n" + promptText
}
```

**Why user message?** Claude Code thinking triggers only work in user messages, not system prompts. See `session_adapter.go:76-78` for explanation.

### Configuration

```yaml
# .orc/config.yaml
models:
  default:
    model: opus
    thinking: false
  medium:
    spec:
      model: opus
      thinking: true
    implement:
      model: sonnet
      thinking: false
```

## Key Components

### Template Variables (`template.go:40-120`)

| Variable | Description |
|----------|-------------|
| `{{TASK_ID}}`, `{{TASK_TITLE}}` | Task identifiers |
| `{{SPEC_CONTENT}}` | Specification from spec phase |
| `{{DESIGN_CONTENT}}` | Design artifact (large/greenfield) |
| `{{RETRY_CONTEXT}}` | Failure info on retry |
| `{{WORKTREE_PATH}}`, `{{TASK_BRANCH}}` | Git worktree context |
| `{{INITIATIVE_CONTEXT}}` | Initiative details if linked |

### Token Usage (`session_adapter.go:118-135`)

```go
// Raw InputTokens is misleadingly low when cached
// Always use EffectiveInputTokens() for actual context size
effective := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
```

### Completion Detection (`completion.go`)

```xml
<phase_complete>true</phase_complete>   <!-- Success -->
<phase_blocked>reason: ...</phase_blocked>  <!-- Needs help -->
```

### Retry Context (`retry.go`)

Cross-phase retry when tests fail:
```go
ctx := buildRetryContext(failedPhase, output, attempt)
// Phase receives {{RETRY_CONTEXT}} with failure details
```

## FinalizeExecutor

**Location:** `finalize.go`

Dedicated executor for large/greenfield finalize phase.

| Step | Method | Purpose |
|------|--------|---------|
| 1 | `fetchTarget()` | Fetch origin/main |
| 2 | `checkDivergence()` | Count commits ahead/behind |
| 3 | `syncWithTarget()` | Merge or rebase per config |
| 4 | `resolveConflicts()` | AI-assisted conflict resolution |
| 5 | `runTests()` | Verify tests pass |
| 6 | `assessRisk()` | Classify: low/medium/high/critical |

**Risk Thresholds:**

| Metric | Low | Medium | High | Critical |
|--------|-----|--------|------|----------|
| Files | 1-5 | 6-15 | 16-30 | >30 |
| Lines | <100 | 100-500 | 500-1000 | >1000 |
| Conflicts | 0 | 1-3 | 4-10 | >10 |

**Escalation:** >10 unresolved conflicts or >5 test failures → retry from implement phase

See `docs/FINALIZE.md` for detailed flow.

## CI Merger

**Location:** `ci_merge.go`

Handles CI polling and auto-merge after finalize.

```go
merger.WaitForCIAndMerge(ctx, task)
├── WaitForCI()      // Poll gh pr checks (30s interval, 10m timeout)
└── MergePR()        // gh pr merge --squash
```

**Profile Behavior:**
- `auto`/`fast`: Wait for CI, auto-merge on pass
- `safe`/`strict`: No auto-merge (human approval required)

See `docs/CI_MERGE.md` for configuration options.

## Resource Tracker

**Location:** `resource_tracker.go`

Detects orphaned processes (MCP servers, browsers) after task execution.

```go
tracker.SnapshotBefore()   // Before task
// ... task runs ...
tracker.SnapshotAfter()    // After task
tracker.DetectOrphans()    // Find orphaned processes
tracker.CheckMemoryGrowth() // Check memory delta
```

**Orphan criteria:** New PID + (parent is PID 1 OR parent doesn't exist)

See `docs/RESOURCE_TRACKER.md` for platform-specific details.

## Testing

```bash
go test ./internal/executor/... -v                    # All tests
go test ./internal/executor/... -run TestResolve -v   # Model resolution
go test ./internal/executor/... -run TestTemplate -v  # Template vars
```

| Test File | Coverage |
|-----------|----------|
| `executor_test.go` | Integration tests |
| `template_test.go` | Variable substitution, initiative context |
| `session_adapter_test.go` | Token usage, effective tokens |
| `finalize_test.go` | Sync strategies, risk assessment |
| `ci_merge_test.go` | CI status, polling, merge methods |
| `resource_tracker_test.go` | Orphan detection, memory growth |

## Common Gotchas

1. **Raw InputTokens misleading** - Use `EffectiveInputTokens()` for actual context size
2. **Ultrathink in system prompt** - Doesn't work; must be in user message
3. **FinalizeExecutor OrcConfig** - `WithFinalizeOrcConfig()` must set both `e.orcConfig` and `e.config.OrcConfig`
4. **Model not resolved in trivial** - Fixed: now uses `ResolveModelSetting()` like other executors
