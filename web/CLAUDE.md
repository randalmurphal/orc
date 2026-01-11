# Web Frontend

Svelte 5 SvelteKit application for the orc web UI.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | SvelteKit 2.x |
| UI Library | Svelte 5 (runes) |
| Styling | CSS (component-scoped) |
| Testing | Vitest (unit), Playwright (E2E) |
| Build | Vite |

## Directory Structure

```
web/
├── src/
│   ├── lib/
│   │   ├── components/       # Reusable components
│   │   │   ├── dashboard/    # Dashboard sub-components
│   │   │   ├── layout/       # Header, Sidebar
│   │   │   ├── overlays/     # Modal, CommandPalette
│   │   │   └── ui/           # Icon, StatusIndicator, Toast
│   │   ├── stores/           # Svelte stores
│   │   ├── utils/            # Utility functions
│   │   ├── api.ts            # API client
│   │   ├── websocket.ts      # WebSocket client
│   │   ├── shortcuts.ts      # Keyboard shortcuts
│   │   └── types.ts          # TypeScript types
│   └── routes/               # SvelteKit routes
│       ├── +layout.svelte    # App layout
│       ├── +page.svelte      # Dashboard (/)
│       ├── tasks/            # Task pages
│       ├── config/           # Configuration
│       ├── prompts/          # Prompt editor
│       ├── hooks/            # Hook management
│       ├── skills/           # Skill editor
│       └── ...
├── tests/
│   └── e2e/                  # Playwright tests
└── static/                   # Static assets
```

## Key Components

### Layout Components

| Component | Purpose |
|-----------|---------|
| `Header.svelte` | Top navigation, project switcher |
| `Sidebar.svelte` | Navigation menu with icons |

### Dashboard Components

| Component | Purpose |
|-----------|---------|
| `Dashboard.svelte` | Main dashboard container |
| `DashboardStats.svelte` | Quick stats cards |
| `DashboardQuickActions.svelte` | Action buttons |
| `DashboardActiveTasks.svelte` | Running/paused tasks |
| `DashboardRecentActivity.svelte` | Recent task activity |
| `DashboardSummary.svelte` | Task summary |

### UI Components

| Component | Purpose |
|-----------|---------|
| `Icon.svelte` | SVG icon component (34 icons) |
| `StatusIndicator.svelte` | Task/phase status badges |
| `ToastContainer.svelte` | Toast notifications |
| `Modal.svelte` | Modal dialogs |
| `CommandPalette.svelte` | Keyboard command palette |

### Task Components

| Component | Purpose |
|-----------|---------|
| `TaskCard.svelte` | Task list item |
| `Timeline.svelte` | Phase timeline visualization |
| `Transcript.svelte` | Claude conversation viewer |

## Utilities

### format.ts
```typescript
formatRelativeTime(date)    // "2 hours ago"
formatDuration(ms)          // "1h 23m"
formatCompactNumber(n)      // "1.2K"
```

### status.ts
```typescript
taskStatusStyles[status]    // { bg, text, border }
phaseStatusStyles[status]   // { bg, text }
weightStyles[weight]        // { bg, text }
```

### platform.ts
```typescript
isMac()                     // Detect macOS for shortcuts
```

## Stores

| Store | Purpose |
|-------|---------|
| `project.ts` | Current project state |
| `sidebar.ts` | Sidebar collapsed state |
| `toast.svelte.ts` | Toast notification queue |

## API Client (api.ts)

```typescript
// Tasks
api.listTasks(projectId?)
api.getTask(taskId, projectId?)
api.createTask(data, projectId?)
api.deleteTask(taskId, projectId?)
api.runTask(taskId, projectId?)
api.pauseTask(taskId, projectId?)
api.resumeTask(taskId, projectId?)

// Projects
api.listProjects()
api.getProject(id)
```

## WebSocket (websocket.ts)

Real-time updates for task events:
```typescript
const ws = createWebSocket()
ws.subscribe(taskId)
ws.onEvent((event) => { ... })
ws.unsubscribe()
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Cmd+K` | Command palette |
| `Cmd+N` | New task |
| `Cmd+B` | Toggle sidebar |
| `Cmd+P` | Project switcher |
| `g d` | Go to dashboard |
| `g t` | Go to tasks |
| `j/k` | Navigate task list |
| `Enter` | Open selected task |
| `r` | Run selected task |

## Development

```bash
# Install dependencies
bun install

# Development server
bun run dev

# Build
bun run build

# Unit tests
bun run test

# E2E tests
bunx playwright test
```

## Svelte 5 Patterns

Using Svelte 5 runes syntax:
```svelte
<script>
  let { data } = $props()       // Props
  let count = $state(0)         // Reactive state
  let doubled = $derived(count * 2)  // Derived value
</script>
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
bunx playwright test --ui  # Interactive mode
```

Test files in `tests/e2e/`:
- `tasks.spec.ts` - Task CRUD operations
- `navigation.spec.ts` - Routing and navigation
