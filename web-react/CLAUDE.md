# React 19 Frontend (Migration)

React 19 application for orc web UI, running in parallel with existing Svelte app during migration.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | React 19, Vite |
| Language | TypeScript 5.6+ |
| State Management | Zustand |
| Routing | React Router 7 |
| Styling | CSS (component-scoped, matching Svelte) |
| Testing | Vitest (unit), Playwright (E2E - shared) |

## Directory Structure

```
web-react/src/
├── main.tsx              # Entry point (BrowserRouter)
├── App.tsx               # Root component (useRoutes + ShortcutProvider + WebSocketProvider)
├── index.css             # Global styles
├── router/               # Route configuration
│   ├── index.ts          # Exports
│   └── routes.tsx        # Route definitions
├── lib/                  # Shared utilities
│   ├── types.ts          # TypeScript interfaces
│   ├── websocket.ts      # OrcWebSocket class
│   ├── shortcuts.ts      # ShortcutManager class
│   └── platform.ts       # Platform detection (isMac)
├── components/           # UI components
│   ├── board/            # Kanban board components
│   │   ├── Board.tsx     # Main board (flat/swimlane views)
│   │   ├── Column.tsx    # Board column with drop zone
│   │   ├── QueuedColumn.tsx # Queued column (active/backlog)
│   │   ├── Swimlane.tsx  # Initiative swimlane row
│   │   ├── TaskCard.tsx  # Task card for board
│   │   ├── ViewModeDropdown.tsx # Flat/swimlane toggle
│   │   └── InitiativeDropdown.tsx # Initiative filter
│   ├── dashboard/        # Dashboard components
│   │   ├── DashboardStats.tsx      # Quick stats cards
│   │   ├── DashboardActiveTasks.tsx # Running/paused/blocked tasks
│   │   ├── DashboardQuickActions.tsx # New Task / View All buttons
│   │   ├── DashboardRecentActivity.tsx # Recently completed tasks
│   │   ├── DashboardInitiatives.tsx # Active initiatives with progress
│   │   └── DashboardSummary.tsx    # Total/completed/failed counts
│   ├── layout/           # Layout components
│   │   ├── AppLayout.tsx # Main layout (Sidebar + Header + Outlet)
│   │   ├── Sidebar.tsx   # Left navigation
│   │   ├── Header.tsx    # Top bar
│   │   └── UrlParamSync.tsx # URL <-> Store bidirectional sync
│   ├── task-detail/      # Task detail components
│   │   ├── TaskHeader.tsx        # Header with actions
│   │   ├── TabNav.tsx            # 6-tab navigation
│   │   ├── DependencySidebar.tsx # Collapsible deps panel
│   │   ├── TimelineTab.tsx       # Phase timeline + tokens
│   │   ├── ChangesTab.tsx        # Git diff viewer
│   │   ├── TranscriptTab.tsx     # Transcript history
│   │   ├── TestResultsTab.tsx    # Test results + screenshots
│   │   ├── AttachmentsTab.tsx    # File uploads
│   │   ├── CommentsTab.tsx       # Task comments
│   │   ├── TaskEditModal.tsx     # Edit form modal
│   │   ├── ExportDropdown.tsx    # Export options
│   │   └── diff/                 # Diff viewer sub-components
│   │       ├── DiffStats.tsx
│   │       ├── DiffFile.tsx
│   │       ├── DiffHunk.tsx
│   │       ├── DiffLine.tsx
│   │       └── InlineCommentThread.tsx
│   ├── filters/          # Filter dropdowns
│   │   └── DependencyDropdown.tsx # Dependency status filter
│   ├── overlays/         # Modal overlays
│   │   ├── Modal.tsx     # Base modal component
│   │   └── KeyboardShortcutsHelp.tsx # Shortcuts help modal
│   └── ui/               # UI primitives
│       ├── Icon.tsx      # SVG icons (60+ built-in)
│       ├── StatusIndicator.tsx # Status orb with animations
│       ├── ToastContainer.tsx  # Toast notification queue
│       └── Breadcrumbs.tsx     # Route-based breadcrumbs
├── pages/                # Route pages
│   ├── TaskList.tsx      # / - Task list
│   ├── Board.tsx         # /board - Kanban board
│   ├── Dashboard.tsx     # /dashboard - Dashboard
│   ├── TaskDetail.tsx    # /tasks/:id - Task detail
│   ├── InitiativeDetail.tsx # /initiatives/:id
│   ├── Preferences.tsx   # /preferences
│   └── environment/      # /environment/* pages
├── stores/               # Zustand stores
└── hooks/                # Custom hooks
```

## Development

```bash
# Install dependencies
cd web-react && npm install

# Start dev server (port 5174)
npm run dev

# Run tests
npm run test
npm run test:watch
npm run test:coverage

# Build for production
npm run build
```

**Ports:**
- Svelte (current): `http://localhost:5173`
- React (migration): `http://localhost:5174`
- API server: `http://localhost:8080`

## Configuration

### Vite Config

| Setting | Value | Purpose |
|---------|-------|---------|
| Port | 5174 | Avoid conflict with Svelte on 5173 |
| API Proxy | `/api` → `:8080` | Backend communication |
| WebSocket | Proxied via `/api` | Real-time updates |
| Path Alias | `@/` → `src/` | Clean imports |
| Build Output | `build/` | Matches Svelte structure |

### TypeScript Config

| Setting | Purpose |
|---------|---------|
| `strict: true` | Full type safety |
| `noUnusedLocals: true` | Clean code |
| `jsx: react-jsx` | React 19 JSX transform |
| `paths: @/*` | Import aliases |
| `types: vitest/globals` | Test globals |

## Migration Strategy

This React app runs alongside Svelte during migration:

1. **Phase 1** ✅: Project scaffolding, Zustand stores mirroring Svelte stores
2. **Phase 2** ✅: Core infrastructure (API client, WebSocket, Router with URL sync), Dashboard page, Board page (flat/swimlane views), TaskList page, TaskDetail page with all 6 tabs
3. **Phase 3** (current): Component migration (parallel implementation) - InitiativeDetail, remaining environment pages
4. **Phase 4**: E2E test validation, feature parity verification
5. **Phase 5**: Cutover and Svelte removal

### Shared Resources

| Resource | Location | Notes |
|----------|----------|-------|
| E2E tests | `web/e2e/` | Shared, framework-agnostic |
| API server | `:8080` | Same backend |
| Visual baselines | `web/e2e/__snapshots__/` | Will need React baselines |

### Component Mapping

Migration follows the existing Svelte component structure:

| Svelte Component | React Equivalent | Status |
|------------------|------------------|--------|
| `+layout.svelte` | `App.tsx` + Router | ✅ Complete |
| `lib/components/` | `src/components/` | In Progress |
| `lib/components/Icon.svelte` | `components/ui/Icon.tsx` | ✅ Complete |
| `lib/components/StatusIndicator.svelte` | `components/ui/StatusIndicator.tsx` | ✅ Complete |
| `lib/components/Modal.svelte` | `components/overlays/Modal.tsx` | ✅ Complete |
| `lib/components/ToastContainer.svelte` | `components/ui/ToastContainer.tsx` | ✅ Complete |
| `lib/components/Breadcrumbs.svelte` | `components/ui/Breadcrumbs.tsx` | ✅ Complete |
| `lib/components/Sidebar.svelte` | `components/layout/Sidebar.tsx` | ✅ Complete |
| `lib/components/Header.svelte` | `components/layout/Header.tsx` | ✅ Complete |
| `+layout.svelte` (full layout) | `components/layout/AppLayout.tsx` | ✅ Complete |
| `lib/components/ProjectSwitcher.svelte` | `components/overlays/ProjectSwitcher.tsx` | ✅ Complete |
| `lib/components/Dashboard.svelte` | `pages/Dashboard.tsx` | ✅ Complete |
| `lib/components/dashboard/*` | `components/dashboard/*` | ✅ Complete |
| `lib/components/Board.svelte` | `components/board/Board.tsx` | ✅ Complete |
| `lib/components/Column.svelte` | `components/board/Column.tsx` | ✅ Complete |
| `lib/components/QueuedColumn.svelte` | `components/board/QueuedColumn.tsx` | ✅ Complete |
| `lib/components/Swimlane.svelte` | `components/board/Swimlane.tsx` | ✅ Complete |
| `lib/components/TaskCard.svelte` (kanban) | `components/board/TaskCard.tsx` | ✅ Complete |
| `lib/components/InitiativeDropdown.svelte` | `components/board/InitiativeDropdown.tsx` | ✅ Complete |
| `lib/components/DependencyDropdown.svelte` | `components/filters/DependencyDropdown.tsx` | ✅ Complete |
| `routes/+page.svelte` (task list) | `pages/TaskList.tsx` | ✅ Complete |
| `routes/tasks/[id]/+page.svelte` | `pages/TaskDetail.tsx` | ✅ Complete |
| `lib/components/TaskHeader.svelte` | `components/task-detail/TaskHeader.tsx` | ✅ Complete |
| `lib/components/DependencySidebar.svelte` | `components/task-detail/DependencySidebar.tsx` | ✅ Complete |
| `lib/components/TabNav.svelte` | `components/task-detail/TabNav.tsx` | ✅ Complete |
| `lib/components/TimelineTab.svelte` | `components/task-detail/TimelineTab.tsx` | ✅ Complete |
| `lib/components/ChangesTab.svelte` | `components/task-detail/ChangesTab.tsx` | ✅ Complete |
| `lib/components/TranscriptTab.svelte` | `components/task-detail/TranscriptTab.tsx` | ✅ Complete |
| `lib/components/TestResultsTab.svelte` | `components/task-detail/TestResultsTab.tsx` | ✅ Complete |
| `lib/components/AttachmentsTab.svelte` | `components/task-detail/AttachmentsTab.tsx` | ✅ Complete |
| `lib/components/CommentsTab.svelte` | `components/task-detail/CommentsTab.tsx` | ✅ Complete |
| `lib/components/diff/*` | `components/task-detail/diff/*` | ✅ Complete |
| `lib/stores/` | `src/stores/` (Zustand) | ✅ Complete |
| `lib/websocket.ts` | `src/lib/websocket.ts` | ✅ Complete |
| `lib/utils/` | `src/lib/` | ✅ Complete |
| Route pages | `src/pages/` | ✅ Complete |

**Stores implemented (Phase 1 + Phase 3):**
- `taskStore.ts` - Task data and execution state with derived selectors
- `projectStore.ts` - Project selection with URL/localStorage sync
- `initiativeStore.ts` - Initiative filter with progress tracking
- `dependencyStore.ts` - Dependency status filter with URL/localStorage sync
- `uiStore.ts` - Sidebar, WebSocket status, toast notifications

**WebSocket hooks implemented (Phase 2):**
- `useWebSocket.tsx` - WebSocketProvider, useWebSocket, useTaskSubscription, useConnectionStatus

**Router implemented (Phase 2):**
- `router/routes.tsx` - Route configuration matching Svelte app
- `components/layout/UrlParamSync.tsx` - Bidirectional URL/store sync

**Keyboard shortcuts implemented (Phase 1):**
- `lib/shortcuts.ts` - ShortcutManager class with sequence support
- `hooks/useShortcuts.tsx` - ShortcutProvider, useShortcuts, useGlobalShortcuts, useTaskListShortcuts
- `components/overlays/KeyboardShortcutsHelp.tsx` - Help modal with platform-aware key display

**UI primitives implemented (Phase 2):**
- `components/ui/Icon.tsx` - SVG icon component with 60+ built-in icons
- `components/ui/StatusIndicator.tsx` - Colored status orb with animations
- `components/ui/ToastContainer.tsx` - Toast notification queue (uses uiStore)
- `components/ui/Breadcrumbs.tsx` - Route-based navigation breadcrumbs
- `components/overlays/Modal.tsx` - Portal-based modal with focus trap

**Layout components implemented (Phase 3):**
- `components/layout/AppLayout.tsx` - Root layout with sidebar, header, outlet
- `components/layout/Sidebar.tsx` - Navigation with initiative filtering
- `components/layout/Header.tsx` - Project selector, page title, actions
- `components/overlays/ProjectSwitcher.tsx` - Modal for project selection

**Pages implemented (Phase 3):**
- `pages/TaskList.tsx` - Task list with filtering, search, keyboard navigation
- `components/filters/DependencyDropdown.tsx` - Dependency status filter dropdown

## UI Primitives

Foundational components that other components depend on.

### Icon

SVG icon component with 60+ built-in icons using stroke-based rendering.

```tsx
import { Icon } from '@/components/ui';

<Icon name="dashboard" />
<Icon name="check" size={16} />
<Icon name="error" size={24} className="text-danger" />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `name` | `IconName` | required | Icon identifier |
| `size` | `number` | `20` | Width/height in pixels |
| `className` | `string` | `''` | Additional CSS classes |

**Icon categories:** Navigation (dashboard, tasks, board, etc.), Actions (plus, search, close, check, trash), Playback (play, pause), Chevrons, Status (success, error, warning, info), Git (branch, git-branch), Circle variants (circle, check-circle, play-circle, etc.)

### StatusIndicator

Colored status orb with animations for running/paused states.

```tsx
import { StatusIndicator } from '@/components/ui';

<StatusIndicator status="running" />
<StatusIndicator status="completed" size="lg" showLabel />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `status` | `TaskStatus` | required | Task status (running, paused, completed, etc.) |
| `size` | `'sm' \| 'md' \| 'lg'` | `'md'` | Indicator size |
| `showLabel` | `boolean` | `false` | Show status text label |

**Status colors:** running (accent/pulse), paused (warning/pulse), blocked (danger), completed/finished (success), failed (danger), classifying (warning), created/planned (muted)

### Modal

Portal-based modal with focus trap, escape-to-close, and backdrop click handling.

```tsx
import { Modal } from '@/components/overlays';

<Modal open={isOpen} onClose={() => setIsOpen(false)} title="Confirm">
  <p>Are you sure?</p>
  <button onClick={() => setIsOpen(false)}>Cancel</button>
</Modal>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `open` | `boolean` | required | Whether modal is visible |
| `onClose` | `() => void` | required | Close handler |
| `title` | `string` | - | Optional header title |
| `size` | `'sm' \| 'md' \| 'lg' \| 'xl'` | `'md'` | Max width |
| `showClose` | `boolean` | `true` | Show close button |
| `children` | `ReactNode` | required | Modal content |

**Features:** Focus trap (Tab cycles within modal), Escape key closes, Click outside closes, Body scroll lock, Focus restoration on close, Portal renders to document.body

### ToastContainer

Toast notification queue rendered via portal. Uses `uiStore` for state management.

```tsx
// Add ToastContainer to app root (renders via portal)
import { ToastContainer } from '@/components/ui';
<ToastContainer />

// Trigger toasts from anywhere
import { toast } from '@/stores';
toast.success('Task created');
toast.error('Failed to save', { duration: 10000 });
toast.warning('Unsaved changes');
toast.info('Processing...');
toast.dismiss('toast-id');  // Dismiss specific toast
toast.clear();              // Clear all toasts
```

**Toast types:** success (5s), error (8s), warning (5s), info (5s)

### Breadcrumbs

Route-based navigation breadcrumb trail. Only renders for `/environment/*` and `/preferences` routes.

```tsx
import { Breadcrumbs } from '@/components/ui';

// Typically placed in Header or page layout
<Breadcrumbs />
```

**Behavior:** Auto-generates from current route path, Category segments (claude, orchestrator) link to parent `/environment`, Last segment is non-clickable current page

## Layout Components

Main layout components that wrap all pages.

### AppLayout

Root layout component combining Sidebar, Header, and page content area.

```tsx
// Used in router configuration (routes.tsx)
import { AppLayout } from '@/components/layout/AppLayout';

const routes = [
  {
    element: <AppLayout />,
    children: [/* page routes */]
  }
];
```

**Structure:**
- Handles global keyboard shortcuts
- Manages modal states (shortcuts help, project switcher)
- Provides responsive layout with sidebar margin
- Uses React Router's `<Outlet>` for page content

**CSS classes:**
- `.app-layout` - Root container (flex)
- `.app-layout.sidebar-expanded` - When sidebar is open (240px margin)
- `.app-layout.sidebar-collapsed` - When sidebar is collapsed (60px margin)
- `.app-main` - Main content wrapper
- `.app-content` - Page content area (scrollable)

### Sidebar

Left navigation with collapsible sections and initiative filtering.

```tsx
import { Sidebar } from '@/components/layout/Sidebar';

<Sidebar onNewInitiative={() => openNewInitiativeModal()} />
```

| Prop | Type | Description |
|------|------|-------------|
| `onNewInitiative` | `() => void` | Optional callback for new initiative button |

**Sections:**
- **Work**: Dashboard, Tasks, Board navigation links
- **Initiatives**: Collapsible list with progress counts, filters tasks
- **Environment**: Claude Code and Orchestrator sub-groups with nested navigation
- **Preferences**: Bottom-pinned settings link

**Features:**
- Expand/collapse toggle persisted in localStorage
- Section/group expansion states persisted separately
- Initiative list shows progress (completed/total) or status badge
- Active route highlighting via NavLink
- Keyboard shortcut hint (⇧⌥B to toggle)

**CSS classes:**
- `.sidebar` / `.sidebar.expanded` - Main container
- `.nav-section` / `.nav-item` / `.nav-item.active` - Navigation structure
- `.initiative-list` / `.initiative-item` - Initiative entries
- `.nav-group` / `.group-header` - Environment sub-groups

### Header

Top bar with project selector, page title, and action buttons.

```tsx
import { Header } from '@/components/layout/Header';

<Header
  onProjectClick={() => openProjectSwitcher()}
  onNewTask={() => openNewTaskModal()}
  onCommandPalette={() => openCommandPalette()}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `onProjectClick` | `() => void` | Project switcher trigger |
| `onNewTask` | `() => void` | Optional new task button handler |
| `onCommandPalette` | `() => void` | Command palette trigger |

**Features:**
- Project name with folder icon and chevron
- Auto-derived page title from current route
- Commands button with keyboard shortcut hint (⇧⌥K)
- New Task button (primary style)

### ProjectSwitcher

Modal overlay for switching between projects.

```tsx
import { ProjectSwitcher } from '@/components/overlays';

<ProjectSwitcher
  open={showProjectSwitcher}
  onClose={() => setShowProjectSwitcher(false)}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `open` | `boolean` | Whether modal is visible |
| `onClose` | `() => void` | Close handler |

**Features:**
- Search/filter projects by name or path
- Keyboard navigation (↑/↓ arrows, Enter to select, Esc to close)
- Shows current project with "Active" badge
- Loading and error states
- Portal-rendered to document.body

**Keyboard shortcuts:** ⇧⌥P opens project switcher (handled by AppLayout)

## Dashboard Components

Components for the Dashboard page (`/dashboard`).

### Dashboard (Page)

Main dashboard page component that orchestrates all dashboard sections.

```tsx
import { Dashboard } from '@/pages/Dashboard';

// Used in route configuration
<Route path="/dashboard" element={<Dashboard />} />
```

**Data flow:**
- Fetches `DashboardStats` from `/api/dashboard/stats`
- Fetches active initiatives from `/api/initiatives?status=active`
- Derives active/recent tasks from `TaskStore`
- Subscribes to WebSocket events for real-time updates

**URL params:**
- `project`: Project filter (handled by UrlParamSync)

### DashboardStats

Quick stats cards with live connection indicator.

```tsx
import { DashboardStats } from '@/components/dashboard';

<DashboardStats
  stats={stats}
  wsStatus={wsStatus}
  onFilterClick={(status) => navigate(`/?status=${status}`)}
  onDependencyFilterClick={(status) => navigate(`/?dependency_status=${status}`)}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `stats` | `DashboardStats` | Stats object from API |
| `wsStatus` | `ConnectionStatus` | WebSocket connection status |
| `onFilterClick` | `(status: string) => void` | Handler for status card clicks |
| `onDependencyFilterClick` | `(status: string) => void` | Optional handler for blocked card |

**Stats displayed:**
- **Running**: Tasks currently executing (clickable)
- **Blocked**: Tasks waiting on dependencies (clickable)
- **Today**: Tasks completed today (clickable)
- **Tokens**: Total token usage with cached tokens breakdown (tooltip)

**Connection indicator states:**
- Connected: Green dot, "Live"
- Connecting/Reconnecting: Yellow dot, pulsing
- Disconnected: Gray dot, "Offline"

### DashboardActiveTasks

List of running/paused/blocked tasks with navigation.

```tsx
import { DashboardActiveTasks } from '@/components/dashboard';

<DashboardActiveTasks tasks={activeTasks} />
```

| Prop | Type | Description |
|------|------|-------------|
| `tasks` | `Task[]` | Tasks with status: running, paused, or blocked |

**Features:**
- Click to navigate to task detail
- Shows StatusIndicator, task ID, title, and current phase
- Limited to 5 tasks
- Hidden when no active tasks

### DashboardQuickActions

Action buttons for common operations.

```tsx
import { DashboardQuickActions } from '@/components/dashboard';

<DashboardQuickActions
  onNewTask={() => window.dispatchEvent(new CustomEvent('orc:new-task'))}
  onViewTasks={() => navigate('/')}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `onNewTask` | `() => void` | Handler for "New Task" button |
| `onViewTasks` | `() => void` | Handler for "View All Tasks" button |

### DashboardRecentActivity

Timeline of recently completed/failed tasks.

```tsx
import { DashboardRecentActivity } from '@/components/dashboard';

<DashboardRecentActivity tasks={recentTasks} />
```

| Prop | Type | Description |
|------|------|-------------|
| `tasks` | `Task[]` | Tasks with status: completed or failed, sorted by updated_at |

**Features:**
- Click to navigate to task detail
- Shows StatusIndicator, task ID, title, and relative timestamp
- Relative time format: "just now", "5m ago", "2h ago", "3d ago"
- Limited to 5 most recent
- Hidden when no recent tasks

### DashboardInitiatives

Active initiatives with progress bars.

```tsx
import { DashboardInitiatives } from '@/components/dashboard';

<DashboardInitiatives initiatives={initiatives} />
```

| Prop | Type | Description |
|------|------|-------------|
| `initiatives` | `Initiative[]` | Active initiatives with embedded tasks |

**Features:**
- Click to filter board by initiative (`/board?initiative=XXX`)
- Progress bar shows completed/total tasks
- Progress color: green (75%+), yellow (25-74%), red (<25%)
- Non-active initiatives show status badge instead of progress
- Sorted by updated_at, limited to 5
- "View All" link when >5 initiatives
- Vision text shown in tooltip
- Hidden when no initiatives

### DashboardSummary

Overall task counts at bottom of dashboard.

```tsx
import { DashboardSummary } from '@/components/dashboard';

<DashboardSummary stats={stats} />
```

| Prop | Type | Description |
|------|------|-------------|
| `stats` | `DashboardStats` | Stats object from API |

**Displays:**
- Total Tasks (all tasks)
- Completed (green)
- Failed (red)

### DashboardStats Type

Stats returned by `/api/dashboard/stats`:

```typescript
interface DashboardStats {
  running: number;
  paused: number;
  blocked: number;
  completed: number;
  failed: number;
  today: number;           // Completed today
  total: number;
  tokens: number;          // Total tokens used
  cache_creation_input_tokens?: number;
  cache_read_input_tokens?: number;
  cost: number;            // Estimated cost
}
```

## TaskList Page

The task list page (`/`) provides a filterable, searchable list view of all tasks with keyboard navigation.

### TaskList (Page)

Main task list page component with comprehensive filtering and search.

```tsx
import { TaskList } from '@/pages/TaskList';

// Used in route configuration
<Route path="/" element={<TaskList />} />
```

**Features:**
- Status tabs (All/Active/Completed/Failed) with counts
- Search with 300ms debounce
- Filter by initiative, dependency status, weight
- Sort by recent/oldest/status
- Keyboard navigation (j/k/Enter/r/p/d)
- Initiative filter banner when filtered
- New task button

**URL params:**
- `project`: Project filter (handled by UrlParamSync)
- `initiative`: Initiative filter (synced via store)
- `dependency_status`: Dependency status filter

**Keyboard shortcuts (context: 'tasks'):**

| Key | Action |
|-----|--------|
| `j` | Select next task |
| `k` | Select previous task |
| `Enter` | Open selected task |
| `r` | Run selected task |
| `p` | Pause selected task |
| `d` | Delete selected task (with confirmation) |
| `/` | Focus search input |
| `?` | Show keyboard help |

**Status Filters:**
- **All**: All tasks
- **Active**: Tasks not in terminal status (running, paused, blocked, planned, created)
- **Completed**: Tasks with status completed or finished
- **Failed**: Tasks with status failed

**Sorting:**
- **Recent**: By updated_at descending (default)
- **Oldest**: By updated_at ascending
- **Status**: By status order (running → paused → blocked → planned → created → finalizing → completed → finished → failed)

## Filter Components

Reusable filter dropdowns for task filtering.

### DependencyDropdown

Dropdown to filter tasks by dependency status.

```tsx
import { DependencyDropdown } from '@/components/filters';

<DependencyDropdown tasks={tasks} />
```

| Prop | Type | Description |
|------|------|-------------|
| `tasks` | `Task[]` | Tasks to count (for badge numbers) |

**Filter Options:**
- **All tasks**: No filter (shows all)
- **Ready**: Tasks with `dependency_status: 'ready'`
- **Blocked**: Tasks with `dependency_status: 'blocked'`
- **No dependencies**: Tasks with `dependency_status: 'none'`

**Features:**
- Task count badges for each option
- Active filter visual indication
- Click outside to close
- Escape key closes dropdown
- Syncs with `dependencyStore`

**CSS classes:**
- `.dependency-dropdown` - Container
- `.dropdown-trigger` / `.dropdown-trigger.active` - Button
- `.dropdown-menu` / `.dropdown-item` - Menu

## Routing

### Route Configuration

All routes are defined in `src/router/routes.tsx` using React Router's `RouteObject` array pattern.

| Route | Component | URL Params |
|-------|-----------|------------|
| `/` | `TaskList` | `?project`, `?initiative`, `?dependency_status` |
| `/board` | `Board` | `?project`, `?initiative`, `?dependency_status` |
| `/dashboard` | `Dashboard` | `?project` |
| `/tasks/:id` | `TaskDetail` | `?tab` |
| `/initiatives/:id` | `InitiativeDetail` | - |
| `/preferences` | `Preferences` | - |
| `/environment/*` | `EnvironmentLayout` | - |

**Environment sub-routes:**
- `/environment/settings`
- `/environment/prompts`
- `/environment/scripts`
- `/environment/hooks`
- `/environment/skills`
- `/environment/mcp`
- `/environment/config`
- `/environment/claudemd`
- `/environment/tools`
- `/environment/agents`

### Layout Structure

```
main.tsx
└── BrowserRouter
    └── App.tsx (useRoutes + WebSocketProvider)
        └── AppLayout
            ├── UrlParamSync (invisible, handles URL <-> store sync)
            ├── Sidebar
            ├── Header
            └── Outlet (page content)
```

### URL Parameter Handling

The `UrlParamSync` component provides bidirectional sync between URL params and Zustand stores:

**URL -> Store (on navigation/back/forward):**
- Reads `?project` and `?initiative` from URL
- Updates `projectStore.selectProject()` and `initiativeStore.selectInitiative()`

**Store -> URL (on programmatic changes):**
- Listens to store state changes
- Updates URL via `setSearchParams()` with `replace: true`
- Uses ref flags (`isSyncingFromUrl`, `isSyncingFromStore`) to prevent infinite loops

**Route-specific params:**
- `project`: Available on all routes
- `initiative`: Only synced on `/` and `/board`
- `dependency_status`: Read directly in components (not store-synced)

### Usage in Components

```tsx
import { useSearchParams, useParams } from 'react-router-dom';
import { useCurrentProjectId, useCurrentInitiativeId } from '@/stores';

function TaskList() {
  // Store state (synced from URL automatically)
  const projectId = useCurrentProjectId();
  const initiativeId = useCurrentInitiativeId();

  // Direct URL param access (for non-store params)
  const [searchParams] = useSearchParams();
  const dependencyStatus = searchParams.get('dependency_status');

  // Route params
  const { id } = useParams();  // For /tasks/:id
}
```

### Navigation

```tsx
import { Link, NavLink, useNavigate } from 'react-router-dom';

// Declarative links
<Link to="/board">Board</Link>
<NavLink to="/board" className={({ isActive }) => isActive ? 'active' : ''}>
  Board
</NavLink>

// Programmatic navigation
const navigate = useNavigate();
navigate('/tasks/TASK-001');
navigate('/board?project=abc&initiative=xyz');
```

## Testing

### Unit Tests (Vitest)

```bash
npm run test          # Run once
npm run test:watch    # Watch mode
npm run test:coverage # With coverage
```

Test files use `*.test.tsx` convention. Setup in `src/test-setup.ts` includes:
- `@testing-library/react` for component testing
- `@testing-library/jest-dom` for DOM matchers
- jsdom environment

### E2E Tests (Playwright)

E2E tests are shared with Svelte in `web/e2e/`. Tests use framework-agnostic selectors:
- `getByRole()` for semantic elements
- `getByText()` for headings/labels
- `.locator()` with class names for structural elements

## API Integration

The app connects to the orc API server via Vite proxy:

```typescript
// Example: Health check
fetch('/api/health')
  .then(res => res.json())
  .then(data => console.log(data.status));
```

All `/api/*` requests proxy to `localhost:8080`. WebSocket connections also proxy through the same path.

## Patterns

### Component Structure

```tsx
// Functional components with hooks
function TaskCard({ task }: { task: Task }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="task-card">
      {/* ... */}
    </div>
  );
}
```

### State Management (Zustand)

Five Zustand stores mirror the Svelte store behavior. All use `subscribeWithSelector` middleware for efficient derived state.

#### Store Overview

| Store | State | Persistence | Purpose |
|-------|-------|-------------|---------|
| `useTaskStore` | tasks, taskStates | None (API-driven) | Task data and execution state |
| `useProjectStore` | projects, currentProjectId | URL + localStorage | Project selection |
| `useInitiativeStore` | initiatives, currentInitiativeId | URL + localStorage | Initiative filter |
| `useDependencyStore` | currentDependencyStatus | URL + localStorage | Dependency status filter |
| `useUIStore` | sidebarExpanded, wsStatus, toasts | localStorage (sidebar) | UI state |

**URL/localStorage priority:** URL param > localStorage > default

#### TaskStore

Primary state for task data and execution states.

| State | Type | Description |
|-------|------|-------------|
| `tasks` | `Task[]` | Main task array |
| `taskStates` | `Map<string, TaskState>` | Execution state by task ID |
| `loading` | `boolean` | Loading indicator |
| `error` | `string \| null` | Error message |

| Derived | Hook | Description |
|---------|------|-------------|
| Active tasks | `useActiveTasks()` | Tasks with status: running, blocked, paused |
| Recent tasks | `useRecentTasks()` | Last 10 completed/failed/finished, sorted by updated_at |
| Running tasks | `useRunningTasks()` | Tasks with status: running |
| Status counts | `useStatusCounts()` | Counts: all, active, completed, failed, running, blocked |
| Single task | `useTask(id)` | Get task by ID |
| Task state | `useTaskState(id)` | Get execution state by ID |

| Action | Purpose |
|--------|---------|
| `setTasks(tasks)` | Replace all tasks |
| `addTask(task)` | Add task (prevents duplicates) |
| `updateTask(id, updates)` | Partial update |
| `updateTaskStatus(id, status, phase?)` | Update status and optionally current_phase |
| `removeTask(id)` | Remove task and its state |
| `updateTaskState(id, state)` | Set execution state (syncs status to task) |
| `removeTaskState(id)` | Remove execution state |
| `getTask(id)` | Get task directly |
| `getTaskState(id)` | Get state directly |

#### ProjectStore

Project selection with URL and localStorage sync.

| State | Type | Description |
|-------|------|-------------|
| `projects` | `Project[]` | Available projects |
| `currentProjectId` | `string \| null` | Selected project |
| `_isHandlingPopState` | `boolean` | Internal flag for history handling |

| Hook | Description |
|------|-------------|
| `useProjects()` | All projects |
| `useCurrentProject()` | Current project object |
| `useCurrentProjectId()` | Current project ID |

| Action | Purpose |
|--------|---------|
| `setProjects(projects)` | Set projects (auto-selects first if current invalid) |
| `selectProject(id)` | Select project (updates URL and localStorage) |
| `handlePopState(event)` | Handle browser back/forward |
| `initializeFromUrl()` | Initialize from URL on mount |

#### InitiativeStore

Initiative filter with URL sync. Stores initiatives in a Map for O(1) lookup.

| State | Type | Description |
|-------|------|-------------|
| `initiatives` | `Map<string, Initiative>` | Initiatives by ID |
| `currentInitiativeId` | `string \| null` | Filter selection (null = all) |
| `hasLoaded` | `boolean` | Tracks initial load |

| Hook | Description |
|------|-------------|
| `useInitiatives()` | All initiatives as array |
| `useCurrentInitiative()` | Current initiative object |
| `useCurrentInitiativeId()` | Current filter ID |

| Action | Purpose |
|--------|---------|
| `setInitiatives(list)` | Set initiatives (clears filter if selected no longer exists) |
| `addInitiative(initiative)` | Add single initiative |
| `updateInitiative(id, updates)` | Partial update |
| `removeInitiative(id)` | Remove (clears filter if selected) |
| `selectInitiative(id)` | Set filter |
| `getInitiative(id)` | Get by ID |
| `getInitiativeTitle(id)` | Get title (falls back to ID) |
| `getInitiativeProgress(tasks)` | Calculate completed/total per initiative |

**Helper functions:**
- `truncateInitiativeTitle(title, maxLength)` - Truncate for badges
- `getInitiativeBadgeTitle(id, maxLength)` - Get display and full title for tooltip

#### UIStore

UI state including sidebar, WebSocket status, and toast notifications.

| State | Type | Description |
|-------|------|-------------|
| `sidebarExpanded` | `boolean` | Sidebar state (persisted) |
| `wsStatus` | `ConnectionStatus` | WebSocket connection status |
| `toasts` | `Toast[]` | Active toast queue |

| Hook | Description |
|------|-------------|
| `useSidebarExpanded()` | Sidebar expanded state |
| `useWsStatus()` | WebSocket status |
| `useToasts()` | Toast array |

| Action | Purpose |
|--------|---------|
| `toggleSidebar()` | Toggle and persist |
| `setSidebarExpanded(bool)` | Set and persist |
| `setWsStatus(status)` | Update WebSocket status |
| `addToast(toast)` | Add toast (returns ID) |
| `dismissToast(id)` | Remove toast |
| `clearToasts()` | Remove all |

**Toast default durations:** success/warning/info: 5s, error: 8s

#### DependencyStore

Dependency status filter with URL and localStorage sync.

| State | Type | Description |
|-------|------|-------------|
| `currentDependencyStatus` | `DependencyStatusFilter` | Current filter ('all', 'blocked', 'ready', 'none') |
| `_isHandlingPopState` | `boolean` | Internal flag for history handling |

| Hook | Description |
|------|-------------|
| `useCurrentDependencyStatus()` | Current filter selection |

| Action | Purpose |
|--------|---------|
| `selectDependencyStatus(status)` | Set filter (null or 'all' clears filter) |
| `handlePopState(event)` | Handle browser back/forward |
| `initializeFromUrl()` | Initialize from URL on mount |

**Type exports:**
- `DependencyStatusFilter` - 'all' | 'blocked' | 'ready' | 'none'
- `DEPENDENCY_OPTIONS` - Array of { value, label } for dropdown options

**URL param:** `?dependency_status=blocked|ready|none`

**localStorage key:** `orc_dependency_status_filter`

#### Usage Examples

```tsx
import {
  useTaskStore,
  useProjectStore,
  useInitiativeStore,
  useDependencyStore,
  useUIStore,
  useCurrentDependencyStatus,
  DEPENDENCY_OPTIONS,
  toast,
} from '@/stores';

// Direct state access
const tasks = useTaskStore((state) => state.tasks);

// Derived state via selector hooks
import { useActiveTasks, useStatusCounts } from '@/stores';
const activeTasks = useActiveTasks();
const counts = useStatusCounts();

// Dependency filter
const dependencyStatus = useCurrentDependencyStatus();
useDependencyStore.getState().selectDependencyStatus('blocked');

// Actions (can be called outside components)
useTaskStore.getState().updateTask('TASK-001', { status: 'running' });
useProjectStore.getState().selectProject('proj-001');

// Toast notifications (works outside React components)
toast.success('Task created');
toast.error('Failed to load', { duration: 10000 });
toast.dismiss('toast-id');
toast.clear();
```

**Special values:**
- `UNASSIGNED_INITIATIVE = '__unassigned__'` - Filter for tasks without an initiative
- `DEPENDENCY_OPTIONS` - Array of { value, label } for dependency filter dropdown

#### Key Implementation Patterns

1. **URL sync middleware:** Project and Initiative stores use custom URL sync with `isHandlingPopState` flag to prevent recursive updates during browser navigation

2. **localStorage sync:** All persisted stores subscribe to state changes and sync to localStorage automatically

3. **Derived state as getters:** Computed values (activeTasks, statusCounts) are methods on the store rather than stored state, ensuring fresh calculations

4. **Map vs Array:** InitiativeStore uses `Map<string, Initiative>` for O(1) lookups; `getInitiativesList()` converts to array when needed

### WebSocket Hooks

Real-time task updates via WebSocket connection to the orc API.

#### WebSocketProvider

Wraps the app to provide WebSocket functionality. Must be a parent of any component using WebSocket hooks.

```tsx
import { WebSocketProvider } from '@/hooks';

function App() {
  return (
    <WebSocketProvider autoConnect={true} autoSubscribeGlobal={true}>
      <YourApp />
    </WebSocketProvider>
  );
}
```

| Prop | Default | Description |
|------|---------|-------------|
| `autoConnect` | `true` | Connect on mount |
| `autoSubscribeGlobal` | `true` | Subscribe to all task events |
| `baseUrl` | `window.location.host` | Custom WebSocket host |

#### useWebSocket

Access WebSocket functionality from any component.

```tsx
import { useWebSocket } from '@/hooks';

function TaskControls({ taskId }: { taskId: string }) {
  const { status, command, subscribe, on } = useWebSocket();

  // Send commands
  const handlePause = () => command(taskId, 'pause');
  const handleResume = () => command(taskId, 'resume');

  // Subscribe to events
  useEffect(() => {
    const unsub = on('state', (event) => {
      if ('event' in event && event.task_id === taskId) {
        console.log('State update:', event.data);
      }
    });
    return unsub;
  }, [taskId, on]);

  return <div>Status: {status}</div>;
}
```

| Return | Type | Description |
|--------|------|-------------|
| `status` | `ConnectionStatus` | 'connecting' \| 'connected' \| 'disconnected' \| 'reconnecting' |
| `subscribe(taskId)` | `void` | Subscribe to task events |
| `unsubscribe()` | `void` | Unsubscribe from current task |
| `subscribeGlobal()` | `void` | Subscribe to all task events |
| `on(eventType, callback)` | `() => void` | Add event listener, returns cleanup |
| `command(taskId, action)` | `void` | Send pause/resume/cancel command |
| `isConnected()` | `boolean` | Check connection state |
| `getTaskId()` | `string \| null` | Current subscribed task |

#### useTaskSubscription

Subscribe to a specific task for streaming updates.

```tsx
import { useTaskSubscription } from '@/hooks';

function TaskTranscript({ taskId }: { taskId: string }) {
  const { state, transcript, isSubscribed, connectionStatus, clearTranscript } =
    useTaskSubscription(taskId);

  return (
    <div>
      <div>Phase: {state?.current_phase}</div>
      <div>
        {transcript.map((line, i) => (
          <div key={i}>{line.content}</div>
        ))}
      </div>
    </div>
  );
}
```

| Return | Type | Description |
|--------|------|-------------|
| `state` | `TaskState \| undefined` | Current execution state |
| `transcript` | `TranscriptLine[]` | Streaming transcript lines |
| `isSubscribed` | `boolean` | Whether actively subscribed |
| `connectionStatus` | `ConnectionStatus` | WebSocket connection status |
| `clearTranscript()` | `void` | Clear transcript array |

#### useConnectionStatus

Simple hook for connection status only.

```tsx
import { useConnectionStatus } from '@/hooks';

function ConnectionIndicator() {
  const status = useConnectionStatus();
  return <span className={`indicator ${status}`} />;
}
```

#### Event Types

| Event | Data | Description |
|-------|------|-------------|
| `state` | `TaskState` | Task execution state update |
| `transcript` | `TranscriptLine` | New transcript line |
| `phase` | `{ phase, status }` | Phase transition |
| `tokens` | `TokenUsage` | Token usage update |
| `complete` | `{ status, phase? }` | Task completed |
| `finalize` | `{ step, status, progress? }` | Finalize phase update |
| `task_created` | `Task` | New task created (file watcher) |
| `task_updated` | `Task` | Task modified (file watcher) |
| `task_deleted` | `null` | Task deleted (file watcher) |
| `error` | `{ message }` | Error from server |

#### Connection Behavior

- **Auto-connect:** Connects on mount, subscribes to global events
- **Auto-reconnect:** Exponential backoff (1s, 2s, 4s...), max 5 attempts
- **Ping/pong:** 30s heartbeat to keep connection alive
- **Primary subscription:** Global subscription restored after reconnect
- **Store integration:** Events automatically update TaskStore and UIStore

#### OrcWebSocket Class (Internal)

The hooks wrap `OrcWebSocket` from `@/lib/websocket`. For most cases, use the hooks. Direct class usage is only needed for advanced scenarios outside React.

```typescript
import { OrcWebSocket, GLOBAL_TASK_ID } from '@/lib/websocket';

const ws = new OrcWebSocket();
ws.connect(GLOBAL_TASK_ID);  // Connect and subscribe to all events
ws.on('state', (event) => console.log(event));
ws.pause('TASK-001');  // Send pause command
ws.disconnect();  // Cleanup
```

### Lib Utilities

| File | Purpose |
|------|---------|
| `lib/types.ts` | TypeScript interfaces matching Go backend types |
| `lib/websocket.ts` | OrcWebSocket class for WebSocket connection management |
| `lib/shortcuts.ts` | ShortcutManager class for keyboard shortcuts |
| `lib/platform.ts` | Platform detection (isMac) and modifier key formatting |

### Keyboard Shortcuts

The keyboard shortcut system uses context and hooks pattern.

#### ShortcutProvider

Wraps the app at root level in `App.tsx`:

```tsx
<ShortcutProvider>
  <WebSocketProvider>{children}</WebSocketProvider>
</ShortcutProvider>
```

#### Hooks

| Hook | Purpose |
|------|---------|
| `useShortcuts()` | Access shortcut manager methods |
| `useShortcutContext(context)` | Set active context for a component |
| `useGlobalShortcuts(options)` | Register global shortcuts with navigation |
| `useTaskListShortcuts(options)` | Register task list shortcuts (j/k navigation) |

#### Global Shortcuts (Shift+Alt modifier)

| Shortcut | Action |
|----------|--------|
| `Shift+Alt+K` | Open command palette |
| `Shift+Alt+N` | Create new task |
| `Shift+Alt+B` | Toggle sidebar |
| `Shift+Alt+P` | Switch project |
| `/` | Focus search |
| `?` | Show keyboard help |
| `Escape` | Close modal |

#### Navigation Sequences

| Sequence | Destination |
|----------|-------------|
| `g d` | Dashboard |
| `g t` | Tasks |
| `g e` | Environment |
| `g r` | Preferences |
| `g p` | Prompts |
| `g h` | Hooks |
| `g k` | Skills |

#### Task List Shortcuts (context: 'tasks')

| Key | Action |
|-----|--------|
| `j` | Select next task |
| `k` | Select previous task |
| `Enter` | Open selected task |
| `r` | Run selected task |
| `p` | Pause selected task |
| `d` | Delete selected task |

#### Implementation Notes

- Uses `Shift+Alt` modifier instead of `Cmd/Ctrl` to avoid browser conflicts
- Multi-key sequences have 1000ms timeout window
- Shortcuts disabled in input/textarea fields (except Escape)
- Context system filters shortcuts by active context

#### Usage Example

```tsx
// In a component
import { useGlobalShortcuts, useTaskListShortcuts } from '@/hooks';

function TaskList() {
  useTaskListShortcuts({
    onNavDown: () => setSelectedIndex(i => i + 1),
    onNavUp: () => setSelectedIndex(i => Math.max(0, i - 1)),
    onOpen: () => navigate(`/tasks/${selectedTask.id}`),
    onRun: () => runTask(selectedTask.id),
  });

  // ...
}
```

## Board Components

Components for the Kanban board page (`/board`).

### Board (Page)

Page component that orchestrates the board display with data loading, filtering, and action handling.

```tsx
import { Board } from '@/pages/Board';

// Used in route configuration
<Route path="/board" element={<Board />} />
```

**Features:**
- View mode persistence (localStorage)
- Initiative filtering (URL + store sync)
- Task actions (run/pause/resume/escalate)
- Drag-drop handling for status/initiative changes
- Loading/error/empty states

**URL params:**
- `project`: Project filter (handled by UrlParamSync)
- `initiative`: Initiative filter
- `dependency_status`: Filter by blocked/ready

### Board (Component)

Main board component with flat and swimlane view modes.

```tsx
import { Board, BOARD_COLUMNS, type BoardViewMode } from '@/components/board';

<Board
  tasks={tasks}
  viewMode="flat"               // or "swimlane"
  initiatives={initiatives}
  onAction={handleAction}       // run/pause/resume
  onEscalate={handleEscalate}   // escalation with reason
  onTaskClick={handleTaskClick} // for running tasks modal
  onFinalizeClick={handleFinalize}
  onInitiativeClick={handleInitiativeClick}
  onInitiativeChange={handleInitiativeChange} // drag-drop initiative change
  getFinalizeState={getFinalizeState}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `tasks` | `Task[]` | Tasks to display |
| `viewMode` | `'flat' \| 'swimlane'` | View mode (default: flat) |
| `initiatives` | `Initiative[]` | For swimlane grouping |
| `onAction` | `(id, action) => Promise` | Run/pause/resume handler |
| `onEscalate` | `(id, reason) => Promise` | Escalation handler (optional) |
| `onTaskClick` | `(task) => void` | Task click handler |
| `onFinalizeClick` | `(task) => void` | Finalize button handler |
| `onInitiativeClick` | `(id) => void` | Initiative badge click |
| `onInitiativeChange` | `(taskId, initId) => Promise` | Initiative change via drag |
| `getFinalizeState` | `(id) => FinalizeState` | Get finalize state for task |

**Column Configuration (`BOARD_COLUMNS`):**

| Column ID | Title | Phases |
|-----------|-------|--------|
| `queued` | Queued | (none - uses status) |
| `spec` | Spec | research, spec, design |
| `implement` | Implement | implement |
| `test` | Test | test |
| `review` | Review | docs, validate, review |
| `done` | Done | (terminal statuses) |

**View Modes:**
- **Flat**: Traditional kanban columns side by side
- **Swimlane**: Horizontal rows grouped by initiative with collapsible headers

**Task Sorting:** Running tasks first, then by priority (critical > high > normal > low)

### Column

Standard board column with header and task cards.

```tsx
import { Column, type ColumnConfig } from '@/components/board';

<Column
  column={{ id: 'implement', title: 'Implement', phases: ['implement'] }}
  tasks={tasks}
  onDrop={handleDrop}
  onAction={handleAction}
  onTaskClick={handleTaskClick}
/>
```

**Features:**
- Column-specific accent colors
- Drag-over visual feedback (counter-based for nested elements)
- Empty state when no tasks

**Column Styles:**

| Column | Accent Color |
|--------|--------------|
| queued | muted gray |
| spec | blue |
| implement | purple (accent) |
| test | cyan |
| review | warning yellow |
| done | success green |

### QueuedColumn

Special column for queued tasks with active/backlog sections.

```tsx
import { QueuedColumn } from '@/components/board';

<QueuedColumn
  column={column}
  activeTasks={activeTasks}    // queue !== 'backlog'
  backlogTasks={backlogTasks}  // queue === 'backlog'
  showBacklog={showBacklog}
  onToggleBacklog={toggleBacklog}
  onDrop={handleDrop}
  onAction={handleAction}
/>
```

**Features:**
- Active section always visible
- Backlog section collapsible with toggle button
- Backlog count badge
- State persisted to localStorage (`orc-show-backlog`)

### Swimlane

Initiative row for swimlane view with all columns.

```tsx
import { Swimlane } from '@/components/board';

<Swimlane
  initiative={initiative}       // null for unassigned
  tasks={tasks}
  columns={BOARD_COLUMNS}
  tasksByColumn={tasksByColumn}
  collapsed={isCollapsed}
  onToggleCollapse={toggle}
  onDrop={handleSwimlaneDrop}   // receives targetInitiativeId
  onAction={handleAction}
/>
```

**Features:**
- Collapsible header with chevron icon
- Progress bar (completed/total tasks)
- Progress percentage display
- Keyboard accessible toggle (Enter/Space)
- Unassigned swimlane for tasks without initiative

### TaskCard

Task card for kanban board with full feature set.

```tsx
import { TaskCard } from '@/components/board';

<TaskCard
  task={task}
  onAction={handleAction}
  onTaskClick={handleTaskClick}     // Opens transcript modal for running
  onFinalizeClick={handleFinalize}  // Opens finalize modal
  onInitiativeClick={handleInitiativeClick}
  finalizeState={finalizeState}
/>
```

**Display Elements:**
- Task ID and priority badge (critical/high/low icons)
- Status indicator (colored orb with animation)
- Title and description preview
- Current phase (when running)
- Weight badge with color coding
- Blocked badge (when is_blocked)
- Initiative badge (clickable, truncated with tooltip)
- Relative timestamp

**Action Buttons (contextual):**
- **Run** (play icon): created/planned status
- **Pause** (pause icon): running status
- **Resume** (play icon): paused status
- **Finalize** (merge icon): completed status
- **Quick menu** (three dots): queue/priority changes

**Visual States:**
- **Running**: Pulsing border animation
- **Finalizing**: Progress bar with step label
- **Finished**: Merge info (commit SHA + target branch)
- **Dragging**: Reduced opacity

**Quick Menu:**
- Queue selection (Active/Backlog)
- Priority selection (Critical/High/Normal/Low)
- Updates via API and store

**Drag-Drop:**
- Native HTML5 drag-drop
- Sets `application/json` data with task object
- Visual feedback on drag start/end

### ViewModeDropdown

Dropdown to toggle between flat and swimlane views.

```tsx
import { ViewModeDropdown } from '@/components/board';

<ViewModeDropdown
  value={viewMode}
  onChange={setViewMode}
  disabled={initiativeFilterActive}  // Swimlane disabled when filtered
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `value` | `'flat' \| 'swimlane'` | Current view mode |
| `onChange` | `(mode) => void` | Change handler |
| `disabled` | `boolean` | Disable dropdown |

**Options:**
- **Flat**: All tasks in columns
- **By Initiative**: Grouped by initiative (swimlane)

### InitiativeDropdown

Dropdown to filter tasks by initiative.

```tsx
import { InitiativeDropdown } from '@/components/board';

<InitiativeDropdown
  currentInitiativeId={currentInitiativeId}
  onSelect={setInitiativeFilter}
  tasks={tasks}  // For task counts
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `currentInitiativeId` | `string \| null` | Current filter |
| `onSelect` | `(id) => void` | Selection handler |
| `tasks` | `Task[]` | For calculating task counts |

**Options:**
- **All initiatives**: No filter (null)
- **Unassigned**: Tasks without initiative (UNASSIGNED_INITIATIVE constant)
- **Initiative list**: Sorted (active first, then alphabetical) with task counts

**Features:**
- Task counts per initiative
- Title truncation with tooltip
- Active filter visual indication
- Click outside to close

## Task Detail Components

Components for the Task Detail page (`/tasks/:id`).

### TaskDetail (Page)

Main page component that orchestrates task display with all tabs and real-time updates.

```tsx
import { TaskDetail } from '@/pages/TaskDetail';

// Used in route configuration
<Route path="/tasks/:id" element={<TaskDetail />} />
```

**Features:**
- Loads task, plan, and state data on mount
- Tab navigation with URL persistence (`?tab=xxx`)
- Real-time WebSocket subscription for running tasks
- Collapsible dependencies sidebar
- Task actions (run/pause/resume/delete)

**URL params:**
- `id`: Task ID (route param)
- `tab`: Active tab (timeline, changes, transcript, test-results, attachments, comments)

**Data flow:**
- Fetches task data via `/api/tasks/:id`
- Fetches plan via `/api/tasks/:id/plan`
- Subscribes to task via `useTaskSubscription(id)`
- Updates store from WebSocket events

### TaskHeader

Header component with task metadata, status, and action buttons.

```tsx
import { TaskHeader } from '@/components/task-detail';

<TaskHeader
  task={task}
  taskState={taskState}
  plan={plan}
  onRun={handleRun}
  onPause={handlePause}
  onResume={handleResume}
  onDelete={handleDelete}
  onEdit={() => setShowEditModal(true)}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `task` | `Task` | Task data |
| `taskState` | `TaskState \| undefined` | Execution state |
| `plan` | `Plan \| undefined` | Phase plan |
| `onRun` | `() => void` | Run task handler |
| `onPause` | `() => void` | Pause handler |
| `onResume` | `() => void` | Resume handler |
| `onDelete` | `() => void` | Delete handler |
| `onEdit` | `() => void` | Open edit modal |

**Display elements:**
- Back navigation link
- Task ID and status indicator
- Weight badge with color coding
- Category and priority badges
- Initiative badge (if assigned)
- Branch name display
- Phase progress (e.g., "3/6")

**Action buttons (contextual):**
- **Run**: For created/planned tasks
- **Pause**: For running tasks
- **Resume**: For paused tasks
- **Edit**: Opens TaskEditModal
- **Delete**: With confirmation dialog

### TabNav

Tab navigation component with 6 tabs.

```tsx
import { TabNav, type TabId } from '@/components/task-detail';

<TabNav
  activeTab={activeTab}
  onTabChange={handleTabChange}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `activeTab` | `TabId` | Current active tab |
| `onTabChange` | `(tab: TabId) => void` | Tab change handler |

**Tab configuration:**

| Tab ID | Label | Icon |
|--------|-------|------|
| `timeline` | Timeline | clock |
| `changes` | Changes | branch |
| `transcript` | Transcript | file-text |
| `test-results` | Test Results | check-circle |
| `attachments` | Attachments | folder |
| `comments` | Comments | message-circle |

### DependencySidebar

Collapsible sidebar showing task dependencies.

```tsx
import { DependencySidebar } from '@/components/task-detail';

<DependencySidebar
  task={task}
  collapsed={sidebarCollapsed}
  onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
  onUpdate={handleTaskUpdate}
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `task` | `Task` | Task with dependency fields |
| `collapsed` | `boolean` | Collapsed state |
| `onToggle` | `() => void` | Toggle handler |
| `onUpdate` | `(task: Task) => void` | Update callback |

**Sections:**
- **Blocked By**: Tasks blocking this one (removable via API)
- **Blocks**: Tasks this one blocks (computed, read-only)
- **Related To**: Related tasks (removable)
- **Referenced By**: Tasks mentioning this one (computed, read-only)

**Features:**
- Expand/collapse toggle with chevron icon
- Add blocker/related task via modal
- Remove with single click
- Status indicators for each dependency
- Click to navigate to dependency

### TimelineTab

Phase execution timeline with token usage stats.

```tsx
import { TimelineTab } from '@/components/task-detail';

<TimelineTab
  task={task}
  taskState={taskState}
  plan={plan}
/>
```

**Features:**
- Horizontal phase flow visualization
- Phase status icons (pending/running/completed/failed/skipped)
- Phase connector lines
- Duration display per phase
- Iteration/retry counts
- Error messages for failed phases
- Commit SHA links for completed phases

**Token Stats Panel:**
- Total input/output tokens
- Cache creation/read tokens
- Cache hit rate percentage
- Per-phase token breakdown

**Task Info Section:**
- Weight classification
- Status with timestamp
- Created/started/completed dates

### ChangesTab

Git diff viewer with inline review comments.

```tsx
import { ChangesTab } from '@/components/task-detail';

<ChangesTab taskId={taskId} />
```

**Features:**
- Split/unified view mode toggle
- File list with expand/collapse all
- Lazy-loaded file hunks (fetch on expand)
- Diff statistics (additions, deletions)
- Line numbers with syntax context
- Review comments at specific lines
- Comment severity levels (blocker, issue, suggestion)
- "Send to Agent" for retry with feedback
- General comments section

**Sub-components:**
- `DiffFile` - File container with expand/collapse
- `DiffHunk` - Code hunk with context lines
- `DiffLine` - Individual line with optional comments
- `DiffStats` - Addition/deletion counts
- `InlineCommentThread` - Comments on specific lines

### TranscriptTab

Transcript viewer with pagination and streaming support.

```tsx
import { TranscriptTab } from '@/components/task-detail';

<TranscriptTab taskId={taskId} isRunning={isRunning} />
```

| Prop | Type | Description |
|------|------|-------------|
| `taskId` | `string` | Task ID |
| `isRunning` | `boolean` | Show streaming content |

**Features:**
- Paginated transcript list (10 per page)
- Expand/collapse individual transcripts
- Auto-expand on initial load
- Section types: prompt, retry-context, response, metadata
- Token counts per turn (input, output, cached)
- Status badges (complete, blocked)
- Live streaming content for running tasks
- Export to markdown
- Copy to clipboard
- Auto-scroll toggle
- Relative time formatting

**Parsed transcript structure:**
```typescript
interface ParsedTranscript {
  phase: string;
  iteration: number;
  sections: ParsedSection[];
  metadata: {
    inputTokens: number;
    outputTokens: number;
    cacheCreationTokens: number;
    cacheReadTokens: number;
    complete: boolean;
    blocked: boolean;
  };
}
```

### TestResultsTab

Test results display with screenshots and report links.

```tsx
import { TestResultsTab } from '@/components/task-detail';

<TestResultsTab taskId={taskId} />
```

**Features:**
- Summary metrics: passed, failed, skipped, total
- Pass rate bar with color coding (green/yellow/red)
- Code coverage breakdown (lines, branches, functions)
- Screenshot gallery with lightbox modal
- Test suites with individual test results
- Quick links to HTML report and trace files
- Lazy-loaded images

**Tab navigation within component:**
- **Summary**: Overview metrics and coverage
- **Screenshots**: Gallery view with lightbox
- **Test Suites**: Detailed test breakdown

### AttachmentsTab

File attachments with upload and gallery view.

```tsx
import { AttachmentsTab } from '@/components/task-detail';

<AttachmentsTab taskId={taskId} />
```

**Features:**
- Drag-and-drop file upload
- Multi-file upload support
- Image gallery with lightbox
- File list with metadata (size, date)
- File type detection (image vs document)
- Delete with confirmation
- Lazy-loaded images
- Upload progress feedback
- Error handling with toast notifications

**State management:**
- `dragOver` - Visual feedback during drag
- `uploading` - Upload progress state
- `lightboxImage` - Current lightbox image

### CommentsTab

Task discussion with author classification.

```tsx
import { CommentsTab } from '@/components/task-detail';

<CommentsTab taskId={taskId} />
```

**Features:**
- Author type classification: human, agent, system
- Phase-scoped comments (optional)
- Custom author names
- Edit/delete functionality
- Filter by author type
- Comment counts per type
- Relative time formatting
- Edit mode with cancel/save
- Keyboard shortcuts: Cmd/Ctrl+Enter to submit, Escape to cancel

**Comment form:**
- Author type selector dropdown
- Optional phase association
- Custom author name field
- Textarea with auto-focus

### TaskEditModal

Modal form for editing task metadata.

```tsx
import { TaskEditModal } from '@/components/task-detail';

<TaskEditModal
  task={task}
  open={showEditModal}
  onClose={() => setShowEditModal(false)}
  onSave={handleSave}
/>
```

**Editable fields:**
- Title
- Description
- Weight (trivial, small, medium, large, greenfield)
- Priority (critical, high, normal, low)
- Category (feature, bug, refactor, chore, docs, test)
- Queue (active, backlog)
- Initiative (dropdown with search)

### ExportDropdown

Export options for task data.

```tsx
import { ExportDropdown } from '@/components/task-detail';

<ExportDropdown
  taskId={taskId}
  onExport={handleExport}
/>
```

**Export formats:**
- JSON (task data)
- Markdown (formatted report)
- Transcript (raw transcript files)

### Diff Sub-components

Located in `components/task-detail/diff/`:

| Component | Purpose |
|-----------|---------|
| `DiffStats.tsx` | File statistics (additions, deletions, file count) |
| `DiffFile.tsx` | File container with header and hunks |
| `DiffHunk.tsx` | Code hunk with context lines |
| `DiffLine.tsx` | Single line with type styling and optional comments |
| `InlineCommentThread.tsx` | Review comments at specific line |

**DiffLine types:**
- `added` - Green background, "+" prefix
- `removed` - Red background, "-" prefix
- `context` - Normal background, " " prefix
- `hunk-header` - Blue background, "@@ ... @@" format

**Review comment severity:**
- `blocker` - Red, must fix before merge
- `issue` - Orange, should fix
- `suggestion` - Blue, optional improvement

## Known Differences from Svelte

| Aspect | Svelte 5 | React 19 |
|--------|----------|----------|
| Reactivity | `$state()`, `$derived()` | `useState()`, `useMemo()` |
| Props | `$props()` | Destructured props |
| Events | `onclick` | `onClick` |
| Two-way binding | `bind:value` | `value` + `onChange` |
| Stores | Svelte stores | Zustand stores |
| Routing | SvelteKit (`+page.svelte`) | React Router (`useRoutes`) |
| URL params | `$page.url.searchParams` | `useSearchParams()` |
| Route params | `$page.params` | `useParams()` |
| Navigation | `goto()` | `useNavigate()` |
| Active links | `$page.url.pathname` | `NavLink` with `isActive` |

## Dependencies

### Production
- `react@19`, `react-dom@19` - Core framework
- `react-router-dom@7` - Client-side routing
- `zustand@5` - State management with subscribeWithSelector middleware
- `@fontsource/inter`, `@fontsource/jetbrains-mono` - Typography (matching Svelte)

### Development
- `vite`, `@vitejs/plugin-react` - Build tooling
- `typescript`, `@types/react*` - Type safety
- `vitest`, `@testing-library/*`, `jsdom` - Testing
