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
│   │   ├── kanban/       # Board, Column, QueuedColumn, TaskCard
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
| Task | TaskCard, Timeline, Transcript, TaskHeader, TaskEditModal, PRActions, Attachments, TokenUsage | Task detail |
| Diff | DiffViewer, DiffFile, DiffHunk, DiffLine, VirtualScroller | Changes tab |
| Kanban | Board, Column, QueuedColumn, TaskCard, ConfirmModal | Board view with queue/priority |
| Overlays | Modal, LiveTranscriptModal, CommandPalette, KeyboardShortcutsHelp | Modal dialogs and overlays |
| Comments | TaskCommentsPanel, TaskCommentThread, TaskCommentForm | Task discussion notes |
| Review | CommentForm, CommentThread, ReviewPanel, ReviewSummary | Code review comments |
| UI | Icon (34 icons), StatusIndicator, Toast, Modal | Shared components |

## State Management

| Store | Purpose |
|-------|---------|
| `tasks` | Global reactive task state, WebSocket updates |
| `project` | Current project selection with persistence |
| `sidebar` | Expanded/collapsed state (persisted in localStorage) |
| `toast` | Notification queue |

**Task store** initialized in `+layout.svelte`, synced via global WebSocket. Pages subscribe for reactive updates.

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

Tasks are sorted by priority within each column. Priority badges only appear for non-normal priorities.

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

### TaskCard Quick Menu

Right-click or use the "..." menu on TaskCard to:
- Move to Active/Backlog queue
- Set priority (Critical/High/Normal/Low)
- Set category (Feature/Bug/Refactor/Chore/Docs/Test)
- Run/Pause task actions

## Attachments

Task attachments (images, files) are managed through the Attachments component:
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
