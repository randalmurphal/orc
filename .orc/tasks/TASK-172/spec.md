# Phase 2: React - Board Page (Flat and Swimlane Views)

## Overview

Port the Svelte Kanban board to React, implementing both flat and swimlane view modes with full drag-drop functionality for task status changes and initiative reassignment.

## Scope

### In Scope
- Board page component (`/board` route)
- Column components for 6 fixed columns (Queued, Spec, Implement, Test, Review, Done)
- QueuedColumn with active/backlog split
- TaskCard for kanban (draggable, action buttons, finalize states)
- Swimlane component for initiative grouping
- Drag-drop for status changes (column moves)
- Drag-drop for initiative changes (swimlane moves with confirmation)
- View mode toggle (flat/swimlane) with localStorage persistence
- Confirmation modals for actions (run, pause, resume, escalate)
- Filter bar with initiative dropdown

### Out of Scope
- NewTaskModal (already exists or separate task)
- CommandPalette (already exists)
- LiveTranscriptModal (separate component)
- FinalizeModal (separate component - just trigger it)
- Task detail page navigation

## Technical Approach

### Drag-Drop Strategy

**Decision: Use HTML5 native drag-drop API** (same as Svelte implementation)

Rationale:
- Consistent with existing Svelte behavior
- No additional dependencies
- Sufficient for column-to-column moves
- Simpler than @dnd-kit or react-beautiful-dnd for this use case

Implementation pattern:
```tsx
// TaskCard: drag source
<div
  draggable="true"
  onDragStart={(e) => {
    e.dataTransfer.setData('application/json', JSON.stringify(task));
    e.dataTransfer.effectAllowed = 'move';
    setIsDragging(true);
  }}
  onDragEnd={() => setIsDragging(false)}
>
```

```tsx
// Column: drop target
<div
  onDragOver={(e) => e.preventDefault()}
  onDragEnter={(e) => { dragCounter.current++; setDragOver(true); }}
  onDragLeave={(e) => { if (--dragCounter.current === 0) setDragOver(false); }}
  onDrop={(e) => {
    e.preventDefault();
    setDragOver(false);
    const task = JSON.parse(e.dataTransfer.getData('application/json'));
    onDrop(task);
  }}
>
```

### Component Architecture

```
web-react/src/
├── pages/
│   └── Board.tsx              # Main page (existing, replace placeholder)
├── components/
│   └── board/                 # NEW directory
│       ├── index.ts           # Exports
│       ├── KanbanBoard.tsx    # Main board orchestrator
│       ├── Column.tsx         # Standard column
│       ├── QueuedColumn.tsx   # Special first column with backlog
│       ├── Swimlane.tsx       # Initiative row
│       ├── SwimlaneHeaders.tsx # Column headers for swimlane view
│       ├── TaskCard.tsx       # Draggable task card (board-specific)
│       ├── ConfirmModal.tsx   # Action confirmation
│       ├── EscalateModal.tsx  # Escalation reason capture
│       └── InitiativeChangeModal.tsx # Swimlane initiative reassignment
└── lib/
    └── board-utils.ts         # Column placement logic, sorting
```

### State Management

**Use existing Zustand stores:**

| Store | Usage |
|-------|-------|
| `useTaskStore` | Tasks, task states, updates |
| `useInitiativeStore` | Initiative filter, progress |
| `useProjectStore` | Current project for API calls |
| `useUIStore` | Toast notifications |

**Local component state:**

| State | Location | Persistence |
|-------|----------|-------------|
| `viewMode` | Board.tsx | localStorage `orc-board-view-mode` |
| `showBacklog` | Board.tsx | localStorage `orc-show-backlog` |
| `collapsedSwimlanes` | Board.tsx | localStorage `orc-collapsed-swimlanes` |
| `confirmModal` | Board.tsx | None |
| `escalateModal` | Board.tsx | None |
| `initiativeChangeModal` | Board.tsx | None |
| `actionLoading` | Board.tsx | None |

### Column Logic

**6 fixed columns:**

| Column | ID | Phases |
|--------|-----|--------|
| Queued | `queued` | - (terminal: created, classifying, planned) |
| Spec | `spec` | research, spec, design |
| Implement | `implement` | implement |
| Test | `test` | test |
| Review | `review` | docs, validate, review |
| Done | `done` | - (terminal: finalizing, completed, finished, failed) |

**Task placement algorithm (`getTaskColumn`):**

```typescript
function getTaskColumn(task: Task): string {
  // Terminal statuses
  if (['finalizing', 'completed', 'finished', 'failed'].includes(task.status)) {
    return 'done';
  }

  // Non-started statuses
  if (['created', 'classifying', 'planned'].includes(task.status)) {
    return 'queued';
  }

  // Running without phase (edge case from TASK-041)
  if (task.status === 'running' && !task.current_phase) {
    return 'implement';
  }

  // Map phase to column
  const phase = task.current_phase;
  if (['research', 'spec', 'design'].includes(phase)) return 'spec';
  if (phase === 'implement') return 'implement';
  if (phase === 'test') return 'test';
  if (['docs', 'validate', 'review'].includes(phase)) return 'review';

  // Default
  return 'implement';
}
```

**Sorting within columns:**

```typescript
function sortTasks(tasks: Task[]): Task[] {
  return [...tasks].sort((a, b) => {
    // Running tasks first
    const aRunning = a.status === 'running' ? 0 : 1;
    const bRunning = b.status === 'running' ? 0 : 1;
    if (aRunning !== bRunning) return aRunning - bRunning;

    // Then by priority
    return PRIORITY_ORDER[a.priority || 'normal'] - PRIORITY_ORDER[b.priority || 'normal'];
  });
}
```

### API Integration

**Required API calls:**

| Action | Endpoint | When |
|--------|----------|------|
| Run task | `POST /api/projects/:id/tasks/:taskId/run` | Drop on phase column (created/planned) |
| Pause task | `POST /api/projects/:id/tasks/:taskId/pause` | Pause button or escalation target |
| Resume task | `POST /api/projects/:id/tasks/:taskId/resume` | Drop on phase column (paused) |
| Escalate task | `POST /api/projects/:id/tasks/:taskId/escalate` | Drop backward (e.g., Review → Implement) |
| Update task | `PATCH /api/projects/:id/tasks/:taskId` | Initiative change in swimlane |
| Trigger finalize | `POST /api/tasks/:taskId/finalize` | Finalize button click |

### View Modes

**Flat view (default):**
- 6 columns side by side
- All tasks grouped by column
- Standard drag-drop between columns

**Swimlane view:**
- Horizontal swimlanes grouped by initiative
- Each swimlane contains 6 columns
- Swimlane header with collapse/expand, progress bar
- "Unassigned" swimlane at bottom for tasks without initiative
- Cross-swimlane drag = initiative change (with confirmation)

**View mode toggle:**
- Dropdown in page header
- Options: "Flat" | "By Initiative"
- Persisted in localStorage
- Disabled when initiative filter is active (swimlanes don't make sense when filtering to one initiative)

### Modal Flows

**ConfirmModal** (run/pause/resume):
```
User drops task → Determine action → Show modal → User confirms → Execute API call → Toast result
```

**EscalateModal** (backward drop):
```
User drops task backward → Show escalate modal with reason textarea → User enters reason → Execute API call → Toast result
```

**InitiativeChangeModal** (swimlane cross-drop):
```
User drops task in different swimlane → Show initiative change modal → User confirms → Update task initiative_id → Toast result
```

### CSS Classes Required

Match Svelte implementation for E2E test compatibility:

| Class | Element | Purpose |
|-------|---------|---------|
| `.board-page` | Page container | E2E selector |
| `.page-header` | Header area | Contains task count |
| `.task-count` | Count badge | "X tasks" display |
| `.board` | Flat board container | E2E selector |
| `.swimlane-view` | Swimlane container | E2E selector |
| `.column` | Column container | E2E selector |
| `.column-header` | Column header | Contains title + count |
| `.task-card` | Task card | E2E selector |
| `.task-id` | Task ID badge | E2E selector |
| `.swimlane` | Swimlane row | E2E selector |
| `.swimlane.collapsed` | Collapsed state | E2E selector |
| `.swimlane-header` | Swimlane header | Click to collapse |
| `.swimlane-title` | Initiative title | Text content |
| `.swimlane-content` | Swimlane columns | Hidden when collapsed |
| `.swimlane-headers` | Column headers in swimlane view | Fixed header row |
| `.view-mode-dropdown` | View toggle | E2E selector |
| `.view-mode-disabled` | Disabled state wrapper | When initiative filter active |
| `.initiative-dropdown` | Initiative filter | E2E selector |
| `.initiative-banner` | Active filter banner | Shows when filtering |
| `.banner-clear` | Clear filter button | "Clear filter" text |
| `.confirm-modal` | Confirmation dialog | E2E selector |
| `.status-indicator` | Status orb | E2E selector |
| `.status-indicator.paused` | Paused state | E2E selector |
| `.loading-state` | Loading spinner | E2E selector |

**ARIA attributes for accessibility:**

| Attribute | Element | Value |
|-----------|---------|-------|
| `role="region"` | Column | Landmark |
| `aria-label` | Column | "{Name} column" |
| `role="listbox"` | Dropdown menu | For keyboard nav |
| `role="dialog"` | Confirm modal | For screen readers |

## Component Specifications

### 1. Board.tsx (Page)

**Props:** None (page component)

**State:**
- `viewMode: 'flat' | 'swimlane'` (localStorage)
- `showBacklog: boolean` (localStorage)
- `collapsedSwimlanes: Set<string>` (localStorage)
- `confirmModal: { task: Task; action: string; targetColumn: string } | null`
- `escalateModal: { task: Task; targetColumn: string } | null`
- `initiativeChangeModal: { task: Task; targetInitiativeId: string | null; columnId: string } | null`
- `escalateReason: string`
- `actionLoading: boolean`

**Responsibilities:**
- Fetch tasks via `useTaskStore`
- Filter by current initiative from `useInitiativeStore`
- Compute `tasksByColumn` derived state
- Render KanbanBoard or SwimlanesView based on viewMode
- Handle all modal states
- Handle all API actions

### 2. KanbanBoard.tsx

**Props:**
```typescript
interface Props {
  tasksByColumn: Record<string, Task[]>;
  onDrop: (columnId: string, task: Task) => void;
  onTaskAction: (taskId: string, action: 'run' | 'pause' | 'resume') => void;
  onTaskClick?: (task: Task) => void;
  onFinalizeClick?: (task: Task) => void;
  showBacklog: boolean;
  onBacklogToggle: () => void;
}
```

**Responsibilities:**
- Render 6 columns in flex row
- Pass drop handlers to columns
- QueuedColumn for first column
- Regular Column for others

### 3. Column.tsx

**Props:**
```typescript
interface Props {
  id: string;
  title: string;
  tasks: Task[];
  onDrop: (task: Task) => void;
  onTaskAction: (taskId: string, action: 'run' | 'pause' | 'resume') => void;
  onTaskClick?: (task: Task) => void;
  onFinalizeClick?: (task: Task) => void;
  accentColor?: string;
}
```

**Responsibilities:**
- Column header with title and count
- Drop zone with visual feedback
- Render TaskCards
- Handle drag-over state

### 4. QueuedColumn.tsx

**Props:**
```typescript
interface Props {
  activeTasks: Task[];
  backlogTasks: Task[];
  showBacklog: boolean;
  onBacklogToggle: () => void;
  onDrop: (task: Task, queue: 'active' | 'backlog') => void;
  onTaskAction: (taskId: string, action: 'run' | 'pause' | 'resume') => void;
  onTaskClick?: (task: Task) => void;
  onFinalizeClick?: (task: Task) => void;
}
```

**Responsibilities:**
- Active section (always visible)
- Backlog section (collapsible)
- Toggle button with count
- Separate drop zones for active/backlog

### 5. TaskCard.tsx (Board-specific)

**Props:**
```typescript
interface Props {
  task: Task;
  onAction: (action: 'run' | 'pause' | 'resume') => void;
  onClick?: () => void;
  onFinalizeClick?: () => void;
  finalizeState?: FinalizeState | null;
}
```

**Visual states:**
- Default: Standard card
- Running: Pulsing border, gradient background
- Finalizing: Info border, progress bar, step label
- Finished: Success border, commit SHA, target branch

**Features:**
- Draggable
- Task ID badge
- Title (truncated)
- Description preview (truncated)
- Weight badge
- Priority indicator (critical=red pulse, high=orange arrow, low=gray arrow)
- Blocked badge with count
- Initiative badge (truncated, clickable)
- Relative update time
- Phase display (if running)
- Action buttons (status-dependent)
- Quick menu for queue/priority changes

### 6. Swimlane.tsx

**Props:**
```typescript
interface Props {
  initiative: Initiative | null;  // null = Unassigned
  tasksByColumn: Record<string, Task[]>;
  collapsed: boolean;
  onToggle: () => void;
  onDrop: (columnId: string, task: Task) => void;
  onTaskAction: (taskId: string, action: 'run' | 'pause' | 'resume') => void;
  onTaskClick?: (task: Task) => void;
  onFinalizeClick?: (task: Task) => void;
  progress: { completed: number; total: number };
}
```

**Features:**
- Collapsible header with chevron
- Initiative title (or "Unassigned")
- Task count and progress bar
- 6 columns when expanded
- Smooth collapse animation

### 7. ConfirmModal.tsx

**Props:**
```typescript
interface Props {
  open: boolean;
  task: Task;
  action: 'run' | 'pause' | 'resume';
  loading: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}
```

**Features:**
- Action-specific title and message
- Keyboard support (Enter to confirm, Escape to cancel)
- Loading state during API call

### 8. EscalateModal.tsx

**Props:**
```typescript
interface Props {
  open: boolean;
  task: Task;
  targetColumn: string;
  reason: string;
  onReasonChange: (reason: string) => void;
  loading: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}
```

**Features:**
- Explanation of escalation
- Reason textarea (required)
- Keyboard support

### 9. InitiativeChangeModal.tsx

**Props:**
```typescript
interface Props {
  open: boolean;
  task: Task;
  fromInitiative: Initiative | null;
  toInitiative: Initiative | null;
  loading: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}
```

**Features:**
- Shows current and target initiative
- Keyboard support

## Success Criteria

### Functional Requirements

- [ ] Board page renders at `/board` route
- [ ] 6 columns display correctly (Queued, Spec, Implement, Test, Review, Done)
- [ ] Tasks appear in correct columns based on status/phase
- [ ] Tasks sorted by running status first, then priority
- [ ] Running tasks show pulsing animation
- [ ] Drag-drop between columns works
- [ ] Drop triggers appropriate action (run/pause/resume/escalate)
- [ ] Confirmation modal appears before actions
- [ ] Escalation modal captures reason for backward moves
- [ ] View mode toggle switches between flat and swimlane
- [ ] View mode persists in localStorage
- [ ] Swimlane view groups tasks by initiative
- [ ] Swimlanes can collapse/expand
- [ ] Collapsed state persists in localStorage
- [ ] Cross-swimlane drag triggers initiative change modal
- [ ] Initiative change updates task correctly
- [ ] Backlog section in Queued column works
- [ ] Backlog toggle persists in localStorage
- [ ] Initiative filter dropdown works
- [ ] Initiative banner shows when filtering
- [ ] Clear filter button works
- [ ] View mode disabled when initiative filter active
- [ ] Finalize button visible on completed tasks
- [ ] Finalize button triggers FinalizeModal
- [ ] Finalizing tasks show progress bar
- [ ] Finished tasks show commit SHA and target branch

### Non-Functional Requirements

- [ ] All existing E2E tests in `web/e2e/board.spec.ts` pass
- [ ] No console errors during normal operation
- [ ] Responsive layout (horizontal scroll on small screens)
- [ ] Keyboard accessible (Tab navigation, Enter to confirm)
- [ ] Screen reader accessible (ARIA attributes)

## Testing Strategy

### Unit Tests (Vitest)

**board-utils.test.ts:**
- `getTaskColumn()` - 9 test cases for phase-based placement
- `sortTasks()` - 5 test cases for priority sorting
- `groupTasksByInitiative()` - Test grouping logic

**TaskCard.test.tsx:**
- Renders task info correctly
- Shows correct action buttons based on status
- Shows priority indicator
- Shows blocked badge
- Shows running animation
- Shows finalizing progress
- Shows finished merge info

**Column.test.tsx:**
- Renders header and count
- Renders task cards
- Shows drop zone feedback
- Handles drag events

**QueuedColumn.test.tsx:**
- Renders active and backlog sections
- Backlog collapse/expand works
- Shows correct counts

**Swimlane.test.tsx:**
- Renders initiative header
- Shows progress bar
- Collapse/expand works
- Renders columns when expanded

**KanbanBoard.test.tsx:**
- Renders all 6 columns
- Task distribution correct

### Integration Tests

**Board.test.tsx:**
- View mode persistence
- Backlog toggle persistence
- Swimlane collapse persistence
- API calls triggered on actions
- Toast notifications on success/error

### E2E Tests (Playwright)

**Existing tests in `web/e2e/board.spec.ts` must pass:**

1. Board Rendering (4 tests)
   - All 6 columns visible
   - Column headers with counts
   - Task cards in correct columns
   - Task count in header

2. View Mode Toggle (5 tests)
   - Default to flat view
   - Switch to swimlane view
   - Persist view mode
   - Disable when initiative filter active
   - Show initiative banner when filtering

3. Drag-Drop (5 tests)
   - Move task between columns
   - Reorder within column
   - Visual feedback during drag
   - Update status after drop
   - Cancel drop with Escape

4. Swimlane View (4 tests)
   - Group tasks by initiative
   - Collapse/expand swimlanes
   - Persist collapsed state
   - Show Unassigned swimlane

## File Changes

### New Files

| File | Lines (est.) | Purpose |
|------|--------------|---------|
| `components/board/index.ts` | 10 | Exports |
| `components/board/KanbanBoard.tsx` | 120 | Main board |
| `components/board/KanbanBoard.css` | 100 | Board styles |
| `components/board/Column.tsx` | 100 | Column component |
| `components/board/Column.css` | 80 | Column styles |
| `components/board/QueuedColumn.tsx` | 150 | Queued column |
| `components/board/QueuedColumn.css` | 60 | Queued styles |
| `components/board/TaskCard.tsx` | 300 | Board task card |
| `components/board/TaskCard.css` | 200 | Card styles |
| `components/board/Swimlane.tsx` | 150 | Swimlane row |
| `components/board/Swimlane.css` | 100 | Swimlane styles |
| `components/board/SwimlaneHeaders.tsx` | 50 | Column headers |
| `components/board/ConfirmModal.tsx` | 80 | Confirm dialog |
| `components/board/EscalateModal.tsx` | 100 | Escalate dialog |
| `components/board/InitiativeChangeModal.tsx` | 80 | Initiative change dialog |
| `lib/board-utils.ts` | 80 | Column/sorting logic |
| `lib/board-utils.test.ts` | 150 | Utils tests |

### Modified Files

| File | Change |
|------|--------|
| `pages/Board.tsx` | Replace placeholder with full implementation |

**Total new code:** ~1,910 lines estimated

## Dependencies

### Existing (no new packages needed)

- React 19
- React Router 7
- Zustand 5
- Existing Modal component
- Existing Icon component
- Existing StatusIndicator component
- Existing toast API

### API Endpoints Used

All endpoints already exist:
- `GET /api/projects/:id/tasks`
- `POST /api/projects/:id/tasks/:taskId/run`
- `POST /api/projects/:id/tasks/:taskId/pause`
- `POST /api/projects/:id/tasks/:taskId/resume`
- `POST /api/projects/:id/tasks/:taskId/escalate`
- `PATCH /api/projects/:id/tasks/:taskId`
- `POST /api/tasks/:taskId/finalize`
- `GET /api/initiatives`

## Implementation Order

1. **Core utilities** (`lib/board-utils.ts`)
   - Column placement logic
   - Sorting logic
   - Unit tests

2. **TaskCard** (`components/board/TaskCard.tsx`)
   - Basic card rendering
   - Visual states (running, finalizing, finished)
   - Drag source implementation
   - Unit tests

3. **Column** (`components/board/Column.tsx`)
   - Basic column with header
   - Drop target implementation
   - Unit tests

4. **QueuedColumn** (`components/board/QueuedColumn.tsx`)
   - Active/backlog split
   - Collapsible backlog
   - Unit tests

5. **KanbanBoard** (`components/board/KanbanBoard.tsx`)
   - Render 6 columns
   - Integration tests

6. **Board page flat view** (`pages/Board.tsx`)
   - Replace placeholder
   - State management
   - API integration
   - E2E tests (flat view subset)

7. **Swimlane** (`components/board/Swimlane.tsx`)
   - Initiative row
   - Collapse/expand
   - Unit tests

8. **Board page swimlane view**
   - View mode toggle
   - Swimlane grouping
   - Cross-swimlane drag-drop
   - E2E tests (swimlane subset)

9. **Modals** (ConfirmModal, EscalateModal, InitiativeChangeModal)
   - Confirmation flows
   - Keyboard support

10. **Final polish**
    - Full E2E test suite pass
    - Accessibility audit
    - Performance check

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| HTML5 drag-drop quirks | Follow exact Svelte implementation pattern |
| E2E test flakiness | Use Svelte E2E test helpers and wait patterns |
| localStorage conflicts | Use same keys as Svelte for compatibility |
| WebSocket event handling | Leverage existing WebSocketProvider |
| CSS class mismatches | Reference E2E tests for exact class names |

## Open Questions

None - specification is complete based on existing Svelte implementation analysis.
