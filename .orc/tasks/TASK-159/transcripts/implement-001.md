# implement - Iteration 1

## Prompt

Implement the large task according to the specification:

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

**Specification**:
## Purpose
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



Implementation protocol:
1. Implement all components defined in the spec
2. Write unit tests alongside code
3. Run tests frequently: `go test ./... -v -race`
4. Fix failures before continuing
5. If frontend exists:
   - Implement all components
   - Add loading/error states
   - Integrate with API

Keep iterating until:
- All components implemented
- All unit tests pass
- No race conditions

When done, output:
<phase_complete>true</phase_complete>


## Response

The new files are untracked. Let me summarize what was implemented:

## Implementation Summary

### Files Created:
1. **`web/e2e/visual.spec.ts`** - Visual regression test suite with:
   - Mock data fixtures for deterministic testing
   - CSS animation disabling utilities
   - Dynamic content masking (timestamps, token counts, etc.)
   - 16 visual tests covering all specified pages/states

2. **`web/e2e/__snapshots__/visual.spec.ts-snapshots/`** - 16 baseline screenshots:

### Files Modified:
1. **`web/playwright.config.ts`** - Updated with:
   - Snapshot directory configuration
   - Visual comparison thresholds (maxDiffPixels: 100, threshold: 0.2)
   - New "visual" project with 1440x900 @2x viewport
   - testIgnore to exclude visual tests from chromium project

### Baselines Captured:

| Category | Screenshots |
|----------|-------------|
| **Dashboard** | populated, empty, loading |
| **Board (Flat)** | populated, with-running |
| **Board (Swimlane)** | populated, collapsed |
| **Task Detail** | timeline/running, timeline/completed, changes/split-view, changes/unified-view, transcript/with-content |
| **Modals** | new-task/empty, new-task/filled, command-palette/open, keyboard-shortcuts |

### Configuration:
- Viewport: 1440x900 @2x (retina quality)
- CSS animations: Disabled via injected styles
- Dynamic content masked: timestamps, token counts, connection status, PIDs
- Single browser: Chromium only (for consistency)

All 16 visual tests pass consistently.

<phase_complete>true</phase_complete>

---
Tokens: 4848700 input, 20616 output, 175077 cache_creation, 4662295 cache_read
Complete: true
Blocked: false
