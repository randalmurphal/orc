# Executor Package

Unified workflow execution engine. All execution goes through `WorkflowExecutor` which uses database-first workflows and the variable resolution system.

## File Structure

### WorkflowExecutor (Split into 8 files)

| File | Lines | Key Functions | Purpose |
|------|-------|---------------|---------|
| `workflow_executor.go` | ~400 | `NewWorkflowExecutor()`, `Run()`, `applyPhaseContentToVars()` | Core types, options, entry point, phase loop logic, gate action dispatch |
| `workflow_context.go` | ~196 | `buildResolutionContext()`, `enrichContextForPhase()`, `loadInitiativeContext()` | Context building, initiative/project loading, variable conversion |
| `workflow_phase.go` | ~195 | `executePhase()`, `executePhaseWithTimeout()`, `executeWithProvider()`, `resolvePhaseProvider()`, `checkSpecRequirements()` | Phase execution, timeout handling, provider-adapter dispatch, spec validation |
| `workflow_completion.go` | ~380 | `runCompletion()`, `createPR()`, `directMerge()`, `ResolvePROptions()`, `cleanupSyncFailure()`, `detectExistingWork()` | PR creation/reuse, merge, worktree setup/cleanup, sync, work-aware cleanup |
| `workflow_state.go` | ~300 | `failRun()`, `failSetup()`, `interruptRun()`, `commitWIPOnInterrupt()`, `recordCostToGlobal()` | Failure/interrupt handling, work preservation, cost tracking |
| `workflow_gates.go` | ~250 | `evaluatePhaseGate()`, `applyGateOutputToVars()`, `resolveGateType()`, `runGateScript()` | Gate evaluation (auto/human/AI), output variable pipeline, script execution, type resolution |
| `workflow_triggers.go` | ~127 | `evaluateBeforePhaseTriggers()`, `fireLifecycleTriggers()`, `handleCompletionWithTriggers()` | Trigger evaluation (before-phase + lifecycle events) |
| `gate_actions.go` | ~295 | `resolveApprovedAction()`, `resolveRejectedAction()`, `resolveRetryFrom()` | Gate output action resolution: maps `OnApproved`/`OnRejected` config to `GateAction` enum |

### Support Files

| File | Purpose |
|------|---------|
| `branch.go` | Branch resolution: `ResolveTargetBranch()`, `ResolveBranchName()`, `IsDefaultBranch()` |
| `executor.go` | `Result` struct, `ResolveClaudePath()`, `findClaudeInCommonLocations()` |
| `claude_executor.go` | `TurnExecutor` interface, `MockTurnExecutor`, `ClaudeExecutor` wrapper |
| `phase_response.go` | JSON schemas for phase completion (`GetSchemaForPhaseWithRound()`) |
| `phase_executor.go` | `PhaseExecutor` interface, weight-based executor config |
| `phase_registry.go` | `PhaseTypeRegistry`, `PhaseTypeExecutor` interface — maps type strings to executors |
| `script_executor.go` | `ScriptPhaseExecutor` — runs shell commands as phases (command, workdir, timeout, success_pattern, output_var) |
| `api_executor.go` | `APIPhaseExecutor` — makes HTTP requests as phases (method, URL, headers, body, success_status, output_var) |
| `knowledge_executor.go` | `KnowledgePhaseExecutor`, `KnowledgeQueryService` interface — knowledge retrieval phase |
| `retry.go` | Retry context building (`BuildRetryContextForFreshSession`, `CompressPreviousContext`, `BuildRetryPreview`) |
| `review.go` | Review findings parsing, formatting for round 2 (`FormatFindingsForRound2`) |
| `qa.go` | QA E2E types, parsing (`ParseQAE2ETestResult`, `ParseQAE2EFixResult`) |
| `finalize.go` | Branch sync, test fixing with retry (see `docs/architecture/FINALIZE.md`) |
| `conflict_resolver.go` | Automatic merge conflict resolution via Claude sub-agent |
| `ci_merge.go` | CI polling, auto-merge with retry logic, commit templates, SHA verification |
| `cost_tracking.go` | `RecordCostEntry()` - global cost recording; `TokenRate`, `EstimateTokenCostUSD()` - provider-aware cost estimation |
| `resource_tracker.go` | `RunResourceAnalysis()` - orphan process detection |
| `quality_checks.go` | Phase-level quality checks (tests, lint, build, typecheck) |
| `checklist_validation.go` | Spec and criteria validation |
| `phase_settings.go` | Unified `ApplyPhaseSettings()` (Claude) + `applyCodexInstructions()` (.codex/instruction.md) |
| `claude_hooks.go` | `applyPhaseHooks()` - writes hooks to `.claude/settings.local.json` |
| `hook_scripts.go` | `applyPhaseHookScripts()` - copies scripts to `.claude/hooks/` |
| `heartbeat.go` | `HeartbeatRunner` - periodic heartbeat updates during execution (`DefaultHeartbeatInterval=2m`) |
| `history.go` | `StartRun()`, `CompleteRun()`, `FailRun()`, `InterruptRun()` - append-only run history tracking |
| `idle_guard.go` | `IdleGuard` - monitors heartbeat freshness, detects stale executors (2m interval, 15m timeout) |
| `condition.go` | Condition evaluator: `EvaluateCondition()`, `ConditionContext` (phase skip + loop conditions) |
| `topo_sort.go` | Phase ordering: `topologicalSort()`, `computeExecutionLevels()` (DAG execution levels for parallel phases) |
| `phase_loop_test.go` | Phase loop integration tests (10 success criteria + failure modes) |
| `scratchpad.go` | Scratchpad extraction, formatting, persistence, and context population |
| `docs_response.go` | Docs phase response parsing: `ParseDocsResponse()`, `PersistInitiativeNotes()` (knowledge curator integration) |
| `provider.go` | Provider resolution: `resolvePhaseProvider()`, `ParseProviderModel()`, `isCodexFamilyProvider()`, `normalizeProvider()`, `validateProviderCapabilities()` |
| `provider_adapter.go` | `ProviderAdapter` interface, `claudeAdapter`, `codexAdapter` — isolate provider-specific behavior around shared `executeWithProvider()` loop |
| `codex_executor.go` | `CodexExecutor` — Codex CLI wrapper, session management, JSON event parsing |
| `transcript_stream.go` | Transcript streaming; `StoreAssistantText()`, `OnCodexEvent()` for transcript ingestion |

## Two-Tier Database Access

`WorkflowExecutor` requires both `ProjectDB` and `GlobalDB`:

```go
we := NewWorkflowExecutor(backend, projectDB, globalDB, orcConfig, workingDir, opts...)
```

| Database | Used For | Call Sites |
|----------|----------|------------|
| `globalDB` | Workflow definitions: `GetWorkflow()`, `GetWorkflowPhases()`, `GetWorkflowVariables()`, `GetPhaseTemplate()` | `workflow_executor.go:326-641`, `parallel_execution.go:254` |
| `projectDB` | Execution records: workflow runs, phases, transcripts, events | Throughout executor |

**Why both?** Workflow definitions are seeded to GlobalDB (shared across projects). ProjectDB holds per-project execution data. Reading definitions from ProjectDB returns stale/empty data.

## Architecture

```
WorkflowExecutor.Run()
├── setupForContext()          # Task/branch/standalone setup
├── loadWorkflow()             # Get workflow + phases from GlobalDB
├── checkSpecRequirements()    # Validate spec exists for non-trivial weights
├── buildResolutionContext()   # Create variable context
├── for each phase:
│   ├── shouldSkipPhase()             # Evaluate phase condition (condition.go)
│   ├── enrichContextForPhase()       # Add phase-specific context (incl. scratchpad)
│   ├── resolver.ResolveAll()         # Resolve all variables
│   ├── evaluateBeforePhaseTriggers() # Run before-phase triggers (gate/reaction)
│   ├── evaluatePhaseGate()            # Gate evaluation (auto/human/AI via gate.Evaluator)
│   ├── applyGateOutputToVars()       # Store gate output data as workflow variable (if configured)
│   ├── [gate action dispatch]        # OnApproved/OnRejected → continue/retry/fail/skip/script
│   ├── SetCurrentPhaseProto(t, id)   # Persist phase to task record (authoritative for `orc status`)
│   ├── ApplyPhaseSettings()          # Configure Claude Code env (hooks, skills, hook scripts)
│   ├── executePhaseWithTimeout()     # Run with timeout
│   │   ├── [phase type dispatch]     # TypeOverride > Template.Type > "llm"
│   │   │   ├── non-LLM → phaseTypeRegistry.Get(type).ExecutePhase()
│   │   │   └── "llm" → fall through to LLM provider path
│   │   ├── resolvePhaseProvider()    # Priority: run flag > phase > workflow > template > config > "claude"
│   │   ├── providerAdapterFor()     # Returns claudeAdapter or codexAdapter
│   │   └── executeWithProvider()    # Shared orchestration loop (all providers)
│   ├── persistScratchpadEntries()     # Save scratchpad notes from phase output
│   ├── applyPhaseContentToVars()     # Store output for subsequent phases
│   ├── evaluateLoopConfig()          # Check loop_config: condition + max_loops → jump back
│   └── recordCostToGlobal()          # Track costs
├── handleCompletionWithTriggers()    # on_task_completed gates
├── fireLifecycleTriggers()           # on_task_failed (on failure path)
└── completeRun()              # Finalization, cleanup
```

## Branch Resolution (`branch.go`)

### Target Branch (PR destination)

5-level priority hierarchy resolved by `ResolveTargetBranch()`:

| Priority | Source | Example |
|----------|--------|---------|
| 1 | `task.TargetBranch` | Task-level override |
| 2 | `initiative.BranchBase` | Inherited from initiative |
| 3 | `developer.StagingBranch` | Personal staging (when enabled) |
| 4 | `config.Completion.TargetBranch` | Project default |
| 5 | `"main"` | Hardcoded fallback |

Invalid branch names at any level fall back to `"main"` with a warning log.

### Task Branch (feature branch name)

Resolved by `ResolveBranchName()` at `branch.go:169`:

| Priority | Source | Result |
|----------|--------|--------|
| 1 | `task.BranchName` (if valid) | Custom name as-is |
| 2 | Auto-generated | `orc/TASK-001` or `prefix/TASK-001` (with initiative prefix) |

### PR Creation Flow (`workflow_completion.go:220`)

`createPR()` is **idempotent** — safe to call on resume or retry:

```
createPR()
├── HasPRProto(t)?           → skip (fast path: task already has PR metadata)
├── PushWithForceFallback()  → push branch to remote
├── FindPRByBranch()         → check for existing open PR on branch
│   ├── Found?               → reuse: UpdatePR(title/body) + save PR info
│   ├── ErrNoPRFound?        → create new PR via CreatePR()
│   └── Network error?       → log warning, fall through to CreatePR()
└── applyPRAutomation()      → auto-merge/approve on both new and reused PRs
```

**Key behaviors:**
- Reused PRs get title/body updated to match current task
- `FindPRByBranch` failure is best-effort (doesn't block PR creation)
- `UpdatePR` failure on reuse logs warning but saves PR info (PR exists, just stale metadata)
- PR info persisted to task via `task.SetPRInfoProto()` + `backend.SaveTask()`

### Automatic Conflict Resolution (`conflict_resolver.go`)

When sync with target branch detects merge conflicts and `sync.auto_resolve: true`:

```
runCompletion()
├── RebaseWithConflictCheck()    → detect conflicts
├── if conflicts && AutoResolve:
│   └── attemptConflictResolution()
│       └── ConflictResolver.Resolve()
│           ├── buildPrompt()        → create resolution prompt with task context
│           ├── TurnExecutor         → spawn Claude sub-agent (configurable model)
│           ├── verify resolution    → git diff --name-only --diff-filter=U
│           └── retry if needed      → up to MaxResolveAttempts
└── if resolved: continue to PR
```

**Config** (`sync:` in `.orc/config.yaml`):

| Field | Default | Purpose |
|-------|---------|---------|
| `auto_resolve` | `false` | Enable automatic conflict resolution |
| `max_resolve_attempts` | `2` | Retry limit before manual intervention |
| `resolve_model` | `"sonnet"` | Claude model for resolution |

**Key behaviors:**
- Resolution uses task context (ID, title, description) to inform decisions
- After resolution, verifies no unmerged files remain via `git diff --diff-filter=U`
- On failure, task continues with conflicts for manual resolution or retry from implement

### PR Options (task overrides)

`ResolvePROptions()` at `workflow_completion.go:194` merges project-level PR config with task-level overrides:

| Task Field | Override Behavior |
|------------|-------------------|
| `PrDraft` | Overrides `config.Completion.PR.Draft` (if non-nil) |
| `PrLabels` | Replaces project labels (if `PrLabelsSet` is true) |
| `PrReviewers` | Replaces project reviewers (if `PrReviewersSet` is true) |

### Worktree Creation (`worktree.go`)

`SetupWorktreeForTask()` routes to custom or standard worktree creation:
- **Custom branch** (`task.BranchName` set): Uses `gitOps.CreateWorktreeWithCustomBranch()`, worktree path derived from branch name
- **Standard**: Uses `gitOps.CreateWorktreeWithInitiativePrefix()`, worktree path derived from task ID

Worktrees are created at `~/.orc/worktrees/<project-id>/orc-TASK-XXX/` (outside the project directory). The worktree path is resolved via `config.ResolveWorktreeDir()` and displayed in `orc show` and `orc status`.

## Key Functions

### Shared Utilities

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `ResolveBranchName()` | `branch.go:169` | Resolves task branch name: custom name (if valid) > auto-generated |
| `ResolveTargetBranch()` | `branch.go:34` | Resolves PR target branch via 5-level hierarchy |
| `ResolvePROptions()` | `workflow_completion.go:194` | Merges project PR config with task-level overrides (draft, labels, reviewers) |
| `RecordCostEntry()` | `cost_tracking.go:21` | Records phase costs to global DB |
| `RunResourceAnalysis()` | `resource_tracker.go:531` | Detects orphaned MCP processes |
| `applyPhaseContentToVars()` | `workflow_executor.go:739` | Propagates phase content to subsequent phases |
| `applyGateOutputToVars()` | `workflow_gates.go:235` | Stores gate output data as JSON workflow variable; persists to `rctx.PhaseOutputVars` to survive retry (see `docs/architecture/GATES.md`) |
| `resolveApprovedAction()` | `gate_actions.go:18` | Maps `OnApproved` config to `GateAction` (default: `continue`) |
| `resolveRejectedAction()` | `gate_actions.go:31` | Maps `OnRejected` config to `GateAction` (empty = legacy behavior) |
| `resolveRetryFrom()` | `gate_actions.go:44` | Determines retry target: `OutputConfig.RetryFrom` > `tmpl.RetryFromPhase` |
| `runGateScript()` | `workflow_gates.go:145` | Executes gate output script; script can override gate decision |
| `ApplyPhaseSettings()` | `phase_settings.go:34` | Unified phase settings: reset → load config → hooks + skills + scripts |
| `applyCodexInstructions()` | `phase_settings.go:399` | Writes `.codex/instruction.md` when configured |

### Phase Execution

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `executePhaseWithTimeout()` | `workflow_phase.go:567` | Wraps `executePhase()` with PhaseMax timeout |
| `checkSpecRequirements()` | `workflow_phase.go:681` | Validates spec exists for non-trivial weights |
| `IsPhaseTimeoutError()` | `workflow_phase.go:558` | Checks if error is `phaseTimeoutError` |
| `IsPhaseBlockedError()` | `workflow_phase.go:43` | Checks if error is `PhaseBlockedError` |

### Phase Type Dispatch (`phase_registry.go`, `knowledge_executor.go`, `workflow_phase.go:123`)

Non-LLM phase types bypass prompt loading and Claude execution. Instead, they dispatch to a `PhaseTypeExecutor` registered in `PhaseTypeRegistry`.

**Type resolution order:** `WorkflowPhase.TypeOverride` > `PhaseTemplate.Type` > `"llm"` (default)

| Type | Executor | Behavior |
|------|----------|----------|
| `llm` | Sentinel (never called) | Falls through to provider dispatch (`providerAdapterFor()` → `executeWithProvider()`) |
| `knowledge` | `KnowledgePhaseExecutor` | Queries knowledge service, stores result in workflow variable |
| `script` | `ScriptPhaseExecutor` | Runs shell command, captures stdout, optional regex validation |
| `api` | `APIPhaseExecutor` | Makes HTTP request, captures response body, checks status code |

**Registration:** `NewDefaultPhaseTypeRegistry()` pre-registers `llm`, `knowledge`, `script`, and `api`. Custom types via `WithPhaseTypeExecutor(name, exec)`. If `WithWorkflowKnowledgeService(svc)` is used, the knowledge executor is re-registered with the live service.

**KnowledgePhaseExecutor** (`knowledge_executor.go`):
- Requires `KnowledgeQueryService` interface (satisfied by `*knowledge.Service`)
- Query source: `KnowledgePhaseConfig.Query` > `task.Description` > `task.Title`
- Output stored to workflow variables via `KnowledgePhaseConfig.OutputVar` or `PhaseTemplate.OutputVarName`
- Fallback: `"skip"` returns `SKIPPED` status; `"error"` (default) returns error

**Condition field:** `knowledge.available` resolves to `"true"`/`"false"` based on `we.knowledgeService != nil && svc.IsAvailable()`. Use in phase conditions to skip knowledge phases when no service is configured.

**ScriptPhaseExecutor** (`script_executor.go`):
- Command source: `SCRIPT_COMMAND` variable > `PhaseTemplate.PromptContent`
- Variable interpolation on command and workdir via `variable.RenderTemplate()`
- Optional timeout via `context.WithTimeout`, optional `success_pattern` regex validation on stdout
- Output stored via `storeOutputVar()` to both `params.Vars` and `params.RCtx.PhaseOutputVars`
- Empty command → completes with empty content (no error)

**APIPhaseExecutor** (`api_executor.go`):
- URL source: `PhaseTemplate.PromptContent` (after variable interpolation) > URL-like values in `params.Vars`
- Defaults: method=`GET`, success_status=`[200]`
- Variable interpolation on URL, body, and headers
- 1MB response body limit (`maxAPIResponseBody`), truncated if exceeded
- Empty URL → completes with empty content (no error)

**Shared utilities** (`script_executor.go`):
- `storeOutputVar(params, outputVar, value)` — stores to both `Vars` and `RCtx.PhaseOutputVars` when outputVar is non-empty
- `durationMS(start)` — elapsed milliseconds with minimum of 1

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

### Phase Loop System (`workflow_executor.go:721-841`)

Generic loop-back mechanism. Any phase can loop to an earlier phase based on configurable conditions stored in `WorkflowPhase.LoopConfig` (JSON in DB).

**Loop Config** (`db.LoopConfig` at `db/workflow.go:114`):

| Field | Type | Purpose |
|-------|------|---------|
| `loop_to_phase` | string | Target phase (must precede current) |
| `condition` | JSON | Legacy string OR JSON object condition |
| `max_loops` | int | Loop limit (`EffectiveMaxLoops()`: max_loops > max_iterations > 3) |

**Execution Flow** (after phase completes):
1. Parse `phase.LoopConfig` → `db.LoopConfig`
2. Evaluate condition (legacy string → `evaluateLoopCondition()`, JSON object → `EvaluateCondition()`)
3. Check iteration count < `EffectiveMaxLoops()`
4. Find target phase index (must be earlier)
5. Reset phases from target to current → PENDING
6. Increment `PhaseState.Iterations` (unified with gate retry counter)
7. Publish `PhaseLoop` event (`events/publish_helper.go:176`)
8. Jump index back to target phase

**Fail-safe behavior:** Invalid condition → no loop. Missing target → no loop. Max exceeded → continue forward.

**Legacy loop conditions** (string format, used by QA E2E):

| Condition | Evaluates True When |
|-----------|---------------------|
| `has_findings` | `findings` array is non-empty |
| `not_empty` | Output is non-empty (trimmed) |
| `status_needs_fix` | Status field is "needs_fix" |

**JSON object conditions** use the same `EvaluateCondition()` system as phase skip conditions (see `docs/architecture/PHASE_MODEL.md`).

## Variable Resolution

All template variables resolved via `internal/variable/Resolver`. Resolution context includes:

| Category | Variables |
|----------|-----------|
| Task | TASK_ID, TASK_TITLE, TASK_DESCRIPTION, TASK_CATEGORY, WEIGHT |
| Phase | PHASE, ITERATION, RETRY_ATTEMPT, RETRY_FROM_PHASE, RETRY_REASON |
| Git | WORKTREE_PATH, PROJECT_ROOT, TASK_BRANCH, TARGET_BRANCH |
| Control Plane | PENDING_RECOMMENDATIONS, ATTENTION_SUMMARY, HANDOFF_CONTEXT |
| Initiative | INITIATIVE_ID, INITIATIVE_TITLE, INITIATIVE_VISION, INITIATIVE_DECISIONS |
| Review | REVIEW_ROUND, REVIEW_FINDINGS |
| Project | LANGUAGE, HAS_FRONTEND, HAS_TESTS, TEST_COMMAND, FRAMEWORKS, ERROR_PATTERNS |
| QA E2E | QA_ITERATION, QA_MAX_ITERATIONS, BEFORE_IMAGES, PREVIOUS_FINDINGS, QA_FINDINGS |
| Scratchpad | PREV_SCRATCHPAD (prior phases' notes), RETRY_SCRATCHPAD (prior attempt's notes) |
| Gate Outputs | Custom variable names via `GateOutputConfig.variable_name` (JSON-serialized) |
| Phase Outputs | Data-driven from phase template `output_var_name`. Built-ins: SPEC_CONTENT, RESEARCH_CONTENT, TDD_TESTS_CONTENT, BREAKDOWN_CONTENT, QA_FINDINGS. Generic: OUTPUT_{PHASE_ID} |

See `internal/variable/CLAUDE.md` for resolution sources (static, env, script, API, phase_output).

`enrichContextForPhase()` also hydrates the control-plane variables from project state before resolution: pending recommendations are summarized for prompt injection, blocked and failed tasks are compacted into `ATTENTION_SUMMARY`, and the current task gets a bounded `HANDOFF_CONTEXT`. Each value is cleared to an empty string when the source data is unavailable.

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

## Gate Output Action Dispatch (`gate_actions.go`, `workflow_executor.go`)

When a gate evaluates, its `GateOutputConfig.OnApproved`/`OnRejected` determines what happens next. Action resolution is in `gate_actions.go`; dispatch logic is in the main phase loop (`workflow_executor.go:896-1094`).

### On Rejection

| Action | Behavior | Fallback |
|--------|----------|----------|
| `fail` | Fails task immediately via `failGateRejection()` | — |
| `retry` | Retries from `RetryFrom` phase; fails if max retries exceeded | Fail on missing `retry_from` or max exceeded |
| `skip_phase` | Skips current phase, continues to next | — |
| `continue` | Same as `skip_phase` | — |
| `run_script` | Script already ran in `evaluatePhaseGate()`; applies `fail` as secondary | Warn if empty script path |
| `""` (empty) | **Legacy behavior**: review→fail, other phases→continue | — |

### On Approval

| Action | Behavior |
|--------|----------|
| `continue` | Default — proceed to next phase |
| `skip_phase` | Skips the NEXT phase in sequence (marks it `SKIPPED`) |
| `run_script` | Script already ran in `evaluatePhaseGate()`; secondary is continue |
| `""` (empty) | Same as `continue` |

### Key Behaviors

- **Action validation**: Invalid actions fall back to `continue` (approved) or `""` (rejected/legacy)
- **Script execution**: Scripts run during `evaluatePhaseGate()` via `runGateScript()`, can override gate decision (flip approved↔rejected)
- **RetryFrom precedence**: `OutputConfig.RetryFrom` > `tmpl.RetryFromPhase` > config retry map (`resolveRetryFrom()` at `gate_actions.go:44`)
- **OutputConfig on GateEvaluationResult**: Parsed from `PhaseTemplate.GateOutputConfig` and carried on `GateEvaluationResult.OutputConfig` for dispatch access

### Implementation

```
evaluatePhaseGate()                    # workflow_gates.go — evaluates gate, runs script
  ├── resolveRetryFrom()               # gate_actions.go — determines retry target
  └── runGateScript()                  # workflow_gates.go — script execution + override

[main phase loop]                      # workflow_executor.go
  ├── if !approved:
  │   ├── resolveRejectedAction()      # gate_actions.go — maps config to action
  │   └── switch: fail/retry/skip/continue/run_script/legacy
  └── if approved:
      ├── resolveApprovedAction()      # gate_actions.go — maps config to action
      └── switch: skip_phase/run_script/continue
```

## Phase Settings (`phase_settings.go`, `claude_hooks.go`, `hook_scripts.go`)

Unified system to configure Claude Code's environment per-phase. Config source: `.orc/config.yaml` under `phase_settings:`.

**Flow:** `ApplyPhaseSettings()` → reset previous → load config → apply hooks + skills + scripts

| Component | Target File | Applied By |
|-----------|------------|------------|
| Hooks (PreToolUse, PostToolUse, etc.) | `.claude/settings.local.json` | `applyPhaseHooks()` |
| Skills (file paths) | `.claude/settings.json` (merged) | `applyPhaseSkills()` |
| Hook scripts (`.orc/hooks/{phase}/`) | `.claude/hooks/` (copied) | `applyPhaseHookScripts()` |

**Resolution:** phase-specific settings > `"default"` > empty. Config loaded via `internal/config.LoadPhaseSettings()`.

**Skill merging:** Orc-managed skills (paths containing `.orc/` or `orc-` prefix) are replaced on phase switch. User skills are preserved. See `isOrcManagedSkill()`.

**Config:** See `internal/config/phase_settings.go` for types (`PhaseSettings`, `HookCommand`, `PhaseSettingsConfig`).

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
| review | 1 | `ReviewFindingsSchema` (needs_changes: boolean) |
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

The review phase supports multiple rounds via the loop system:

| Round | Template | Trigger | Detection |
|-------|----------|---------|-----------|
| 1 | `review.md` | Initial review | `LoopIteration == 0` or `LoopIteration == 1` |
| 2+ | `review_round2.md` | After implement loop | `LoopIteration > 1` |

**Round Detection:** The loop system sets `LoopIteration` in `ResolutionContext`. `GetEffectiveReviewRound()` returns this value (or falls back to `ReviewRound` for backward compatibility).

**Findings Flow:**
1. Round 1 blocks → Loop system captures output
2. `output_transform: format_findings` parses findings via `ParseReviewFindings()` and formats via `FormatFindingsForRound2()`
3. Loop condition triggers jump back to implement phase
4. On round 2+, `{{REVIEW_FINDINGS}}` variable is populated from prior phase output
5. Round 2 uses `review_round2.md` template (iteration-specific template selection)

## Agent & Model Resolution

Phase = Agent (WHO) + Prompt (WHAT). Resolution functions in `workflow_phase.go`:

| Function | Line | Resolution Order |
|----------|------|------------------|
| `resolveExecutorAgent()` | 658 | phase.AgentOverride → tmpl.AgentID → nil |
| `resolvePhaseModel()` | 693 | phase.ModelOverride → workflow.DefaultModel → agent.Model → config.Model → "opus" |
| `getEffectivePhaseClaudeConfig()` | 970 | Merge agent + phase config → nil if empty (`AllowAgentFolding` bool on `PhaseClaudeConfig`) |
| `getEffectivePhaseCodexConfig()` | — | Merge agent + phase Codex config (`PhaseCodexConfig`: sandbox_mode, approval_mode, reasoning_effort, instructions) |
| `shouldUseThinking()` | 724 | phase.ThinkingOverride → workflow.DefaultThinking (true only) → tmpl.ThinkingEnabled → phase defaults |

**Phase defaults:** spec/review → thinking=true, implement → thinking=false

### Provider Resolution (`provider.go`)

Determines which LLM provider executes each phase. Resolution in `resolvePhaseProvider()`:

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | `runProvider` | `--provider` CLI flag |
| 2 | `phase.ProviderOverride` | Per-workflow phase override |
| 3 | `workflow.DefaultProvider` | Workflow-level default |
| 4 | `tmpl.Provider` | Phase template default |
| 5 | `agent.Provider` | Executor agent's provider |
| 6 | `config.Provider` | Project config default |
| 7 | Model tuple extraction | Provider prefix from model string (e.g., `codex:gpt-5`) |
| 8 | `"claude"` | Ultimate fallback |

**Provider families:**

| Provider | Family | Adapter | Side Effects |
|----------|--------|---------|-------------|
| `claude` | Claude | `claudeAdapter` | `.claude/settings.json`, thinking env |
| `codex` | Codex | `codexAdapter` | `.codex/instruction.md` (if configured) |
| `ollama` | Codex | `codexAdapter` | Same as codex (routed via Codex CLI) |
| `lmstudio` | Codex | `codexAdapter` | Same as codex (routed via Codex CLI) |

**Key functions:**

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `resolvePhaseProvider()` | `provider.go:128` | Priority-chain provider resolution |
| `isCodexFamilyProvider()` | `provider.go:33` | Returns true for codex/ollama/lmstudio |
| `normalizeProvider()` | `provider.go:19` | Lowercases, maps aliases (anthropic→claude, openai→codex) |
| `ParseProviderModel()` | `provider.go:85` | Splits "provider:model" tuples (bare models default to "claude") |
| `validateProviderCapabilities()` | `provider.go:206` | Checks provider supports phase requirements (e.g., inline agents) |
| `validateProvider()` | `provider.go:200` | Rejects unknown providers (must be claude/codex/ollama/lmstudio) |
| `providerDefaultModel()` | `provider.go:185` | Returns default model per provider |
| `resolveCodexPath()` | `workflow_executor.go:180` | Codex path fallback chain |

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

**Note:** `TotalTokens` includes `InputTokens + OutputTokens + CacheReadTokens + CacheCreationTokens`. Raw `InputTokens` alone is misleading for Claude (cache tokens dominate). Cost estimation uses all four token types with provider-specific rates. Model name prefix matching resolves versioned model names (e.g., `gpt-5.3-codex` matches `gpt-5` rate).

### Token Rate Estimation (`cost_tracking.go`)

Provider-aware cost estimation with configurable rates:

| Function | Purpose |
|----------|---------|
| `EstimateTokenCostUSD()` | Estimates cost from token counts using provider rates |
| `EstimateTokenCostUSDWithRates()` | Same with custom rate table |
| `providerRatesForConfig()` | Builds rate table from config overrides + defaults |

Default rates are defined per provider/model pair. Config overrides via `providers.codex.rates` etc.

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

## Heartbeat and Stale Detection

### HeartbeatRunner (`heartbeat.go`)

Periodic heartbeat updates during long-running phases to prevent false orphan detection.

- **Interval:** `DefaultHeartbeatInterval = 2 * time.Minute`
- **Purpose:** Long implement phases can take hours; without heartbeats, task appears orphaned
- **Priority:** PID check takes precedence over heartbeat staleness (live PID = healthy task)

### IdleGuard (`idle_guard.go`)

Monitors heartbeat freshness during execution, warns when executor appears stale.

- **Check Interval:** `DefaultHeartbeatInterval` (2 minutes)
- **Stale Timeout:** `task.StaleHeartbeatThreshold` (15 minutes)
- **OnStale callback:** Logs warning (does not auto-release)

Wired into executor lifecycle via `startIdleGuard()` / `defer guard.Stop()`.

### Stale Detection (`task/stale_detection.go`)

| Function | Purpose |
|----------|---------|
| `IsClaimStale(task)` | Returns (stale bool, reason string) based on heartbeat age vs `StaleHeartbeatThreshold` |
| `FormatHeartbeatStatus(task)` | Human-readable status: "healthy (2m ago)" or "stale (23m ago)" |

**Note:** `CheckOrphanedProto` in `task/proto_helpers.go` still uses PID-based detection only. Hostname-aware detection for PostgreSQL team mode (where PID checks are meaningless across hosts) is not yet implemented.

## User Claim System

`WorkflowExecutor.Run()` calls `backend.ClaimTaskByUser()` before execution to prevent concurrent runs. Claims are user-based (via `UserIDFromContext(ctx)`), separate from PID-based `TryClaimTaskExecution`.

| Operation | Method | Behavior |
|-----------|--------|----------|
| Claim | `ClaimTaskByUser(taskID, userID)` | Atomic UPDATE; idempotent (re-claiming own task succeeds) |
| Force Claim | `ForceClaimTaskByUser(taskID, userID)` | Steals from current owner; records `stolen_from` in history |
| Release | `ReleaseUserClaim(taskID, userID)` | Releases if owned; via `defer` in executor |
| History | `GetUserClaimHistory(taskID)` | Append-only audit trail with steal tracking |

**SQLITE_BUSY handling:** Treated as "claim not acquired" (returns 0 rows, not error). See `db/task_claim.go:isSQLiteBusy()`.

**DB schema:** `claimed_by` and `claimed_at` columns on `tasks` table + `task_claim_history` table (`project_058.sql`).

## Testing

```bash
go test ./internal/executor/... -v
```

| Test File | Coverage |
|-----------|----------|
| `branch_test.go` | Branch resolution: `TestResolveBranchName` (9 cases), target branch resolution |
| `executor_resolution_test.go` | Agent/model/config resolution (`setupTestExecutor` helper) |
| `workflow_completion_test.go` | PR options merging: `TestResolvePROptions` (8 cases) |
| `create_pr_test.go` | PR creation/reuse: stale PR detection, idempotency, error paths, automation settings |
| `workflow_executor_test.go` | Core executor behavior |
| `phase_response_test.go` | Phase completion parsing |
| `gate_actions_test.go` | Gate output action dispatch: resolution (10 SC), approved/rejected dispatch, retry exhaustion, script handling, edge cases |
| `gate_output_pipeline_test.go` | Gate output variable pipeline: propagation, storage, retry context, rejection |
| `before_phase_trigger_test.go` | Before-phase gate/reaction, output variables, error resilience |
| `lifecycle_trigger_test.go` | Lifecycle trigger firing: completed, failed, gate blocking |
| `phase_settings_test.go` | Phase settings: apply/cleanup, skill merging, orc-managed detection |
| `condition_test.go` | Phase condition evaluation: operators, composites, variable resolution |
| `phase_loop_test.go` | Phase loop: JSON/legacy conditions, max enforcement, counter unification, events |
| `topo_sort_test.go` | Topological sort and execution levels: cycle detection, level grouping, sequence tiebreakers |
| `parallel_execution_test.go` | Parallel phase execution: diamond pattern, failure cancellation, variable safety |
| `dag_skip_integration_test.go` | DAG skip integration: skipped phases don't block dependents (SC-7 verification) |
| `executor_run_test.go` | Executor.Run claim-on-run integration: claim/release lifecycle, concurrent claim rejection |
| `heartbeat_test.go` | HeartbeatRunner: update lifecycle, interval timing |
| `history_test.go` | Run history tracking: start/complete/fail/interrupt |
| `idle_guard_test.go` | IdleGuard heartbeat loop and stale claim detection |
| `phase_registry_test.go` | PhaseTypeRegistry: registration, Get(), default types, nil panic |
| `knowledge_executor_test.go` | KnowledgePhaseExecutor: query routing, fallback skip/error, output vars, unavailable service |
| `knowledge_condition_test.go` | `knowledge.available` condition field resolution |
| `knowledge_wiring_integration_test.go` | Integration: registry wiring via `WithWorkflowKnowledgeService`, condition eval with live context |
| `script_executor_test.go` | ScriptPhaseExecutor: command execution, timeout, success_pattern, output vars, empty command, workdir |
| `api_executor_test.go` | APIPhaseExecutor: HTTP requests, status codes, headers, body, response limits, output vars, empty URL |
| `script_api_wiring_integration_test.go` | Integration: script/API executor registration, dispatch through phase loop, variable propagation |
| `phase_dispatch_test.go` | Phase type dispatch: non-LLM routing, type override precedence, error propagation, event publishing |
| `provider_test.go` | Provider resolution: `ParseProviderModel`, `normalizeProvider`, `isCodexFamilyProvider`, priority chain (18 cases) |
| `provider_dispatch_test.go` | Provider dispatch integration: codex route via session ID signal, priority chain propagation, ollama routing (7 cases) |

**Mock injection:** Use `WithWorkflowTurnExecutor(mock)`, `WithFinalizeTurnExecutor(mock)`, `WithResolverTurnExecutor(mock)`, `WithWorkflowTriggerRunner(mock)`, `WithPhaseTypeExecutor(name, mock)` (for script/api/knowledge/custom), `WithWorkflowKnowledgeService(mock)`, `hostingProvider` field for PR tests

## Common Gotchas

| Issue | Solution |
|-------|----------|
| Raw InputTokens misleading | `TotalTokens` includes cache; use it for display |
| Ultrathink in system prompt | Must be user message |
| User agents unavailable | Need `WithSettingSources` with "user" |
| Worktree cleanup by path | Use `CleanupWorktreeAtPath(e.worktreePath)` |
| Spec not found in templates | Use `WithSpecFromDatabase()` |
| Invalid session ID errors | Only pass custom session IDs when `Persistence: true` |
| Validation can't see files | Create clients dynamically with correct workdir |
| Provider not dispatching to codex | Check `resolvePhaseProvider()` priority chain; empty string defaults to "claude" |
| Declared field has no effect | Every struct/config field must flow to a runtime consumer — see `internal/AGENTS.md` "Wiring Contracts" |

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

### Work Preservation

Both `failRun()` and `interruptRun()` commit work-in-progress before updating status. This preserves uncommitted changes so they can be recovered on retry.

| Event | Commits WIP? | Preserves Worktree? |
|-------|--------------|---------------------|
| Ctrl+C / SIGUSR1 (pause) | ✅ via `commitWIPOnInterrupt()` | ✅ |
| Task failure (`failRun`) | ✅ via `commitWIPOnInterrupt()` | N/A |
| Sync-on-start failure | N/A | ✅ if work exists via `detectExistingWork()` |
| Completion | ✅ via `autoCommitBeforeCompletion()` | Config-dependent |

**`detectExistingWork()`** checks three signals before allowing cleanup:
1. Uncommitted changes (staged, unstaged, or untracked)
2. Commits ahead of target branch
3. Phase execution state (any `StartedAt` timestamp)

If any signal is found, worktree/branch is preserved. Fail-safe: detection errors preserve (can't confirm no work = don't delete).

### Anti-Patterns

| Bad | Why |
|-----|-----|
| `if err != nil { return err }` | Task still shows "running" |
| Skip `t.Execution.Error = err.Error()` | User can't see what went wrong |
| Forget to call `backend.SaveTask(t)` | Changes not persisted |
