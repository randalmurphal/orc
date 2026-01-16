---
description: "Tech Lead UI session for orc development - test the frontend, find visual bugs, verify UX flows"
---

# Orc UI Development Session

You are acting as Tech Lead for the orc frontend, using the Playwright MCP tools to interact with the web UI.

## Primary Mission: Find What's Broken in the UI

**Your main job is discovering visual bugs, UX issues, and frontend edge cases.** Use the browser to exercise every feature and scrutinize every pixel.

Every time you interact with the UI:
- **Watch for visual glitches** - Misaligned elements, truncated text, wrong colors, broken layouts
- **Notice UX friction** - Confusing flows, too many clicks, missing feedback, unclear states
- **Catch edge cases** - Empty states, loading states, error states, long content overflow
- **Feel the experience** - Would a user understand what's happening? Is it delightful or frustrating?

When you find something wrong, **that finding is more valuable than completing the test**. Create a task for it immediately.

---

## Step 1: Start the Frontend

Ensure the orc server is running:

```bash
# Check if already running
curl -s http://localhost:8080 > /dev/null && echo "Server running" || echo "Need to start server"

# If not running, build and start
cd /home/randy/repos/orc && make build && ./bin/orc serve
```

## Step 2: Navigate to the App

Use Playwright to open the browser:

```
mcp__playwright__browser_navigate to http://localhost:8080
```

Then take a snapshot to see the current state:

```
mcp__playwright__browser_snapshot
```

## Step 3: Systematic UI Exploration

Work through each area of the UI, taking snapshots and looking for issues:

### Dashboard Page
1. Navigate to Dashboard (usually the home page)
2. Check: Task counts, initiative progress bars, recent activity
3. Look for: Alignment issues, number formatting, empty states

### Board Page
1. Navigate to Board view
2. Check: Column layout, task cards, drag indicators (if any)
3. Look for: Card overflow, status colors, initiative badges
4. Test: Filtering by initiative, status, priority

### Task List Page
1. Navigate to Task List
2. Check: Table alignment, sorting, pagination
3. Look for: Long title truncation, date formatting, status badges

### Task Detail
1. Click on a task to open detail view
2. Check: All fields populated, phase timeline, action buttons
3. Look for: Modal sizing, scroll behavior, button states

### Initiative Views
1. Navigate to Initiatives
2. Check: Initiative cards, progress indicators, task counts
3. Click into an initiative detail

### Settings/Preferences
1. Navigate to Settings
2. Check: Form layouts, toggles, save/cancel buttons
3. Test: Making changes and saving

### Modals & Dialogs
1. Open New Task modal
2. Check: Form validation, field labels, submit flow
3. Open Edit Task modal
4. Check: Pre-populated fields, cancel behavior

### Keyboard Shortcuts
1. Test Shift+Alt+P (command palette or project selector)
2. Test other shortcuts documented in the app
3. Check: Shortcut hints visible, consistent behavior

## Step 4: Visual Checklist

For each page, verify:

| Check | What to Look For |
|-------|------------------|
| **Alignment** | Elements line up, consistent spacing |
| **Typography** | Readable fonts, proper hierarchy, no truncation issues |
| **Colors** | Consistent palette, sufficient contrast, status colors correct |
| **Responsive** | Resize browser, check mobile breakpoints |
| **Loading** | Spinners appear, skeleton states, no flash of content |
| **Empty States** | Helpful messages when no data |
| **Error States** | Clear error messages, recovery options |
| **Hover States** | Buttons/links have hover feedback |
| **Focus States** | Keyboard navigation shows focus |
| **Overflow** | Long text handled gracefully |

## Step 5: Create Tasks for Issues

When you find something wrong:

```bash
# Visual bug
orc new "UI: [Component] - [description of visual issue]" --category bug --priority normal

# UX friction
orc new "UX: [Flow] - [description of friction]" --category refactor --priority normal

# Missing feature
orc new "Frontend: [Feature] - [what's needed]" --category feature --priority normal
```

### Severity Guide
| Issue Type | Priority | Example |
|------------|----------|---------|
| **Broken functionality** | high | Button doesn't work, page crashes |
| **Data display wrong** | high | Wrong task status, missing fields |
| **Visual regression** | normal | Misaligned elements, wrong colors |
| **Minor polish** | low | Spacing inconsistency, hover state |

## Step 6: Test Interactions

Use Playwright to test actual user flows:

### Create a Task
1. Click "New Task" button
2. Fill in form fields
3. Submit and verify task appears

### Edit a Task
1. Open task detail
2. Click edit
3. Change fields
4. Save and verify changes

### Filter/Sort
1. Apply filters on board/list
2. Verify correct results
3. Clear filters

### Navigation
1. Use sidebar navigation
2. Use breadcrumbs
3. Use browser back/forward

## Step 7: Take Screenshots of Issues

When you find a visual bug, capture it:

```
mcp__playwright__browser_take_screenshot with filename "issue-[description].png"
```

Include the screenshot path when creating the task.

## Playwright Commands Reference

| Action | Tool |
|--------|------|
| Navigate | `mcp__playwright__browser_navigate` |
| Get page state | `mcp__playwright__browser_snapshot` |
| Click element | `mcp__playwright__browser_click` |
| Type text | `mcp__playwright__browser_type` |
| Fill form | `mcp__playwright__browser_fill_form` |
| Press key | `mcp__playwright__browser_press_key` |
| Screenshot | `mcp__playwright__browser_take_screenshot` |
| Resize | `mcp__playwright__browser_resize` |
| Wait | `mcp__playwright__browser_wait_for` |
| Console logs | `mcp__playwright__browser_console_messages` |

## The Discovery Mindset

You're the first user every time. **Question everything.**

### Red Flags to Watch For
- "That looks off" → Trust your instincts, investigate
- "I expected X but got Y" → UX mismatch, create task
- "Where do I click?" → Navigation unclear, create task
- "What does this mean?" → Missing labels/tooltips, create task
- "Why is this slow?" → Performance issue, check console
- "That flickered" → Race condition or loading issue

### After Each Page
1. Does it look professional?
2. Would a new user understand it?
3. Are all states handled (loading, empty, error)?
4. Does it match the rest of the app?

**Don't complete a session without finding at least one visual or UX issue.** Fresh eyes always find something.

## Reporting

At the end of your session, summarize:

1. **Pages tested** - Which areas you covered
2. **Issues found** - Tasks created with IDs
3. **Screenshots taken** - Visual evidence of issues
4. **Overall assessment** - General state of the UI
