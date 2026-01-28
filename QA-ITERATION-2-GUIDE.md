# QA Testing Guide - Settings Page (Iteration 2)

## Overview

This guide documents the QA testing process for verifying bug fixes from Iteration 1 and performing comprehensive edge case testing of the Settings > Slash Commands page.

## Prerequisites

### 1. Servers Running

```bash
# Terminal 1: API server
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616
./bin/orc serve

# Terminal 2: Frontend server
cd web
bun run dev
```

Verify:
- API: http://localhost:8080/api/health
- Frontend: http://localhost:5173

### 2. Playwright Installed

```bash
cd web
bun install
bunx playwright install chromium
```

## Running Tests

### Quick Run

```bash
chmod +x run-qa-iteration2.sh
./run-qa-iteration2.sh
```

### Manual Run

```bash
cd web
bunx playwright test settings-slash-commands-iteration2.spec.ts --headed
```

### Debug Mode

```bash
cd web
bunx playwright test settings-slash-commands-iteration2.spec.ts --debug
```

## Test Categories

### 🐛 Bug Verification (Iteration 1 Fixes)

| Test ID | Bug Description | Expected Behavior | Confidence |
|---------|-----------------|-------------------|------------|
| **QA-001** | Editor content doesn't update when switching commands | Content should change when different command selected | 95% |
| **QA-002** | No validation for forward slash (/) | Should show error for "test/command" | 90% |
| **QA-003** | No validation for spaces | Should show error for "test command" | 90% |
| **QA-004** | No max length validation | Should show error for 200+ char names | 90% |

**Pass Criteria:**
- Editor content differs between commands
- Validation errors appear for invalid inputs
- Modal stays open when validation fails
- Error messages are user-friendly

**Fail Indicators:**
- Screenshots named `*-BUG-STILL-PRESENT.png`
- Console output shows "✗ FAILED"
- Modal closes without showing error

### 🔬 Edge Case Testing

#### Special Characters

Tests: `@`, `#`, `$`, `%`, `^`, `&`, `*`, `(`, `)`, `=`, `+`, `..`, `../`

**Expected Behavior:**
- Only alphanumeric, hyphens, and underscores allowed
- Path traversal attempts (`../`, `..`) rejected
- Special chars show validation error

**Screenshots:**
- `edge-special-chars-summary.png` - Final state
- Individual character tests if failures occur

#### Unicode and Emoji

Tests: Japanese, Hebrew, Chinese, emoji

**Expected Behavior:**
- Validation policy is CONSISTENT (either allow all or reject all)
- No rendering issues with RTL text
- No JavaScript errors

**Screenshots:**
- `edge-unicode-Japanese-characters.png`
- `edge-unicode-Emoji.png`
- `edge-unicode-Hebrew-(RTL).png`
- `edge-unicode-Chinese.png`

#### Empty/Whitespace

Tests: Empty string, spaces, tabs, newlines

**Expected Behavior:**
- All should be rejected
- Clear error message shown

**Screenshots:**
- Screenshots capture the error state for each case

#### Rapid Interactions

Tests: Rapid command switching, double-click submit

**Expected Behavior:**
- No console errors during rapid clicking
- No duplicate submissions on double-click
- UI remains responsive

**Screenshots:**
- `edge-rapid-switching.png`
- `edge-double-click-BUG.png` (if bug exists)

### 📱 Mobile Viewport Testing

Viewport: 375x667 (iPhone SE)

**Pass Criteria:**
- No horizontal scroll
- Touch targets ≥ 44x44px
- Modal fits viewport
- Command list accessible
- Editor visible after selection

**Fail Indicators:**
- `mobile-horizontal-scroll-BUG.png` - Body wider than viewport
- Warning about button sizes < 44px
- Modal overflow

**Screenshots:**
- `mobile-initial.png`
- `mobile-command-selected.png`
- `mobile-modal-open.png`

### 🔄 State Management

#### Browser Refresh

**Test:** Edit content → Refresh page

**Expected:**
- Unsaved changes lost (acceptable)
- Page still functional after refresh
- No console errors

**Screenshots:**
- `state-before-refresh.png`
- `state-after-refresh.png`

#### Navigation Without Saving

**Test:** Edit content → Navigate to different settings page

**Expected:**
- Warning dialog appears (if implemented)
- OR changes silently lost but page functional
- No crashes or errors

**Screenshots:**
- `state-navigation-warning.png` (if warning exists)

### 🐛 Console Monitoring

**Checks:**
- JavaScript errors
- React warnings
- Network errors (except expected 404s)

**Pass Criteria:**
- Zero console errors during normal use
- Warnings are acceptable (logged but don't fail)

## Interpreting Results

### Test Output

```bash
✓ QA-001 PASSED: Editor content correctly updated when switching commands
✗ QA-002 FAILED: Accepted slash (/) in command name without validation
```

### Screenshot Naming Convention

| Pattern | Meaning |
|---------|---------|
| `qa-001-first-command-selected.png` | Step in test flow |
| `qa-002-BUG-STILL-PRESENT.png` | **Bug still exists** |
| `qa-002-FIXED-validation-error.png` | **Bug is fixed** |
| `edge-*.png` | Edge case test result |
| `mobile-*.png` | Mobile viewport test |

### HTML Report

After tests complete:

```bash
cd web
bunx playwright show-report
```

Navigate to:
- **Failed tests** - Click to see trace, screenshots, video
- **Passed tests** - Verify expected behavior
- **Flaky tests** - Tests that failed then passed on retry

## Generating QA Report

### 1. Collect Results

```bash
ls -lh /tmp/qa-TASK-616-iteration2/
```

### 2. Identify Issues

Look for:
- `*-BUG-STILL-PRESENT.png` screenshots
- Test failures in console output
- Console errors in output

### 3. Create Findings

For each issue found:

```json
{
  "id": "QA-ITER2-001",
  "severity": "critical|high|medium|low",
  "confidence": 80-100,
  "category": "functional",
  "title": "Brief description",
  "steps_to_reproduce": [
    "Step 1",
    "Step 2"
  ],
  "expected": "What should happen",
  "actual": "What happened",
  "screenshot_path": "/tmp/qa-TASK-616-iteration2/qa-002-BUG-STILL-PRESENT.png",
  "console_errors": ["error 1", "error 2"]
}
```

### 4. Severity Guidelines

| Severity | Examples |
|----------|----------|
| **Critical** | Editor doesn't update (data loss risk), crashes |
| **High** | No validation (security/data integrity), major UX issue |
| **Medium** | Minor validation gaps, edge case failures |
| **Low** | Cosmetic, rare edge cases |

### 5. Confidence Guidelines

| Score | Meaning |
|-------|---------|
| 95-100 | Definite bug, clear reproduction |
| 85-94 | Likely bug, reproducible |
| 80-84 | Probable bug, minor uncertainty |
| < 80 | Don't report without more testing |

## Common Issues and Debugging

### Test Fails: "Modal did not appear"

**Cause:** Button selector wrong, or modal takes longer to open

**Debug:**
```bash
# Run in headed mode to watch
bunx playwright test settings-slash-commands-iteration2.spec.ts --headed --debug
```

### Test Fails: "Element not stable"

**Cause:** Animations still running despite being disabled

**Fix:** Add longer wait after click:
```typescript
await button.click();
await page.waitForTimeout(500); // Increase if needed
```

### Screenshots All Black

**Cause:** Modal overlay blocking content

**Fix:** Screenshot includes full page, should capture modal + content

### False Positive: Empty Commands List

**Cause:** Test environment has no commands

**Expected:** Tests should SKIP validation tests gracefully with warning

## Reference Screenshots

### Expected Layout

Compare test screenshots against `example_ui/settings-slash-commands.png`:

| Element | Expected |
|---------|----------|
| Sidebar | 240px width, "Slash Commands" selected |
| Badge | Count of commands (e.g., "8") |
| Sections | "Project Commands" and "Global Commands" |
| Cards | Command name, description, edit/delete icons |
| Editor | File path, content, Save button |

### Validation Errors

Expected error message format:
- **Slash:** "Name can only contain letters, numbers, hyphens, and underscores"
- **Space:** Same as slash
- **Length:** "Name must be 50 characters or less" (or configured max)
- **Empty:** "Name is required"

## Success Criteria

### All Tests Pass

- ✅ All 4 bug verification tests pass
- ✅ Edge cases handled correctly
- ✅ Mobile viewport usable
- ✅ No console errors

### Partial Pass

If some edge cases fail but core bugs are fixed:
- Document edge case failures separately
- Mark core bugs as FIXED
- Recommend edge case improvements

### Failure

If any Iteration 1 bugs still present:
- Mark as **REGRESSION** or **NOT FIXED**
- Provide detailed reproduction steps
- Include screenshots and console output

## Deliverables

1. **Test Execution Log**
   - Console output from test run
   - Pass/fail summary

2. **Screenshots**
   - All screenshots in `/tmp/qa-TASK-616-iteration2/`
   - Organized by test category

3. **QA Report**
   - JSON findings for each issue (confidence ≥ 80)
   - Summary of verified fixes
   - New issues discovered

4. **Playwright HTML Report**
   - Full trace and video for failures
   - Available via `bunx playwright show-report`

## Next Steps After Testing

### All Tests Pass
- ✅ Mark TASK-616 as complete
- Document any minor issues for future enhancement
- Update project knowledge base

### Tests Fail
- 🔴 Create new tasks for unfixed bugs
- Link back to original bugs (QA-001, etc.)
- Assign appropriate priority

### Edge Cases Fail
- 🟡 Evaluate severity
- Decide if blocking or can defer
- Document in project knowledge

## Appendix: Manual Testing Checklist

If automated tests can't run, use this manual checklist:

### Bug Verification

- [ ] **QA-001**: Select command A, note content, select command B, verify content changed
- [ ] **QA-002**: New Command → Enter "test/command" → Click Create → Error shown?
- [ ] **QA-003**: New Command → Enter "test command" → Click Create → Error shown?
- [ ] **QA-004**: New Command → Enter 200 'a' characters → Click Create → Error shown?

### Edge Cases

- [ ] Try special chars: @, #, $, %, ^, &, *, (, ), =, +
- [ ] Try unicode: 日本語, emoji 🚀
- [ ] Try empty string
- [ ] Try whitespace only
- [ ] Rapidly click between 5 commands
- [ ] Double-click Create button

### Mobile

- [ ] Resize browser to 375x667
- [ ] Check for horizontal scroll (use DevTools)
- [ ] Tap commands, verify selection
- [ ] Open modal, verify it fits
- [ ] Measure button sizes (DevTools inspector)

### State

- [ ] Edit content, refresh page, verify changes lost
- [ ] Edit content, navigate away, check for warning

### Console

- [ ] Open DevTools Console
- [ ] Perform all above actions
- [ ] Note any errors or warnings

## Contact

For questions or issues with this QA process:
- Check Playwright docs: https://playwright.dev
- Review test file: `web/e2e/settings-slash-commands-iteration2.spec.ts`
- Check CI/CD pipeline for similar test execution
