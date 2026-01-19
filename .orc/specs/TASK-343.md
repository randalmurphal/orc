# Specification: Create Swimlane component for initiative grouping

## Problem Statement

The Board view needs a Swimlane component for grouping tasks by initiative in the Queue column, matching the reference design in example_ui/board.html. The existing Swimlane.tsx is designed for a different layout (horizontal columns within swimlanes) and needs to be refactored to support the Queue column's vertical task list design with collapsible content and smooth animations.

## Success Criteria

- [ ] `web/src/components/board/Swimlane.tsx` exports a Swimlane component
- [ ] Header row contains:
  - [ ] Collapse/expand chevron icon that rotates -90deg when collapsed
  - [ ] Initiative icon (emoji displayed in 20x20 colored circle)
  - [ ] Initiative name (font-weight 500, truncates with ellipsis)
  - [ ] Task count badge showing "{completed}/{total} complete"
  - [ ] Small progress bar (40px wide, 3px height) with fill based on completion
- [ ] Collapsible content area renders TaskCard components vertically
- [ ] Collapsed state: only header visible, chevron rotated -90deg
- [ ] Smooth height animation on expand/collapse (0.2s transition)
- [ ] Component accepts props matching the interface:
  ```typescript
  interface SwimlaneProps {
    initiative: Initiative | null;  // null = unassigned
    tasks: Task[];
    isCollapsed: boolean;
    onToggle: () => void;
    onTaskClick?: (task: Task) => void;
    onContextMenu?: (task: Task, e: React.MouseEvent) => void;
    maxVisible?: number;  // For "+N more" truncation
  }
  ```
- [ ] 'Unassigned' swimlane displays when initiative is null, with neutral styling
- [ ] Empty swimlanes show "No tasks" message in muted text
- [ ] Very long initiative names truncate with ellipsis (text-overflow: ellipsis)
- [ ] `npm run typecheck` exits with code 0

## Testing Requirements

- [ ] Unit test: Swimlane renders header with initiative name, icon, and progress
- [ ] Unit test: Swimlane renders TaskCards for provided tasks
- [ ] Unit test: Chevron rotates when isCollapsed is true
- [ ] Unit test: Content area hidden when isCollapsed is true
- [ ] Unit test: Empty swimlane shows "No tasks" message
- [ ] Unit test: "Unassigned" label displays when initiative is null
- [ ] Unit test: Progress bar width matches completion percentage
- [ ] Unit test: Task count shows correct completed/total format
- [ ] Unit test: Long initiative names truncate with ellipsis
- [ ] Unit test: onToggle callback fires when header is clicked

## Scope

### In Scope

- Refactoring Swimlane component to match Queue column design
- Updating Swimlane.css with styles matching example_ui/board.html (lines 296-345)
- Header with icon, name, progress bar, count, and chevron
- Collapsible task list with animation
- "No tasks" empty state
- "Unassigned" special case handling
- Keyboard accessibility (Enter/Space to toggle)
- ARIA attributes for collapse state

### Out of Scope

- Drag-and-drop reordering of tasks within swimlanes
- "+N more" button with expand functionality (can be added later)
- Initiative color customization API
- Swimlane reordering
- Persisting collapsed state (handled by parent component)

## Technical Approach

### Component Architecture

The Swimlane component will be a controlled component receiving `isCollapsed` state from its parent (Queue column). The parent manages which swimlanes are collapsed via a Set<string> state.

### Props Interface Change

Current interface relies on `columns` and `tasksByColumn` for horizontal layout. New interface simplifies to vertical task list:
- Remove: `columns`, `tasksByColumn`
- Rename: `collapsed` -> `isCollapsed`, `onToggleCollapse` -> `onToggle`
- Add: `maxVisible` for truncation support

### Styling Approach

- Use CSS flexbox for header layout matching reference design
- CSS transition on max-height/opacity for smooth collapse animation
- Color variations: use initiative metadata or fallback to purple for unassigned

### Reference Design (from example_ui/board.html)

Header structure:
```html
<div class="swimlane-header">
  <div class="swimlane-icon purple">emoji</div>
  <div class="swimlane-info">
    <div class="swimlane-name">Initiative Name</div>
    <div class="swimlane-meta">X/Y complete</div>
  </div>
  <div class="swimlane-progress">
    <div class="swimlane-progress-fill purple" style="width: X%"></div>
  </div>
  <svg class="swimlane-chevron">...</svg>
</div>
```

Key CSS from reference:
- `.swimlane-header`: flex, gap 8px, padding 8px 10px, cursor pointer
- `.swimlane-icon`: 20x20, border-radius 4px, flex centered, font-size 10px
- `.swimlane-name`: font-size 11px, font-weight 600
- `.swimlane-meta`: font-size 9px, color muted
- `.swimlane-progress`: 40px wide, 3px height
- `.swimlane-chevron`: rotates -90deg when collapsed
- `.swimlane.collapsed .swimlane-tasks`: display none

### Files to Modify

| File | Changes |
|------|---------|
| `web/src/components/board/Swimlane.tsx` | Refactor to new vertical design with simplified props |
| `web/src/components/board/Swimlane.css` | Update styles to match reference design |
| `web/src/components/board/Swimlane.test.tsx` | Create comprehensive test suite |
| `web/src/components/board/Board.tsx` | Update Swimlane usage to new props interface |

### Implementation Details

1. **Swimlane.tsx changes**:
   - Simplify props interface (remove columns, tasksByColumn)
   - Update header to match reference (icon + info + progress + chevron)
   - Replace column grid with vertical task list
   - Add empty state handling
   - Keep keyboard accessibility

2. **Swimlane.css changes**:
   - Match `.swimlane`, `.swimlane-header` from board.html
   - Add `.swimlane-icon`, `.swimlane-info`, `.swimlane-meta` classes
   - Add `.swimlane-progress`, `.swimlane-progress-fill` classes
   - Add collapse animation via CSS transition
   - Add color variants (purple, green, amber)

3. **Board.tsx updates**:
   - Update prop names: `collapsed` -> `isCollapsed`, `onToggleCollapse` -> `onToggle`
   - Remove `columns` and `tasksByColumn` props
   - Adjust swimlane rendering logic

## Feature-Specific Analysis

### User Story

As a user viewing the Queue column, I want tasks grouped by initiative in collapsible swimlanes so that I can focus on specific initiatives and understand progress at a glance.

### Acceptance Criteria

1. Swimlane header shows initiative emoji, name, progress bar, and task count
2. Clicking header toggles swimlane between expanded/collapsed states
3. Collapsed swimlanes show only the header with rotated chevron
4. Expanded swimlanes show all tasks as TaskCard components
5. Empty swimlanes indicate "No tasks" to avoid confusion
6. Progress bar accurately reflects completion percentage
7. Animation provides smooth visual feedback during toggle (0.2s)
8. Unassigned tasks grouped under "Unassigned" swimlane with neutral styling
