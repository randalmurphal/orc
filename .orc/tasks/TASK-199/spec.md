# Specification: Bug: Blocker check doesn't recognize 'finished' status as complete

## Problem Statement

The `GetIncompleteBlockers()` function in `internal/task/task.go` only checks for `StatusCompleted` when determining if a blocking task is done, ignoring `StatusFinished`. This causes tasks with `finished` blockers to incorrectly appear as blocked in the UI and API responses, even though `finished` tasks (merged to main) are fully complete.

## Success Criteria

- [ ] `GetIncompleteBlockers()` returns empty list when all blockers have status `finished`
- [ ] `GetIncompleteBlockers()` correctly handles mixed `completed` and `finished` blockers
- [ ] Existing behavior preserved: running, planned, failed tasks still count as incomplete blockers
- [ ] Unit test added: `TestGetIncompleteBlockers_FinishedBlocker` passes
- [ ] All existing task dependency tests pass (`go test ./internal/task/...`)

## Testing Requirements

- [ ] Unit test: New test case in `TestGetIncompleteBlockers` for `StatusFinished` blocker (should return 0 blockers)
- [ ] Unit test: Test case for mixed `completed` and `finished` blockers
- [ ] Regression: Verify `TestHasUnmetDependencies` and `TestGetUnmetDependencies` still pass (they already test `finished` correctly)
- [ ] Integration: Run `make test` to verify no regressions across the codebase

## Scope

### In Scope
- Fix `GetIncompleteBlockers()` in `internal/task/task.go` to use the existing `isDone()` helper
- Add test coverage for `finished` status in `GetIncompleteBlockers` tests

### Out of Scope
- Modifying initiative dependency checking (initiatives don't have a `finished` status)
- Changing the `isDone()` helper function itself (already correct)
- UI/API changes (the fix is in the data layer)

## Technical Approach

The fix is minimal: change line 992 in `internal/task/task.go` from:
```go
if blocker.Status != StatusCompleted {
```
to:
```go
if !isDone(blocker.Status) {
```

This aligns `GetIncompleteBlockers()` with `HasUnmetDependencies()` and `GetUnmetDependencies()` which already use `isDone()` correctly.

### Files to Modify

- `internal/task/task.go:992`: Change direct status check to use `isDone()` helper
- `internal/task/task_test.go`: Add `StatusFinished` to `TestGetIncompleteBlockers` taskMap and add test case

## Bug Analysis

### Reproduction Steps
1. Create TASK-001 and TASK-002 where TASK-002 is blocked by TASK-001
2. Complete TASK-001 through the full workflow until status is `finished` (PR merged)
3. Call `task.GetIncompleteBlockers(taskMap)` on TASK-002
4. Observe TASK-001 is incorrectly returned as an incomplete blocker

### Current Behavior
`GetIncompleteBlockers()` returns TASK-001 as a blocker because it checks `blocker.Status != StatusCompleted`, and `finished` ≠ `completed`.

### Expected Behavior
`GetIncompleteBlockers()` should return an empty list because `finished` indicates the task is done and merged.

### Root Cause
Line 992 of `internal/task/task.go` directly compares against `StatusCompleted` instead of using the `isDone()` helper function (defined at line 939) which correctly checks for both `completed` and `finished` statuses.

The helper exists and is already used by `HasUnmetDependencies()` and `GetUnmetDependencies()`, but `GetIncompleteBlockers()` was not updated to use it when the `finished` status was added.

### Verification
After the fix:
1. Run `go test ./internal/task/... -v -run TestGetIncompleteBlockers` - all tests pass including new `finished` test
2. Run `make test` - full test suite passes
3. Manually verify: TASK-002 blocked by finished TASK-001 should show no incomplete blockers
