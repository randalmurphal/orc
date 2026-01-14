# test - Iteration 1

## Prompt

Comprehensive testing for:

**Task**: Phase 0: Capture visual regression baselines for all pages

**Description**: ## Purpose
Capture Playwright screenshot baselines for visual regression testing during migration.

## Pages to Capture

### Dashboard
- dashboard/populated.png - Full data state
- dashboard/empty.png - No tasks, no initiatives
- dashboard/loading.png - Skeleton loading state

### Board
- board/flat/populated.png - Tasks in all columns
- board/flat/with-running.png - Running task with pulse animation
- board/swimlane/populated.png - Multiple initiative swimlanes
- board/swimlane/collapsed.png - Collapsed swimlanes

### Task Detail
- task-detail/timeline/running.png - Active phase
- task-detail/timeline/completed.png - All phases done
- task-detail/changes/split-view.png - Split diff mode
- task-detail/changes/unified-view.png - Unified diff mode
- task-detail/transcript/with-content.png - Multiple iterations

### Modals
- modals/new-task/empty.png - Fresh form
- modals/new-task/filled.png - Completed form
- modals/command-palette/open.png - Initial state
- modals/keyboard-shortcuts.png - Help modal

## Configuration
- Disable CSS animations during capture
- Mask dynamic content (timestamps, token counts)
- Use consistent viewport: 1440x900 @2x
- Single browser (Chromium) for consistency

## Success Criteria
- Baselines captured for all states listed above
- Playwright config updated with visual comparison settings
- Dynamic content masking implemented

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

Now let me summarize what was accomplished:

## Summary

**Captured 16 visual regression baselines** covering all pages/states from the spec:

### Dashboard (3)
- `dashboard-populated.png` - Full data state
- `dashboard-empty.png` - No tasks, no initiatives  
- `dashboard-loading.png` - Skeleton loading state

### Board (4)
- `board-flat-populated.png` - Tasks in all columns
- `board-flat-with-running.png` - Running task with pulse animation
- `board-swimlane-populated.png` - Multiple initiative swimlanes
- `board-swimlane-collapsed.png` - Collapsed swimlanes

### Task Detail (5)
- `task-detail-timeline-running.png` - Active phase
- `task-detail-timeline-completed.png` - All phases done
- `task-detail-changes-split-view.png` - Split diff mode
- `task-detail-changes-unified-view.png` - Unified diff mode
- `task-detail-transcript-with-content.png` - Multiple iterations

### Modals (4)
- `modals-new-task-empty.png` - Fresh form
- `modals-new-task-filled.png` - Completed form  
- `modals-command-palette-open.png` - Initial state
- `modals-keyboard-shortcuts.png` - Help modal

### Configuration
- ✅ Viewport: 1440x900 @2x (retina)
- ✅ Single browser: Chromium
- ✅ CSS animations disabled via `disableAnimations()` 
- ✅ Dynamic content masking for timestamps, token counts, PIDs, etc.
- ✅ Threshold tuned to allow minor anti-aliasing variance (1000 pixels / 2% ratio)

All tests passing consistently.

<phase_complete>true</phase_complete>

---
Tokens: 959802 input, 3413 output, 61846 cache_creation, 897068 cache_read
Complete: true
Blocked: false
