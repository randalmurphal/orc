# Specification: Create RunningColumn component for active tasks

## Problem Statement

The Board view needs a dedicated RunningColumn component to display currently executing tasks with a fixed 420px width, pulsing status indicator, and proper scrolling behavior for when more than 2 tasks are running simultaneously.

## Success Criteria

- [ ] `web/src/components/board/RunningColumn.tsx` exists and exports `RunningColumn` component
- [ ] `web/src/components/board/RunningColumn.css` exists with complete styling
- [ ] Column header displays "Running" title with pulsing indicator (purple glow, 2s pulse animation)
- [ ] Column header shows active task count in a badge
- [ ] Fixed width of 420px (`flex: 0 0 420px`)
- [ ] Body scrolls when more than 2 cards are visible (`max-height` constraint + `overflow-y: auto`)
- [ ] 12px padding on column body
- [ ] 12px gap between RunningCard components
- [ ] Empty state shows "No running tasks" message with suggestion text
- [ ] Component renders RunningCard for each task passed in props
- [ ] Props interface accepts `tasks: Task[]` and optional callbacks (`onTaskClick`, `onToggleExpand`)
- [ ] Maintains expanded state per card (tracks which card ID is expanded)
- [ ] Preserves scroll position when task list updates
- [ ] Task completion triggers fade-out animation (300ms scale-out)
- [ ] `npm run typecheck` exits 0

## Testing Requirements

- [ ] Unit test: RunningColumn renders with tasks array
- [ ] Unit test: Empty state renders when tasks array is empty
- [ ] Unit test: Count badge shows correct number
- [ ] Unit test: Click handler is called when card is clicked
- [ ] Unit test: Only one card can be expanded at a time
- [ ] Snapshot test: RunningColumn with 2 tasks
- [ ] Snapshot test: RunningColumn empty state

## Scope

### In Scope

- RunningColumn component with header and scrollable body
- CSS styling matching reference design (example_ui/board.html lines 247-295)
- Pulsing column indicator animation
- Empty state with helpful message
- Integration with existing RunningCard component
- Expand/collapse state management (one card at a time)
- Scroll preservation on updates

### Out of Scope

- WebSocket subscription logic (handled by parent Board component)
- Task filtering logic (parent passes filtered tasks)
- Output line streaming (RunningCard handles this)
- Task completion detection (parent handles via status changes)
- Drag and drop reordering

## Technical Approach

### Files to Create

- `web/src/components/board/RunningColumn.tsx`: Main component
- `web/src/components/board/RunningColumn.css`: Styles
- `web/src/components/board/RunningColumn.test.tsx`: Unit tests

### Component Structure

```tsx
interface RunningColumnProps {
  tasks: Task[];
  taskStates?: Record<string, TaskState>;
  outputLines?: Record<string, string[]>;
  onTaskClick?: (task: Task) => void;
}

function RunningColumn({ tasks, taskStates, outputLines, onTaskClick })
```

### Key Implementation Details

1. **State Management**:
   - `expandedTaskId: string | null` - tracks which card is expanded
   - Toggle collapses current card before expanding new one

2. **Scroll Preservation**:
   - Use `useRef` to hold scrollable container reference
   - Store `scrollTop` before render, restore after if task count unchanged

3. **Animation for Completion**:
   - CSS class `.running-card-exiting` triggers `scale-out` animation
   - Parent component responsible for removing task after animation completes
   - Use `onAnimationEnd` to signal completion

4. **Styling (from reference)**:
   - Column: `width: 420px; min-width: 420px; flex-shrink: 0`
   - Header: `padding: 10px 12px; background: var(--bg-elevated); border-bottom: 1px solid var(--border)`
   - Indicator: `width: 8px; height: 8px; border-radius: 2px; background: var(--primary); animation: pulse 2s ease-in-out infinite`
   - Body: `flex: 1; overflow-y: auto; padding: 12px`
   - Max visible height calculated as: `2 * card_height + gap` (~340px based on RunningCard size)

5. **Empty State**:
   - Centered vertically
   - "No running tasks" in muted text
   - Smaller helper text: "Use 'orc run' to start a task"

## Feature Analysis

### User Story

As a developer using the orc Board view, I want to see my currently running tasks in a dedicated column so that I can monitor their progress, view their pipeline status, and expand individual tasks to see their output.

### Acceptance Criteria

1. **Visual Design**:
   - Matches screenshot reference (center column)
   - Purple pulsing indicator distinguishes from static columns
   - Cards have gradient background per RunningCard design

2. **Interaction**:
   - Clicking a card expands/collapses its output section
   - Only one card expanded at a time
   - Smooth scroll when content exceeds 2 cards

3. **Real-time Updates**:
   - New running tasks appear without losing scroll position
   - Completed tasks animate out gracefully
   - Count badge updates immediately

4. **Accessibility**:
   - Column has `role="region"` with `aria-label`
   - Count badge is not announced separately (decorative)
   - Focus management delegates to RunningCard components
