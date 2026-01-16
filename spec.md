# Specification: Update visual regression baselines

## Problem Statement
The visual regression test baselines need to be updated after completing UI primitive migrations (Button, Radix components) in the React frontend. This ensures baselines reflect intentional styling changes while catching any unexpected regressions.

## Success Criteria
- [ ] All 16 visual regression tests pass with updated baselines
- [ ] Each failing diff reviewed and categorized (intentional vs regression)
- [ ] No unexpected regressions remain unfixed
- [ ] Intentional changes documented in commit message
- [ ] Updated baselines committed to repository

## Testing Requirements
- [ ] Visual test: `bunx playwright test --project=visual` passes (all 16 tests)
- [ ] HTML report reviewed for all diffs via `bunx playwright show-report`
- [ ] Existing E2E tests still pass: `bunx playwright test --project=chromium`

## Scope

### In Scope
- Run visual regression tests to identify diffs
- Review each diff against expected UI changes from this initiative
- Update baselines for intentional changes:
  - Button primitive styling (focus rings, variants)
  - Radix component states (`data-state` classes)
  - Modal styling (Radix Dialog)
  - Dropdown styling (Radix Select/DropdownMenu)
  - TabNav styling (Radix Tabs)
  - Tooltip styling (Radix Tooltip)
- Document all baseline changes in commit message
- Create bug tasks for any regressions found

### Out of Scope
- Modifying test logic or adding new tests
- Fixing regressions (those get separate bug tasks)
- Changing screenshot configuration or viewport

## Technical Approach

### Phase 1: Initial Test Run
```bash
cd web && bunx playwright test --project=visual
```
Capture which tests fail and review Playwright HTML report.

### Phase 2: Diff Analysis
For each failing screenshot:
1. Open HTML report (`bunx playwright show-report`)
2. Compare expected vs actual side-by-side
3. Check if diff relates to known UI changes:
   - Button: focus-visible ring, variant colors, padding
   - Badge: font-size, padding adjustments
   - Modal: Radix overlay/content styling
   - Dropdown/Select: open state animations, highlight styles
   - Tabs: active state indicator, focus styling
   - Tooltip: arrow, positioning, fade animation

### Phase 3: Categorize and Act
| Diff Type | Action |
|-----------|--------|
| Intentional (matches UI primitive changes) | Update baseline |
| Regression (unexpected visual change) | Create bug task, block this task |
| Flaky (animation timing, etc.) | Review masking in visual.spec.ts |

### Phase 4: Update Baselines
```bash
bunx playwright test --project=visual --update-snapshots
```

### Phase 5: Verify
```bash
bunx playwright test --project=visual  # All should pass now
bunx playwright test --project=chromium  # Ensure no breakage
```

### Files to Modify
- `web/e2e/__snapshots__/visual.spec.ts-snapshots/*.png`: Updated baseline images

### Files to Review (Not Modify)
- `web/e2e/visual.spec.ts`: Understand masking and test setup
- `web/playwright.config.ts`: Understand tolerance thresholds

## Expected Intentional Changes

Based on the UI primitives initiative tasks (TASK-207, TASK-209, TASK-212, TASK-213, TASK-214, TASK-215, TASK-216):

| Component | Expected Visual Changes |
|-----------|------------------------|
| Button | Unified focus ring (`focus-visible` outline), consistent padding across variants |
| TaskCard | Radix DropdownMenu for quick menu (may have slight position/animation diff) |
| TabNav | Radix Tabs active state (`data-state='active'`), focus indicator |
| Dropdowns | Radix Select styling, typeahead highlight, keyboard focus states |
| Modal | Radix Dialog overlay opacity, focus trap behavior |
| Tooltip | Radix Tooltip arrow, delay behavior (masked by animation disable) |

## Regression Indicators

Watch for these signs of regression (NOT intentional):
- Layout shifts (elements moved significantly)
- Missing content (text, icons disappeared)
- Color changes unrelated to Button/Radix work
- Broken alignment or spacing
- Z-index issues (overlapping elements incorrectly)

## Category-Specific Section: Test/Chore

### Acceptance Criteria
1. Visual regression tests pass cleanly
2. Baseline updates reflect only intentional UI changes
3. No regressions shipped
4. Changes documented for future reference
