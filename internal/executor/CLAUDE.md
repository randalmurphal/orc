# Executor Package

Phase execution engine with Ralph-style iteration loops and weight-based executor strategies.

## File Structure

| File | Purpose |
|------|---------|
| `executor.go` | Main orchestrator, `getPhaseExecutor()` |
| `task_execution.go` | `ExecuteTask()`, `ResumeFromPhase()` |
| `phase.go` | `ExecutePhase()`, executor dispatch |
| `phase_executor.go` | `PhaseExecutor` interface, `ResolveModelSetting()` |

### Executor Types

| File | Strategy | Weight |
|------|----------|--------|
| `trivial.go` | ClaudeExecutor, no session persistence | trivial |
| `standard.go` | ClaudeExecutor per phase | small/medium |
| `full.go` | ClaudeExecutor with checkpointing | large/greenfield |
| `finalize.go` | Branch sync, conflict resolution | large/greenfield |

### Support Modules

| File | Purpose |
|------|---------|
| `claude_executor.go` | `TurnExecutor` interface, ClaudeCLI wrapper with `--json-schema` |
| `execution_context.go` | `BuildExecutionContext()` - centralized context building |
| `template.go` | `BuildTemplateVars()`, `RenderTemplate()` |
| `phase_response.go` | JSON schema for phase completion |
| `ci_merge.go` | CI polling and auto-merge |
| `resource_tracker.go` | Orphan process detection |
| `heartbeat.go` | Periodic heartbeat updates during execution |
| `backpressure.go` | Deterministic quality checks (tests, lint, build) |
| `haiku_validation.go` | Haiku-based spec and criteria validation |
| `jsonl_sync.go` | `JSONLSyncer` for Claude JSONL → DB sync |
| `publish.go` | `EventPublisher` for real-time events |
| `cost_tracking.go` | Global cost recording to `~/.orc/orc.db` |

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

## Unified Execution Context

All executors (trivial, standard, full) use the same context building via `BuildExecutionContext()`:

```go
execCtx, err := BuildExecutionContext(ExecutionContextConfig{
    Task:            t,
    Phase:           p,
    State:           s,
    Backend:         backend,
    WorkingDir:      workingDir,
    MCPConfigPath:   mcpConfigPath,
    ExecutorConfig:  config,
    OrcConfig:       orcConfig,
    ResumeSessionID: resumeSessionID,
    Logger:          logger,
})
```

**What it handles:**
- Template loading and rendering
- Spec content from database
- Review context (round, findings)
- Initiative context
- Automation context
- UI testing context
- Ultrathink injection
- Model resolution
- Session ID generation

**ClaudeExecutor creation:**
```go
turnExec := NewClaudeExecutorFromContext(execCtx, claudePath, maxIterations, logger)
```

## Template Variables

Key variables: `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}`, `{{TASK_CATEGORY}}`, `{{SPEC_CONTENT}}`, `{{DESIGN_CONTENT}}`, `{{RETRY_CONTEXT}}`, `{{WORKTREE_PATH}}`, `{{TASK_BRANCH}}`, `{{TARGET_BRANCH}}`, `{{INITIATIVE_CONTEXT}}`, `{{REQUIRES_UI_TESTING}}`, `{{SCREENSHOT_DIR}}`, `{{REVIEW_ROUND}}`, `{{REVIEW_FINDINGS}}`, `{{VERIFICATION_RESULTS}}`

**Spec content loading:** `{{SPEC_CONTENT}}` is populated via `WithSpecFromDatabase()` from the storage backend. Specs are stored exclusively in the database (not as file artifacts) to avoid merge conflicts in worktrees.

## Session Configuration

Sessions need user source for agents in headless mode:

```go
session.WithSettingSources([]string{"project", "local", "user"})
```

Sources: `project` (.claude/), `local` (worktree .claude/), `user` (~/.claude/)

## Completion Detection

```json
{"status": "complete", "summary": "Work done"}    // Success
{"status": "blocked", "reason": "Need X"}         // Needs help
{"status": "continue", "reason": "In progress"}   // More work needed
```

### Extraction Function

| Function | Use Case |
|----------|----------|
| `CheckPhaseCompletionJSON()` | Parse pure JSON from `--json-schema` output |

The executors use `ClaudeExecutor` with `--json-schema` in headless mode, which guarantees pure JSON output.

## Phase Retry Map

When phases fail or output `{"status": "blocked"}`, they retry from an earlier phase:

| Failed Phase | Retries From | Reason |
|--------------|--------------|--------|
| design | spec | Design issues often stem from incomplete spec |
| review | implement | Review findings need code changes |
| test, test_unit, test_e2e | implement | Test failures need code fixes |
| validate | implement | Validation issues need code changes |

**Not in map:** `spec`, `implement`, `docs`, `research` - these either have no upstream phase or retry wouldn't help.

**Blocked output preservation:** When `PhaseStatusBlocked` is detected, executors preserve `result.Output` with the response content so retry context includes what the agent reported as blocking.

## FinalizeExecutor

Steps: fetchTarget → checkDivergence → syncWithTarget → resolveConflicts → runTests → assessRisk

**Escalation:** >10 conflicts or >5 test failures → retry from implement phase

See `docs/architecture/FINALIZE.md` for detailed flow.

## CI Merger

`ci_merge.go` handles CI polling and auto-merge after finalize.

**Profiles:** `auto`/`fast` auto-merge on CI pass; `safe`/`strict` require human approval.

**Merge retry logic:** `MergePR()` handles HTTP 405 "Base branch was modified" errors from GitHub (race condition when parallel tasks merge):
- Detects retryable error via `isRetryableMergeError()`
- Retries up to 3 times with exponential backoff (2s, 4s, 8s)
- Rebases branch onto target via `rebaseOnTarget()` before each retry
- Returns `ErrMergeFailed` if retries exhausted or rebase conflicts

**Error handling in completeTask():**
- `ErrMergeFailed` blocks task with `blocked_reason=merge_failed`
- Returns `ErrTaskBlocked` so CLI shows blocked message instead of celebration
- User runs `orc resume TASK-XXX` after resolving

## Resource Tracker

`resource_tracker.go` detects orphaned MCP processes after task execution.

## Heartbeat Runner

`heartbeat.go` provides periodic heartbeat updates during phase execution to support orphan detection.

**Purpose:** Long-running phases (especially `implement`) can take hours. Without periodic heartbeats, the heartbeat would become stale even though the task is healthy.

**Key constants:**
- `DefaultHeartbeatInterval`: 2 minutes between heartbeat updates

**Integration:**
```go
// In ExecuteTask, heartbeat runner starts before phases
heartbeatRunner := NewHeartbeatRunner(e.Backend, state, e.Logger)
heartbeatRunner.Start(ctx)
defer heartbeatRunner.Stop()
```

**Orphan detection priority:** The state package's `CheckOrphaned()` prioritizes PID check over heartbeat staleness. A live PID always indicates a healthy task - heartbeat staleness is only used as additional context when PID is dead. This prevents false positives during long-running phases.

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
| `heartbeat_test.go` | Heartbeat updates, stop/cancel |

## Artifact Storage

| Phase | Storage | Extraction |
|-------|---------|------------|
| spec, design, research, docs | Database | From JSON `artifact` field via `--json-schema` |
| implement, test, validate | Code changes only | No artifact extraction |

**JSON-based artifact extraction:**
- Phase prompts use `--json-schema` for constrained JSON output
- `GetSchemaForPhase()` returns appropriate schema (with or without `artifact` field)
- `ExtractArtifactFromOutput()` parses JSON and extracts `artifact` field
- `SaveSpecToDatabase()` extracts spec from JSON and saves to database
- **Failure handling:** Extraction failures call `failTask()` to ensure task status becomes `StatusFailed`

## Backpressure & Haiku Validation

Objective quality checks run after agent claims completion. See `docs/research/EXECUTION_PHILOSOPHY.md` for design rationale.

| Component | File | Purpose |
|-----------|------|---------|
| Backpressure | `backpressure.go:146` | Runs tests/lint/build after `{"status": "complete"}` |
| Haiku Spec Validation | `haiku_validation.go` | Validates spec quality before execution (pre-gate) |
| Haiku Criteria Validation | `haiku_validation.go` | Validates success criteria on completion claim |
| Config Helpers | `config.go:2138` | `ShouldRunBackpressure()`, `ShouldValidateSpec()`, `ShouldValidateCriteria()` |

**Flow:** Agent outputs `{"status": "complete"}` → Backpressure runs → Criteria validation runs → If any fail, inject context and continue iteration.

**API Error Handling:** Controlled by `config.Validation.FailOnAPIError`:
- `true` (default for auto/safe/strict): Fail task properly (resumable via `orc resume`)
- `false` (fast profile): Fail open, continue execution without validation

**Validation Client Creation:** Validation clients are created **dynamically per-call** with the correct workdir:
- `Executor.CreateValidationClient(workdir)` creates a client for a given directory
- Sub-executors use their `workingDir` field (the worktree path) for client creation
- This ensures validation runs in the worktree context where task files exist
- **CRITICAL:** Never store a pre-created validation client - always create dynamically with correct workdir

## Claude Call Patterns (CRITICAL)

**All Claude calls MUST follow these consolidated patterns. Deviating causes bugs.**

### Pattern 1: TurnExecutor for Phase Execution

```go
// Phase execution with sessions - model from ResolveModelSetting()
turnExec := NewClaudeExecutorFromContext(execCtx, claudePath, maxIterations, logger)
result, err := turnExec.ExecuteTurn(ctx, prompt)              // With JSON schema
result, err := turnExec.ExecuteTurnWithoutSchema(ctx, prompt) // Freeform output
```

### Pattern 2: Schema-Constrained Validation (ONE WAY)

**Use `llmutil.ExecuteWithSchema[T]()` for ALL schema-constrained LLM calls.**

```go
// The ONLY way to do schema-constrained calls - no exceptions
schemaResult, err := llmutil.ExecuteWithSchema[responseType](ctx, client, prompt, schema)
if err != nil {
    return nil, fmt.Errorf("validation failed: %w", err)  // ALWAYS propagate error
}
// Use schemaResult.Data (typed) - no manual json.Unmarshal needed
```

**Why this is the only pattern:**
- Enforces `--output-format json` when schema specified
- Errors if `structured_output` is empty (no silent fallback to `result`)
- Handles JSON parsing with proper error propagation
- Generic type `[T]` provides compile-time type safety

### Model Configuration

| Call Type | Model Source | Config Key |
|-----------|--------------|------------|
| Phase execution | `ResolveModelSetting(weight, phase)` | `config.Models[weight][phase]` |
| Haiku validation | Client configured at creation | `config.Validation.Model` |
| Gate evaluation | Main executor client | `executor.Config.Model` |

**NEVER hardcode model in CompletionRequest** - model is set when creating the client.

### Schema Routing (`phase_response.go`)

`GetSchemaForPhaseWithRound(phaseID, round)` returns the correct schema:

| Phase | Round | Schema |
|-------|-------|--------|
| spec, design, research, docs | - | `PhaseCompletionWithArtifactSchema` |
| review | 1 | `ReviewFindingsSchema` |
| review | 2 | `ReviewDecisionSchema` |
| qa | - | `QAResultSchema` |
| other | - | `PhaseCompletionSchema` |

### ExecuteTurn vs ExecuteTurnWithoutSchema

| Method | When to Use | Example |
|--------|-------------|---------|
| `ExecuteTurn()` | Need completion detection from JSON | Phase execution |
| `ExecuteTurnWithoutSchema()` | Verify success externally | `conflict_resolver.go` (checks git status) |

## Structured Output (JSON Schema)

**ALL LLM output is pure JSON via `--json-schema`.** No mixed text/JSON. No extraction needed.

### Schema Definitions

| File | Schema Constant | Purpose |
|------|-----------------|---------|
| `phase_response.go` | `PhaseCompletionSchema`, `PhaseCompletionWithArtifactSchema` | Phase completion |
| `haiku_validation.go` | `taskReadinessSchema`, `criteriaCompletionSchema` | Validation |
| `review.go` | `ReviewFindingsSchema`, `ReviewDecisionSchema` | Code review |
| `qa.go` | `QAResultSchema` | QA session |
| `../gate/gate.go` | `gateDecisionSchema` | Gate decisions |

### Parsing (via ExecuteWithSchema)

**DO NOT manually parse JSON.** Use `llmutil.ExecuteWithSchema[T]()` which handles parsing internally.

```go
// ❌ WRONG - manual parsing with silent failure risk
var result readinessResponse
if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
    return true, nil, nil  // BUG: silent success on parse error!
}

// ✅ CORRECT - use ExecuteWithSchema which returns error
schemaResult, err := llmutil.ExecuteWithSchema[readinessResponse](ctx, client, prompt, schema)
if err != nil {
    return true, nil, err  // Error propagated to caller
}
// schemaResult.Data is already typed and parsed
```

**Phase completion parsing** uses `CheckPhaseCompletionJSON()` which returns `(status, reason, error)` - the error MUST be handled.

## Transcript Persistence (JSONL Sync)

`jsonl_sync.go` syncs Claude Code JSONL session files to the database in real-time and as a final catchup.

**Key features:**
- Reads JSONL files written by Claude Code (`~/.claude/projects/`)
- Extracts: messages, tool calls, token usage, todos
- Deduplicates via `MessageUUID` (append mode)
- Filters out `queue-operation` messages (internal bookkeeping)
- Real-time streaming via `TranscriptStreamer` (fsnotify-based file watcher)

**Two-phase sync strategy:**

1. **Real-time streaming** (during execution): `TranscriptStreamer` watches the JSONL file using fsnotify and syncs new messages to DB in batches (every 100ms or 10 messages)
2. **Post-phase catchup** (after completion): `SyncFromFile()` runs as a final sweep to ensure no messages were missed

**Integration:**
```go
// Real-time streaming (in standard.go/full.go)
syncer := NewJSONLSyncer(backend, logger)
streamer, _ := syncer.StartStreaming(jsonlPath, SyncOptions{
    TaskID: task.ID,
    Phase:  phase.ID,
    Append: true,
})
// ... phase executes ...
streamer.Stop()  // Flushes remaining messages

// Post-phase catchup (in task_execution.go)
syncer.SyncFromFile(ctx, jsonlPath, SyncOptions{
    TaskID: task.ID,
    Phase:  phase.ID,
    Append: true,  // Deduplicates by UUID
})
```

**What gets synced:**

| Data | Source | DB Table |
|------|--------|----------|
| Messages | `message.content` | `transcripts` |
| Tokens | `message.usage` | `transcripts` (per-message) |
| Tool calls | `content[type=tool_use]` | `transcripts.tool_calls` |
| Todos | `TodoWrite` tool results | `todo_snapshots` |

**Token aggregation:** DB views compute per-task/phase totals from per-message tokens. See `db/CLAUDE.md`.

## Cost Tracking

`cost_tracking.go` records phase costs to the global database (`~/.orc/orc.db`) after each phase completion.

**Data flow:**
```
TurnResult.Usage (llmkit/claude, via claude_executor.go)
    ↓ accumulated per iteration
Result{InputTokens, OutputTokens, CacheCreation/ReadTokens, CostUSD} (executor.go:180-187)
    ↓ after phase completion
recordCostToGlobal() (cost_tracking.go:21)
    ↓
GlobalDB.RecordCostExtended() (db/global.go:340)
```

**What gets recorded:**

| Field | Source | Purpose |
|-------|--------|---------|
| `CostUSD` | `TurnResult.CostUSD` from Claude CLI | Actual cost from API |
| `InputTokens` | `TurnResult.Usage.EffectiveInputTokens()` | Includes cache tokens |
| `OutputTokens` | `TurnResult.Usage.OutputTokens` | Response tokens |
| `CacheCreationTokens` | `TurnResult.Usage.CacheCreationInputTokens` | New cache entries |
| `CacheReadTokens` | `TurnResult.Usage.CacheReadInputTokens` | Cache hits |
| `DurationMs` | `Result.Duration.Milliseconds()` | Phase execution time |
| `Model` | `DetectModel(modelSetting.Model)` | opus/sonnet/haiku/unknown |

**Integration point:** `task_execution.go:~280` calls `recordCostToGlobal()` after phase completion.

## Testing with TurnExecutor

All executors accept a `TurnExecutor` interface for testing without spawning real Claude CLI:

```go
// Create mock executor
mock := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

// Inject via option
executor := NewStandardExecutor(
    WithStandardTurnExecutor(mock),
    // ... other options
)
```

**TurnExecutor interface:**
```go
type TurnExecutor interface {
    ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error)
    ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error)
    UpdateSessionID(id string)
    SessionID() string
}
```

**Available mocks:** `MockTurnExecutor` with configurable responses, delays, and errors.

**Executor-specific injection:**

| Executor | Option Function |
|----------|-----------------|
| `StandardExecutor` | `WithStandardTurnExecutor(mock)` |
| `FullExecutor` | `WithFullTurnExecutor(mock)` |
| `TrivialExecutor` | `WithTrivialTurnExecutor(mock)` |
| `FinalizeExecutor` | `WithFinalizeTurnExecutor(mock)` |
| `ConflictResolver` | `WithResolverTurnExecutor(mock)` |

**In-memory backends for fast tests:**

```go
// Use in-memory backend (no disk I/O)
backend := storage.NewTestBackend(t)  // Auto-cleanup via t.Cleanup()

// Or manually for more control
backend, _ := storage.NewInMemoryBackend()
defer backend.Close()
```

**Test parallelization:**
- Most tests use `t.Parallel()` for concurrent execution
- Tests using `t.Setenv()` or `os.Chdir()` CANNOT use `t.Parallel()` (process-wide state)
- CLI tests are mostly sequential due to chdir usage

## Common Gotchas

1. **Raw InputTokens misleading** - Use `EffectiveInputTokens()`
2. **Ultrathink in system prompt** - Doesn't work; must be user message
3. **User agents unavailable** - Need `WithSettingSources` with "user"
4. **Worktree cleanup by path** - Use `CleanupWorktreeAtPath(e.worktreePath)` not `CleanupWorktree(taskID)` to handle initiative-prefixed worktrees correctly
5. **Spec not found in templates** - Use `WithSpecFromDatabase()` to load spec content; file-based specs are legacy
6. **Invalid session ID errors** - Only pass custom session IDs when `Persistence: true`; Claude CLI expects UUIDs it generates for ephemeral sessions
7. **Transcripts not persisting** - Ensure `JSONLSyncer.SyncFromFile()` called after phase completion with correct JSONL path
8. **Testing real Claude CLI** - Use `WithStandardTurnExecutor(mock)` to inject test doubles; avoids real API calls
9. **Validation can't see worktree files** - Validation clients must be created dynamically with correct workdir; never store a pre-created client at executor startup
