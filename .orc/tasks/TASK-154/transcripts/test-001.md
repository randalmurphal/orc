# test - Iteration 1

## Prompt

Comprehensive testing for:

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

## Unit Tests
1. Run: `go test ./... -v -race -cover`
2. Verify coverage > 80%
3. Fix any failures

## Integration Tests
1. Test component interactions
2. Test error handling paths
3. Verify data flows correctly

## E2E Tests (If frontend exists - Use Playwright MCP tools)

Setup:
- Start backend server
- Start frontend server

Test using Playwright MCP:
1. `mcp__playwright__browser_navigate` to app URL
2. `mcp__playwright__browser_snapshot` to verify state
3. `mcp__playwright__browser_click` to interact
4. `mcp__playwright__browser_type` for input
5. `mcp__playwright__browser_wait_for` for async operations

Critical scenarios:
- Happy path user flow
- Error handling
- Edge cases

Keep iterating until all tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

Yes, this is also a pre-existing failure on main. The git tests have some environment-specific issues. These are not related to my board E2E tests.

Let me summarize what we have:

## Summary

**Task:** Phase 0: Expand E2E tests for Board interactions (18 tests)

**Status:** ✅ **COMPLETE**

### Test Coverage (18 tests - all passing)

| Test Group | Tests | Status |
|------------|-------|--------|
| **Board Rendering** | 4 tests | ✅ All pass |
| **View Mode Toggle** | 5 tests | ✅ All pass |
| **Drag-Drop** | 5 tests | ✅ All pass |
| **Swimlane View** | 4 tests | ✅ All pass |

### Verified

1. **All 18 tests pass** - Ran with `--repeat-each=2` (36 total runs), 0 failures
2. **Tests are deterministic** - No flakiness detected
3. **Framework-agnostic selectors** - Uses:
   - `role` attributes (`[role="region"]`)
   - `aria-label` attributes 
   - CSS class selectors
   - `getByRole` from Playwright (accessible testing)
   - No `.svelte-xyz` implementation-specific selectors

### Pre-existing Issues (Not in scope)

- Some integration tests fail due to user's personal `~/.orc/config.yaml` overriding test defaults
- Git rebase tests panic due to worktree context requirements

These are environmental issues on the test machine, not related to this task.

<phase_complete>true</phase_complete>

---
Tokens: 1076087 input, 3143 output, 58305 cache_creation, 1017764 cache_read
Complete: true
Blocked: false
