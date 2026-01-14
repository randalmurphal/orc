# docs - Iteration 1

## Prompt

Update documentation for:

**Task**: Phase 0: Expand E2E tests for Board interactions (18 tests)

**Description**: ## Purpose
Comprehensive E2E tests for Board page that define BEHAVIOR (framework-agnostic) so they work on both Svelte and React during migration.

## Test Coverage (18 tests)

### Board Rendering (4 tests)
- should display board page with all 6 columns (Queued, Spec, Implement, Test, Review, Done)
- should show correct column headers and task counts
- should render task cards in correct columns based on status/phase
- should show task count in header

### View Mode Toggle (5 tests)
- should default to flat view mode
- should switch to swimlane view when selected
- should persist view mode in localStorage
- should disable swimlane toggle when initiative filter active
- should show initiative banner when filtering

### Drag-Drop (5 tests) - CRITICAL
- should move task between columns (triggers status change API)
- should reorder tasks within column (priority change)
- should show visual feedback during drag
- should update task status after drop completes
- should handle drop cancellation (escape key)

### Swimlane View (4 tests)
- should group tasks by initiative in swimlane view
- should collapse/expand swimlanes
- should persist collapsed state in localStorage
- should show Unassigned swimlane for tasks without initiative

## Implementation Notes
- Use behavior-based selectors: page.getByRole(), page.getByTestId()
- Avoid implementation-specific selectors like .svelte-xyz
- All tests must pass on BOTH Svelte and React implementations
- See screenshots in .orc/initiatives/INIT-005/ for reference

## Success Criteria
- All 18 tests pass on current Svelte implementation
- Tests are deterministic (no flaky tests)
- Tests use framework-agnostic selectors

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

All 4 test groups are there as documented:
- Board Rendering (4 tests)
- View Mode Toggle (5 tests)
- Drag-Drop (5 tests)
- Swimlane View (4 tests)

The documentation is complete and accurate. Here's what was updated:

**Summary of documentation updates:**

1. **`web/e2e/board.spec.ts`** - Added comprehensive JSDoc header documenting:
   - Purpose (framework-agnostic tests for React migration)
   - Test coverage breakdown (18 tests across 4 groups)
   - Selector strategy priority order
   - Reference to web/CLAUDE.md

2. **`web/CLAUDE.md`** - Updated Testing section with:
   - Complete table of all E2E test files with coverage descriptions
   - New "Framework-Agnostic E2E Testing" section with selector priority table
   - Helper function documentation from board.spec.ts
   - Flakiness prevention best practices

The main `CLAUDE.md` was not modified since it already appropriately references `web/CLAUDE.md` for frontend-specific details and has a concise testing section pointing to `make e2e`.

<phase_complete>true</phase_complete>

---
Tokens: 902579 input, 3515 output, 51184 cache_creation, 851014 cache_read
Complete: true
Blocked: false
