# docs - Iteration 1

## Prompt

Update documentation for:

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

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

Documentation updates are complete. Let me do a summary of what was updated:

## Documentation Updates Summary

**1. web/CLAUDE.md - Test files table (line 890)**
- Added `e2e/task-detail.spec.ts` entry documenting 15 tests covering: tab navigation, timeline, changes, transcript, attachments

**2. web/CLAUDE.md - Routes table (line 841)**  
- Updated `/tasks/:id` route description to include all 6 tabs: Timeline/Changes/Transcript/Test Results/Attachments/Comments (was missing Test Results and Comments)

**3. Test file itself (web/e2e/task-detail.spec.ts)**
- Already has comprehensive JSDoc documentation at the top describing:
  - Test coverage breakdown (15 tests across 5 groups)
  - Selector strategy priority order
  - Framework-agnostic approach for future React migration
  - Reference to CLAUDE.md for selector documentation

The CLAUDE.md in the main project root doesn't need updates since E2E test details belong in the web-specific documentation, and the test file's inline documentation is thorough.

<phase_complete>true</phase_complete>

---
Tokens: 735440 input, 2315 output, 33078 cache_creation, 702352 cache_read
Complete: true
Blocked: false
