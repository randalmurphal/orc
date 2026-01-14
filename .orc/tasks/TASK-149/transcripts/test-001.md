# test - Iteration 1

## Prompt

Final test verification for: Add initiative status management to UI - complete/archive actions

**Original task**: ## Problem
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

1. Run the full test suite
2. Fix any failures found
3. Verify edge cases are covered

Keep iterating until all tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

The background dev server was killed as expected. The task is complete - all tests pass and the initiative status management feature is fully functional in the UI.

---
Tokens: 84684 input, 33 output, 453 cache_creation, 84228 cache_read
Complete: false
Blocked: false
