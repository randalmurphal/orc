# Phase 2: React - Initiative Detail Page

## Overview

Port the Initiative Detail page from Svelte (`web/src/routes/initiatives/[id]/+page.svelte`) to React (`web-react/src/pages/InitiativeDetail.tsx`). This page manages initiatives with three tabs (Tasks, Decisions, Graph) and supports status transitions, task linking, decision recording, and interactive dependency visualization.

## Requirements and Scope

### Core Features to Port
1. **Initiative Header** - Title, status badge, progress bar, metadata grid
2. **Status Management** - Status transitions (draft → active → completed/archived) with confirmation
3. **Tasks Tab** - Linked tasks list, add/link/unlink tasks, status indicators
4. **Decisions Tab** - Decision list with dates/authors, add decision form
5. **Graph Tab** - Interactive DAG visualization with zoom/pan, export to PNG

### Out of Scope
- Initiative creation (handled by NewInitiativeModal in sidebar)
- Initiative deletion (handled by archive + delete elsewhere)
- Initiative dependencies (blocked_by for initiative-to-initiative - different from task dependencies shown in Graph)

## Technical Approach

### Architecture

```
web-react/src/
├── pages/
│   └── InitiativeDetail.tsx          # Main page component with tabs
├── components/initiative/
│   ├── InitiativeHeader.tsx          # Header with title, status, progress
│   ├── InitiativeTasksTab.tsx        # Tasks list and management
│   ├── InitiativeDecisionsTab.tsx    # Decisions list and form
│   ├── InitiativeGraphTab.tsx        # Graph wrapper with loading state
│   ├── EditInitiativeModal.tsx       # Edit title/vision/status
│   ├── LinkTaskModal.tsx             # Search and link existing tasks
│   └── AddDecisionModal.tsx          # Add decision form
├── components/visualization/
│   └── DependencyGraph.tsx           # Reusable SVG graph component
└── lib/utils/
    └── graph-layout.ts               # Copy from web/ (pure JS, no changes)
```

### State Management

Use local React state (`useState`) for:
- Active tab (`'tasks' | 'graph' | 'decisions'`)
- Modal open states (edit, linkTask, addDecision, confirmArchive)
- Form field values
- Loading/error states

Use existing Zustand stores for:
- `useTaskStore` - Task data for linking
- `useProjectStore` - Current project context
- `useUIStore` - Toast notifications

### Data Flow

1. **Load Initiative**: `getInitiative(id)` on mount and after mutations
2. **Load Graph Data**: `getInitiativeDependencyGraph(id)` lazily when Graph tab selected
3. **Load Available Tasks**: `listTasks()` when LinkTaskModal opens
4. **Mutations**: Call API, show toast, refetch initiative

### API Functions (Already Implemented in api.ts)

| Function | Purpose |
|----------|---------|
| `getInitiative(id)` | Load initiative with tasks/decisions |
| `updateInitiative(id, req)` | Edit title/vision/status |
| `addInitiativeTask(id, {task_id})` | Link task to initiative |
| `removeInitiativeTask(id, taskId)` | Unlink task |
| `addInitiativeDecision(id, req)` | Add decision |
| `getInitiativeDependencyGraph(id)` | Load dependency graph |
| `listTasks()` | Get all tasks for linking |

## Component Breakdown

### 1. InitiativeDetail.tsx (Main Page)

**Responsibilities:**
- Route handling (`useParams`, `useSearchParams`)
- Initiative data loading and error handling
- Tab navigation with URL sync (`?tab=tasks`)
- Pass initiative data to child components

**State:**
```typescript
const [initiative, setInitiative] = useState<Initiative | null>(null)
const [loading, setLoading] = useState(true)
const [error, setError] = useState<string | null>(null)
const [tab, setTab] = useSearchParams('tab', 'tasks')
```

### 2. InitiativeHeader.tsx

**Props:**
```typescript
interface InitiativeHeaderProps {
  initiative: Initiative
  onEdit: () => void
  onStatusChange: (status: InitiativeStatus) => Promise<void>
  onArchive: () => void
}
```

**Features:**
- Breadcrumbs: Dashboard > Initiatives > {title}
- Title with status badge (draft/active/completed/archived)
- Progress bar: "X of Y tasks completed (Z%)"
- Metadata: Owner, Status, Created date
- Vision statement (if present)
- Action buttons: Edit, Status transitions, Archive

**Status Transition Logic:**
| Current | Available Actions |
|---------|------------------|
| draft | Activate |
| active | Complete |
| completed | Reopen (→ active) |
| archived | (none) |

### 3. InitiativeTasksTab.tsx

**Props:**
```typescript
interface InitiativeTasksTabProps {
  tasks: InitiativeTaskRef[]
  onAddTask: () => void
  onLinkTask: () => void
  onUnlinkTask: (taskId: string) => Promise<void>
}
```

**Features:**
- Task list with status indicators (icon + color + text)
- Click task to navigate to `/tasks/:id`
- Unlink button per task (with confirmation)
- Empty state with "Add Task" CTA
- Dependencies section showing blocked_by within initiative

**Status Colors:**
| Status | Color | Icon |
|--------|-------|------|
| completed/finished | green | ✓ |
| running | blue (pulsing) | ▶ |
| blocked | red | ⊘ |
| paused | yellow | ⏸ |
| failed | red | ✕ |
| created/planned | gray | ○ |

### 4. InitiativeDecisionsTab.tsx

**Props:**
```typescript
interface InitiativeDecisionsTabProps {
  decisions: InitiativeDecision[]
  onAddDecision: () => void
}
```

**Features:**
- Decision list sorted by date (newest first)
- Each decision shows: date, author, decision text, rationale (if any)
- "Add Decision" button opens modal
- Empty state with "Record Decision" CTA

### 5. InitiativeGraphTab.tsx

**Props:**
```typescript
interface InitiativeGraphTabProps {
  initiativeId: string
}
```

**Features:**
- Lazy-load graph data on tab selection
- Loading spinner during fetch
- Error state with retry button
- Empty state when no dependencies exist
- Renders DependencyGraph component

### 6. DependencyGraph.tsx (Visualization Component)

**Props:**
```typescript
interface DependencyGraphProps {
  nodes: DependencyGraphNode[]
  edges: DependencyGraphEdge[]
  onNodeClick?: (nodeId: string) => void
}
```

**Features:**
- SVG-based rendering with computed layout
- Interactive pan (mouse drag) and zoom (wheel scroll)
- Zoom controls: +, -, Fit to View
- Node click navigates to task or calls callback
- Node hover shows tooltip with title + status
- Legend showing status colors
- Export to PNG button

**Implementation Notes:**
- Use `useRef` for SVG element and transform state
- Use `useState` for zoom/pan values
- Use `useMemo` for layout computation
- Event handlers: onMouseDown/Move/Up for pan, onWheel for zoom
- Use `html2canvas` or native canvas API for PNG export

### 7. EditInitiativeModal.tsx

**Props:**
```typescript
interface EditInitiativeModalProps {
  initiative: Initiative
  open: boolean
  onClose: () => void
  onSave: (req: UpdateInitiativeRequest) => Promise<void>
}
```

**Form Fields:**
- Title (text input, required)
- Vision (textarea, optional)
- Status (select: draft/active/completed/archived)

### 8. LinkTaskModal.tsx

**Props:**
```typescript
interface LinkTaskModalProps {
  initiativeId: string
  linkedTaskIds: string[]
  open: boolean
  onClose: () => void
  onLink: (taskId: string) => Promise<void>
}
```

**Features:**
- Search/filter tasks by title
- Filter out already-linked tasks
- Click task to link
- Loading state while linking

### 9. AddDecisionModal.tsx

**Props:**
```typescript
interface AddDecisionModalProps {
  open: boolean
  onClose: () => void
  onAdd: (decision: string, rationale?: string, by?: string) => Promise<void>
}
```

**Form Fields:**
- Decision (textarea, required)
- Rationale (textarea, optional)
- By (text input, optional, defaults to "user")

## Success Criteria

### Functional Requirements

- [ ] Initiative loads and displays correctly
- [ ] All three tabs render and switch properly
- [ ] Tab selection persists in URL (`?tab=tasks`)
- [ ] Status transitions work with confirmation
- [ ] Edit modal saves changes
- [ ] Tasks can be linked and unlinked
- [ ] Task click navigates to task detail
- [ ] Decisions can be added
- [ ] Dependency graph renders with correct layout
- [ ] Graph supports pan and zoom
- [ ] Graph node click navigates to task
- [ ] Graph export to PNG works
- [ ] Loading states show during data fetches
- [ ] Error states display with retry option
- [ ] Empty states show for no tasks/decisions/dependencies
- [ ] Toast notifications for success/error actions

### Non-Functional Requirements

- [ ] No console errors or warnings
- [ ] TypeScript compiles without errors
- [ ] Components are properly typed
- [ ] Follows existing React patterns in codebase
- [ ] CSS matches Svelte implementation styling

## Testing Strategy

### Unit Tests (Vitest)

**Files to Create:**
- `InitiativeDetail.test.tsx`
- `InitiativeHeader.test.tsx`
- `InitiativeTasksTab.test.tsx`
- `InitiativeDecisionsTab.test.tsx`
- `InitiativeGraphTab.test.tsx`
- `DependencyGraph.test.tsx`

**Test Categories:**
1. **Rendering**: Component renders with mock data
2. **Loading States**: Shows spinner during loading
3. **Error States**: Shows error message on failure
4. **Empty States**: Shows empty state when no data
5. **User Interactions**: Tab switches, button clicks, modal opens

**Example Test Cases:**
```typescript
// InitiativeDetail.test.tsx
describe('InitiativeDetail', () => {
  it('renders initiative header with title and status')
  it('shows loading spinner while fetching')
  it('shows error state on fetch failure')
  it('switches tabs when clicked')
  it('syncs tab to URL param')
})

// DependencyGraph.test.tsx
describe('DependencyGraph', () => {
  it('renders nodes with correct status colors')
  it('renders edges between connected nodes')
  it('calls onNodeClick when node clicked')
  it('zooms on wheel scroll')
  it('pans on mouse drag')
})
```

### Integration Tests

**Test via Mock API:**
1. Load initiative and verify all fields display
2. Change status and verify API called
3. Link task and verify task appears in list
4. Add decision and verify decision appears

### E2E Tests (Playwright)

**File:** `web-react/e2e/initiative-detail.spec.ts`

**Test Scenarios:**

```typescript
// Navigation
test('navigates to initiative detail from sidebar')
test('shows correct breadcrumbs')
test('tabs switch correctly with URL update')

// Tasks Tab
test('displays linked tasks with status indicators')
test('opens link task modal')
test('links a task successfully')
test('unlinks a task with confirmation')
test('navigates to task detail on click')

// Decisions Tab
test('displays decisions with date and author')
test('opens add decision modal')
test('adds a decision successfully')

// Graph Tab
test('lazy-loads graph data on tab select')
test('renders dependency graph with nodes')
test('zooms graph with controls')
test('navigates to task on node click')
test('exports graph to PNG')

// Status Management
test('activates draft initiative')
test('completes active initiative')
test('reopens completed initiative')
test('shows confirmation before archive')

// Edit Modal
test('opens edit modal')
test('saves edited title')
test('saves edited vision')
```

### Existing E2E Tests to Pass

The following tests from Phase 0 should pass (assuming they exist in `web/e2e/`):
- Initiative detail navigation
- Initiative status changes
- Initiative task management
- Initiative decision recording

## File Checklist

### Code Files to Create

| File | Lines (est.) | Complexity |
|------|--------------|------------|
| `pages/InitiativeDetail.tsx` | 150 | Medium |
| `components/initiative/InitiativeHeader.tsx` | 120 | Low |
| `components/initiative/InitiativeTasksTab.tsx` | 100 | Low |
| `components/initiative/InitiativeDecisionsTab.tsx` | 80 | Low |
| `components/initiative/InitiativeGraphTab.tsx` | 60 | Low |
| `components/initiative/EditInitiativeModal.tsx` | 100 | Low |
| `components/initiative/LinkTaskModal.tsx` | 120 | Medium |
| `components/initiative/AddDecisionModal.tsx` | 80 | Low |
| `components/visualization/DependencyGraph.tsx` | 300 | High |
| `lib/utils/graph-layout.ts` | 200 | Medium (copy) |

### Test Files to Create

| File | Tests (est.) |
|------|--------------|
| `InitiativeDetail.test.tsx` | 8 |
| `InitiativeHeader.test.tsx` | 5 |
| `InitiativeTasksTab.test.tsx` | 6 |
| `InitiativeDecisionsTab.test.tsx` | 4 |
| `InitiativeGraphTab.test.tsx` | 5 |
| `DependencyGraph.test.tsx` | 8 |
| `e2e/initiative-detail.spec.ts` | 15 |

### CSS Files

Use existing CSS patterns from other React pages. Component-specific styles inline or in shared stylesheets.

## Dependencies

### Existing (No New Dependencies)
- React 19 + hooks
- react-router-dom (routing, useParams, useSearchParams)
- Existing Modal component (`components/overlays/Modal.tsx`)
- Existing Breadcrumbs component (`components/ui/Breadcrumbs.tsx`)
- Existing StatusIndicator component (`components/ui/StatusIndicator.tsx`)
- Existing API functions in `lib/api.ts`
- Existing types in `lib/types.ts`

### Optional (For PNG Export)
- `html-to-image` or native Canvas API for graph export (evaluate during implementation)

## Implementation Order

1. **Copy graph-layout.ts** - Framework-agnostic utility
2. **DependencyGraph.tsx** - Core visualization component (most complex)
3. **InitiativeHeader.tsx** - Header with status management
4. **InitiativeTasksTab.tsx** - Tasks list and management
5. **InitiativeDecisionsTab.tsx** - Decisions list
6. **InitiativeGraphTab.tsx** - Graph wrapper
7. **Modals** - EditInitiativeModal, LinkTaskModal, AddDecisionModal
8. **InitiativeDetail.tsx** - Main page assembling all components
9. **Unit Tests** - For each component
10. **E2E Tests** - End-to-end scenarios

## Validation Checklist

Before marking complete:

- [ ] All components render without errors
- [ ] TypeScript compiles (`npm run typecheck`)
- [ ] Linting passes (`npm run lint`)
- [ ] Unit tests pass (`npm run test`)
- [ ] E2E tests pass (`npm run e2e`)
- [ ] Visual parity with Svelte version
- [ ] No console errors in browser
- [ ] All success criteria checkboxes checked
