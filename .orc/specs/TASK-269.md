# Specification: Fix: Visual regression baseline dashboard-populated.png is blank

## Problem Statement

Multiple visual regression baselines are incorrect - they either capture identical states for tests that should show different states, or capture error pages ("Failed to load task") instead of actual UI content. This means visual regression tests are not detecting real regressions.

## Root Cause Analysis

Investigation revealed two distinct issues:

### Issue 1: Multiple baselines are identical (same checksum)
- **Dashboard group**: `dashboard-empty`, `dashboard-loading`, `dashboard-populated` - all identical
- **Board flat group**: `board-flat-populated`, `board-flat-with-running`, `modals-command-palette-open` - all identical
- **Board swimlane group**: `board-swimlane-collapsed`, `board-swimlane-populated` - both identical
- **Task detail/modals group**: 7 files all identical (various task detail tabs and modal states)

### Issue 2: Task detail pages show error states
The baseline for task detail pages shows "Failed to load task - Request failed" error instead of actual task content. This suggests:
1. The sandbox may have been cleaned up before screenshots were captured
2. Navigation to specific task IDs (e.g., `/tasks/TASK-005`) failed because sandbox tasks weren't available
3. The tests silently captured error states as baselines

### Issue 3: Test design flaws
Looking at `visual.spec.ts`:
- `dashboard-empty` test admits it's "effectively a duplicate" in comments
- `dashboard-loading` uses `waitUntil: 'commit'` but still captures populated state (timing issue)
- Several tests have `.catch(() => {})` swallowing errors silently
- No assertions verify expected content before taking screenshots

## Success Criteria

- [ ] Each visual baseline captures the correct, distinct state it's named for
- [ ] No two baselines have identical checksums (unless legitimately same state)
- [ ] Task detail baselines show actual task content, not error pages
- [ ] Dashboard loading baseline shows skeleton/loading state
- [ ] Dashboard empty baseline shows empty state placeholder (requires test setup change)
- [ ] Command palette baseline shows open command palette, not board
- [ ] New task modal baselines show empty vs filled form states
- [ ] All 18 baseline files are regenerated and visually verified
- [ ] Visual tests pass when run: `cd web && bunx playwright test --project=visual`

## Testing Requirements

- [ ] E2E test: Run `cd web && bunx playwright test --project=visual` - all tests pass
- [ ] Manual verification: Visually inspect each regenerated baseline to confirm it shows the correct state
- [ ] Uniqueness check: Run `md5sum` on all baselines - verify appropriate files are unique

## Scope

### In Scope
- Fix test setup/timing issues in `web/e2e/visual.spec.ts`
- Add assertions to verify expected state before capturing screenshots
- Ensure sandbox tasks are available for task detail screenshots
- Regenerate all baselines with correct content
- Remove or fix tests that can't realistically capture distinct states

### Out of Scope
- Adding new visual regression tests
- Changing the visual regression test infrastructure (Playwright config)
- Modifying the sandbox setup beyond what's needed for these tests

## Technical Approach

### 1. Fix dashboard tests
- **dashboard-populated**: Works correctly (shows populated dashboard)
- **dashboard-empty**: Either create a separate empty sandbox OR remove this test (sandbox always has data)
- **dashboard-loading**: Block API responses to capture loading skeleton state

### 2. Fix task detail tests
- Add explicit waits for task content to load
- Assert task title/content is visible before screenshot
- Don't swallow errors with `.catch(() => {})`

### 3. Fix modal tests
- **command-palette**: Use keyboard shortcut (Shift+Alt+K) to actually open command palette
- **new-task-empty/filled**: Navigate to `/tasks/new` and verify form is visible

### 4. Fix board tests
- **board-flat-with-running**: This requires a running task which sandbox doesn't have; consider removing or documenting as duplicate

### 5. Add pre-screenshot assertions
Before each `toHaveScreenshot()`, add assertions that verify:
- Expected content is visible
- No error states are shown
- The correct page/modal is displayed

### Files to Modify

| File | Changes |
|------|---------|
| `web/e2e/visual.spec.ts` | Fix timing issues, add assertions, improve test reliability |
| `web/e2e/__snapshots__/visual.spec.ts-snapshots/*.png` | Regenerate all baselines |

### Implementation Steps

1. Review and fix each test in `visual.spec.ts`:
   - Add `await expect(page.locator(...)).toBeVisible()` before screenshots
   - Remove silent error swallowing
   - Fix timing issues (use route interception for loading states)

2. For tests that can't show distinct states, either:
   - Remove the test (if duplicate)
   - Document why states are similar
   - Fix test setup to enable distinct states

3. Regenerate baselines:
   ```bash
   cd web && bunx playwright test --project=visual --update-snapshots
   ```

4. Manually verify each baseline shows correct content

5. Run tests to confirm they pass:
   ```bash
   cd web && bunx playwright test --project=visual
   ```

## Bug-Specific Analysis

### Reproduction Steps
1. Navigate to `web/e2e/__snapshots__/visual.spec.ts-snapshots/`
2. Run `md5sum *.png | sort` to see checksum groups
3. Open any task detail baseline - shows error page

### Current Behavior
- Multiple baselines are identical files
- Task detail baselines show "Failed to load task" error
- Visual tests pass but don't catch regressions

### Expected Behavior
- Each baseline captures a unique, correct UI state
- Task detail baselines show actual task content
- Visual tests detect real UI changes

### Root Cause
1. Tests captured screenshots at wrong time (before content loaded, after sandbox torn down)
2. Tests silently swallowed errors with `.catch(() => {})`
3. No assertions verified expected content before screenshot
4. Some tests are conceptually duplicates (can't show different state with current sandbox)

### Verification
After fix:
1. `md5sum *.png | sort` shows unique checksums for distinct states
2. Visual inspection confirms correct content in each baseline
3. `bunx playwright test --project=visual` passes
