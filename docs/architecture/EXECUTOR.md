# Executor Model

**Purpose**: Unified workflow execution through `WorkflowExecutor`.

> **Code Reference**: See `internal/executor/CLAUDE.md` for implementation details.

---

## Module Structure

The executor package uses a unified execution model:

| Module | File | Responsibility |
|--------|------|----------------|
| **WorkflowExecutor** | `workflow_executor.go` | THE executor - all execution goes through `Run()` |
| **Phase Execution** | `workflow_phase.go:executePhase()` | Single phase execution with ClaudeExecutor |
| **Claude Executor** | `claude_executor.go` | `TurnExecutor` interface, ClaudeCLI wrapper with `--json-schema` |
| **Phase Response** | `phase_response.go` | JSON schemas for phase completion |
| **Finalize Executor** | `finalize.go` | Explicit branch sync, conflict resolution, risk assessment |
| **Publishing** | `publish.go` | Nil-safe event publishing |
| **Variable Resolution** | `../variable/resolver.go` | Template variable substitution |
| **Retry** | `retry.go` | Cross-phase retry context |
| **Worktree** | `worktree.go` | Git worktree isolation |

---

## Execution Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    WorkflowExecutor.Run()                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐    ┌─────────────┐    ┌─────────────┐    │
│  │ buildResolution │───►│  executePhase│───►│   Output    │    │
│  │    Context()    │    │     ()       │    │   Parser    │    │
│  └─────────────────┘    └─────────────┘    └─────────────┘    │
│        │                       │                   │            │
│        ▼                       ▼                   ▼            │
│  ┌─────────────────┐    ┌─────────────┐    ┌─────────────┐    │
│  │ variable.Resolve│    │ClaudeExecutor│    │ Completion  │    │
│  │     All()       │    │ ExecuteTurn()│    │  Detector   │    │
│  └─────────────────┘    └─────────────┘    └──────┬──────┘    │
│                                                   │             │
│                            ┌──────────────────────┤             │
│                            ▼                      ▼             │
│                   ┌─────────────┐        ┌─────────────┐        │
│                   │  CONTINUE   │        │  COMPLETE   │        │
│                   │ (loop/retry)│        │(save artifact)│      │
│                   └─────────────┘        └─────────────┘        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key Integration Points:**
- `buildResolutionContext()` creates variable context with task, initiative, project detection
- `enrichContextForPhase()` adds phase-specific context (review findings, test results)
- `variable.ResolveAll()` resolves all template variables
- `variable.RenderTemplate()` applies variables to prompt template

---

## Phase Execution Flow

```go
// Simplified flow - see workflow_executor.go for full implementation
func (we *WorkflowExecutor) Run(ctx, workflowID, opts) {
    // 1. Setup context (worktree, task status, heartbeat)
    we.setupForContext(ctx, opts)

    // 2. Load workflow phases from database
    phases := we.loadWorkflow(workflowID)

    // 3. Build resolution context
    rctx := we.buildResolutionContext(opts, task, workflow)

    // 4. Execute each phase
    for _, phase := range phases {
        // Enrich context for this phase
        we.enrichContextForPhase(rctx, phase.ID, task, state)

        // Resolve all variables
        vars := we.resolver.ResolveAll(ctx, defs, rctx)

        // Evaluate gate (if configured)
        if !we.evaluatePhaseGate(ctx, phase, vars) {
            continue // Skip or block
        }

        // Execute phase
        result := we.executePhase(ctx, phase, vars, rctx)

        // Save artifact (if phase produces one)
        we.saveArtifact(task.ID, phase.ID, result.Artifact)
    }

    // 5. Complete run
    we.completeRun(run, task)
}
```

---

## Completion Detection

### JSON Schema Pattern

Claude outputs completion signals via `--json-schema`:

```json
{"status": "complete", "summary": "Implemented feature X with tests"}
{"status": "blocked", "reason": "Need clarification on Y"}
{"status": "continue", "reason": "Still implementing"}
```

### Schema Selection

`GetSchemaForPhaseWithRound(phaseID, round)` returns appropriate schema:

| Phase | Round | Schema | Fields |
|-------|-------|--------|--------|
| spec, design, research, docs | - | `PhaseCompletionWithArtifactSchema` | status, summary, artifact |
| review | 1 | `ReviewFindingsSchema` | findings array |
| review | 2 | `ReviewDecisionSchema` | status (pass/fail/needs_user_input) |
| qa | - | `QAResultSchema` | status, issues |
| other | - | `PhaseCompletionSchema` | status, summary |

### Parsing Functions

| Function | Use Case |
|----------|----------|
| `ParsePhaseSpecificResponse()` | Route to correct parser by phase type |
| `CheckPhaseCompletionJSON()` | Parse standard completion (complete/blocked/continue) |

---

## Additional Completion Criteria

| Criterion | Check Method |
|-----------|--------------|
| `all_tests_pass` | Run `go test ./...`, check exit code |
| `no_lint_errors` | Run linter, check exit code |
| `files_exist` | Check filesystem |
| `coverage_above: N` | Parse coverage report, verify >= N% |
| `claude_confirms` | Claude outputs `{"status": "complete"}` |
| `spec_complete` | Spec artifact exists and passes validation |
| `review_approved` | Review completed with no major findings |

---

## Cross-Phase Retry

When phases fail, they can retry from an earlier phase:

| Failed Phase | Retries From | Reason |
|--------------|--------------|--------|
| design | spec | Design issues stem from incomplete spec |
| review | implement | Review findings need code changes |
| test, test_unit, test_e2e | implement | Test failures need code fixes |
| validate | implement | Validation issues need code changes |

### Retry Context

When retrying, `{{RETRY_CONTEXT}}` template variable contains:
- Which phase failed and why
- The failure output (test errors, validation messages)
- Attempt number

```yaml
# .orc/config.yaml
executor:
  max_retries: 5  # Max retry attempts per phase (default: 5)
```

---

## Finalize Executor

Specialized executor for explicit `orc finalize TASK-XXX` command:

### Execution Steps

```
1. Fetch target branch    → Get latest changes from remote
2. Check divergence       → Count commits ahead/behind
3. Sync with target       → Merge or rebase (per config)
4. Resolve conflicts      → AI-assisted if conflicts detected
5. Run tests              → Verify tests pass after sync
6. Fix tests (if needed)  → AI attempts to fix failures
7. Risk assessment        → Classify merge risk level
8. Create finalize commit → Document finalization
```

### Sync Strategies

| Strategy | Behavior | Use Case |
|----------|----------|----------|
| `merge` (default) | Merge target into task branch | Preserves commit history |
| `rebase` | Rebase task branch onto target | Linear history, cleaner |

### Risk Assessment

| Metric | Low | Medium | High | Critical |
|--------|-----|--------|------|----------|
| Files changed | 1-5 | 6-15 | 16-30 | >30 |
| Lines changed | <100 | 100-500 | 500-1000 | >1000 |
| Conflicts | 0 | 1-3 | 4-10 | >10 |

---

## CI Wait and Auto-Merge

After finalize, orc waits for CI and merges via GitHub REST API:

```
1. Push finalize changes     → Sync commits, conflict resolutions
2. Poll CI checks            → Wait for all checks to pass
3. Merge PR via API          → PUT /repos/{owner}/{repo}/pulls/{number}/merge
4. Delete branch via API     → DELETE /repos/{owner}/{repo}/git/refs/heads/{branch}
5. Update task state         → Record merge commit SHA
```

### CI Status Evaluation

| Status | Action |
|--------|--------|
| All checks passed | Proceed to merge |
| Checks pending | Continue polling |
| Checks failed | Abort, PR remains open |
| No checks configured | Treat as passed |
| Timeout reached | Abort, PR remains open |

### Merge Retry Logic

When parallel tasks target the same branch:

| Attempt | Backoff | Action |
|---------|---------|--------|
| 1 | 0s | Initial merge attempt |
| 2 | 2s | Rebase onto target, retry |
| 3 | 4s | Rebase onto target, retry |
| 4 | 8s | Final rebase and retry |

---

## Activity Tracking

| State | Description |
|-------|-------------|
| `idle` | No activity |
| `waiting_api` | Waiting for Claude API response |
| `streaming` | Receiving streaming response |
| `running_tool` | Claude executing a tool |
| `processing` | Processing response |

### Timeouts

| Timeout | Default | Purpose |
|---------|---------|---------|
| `turn_max` | 10m | Max time for single API turn |
| `idle_timeout` | 2m | Warn if no streaming activity |
| `phase_max` | 30m | Max time for entire phase |

---

## Configuration

```yaml
# .orc/config.yaml
executor:
  max_retries: 5

completion:
  finalize:
    enabled: true
    sync:
      strategy: merge
    conflict_resolution:
      enabled: true
  ci:
    wait_for_ci: true
    ci_timeout: 10m
    merge_on_ci_pass: true
    merge_method: squash
  delete_branch: true

timeouts:
  phase_max: 30m
  turn_max: 10m
  idle_warning: 5m
  heartbeat_interval: 30s
```

See `docs/specs/CONFIG_HIERARCHY.md` for full configuration options.
