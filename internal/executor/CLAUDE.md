# Executor Package

Unified workflow execution engine. All execution goes through `WorkflowExecutor` which uses database-first workflows and the variable resolution system.

## File Structure

**WorkflowExecutor** (split into 6 modules): `workflow_executor.go` (core), `workflow_context.go` (variable resolution), `workflow_phase.go` (execution), `workflow_completion.go` (PR/merge), `workflow_state.go` (failure handling), `workflow_gates.go` (gate evaluation)

**Support**: `claude_executor.go` (`TurnExecutor`, `--json-schema`), `phase_response.go` (completion schemas), `finalize.go` (sync/conflicts), `ci_merge.go` (auto-merge), `cost_tracking.go`, `quality_checks.go`

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

**Phase execution**: `executePhaseWithTimeout()` (timeout wrapper), `checkSpecRequirements()` (validation), `executeWithClaude()` (Claude CLI)

**Context**: `buildResolutionContext()`, `enrichContextForPhase()`, `loadInitiativeContext()`

**Utilities**: `RecordCostEntry()`, `RunResourceAnalysis()` (orphan detection), `applyArtifactToVars()` (artifact propagation)

## Variable Resolution

All templates use `internal/variable/Resolver`. Context includes: Task (ID, title, description, category, weight), Phase (name, iteration, retry context), Git (worktree, branches), Initiative (vision, decisions), Project (language, frameworks, tests), Prior Outputs (spec, research, tdd, breakdown).

See `internal/variable/CLAUDE.md` for sources.

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

## Transcript Storage

Transcripts are stored directly to the database during phase execution via `ClaudeExecutor`:

1. **User prompts**: Stored before sending to Claude
2. **Assistant responses**: Stored after receiving from Claude

| Role | Content | Stored |
|------|---------|--------|
| `"prompt"` | User/system prompts | Before Claude call |
| `"response"` | Model responses | After Claude call |
| `"chunk"` | Aggregated streaming chunks | During streaming |
| `"combined"` | Full transcript for phase | On phase completion |

**Note:** Direct database storage replaced JSONL file syncing. No filesystem-based transcript reading.

## Cost Tracking

Data flow:
```
TurnResult.Usage → Result{InputTokens, OutputTokens, CostUSD} → recordCostToGlobal() → GlobalDB.RecordCostExtended()
```

**Note:** Use `EffectiveInputTokens()` not raw `InputTokens` (includes cache tokens).

## Quality Checks & Validation

Quality checks are defined at the **phase template level**, not globally. Each phase template can specify which checks to run via the `quality_checks` JSON field.

| Component | File | Purpose |
|-----------|------|---------|
| QualityCheckRunner | `quality_checks.go` | Runs phase-level quality checks after completion claim |
| Haiku Validation | `checklist_validation.go` | Validates spec/criteria quality |

### Quality Check Types

| Type | Behavior |
|------|----------|
| `code` | Looks up command from `project_commands` table by name (tests, lint, build, typecheck) |
| `custom` | Uses the `command` field directly |

### Quality Check Configuration

Phase templates define checks (tests, lint, build, typecheck). On failure modes: `block` (fails phase, retries with context), `warn` (log only), `skip` (disabled).

Project commands seeded during `orc init` and stored in `project_commands` table.

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
| Validation can't see files | Create clients dynamically with correct workdir |
