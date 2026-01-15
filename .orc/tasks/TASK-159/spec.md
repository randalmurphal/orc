# Specification: Visual Regression Baselines for All Pages

## Summary

Capture Playwright screenshot baselines for visual regression testing during React migration. This enables detecting unintended visual changes when migrating components from Svelte 5 to React 19.

## Problem Statement

The orc web UI is preparing for a React migration. Without visual regression baselines:
1. Visual bugs during migration may go unnoticed until production
2. No objective way to verify "looks the same" after component rewrites
3. Manual visual QA is time-consuming and error-prone

## Requirements

### Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1 | Capture baselines for all major pages (Dashboard, Board, Task Detail) | Must Have |
| FR-2 | Capture baselines for both flat and swimlane board views | Must Have |
| FR-3 | Capture baselines for all modal states (New Task, Command Palette, Keyboard Shortcuts) | Must Have |
| FR-4 | Support loading/empty states for Dashboard | Must Have |
| FR-5 | Disable CSS animations during capture for determinism | Must Have |
| FR-6 | Mask dynamic content (timestamps, token counts, PIDs) | Must Have |

### Non-Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| NFR-1 | Consistent viewport: 1440x900 @2x for retina quality | Must Have |
| NFR-2 | Single browser (Chromium) for consistency | Must Have |
| NFR-3 | Tolerance settings for minor anti-aliasing differences | Should Have |
| NFR-4 | Mock API responses for deterministic data | Must Have |

## Technical Approach

### Architecture

```
web/
├── playwright.config.ts     # Visual project configuration
├── e2e/
│   ├── visual.spec.ts       # All visual regression tests
│   └── __snapshots__/       # Baseline screenshots
│       └── visual.spec.ts-snapshots/
│           ├── dashboard-populated.png
│           ├── dashboard-empty.png
│           ├── dashboard-loading.png
│           ├── board-flat-populated.png
│           ├── board-flat-with-running.png
│           ├── board-swimlane-populated.png
│           ├── board-swimlane-collapsed.png
│           ├── task-detail-timeline-running.png
│           ├── task-detail-timeline-completed.png
│           ├── task-detail-changes-split-view.png
│           ├── task-detail-changes-unified-view.png
│           ├── task-detail-transcript-with-content.png
│           ├── modals-new-task-empty.png
│           ├── modals-new-task-filled.png
│           ├── modals-command-palette-open.png
│           └── modals-keyboard-shortcuts.png
```

### Playwright Configuration

```typescript
// Dedicated visual project in playwright.config.ts
{
  name: 'visual',
  testMatch: /visual\.spec\.ts$/,
  use: {
    ...devices['Desktop Chrome'],
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2, // @2x retina
  },
}

// Snapshot tolerance settings
expect: {
  toHaveScreenshot: {
    maxDiffPixels: 1000,    // Allow minor anti-aliasing
    threshold: 0.2,          // 20% pixel color threshold
  },
}
```

### Animation Disabling

```typescript
// Injected CSS to disable animations
await page.addStyleTag({
  content: `
    *, *::before, *::after {
      animation-duration: 0s !important;
      transition-duration: 0s !important;
    }
    .running-pulse, .status-pulse, .pulsing {
      animation: none !important;
    }
  `,
});
```

### Dynamic Content Masking

Mask elements that change between runs:
- `.timestamp`, `.time-ago`, `.date-time`
- `.token-value`, `.token-count`
- `.connection-status .status-text`
- `.executor-pid`, `.heartbeat`
- `.progress-percentage`

### Mock API Responses

Use deterministic mock data for all API endpoints:
- `/api/tasks` - Fixed set of tasks in various states
- `/api/initiatives` - Fixed initiatives
- `/api/dashboard/stats` - Fixed statistics
- `/api/tasks/*/state` - Running or completed states
- `/api/tasks/*/diff/*` - Fixed diff content
- `/api/tasks/*/transcripts` - Fixed transcript list

## Pages to Capture

### Dashboard (3 screenshots)

| Screenshot | State | Description |
|------------|-------|-------------|
| `dashboard/populated.png` | Full data | All stats, recent activity, initiatives |
| `dashboard/empty.png` | No data | Empty state for new projects |
| `dashboard/loading.png` | Loading | Skeleton loading state |

### Board Flat View (2 screenshots)

| Screenshot | State | Description |
|------------|-------|-------------|
| `board/flat/populated.png` | Multiple tasks | Tasks in various columns |
| `board/flat/with-running.png` | Running task | Pulse animation disabled |

### Board Swimlane View (2 screenshots)

| Screenshot | State | Description |
|------------|-------|-------------|
| `board/swimlane/populated.png` | Expanded | Multiple initiative swimlanes |
| `board/swimlane/collapsed.png` | Collapsed | All swimlanes collapsed |

### Task Detail (5 screenshots)

| Screenshot | State | Description |
|------------|-------|-------------|
| `task-detail/timeline/running.png` | Running phase | Active implement phase |
| `task-detail/timeline/completed.png` | All phases done | Completed task |
| `task-detail/changes/split-view.png` | Split diff | Side-by-side diff |
| `task-detail/changes/unified-view.png` | Unified diff | Combined diff |
| `task-detail/transcript/with-content.png` | Multiple iterations | Expanded transcript |

### Modals (4 screenshots)

| Screenshot | State | Description |
|------------|-------|-------------|
| `modals/new-task/empty.png` | Fresh form | No input entered |
| `modals/new-task/filled.png` | Completed form | All fields filled |
| `modals/command-palette/open.png` | Initial state | Palette open, no search |
| `modals/keyboard-shortcuts.png` | Help modal | All shortcuts displayed |

## Success Criteria

### Code Requirements

- [x] Playwright config updated with `visual` project
- [x] Viewport set to 1440x900 @2x
- [x] Chromium-only for consistency
- [x] Snapshot tolerance configured (maxDiffPixels: 1000, threshold: 0.2)
- [x] Test file `e2e/visual.spec.ts` created

### Test Coverage

- [x] Dashboard: populated, empty, loading (3 tests)
- [x] Board Flat: populated, with-running (2 tests)
- [x] Board Swimlane: populated, collapsed (2 tests)
- [x] Task Detail Timeline: running, completed (2 tests)
- [x] Task Detail Changes: split-view, unified-view (2 tests)
- [x] Task Detail Transcript: with-content (1 test)
- [x] Modals: new-task empty/filled, command-palette, keyboard-shortcuts (4 tests)
- [x] **Total: 16 visual regression tests**

### Infrastructure

- [x] Animation disabling via CSS injection
- [x] Dynamic content masking for timestamps/tokens/PIDs
- [x] Mock API responses for deterministic data
- [x] Baseline screenshots captured in `e2e/__snapshots__/`

### Documentation

- [x] `web/CLAUDE.md` updated with visual testing section
- [x] Usage commands documented (`--update-snapshots`)
- [x] Techniques for determinism documented

## Testing Strategy

### Running Visual Tests

```bash
# Compare against baselines (CI mode)
bunx playwright test --project=visual

# Capture new baselines (after intentional UI changes)
bunx playwright test --project=visual --update-snapshots
```

### Test Isolation

- Visual tests excluded from `chromium` project via `testIgnore`
- Dedicated `visual` project runs only `visual.spec.ts`
- Mock API prevents flaky tests from real data

### Updating Baselines

After intentional UI changes:
1. Run `--update-snapshots` to capture new baselines
2. Review diff in PR to verify changes are expected
3. Commit new baselines

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Flaky screenshots from animation timing | Disable all animations via CSS injection |
| Different results across machines | Fixed viewport, single browser, mock data |
| Dynamic content causing false failures | Comprehensive masking of timestamps, tokens, PIDs |
| Dropdown interactions flaky | Retry logic with multiple attempts |

## File Changes

| File | Change |
|------|--------|
| `web/playwright.config.ts` | Added `visual` project configuration |
| `web/e2e/visual.spec.ts` | New file: 16 visual regression tests |
| `web/e2e/__snapshots__/` | New directory: 16 baseline screenshots |
| `web/CLAUDE.md` | Updated with visual testing documentation |

## Dependencies

- Playwright 1.x (already installed)
- No new dependencies required

## Notes

- Baselines stored in `e2e/__snapshots__/visual.spec.ts-snapshots/`
- Screenshot naming uses hyphen-separated path: `dashboard-populated.png`
- Tests use `fullPage: true` for complete page capture
- Retry logic handles flaky UI interactions (especially dropdowns)
