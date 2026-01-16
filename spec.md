# Specification: Remove drag-and-drop from board and make task cards clickable

## Problem Statement

Task cards on the Board have drag-and-drop functionality that doesn't work properly - it shows a drag cursor but prevents intuitive click-to-navigate behavior. The drag-and-drop implementation adds complexity without providing value, as task status changes are better handled through explicit action buttons.

## Success Criteria

- [ ] TaskCard no longer has `draggable="true"` attribute
- [ ] TaskCard shows `cursor: pointer` instead of `cursor: grab/grabbing`
- [ ] Clicking anywhere on TaskCard (except action buttons) navigates to `/tasks/:id`
- [ ] Column components no longer have drag event handlers (onDragEnter, onDragLeave, onDragOver, onDrop)
- [ ] QueuedColumn no longer has drag event handlers
- [ ] Swimlane no longer has drag-related handlers for columns
- [ ] Board component no longer has `handleFlatDrop` or `handleSwimlaneDrop` functions
- [ ] All drag-related CSS classes removed (`.dragging`, `.drag-over`)
- [ ] Escalate modal removed (only triggered by drag-drop)
- [ ] Initiative change modal removed (only triggered by cross-swimlane drag)
- [ ] Unit tests updated - drag-related tests removed
- [ ] E2E tests updated - drag-related tests removed or modified
- [ ] Build passes with no errors
- [ ] All remaining tests pass

## Testing Requirements

- [ ] Unit test: TaskCard renders without draggable attribute
- [ ] Unit test: TaskCard click navigates to task detail for non-running tasks
- [ ] Unit test: TaskCard click calls onTaskClick for running tasks (transcript modal)
- [ ] Unit test: Action buttons still work and stop propagation
- [ ] E2E test: Clicking task card navigates to task detail page
- [ ] E2E test: Board renders without drag-over styling
- [ ] E2E test: Swimlane view works without drag functionality

## Scope

### In Scope

- Remove `draggable` attribute and drag handlers from TaskCard
- Remove drag event handlers from Column, QueuedColumn, Swimlane
- Remove drop handling logic from Board component
- Remove escalate modal (only used for drag-back scenario)
- Remove initiative change modal (only used for cross-swimlane drag)
- Update CSS to use pointer cursor instead of grab cursor
- Remove drag-related CSS classes (`.dragging`, `.drag-over`)
- Update/remove affected unit tests
- Update/remove affected E2E tests

### Out of Scope

- Re-implementing drag-and-drop with a library (future task)
- Adding any new functionality
- Changing task action button behavior
- Changing quick menu behavior
- Modifying other pages or components

## Technical Approach

### Files to Modify

| File | Changes |
|------|---------|
| `web/src/components/board/TaskCard.tsx` | Remove `isDragging` state, `handleDragStart`, `handleDragEnd`, `draggable` attribute, `onDragStart`, `onDragEnd` props |
| `web/src/components/board/TaskCard.css` | Change `cursor: grab` to `cursor: pointer`, remove `.dragging` class, remove `:active { cursor: grabbing }` |
| `web/src/components/board/Column.tsx` | Remove `dragOver` state, `dragCounter` state, all drag handlers (handleDragEnter, handleDragLeave, handleDragOver, handleDrop), `onDrop` prop |
| `web/src/components/board/Column.css` | Remove `.drag-over` class |
| `web/src/components/board/QueuedColumn.tsx` | Remove `dragOver` state, `dragCounter` state, all drag handlers, `onDrop` prop |
| `web/src/components/board/QueuedColumn.css` | Remove `.drag-over` class |
| `web/src/components/board/Swimlane.tsx` | Remove `createColumnDropHandler`, `handleDragOver`, `onDrop` prop, drag event attributes from columns |
| `web/src/components/board/Board.tsx` | Remove `handleFlatDrop`, `handleSwimlaneDrop`, escalate modal state/handlers, initiative change modal state/handlers, `onDrop` props from child components, `onEscalate` prop |
| `web/src/components/board/TaskCard.test.tsx` | Remove "drag and drop" describe block (4 tests) |
| `web/e2e/board.spec.ts` | Remove/update drag-related tests (~6 tests) |

### Key Changes Summary

1. **TaskCard**: Remove drag capability, keep click navigation
2. **Column/QueuedColumn**: Remove drop zone functionality, keep rendering
3. **Swimlane**: Remove column drop handlers, keep collapse/expand
4. **Board**: Remove drag state management and modals, keep view mode switching
5. **Tests**: Remove drag-related test coverage

## Bug-Specific Analysis

### Reproduction Steps
1. Navigate to Board page (`/board`)
2. Hover over any task card
3. Observe cursor shows grab icon instead of pointer
4. Try to click card to navigate - may initiate drag instead
5. Drag card to another column - nothing meaningful happens

### Current Behavior
- Task cards have `cursor: grab` and `draggable="true"`
- Clicking can accidentally start drag operation
- Dragging between columns shows modals but doesn't reliably update state
- Users expect click = navigate, not drag = action

### Expected Behavior
- Task cards have `cursor: pointer`
- Clicking navigates to task detail page
- No drag operations on the board
- Explicit action buttons (run/pause/resume/finalize) handle status changes

### Root Cause
The drag-and-drop was implemented partially - the visual feedback exists but the actual status change logic through drag is unreliable and conflicts with expected click behavior.

### Verification
1. Open Board page
2. Hover over task card - cursor should be pointer
3. Click task card - should navigate to `/tasks/{id}`
4. Running tasks should show transcript modal on click
5. Action buttons should still work correctly
6. No drag-related visual feedback should appear
