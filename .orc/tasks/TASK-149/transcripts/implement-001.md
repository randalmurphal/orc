# implement - Iteration 1

## Prompt

Implement the following task:

**Task**: Add initiative status management to UI - complete/archive actions

**Description**: ## Problem
There's no way to mark initiatives as completed or archived through the UI. The CLI has `orc initiative complete` but users should be able to manage initiative lifecycle from the web interface.

## Solution
Add initiative status management to the UI:

1. **Initiative Detail Page** (`/initiatives/[id]`):
   - Add status badge showing current status (draft/active/completed/archived)
   - Add action buttons/dropdown for status transitions:
     - Draft → Active (activate)
     - Active → Completed (complete)
     - Active → Archived (archive)
   - Confirm dialog for destructive actions (archive)

2. **API Endpoints** (if not existing):
   - `PUT /api/initiatives/:id/status` with body `{status: 'completed'|'archived'|'active'}`
   - Or individual endpoints: `POST /api/initiatives/:id/complete`, etc.

3. **Initiatives List**:
   - Show status badge on each initiative card
   - Optional: filter by status

## Success Criteria
1. Users can mark initiative as completed from initiative detail page
2. Users can archive initiatives from UI
3. Status changes reflect immediately in UI
4. API endpoints for status management exist and work
5. Completed/archived initiatives visually distinct in lists

## Files to Modify
- web/src/routes/initiatives/[id]/+page.svelte (add status actions)
- web/src/lib/api.ts (add status change methods)
- internal/api/handlers.go or similar (add/verify endpoints)
- internal/api/routes.go (register endpoints if new)

## Retry Context

This phase is being re-executed due to a failure in a later phase.

**What happened:**
- Phase "test" failed/was rejected
- Reason: Phase test failed: node execute: execute: claude completion: claude complete: chdir /home/randy/repos/orc/.orc/worktrees/orc-TASK-149: no such file or directory: 
- This is retry attempt #1

**What to fix:**
Please address the issues that caused the later phase to fail. The failure output is below:

---
node execute: execute: claude completion: claude complete: chdir /home/randy/repos/orc/.orc/worktrees/orc-TASK-149: no such file or directory: 
---

Focus on fixing the root cause of these issues in this phase.

Detailed context saved to: .orc/tasks/retry-context-test-1.md


1. Implement the required changes
2. Write/update tests as needed
3. Run tests and fix any failures

Keep iterating until implementation is complete and all tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

Let me summarize what was implemented:

## Summary of Changes

### 1. Initiative Detail Page (`web/src/routes/initiatives/[id]/+page.svelte`)
- **Status action buttons** - Added context-aware status transition buttons:
  - Draft → Active: "Activate" button with play icon
  - Active → Completed: "Complete" button (green, success style) with checkmark
  - Completed → Active: "Reopen" button with rotate icon
- **Archive confirmation modal** - Replaced browser `confirm()` with a proper modal dialog for archiving initiatives, explaining the action
- **Loading states** - All status actions show loading states while processing
- **Error handling** - Status change failures show appropriate error messages

### 2. Sidebar (`web/src/lib/components/layout/Sidebar.svelte`)
- **Status badges** on initiative list items for non-active statuses:
  - Draft: gray badge
  - Completed: green badge with success styling
  - Archived: gray muted badge
- Active initiatives show progress counts instead of status badge (existing behavior)

### 3. Dashboard Initiatives (`web/src/lib/components/dashboard/DashboardInitiatives.svelte`)
- **Status badges** for initiatives not in "active" status
- Active initiatives show progress bar (existing behavior)

### API
The existing `PUT /api/initiatives/:id` endpoint already supports status changes via the `status` field. No API changes were needed.

### Success Criteria Met
1. ✅ Users can mark initiative as completed from initiative detail page (Complete button)
2. ✅ Users can archive initiatives from UI (Archive button with confirmation modal)
3. ✅ Status changes reflect immediately in UI (store updates after API call)
4. ✅ API endpoints for status management exist and work (existing PUT endpoint)
5. ✅ Completed/archived initiatives visually distinct in lists (status badges in sidebar and dashboard)

### Testing
- All 325 frontend tests pass
- All backend Go tests pass
- The pre-existing svelte-check error in Board.svelte (unrelated to this task) was not introduced by these changes

<phase_complete>true</phase_complete>

---
Tokens: 3618043 input, 9612 output, 87339 cache_creation, 3529373 cache_read
Complete: true
Blocked: false
