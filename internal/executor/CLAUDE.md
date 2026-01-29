# Executor Package

Unified workflow execution engine. All execution goes through `WorkflowExecutor` which uses database-first workflows and the variable resolution system.

## File Structure

### WorkflowExecutor (Split into 6 files)

| File | Lines | Key Functions | Purpose |
|------|-------|---------------|---------|
| `workflow_executor.go` | ~790 | `NewWorkflowExecutor()`, `Run()`, `applyPhaseContentToVars()` | Core types, options, entry point, result types |
| `workflow_context.go` | ~440 | `buildResolutionContext()`, `enrichContextForPhase()`, `loadInitiativeContext()` | Context building, initiative/project loading, variable conversion |
| `workflow_phase.go` | ~850 | `executePhase()`, `executePhaseWithTimeout()`, `executeWithClaude()`, `checkSpecRequirements()` | Phase execution, timeout handling, spec validation |
| `workflow_completion.go` | ~420 | `runCompletion()`, `createPR()`, `directMerge()`, `setupWorktree()` | PR creation, merge, worktree setup/cleanup, sync |
| `workflow_state.go` | ~195 | `failRun()`, `failSetup()`, `interruptRun()`, `recordCostToGlobal()` | Failure/interrupt handling, cost tracking, transcript sync |
| `workflow_gates.go` | ~105 | `evaluatePhaseGate()`, `runResourceAnalysis()`, `triggerAutomationEvent()` | Gate evaluation, event publishing, resource tracking |

### Support Files

| File | Purpose |
|------|---------|
| `executor.go` | `PhaseState`, model resolution, Claude path detection |
| `claude_executor.go` | `TurnExecutor` interface, ClaudeCLI wrapper with `--json-schema` |
| `phase_response.go` | JSON schemas for phase completion (`GetSchemaForPhaseWithRound()`) |
| `phase_executor.go` | `PhaseExecutor` interface, weight-based executor config |
| `retry.go` | Retry context building (`BuildRetryContext`, `BuildRetryContextForFreshSession`) |
| `review.go` | Review findings parsing, formatting for round 2 (`FormatFindingsForRound2`) |
| `qa.go` | QA E2E types, parsing, loop condition evaluation |
| `finalize.go` | Branch sync, conflict resolution (see `docs/architecture/FINALIZE.md`) |
| `ci_merge.go` | CI polling, auto-merge with retry logic, commit templates, SHA verification |
| `cost_tracking.go` | `RecordCostEntry()` - global cost recording to `~/.orc/orc.db` |
| `resource_tracker.go` | `RunResourceAnalysis()` - orphan process detection |
| `quality_checks.go` | Phase-level quality checks (tests, lint, build, typecheck) |
| `checklist_validation.go` | Spec and criteria validation |
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
│   ├── SetCurrentPhaseProto(t, id)   # Persist phase to task record (authoritative for `orc status`)
│   ├── executePhaseWithTimeout()     # Run with timeout
│   │   └── executeWithClaude()       # ClaudeExecutor
│   ├── applyPhaseContentToVars()     # Store output for subsequent phases
│   └── recordCostToGlobal()          # Track costs
└── completeRun()              # Finalization, cleanup
```

## Key Functions

### Shared Utilities

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `RecordCostEntry()` | `cost_tracking.go:21` | Records phase costs to global DB |
| `RunResourceAnalysis()` | `resource_tracker.go:531` | Detects orphaned MCP processes |
| `applyPhaseContentToVars()` | `workflow_executor.go:820` | Propagates phase content to subsequent phases |

### Phase Execution

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `executePhaseWithTimeout()` | `workflow_phase.go:567` | Wraps `executePhase()` with PhaseMax timeout |
| `checkSpecRequirements()` | `workflow_phase.go:681` | Validates spec exists for non-trivial weights |
| `IsPhaseTimeoutError()` | `workflow_phase.go:558` | Checks if error is `phaseTimeoutError` |
| `IsPhaseBlockedError()` | `workflow_phase.go:43` | Checks if error is `PhaseBlockedError` |

### Blocked Phase Handling

When a phase outputs `{"status": "blocked"}`, it returns a `PhaseBlockedError` instead of a regular error. This allows blocked phases to proceed to gate evaluation rather than immediately failing the run.

```go
type PhaseBlockedError struct {
    Phase  string  // Phase that blocked
    Reason string  // Why it blocked
    Output string  // Full phase output for context
}
```

**Flow:**
1. Phase outputs `{"status": "blocked", "reason": "..."}`
2. `executePhase()` returns `PhaseBlockedError` (not regular error)
3. Executor checks `IsPhaseBlockedError(err)` - if true, proceeds to gate evaluation
4. Gate evaluation sees blocked status and triggers retry from earlier phase
5. Review phase stores findings to `RetryContext.FailureOutput` for round 2

### Context Building

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `buildResolutionContext()` | `workflow_context.go:71` | Creates initial variable context |
| `enrichContextForPhase()` | `workflow_context.go:198` | Adds phase-specific context |
| `loadInitiativeContext()` | `workflow_context.go:135` | Loads initiative vision/decisions |

### QA E2E Loop Execution

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `EvaluateLoopCondition()` | `qa.go` | Evaluates loop condition against phase output |
| `ParseQAE2ETestResult()` | `qa.go` | Parses qa_e2e_test phase JSON output |
| `ParseQAE2EFixResult()` | `qa.go` | Parses qa_e2e_fix phase JSON output |

**Loop Conditions:**

| Condition | Evaluates True When |
|-----------|---------------------|
| `has_findings` | `findings` array is non-empty |
| `not_empty` | Output is non-empty (trimmed) |
| `status_needs_fix` | Status field is "needs_fix" |

**Loop Flow:**
```
qa_e2e_test outputs findings → LoopConfig.Condition="has_findings" → true → inject qa_e2e_fix → execute fix → loop back to qa_e2e_test → repeat until no findings or MaxIterations
```

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
| QA E2E | QA_ITERATION, QA_MAX_ITERATIONS, BEFORE_IMAGES, PREVIOUS_FINDINGS, QA_FINDINGS |
| Prior Outputs | SPEC_CONTENT, RESEARCH_CONTENT, TDD_TESTS_CONTENT, BREAKDOWN_CONTENT |

See `internal/variable/CLAUDE.md` for resolution sources (static, env, script, API, phase_output).

## Phase Content Storage

| Phase | Storage | Extraction |
|-------|---------|------------|
| spec, research, docs | Database | From JSON `content` field via `--json-schema` |
| implement, test | Code changes only | No content extraction |

**JSON-based content extraction:**
- `GetSchemaForPhase()` returns schema with or without `content` field
- `ExtractPhaseContent()` parses JSON and extracts `content`
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
| spec, research, docs | - | `PhaseCompletionWithContentSchema` |
| review | 1 | `ReviewFindingsSchema` (status: complete/blocked) |
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
| review | implement | Review findings need code changes |
| test, test_unit, test_e2e | implement | Test failures need code fixes |

### Review Multi-Round Flow

The review phase supports multiple rounds via `RetryContext`:

| Round | Template | Trigger | Detection |
|-------|----------|---------|-----------|
| 1 | `review.md` | Initial review | Default (no retry context) |
| 2+ | `review_round2.md` | After implement retry | `RetryContext.FromPhase == "review"` |

**Round Detection:** `loadReviewContextProto()` checks `e.RetryContext.FromPhase` to determine round. When `FromPhase == "review"`, it's round 2+ (we're re-reviewing after fixing issues).

**Findings Flow:**
1. Round 1 blocks → `PhaseBlockedError` with full output
2. Executor stores output in `RetryContext.FailureOutput`
3. Gate triggers retry from implement phase
4. On round 2, findings are parsed from `RetryContext.FailureOutput` and formatted via `FormatFindingsForRound2()`
5. Round 2 uses `review_round2.md` template with `{{REVIEW_FINDINGS}}` populated

**Post-Success Cleanup:** After successful review round 2+, `RetryContext` is cleared to prevent stale context on future runs.

## Model Configuration

Model selection is workflow-based via phase templates (see `internal/workflow/seed.go`):

```
workflow_phases.model_override    # Workflow-specific override
phase_templates.model_override    # Phase template default
config.Model                      # Global fallback
```

**Default per phase template:**
| Phase | Model | Thinking | Rationale |
|-------|-------|----------|-----------|
| spec, review, research | opus | true | Decision phases need deep reasoning |
| tiny_spec, tdd_write, breakdown, implement, docs | opus | false | Execution phases |
| qa | sonnet | false | Test execution is mechanical |

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

Phase templates define checks in `quality_checks` JSON:
```json
[
  {"type": "code", "name": "tests", "enabled": true, "on_failure": "block"},
  {"type": "code", "name": "lint", "enabled": true, "on_failure": "warn"}
]
```

Workflow phases can override via `quality_checks_override`.

### On Failure Modes

| Mode | Behavior |
|------|----------|
| `block` | Fails the phase, injects context for retry |
| `warn` | Logs warning but allows completion |
| `skip` | Skips the check entirely |

**Flow:** Agent outputs `{"status": "complete"}` -> Quality checks run -> Criteria validation -> If any blocking checks fail, inject context and continue.

### Project Commands

Commands are stored in `project_commands` table and seeded during `orc init` based on project detection:

| Name | Example Command |
|------|-----------------|
| tests | `go test ./...` |
| lint | `golangci-lint run` |
| build | `go build ./...` |
| typecheck | `go build -o /dev/null ./...` |

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
| Validation can't see files | Create clients dynamically with correct workdir |

## Task/Execution State Consistency

**CRITICAL:** Task status and execution state are unified in `orcv1.Task` (the proto domain model). When execution fails or is interrupted, update the task and save once:

| Field | Must Update |
|-------|-------------|
| `t.Status` | Set to `TASK_STATUS_FAILED`, `TASK_STATUS_PAUSED`, or `TASK_STATUS_BLOCKED` |
| `t.Execution.Error` | Store error message for user visibility |
| `t.Execution.Phases[phase]` | Update phase status via helper methods |

**Why this matters:** If task.Status stays "running" but the executor dies, the task becomes orphaned - it appears running but has no active process.

### Error Handling Checklist

When adding new error paths:

1. **Store the error:** `t.Execution.Error = err.Error()`
2. **Update phase status:** Use helpers in `internal/task/execution_helpers.go`
3. **Update task status:** `t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED`
4. **Save task:** `e.backend.SaveTask(t)` (saves both task and execution state)
5. **Publish events:** `e.publishError()` and `e.publishState()`

**Always use helper functions** (`failRun`, `failSetup`, `interruptRun`) which handle all cleanup consistently.

### Anti-Patterns

| Bad | Why |
|-----|-----|
| `if err != nil { return err }` | Task still shows "running" |
| Skip `t.Execution.Error = err.Error()` | User can't see what went wrong |
| Forget to call `backend.SaveTask(t)` | Changes not persisted |
