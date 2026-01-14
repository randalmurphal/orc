# Phase 0: Visual Regression Baselines for All Pages

## Overview

Capture Playwright screenshot baselines for visual regression testing to enable safe UI migration from Svelte to React. Baselines establish a "golden reference" that future changes are compared against.

## Requirements

### Core Requirements

1. **Capture screenshot baselines** for all major UI pages and states
2. **Disable animations** during capture to ensure deterministic screenshots
3. **Mask dynamic content** (timestamps, token counts, task IDs) to prevent false positives
4. **Configure Playwright** for visual regression testing with appropriate thresholds
5. **Single browser** (Chromium) at 1440x900 @2x for consistency

### Pages and States to Capture

| Category | Screenshot | Description |
|----------|------------|-------------|
| **Dashboard** | `dashboard/populated.png` | Full data state with tasks and initiatives |
| **Dashboard** | `dashboard/empty.png` | No tasks, no initiatives |
| **Dashboard** | `dashboard/loading.png` | Skeleton loading state |
| **Board** | `board/flat/populated.png` | Tasks in all columns |
| **Board** | `board/flat/with-running.png` | Running task with pulse animation frozen |
| **Board** | `board/swimlane/populated.png` | Multiple initiative swimlanes |
| **Board** | `board/swimlane/collapsed.png` | Collapsed swimlanes |
| **Task Detail** | `task-detail/timeline/running.png` | Active phase in timeline |
| **Task Detail** | `task-detail/timeline/completed.png` | All phases done |
| **Task Detail** | `task-detail/changes/split-view.png` | Split diff mode |
| **Task Detail** | `task-detail/changes/unified-view.png` | Unified diff mode |
| **Task Detail** | `task-detail/transcript/with-content.png` | Multiple transcript iterations |
| **Modals** | `modals/new-task/empty.png` | Fresh form state |
| **Modals** | `modals/new-task/filled.png` | Completed form ready to submit |
| **Modals** | `modals/command-palette/open.png` | Initial command palette state |
| **Modals** | `modals/keyboard-shortcuts.png` | Keyboard shortcuts help modal |

## Technical Approach

### 1. Animation Disabling Utility

Create a reusable utility to inject CSS that disables all animations during visual regression tests:

```typescript
// web/e2e/utils/disable-animations.ts
async function disableAnimations(page: Page) {
  await page.addStyleTag({
    content: `
      *, *::before, *::after {
        animation-duration: 0s !important;
        animation-delay: 0s !important;
        transition-duration: 0s !important;
        transition-delay: 0s !important;
      }
    `
  });
}
```

This approach:
- Affects all elements (including pseudo-elements)
- Preserves final animation states (elements at their end position)
- Works without modifying production code
- Compatible with `prefers-reduced-motion` if added later

### 2. Dynamic Content Masking

Use Playwright's built-in `mask` option for `toHaveScreenshot()`:

```typescript
await expect(page).toHaveScreenshot('dashboard/populated.png', {
  mask: [
    page.locator('.timestamp'),           // e.g., "2 hours ago"
    page.locator('.token-count'),         // e.g., "245K tokens"
    page.locator('.task-id'),             // e.g., "TASK-123"
    page.locator('.relative-time'),       // e.g., "Updated 5m ago"
  ]
});
```

Alternative: Use `maskColor: '#FF00FF'` (magenta) for debugging which areas are masked.

### 3. Playwright Configuration Updates

Update `playwright.config.ts` for visual regression:

```typescript
export default defineConfig({
  // ... existing config
  use: {
    // ... existing use config
    screenshot: 'only-on-failure',

    // Visual regression defaults
    viewport: { width: 1440, height: 900 },
  },
  expect: {
    toHaveScreenshot: {
      maxDiffPixels: 100,          // Allow minor anti-aliasing differences
      maxDiffPixelRatio: 0.01,     // 1% pixel tolerance
      threshold: 0.2,              // Per-pixel color threshold
    },
  },
  snapshotDir: './e2e/visual-baselines',  // Store baselines separately
});
```

### 4. Test Data Setup

Visual regression tests need consistent data states:

**Approach A: Use existing test database** (simpler)
- Rely on actual project data
- Some states may not be achievable (empty state, running task)
- More realistic screenshots

**Approach B: Mock API responses** (more reliable)
- Intercept API calls with `page.route()`
- Return predetermined data for each state
- Guarantees all states can be captured

**Recommended: Hybrid**
- Use real data where available
- Mock specific states that are hard to reproduce (loading, running task)

### 5. Visual Regression Test Structure

Create `web/e2e/visual-regression.spec.ts`:

```typescript
import { test, expect, Page } from '@playwright/test';
import { disableAnimations } from './utils/disable-animations';

// Common mask selectors for dynamic content
const DYNAMIC_MASKS = [
  '.timestamp',
  '.relative-time',
  '.token-count',
];

test.describe('Visual Regression', () => {
  test.beforeEach(async ({ page }) => {
    await disableAnimations(page);
  });

  test.describe('Dashboard', () => {
    test('populated state', async ({ page }) => {
      await page.goto('/');
      await page.waitForLoadState('networkidle');

      await expect(page).toHaveScreenshot('dashboard/populated.png', {
        mask: DYNAMIC_MASKS.map(s => page.locator(s)),
        fullPage: true,
      });
    });

    // ... more dashboard tests
  });

  // ... more page categories
});
```

## Component Breakdown

### Files to Create

| File | Purpose |
|------|---------|
| `web/e2e/utils/disable-animations.ts` | Reusable animation disabling utility |
| `web/e2e/utils/visual-masks.ts` | Common mask selector definitions |
| `web/e2e/visual-regression.spec.ts` | Visual regression test suite |
| `web/e2e/visual-baselines/` | Directory for baseline screenshots |

### Files to Modify

| File | Changes |
|------|---------|
| `web/playwright.config.ts` | Add visual regression settings, snapshotDir |
| `web/.gitignore` | Ensure baselines are tracked, diffs ignored |

## Test Strategy

### What We're Testing

Visual regression tests verify:
- Layout consistency (element positions, sizes)
- Color accuracy (theming, status indicators)
- Typography (fonts rendering correctly)
- Component visibility (modals open/closed states)

### What We're NOT Testing

Visual regression tests do not replace:
- Functional E2E tests (already exist in `board.spec.ts`, etc.)
- Unit tests
- Accessibility tests

### Running the Tests

```bash
# Generate baselines (first time or after intentional changes)
bunx playwright test visual-regression --update-snapshots

# Run visual comparison
bunx playwright test visual-regression

# View visual comparison report
bunx playwright show-report
```

### Handling Failures

When visual regression fails:
1. Review the diff in Playwright report
2. If intentional: update baselines with `--update-snapshots`
3. If unintentional: fix the regression

## Success Criteria

### Must Have

- [ ] Animation disabling utility created and working
- [ ] Dynamic content masking implemented for timestamps/tokens/IDs
- [ ] Playwright config updated with visual regression settings
- [ ] Visual regression test file created with all page states
- [ ] All 16 baseline screenshots captured successfully
- [ ] Screenshots are deterministic (running twice produces identical results)

### Test Validation

- [ ] `bunx playwright test visual-regression` passes on second run
- [ ] No false positives from animations
- [ ] No false positives from dynamic content
- [ ] Baselines committed to repository

### Documentation

- [ ] `web/CLAUDE.md` updated with visual regression testing section
- [ ] Comments in test file explaining state setup for each screenshot

## Implementation Order

1. **Create utilities** - `disable-animations.ts`, `visual-masks.ts`
2. **Update Playwright config** - viewport, thresholds, snapshotDir
3. **Create test file structure** - empty test cases for all screenshots
4. **Implement dashboard tests** - populate test logic, capture baselines
5. **Implement board tests** - flat and swimlane views
6. **Implement task detail tests** - timeline, changes, transcript tabs
7. **Implement modal tests** - new task, command palette, shortcuts
8. **Validate baselines** - run twice, ensure identical results
9. **Update documentation** - add visual regression section to CLAUDE.md

## Open Questions / Decisions Needed

### Q1: State Setup for Empty/Loading States

**Options:**
1. Mock API responses to return empty data
2. Use a separate test database that can be reset
3. Skip empty/loading states if not reproducible

**Recommendation:** Mock API for empty state, intercept network for loading state (delay response).

### Q2: Running Task State

Running tasks have animated borders. Options:
1. Mock a task as "running" via API interception
2. Actually run a task (slower, may timeout)
3. Skip running state screenshots

**Recommendation:** Mock via API - return task with `status: 'running'` in the response.

### Q3: Baseline Storage Location

**Options:**
1. `web/e2e/visual-baselines/` - alongside tests
2. `web/e2e/__screenshots__/` - Playwright default naming
3. Root-level `.visual-baselines/` - separate from tests

**Recommendation:** `web/e2e/visual-baselines/` - clear purpose, near tests.

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Flaky tests from async data | Use `waitForLoadState('networkidle')` + explicit waits |
| Font rendering differences | Run in Docker CI with consistent fonts |
| Different monitor DPI | Fixed viewport + deviceScaleFactor in config |
| Large baseline file size | Use PNG compression, consider WebP if supported |

## Out of Scope

- Responsive testing (only 1440x900 viewport)
- Multi-browser testing (Chromium only per requirements)
- Dark mode testing (can be added later)
- Mobile viewport testing
- Performance benchmarking
