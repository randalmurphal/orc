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

## Summary

The E2E tests for the Finalize workflow are already fully implemented in `web/e2e/finalize.spec.ts`. The tests cover all 10 required scenarios:

### Finalize UI (5 tests)
1. **should show finalize button on completed tasks in Done column** - Verifies finalize button appears on completed tasks
2. **should open FinalizeModal when finalize button clicked** - Tests modal opening behavior
3. **should display explanation and start button in modal** - Verifies modal content includes info text and Start Finalize button
4. **should show progress bar with step labels during finalization** - Tests WebSocket-driven progress updates
5. **should update task card to show finalizing state (pulsing border)** - Verifies card visual state during finalization

### Finalize Results (5 tests)
1. **should show success state with merge info** - Tests successful finalize completion display
2. **should display merged commit SHA** - Verifies commit SHA is shown (abbreviated to 7 chars)
3. **should show target branch name** - Verifies target branch display in results
4. **should handle finalize failure with retry option** - Tests failure state and retry button
5. **should update task card to finished state with green merge badge** - Verifies finished task card appearance

All tests pass successfully (10 passed in 12.6s) using the WebSocket event injection pattern documented in the project knowledge.

<phase_complete>true</phase_complete>

---
Tokens: 1005037 input, 2950 output, 50029 cache_creation, 954916 cache_read
Complete: true
Blocked: false
