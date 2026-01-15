# Specification: Convert Worker.run() from recursion to iteration

## Problem Statement
The `Worker.run()` method in `internal/orchestrator/worker.go` uses tail recursion (line 230) to process multiple phases sequentially. This should be converted to an iterative loop to eliminate stack growth risk and improve debuggability.

## Success Criteria
- [ ] `Worker.run()` uses a `for` loop instead of recursive self-call
- [ ] All existing behavior preserved: phase execution, completion detection, error handling, pausing, status transitions
- [ ] Defer block still executes exactly once at function exit (cleanup and pool removal)
- [ ] Event publishing for phase start/complete still occurs correctly
- [ ] State and plan saving after each phase completion still works
- [ ] Context cancellation (pause) still interrupts execution between phases
- [ ] All existing tests pass without modification
- [ ] No stack growth regardless of number of phases

## Testing Requirements
- [ ] Unit test: All existing `worker_test.go` tests pass (8 tests)
- [ ] Unit test: New test `TestWorkerRunsMultiplePhasesIteratively` verifies multi-phase execution completes without recursion
- [ ] Unit test: New test `TestWorkerExitsLoopOnContextCancel` verifies loop exits cleanly when context is cancelled
- [ ] Integration: `make test` passes (full backend test suite)

## Scope
### In Scope
- Converting recursive call at line 230 to iterative loop
- Preserving all existing behavior exactly
- Adding tests for multi-phase iteration

### Out of Scope
- Changing phase execution logic
- Modifying event publishing
- Changing how prompts are loaded
- Modifying WorkerPool behavior
- Refactoring other parts of worker.go

## Technical Approach
Replace the recursive call with a `for` loop that continues until either:
1. No more phases remain (`currentPhase == nil`)
2. Context is cancelled (pause requested)
3. An error occurs

The loop will:
1. Move `currentPhase := pln.CurrentPhase()` check to loop condition
2. Move phase execution logic into loop body
3. Replace recursive call with `continue` to next iteration
4. Use `break` or early returns for exit conditions

### Files to Modify
- `internal/orchestrator/worker.go`: Convert `run()` from recursion to iteration
- `internal/orchestrator/worker_test.go`: Add tests for iterative multi-phase execution

## Refactor Analysis

### Before Pattern (Current)
```go
func (w *Worker) run(pool *WorkerPool, t *task.Task, pln *plan.Plan, st *state.State) {
    defer func() { /* cleanup */ }()

    currentPhase := pln.CurrentPhase()
    if currentPhase == nil { return }

    // ... execute phase ...

    if !mgr.Exists() {
        // Phase completed
        nextPhase := pln.CurrentPhase()
        if nextPhase == nil {
            // Task complete
        } else {
            w.run(pool, t, pln, st)  // RECURSIVE CALL
        }
    }
}
```

### After Pattern (Target)
```go
func (w *Worker) run(pool *WorkerPool, t *task.Task, pln *plan.Plan, st *state.State) {
    defer func() { /* cleanup */ }()

    for {
        currentPhase := pln.CurrentPhase()
        if currentPhase == nil {
            w.setStatus(WorkerStatusComplete)
            return
        }

        // ... execute phase ...

        if !mgr.Exists() {
            // Phase completed, loop continues to next phase
            continue
        }
        // Phase did not complete (ralph state still exists)
        return
    }
}
```

### Risk Assessment
**Low risk** - This is a straightforward tail recursion to iteration conversion:
- The recursive call is in tail position (nothing happens after it)
- All state is already mutated before the recursive call (plan/state saved)
- Defer block behavior unchanged (still runs once at function exit)
- No callers depend on stack behavior

**Testing mitigation**: Existing tests cover cleanup, status transitions, and pool management. New tests will verify multi-phase iteration.
