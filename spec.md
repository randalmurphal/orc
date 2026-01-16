# Specification: Add initiative and priority to task detail header

## Problem Statement
The task detail page header is missing key context information: initiative assignment and priority level are not displayed even though these fields exist on the task object. This makes it harder for users to understand task context when viewing details.

## Success Criteria
- [ ] Initiative badge displays in TaskHeader when `task.initiative_id` is set
- [ ] Initiative badge is clickable and navigates to `/initiatives/:id`
- [ ] Initiative badge shows truncated title (max 20 chars) with full title in tooltip
- [ ] Priority badge displays for non-normal priorities (critical, high, low)
- [ ] Priority badge uses consistent styling from `PRIORITY_CONFIG` (same as TaskCard)
- [ ] Critical priority shows pulsing animation (same as TaskCard)
- [ ] Initiative and priority badges appear in `.task-identity` section after existing badges
- [ ] No visual regression on existing header elements

## Testing Requirements
- [ ] Unit test: TaskHeader renders initiative badge when `task.initiative_id` is set
- [ ] Unit test: TaskHeader hides initiative badge when `task.initiative_id` is null/undefined
- [ ] Unit test: TaskHeader renders priority badge only for non-normal priorities
- [ ] Unit test: Initiative badge click navigates to initiative detail page
- [ ] E2E test: Verify initiative badge visibility and navigation on task detail page
- [ ] E2E test: Verify priority badge visibility for critical/high/low tasks

## Scope
### In Scope
- Add initiative badge to TaskHeader component
- Add priority badge to TaskHeader component (reuse existing styling, currently in wrong position)
- Update TaskHeader.css with initiative badge styles
- Tooltip support for both badges

### Out of Scope
- Modifying Task Info panel (separate enhancement)
- Adding editable initiative/priority from header (use edit modal)
- Showing target branch or blocking dependencies in header

## Technical Approach

### Files to Modify
- `web/src/components/task-detail/TaskHeader.tsx`: Add initiative badge with click handler and navigation; fix priority badge positioning in task-identity section
- `web/src/components/task-detail/TaskHeader.css`: Add `.initiative-badge` styles matching TaskCard pattern
- `web/src/components/task-detail/TaskHeader.test.tsx`: Add unit tests for new badges (create if doesn't exist)
- `web/e2e/task-detail.spec.ts`: Add E2E tests for badge visibility and navigation

### Implementation Details

1. **Initiative Badge** (in `task-identity` section):
   - Use `getInitiativeBadgeTitle()` from initiativeStore (same as TaskCard)
   - Wrap with `Tooltip` component showing full title
   - Use `Button` component with `variant="ghost"` (same pattern as TaskCard)
   - Navigate to `/initiatives/${task.initiative_id}` on click
   - Style with CSS class `.initiative-badge` (adapt from TaskCard)

2. **Priority Badge** (already implemented but verify position):
   - Priority badge code exists at lines 144-151 but need to verify visual ordering
   - Should appear after category badge, before initiative badge
   - Uses inline style with `--priority-color` CSS variable
   - Critical priority needs pulsing animation

3. **Visual Order in `.task-identity`**:
   - Task ID
   - Status indicator
   - Weight badge
   - Category badge
   - Priority badge (non-normal only)
   - Initiative badge (if assigned)

## Feature-Specific Analysis

### User Story
As a user viewing a task detail page, I want to see which initiative the task belongs to and its priority level so that I understand the task's context and urgency without having to scroll or open additional panels.

### Acceptance Criteria
- Initiative badge appears between priority badge and branch info when task has `initiative_id`
- Initiative badge shows truncated title with tooltip for full title
- Clicking initiative badge navigates to initiative detail page
- Priority badge appears for critical/high/low tasks with appropriate color coding
- Critical priority has pulsing animation matching TaskCard behavior
- Header maintains visual balance and doesn't become cluttered
