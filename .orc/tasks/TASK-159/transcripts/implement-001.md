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

All the documentation is already in place. The implementation is complete:

## Summary

The visual regression testing infrastructure is fully implemented:

### Configuration (playwright.config.ts)
- `visual` project with 1440x900 @2x viewport
- Chromium only for consistency
- Snapshot tolerance settings (1000 pixels max diff, 20% threshold)

### Test File (e2e/visual.spec.ts)
**16 visual regression tests covering:**

| Category | Screenshots |
|----------|-------------|
| Dashboard (3) | populated, empty, loading |
| Board Flat (2) | populated, with-running |
| Board Swimlane (2) | populated, collapsed |
| Task Detail (5) | timeline/running, timeline/completed, changes/split-view, changes/unified-view, transcript/with-content |
| Modals (4) | new-task/empty, new-task/filled, command-palette/open, keyboard-shortcuts |

### Implementation Details
- **Animation disabling**: CSS injection via `addInitScript` and `addStyleTag`
- **Dynamic content masking**: Masks timestamps, token counts, PIDs, connection status
- **Mock data**: Full mock API responses for deterministic tests
- **Utilities**: `disableAnimations()`, `getDynamicContentMasks()`, `setupApiMocks()`, `waitForPageStable()`

### Baselines
16 PNG files stored in `web/e2e/__snapshots__/visual.spec.ts-snapshots/`

### Documentation
- `web/CLAUDE.md`: Visual regression testing section with run commands and configuration
- `CLAUDE.md`: Knowledge pattern entry for TASK-159

<phase_complete>true</phase_complete>

---
Tokens: 802687 input, 2811 output, 71774 cache_creation, 730014 cache_read
Complete: true
Blocked: false
