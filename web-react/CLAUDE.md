# React 19 Frontend (Migration)

React 19 application for orc web UI, running in parallel with existing Svelte app during migration.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | React 19, Vite |
| Language | TypeScript 5.6+ |
| State Management | Zustand |
| Routing | React Router (planned) |
| Styling | CSS (component-scoped, matching Svelte) |
| Testing | Vitest (unit), Playwright (E2E - shared) |

## Directory Structure

```
web-react/src/
├── main.tsx              # Entry point
├── App.tsx               # Root component
├── index.css             # Global styles
├── lib/                  # Shared utilities
├── components/           # UI components
├── pages/                # Route pages
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
2. **Phase 2** (current): Core infrastructure (API client, WebSocket ✅, router)
3. **Phase 3**: Component migration (parallel implementation)
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
| `+layout.svelte` | `App.tsx` + Router | Scaffolded |
| `lib/components/` | `src/components/` | Pending |
| `lib/stores/` | `src/stores/` (Zustand) | ✅ Complete |
| `lib/websocket.ts` | `src/lib/websocket.ts` | ✅ Complete |
| `lib/utils/` | `src/lib/` | In Progress |

**Stores implemented (Phase 1):**
- `taskStore.ts` - Task data and execution state with derived selectors
- `projectStore.ts` - Project selection with URL/localStorage sync
- `initiativeStore.ts` - Initiative filter with progress tracking
- `uiStore.ts` - Sidebar, WebSocket status, toast notifications

**WebSocket hooks implemented (Phase 2):**
- `useWebSocket.tsx` - WebSocketProvider, useWebSocket, useTaskSubscription, useConnectionStatus

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

Four Zustand stores mirror the Svelte store behavior. All use `subscribeWithSelector` middleware for efficient derived state.

#### Store Overview

| Store | State | Persistence | Purpose |
|-------|-------|-------------|---------|
| `useTaskStore` | tasks, taskStates | None (API-driven) | Task data and execution state |
| `useProjectStore` | projects, currentProjectId | URL + localStorage | Project selection |
| `useInitiativeStore` | initiatives, currentInitiativeId | URL + localStorage | Initiative filter |
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

#### Usage Examples

```tsx
import { useTaskStore, useProjectStore, useInitiativeStore, useUIStore, toast } from '@/stores';

// Direct state access
const tasks = useTaskStore((state) => state.tasks);

// Derived state via selector hooks
import { useActiveTasks, useStatusCounts } from '@/stores';
const activeTasks = useActiveTasks();
const counts = useStatusCounts();

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

## Known Differences from Svelte

| Aspect | Svelte 5 | React 19 |
|--------|----------|----------|
| Reactivity | `$state()`, `$derived()` | `useState()`, `useMemo()` |
| Props | `$props()` | Destructured props |
| Events | `onclick` | `onClick` |
| Two-way binding | `bind:value` | `value` + `onChange` |
| Stores | Svelte stores | Zustand stores |
| Routing | SvelteKit | React Router |

## Dependencies

### Production
- `react@19`, `react-dom@19` - Core framework
- `zustand@5` - State management with subscribeWithSelector middleware
- `@fontsource/inter`, `@fontsource/jetbrains-mono` - Typography (matching Svelte)

### Development
- `vite`, `@vitejs/plugin-react` - Build tooling
- `typescript`, `@types/react*` - Type safety
- `vitest`, `@testing-library/*`, `jsdom` - Testing
