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
| `phase_response.go` | JSON schema for phase completion |
| `ci_merge.go` | CI polling and auto-merge |
| `resource_tracker.go` | Orphan process detection |
| `heartbeat.go` | Periodic heartbeat updates during execution |
| `backpressure.go` | Deterministic quality checks (tests, lint, build) |
| `haiku_validation.go` | Haiku-based spec and progress validation |
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

## Template Variables

⚠️ **CRITICAL**: Two rendering paths MUST stay in sync:
- `template.go:RenderTemplate()` - Session-based executors
- `flowgraph_nodes.go:renderTemplate()` - Flowgraph execution

Both call `processReviewConditionals()` for `{{#if REVIEW_ROUND_N}}` blocks.

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

**Note:** `--json-schema` only works with `--print` mode, not `stream-json` sessions.

### Extraction Functions

| Function | Use Case |
|----------|----------|
| `CheckPhaseCompletionMixed()` | **Recommended for sessions** - handles mixed text+JSON |
| `ExtractPhaseResponseFromMixed()` | Returns `*PhaseResponse` from mixed content |
| `CheckPhaseCompletionJSON()` | Pure JSON only - use for structured output mode |
| `ExtractPhaseResponse()` | With LLM fallback (requires client) |

**Mixed content extraction order:**
1. Direct JSON parsing (pure JSON output)
2. JSON in markdown code blocks (```json ... ```)
3. JSON object by brace matching (finds `{"status": ...}` pattern)

Session-based executors use `CheckPhaseCompletionMixed()` to handle Claude's natural language + JSON output pattern.

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

| Phase | Storage | Reason |
|-------|---------|--------|
| spec | Database only | Avoids merge conflicts in worktrees |
| research, design, implement, test, docs, validate | File artifacts | Traditional file-based storage |

**Spec handling:**
- `SavePhaseArtifact()` skips file writes for spec phase
- `SaveSpecToDatabase()` extracts spec content and saves to database with source tag
  - Primary: Looks for `<artifact>` tags in agent output
  - Fallback: If no artifact tags, checks for `spec.md` file in task directory (agents sometimes write specs to files instead of using artifact tags)
- `ArtifactDetector` checks database first (via `NewArtifactDetectorWithBackend`), falls back to legacy `spec.md` file
- **Failure handling:** All three spec extraction failure paths (empty output, extraction error, database save error) call `failTask()` to ensure task status becomes `StatusFailed` rather than orphaned in `StatusRunning`

## Backpressure & Haiku Validation

Objective quality checks run after agent claims completion. See `docs/research/EXECUTION_PHILOSOPHY.md` for design rationale.

| Component | File | Purpose |
|-----------|------|---------|
| Backpressure | `backpressure.go:146` | Runs tests/lint/build after `{"status": "complete"}` |
| Haiku Validation | `haiku_validation.go:53` | External LLM validates progress against spec |
| Config Helpers | `config.go:2138` | `ShouldRunBackpressure()`, `ShouldValidateSpec()` |

**Flow:** Agent outputs `{"status": "complete"}` → Backpressure runs → If fail, inject context and continue iteration.

**API Error Handling:** Controlled by `config.Validation.FailOnAPIError`:
- `true` (default for auto/safe/strict): Fail task properly (resumable via `orc resume`)
- `false` (fast profile): Fail open, continue execution without validation

## Structured Output (JSON Schema)

LLM responses requiring structured data use JSON schemas via Claude's `--json-schema` flag. This replaces fragile XML regex parsing with reliable `json.Unmarshal`.

### Schema Definitions

| File | Schema Constant | Purpose |
|------|-----------------|---------|
| `haiku_validation.go` | `iterationProgressSchema`, `taskReadinessSchema` | Progress and spec validation |
| `review.go` | `ReviewFindingsSchema`, `ReviewDecisionSchema` | Code review structured output |
| `qa.go` | `QAResultSchema` | QA session results |
| `../gate/gate.go` | `gateDecisionSchema` | Gate approval decisions |

### Parsing Functions

| Function | Use Case |
|----------|----------|
| `ParseReviewFindings()`, `ParseReviewDecision()` | Direct JSON parsing (clean JSON input) |
| `ParseQAResult()` | Direct JSON parsing (clean JSON input) |
| `ExtractReviewFindings()`, `ExtractReviewDecision()` | Robust extraction from mixed text/JSON session output |
| `ExtractQAResult()` | Robust extraction from mixed text/JSON session output |

**Extract vs Parse:**
- `Parse*` functions expect clean JSON - use when LLM returns pure JSON via `--json-schema`
- `Extract*` functions handle mixed output - use when processing session output that may contain text around JSON

**Extraction Flow (llmkit/claude/extract.go):**
1. Try direct JSON parsing (fast path)
2. Look for JSON in code blocks (```json ... ```)
3. Find JSON object by brace matching
4. If no valid JSON found, fallback to LLM extraction with schema

**Pattern:** Define JSON schema constant → Pass to LLM call → Unmarshal response → Normalize fields (e.g., lowercase status enums).

## Transcript Persistence (JSONL Sync)

`jsonl_sync.go` syncs Claude Code JSONL session files to the database on phase completion.

**Key features:**
- Reads JSONL files written by Claude Code (`~/.claude/projects/`)
- Extracts: messages, tool calls, token usage, todos
- Deduplicates via `MessageUUID` (append mode)
- Filters out `queue-operation` messages (internal bookkeeping)

**Integration:**
```go
// In executor, after phase completion
syncer := NewJSONLSyncer(backend, logger)
err := syncer.SyncFromFile(ctx, jsonlPath, SyncOptions{
    TaskID: task.ID,
    Phase:  phase.ID,
    Append: true,  // Only sync new messages
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
TurnResult.Usage (session_adapter.go:228-234)
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

## Common Gotchas

1. **Raw InputTokens misleading** - Use `EffectiveInputTokens()`
2. **Ultrathink in system prompt** - Doesn't work; must be user message
3. **Template not substituted** - Check BOTH `template.go` AND `flowgraph_nodes.go`
4. **User agents unavailable** - Need `WithSettingSources` with "user"
5. **Worktree cleanup by path** - Use `CleanupWorktreeAtPath(e.worktreePath)` not `CleanupWorktree(taskID)` to handle initiative-prefixed worktrees correctly
6. **Spec not found in templates** - Use `WithSpecFromDatabase()` to load spec content; file-based specs are legacy
7. **Invalid session ID errors** - Only pass custom session IDs when `Persistence: true`; Claude CLI expects UUIDs it generates for ephemeral sessions
8. **Transcripts not persisting** - Ensure `JSONLSyncer.SyncFromFile()` called after phase completion with correct JSONL path
