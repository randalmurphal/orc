# spec - Iteration 1

## Prompt

Create a specification for this large task:

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

The spec is complete. Let me provide a summary:

## Spec Complete

The specification covers:

**Core deliverables:**
- Animation disabling utility (CSS injection approach - cleanest, no production code changes)
- Dynamic content masking for timestamps, token counts, task IDs
- 16 baseline screenshots across Dashboard, Board, Task Detail, and Modals
- Playwright config updates for visual regression (thresholds, viewport, snapshot directory)

**Technical approach:**
- Uses Playwright's built-in `toHaveScreenshot()` with `mask` option
- Fixed 1440x900 @2x viewport, Chromium-only
- Baselines stored in `web/e2e/visual-baselines/`
- Hybrid data setup: real data where available, mocked for specific states (loading, running)

**Key decisions flagged:**
1. Empty/loading states via API mocking
2. Running task state via mocked API response 
3. Baseline storage in `web/e2e/visual-baselines/`

**Success criteria includes:**
- All 16 screenshots captured
- Deterministic results (running twice = identical)
- No false positives from animations or dynamic content
- Documentation updated

The spec is ready for implementation. The utilities and test structure are straightforward since Playwright has excellent visual regression support built-in.

<phase_complete>true</phase_complete>

---
Tokens: 325671 input, 3813 output, 38003 cache_creation, 276321 cache_read
Complete: true
Blocked: false
