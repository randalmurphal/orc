# Executor Package

Unified workflow execution engine. All execution goes through `WorkflowExecutor` which uses database-first workflows and the variable resolution system.

## File Structure

### WorkflowExecutor (Split into 6 files)

| File | Lines | Key Functions | Purpose |
|------|-------|---------------|---------|
| `workflow_executor.go` | ~740 | `NewWorkflowExecutor()`, `Run()`, `applyArtifactToVars()` | Core types, options, entry point, result types |
| `workflow_context.go` | ~430 | `buildResolutionContext()`, `enrichContextForPhase()`, `loadInitiativeContext()` | Context building, initiative/project loading, variable conversion |
| `workflow_phase.go` | ~580 | `executePhase()`, `executePhaseWithTimeout()`, `executeWithClaude()`, `checkSpecRequirements()` | Phase execution, timeout handling, spec validation |
| `workflow_completion.go` | ~380 | `runCompletion()`, `createPR()`, `directMerge()`, `setupWorktree()` | PR creation, merge, worktree setup/cleanup, sync |
| `workflow_state.go` | ~240 | `failRun()`, `failSetup()`, `interruptRun()`, `recordCostToGlobal()` | Failure/interrupt handling, cost tracking, transcript sync |
| `workflow_gates.go` | ~100 | `evaluatePhaseGate()`, `runResourceAnalysis()`, `triggerAutomationEvent()` | Gate evaluation, event publishing, resource tracking |

### Support Files

| File | Purpose |
|------|---------|
| `executor.go` | `PhaseState`, model resolution, Claude path detection |
| `claude_executor.go` | `TurnExecutor` interface, ClaudeCLI wrapper with `--json-schema` |
| `phase_response.go` | JSON schemas for phase completion (`GetSchemaForPhaseWithRound()`) |
| `phase_executor.go` | `PhaseExecutor` interface, `ResolveModelSetting()` |
| `finalize.go` | Branch sync, conflict resolution (see `docs/architecture/FINALIZE.md`) |
| `ci_merge.go` | CI polling and auto-merge with retry logic |
| `cost_tracking.go` | `RecordCostEntry()` - global cost recording to `~/.orc/orc.db` |
| `resource_tracker.go` | `RunResourceAnalysis()` - orphan process detection |
| `backpressure.go` | Deterministic quality checks (tests, lint, build) |
| `haiku_validation.go` | Spec and criteria validation |
| `jsonl_sync.go` | `JSONLSyncer` for Claude JSONL to DB sync |
| `heartbeat.go` | Periodic heartbeat updates during execution |

## Architecture

```
WorkflowExecutor.Run()
├── setupForContext()          # Task/branch/standalone setup
├── loadWorkflow()             # Get phases from database
├── checkSpecRequirements()    # Validate spec exists for non-trivial weights
├── buildResolutionContext()   # Create variable context
├── for each phase:
│   ├── enrichContextForPhase()       # Add phase-specific context
│   ├── resolver.ResolveAll()         # Resolve all variables
│   ├── evaluateGate()                # Check conditions
│   ├── executePhaseWithTimeout()     # Run with timeout
│   │   └── executeWithClaude()       # ClaudeExecutor
│   ├── applyArtifactToVars()         # Store output for subsequent phases
│   └── recordCostToGlobal()          # Track costs
└── completeRun()              # Finalization, cleanup
```

## Key Functions

### Shared Utilities

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `RecordCostEntry()` | `cost_tracking.go:22` | Records phase costs to global DB |
| `RunResourceAnalysis()` | `resource_tracker.go:538` | Detects orphaned MCP processes |
| `applyArtifactToVars()` | `workflow_executor.go:703` | Propagates phase artifacts to subsequent phases |

### Phase Execution

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `executePhaseWithTimeout()` | `workflow_phase.go:421` | Wraps `executePhase()` with PhaseMax timeout |
| `checkSpecRequirements()` | `workflow_phase.go:535` | Validates spec exists for non-trivial weights |
| `IsPhaseTimeoutError()` | `workflow_phase.go:412` | Checks if error is `phaseTimeoutError` |

### Context Building

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `buildResolutionContext()` | `workflow_context.go:71` | Creates initial variable context |
| `enrichContextForPhase()` | `workflow_context.go:198` | Adds phase-specific context |
| `loadInitiativeContext()` | `workflow_context.go:135` | Loads initiative vision/decisions |

## Variable Resolution

All template variables resolved via `internal/variable/Resolver`. Resolution context includes:

| Category | Variables |
|----------|-----------|
| Task | TASK_ID, TASK_TITLE, TASK_DESCRIPTION, TASK_CATEGORY, WEIGHT |
| Phase | PHASE, ITERATION, RETRY_CONTEXT |
| Git | WORKTREE_PATH, PROJECT_ROOT, TASK_BRANCH, TARGET_BRANCH |
| Initiative | INITIATIVE_ID, INITIATIVE_TITLE, INITIATIVE_VISION, INITIATIVE_DECISIONS |
| Review | REVIEW_ROUND, REVIEW_FINDINGS |
| Project | LANGUAGE, HAS_FRONTEND, HAS_TESTS, TEST_COMMAND, FRAMEWORKS |
| Prior Outputs | SPEC_CONTENT, RESEARCH_CONTENT, TDD_TESTS_CONTENT, BREAKDOWN_CONTENT |

See `internal/variable/CLAUDE.md` for resolution sources (static, env, script, API, phase_output).

## Artifact Storage

| Phase | Storage | Extraction |
|-------|---------|------------|
| spec, design, research, docs | Database | From JSON `artifact` field via `--json-schema` |
| implement, test, validate | Code changes only | No artifact extraction |

**JSON-based artifact extraction:**
- `GetSchemaForPhase()` returns schema with or without `artifact` field
- `ExtractArtifactFromOutput()` parses JSON and extracts `artifact`
- `SaveSpecToDatabase()` extracts spec from JSON and saves to database
- **Failure handling:** Extraction failures call `failRun()` to ensure task status becomes `StatusFailed`

## Completion Detection

Claude outputs completion via `--json-schema`:

```json
{"status": "complete", "summary": "Work done"}
{"status": "blocked", "reason": "Need X"}
{"status": "continue", "reason": "In progress"}
```

### Schema Selection (`phase_response.go`)

| Phase | Round | Schema |
|-------|-------|--------|
| spec, design, research, docs | - | `PhaseCompletionWithArtifactSchema` |
| review | 1 | `ReviewFindingsSchema` |
| review | 2 | `ReviewDecisionSchema` |
| qa | - | `QAResultSchema` |
| other | - | `PhaseCompletionSchema` |

### Parsing Functions

| Function | Use Case |
|----------|----------|
| `ParsePhaseSpecificResponse()` | Route to correct parser by phase type |
| `CheckPhaseCompletionJSON()` | Parse standard completion (returns error - MUST handle) |

## Phase Retry

When phases fail or output `{"status": "blocked"}`:

| Failed Phase | Retries From | Reason |
|--------------|--------------|--------|
| design | spec | Design issues from incomplete spec |
| review | implement | Review findings need code changes |
| test, test_unit, test_e2e | implement | Test failures need code fixes |
| validate | implement | Validation issues need code changes |

## Model Configuration

Per-phase, per-weight model selection via `ResolveModelSetting(weight, phase)`:

```
config.OrcConfig.Models[weight][phase]  # Phase-specific
config.OrcConfig.Models.Default         # Global default
config.Model                            # Legacy fallback
```

**Default matrix:** Decision phases (spec, review, validate) use opus + thinking; execution phases (implement, test, docs) use sonnet.

## Claude Call Patterns

### Pattern 1: TurnExecutor for Phase Execution

```go
turnExec := NewClaudeExecutor(
    WithClaudePath(claudePath),
    WithClaudeWorkdir(workingDir),
    WithClaudeModel(model),
    WithClaudeSessionID(sessionID),
    WithClaudeMaxTurns(maxIterations),
)
result, err := turnExec.ExecuteTurn(ctx, prompt)              // With JSON schema
result, err := turnExec.ExecuteTurnWithoutSchema(ctx, prompt) // Freeform output
```

### Pattern 2: Schema-Constrained Validation

**Use `llmutil.ExecuteWithSchema[T]()` for ALL schema-constrained LLM calls:**

```go
schemaResult, err := llmutil.ExecuteWithSchema[responseType](ctx, client, prompt, schema)
if err != nil {
    return nil, fmt.Errorf("validation failed: %w", err)  // ALWAYS propagate error
}
```

## Transcript Sync (JSONL)

`jsonl_sync.go` syncs Claude JSONL session files to DB:

1. **Real-time streaming**: `TranscriptStreamer` watches JSONL via fsnotify, batches to DB
2. **Post-phase catchup**: `SyncFromFile()` ensures no messages missed

| Data | Source | DB Table |
|------|--------|----------|
| Messages | `message.content` | `transcripts` |
| Tokens | `message.usage` | `transcripts` (per-message) |
| Tool calls | `content[type=tool_use]` | `transcripts.tool_calls` |

## Cost Tracking

Data flow:
```
TurnResult.Usage → Result{InputTokens, OutputTokens, CostUSD} → recordCostToGlobal() → GlobalDB.RecordCostExtended()
```

**Note:** Use `EffectiveInputTokens()` not raw `InputTokens` (includes cache tokens).

## Backpressure & Validation

| Component | File | Purpose |
|-----------|------|---------|
| Backpressure | `backpressure.go:146` | Runs tests/lint/build after completion claim |
| Haiku Validation | `haiku_validation.go` | Validates spec/criteria quality |

**Flow:** Agent outputs `{"status": "complete"}` -> Backpressure runs -> Criteria validation -> If any fail, inject context and continue.

**API Error Handling:** `config.Validation.FailOnAPIError` - `true` fails task properly (resumable), `false` continues without validation.

## Heartbeat Runner

`heartbeat.go` provides periodic updates during long-running phases to prevent false orphan detection.

- **Interval:** 2 minutes between heartbeat updates
- **Purpose:** Long implement phases can take hours; without heartbeats, task appears orphaned
- **Priority:** PID check takes precedence over heartbeat staleness (live PID = healthy task)

```go
heartbeatRunner := NewHeartbeatRunner(e.Backend, state, e.Logger)
heartbeatRunner.Start(ctx)
defer heartbeatRunner.Stop()
```

## Testing

```bash
go test ./internal/executor/... -v
```

**Mock injection:**
```go
mock := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
executor := NewWorkflowExecutor(backend, projectDB, orcConfig, workDir,
    WithWorkflowTurnExecutor(mock),
)
```

| Executor | Option |
|----------|--------|
| `WorkflowExecutor` | `WithWorkflowTurnExecutor(mock)` |
| `FinalizeExecutor` | `WithFinalizeTurnExecutor(mock)` |
| `ConflictResolver` | `WithResolverTurnExecutor(mock)` |

## Common Gotchas

| Issue | Solution |
|-------|----------|
| Raw InputTokens misleading | Use `EffectiveInputTokens()` |
| Ultrathink in system prompt | Must be user message |
| User agents unavailable | Need `WithSettingSources` with "user" |
| Worktree cleanup by path | Use `CleanupWorktreeAtPath(e.worktreePath)` |
| Spec not found in templates | Use `WithSpecFromDatabase()` |
| Invalid session ID errors | Only pass custom session IDs when `Persistence: true` |
| Transcripts not persisting | Ensure `SyncFromFile()` called after phase |
| Validation can't see files | Create clients dynamically with correct workdir |
