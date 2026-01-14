# spec - Iteration 1

## Prompt

Create a specification for this large task:

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

Define:
1. Requirements and scope
2. Technical approach
3. Component breakdown (backend, frontend if applicable)
4. API design (if applicable)
5. Success criteria with explicit checkboxes
6. Testing strategy:
   - Unit tests
   - Integration tests
   - E2E tests (using Playwright MCP tools if frontend exists)

Include clear completion criteria:
- What code must be written
- What tests must pass
- What E2E scenarios must work
- What documentation must exist

Keep iterating until the specification is clear and complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

The spec is complete with all 18 tests defined (4 + 5 + 5 + 4 = 18). Let me verify the test count is correct:

- **Board Rendering**: 4 tests
- **View Mode Toggle**: 5 tests  
- **Drag-Drop**: 5 tests
- **Swimlane View**: 4 tests

**Total: 18 tests** ✓

The spec covers:
1. ✅ Requirements and scope
2. ✅ Technical approach (selector strategy, framework-agnostic patterns)
3. ✅ Component breakdown (which components need data-testid)
4. ✅ Success criteria with explicit checkboxes
5. ✅ Testing strategy (E2E focus, patterns to follow)
6. ✅ Implementation notes (drag-drop specifics, localStorage, flakiness prevention)
7. ✅ Reference materials

The spec is clear and complete. It defines behavior (not implementation), uses framework-agnostic selectors, and includes explicit completion criteria.

<phase_complete>true</phase_complete>

---
Tokens: 354783 input, 4843 output, 58685 cache_creation, 294530 cache_read
Complete: true
Blocked: false
