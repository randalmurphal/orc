# validate - Iteration 1

## Prompt

Final validation for:

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

## Validation Checklist

### Code
- [ ] All requirements from spec are met
- [ ] Unit tests pass: `go test ./... -v -race`
- [ ] Test coverage > 80%
- [ ] No race conditions
- [ ] Code quality acceptable (no debug statements, TODOs)

### Integration
- [ ] Components work together
- [ ] Error handling is complete
- [ ] Performance is acceptable

### E2E (If frontend - MUST RUN with Playwright MCP)
- [ ] Main user flows work
- [ ] Error states handled
- [ ] UI updates correctly

### Documentation
- [ ] Code is documented where needed
- [ ] CLAUDE.md updated if necessary
- [ ] API documented if applicable

## Final Verification

If frontend exists, run E2E verification:
1. Start all servers
2. Use Playwright MCP tools to test all features
3. Verify everything works end-to-end

Keep iterating until validation passes.

When done, output:
<phase_complete>true</phase_complete>


## Response

## Final Validation Summary

**All success criteria met:**

### ✅ Baselines Captured (16/16)
| Category | Screenshots | Status |
|----------|-------------|--------|
| Dashboard | `populated`, `empty`, `loading` | ✓ 3 captured |
| Board Flat | `populated`, `with-running` | ✓ 2 captured |
| Board Swimlane | `populated`, `collapsed` | ✓ 2 captured |
| Task Detail Timeline | `running`, `completed` | ✓ 2 captured |
| Task Detail Changes | `split-view`, `unified-view` | ✓ 2 captured |
| Task Detail Transcript | `with-content` | ✓ 1 captured |
| Modals | `new-task/empty`, `new-task/filled`, `command-palette/open`, `keyboard-shortcuts` | ✓ 4 captured |

### ✅ Configuration
- **Viewport**: 1440x900 @2x (deviceScaleFactor: 2) ✓
- **Browser**: Chromium only via `visual` project ✓
- **Snapshot settings**: 1000 maxDiffPixels, 0.2 threshold ✓
- **Snapshot directory**: `e2e/__snapshots__/` ✓

### ✅ Technical Implementation
- **Animation disabling**: CSS injection via `addStyleTag()` and `addInitScript()` ✓
- **Dynamic content masking**: Timestamps, token counts, PIDs masked ✓
- **Mock API responses**: Deterministic data for all states ✓
- **Retry logic**: Handles flaky dropdowns ✓

### ✅ Documentation
- `web/CLAUDE.md` updated with Visual Regression Tests section (lines 885-916) ✓
- Root `CLAUDE.md` updated with visual regression pattern in Knowledge section ✓

### ✅ Tests Pass
All 16 visual regression tests pass in 14.7s.

<phase_complete>true</phase_complete>

---
Tokens: 483309 input, 2035 output, 76186 cache_creation, 406215 cache_read
Complete: true
Blocked: false
