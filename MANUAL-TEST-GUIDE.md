# Manual Testing Guide: Settings > Slash Commands

**Feature:** Settings > Slash Commands Page
**URLs:**
- Frontend: http://localhost:5173
- API: http://localhost:8080

**Prerequisites:**
- Both servers must be running
- Browser: Chrome/Firefox/Edge (latest version)
- Screen: 1440x900 or larger for desktop testing
- Mobile device or browser dev tools for mobile testing

---

## Test Session Setup

### Before Starting
1. Open browser DevTools (F12)
2. Open Console tab (watch for errors)
3. Open Network tab (watch for failed requests)
4. Clear browser cache and localStorage
5. Take note of starting state (number of existing commands)

### Screenshot Requirements
Save screenshots to: `/tmp/qa-TASK-616/`
- Naming: `bug-XXX-description.png` for bugs
- Naming: `baseline-XXX.png` for baselines

---

## Test Suite 1: Navigation & Initial Load

### T1.1 - Navigate to Settings > Slash Commands
**Steps:**
1. Open http://localhost:5173
2. Click Settings icon in left sidebar (gear icon)
3. Click "Slash Commands" in settings submenu

**Expected:**
- Page loads without errors
- URL is `/settings/commands`
- Console has no errors
- Main heading shows "Slash Commands"
- Subtitle shows "Custom commands for Claude Code (~/.claude/commands)"

**Screenshot:** `baseline-01-initial-load.png`

**Pass/Fail:** [ ]

---

### T1.2 - Verify Settings Active State
**Steps:**
1. After navigation, observe left sidebar

**Expected:**
- Settings icon is highlighted/active (different color)
- "Slash Commands" menu item is highlighted/active
- Badge shows correct count (e.g., "4")

**Screenshot:** `baseline-02-active-state.png`

**Pass/Fail:** [ ]

---

### T1.3 - Verify All Page Elements Present
**Steps:**
1. Scan the page for all major components

**Expected:**
- ✅ Header with "Slash Commands" title
- ✅ Subtitle text visible
- ✅ "+ New Command" button (top right)
- ✅ "Project Commands" section header
- ✅ "Global Commands" section header
- ✅ Command Editor section at bottom
- ✅ At least one command card visible

**Pass/Fail:** [ ]

---

## Test Suite 2: Command List Interactions

### T2.1 - Click Command Card
**Steps:**
1. Click on any command card (e.g., /review)
2. Observe command editor

**Expected:**
- Command card highlights/shows selected state
- Editor updates to show command content
- File path appears in editor header (e.g., `.claude/commands/review.md`)
- Save button visible

**Screenshot:** `baseline-03-command-selected.png`

**Pass/Fail:** [ ]

---

### T2.2 - Switch Between Commands
**Steps:**
1. Click /review command
2. Wait for editor to load
3. Click /test command
4. Wait for editor to load
5. Click /commit command

**Expected:**
- Each click updates the editor content immediately
- Selected command highlights correctly
- Previous selection de-highlights
- File path updates for each command
- No console errors

**Pass/Fail:** [ ]

---

### T2.3 - Hover State on Command Cards
**Steps:**
1. Hover mouse over each command card
2. Do NOT click

**Expected:**
- Card shows hover state (background change, border, etc.)
- Hover state is visually distinct from selected state
- Edit and delete icons appear or become more visible

**Screenshot:** `baseline-04-hover-state.png`

**Pass/Fail:** [ ]

---

### T2.4 - Keyboard Navigation
**Steps:**
1. Click on a command card to focus it
2. Press Tab key repeatedly
3. Press Enter when focused on different command

**Expected:**
- Tab moves focus between interactive elements
- Focus indicator is visible
- Enter key selects focused command
- Arrow keys may navigate between commands (bonus)

**Pass/Fail:** [ ]

---

## Test Suite 3: Command Editor

### T3.1 - Edit Command Content
**Steps:**
1. Select /review command
2. Click in editor textarea
3. Type some new text
4. Observe UI changes

**Expected:**
- Cursor appears in editor
- Can type freely
- Syntax highlighting updates as you type
- "Modified" indicator appears (or Save button changes state)
- Save button becomes enabled

**Screenshot:** `baseline-05-editor-modified.png`

**Pass/Fail:** [ ]

---

### T3.2 - Save Edited Content
**Steps:**
1. Edit command content (add "# Test")
2. Click Save button
3. Observe result

**Expected:**
- Save button shows loading state briefly
- Success toast/message appears ("Saved" or similar)
- "Modified" indicator disappears
- Save button may become disabled again
- No console errors

**Pass/Fail:** [ ]

---

### T3.3 - Verify Save Persisted
**Steps:**
1. Edit and save /review command
2. Click different command (/test)
3. Click back to /review command

**Expected:**
- Your edits are still present
- Content was saved to backend

**Pass/Fail:** [ ]

---

### T3.4 - Keyboard Shortcuts in Editor
**Steps:**
1. Select a command
2. Edit content
3. Press Ctrl+S (or Cmd+S on Mac)

**Expected:**
- Content saves (same as clicking Save button)
- No browser "Save Page" dialog

**Pass/Fail:** [ ]

---

### T3.5 - Syntax Highlighting
**Steps:**
1. Select /review command (markdown content)
2. Observe editor content

**Expected:**
- Headers (# ## ###) have distinct color
- Code blocks (```) have distinct color
- Regular text has default color
- Colors are readable against background

**Screenshot:** `baseline-06-syntax-highlighting.png`

**Pass/Fail:** [ ]

---

## Test Suite 4: Create New Command

### T4.1 - Open New Command Modal
**Steps:**
1. Click "+ New Command" button

**Expected:**
- Modal/dialog opens
- Modal has title "New Command"
- Modal shows input fields:
  - Name (required)
  - Description (optional)
  - Scope (dropdown)
- Cancel and Create buttons visible

**Screenshot:** `baseline-07-new-command-modal.png`

**Pass/Fail:** [ ]

---

### T4.2 - Create Command with Valid Input
**Steps:**
1. Click "+ New Command"
2. Enter name: "test-qa"
3. Enter description: "QA test command"
4. Select scope: "Global"
5. Click Create

**Expected:**
- Modal closes
- Success toast appears
- New command appears in Global Commands list
- New command is auto-selected in editor
- Badge count increments by 1

**Pass/Fail:** [ ]

---

### T4.3 - Delete Test Command
**Steps:**
1. Find the "test-qa" command you just created
2. Click delete icon (trash)
3. Observe confirmation UI

**Expected:**
- Delete icon changes to confirm/cancel icons (check/X)
- OR separate confirmation dialog appears

**Screenshot:** `baseline-08-delete-confirmation.png`

**Steps (continued):**
4. Click confirm (check icon)

**Expected:**
- Command is removed from list
- If it was selected, another command is auto-selected
- Badge count decrements by 1

**Pass/Fail:** [ ]

---

## Test Suite 5: Edge Cases

### T5.1 - Empty Command Name
**Steps:**
1. Click "+ New Command"
2. Leave name field empty
3. Click Create

**Expected:**
- Error message appears: "Name is required" or similar
- Modal stays open
- Create button may be disabled

**Screenshot:** `bug-001-empty-name.png` (if different behavior)

**Pass/Fail:** [ ]

---

### T5.2 - Special Characters in Name
**Steps:**
1. Click "+ New Command"
2. Enter name: "test/command"
3. Click Create

**Expected:**
- Error message: "Name can only contain letters, numbers, hyphens, and underscores"
- OR command is rejected by API with error toast

**Actual:** [Document what happens]

**Pass/Fail:** [ ]

**Repeat with:**
- `test\command` (backslash)
- `test command` (space)
- `../../etc/passwd` (path traversal)
- `<script>alert(1)</script>` (XSS attempt)

---

### T5.3 - Very Long Command Name
**Steps:**
1. Click "+ New Command"
2. Enter name: 100+ characters ("a" repeated)
3. Click Create

**Expected:**
- Error message about maximum length
- OR input field limits characters

**Actual:** [Document what happens]

**Pass/Fail:** [ ]

---

### T5.4 - XSS in Command Content
**Steps:**
1. Select any command
2. Edit content to include: `<script>alert('XSS')</script>`
3. Save
4. Refresh page
5. Select same command

**Expected:**
- NO alert dialog appears
- Content is displayed as plain text (escaped)
- `<script>` tags visible as text, not executed

**Pass/Fail:** [ ]

---

### T5.5 - **CRITICAL** Unsaved Changes Warning
**Steps:**
1. Select /review command
2. Edit content (type "# UNSAVED CHANGES TEST")
3. Do NOT save
4. Click different command (/test)

**Expected:**
- ⚠️ Warning dialog: "You have unsaved changes. Discard?"
- Options: "Save", "Discard", "Cancel"

**Actual:** [Document what happens]

**BUG:** If changes are silently lost without warning, this is QA-616-001

**Screenshot:** `bug-QA-616-001-no-warning.png`

**Pass/Fail:** [ ]

---

### T5.6 - Browser Refresh During Edit
**Steps:**
1. Select a command
2. Edit content
3. Do NOT save
4. Press F5 (browser refresh)

**Expected:**
- Browser shows "Leave site?" warning
- OR changes are auto-saved
- OR changes are lost (acceptable if documented)

**Actual:** [Document what happens]

**Pass/Fail:** [ ]

---

### T5.7 - Rapid Command Switching
**Steps:**
1. Quickly click between commands 20 times:
   - /review → /test → /doc → /commit → /review → ...
2. Observe console for errors

**Expected:**
- UI remains responsive
- No race conditions
- No console errors
- Editor updates correctly each time

**Pass/Fail:** [ ]

---

### T5.8 - Very Long File Content
**Steps:**
1. Select a command
2. Paste 1000+ lines of content
3. Save
4. Scroll through editor

**Expected:**
- Editor handles large content smoothly
- Syntax highlighting still works
- No significant lag when typing
- Scrolling is smooth

**Pass/Fail:** [ ]

---

### T5.9 - Delete Currently Selected Command
**Steps:**
1. Click /review command to select it
2. Click delete icon on /review
3. Confirm delete

**Expected:**
- Command is deleted
- Next available command is auto-selected
- Editor shows the newly selected command
- No blank/error state

**Pass/Fail:** [ ]

---

### T5.10 - Network Failure Handling
**Steps:**
1. Open DevTools > Network tab
2. Enable "Offline" mode
3. Try to save a command
4. Try to create a new command

**Expected:**
- Error toast appears: "Network error" or "Failed to save"
- UI remains functional
- User can retry when back online

**Pass/Fail:** [ ]

---

## Test Suite 6: Mobile Testing (375x667)

### Setup
1. Open DevTools (F12)
2. Click "Toggle device toolbar" (Ctrl+Shift+M)
3. Select "iPhone SE" or enter custom: 375x667
4. Refresh page

---

### T6.1 - Mobile Navigation
**Steps:**
1. Navigate to /settings/commands on mobile viewport

**Expected:**
- Page loads correctly
- Sidebar is hidden OR shows as hamburger menu
- Main content visible
- "+ New Command" button accessible
- No horizontal scrolling

**Screenshot:** `mobile-01-initial.png`

**Pass/Fail:** [ ]

---

### T6.2 - Command Cards Stack Vertically
**Steps:**
1. Scroll through command list on mobile

**Expected:**
- Command cards stack vertically (not side-by-side)
- Each card is full width
- Cards are readable

**Screenshot:** `mobile-02-card-list.png`

**Pass/Fail:** [ ]

---

### T6.3 - Touch Targets
**Steps:**
1. Try to tap edit/delete icons

**Expected:**
- Icons are large enough to tap easily (min 44x44px)
- No mis-taps on adjacent elements
- Tap feedback visible

**Pass/Fail:** [ ]

---

### T6.4 - Editor Usability on Mobile
**Steps:**
1. Select a command
2. Try to edit content

**Expected:**
- Editor is usable
- Virtual keyboard doesn't obscure content
- Can scroll to see all content
- Save button accessible

**Pass/Fail:** [ ]

---

### T6.5 - Modal on Mobile
**Steps:**
1. Click "+ New Command"

**Expected:**
- Modal fits on screen
- All form fields accessible
- Virtual keyboard works with form
- Can submit or cancel easily

**Screenshot:** `mobile-03-new-command-modal.png`

**Pass/Fail:** [ ]

---

### T6.6 - Horizontal Scrolling Check
**Steps:**
1. Navigate entire page on mobile
2. Try to scroll horizontally

**Expected:**
- No horizontal scrollbar appears
- Content stays within viewport width
- All content accessible via vertical scrolling only

**Pass/Fail:** [ ]

---

## Test Suite 7: Console Error Monitoring

### T7.1 - Navigation Errors
**Monitor:** Console errors during all navigation tests

**Expected:** Zero JavaScript errors

**Errors Found:**
```
[List any errors here]
```

**Pass/Fail:** [ ]

---

### T7.2 - React Warnings
**Monitor:** Console warnings (yellow text)

**Expected:** No React warnings (e.g., key props, deprecated APIs)

**Warnings Found:**
```
[List any warnings here]
```

**Pass/Fail:** [ ]

---

### T7.3 - Network Errors
**Monitor:** DevTools > Network tab

**Expected:**
- All API calls return 200 or expected status
- No 404 for assets
- No CORS errors

**Errors Found:**
```
[List any 4xx/5xx responses]
```

**Pass/Fail:** [ ]

---

## Test Suite 8: Visual Comparison

### T8.1 - Compare Against Reference Image
**Steps:**
1. Open `example_ui/settings-slash-commands.png`
2. Compare against live page

**Checklist:**
- [ ] Layout structure matches
- [ ] Colors are consistent
- [ ] Spacing and alignment correct
- [ ] Icons match
- [ ] Typography matches
- [ ] Badge counts display correctly

**Differences Found:**
```
[List visual differences]
```

**Pass/Fail:** [ ]

---

## Bug Report Template

When you find a bug, document it using this template:

```markdown
### BUG: [ID] - [Title]

**Severity:** Critical / High / Medium / Low
**Confidence:** 80-100%
**Category:** Functional / Visual / Performance / Security

**Steps to Reproduce:**
1. Step one
2. Step two
3. Step three

**Expected Result:**
What should happen

**Actual Result:**
What actually happened

**Screenshot:**
Path to screenshot file

**Console Errors:**
```
[Paste any console errors]
```

**Environment:**
- Browser: Chrome 120.0.6099.130
- Viewport: 1440x900 / 375x667
- OS: Windows 11 / macOS / Linux

**Suggested Fix:**
[Optional] Your recommendation
```

---

## Test Summary Report Template

After completing all tests, fill out:

```markdown
# Test Execution Summary

**Date:** YYYY-MM-DD
**Tester:** [Your Name]
**Duration:** [Time spent]

## Results

**Total Tests:** X
**Passed:** X
**Failed:** X
**Blocked:** X

## Test Suite Results

- [ ] Suite 1: Navigation & Initial Load (X/Y passed)
- [ ] Suite 2: Command List Interactions (X/Y passed)
- [ ] Suite 3: Command Editor (X/Y passed)
- [ ] Suite 4: Create New Command (X/Y passed)
- [ ] Suite 5: Edge Cases (X/Y passed)
- [ ] Suite 6: Mobile Testing (X/Y passed)
- [ ] Suite 7: Console Errors (X/Y passed)
- [ ] Suite 8: Visual Comparison (X/Y passed)

## Critical Issues (Must Fix Before Release)

1. [List critical bugs]

## High Priority Issues

1. [List high priority bugs]

## Medium/Low Priority Issues

1. [List medium/low bugs]

## Recommendations

1. [Your recommendations]

## Sign-off

**QA Approved:** YES / NO
**Reason:** [If NO, explain why]
```

---

## Notes

- Take screenshots liberally - they're evidence
- Document unexpected behavior even if you're unsure it's a bug
- Test on multiple browsers if possible (Chrome, Firefox, Safari)
- Clear cache between test runs to avoid stale data
- If a test is blocked, note why and move on

---

## Reference Data

**Expected Commands (based on reference image):**

**Project Commands:**
1. `/review` - "Run comprehensive code review with security and performance analysis"
2. `/test` - "Generate tests for the current file using Vitest"
3. `/doc` - "Generate JSDoc comments for functions and classes"

**Global Commands:**
1. `/commit` - "Generate semantic commit message based on staged changes"

**Badge Counts:**
- Slash Commands: 4
- MCP Servers: 2
- Memory: 47
