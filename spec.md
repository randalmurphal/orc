# Specification: Expand Task Info panel with more metadata

## Problem Statement

The Task Info panel in TimelineTab shows only basic information (weight, status, created date, timestamps), missing important task metadata that users need for context: priority, category, queue, initiative, last updated, blocked_by count, and branch name.

## Success Criteria

- [ ] Priority field displayed with appropriate styling (critical/high/normal/low)
- [ ] Category field displayed with icon matching TaskHeader pattern
- [ ] Queue field displayed (active/backlog)
- [ ] Initiative field displayed as clickable link to initiative page (when set)
- [ ] Updated timestamp displayed showing last modification time
- [ ] Blocked by count displayed when task has blockers (e.g., "2 tasks")
- [ ] Branch name displayed with code formatting (currently shown elsewhere but fits here)
- [ ] All new fields match existing dt/dd pair formatting
- [ ] All new fields conditionally render (skip if value is undefined/null)
- [ ] Styling consistent with existing info-item pattern

## Testing Requirements

- [ ] Unit test: TimelineTab renders priority field with correct styling class
- [ ] Unit test: TimelineTab renders category field with correct icon
- [ ] Unit test: TimelineTab renders initiative as clickable link
- [ ] Unit test: TimelineTab renders blocked_by count when blockers exist
- [ ] Unit test: TimelineTab hides optional fields when not set
- [ ] E2E test: Task detail page displays all metadata fields correctly

## Scope

### In Scope
- Adding priority, category, queue, initiative, updated_at, blocked_by count, and branch to Task Info section
- Matching existing styling patterns (dt/dd pairs, status colors)
- Making initiative a clickable link to `/initiatives/:id`
- Conditional rendering for optional fields

### Out of Scope
- Modifying the info-item CSS styling (use existing)
- Adding edit functionality within Task Info panel (TaskEditModal handles edits)
- Adding dependency management UI (DependencySidebar handles this)
- Changing the layout or position of Task Info panel

## Technical Approach

The Task Info section in TimelineTab.tsx needs additional fields. All required data is already available on the `task` prop. The initiative link requires importing `Link` from react-router-dom and `getInitiativeBadgeTitle` from stores (following TaskHeader.tsx pattern).

### Files to Modify

1. **web/src/components/task-detail/TimelineTab.tsx**
   - Import `Link` from react-router-dom
   - Import `Icon` (already imported)
   - Import `getInitiativeBadgeTitle` from `@/stores`
   - Import `CATEGORY_CONFIG`, `PRIORITY_CONFIG` from `@/lib/types`
   - Add priority field with priority-specific class
   - Add category field with icon
   - Add queue field
   - Add initiative field as Link
   - Add updated_at field (using existing `formatDate` helper)
   - Add blocked_by count field
   - Add branch field with code tag

2. **web/src/components/task-detail/TimelineTab.css**
   - Add priority-specific color classes (matching TaskHeader pattern)
   - Add category-color styling
   - Add initiative link styling (clickable, underline on hover)
   - Add branch code styling

3. **web/src/components/task-detail/TimelineTab.test.tsx** (new file)
   - Unit tests for new metadata fields
   - Mock store for initiative badge lookup

## Feature Details

### User Story
As a user viewing a task's timeline, I want to see comprehensive task metadata in the Task Info panel so that I have full context without navigating to other views.

### Acceptance Criteria

1. **Priority** - Displays the task priority with color coding:
   - critical: red/error color
   - high: orange/warning color
   - normal: muted text
   - low: muted text

2. **Category** - Displays task category with matching icon:
   - feature: sparkles icon, green
   - bug: bug icon, red
   - refactor: recycle icon, blue
   - chore: tools icon, muted
   - docs: file-text icon, orange
   - test: beaker icon, accent color

3. **Queue** - Displays "active" or "backlog"

4. **Initiative** - When task has `initiative_id`:
   - Displays initiative badge (from `getInitiativeBadgeTitle`)
   - Clickable link navigating to `/initiatives/:id`
   - Shows layers icon

5. **Updated** - Displays `updated_at` timestamp in same format as created_at

6. **Blocked By** - When `blocked_by` array has items:
   - Shows count: "N task(s)"
   - Links to DependencySidebar (or just informational)

7. **Branch** - Displays branch name in `<code>` tags matching phase-commit styling

### Field Order
1. Weight (existing)
2. Status (existing)
3. Priority (new)
4. Category (new)
5. Queue (new)
6. Initiative (new)
7. Blocked By (new, conditional)
8. Branch (new)
9. Retries (existing, conditional)
10. Created (existing)
11. Updated (new)
12. Started (existing, conditional)
13. Completed (existing, conditional)
