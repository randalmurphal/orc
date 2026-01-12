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
│   │   ├── dashboard/    # Stats, actions, activity
│   │   ├── diff/         # DiffViewer, DiffFile, DiffHunk, VirtualScroller
│   │   ├── kanban/       # Board, Column, TaskCard
│   │   ├── layout/       # Header, Sidebar
│   │   ├── overlays/     # Modal, CommandPalette
│   │   ├── task/         # TaskHeader, Timeline, Transcript, RetryPanel
│   │   └── ui/           # Icon, StatusIndicator, Toast
│   ├── stores/           # tasks.ts, project.ts, sidebar.ts, toast.svelte.ts
│   ├── utils/            # format.ts, status.ts, platform.ts
│   ├── api.ts            # API client
│   ├── websocket.ts      # WebSocket client
│   └── shortcuts.ts      # Keyboard shortcuts
└── routes/               # SvelteKit pages
```

## Key Components

| Category | Components | Purpose |
|----------|------------|---------|
| Layout | Header, Sidebar | Navigation, project switcher |
| Dashboard | Stats, QuickActions, ActiveTasks, RecentActivity | Overview page |
| Task | TaskCard, Timeline, Transcript, TaskHeader, PRActions | Task detail |
| Diff | DiffViewer, DiffFile, DiffHunk, DiffLine, VirtualScroller | Changes tab |
| Kanban | Board, Column, TaskCard, ConfirmModal | Board view |
| UI | Icon (34 icons), StatusIndicator, Toast, Modal | Shared components |

## State Management

| Store | Purpose |
|-------|---------|
| `tasks` | Global reactive task state, WebSocket updates |
| `project` | Current project |
| `sidebar` | Collapsed state |
| `toast` | Notification queue |

**Task store** initialized in `+layout.svelte`, synced via global WebSocket. Pages subscribe for reactive updates.

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Cmd+K` | Command palette |
| `Cmd+N` | New task |
| `Cmd+B` | Toggle sidebar |
| `Cmd+P` | Project switcher |
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

```svelte
<script>
  let { data } = $props()        // Props (not export let)
  let count = $state(0)          // Reactive state
  let doubled = $derived(count * 2)  // Derived value
</script>
```

## WebSocket Architecture

Global WebSocket in `+layout.svelte` subscribes with `"*"`. All task events flow to task store. Pages react to store changes - no individual WebSocket connections needed.

See `QUICKREF.md` for subscription helpers.

## Review Workflow

"Changes" tab combines diff + inline review:
1. View diff (split/unified toggle)
2. Click line number → comment form
3. Set severity (Suggestion/Issue/Blocker)
4. "Send to Agent" → triggers retry with context

See `QUICKREF.md` for component hierarchy.

## Routes

| Route | Page |
|-------|------|
| `/` | Dashboard |
| `/board` | Kanban board |
| `/tasks` | Task list |
| `/tasks/:id` | Task detail (Timeline/Changes/Transcript tabs) |
| `/settings` | Settings hub (Quick Access/Orc Config/Claude Settings) |
| `/prompts`, `/hooks`, `/skills` | Configuration editors |
| `/environment/knowledge` | Knowledge queue |

## API Client

See `QUICKREF.md` for full function list.

```typescript
// Common patterns
await listTasks(projectId?)
await runTask(taskId, projectId?)
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
```

Test files: `tests/e2e/tasks.spec.ts`, `navigation.spec.ts`

## Deep-Dive Reference

See `QUICKREF.md` for:
- Virtual scrolling pattern
- Kanban board phase mapping
- WebSocket subscription helpers
- Task store actions
- API client functions
- Utility functions
- Component gotchas
