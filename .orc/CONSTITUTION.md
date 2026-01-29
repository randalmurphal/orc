# Orc Project Constitution

These rules guide all AI-assisted task execution on the orc codebase.
Invariants CANNOT be ignored or overridden. Breaking these causes bugs that waste hours.

## Priority Hierarchy

When rules conflict, higher priority wins:

1. Safety & correctness (invariants)
2. Security (invariants)
3. Existing patterns (defaults)
4. Performance (defaults)
5. Style (defaults)

## Invariants (MUST NOT violate)

### Database & State

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Specs Live in DB** | Specs stored in `specs` table only, never as files | File-based specs in worktrees cause merge conflicts | Tasks with stale specs, merge failures |
| **Artifacts in DB** | Phase artifacts (tdd_write, breakdown, research) stored via `SaveArtifact()` | Consistent storage pattern, survives worktree cleanup | Missing context in later phases |
| **Per-Phase Sessions** | Each phase gets fresh Claude session, not resumed | Shared sessions contaminate context | TDD context leaks to implement, wrong decisions |
| **State Matches Task** | On failure: update BOTH `task.Status` AND `state.Error` | Out-of-sync causes orphaned tasks | Tasks stuck in "running" forever, invisible errors |

### Error Handling

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **NO Silent Failures** | Every error must propagate or be logged; no silent swallowing | Silent continue hides bugs | Tasks appear complete but aren't |
| **NO Fallbacks** | If expected field missing → ERROR, not fallback to alternative | Multiple code paths hide bugs | Inconsistent behavior |
| **Fail on Parse Error** | JSON parse failure → ERROR, not default value | Silent defaults cause wrong state | Invisible corruption |
| **Error Both Places** | Use `failTask()`, `interruptTask()` helpers which update both task+state | Manual updates miss one side | Orphaned tasks |

### LLM Calls

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **ONE Schema Pattern** | Use `llmutil.ExecuteWithSchema[T]()` for ALL schema-constrained calls | Multiple patterns caused silent failures | Parse errors, lost data |
| **Schema Matches Struct** | JSON schema `required` fields must match Go struct fields used for validation | Missing `status` in review schema let blocked reviews pass as complete (TASK-630) | Silent success on blocked phases |
| **Model From Config** | Never hardcode model in CompletionRequest | Model is set at client creation | Wrong model, wrong cost |
| **Ultrathink in User Message** | `ultrathink\n\n` must prefix user message, not system | System prompt position doesn't work | No extended thinking |
| **Schema = Pure JSON** | With `--json-schema`, output is ONLY JSON | No text/JSON mixing | Parse failures |

### Event Systems

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Wire All Event Consumers** | EventServer must call `SetWebSocketHub()` at startup to forward events to WebSocket clients | Separate event systems must be explicitly connected | UI never receives real-time updates |
| **SessionBroadcaster Wiring** | WorkflowExecutor must receive SessionBroadcaster via `WithWorkflowSessionBroadcaster()` in API server | Session metrics need event publishing to reach WebSocket clients | Header stats always show 0, no real-time session updates |

### RPC Actions

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Status + Side Effect** | RPC methods that change status MUST also trigger the actual side effect (e.g., RunTask must spawn executor, not just set status to running) | Status-only updates create inconsistent state | UI shows "running" but nothing executes |
| **Rollback on Failure** | If side effect fails after status change, revert status to original | Partial updates cause orphaned state | Task stuck in "running" with no executor |
| **Reload After Write** | API handlers returning modified objects MUST reload from database after save | Save may modify timestamps, normalize data | Stale data returned to clients |

### Git & Worktrees

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Cleanup by Path** | Use `CleanupWorktreeAtPath(path)` not `CleanupWorktree(taskID)` | Initiative-prefixed worktrees have different paths | Orphaned worktrees, disk full |
| **No os.Chdir in Tests** | Use explicit path parameters, never `os.Chdir()` | Process-wide, not goroutine-safe | Flaky tests, wrong directory |
| **Worktree Isolation** | Each task runs in its own worktree | Main repo must stay clean | Conflicts between parallel tasks |
| **Sync Failure Cleanup** | Sync-on-start failures MUST cleanup worktree+branch unconditionally (TASK-499) | No phases ran, no work to preserve; zombies block retry | Zombie worktrees accumulate, retry fails with "branch exists" |

### Testing

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Mock TurnExecutor** | Tests inject mock via `WithStandardTurnExecutor(mock)` | Avoid real Claude API calls | Slow tests, API costs, flaky |
| **Dynamic Validation Client** | Validation clients created per-call with correct workdir | Pre-created clients have wrong paths | Can't find worktree files |
| **In-Memory Backend for Tests** | Use `storage.NewTestBackend(t)` for fast tests | No disk I/O needed | Slow tests, temp file leaks |

## Defaults (SHOULD follow)

**These are defaults. Can deviate with documented justification.**

| ID | Default | When to Deviate |
|----|---------|-----------------|
| DEF-1 | Functions < 50 lines | Complex state machines, switch statements |
| DEF-2 | One file = one responsibility | Test helpers, related utilities |
| DEF-3 | Follow existing patterns | When spec explicitly requests new pattern |
| DEF-4 | Error messages include context | Simple wrappers that add no value |
| DEF-5 | Use table-driven tests | Single edge case being tested |

## Anti-Patterns (NEVER DO)

```go
// ❌ Silent error swallowing
if err != nil {
    return defaultValue  // BUG: caller doesn't know something failed
}

// ❌ Fallback on missing field
if resp.StructuredOutput == "" {
    return resp.Result  // BUG: schema was required, empty is an error
}

// ❌ Manual task status update (misses state)
t.Status = task.StatusFailed
backend.SaveTask(t)  // BUG: state.Error not set!

// ❌ Hardcoded model
client.Complete(ctx, CompletionRequest{
    Model: "claude-opus-4-5",  // BUG: use config
})

// ❌ File-based spec in worktree
os.WriteFile(filepath.Join(worktree, ".orc", "spec.md"), content, 0644)  // BUG: use database

// ❌ Return stale object after save
task.BlockedBy = append(task.BlockedBy, blockerID)
backend.UpdateTask(ctx, task)
respondJSON(w, http.StatusOK, task)  // BUG: timestamps/computed fields are stale
```

## Correct Patterns

```go
// ✅ Error propagation
if err != nil {
    return fmt.Errorf("load task %s: %w", id, err)
}

// ✅ Require expected field
schemaResult, err := llmutil.ExecuteWithSchema[T](ctx, client, prompt, schema)
if err != nil {
    return nil, fmt.Errorf("schema call failed: %w", err)  // Error includes parse failure
}

// ✅ Use helper for both task+state
e.failTask(t, phase, s, err)  // Updates task.Status, state.Error, state.Status, saves both

// ✅ Model from workflow (phase template > config default)
model := resolvePhaseModel(tmpl, phase)  // workflow phase override > template default > config.Model
client := claude.NewClient(claude.WithModel(model))

// ✅ Spec in database
backend.SaveSpec(taskID, content, "spec")
content, _ := backend.LoadSpec(taskID)

// ✅ Reload after write for API response
task.BlockedBy = append(task.BlockedBy, blockerID)
backend.UpdateTask(ctx, task)
updated, _ := backend.GetTask(ctx, task.ID)  // Reload to get fresh state
respondJSON(w, http.StatusOK, updated)
```

## Verification

Run `make test` to verify invariants aren't violated. Key test files:
- `internal/executor/executor_test.go` - Error handling paths
- `internal/storage/database_backend_test.go` - DB operations

## Adding New Invariants

When you discover a bug caused by violating an implicit rule:
1. Add the invariant to this file
2. Add a test that would catch the violation
3. Add the anti-pattern to the "NEVER DO" section
