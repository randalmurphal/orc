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
│   │   │   ├── diff/         # Diff visualization
│   │   │   │   ├── DiffViewer.svelte
│   │   │   │   ├── DiffFile.svelte
│   │   │   │   ├── DiffHunk.svelte
│   │   │   │   ├── DiffLine.svelte
│   │   │   │   ├── DiffStats.svelte
│   │   │   │   └── VirtualScroller.svelte
│   │   │   ├── kanban/       # Kanban board
│   │   │   │   ├── Board.svelte
│   │   │   │   ├── Column.svelte
│   │   │   │   ├── TaskCard.svelte
│   │   │   │   └── ConfirmModal.svelte
│   │   │   ├── layout/       # Header, Sidebar
│   │   │   ├── overlays/     # Modal, CommandPalette
│   │   │   ├── review/       # Code review UI
│   │   │   │   ├── ReviewPanel.svelte
│   │   │   │   ├── CommentThread.svelte
│   │   │   │   ├── CommentForm.svelte
│   │   │   │   └── ReviewSummary.svelte
│   │   │   ├── task/         # Task detail components
│   │   │   │   ├── TaskHeader.svelte
│   │   │   │   ├── TaskTabs.svelte
│   │   │   │   └── RetryPanel.svelte
│   │   │   ├── team/         # Team components
│   │   │   │   ├── MemberAvatar.svelte
│   │   │   │   └── ActivityFeed.svelte
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
│       ├── board/            # Kanban board (/board)
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
| `TaskHeader.svelte` | Task title, status, actions |
| `TaskTabs.svelte` | Tab navigation (Timeline/Diff/Review/Transcript) |
| `RetryPanel.svelte` | Retry configuration with context injection |

### Diff Components

| Component | Purpose |
|-----------|---------|
| `DiffViewer.svelte` | Main diff container with split/unified toggle |
| `DiffFile.svelte` | Single file diff with expand/collapse |
| `DiffHunk.svelte` | Code hunk with line numbers |
| `DiffLine.svelte` | Line with syntax highlighting |
| `DiffStats.svelte` | Additions/deletions summary |
| `VirtualScroller.svelte` | Virtual scrolling for 10K+ lines |

### Review Components

| Component | Purpose |
|-----------|---------|
| `ReviewPanel.svelte` | Side panel with all comments |
| `CommentThread.svelte` | Single comment thread |
| `CommentForm.svelte` | Add/edit comment form |
| `ReviewSummary.svelte` | Findings summary with severity counts |

### Kanban Components

| Component | Purpose |
|-----------|---------|
| `Board.svelte` | Main board with drag-drop |
| `Column.svelte` | Status column (To Do, In Progress, etc.) |
| `TaskCard.svelte` | Draggable task card with actions |
| `ConfirmModal.svelte` | Action confirmation dialog |

### Team Components

| Component | Purpose |
|-----------|---------|
| `MemberAvatar.svelte` | Team member avatar with initials |
| `ActivityFeed.svelte` | Recent team activity stream |

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

// Diff
api.getTaskDiff(taskId, { base?, filesOnly? })
api.getTaskDiffFile(taskId, filePath)
api.getTaskDiffStats(taskId)

// Review Comments
api.getReviewComments(taskId)
api.createReviewComment(taskId, { file_path, line_number, content, severity })
api.updateReviewComment(taskId, commentId, { status, content? })
api.deleteReviewComment(taskId, commentId)
api.retryWithReview(taskId, { include_comments: true })

// GitHub Integration
api.createPR(taskId)
api.getPRDetails(taskId)
api.syncPRComments(taskId)
api.autofixPRComment(taskId, commentId)

// Team/Activity
api.getActivityLog(taskId?)
api.getTeamMembers()
api.claimTask(taskId)
api.releaseTask(taskId)
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

## Virtual Scrolling Pattern

For large diffs (10K+ lines), use `VirtualScroller.svelte`:

```svelte
<script lang="ts">
  let { items, itemHeight = 24, buffer = 10 } = $props();

  let containerEl = $state<HTMLElement>();
  let scrollTop = $state(0);
  let containerHeight = $state(0);

  const visibleStart = $derived(Math.max(0, Math.floor(scrollTop / itemHeight) - buffer));
  const visibleEnd = $derived(Math.min(items.length, visibleStart + Math.ceil(containerHeight / itemHeight) + buffer * 2));
  const visibleItems = $derived(items.slice(visibleStart, visibleEnd));
</script>

<div bind:this={containerEl} on:scroll={() => scrollTop = containerEl.scrollTop}>
  <div style="height: {visibleStart * itemHeight}px"></div>
  {#each visibleItems as item, i (visibleStart + i)}
    <slot {item} index={visibleStart + i} />
  {/each}
  <div style="height: {(items.length - visibleEnd) * itemHeight}px"></div>
</div>
```

## Kanban Board Patterns

### Column Mapping

```typescript
const columns = [
  { id: 'created', title: 'To Do', statuses: ['created', 'classifying', 'planned'] },
  { id: 'running', title: 'In Progress', statuses: ['running'] },
  { id: 'review', title: 'In Review', statuses: ['paused', 'blocked'] },
  { id: 'done', title: 'Done', statuses: ['completed', 'failed'] }
];
```

### Drag and Drop with Confirmation

Actions triggered by dropping tasks into columns require confirmation:
- Drop to "In Progress" → Confirm "Run task?"
- Drop to "In Review" → Confirm "Pause task?"
- Drop to "Done" → Confirm "Mark complete?"

Button actions on TaskCard are always available without drag-drop.

## Review Workflow

1. **Adding Comments**: Click line number in diff to add inline comment
2. **Batch Review**: Collect multiple comments, then "Send to Agent"
3. **Context Injection**: All open comments injected into retry context
4. **Resolution**: Mark comments as Resolved or Won't Fix after retry
