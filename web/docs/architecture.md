# Architecture

## State Management (Zustand)

Six stores with `subscribeWithSelector` middleware for efficient derived state.

| Store | State | Persistence | Purpose |
|-------|-------|-------------|---------|
| `taskStore` | tasks, taskStates | None (API) | Task data and execution |
| `projectStore` | projects, currentProjectId | URL + localStorage | Project selection |
| `initiativeStore` | initiatives, currentInitiativeId | URL + localStorage | Initiative filter |
| `dependencyStore` | currentDependencyStatus | URL + localStorage | Dependency filter |
| `sessionStore` | duration, tokens, cost, isPaused | localStorage | Session metrics |
| `uiStore` | sidebarExpanded, wsStatus, toasts | localStorage (sidebar) | UI state |

### Store Patterns

```tsx
// Subscribe to specific state
const tasks = useTaskStore(state => state.tasks);
const setTasks = useTaskStore(state => state.setTasks);

// Selector with equality check
const runningTasks = useTaskStore(
  state => state.tasks.filter(t => t.status === 'running'),
  shallow
);

// Cross-store actions
projectStore.subscribe(
  state => state.currentProjectId,
  (projectId) => taskStore.getState().reset()
);
```

## WebSocket Integration

Real-time updates via `OrcWebSocket` class and React hooks.

### Hooks

| Hook | Purpose |
|------|---------|
| `useWebSocket()` | Access WebSocket instance |
| `useTaskSubscription(callback)` | Subscribe to task events |
| `useConnectionStatus()` | Get connection state |

### Event Types

| Event | Payload |
|-------|---------|
| `task_created` | Task |
| `task_updated` | Task |
| `task_deleted` | `{ id: string }` |
| `state_updated` | TaskState |
| `transcript` | `{ task_id, content, tokens }` |
| `finalize` | `{ task_id, status, step, ... }` |

### Connection Behavior

- Auto-reconnect with exponential backoff (max 30s)
- Heartbeat ping every 30s
- Broadcasts status changes to subscribers

## Routing

React Router 7 with URL/store synchronization.

### Routes

| Path | Component | Purpose |
|------|-----------|---------|
| `/` | - | Redirects to `/board` |
| `/board` | Board | Kanban board |
| `/stats` | Dashboard | Overview stats |
| `/initiatives` | InitiativesPage | Initiatives overview with stats and cards |
| `/initiatives/:id` | InitiativeDetail | Initiative management |
| `/agents` | Agents | Agent configuration |
| `/settings` | SettingsPage | Redirects to `/settings/commands` |
| `/settings/*` | SettingsLayout + children | Settings with 240px sidebar navigation |
| `/tasks/:id` | TaskDetail | Task detail page |

**Settings Routes:**
| Path | Component | Section |
|------|-----------|---------|
| `/settings/commands` | SettingsView | Slash commands editor |
| `/settings/claude-md` | SettingsPlaceholder | CLAUDE.md editor |
| `/settings/mcp` | SettingsPlaceholder | MCP servers |
| `/settings/memory` | SettingsPlaceholder | Memory management |
| `/settings/permissions` | SettingsPlaceholder | Permissions |
| `/settings/projects` | SettingsPlaceholder | Projects |
| `/settings/billing` | SettingsPlaceholder | Billing & Usage |
| `/settings/import-export` | SettingsPlaceholder | Import / Export |
| `/settings/profile` | SettingsPlaceholder | Profile |
| `/settings/api-keys` | SettingsPlaceholder | API Keys |

**Legacy Routes:** `/dashboard` redirects to `/stats`, `/environment/*` redirects to `/settings`.

### URL Parameter Sync

`UrlParamSync` component bidirectionally syncs URL params with stores:
- `?project=xxx` <-> projectStore.currentProjectId
- `?initiative=xxx` <-> initiativeStore.currentInitiativeId
- `?dependency_status=xxx` <-> dependencyStore.currentDependencyStatus

URL takes precedence over localStorage on page load.

## Keyboard Shortcuts

`ShortcutManager` with sequence support and context awareness.

### Global Shortcuts (Shift+Alt modifier)

| Shortcut | Action |
|----------|--------|
| `Shift+Alt+K` | Command palette |
| `Shift+Alt+N` | New task |
| `Shift+Alt+P` | Project switcher |
| `Shift+Alt+B` | Toggle sidebar |
| `?` | Keyboard shortcuts help |

### Navigation Sequences

| Sequence | Destination |
|----------|-------------|
| `g d` | Dashboard |
| `g t` | Tasks |
| `g b` | Board |

### Task List Shortcuts (context: 'tasks')

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate down/up |
| `Enter` | Open selected task |

### Implementation

```tsx
// Register shortcuts in component
useGlobalShortcuts(navigate, {
  onCommandPalette: () => setShowPalette(true),
  onNewTask: () => setShowNewTask(true),
  ...
});

// Task-specific shortcuts
useTaskListShortcuts(tasks, selectedIndex, setSelectedIndex, navigate);
```

Shortcuts disabled when focus is in text inputs or textareas.

## Data Loading

`DataProvider` handles centralized data loading:

1. On mount: Initialize stores from URL, load projects
2. On project change: Clear data, load new project's tasks/initiatives
3. On popstate: Sync stores with browser history
