# Specification: Create InitiativeDetailPage for /initiatives/:id route

## Problem Statement

The existing `InitiativeDetail.tsx` component implements the `/initiatives/:id` route but its layout differs from the design specification. The task requires renaming to `InitiativeDetailPage.tsx` (matching page naming conventions) and restructuring the layout to show a stats row, side-by-side stats/decisions section, filterable task list, and collapsible dependency graph—rather than the current tab-based navigation.

## Success Criteria

- [ ] File `web/src/pages/InitiativeDetailPage.tsx` exists and exports `InitiativeDetailPage` component
- [ ] Route `/initiatives/:id` renders `InitiativeDetailPage` (update `routes.tsx`)
- [ ] Back link navigates to `/initiatives` (not `/board?initiative=...`)
- [ ] Header displays: title with emoji extraction from title/vision, status badge, Edit button
- [ ] Progress bar shows visual progress with "X/Y tasks (Z%)" text
- [ ] Stats row displays 3 stat cards: Total Tasks, Completed, Total Cost
- [ ] Decisions section shows list with "+ Add Decision" button (inline, no tab)
- [ ] Task list is filterable by status (All, Completed, In Progress, Planned)
- [ ] Task items are clickable, navigating to `/tasks/:id`
- [ ] Dependency graph section is collapsible (expand/collapse toggle)
- [ ] 404 state displayed when initiative ID doesn't exist
- [ ] Loading state displayed while fetching data
- [ ] Error state with retry option displayed on API failure
- [ ] Old `InitiativeDetail.tsx` file is removed after migration

## Testing Requirements

- [ ] Unit test: `InitiativeDetailPage` renders loading state initially
- [ ] Unit test: `InitiativeDetailPage` displays 404 when initiative not found
- [ ] Unit test: Stats row calculates correct values from task data
- [ ] Unit test: Task list filter shows correct subset of tasks
- [ ] Unit test: Back link navigates to `/initiatives`
- [ ] E2E test: Navigate to `/initiatives/:id`, verify all sections render
- [ ] E2E test: Click task in list, verify navigation to task detail page
- [ ] E2E test: Add a decision inline, verify it appears in list
- [ ] E2E test: Toggle dependency graph collapse/expand

## Scope

### In Scope

- Rename `InitiativeDetail.tsx` → `InitiativeDetailPage.tsx`
- Restructure layout from tabs to sections (stats row, side-by-side sections, task list, graph)
- Add task list filtering by status
- Add collapsible dependency graph section
- Change back link destination to `/initiatives`
- Update route import in `routes.tsx`
- Create corresponding CSS file `InitiativeDetailPage.css`
- Preserve all existing functionality (edit modal, link task modal, add decision modal, status transitions)

### Out of Scope

- Adding cost/token tracking to API (use placeholder or existing data)
- Creating new API endpoints
- Modifying the `DependencyGraph` component internals
- Initiative list page changes (handled by separate task)
- Mobile responsive breakpoints (follow existing patterns)

## Technical Approach

### Layout Structure (matching spec mockup)

```
┌────────────────────────────────────────────────────────────┐
│ ← Back to Initiatives                                      │
├────────────────────────────────────────────────────────────┤
│ [emoji] Title                          [STATUS] [Edit]     │
│ Vision/description text                                    │
├────────────────────────────────────────────────────────────┤
│ Progress ████████░░░░░░░░░░░░ 45% (5/11 tasks)            │
├──────────────────────────┬─────────────────────────────────┤
│ Stats                    │ Decisions                       │
│ [Task][Done][Cost]       │ • Decision 1                    │
│                          │ • Decision 2                    │
│                          │ [+ Add Decision]                │
├──────────────────────────┴─────────────────────────────────┤
│ Tasks                                      [Filter ▾]      │
│ ┌─────────────────────────────────────────────────────────┐│
│ │ Task list items...                                      ││
│ └─────────────────────────────────────────────────────────┘│
├────────────────────────────────────────────────────────────┤
│ Dependency Graph                           [▼ Expand]      │
│ ┌─────────────────────────────────────────────────────────┐│
│ │ Graph visualization (collapsed by default)              ││
│ └─────────────────────────────────────────────────────────┘│
└────────────────────────────────────────────────────────────┘
```

### Files to Modify

- `web/src/pages/InitiativeDetailPage.tsx` (rename from `InitiativeDetail.tsx`, restructure layout)
- `web/src/pages/InitiativeDetailPage.css` (rename from `InitiativeDetail.css`, update styles)
- `web/src/router/routes.tsx` (update import from `InitiativeDetail` to `InitiativeDetailPage`)
- `web/src/pages/index.ts` (update export if exists)

### Key Implementation Details

1. **Emoji extraction**: Parse first emoji from title or use default based on status
   ```typescript
   const extractEmoji = (text: string): string => {
     const emojiMatch = text.match(/^(\p{Emoji})/u);
     return emojiMatch ? emojiMatch[1] : '📋';
   };
   ```

2. **Stats row**: Reuse `StatCard` component or create inline
   ```typescript
   <div className="stats-row">
     <StatCard label="Tasks" value={progress.total} />
     <StatCard label="Completed" value={progress.completed} />
     <StatCard label="Cost" value={formatCost(totalCost)} variant="primary" />
   </div>
   ```

3. **Task filter state**:
   ```typescript
   type TaskFilter = 'all' | 'completed' | 'running' | 'planned';
   const [taskFilter, setTaskFilter] = useState<TaskFilter>('all');
   ```

4. **Collapsible graph section**:
   ```typescript
   const [graphExpanded, setGraphExpanded] = useState(false);
   ```

5. **Back link change**:
   ```tsx
   <Link to="/initiatives" className="back-link">
     <Icon name="arrow-left" size={16} />
     <span>Back to Initiatives</span>
   </Link>
   ```

## Feature-Specific Analysis

### User Story

As a project manager, I want to view comprehensive initiative details on a single page so that I can quickly assess progress, review decisions, and navigate to individual tasks without switching between tabs.

### Acceptance Criteria

1. **Header Section**
   - Initiative title displayed with extracted emoji icon
   - Status badge (draft/active/completed/archived) with appropriate color
   - Edit button opens edit modal (existing functionality)
   - Status transition buttons based on current status (existing functionality)

2. **Progress Section**
   - Visual progress bar with fill percentage
   - Text showing "X/Y tasks (Z%)" format
   - Green fill color for completed portion

3. **Stats Row**
   - 3 stat cards displayed horizontally
   - Task count: total linked tasks
   - Completed: count of completed tasks
   - Cost: total cost (placeholder "$0.00" if not available from API)

4. **Decisions Section**
   - Listed inline (not in tab)
   - Each decision shows date, author, text, rationale
   - "+ Add Decision" button at bottom opens modal (existing functionality)

5. **Task List**
   - Filter dropdown with options: All, Completed, Running, Planned
   - Each task row shows: status icon, task ID, title, status text
   - Rows are clickable, navigate to `/tasks/:id`
   - "Link Existing" and "Add Task" buttons preserved

6. **Dependency Graph**
   - Section collapsed by default
   - "Expand" / "Collapse" toggle button
   - When expanded, shows `DependencyGraph` component (existing)
   - Lazy-loads graph data when first expanded
