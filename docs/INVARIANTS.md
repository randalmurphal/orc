# Orc Invariants

Hard rules that must never be violated. Breaking these causes bugs that waste hours.

## Canonical: Database & State

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Specs Live in DB** | Specs stored in `specs` table only, never as files | File-based specs in worktrees cause merge conflicts | Tasks with stale specs, merge failures |
| **Artifacts in DB** | Phase artifacts (tdd_write, breakdown, research) stored via `SaveArtifact()` | Consistent storage pattern, survives worktree cleanup | Missing context in later phases |
| **Per-Phase Sessions** | Each phase gets fresh Claude session, not resumed | Shared sessions contaminate context | TDD context leaks to implement, wrong decisions |
| **State Matches Task** | On failure: update BOTH `task.Status` AND `state.Error` | Out-of-sync causes orphaned tasks | Tasks stuck in "running" forever, invisible errors |

## Canonical: Error Handling

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **NO Silent Failures** | Every error must propagate or be logged; no silent swallowing | Silent continue hides bugs | Tasks appear complete but aren't |
| **NO Fallbacks** | If expected field missing → ERROR, not fallback to alternative | Multiple code paths hide bugs | Inconsistent behavior |
| **Fail on Parse Error** | JSON parse failure → ERROR, not default value | Silent defaults cause wrong state | Invisible corruption |
| **Error Both Places** | Use `failTask()`, `interruptTask()` helpers which update both task+state | Manual updates miss one side | Orphaned tasks |

## Canonical: LLM Calls

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **ONE Schema Pattern** | Use `llmutil.ExecuteWithSchema[T]()` for ALL schema-constrained calls | Multiple patterns caused silent failures | Parse errors, lost data |
| **Model From Config** | Never hardcode model in CompletionRequest | Model is set at client creation | Wrong model, wrong cost |
| **Ultrathink in User Message** | `ultrathink\n\n` must prefix user message, not system | System prompt position doesn't work | No extended thinking |
| **Schema = Pure JSON** | With `--json-schema`, output is ONLY JSON | No text/JSON mixing | Parse failures |

## Canonical: Git & Worktrees

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Cleanup by Path** | Use `CleanupWorktreeAtPath(path)` not `CleanupWorktree(taskID)` | Initiative-prefixed worktrees have different paths | Orphaned worktrees, disk full |
| **No os.Chdir in Tests** | Use explicit path parameters, never `os.Chdir()` | Process-wide, not goroutine-safe | Flaky tests, wrong directory |
| **Worktree Isolation** | Each task runs in its own worktree | Main repo must stay clean | Conflicts between parallel tasks |

## Canonical: API Responses

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Missing Data ≠ 500** | When task lacks optional data (branch, worktree), return empty/default response (200), not 500 | Missing optional data is not a server error | UI shows error for valid task with no diff (TASK-528) |
| **Empty Over Null** | Return empty arrays `[]` not `null` in JSON | Null requires special handling in clients | `TypeError: cannot iterate null` |

## Canonical: Testing

| Invariant | Rule | Why | Consequence |
|-----------|------|-----|-------------|
| **Mock TurnExecutor** | Tests inject mock via `WithStandardTurnExecutor(mock)` | Avoid real Claude API calls | Slow tests, API costs, flaky |
| **Dynamic Validation Client** | Validation clients created per-call with correct workdir | Pre-created clients have wrong paths | Can't find worktree files |
| **In-Memory Backend for Tests** | Use `storage.NewTestBackend(t)` for fast tests | No disk I/O needed | Slow tests, temp file leaks |

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

// ❌ 500 for missing optional data (API)
if t.Branch == "" {
    s.jsonError(w, "no branch", http.StatusInternalServerError)  // BUG: missing data ≠ server error
}
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

// ✅ Empty response for missing optional data (API)
if t.Branch == "" {
    s.jsonResponse(w, emptyDiffResult())  // 200 with empty data, not 500
    return
}
```

## Verification

Run `make test` to verify invariants aren't violated. Key test files:
- `internal/executor/executor_test.go` - Error handling paths
- `internal/storage/database_backend_test.go` - DB operations
- `internal/db/constitution_test.go` - Constitution CRUD

## Adding New Invariants

When you discover a bug caused by violating an implicit rule:
1. Add the invariant to this file
2. Add a test that would catch the violation
3. Add the anti-pattern to the "NEVER DO" section
