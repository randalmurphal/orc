# spec - Iteration 1

## Prompt

Create a specification for this large task:

**Task**: Phase 0: Expand E2E tests for Task Detail tabs (15 tests)

**Description**: ## Purpose
Comprehensive E2E tests for Task Detail page tabs that define BEHAVIOR (framework-agnostic).

## Test Coverage (15 tests)

### Tab Navigation (4 tests)
- should show all tabs (Timeline, Changes, Transcript, Test Results, Attachments, Comments)
- should switch tabs when clicked
- should update URL with tab parameter
- should load correct tab from URL query param

### Timeline Tab (3 tests)
- should show phase timeline with status indicators
- should show token usage stats (input, output, cached, total)
- should show iteration and retry counts

### Changes Tab - Diff Viewer (5 tests)
- should load and display diff stats
- should show file list with additions/deletions counts
- should expand/collapse files
- should toggle between split and unified view
- should show line numbers

### Transcript Tab (2 tests)
- should show transcript history
- should expand transcript content sections

### Attachments Tab (1 test)
- should display attachment list with thumbnails

## Implementation Notes
- Tests must work on both Svelte and React
- Use data-testid for tab content areas
- Verify API calls are made correctly

## Success Criteria
- All 15 tests pass on current Svelte implementation
- Tests cover all tab functionality

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

The spec is complete. Let me verify the test count adds up correctly:
- Tab Navigation: 4 tests
- Timeline Tab: 3 tests
- Changes Tab: 5 tests
- Transcript Tab: 2 tests
- Attachments Tab: 1 test
- **Total: 15 tests** ✓

The specification covers:
1. **Requirements and scope** - Clear boundaries, selector strategy
2. **Technical approach** - File structure, selector priority, prerequisites
3. **Component breakdown** - Backend APIs (no changes), Frontend components (no changes), Test structure
4. **API design** - N/A (using existing endpoints)
5. **Success criteria** - Explicit checkboxes for code, tests, and docs
6. **Testing strategy** - E2E only (this IS the test task), with data requirements

The spec is clear and actionable. The implementation phase can proceed directly from this specification.

<phase_complete>true</phase_complete>

---
Tokens: 305100 input, 4199 output, 19275 cache_creation, 285034 cache_read
Complete: true
Blocked: false
