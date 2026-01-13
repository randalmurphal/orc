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
│   │   ├── comments/     # TaskCommentsPanel, TaskCommentThread, TaskCommentForm
│   │   ├── dashboard/    # Stats, actions, activity
│   │   ├── diff/         # DiffViewer, DiffFile, DiffHunk, VirtualScroller
│   │   ├── kanban/       # Board, Column, TaskCard
│   │   ├── layout/       # Header, Sidebar
│   │   ├── overlays/     # Modal, CommandPalette, NewTaskModal, KeyboardShortcutsHelp
│   │   ├── review/       # CommentForm, CommentThread, ReviewPanel
│   │   ├── task/         # TaskHeader, TaskEditModal, Timeline, Transcript, RetryPanel, Attachments
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
| Task | TaskCard, Timeline, Transcript, TaskHeader, TaskEditModal, PRActions, Attachments | Task detail |
| Diff | DiffViewer, DiffFile, DiffHunk, DiffLine, VirtualScroller | Changes tab |
| Kanban | Board, Column, TaskCard, ConfirmModal | Board view |
| Comments | TaskCommentsPanel, TaskCommentThread, TaskCommentForm | Task discussion notes |
| Review | CommentForm, CommentThread, ReviewPanel, ReviewSummary | Code review comments |
| UI | Icon (34 icons), StatusIndicator, Toast, Modal | Shared components |

## State Management

| Store | Purpose |
|-------|---------|
| `tasks` | Global reactive task state, WebSocket updates |
| `project` | Current project selection with persistence |
| `sidebar` | Collapsed state |
| `toast` | Notification queue |

**Task store** initialized in `+layout.svelte`, synced via global WebSocket. Pages subscribe for reactive updates.

### Project Selection

Project selection uses a 3-tier fallback system:

1. **localStorage** (`orc_current_project_id`) - User's last selection in this browser
2. **Server default** (`GET /api/projects/default`) - Global default from `~/.orc/projects.yaml`
3. **First project** - Falls back to first available project

This allows the server to run from any directory while the UI remembers the user's project choice. Use `setDefaultProject(id)` to persist a default server-side.

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
| `NewTaskModal` | `orc:new-task` | `Cmd+N` |
| `CommandPalette` | `orc:command-palette` | `Cmd+K` |
| `KeyboardShortcutsHelp` | `orc:show-shortcuts` | `?` |

**To trigger from any page:**
```svelte
window.dispatchEvent(new CustomEvent('orc:new-task'));
```

Page-specific modals (like `TaskEditModal`) can live in individual routes.

## WebSocket Architecture

Global WebSocket in `+layout.svelte` subscribes with `"*"`. All task events flow to task store. Pages react to store changes - no individual WebSocket connections needed.

See `QUICKREF.md` for subscription helpers.

## Attachments

Task attachments (images, files) are managed through the Attachments component:
- Drag-and-drop upload with visual feedback
- Image gallery with thumbnails and lightbox viewer
- File list with metadata (size, date)
- Supports delete with confirmation

API functions: `listAttachments()`, `uploadAttachment()`, `getAttachmentUrl()`, `deleteAttachment()`

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
| `/tasks/:id` | Task detail (Timeline/Changes/Transcript/Attachments tabs) |
| `/environment` | Environment hub (Claude Code + Orchestrator config) |
| `/environment/docs` | CLAUDE.md editor (`?scope=global\|user\|project`) |
| `/environment/claude/skills` | Skills (`?scope=global`) |
| `/environment/claude/hooks` | Hooks (`?scope=global`) |
| `/environment/claude/agents` | Agents (`?scope=global`) |
| `/environment/claude/mcp` | MCP servers (`?scope=global`) |
| `/preferences` | User preferences |

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
