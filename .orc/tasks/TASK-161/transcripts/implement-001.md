# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Phase 0: Expand E2E tests for Finalize workflow (10 tests)

**Description**: ## Purpose
E2E tests for the finalize phase workflow - branch sync, CI verification, and merge.

## Test Coverage (10 tests)

### Finalize UI (5 tests)
- should show finalize button on completed tasks in Done column
- should open FinalizeModal when finalize button clicked
- should display explanation and start button in modal
- should show progress bar with step labels during finalization
- should update task card to show finalizing state (pulsing border)

### Finalize Results (5 tests)
- should show success state with merge info
- should display merged commit SHA
- should show target branch name
- should handle finalize failure with retry option
- should update task card to finished state with green merge badge

## Implementation Notes
- Finalize involves real git operations - may need test branch setup
- WebSocket events: finalize progress updates in real-time
- Task states: completed -> finalizing -> finished

## Success Criteria
- All 10 tests pass
- Tests cover both success and failure paths



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

Excellent! The implementation is complete. Let me provide a summary:

## Summary

Successfully implemented **10 E2E tests** for the Finalize workflow in `web/e2e/finalize.spec.ts`:

### Finalize UI (5 tests)
1. ✅ `should show finalize button on completed tasks in Done column`
2. ✅ `should open FinalizeModal when finalize button clicked`
3. ✅ `should display explanation and start button in modal`
4. ✅ `should show progress bar with step labels during finalization`
5. ✅ `should update task card to show finalizing state (pulsing border)`

### Finalize Results (5 tests)
6. ✅ `should show success state with merge info`
7. ✅ `should display merged commit SHA`
8. ✅ `should show target branch name`
9. ✅ `should handle finalize failure with retry option`
10. ✅ `should update task card to finished state with green merge badge`

### Key Implementation Details
- Uses WebSocket route interception (`page.routeWebSocket`) to inject `finalize` events
- Tests both success and failure paths
- Follows existing test patterns from `websocket.spec.ts`
- Uses framework-agnostic selectors (roles, semantic text, structural classes)
- All 10 tests pass consistently (ran multiple times)

<phase_complete>true</phase_complete>

---
Tokens: 1992344 input, 14976 output, 84939 cache_creation, 1897676 cache_read
Complete: true
Blocked: false
