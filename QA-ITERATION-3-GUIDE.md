# QA Iteration 3 - Testing Guide

## Overview

This guide provides step-by-step instructions for running comprehensive E2E tests on the Settings page to verify whether previous findings have been fixed.

---

## Prerequisites

### 1. Dev Server Running

The application must be running at `http://localhost:5173`

**Check if running:**
```bash
curl -s http://localhost:5173 > /dev/null && echo "✓ Server running" || echo "✗ Server not running"
```

**Start if needed:**
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web
bun run dev
```

### 2. Playwright Installed

**Check installation:**
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web
node -e "require('@playwright/test')" && echo "✓ Playwright installed" || echo "✗ Not installed"
```

**Install if needed:**
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web
npm install
npx playwright install chromium
```

---

## Running Tests

### Quick Test (Recommended for verification)

```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616
./RUN-QA-ITERATION-3.sh
```

**What it does:**
- Checks prerequisites
- Runs focused tests on the 4 previous findings
- Takes screenshots at each step
- Generates JSON report
- Provides clear pass/fail status

**Expected runtime:** ~2-3 minutes

### Detailed Test (Full coverage)

```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web
node run-qa-iteration3.mjs
```

**What it does:**
- All previous findings verification
- Happy path testing
- Edge case testing
- Mobile viewport testing (375x667)
- Console error checking

**Expected runtime:** ~5-7 minutes

---

## Test Phases

### Phase 1: Verify Previous Findings ⭐ (Priority)

Tests the 4 specific issues from Iteration 2:

#### QA-002: Forward Slash Validation
- Opens New Command modal
- Types "test/command" in name field
- Clicks Create
- **PASS**: Shows validation error
- **FAIL**: Accepts name with slash

#### QA-003: Spaces Validation
- Opens New Command modal
- Types "test command" in name field
- Clicks Create
- **PASS**: Shows validation error
- **FAIL**: Accepts name with spaces

#### QA-004: Length Validation
- Opens New Command modal
- Types 200 'a' characters in name field
- Clicks Create
- **PASS**: Shows validation error
- **FAIL**: Accepts extremely long name

#### QA-005: Modified Indicator
- Clicks first command
- Does NOT edit anything
- Clicks second command
- **PASS**: No "Modified" indicator shown
- **FAIL**: "Modified" indicator shown incorrectly

### Phase 2: Happy Path Testing

- Navigate to Settings
- Create command with valid name
- Edit command content
- Save changes
- Verify success

### Phase 3: Edge Case Testing

- Empty command name
- Special characters (@, #, $, %)
- Very long command content (10000+ chars)
- Rapid clicking Save button
- Command switching with unsaved changes

### Phase 4: Mobile Testing

- Resize to 375x667 viewport
- Verify navigation accessible
- Verify modal usable on small screen
- Verify editor usable
- Verify touch targets adequate

### Phase 5: Console Errors

- Monitor console during typical workflow
- Report any JavaScript errors
- Correlate errors with functionality

---

## Interpreting Results

### Test Output Format

```
========================================
QA ITERATION 3 - Settings Page Testing
========================================

QA-002: Forward slash validation
────────────────────────────────────────────────────────────────
  Looking for Slash Commands section...
  ✓ Found Slash Commands section
  ✓ Clicked Slash Commands
  ✓ Clicked New Command button
  Entering test value: "test/command"
  ✓ Entered value in name field
  ✓ Clicked Create/Save button
  Checking for validation errors...
  ❌ STILL_PRESENT - No validation error shown
  📸 QA-002-4-validation-result.png - Final result after submission

[... similar for QA-003, QA-004, QA-005 ...]

========================================
SUMMARY
========================================

❌ QA-002 (high): Forward slash validation
   Status: STILL_PRESENT (confidence: 95%)

❌ QA-003 (high): Spaces validation
   Status: STILL_PRESENT (confidence: 95%)

❌ QA-004 (high): Length validation
   Status: STILL_PRESENT (confidence: 95%)

❌ QA-005 (medium): Modified indicator bug
   Status: STILL_PRESENT (confidence: 90%)

Screenshots saved to: /home/randy/repos/.../qa-screenshots-iter3/
Total screenshots: 24

⚠️  4 issue(s) still present
```

### Status Indicators

| Icon | Meaning |
|------|---------|
| ✅ | Issue FIXED - validation working correctly |
| ❌ | Issue STILL_PRESENT - bug not fixed |
| ⚠️ | ERROR - Could not complete test |
| ❓ | UNKNOWN - Indeterminate result |

### Exit Codes

| Code | Meaning | Action |
|------|---------|--------|
| 0 | All tests passed | Ready for merge |
| 1 | Some tests failed | Review findings, implement fixes |
| 2 | Test error | Check prerequisites, review logs |

---

## Reviewing Screenshots

### Screenshot Naming Convention

```
{FINDING-ID}-{STEP}-{DESCRIPTION}.png
```

Examples:
- `QA-002-1-section-loaded.png` - Initial state
- `QA-002-2-modal-open.png` - Modal opened
- `QA-002-3-value-entered.png` - Test value entered
- `QA-002-4-validation-result.png` - Final result showing bug

### Screenshot Directory

```
/home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web/qa-screenshots-iter3/
```

**View screenshots:**
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web/qa-screenshots-iter3/
ls -lh
```

### What to Look For

#### FIXED Indicators (Good!)
- Error messages in red/alert styling
- Modal stays open after submit
- Error text clearly describes the issue
- Submit button disabled with invalid input

#### STILL_PRESENT Indicators (Bad!)
- No error messages visible
- Modal closes after submit
- Command appears in list with invalid name
- No visual feedback for invalid input

---

## Generated Reports

### 1. JSON Report

**Location**: `web/qa-iteration3-report.json`

**Structure:**
```json
{
  "timestamp": "2026-01-28T...",
  "iteration": 3,
  "findings": {
    "QA-002": { "status": "STILL_PRESENT", "confidence": 95 },
    ...
  },
  "screenshotDir": "..."
}
```

### 2. Comprehensive Findings

**Location**: `qa-iteration3-findings.json`

Contains:
- Detailed finding descriptions
- Code analysis results
- Suggested fixes
- Impact assessments
- Validation rules
- Next steps

---

## After Testing

### If All Tests Pass ✅

1. Review screenshots to confirm visual evidence
2. Verify JSON report shows all FIXED
3. Run full test suite one more time
4. Approve for merge

### If Tests Fail ❌

1. **Review Code Analysis**: See `QA-ITERATION-3-CODE-ANALYSIS.md`
2. **Review Findings**: See `qa-iteration3-findings.json`
3. **Examine Screenshots**: Visual evidence of bugs
4. **Implement Fixes**: Use suggested fixes from reports
5. **Re-test**: Run tests again after fixes
6. **Repeat**: Until all tests pass

---

## Troubleshooting

### Issue: "Dev server not running"

**Solution:**
```bash
cd /home/randy/repos/orc/.orc/worktrees/orc-TASK-616/web
bun run dev
# Wait for "Local: http://localhost:5173"
# In another terminal, run tests
```

### Issue: "Could not find Slash Commands section"

**Possible causes:**
- Page not fully loaded
- Settings page structure changed
- JavaScript error preventing render

**Debug:**
```bash
# Check if settings page loads
curl http://localhost:5173/settings

# Check browser console
# Run test with --headed to see browser
```

### Issue: "No input fields found in modal"

**Possible causes:**
- Modal animation not complete
- Modal structure changed
- CSS hiding modal

**Debug:**
- Review screenshot `*-modal-open.png`
- Check if modal is visible
- Increase wait timeouts in test

### Issue: "Could not find New Command button"

**Possible causes:**
- Button text changed
- Button not rendered
- Wrong section selected

**Debug:**
- Review screenshot `*-section-loaded.png`
- Verify Slash Commands section is visible
- Check button selector in test code

---

## Manual Verification (Backup)

If automated tests fail to run, perform manual verification:

### Manual Test: QA-002 (Forward Slash)

1. Open http://localhost:5173/settings in browser
2. Click "Slash Commands"
3. Click "+ New Command"
4. Type "test/command" in Name field
5. Click "Create"
6. **Expected**: Error message "Command names cannot contain forward slashes"
7. **If no error shown**: BUG STILL PRESENT

### Manual Test: QA-003 (Spaces)

1. Open http://localhost:5173/settings
2. Click "Slash Commands"
3. Click "+ New Command"
4. Type "test command" in Name field
5. Click "Create"
6. **Expected**: Error message "Command names cannot contain spaces"
7. **If no error shown**: BUG STILL PRESENT

### Manual Test: QA-004 (Length)

1. Open http://localhost:5173/settings
2. Click "Slash Commands"
3. Click "+ New Command"
4. Type 200 'a' characters in Name field
5. Click "Create"
6. **Expected**: Error message "Command name must be 50 characters or less"
7. **If no error shown**: BUG STILL PRESENT

### Manual Test: QA-005 (Modified Indicator)

1. Open http://localhost:5173/settings
2. Click "Slash Commands"
3. Click first command in list
4. Do NOT edit anything
5. Click second command in list
6. Look at editor header for "Modified" text
7. **Expected**: NO "Modified" indicator shown
8. **If "Modified" shown**: BUG STILL PRESENT

Take screenshots of each test result!

---

## Summary

This testing guide provides comprehensive instructions for:
- ✅ Running automated E2E tests
- ✅ Interpreting test results
- ✅ Reviewing screenshots
- ✅ Understanding findings
- ✅ Troubleshooting issues
- ✅ Manual verification as backup

**Key Takeaway**: The automated tests are designed to provide definitive evidence of whether the 4 previous findings have been fixed. Screenshots serve as visual proof that can be included in QA reports and shared with the development team.
