# Web Frontend

Svelte 5 SvelteKit application for the orc web UI.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | SvelteKit 2.x, Svelte 5 (runes) |
| Styling | CSS (component-scoped) |
| Testing | Vitest (unit), Playwright (E2E) |
| Build | Vite, Bun |

## Directory Structure

```
web/src/
├── lib/
│   ├── components/
│   │   ├── DependencyGraph.svelte  # Task dependency DAG visualization
│   │   ├── comments/     # TaskCommentsPanel, TaskCommentThread, TaskCommentForm
│   │   ├── dashboard/    # Stats, actions, activity
│   │   ├── diff/         # DiffViewer, DiffFile, DiffHunk, VirtualScroller
│   │   ├── filters/      # InitiativeDropdown, ViewModeDropdown
│   │   ├── kanban/       # Board, Column, QueuedColumn, TaskCard, Swimlane
│   │   ├── layout/       # Header, Sidebar
│   │   ├── overlays/     # Modal, CommandPalette, NewTaskModal, KeyboardShortcutsHelp
│   │   ├── review/       # CommentForm, CommentThread, ReviewPanel
│   │   ├── task/         # TaskHeader, TaskEditModal, Timeline, Transcript, RetryPanel, Attachments
│   │   └── ui/           # Icon, StatusIndicator, Toast
│   ├── stores/           # tasks.ts, project.ts, sidebar.ts, toast.svelte.ts
│   ├── utils/            # format.ts, status.ts, platform.ts, graph-layout.ts
│   ├── api.ts            # API client
│   ├── websocket.ts      # WebSocket client
│   └── shortcuts.ts      # Keyboard shortcuts
└── routes/               # SvelteKit pages
```

## Key Components

| Category | Components | Purpose |
|----------|------------|---------|
| Layout | Header, Sidebar | Navigation, project/initiative switcher |
| Dashboard | Stats, QuickActions, ActiveTasks, RecentActivity | Overview page |
| Task | TaskCard, Timeline, Transcript, TaskHeader, TaskEditModal, PRActions, Attachments, TokenUsage, DependencySidebar, AddDependencyModal | Task detail |
| Graph | DependencyGraph | Task dependency visualization |
| Diff | DiffViewer, DiffFile, DiffHunk, DiffLine, VirtualScroller | Changes tab |
| Filters | InitiativeDropdown, ViewModeDropdown | Filter bar dropdowns |
| Kanban | Board, Column, QueuedColumn, TaskCard, Swimlane, ConfirmModal | Board view with queue/priority/swimlanes |
| Overlays | Modal, LiveTranscriptModal, FinalizeModal, CommandPalette, KeyboardShortcutsHelp | Modal dialogs and overlays |
| Comments | TaskCommentsPanel, TaskCommentThread, TaskCommentForm | Task discussion notes |
| Review | CommentForm, CommentThread, ReviewPanel, ReviewSummary | Code review comments |
| UI | Icon (40 icons), StatusIndicator, Toast, Modal | Shared components |

## State Management

| Store | Purpose |
|-------|---------|
| `tasks` | Global reactive task state, WebSocket updates |
| `project` | Current project selection with persistence |
| `initiative` | Initiative filter selection with URL + localStorage persistence |
| `sidebar` | Expanded/collapsed state (persisted in localStorage) |
| `toast` | Notification queue |

**Task store** initialized in `+layout.svelte`, synced via global WebSocket. Pages subscribe for reactive updates.

### Initiative Filter

Initiative filtering persists across page refreshes using URL and localStorage:

**Priority order** (highest to lowest):
1. **URL parameter** (`?initiative=<id>`) - Shareable links, survives refresh
2. **localStorage** (`orc_current_initiative_id`) - User's last selection
3. **null** - No filter (show all tasks)

**Browser history:** Selecting initiatives pushes to browser history, so back/forward buttons navigate between filter states.

**Store:** Use `currentInitiativeId` for the active filter, `initiatives` for the list, and `initiativeProgress` for completion counts.

**API:** Use `selectInitiative(id)` to filter by initiative (updates URL + localStorage). Pass `null` to clear the filter.

### Project Selection

Project selection persists across page refreshes using URL and localStorage:

**Priority order** (highest to lowest):
1. **URL parameter** (`?project=<id>`) - Shareable links, survives refresh
2. **localStorage** (`orc_current_project_id`) - User's last selection
3. **Server default** (`GET /api/projects/default`) - From `~/.orc/projects.yaml`
4. **First project** - Falls back to first available project

**Browser history:** Switching projects pushes to browser history, so back/forward buttons navigate between previously viewed projects.

**API:** Use `selectProject(id)` to switch projects (updates URL + localStorage). Use `setDefaultProject(id)` to persist a server-side default.

**Note:** Task operations (create, run, pause, resume, delete) require a project to be selected. When no project is selected, the UI shows a "Select Project" prompt instead of an empty task list. All task operations use the project-scoped API endpoints (`/api/projects/:id/tasks/*`) rather than the CWD-based endpoints.

## Keyboard Shortcuts

**Global shortcuts use Shift+Alt** (Shift+Option on Mac) to avoid browser conflicts with Cmd+K, Cmd+N, etc.

| Shortcut | Action |
|----------|--------|
| `Shift+Alt+K` | Command palette |
| `Shift+Alt+N` | New task |
| `Shift+Alt+B` | Toggle sidebar |
| `Shift+Alt+P` | Project switcher |
| `g d/t/s` | Go to dashboard/tasks/settings |
| `j/k` | Navigate task list |
| `Enter` | Open selected |
| `r/p/d` | Run/Pause/Delete task |

## Development

```bash
bun install           # Install deps
bun run dev           # Dev server
bun run build         # Production build
bun run test          # Unit tests
bunx playwright test  # E2E tests
```

## Svelte 5 Runes

**All pages use runes mode.** Legacy syntax causes build errors.

```svelte
<script>
  let { data } = $props()        // Props (not export let)
  let count = $state(0)          // Reactive state (NOT let count = 0)
  let doubled = $derived(count * 2)  // Derived (NOT $: doubled)
</script>

<!-- Event handlers: use onclick, NOT on:click -->
<button onclick={handleClick}>Click</button>
<form onsubmit={(e) => { e.preventDefault(); save(); }}>
```

**Common mistakes:** `let x = []` without `$state()`, `$:` reactive statements, `on:click` handlers.

## Global Modal Pattern

Modals that can be triggered from multiple pages (via keyboard shortcuts, command palette, etc.) live in `+layout.svelte`:

| Modal | Trigger Event | Keyboard |
|-------|--------------|----------|
| `NewTaskModal` | `orc:new-task` | `Shift+Alt+N` |
| `CommandPalette` | `orc:command-palette` | `Shift+Alt+K` |
| `KeyboardShortcutsHelp` | `orc:show-shortcuts` | `?` |

**To trigger from any page:**
```svelte
window.dispatchEvent(new CustomEvent('orc:new-task'));
```

Page-specific modals (like `TaskEditModal`) can live in individual routes.

## WebSocket Architecture

Global WebSocket in `+layout.svelte` subscribes with `"*"`. All task events flow to task store. Pages react to store changes - no individual WebSocket connections needed.

### Live Refresh

The board and task list automatically update when tasks are created, modified, or deleted via CLI or filesystem:

| Event | Store Action | UI Effect |
|-------|--------------|-----------|
| `task_created` | `addTask(task)` | New card appears, toast notification |
| `task_updated` | `updateTask(id, task)` | Card updates in place |
| `task_deleted` | `removeTask(id)` | Card removed, toast notification |

**Event flow:** File watcher (backend) → WebSocket → `+layout.svelte` handler → task store → reactive UI update

The file watcher uses content hashing and debouncing to prevent duplicate notifications from atomic saves or git operations.

See `QUICKREF.md` for subscription helpers.

## Task Organization (Queue, Priority & Category)

Tasks support queue, priority, and category organization to manage and filter work:

### Queue

| Queue | Display | Purpose |
|-------|---------|---------|
| `active` | Prominent in column | Current work |
| `backlog` | Collapsed section, dashed borders | "Someday" items |

Each column shows active tasks first, then a collapsible "Backlog" divider with count.

### Priority

| Priority | Indicator | Sort Order |
|----------|-----------|------------|
| `critical` | Pulsing red icon | First |
| `high` | Orange up arrow | Second |
| `normal` | None shown | Third |
| `low` | Gray down arrow | Fourth |

Tasks are sorted within each column by: **running status first** (running tasks always appear at the top), then by priority. Priority badges only appear for non-normal priorities.

### Category

| Category | Badge Style | Description |
|----------|-------------|-------------|
| `feature` | Purple | New functionality (default) |
| `bug` | Red | Bug fix |
| `refactor` | Blue | Code restructuring |
| `chore` | Gray | Maintenance tasks |
| `docs` | Green | Documentation |
| `test` | Orange | Test-related |

Categories are displayed as badges on task cards and can be used for filtering. Set via CLI (`--category`) or web UI.

### Running Task Indicator

Running tasks display a distinct visual indicator:
- Thicker accent-colored border (2px)
- Subtle gradient background
- Pulsing glow animation (2s cycle)

This makes running tasks immediately visible in any column, distinguishing them from pending tasks.

### Live Transcript Modal

Clicking a running task opens `LiveTranscriptModal` - a modal overlay showing real-time Claude output:

| Feature | Description |
|---------|-------------|
| Live streaming | Shows current response as it generates via WebSocket |
| Connection status | Displays "Live", "Connecting", or "Disconnected" indicator |
| Token tracking | Updates input/output/cached token counts in real-time |
| Phase display | Shows current phase badge and task status |
| Transcript history | Paginated list of completed transcript files |
| Full view link | Button to open `/tasks/:id?tab=transcript` |
| Auto-scroll | Scrolls to bottom as new content arrives |

**WebSocket events handled:**
- `transcript` - Streaming chunks and complete responses
- `state` - Task state updates (phase, status)
- `tokens` - Token usage updates
- `phase` / `complete` - Triggers transcript reload

**Triggering the modal:**
- Click running task card on board or task list
- Cards pass `onViewTranscript` callback to open modal

### Finalize Modal

The `FinalizeModal` (`overlays/FinalizeModal.svelte`) manages the finalize phase UI for completed tasks with approved PRs. It provides real-time progress tracking, result display, and retry capabilities.

**Opening the modal:**
- Click the finalize button on a completed task card in the Done column
- Button only appears when task status is `completed`

**Modal states:**

| State | Display | Actions |
|-------|---------|---------|
| Not Started | Explanation + "Start Finalize" button | Start finalize |
| Pending | Status badge, waiting message | - |
| Running | Progress bar, step label, live updates | - |
| Completed | Success message, result details | Close |
| Failed | Error message, explanation | Retry |

**Result display (on completion):**

| Field | Description |
|-------|-------------|
| Merged Commit | 7-char abbreviated SHA |
| Target Branch | Branch merged into (e.g., `main`) |
| Files Changed | Number of modified files |
| Conflicts Resolved | Count of merge conflicts handled |
| Tests | Pass/fail status with color indicator |
| Risk Level | Low (green) / Medium (yellow) / High (red) |

**WebSocket integration:**
- Subscribes to `finalize` events for real-time progress
- Updates step label, progress message, and percentage
- Shows connection status indicator (Live/Connecting/Disconnected)

**API functions:**
- `triggerFinalize(taskId)` - Start finalize operation
- `getFinalizeStatus(taskId)` - Load current finalize state

### TaskCard Finalize States

TaskCard displays different visual states for finalize workflow:

| Task Status | Visual Indicator | Actions |
|-------------|------------------|---------|
| `completed` | Success border, finalize button in actions | Click to open FinalizeModal |
| `finalizing` | Info border (2px), pulsing animation, progress bar | View progress |
| `finished` | Success border, merged commit info | View merge details |

**Finalize button:**
- Appears in card action area for `completed` tasks
- Icon: two connected circles (merge symbol)
- Shows spinner when loading
- Disabled during finalizing

**Progress indicator (finalizing state):**
- Shows current step label
- Progress bar with percentage
- Animated border pulse (2s cycle)

**Finished info:**
- Green background section
- Displays abbreviated commit SHA (7 chars)
- Shows target branch name
- Merge icon indicator

**CSS classes:**
```css
.task-card.finalizing {
  border-color: var(--status-info);
  border-width: 2px;
  animation: finalize-pulse 2s ease-in-out infinite;
}

.task-card.finished {
  border-color: var(--status-success);
  background: gradient with success tint;
}
```

### Task Dependency Sidebar

The task detail page includes a `DependencySidebar` component showing task relationships and dependencies.

```
┌─ Dependencies ────────────────────────────┐
│                                           │
│ Blocked by (2)              [+ Add]       │
│ ┌─────────────────────────────────────┐   │
│ │ ✓ TASK-060 Add initiative_id...     │   │  ← completed, green check
│ │ ○ TASK-061 Add sidebar navigation   │   │  ← pending, gray circle
│ └─────────────────────────────────────┘   │
│                                           │
│ Blocks (1)                                │
│ ┌─────────────────────────────────────┐   │
│ │ ○ TASK-065 Swimlane view toggle     │   │
│ └─────────────────────────────────────┘   │
│                                           │
│ Related (1)                   [+ Add]     │
│ ┌─────────────────────────────────────┐   │
│ │ TASK-063 Initiative badge on cards  │   │
│ └─────────────────────────────────────┘   │
│                                           │
│ Referenced in (2)                         │
│ ┌─────────────────────────────────────┐   │
│ │ TASK-072 Dependency documentation   │   │
│ └─────────────────────────────────────┘   │
│                                           │
└───────────────────────────────────────────┘
```

| Section | Description | Editable |
|---------|-------------|----------|
| **Blocked by** | Tasks that must complete first | Yes (+ Add / Remove) |
| **Blocks** | Tasks waiting on this task | No (computed inverse) |
| **Related** | Informational task relationships | Yes (+ Add / Remove) |
| **Referenced in** | Tasks mentioning this one in description | No (auto-detected) |

**Status indicators:**
- ✓ (green) - Completed task
- ● (blue, pulsing) - Running task
- ○ (gray) - Pending/planned task

**Features:**
- Collapsible header with task count badge
- Blocked banner when task has unmet dependencies
- Blocking info when task is blocking other tasks
- Click dependency to navigate to that task
- `+ Add` button opens `AddDependencyModal` for search/select
- Remove buttons appear on hover for editable sections
- Empty states for sections with no items

**API integration:**
- Uses `getTaskDependencies(taskId)` to fetch dependency graph
- Uses `addBlocker()`, `removeBlocker()`, `addRelated()`, `removeRelated()` for mutations
- Notifies parent via `onTaskUpdated` callback after changes

### Dependency Graph Visualization

The `DependencyGraph` component (`lib/components/DependencyGraph.svelte`) provides an interactive DAG (directed acyclic graph) visualization of task dependencies.

```
┌─ Dependency Graph ─────────────────────────────────────────────┐
│ [Zoom +] [Zoom -] [Fit] [Export PNG]                           │
│                                                                │
│           ┌──────────┐                                         │
│           │ TASK-060 │                                         │
│           │ (done)   │                                         │
│           └────┬─────┘                                         │
│      ┌─────────┼─────────┬─────────┐                           │
│      ▼         ▼         ▼         ▼                           │
│ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐                    │
│ │TASK-061│ │TASK-063│ │TASK-064│ │TASK-066│                    │
│ │(ready) │ │(ready) │ │(ready) │ │(ready) │                    │
│ └────┬───┘ └────────┘ └────────┘ └────────┘                    │
│      ▼                                                         │
│ ┌────────┐                                                     │
│ │TASK-062│                                                     │
│ │(blocked)│                                                    │
│ └────────┘                                                     │
│                                                                │
│ Legend: ■ done  ○ ready  ⊘ blocked  ● running                  │
└────────────────────────────────────────────────────────────────┘
```

| Feature | Description |
|---------|-------------|
| Automatic layout | Topological sort positions tasks by dependency order (root tasks at top) |
| Interactive pan/zoom | Mouse wheel zoom, drag to pan, "Fit" button auto-sizes |
| Node colors | Status-based colors (green=done, blue=ready, red=blocked, etc.) |
| Click navigation | Click node to navigate to task detail page |
| Hover tooltips | Shows full task title and status on hover |
| Export PNG | Download graph as PNG image |
| Legend | Color key at bottom of graph |

**Props:**
```typescript
interface Props {
  nodes: DependencyGraphNode[];  // Tasks with id, title, status
  edges: DependencyGraphEdge[];  // { from, to } pairs
  onNodeClick?: (nodeId: string) => void;  // Custom click handler (default: navigate)
}
```

**Status colors:**
| Status | Border/Text | Background |
|--------|-------------|------------|
| `done` | `--status-success` | `--status-success-bg` |
| `running` | `--accent-primary` | `--accent-subtle` |
| `blocked` | `--status-danger` | `--status-danger-bg` |
| `ready` | `--status-info` | `--status-info-bg` |
| `pending` | `--text-muted` | `--bg-tertiary` |
| `paused` | `--status-warning` | `--status-warning-bg` |
| `failed` | `--status-danger` | `--status-danger-bg` |

**Layout algorithm** (`lib/utils/graph-layout.ts`):
- Uses Kahn's algorithm for topological sort
- Groups tasks into horizontal layers by dependency depth
- Centers layers horizontally within the viewport
- Curved bezier edges for visual clarity
- Handles cycles gracefully (places remaining nodes in current layer)

**Integration:**
- Used in Initiative detail page ("Graph" tab) via `getInitiativeDependencyGraph(id)`
- Can be used for arbitrary task sets via `getTasksDependencyGraph(taskIds)`

### TaskCard Quick Menu

Right-click or use the "..." menu on TaskCard to:
- Move to Active/Backlog queue
- Set priority (Critical/High/Normal/Low)
- Set category (Feature/Bug/Refactor/Chore/Docs/Test)
- Run/Pause task actions

### Initiatives Sidebar Navigation

The sidebar includes a collapsible Initiatives section following Linear-style UX patterns:

```
Work
├── Dashboard
├── Tasks
├── Board
└── Initiatives           ← Collapsible section
    ├── ● All Tasks       (selected = shows all tasks)
    ├── ○ Frontend Migration (3/7)
    ├── ○ Auth Rework (1/4)
    └── + New Initiative  (opens create modal)
```

| Feature | Description |
|---------|-------------|
| Selection indicator | Filled dot (●) for selected, empty dot (○) for others |
| Progress display | Shows (completed/total) count from initiative tasks |
| Sorting | Active initiatives first, then by recency |
| Create button | '+ New Initiative' triggers `onNewInitiative` callback |
| Filtering | Selection applies to both Board and Tasks pages |

**Selection behavior:**
- Click "All Tasks" to clear filter and show all tasks
- Click an initiative to filter Board/Tasks to only those tasks
- Selection persists in URL (`?initiative=INIT-001`) and localStorage

**Store integration:**
- Uses `$initiatives` for the list
- Uses `$currentInitiativeId` for the selection (null = all tasks)
- Uses `$initiativeProgress` for completion counts

### Board View Mode Toggle

The board supports two view modes selectable via a dropdown in the header:

| Mode | Description |
|------|-------------|
| **Flat** (default) | Traditional kanban - all tasks in columns |
| **By Initiative** | Swimlane view - tasks grouped by initiative |

**Toggle control:** `ViewModeDropdown` in board header persists selection in localStorage (`orc-board-view-mode`).

**Filter interaction:** When an initiative filter is active, swimlane view is disabled (the toggle becomes inactive) since filtering already shows a single initiative's tasks.

### Swimlane View

When "By Initiative" view is selected, tasks are grouped into horizontal swimlanes:

```
[By Initiative ▾]  Task Board (59 tasks)

Frontend Migration (6/8)                                    67%
┌─────────┬─────────┬─────────┬─────────┬─────────┬─────────┐
│ Queued  │ Spec    │ Impl    │ Test    │ Review  │ Done    │
│ task    │         │ task    │         │         │ task    │
└─────────┴─────────┴─────────┴─────────┴─────────┴─────────┘

Unassigned (12)                                             0%
┌─────────┬─────────┬─────────┬─────────┬─────────┬─────────┐
│ task    │         │ task    │         │         │ task    │
└─────────┴─────────┴─────────┴─────────┴─────────┴─────────┘
```

| Feature | Description |
|---------|-------------|
| Swimlane header | Initiative title + progress (completed/total) + percentage bar |
| Collapsible | Click header to collapse/expand; state persists in localStorage (`orc-collapsed-swimlanes`) |
| Sort order | Active initiatives first (alphabetically), then other statuses, then "Unassigned" at bottom |
| Empty swimlanes | Initiatives with no tasks are hidden |
| Cross-swimlane drag-drop | Dragging task to different swimlane prompts initiative change confirmation |

**Components:**
- `ViewModeDropdown` (`filters/ViewModeDropdown.svelte`) - View mode selector
- `Swimlane` (`kanban/Swimlane.svelte`) - Individual swimlane row with columns
- `Board` (`kanban/Board.svelte`) - Renders flat or swimlane view based on `viewMode` prop

**Initiative change via drag-drop:** When a task is dropped into a different initiative's swimlane, a confirmation modal appears asking to change the task's initiative. This provides a quick way to reassign tasks between initiatives.

### Initiative Filter Dropdown

The `InitiativeDropdown` component (`filters/InitiativeDropdown.svelte`) provides initiative filtering in the filter bars on both Tasks and Board pages:

```
[All | Active | Completed | Failed] [Search...] [Initiative ▾] [Weight ▾] [Sort ▾]
```

| Feature | Description |
|---------|-------------|
| All initiatives | Default option showing all tasks |
| Unassigned | Shows only tasks with no initiative_id |
| Initiative list | Sorted by status (active first), then by title |
| Task count | Shows task count in parentheses: 'Frontend Migration (7)' |
| Title truncation | Long titles truncated to 24 chars with ellipsis |

**Dropdown options:**
1. "All initiatives" - Clears filter, shows all tasks
2. "Unassigned" - Shows only standalone tasks (no initiative_id)
3. Initiative items - Each shows truncated title + task count

**State sync:** The dropdown uses the same initiative store as the sidebar, so selections are synchronized. Selecting from either location updates both.

**Special value:** The `UNASSIGNED_INITIATIVE` constant (`'__unassigned__'`) is used to filter tasks without an initiative_id. This is exported from the initiative store for use in filtering logic.

**Filter bar placement:**
- Tasks page: Between search input and weight filter
- Board page: In header alongside "New Task" button

### Initiative Detail Page

The initiative detail page (`/initiatives/:id`) provides a dedicated view for managing individual initiatives including their tasks, dependency graph, and decisions.

```
← Back to Tasks

Frontend Framework Migration                    [Edit] [Archive]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Vision: Migrate from Svelte 5 to React 19 for better ecosystem

Progress  ████████████░░░░░░  12/18 tasks (67%)
Owner: RM
Status: Active
Created: Jan 10, 2026

┌─ Tasks ─────────────────────────────────────────────────────┐
│ [+ Add Task]  [+ Link Existing Task]                        │
│                                                             │
│ ✓ TASK-060 Add initiative_id field...          completed   │
│ ● TASK-061 Add sidebar navigation...           running     │
│ ○ TASK-062 Add initiative filter...            planned     │
└─────────────────────────────────────────────────────────────┘

┌─ Decisions ─────────────────────────────────────────────────┐
│ [+ Add Decision]                                            │
│                                                             │
│ DEC-001 (Jan 10): Use filter-based nav, not more columns   │
│   Rationale: Columns are workflow stages, not groupings     │
└─────────────────────────────────────────────────────────────┘
```

**Tabs:**
| Tab | Content |
|-----|---------|
| **Tasks** | Task list with status icons, add/link/remove tasks |
| **Graph** | Interactive dependency graph visualization (see Dependency Graph Visualization section) |
| **Decisions** | Decision list with date/author/rationale |

| Section | Features |
|---------|----------|
| **Header** | Title, status badge, edit/archive buttons, vision statement, progress bar, owner, dates |
| **Tasks tab** | Task list with status icons, add new task, link existing tasks, remove tasks |
| **Graph tab** | Visual DAG showing task dependencies within the initiative (loads via `getInitiativeDependencyGraph`) |
| **Decisions tab** | Decision list with date/author/rationale, add new decisions |

**Header section:**
- Initiative title with status badge (draft/active/completed/archived)
- Edit button opens modal for title, vision, status changes
- Archive button changes status to archived (with confirmation)
- Progress bar shows completed/total task count with percentage
- Owner display (if set) and creation date

**Tasks tab:**
- List shows task ID, title, and status with colored icon indicators
- Click task to navigate to task detail page
- "Add Task" button opens new task modal with initiative pre-selected
- "Link Existing" button opens search modal to add existing tasks
- Remove button (X) unlinks task from initiative (doesn't delete task)

**Graph tab:**
- Interactive dependency graph using `DependencyGraph` component
- Shows tasks with `blocked_by` relationships as a visual DAG
- Click nodes to navigate to task detail
- Zoom/pan controls, export to PNG
- Loads data via `getInitiativeDependencyGraph(id)` API

**Decisions tab:**
- Each decision shows ID, date, optional author, decision text, and rationale
- "Add Decision" opens modal with decision text, rationale, and author fields
- Decisions provide context for AI when working on initiative tasks

**Status indicators:**
| Icon | Status | Color |
|------|--------|-------|
| ✓ (check-circle) | completed | Green (success) |
| ✓✓ (check-circle-2) | finished | Green (success), bold |
| ● (play-circle) | running | Blue (info) |
| ⟳ (refresh-cw) | finalizing | Blue (info), animated |
| ○ (circle) | planned/created | Gray (muted) |
| ⚠ (alert-circle) | blocked | Yellow (warning) |
| ✗ (x-circle) | failed | Red (danger) |
| ⏸ (pause-circle) | paused | Yellow (warning) |

**API integration:**
- `getInitiative(id)` - Load initiative with tasks and decisions
- `updateInitiative(id, data)` - Update title, vision, status
- `addInitiativeTask(id, { task_id })` - Link existing task
- `removeInitiativeTask(id, taskId)` - Unlink task from initiative
- `addInitiativeDecision(id, { decision, rationale?, by? })` - Add decision

**Store integration:**
- `updateInitiativeInStore()` - Sync changes to initiative store after edits

## Attachments

Task attachments (images, files) can be added during task creation or after via the Attachments tab.

### Task Creation

`NewTaskModal` supports attaching files during task creation:
- Drag-and-drop zone or file picker
- Image thumbnails for preview
- Supports images, PDF, text, markdown, JSON, and log files
- Files included in multipart form submission

### Task Detail (Attachments Tab)

`Attachments` component on task detail page:
- Drag-and-drop upload with visual feedback
- Image gallery with thumbnails and lightbox viewer
- File list with metadata (size, date)
- Supports delete with confirmation

API functions: `listAttachments()`, `uploadAttachment()`, `getAttachmentUrl()`, `deleteAttachment()`

## Token Usage Display

Token usage is displayed in multiple locations with cached token support:

| Location | Component | Display |
|----------|-----------|---------|
| Dashboard stats | `DashboardStats` | Total tokens with cached count in label and tooltip |
| Task detail (Timeline tab) | Stats grid | Input/Output/Cached/Total breakdown |
| Transcript | `Transcript` | Per-iteration tokens with cache info in tooltip |

**Cached tokens:** When `cache_creation_input_tokens` or `cache_read_input_tokens` are present, UI shows:
- Combined cached total in parentheses (e.g., "245K tokens (120K cached)")
- Tooltip with breakdown: cache creation vs cache read
- Cached stat card styled in success color (green)

**Data flow:** WebSocket `tokens` events update `taskState.tokens` in real-time. Components derive display values from the `TokenUsage` interface.

## Review Workflow

"Changes" tab combines diff + inline review:
1. View diff (split/unified toggle)
2. Click line number → comment form
3. Set severity (Suggestion/Issue/Blocker)
4. "Send to Agent" → triggers retry with context

See `QUICKREF.md` for component hierarchy.

## Statusline Configuration

The statusline page (`/environment/claude/statusline`) provides a user-friendly interface for configuring Claude Code's terminal statusline.

### Configuration Modes

| Mode | Purpose |
|------|---------|
| Simple | User-friendly UI with checkboxes and presets |
| Advanced | Raw shell command or script path input |

### Simple Mode Features

| Feature | Description |
|---------|-------------|
| Presets | Quick configuration templates (Minimal, Standard, Developer, Plain) |
| Components | Toggle username, hostname, directory, git branch, Python venv |
| Colors | Enable/disable ANSI color codes in output |
| Custom text | Add prefix/suffix to the statusline |
| Live preview | Shows sample statusline output as you configure |

**Presets:**
- **Minimal**: Directory + git branch only
- **Standard**: All components enabled with colors
- **Developer**: Venv + git branch + directory
- **Plain**: All components without colors

### Advanced Mode

Enter raw shell commands or script paths directly. The statusline receives JSON context on stdin with model info, workspace, and token usage.

### Scope Toggle

| Scope | Path | Purpose |
|-------|------|---------|
| Global | `~/.claude/settings.json` | Applies to all projects |
| Project | `.claude/settings.json` | Project-specific override |

**API:** Use `updateSettings(settings, 'global')` to save globally, or `updateSettings(settings)` for project scope.

### Generated Script Format

Simple mode generates shell scripts with:
- Bash builtins for performance (`$PWD`, `$USER`, `$HOSTNAME`)
- ANSI escape codes for colors when enabled
- Git branch detection with proper quoting
- Python virtual environment display
- Shell injection prevention via escaping

## Plugins Page

The plugins page (`/environment/claude/plugins`) manages Claude Code plugins with two tabs:

| Tab | Purpose |
|-----|---------|
| Installed | Manage local plugins in `.claude/plugins/` |
| Marketplace | Browse and install plugins from the marketplace |

**Features:**
- Scope filter (All/Global/Project) for installed plugins
- Enable/disable toggle per plugin
- Update indicator when newer versions available
- Plugin detail panel showing commands, hooks, MCP servers
- Marketplace search and browsing with pagination
- Install to project or global scope

**Marketplace fallback:** When the official Claude Code plugin marketplace is unavailable, the UI displays sample plugins with a message explaining how to manually install plugins via CLI (`claude plugin add <github-repo>`).

**API functions:** `listPlugins()`, `enablePlugin()`, `disablePlugin()`, `browseMarketplace()`, `searchMarketplace()`, `installPlugin()`, `checkPluginUpdates()`, `updatePlugin()`

## Preferences Page

The preferences page (`/preferences`) provides a unified interface for editing both global and project Claude Code settings.

### Settings Tabs

| Tab | Scope | Path |
|-----|-------|------|
| Global | All projects | `~/.claude/settings.json` |
| Project | Current project | `.claude/settings.json` |

### Editable Settings

| Setting | Description |
|---------|-------------|
| Environment Variables | Key-value pairs passed to Claude Code |
| StatusLine Type | Type of statusline command |
| StatusLine Command | Shell command for terminal statusline |

**Note:** Both global and project settings are fully editable through the UI. Changes are saved directly to the respective `settings.json` files.

### CLAUDE.md Display

The preferences page also displays CLAUDE.md file hierarchy (read-only display):
- Global: `~/.claude/CLAUDE.md`
- User: `~/CLAUDE.md`
- Project: `./CLAUDE.md`

Edit CLAUDE.md files via `/environment/docs` route.

## Orchestrator Settings Page

The automation page (`/environment/orchestrator/automation`) provides a complete interface for configuring orc behavior.

### Editable Settings

| Section | Settings |
|---------|----------|
| **Profile** | auto, fast, safe, strict |
| **Automation** | Gates default (auto/human/ai), retry enabled, max retries |
| **Execution** | Model, max iterations, timeout |
| **Git** | Branch prefix, commit prefix |
| **Worktree** | Enabled, directory, cleanup on complete/fail |
| **Completion** | Action (pr/merge/none), target branch, delete branch |
| **Timeouts** | Phase max, turn max, idle warning, heartbeat interval, idle timeout |

**Note:** All orc configuration is editable through the UI. Changes are saved to `.orc/config.yaml`.

**API functions:** `getConfig()`, `updateConfig()`

## Routes

| Route | Page |
|-------|------|
| `/` | Dashboard |
| `/board` | Kanban board |
| `/tasks` | Task list |
| `/tasks/:id` | Task detail (Timeline/Changes/Transcript/Test Results/Attachments/Comments tabs) |
| `/initiatives/:id` | Initiative detail (Tasks/Decisions sections, edit capabilities) |
| `/config` | Redirects to `/environment/orchestrator/automation` |
| `/environment` | Environment hub (Claude Code + Orchestrator config) |
| `/environment/docs` | CLAUDE.md editor (`?scope=global\|user\|project`) |
| `/environment/claude/skills` | Skills (`?scope=global`) |
| `/environment/claude/hooks` | Hooks (`?scope=global`) |
| `/environment/claude/agents` | Agents (`?scope=global`) |
| `/environment/claude/mcp` | MCP servers (`?scope=global`) |
| `/environment/claude/plugins` | Plugin management & marketplace |
| `/environment/claude/statusline` | Statusline configuration (`?scope=global`) |
| `/environment/orchestrator/automation` | Orc automation settings |
| `/environment/orchestrator/prompts` | Phase prompt overrides |
| `/environment/orchestrator/scripts` | Script registry |
| `/environment/orchestrator/export` | Export configuration |
| `/preferences` | User preferences (global + project settings)

## API Client

See `QUICKREF.md` for full function list.

```typescript
// Common patterns
await listTasks(projectId?)
await runTask(taskId, projectId?)
await updateTask(taskId, { title?, description?, weight?, metadata? })
await createReviewComment(taskId, { file_path, line_number, content, severity })
```

## Testing

### Unit Tests (Vitest)
```bash
bun run test
bun run test:coverage
```

### E2E Tests (Playwright)
```bash
bunx playwright test
bunx playwright test --ui
bunx playwright test --grep board  # Run board tests only
```

**Test files:**

| File | Coverage |
|------|----------|
| `e2e/board.spec.ts` | Board page: rendering, view modes, drag-drop, swimlanes (18 tests) |
| `e2e/initiatives.spec.ts` | Initiative CRUD, detail page, task linking, decisions, dependency graph (20 tests) |
| `e2e/task-detail.spec.ts` | Task detail tabs: navigation, timeline, changes, transcript, attachments (15 tests) |
| `e2e/tasks.spec.ts` | Task list, task detail, CRUD operations |
| `e2e/navigation.spec.ts` | Routing, navigation, back button |
| `e2e/sidebar.spec.ts` | Sidebar navigation, collapse state |
| `e2e/dashboard.spec.ts` | Dashboard stats, quick actions |
| `e2e/keyboard-shortcuts.spec.ts` | Keyboard shortcut handling |
| `e2e/hooks.spec.ts` | Hook configuration UI |
| `e2e/prompts.spec.ts` | Prompt editor UI |
| `e2e/websocket.spec.ts` | WebSocket real-time updates, connection handling (17 tests) |

### Framework-Agnostic E2E Testing

E2E tests use framework-agnostic selectors to support future React migration:

| Priority | Method | Example | Use Case |
|----------|--------|---------|----------|
| 1 | `getByRole()` | `getByRole('region', { name: 'Queued column' })` | Semantic elements |
| 2 | `getByText()` | `getByText('Task Board')` | Headings, labels |
| 3 | `.locator()` with class | `locator('.task-card')` | Structural elements |
| 4 | ARIA attributes | `locator('[aria-label="..."]')` | Accessible elements |

**Avoid:**
- CSS class fragments (`.svelte-abc123`, `.react-xyz`)
- Implementation-specific attributes
- Deep DOM path selectors

**Helper functions** (see `board.spec.ts`):
- `waitForBoardLoad(page)` - Wait for board to render
- `clearBoardStorage(page)` - Reset localStorage for test isolation
- `switchToSwimlaneView(page)` - Toggle view mode

**Flakiness prevention:**
- Use `waitForSelector()` with reasonable timeouts
- Clear localStorage before persistence tests
- Use `.catch(() => false)` for optional element checks
- Add small waits after animations (`waitForTimeout(100)`)

### WebSocket E2E Testing

The `websocket.spec.ts` file provides comprehensive E2E tests for WebSocket real-time updates, using Playwright's WebSocket route interception to inject events without mocking.

**Test Categories (17 tests total):**

| Category | Tests | Coverage |
|----------|-------|----------|
| Task Lifecycle (5) | Status changes, phase moves, create/delete events, progress indicators | `state`, `task_created`, `task_deleted`, `phase` events |
| Live Transcript (4) | Modal open, streaming content, connection status, token updates | `transcript`, `tokens` events |
| Connection Handling (3) | Auto-reconnect, reconnecting status, resume after reconnect | WebSocket disconnect/reconnect cycle |
| Legacy (5) | Connection status display, page reload, transcript/timeline tabs | Regression coverage |

**WebSocket Event Injection Pattern:**

```typescript
// Set up WebSocket interception with event injection
let wsSendToPage: ((data: string) => void) | null = null;

await page.routeWebSocket(/\/api\/ws/, async (ws) => {
  const server = await ws.connectToServer();
  wsSendToPage = (data: string) => ws.send(data);  // Capture send function

  ws.onMessage((message) => server.send(message));
  server.onMessage((message) => ws.send(message));
});

// Inject events to test UI response
if (wsSendToPage) {
  const event = createWSEvent('state', taskId, { status: 'running' });
  wsSendToPage(JSON.stringify(event));
}
```

**Event Types Tested:**
- `state` - Task status and phase changes
- `transcript` - Streaming content chunks
- `tokens` - Token usage updates (input/output/cached)
- `phase` - Phase transitions (started/completed/failed)
- `task_created` / `task_updated` / `task_deleted` - File watcher events

**Key Testing Helpers:**
- `createWSEvent(event, taskId, data)` - Create properly structured WebSocket event message
- `waitForBoardLoad(page)` - Wait for board to render with tasks
- `findTask(page, preferRunning)` - Find a task card, optionally preferring running tasks

**Why This Approach:**
- Uses real WebSocket connections (not mocked) for true E2E testing
- Tests actual UI updates in response to events
- Framework-agnostic - works with any frontend (Svelte, React, etc.)
- Playwright's `routeWebSocket` allows event injection without modifying production code

## Deep-Dive Reference

See `QUICKREF.md` for:
- Virtual scrolling pattern
- Kanban board phase mapping
- WebSocket subscription helpers
- Task store actions
- API client functions
- Utility functions
- Component gotchas
