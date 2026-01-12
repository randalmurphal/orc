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
| `Sidebar.svelte` | Minimal navigation: Dashboard, Tasks, Board, Settings |

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
| `TabNav.svelte` | Tab navigation (Timeline/Changes/Transcript) |
| `RetryPanel.svelte` | Retry configuration with context injection |
| `PRActions.svelte` | GitHub PR status, create/merge buttons |

### Diff Components (Unified Changes View)

| Component | Purpose |
|-----------|---------|
| `DiffViewer.svelte` | Main diff container with inline review, split/unified toggle |
| `DiffFile.svelte` | Single file diff with expand/collapse, comment counts |
| `DiffHunk.svelte` | Code hunk with line numbers, inline comment threads |
| `DiffLine.svelte` | Line with syntax highlighting, comment badge, add-comment hover |
| `InlineCommentThread.svelte` | Inline comments below diff lines |
| `DiffStats.svelte` | Additions/deletions summary |
| `VirtualScroller.svelte` | Virtual scrolling for 10K+ lines |

### Review Components (Legacy - Unused)

These components were the original standalone review UI before inline review was integrated into the diff view. Kept for reference but no longer imported anywhere.

| Component | Purpose |
|-----------|---------|
| `ReviewPanel.svelte` | Standalone review panel (legacy) |
| `CommentThread.svelte` | Single comment thread display (legacy) |
| `CommentForm.svelte` | Add/edit comment form (legacy) |
| `ReviewSummary.svelte` | Findings summary with severity counts (legacy) |

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
| `tasks.ts` | Global reactive task state with real-time updates |
| `project.ts` | Current project state |
| `sidebar.ts` | Sidebar collapsed state |
| `toast.svelte.ts` | Toast notification queue |

### Task Store (`tasks.ts`)

The task store provides global reactive state for all tasks, updated in real-time via WebSocket:

```typescript
// Reactive stores
tasks           // All tasks
tasksLoading    // Loading state
tasksError      // Error state

// Derived stores
activeTasks     // Running, blocked, paused
recentTasks     // Recently completed/failed
runningTasks    // Currently running
statusCounts    // Counts by status

// Actions
loadTasks()                          // Fetch all tasks from API
updateTask(taskId, updates)          // Update task in store
updateTaskStatus(taskId, status)     // Update task status
updateTaskState(taskId, state)       // Update from WebSocket state event
addTask(task)                        // Add new task
removeTask(taskId)                   // Remove task
refreshTask(taskId)                  // Fetch single task from API
```

The task store is initialized and kept in sync by the global WebSocket handler in `+layout.svelte`. Pages subscribe to the store for reactive updates without needing their own WebSocket connections.

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

Real-time updates for task events. Supports both task-specific and global subscriptions:

```typescript
// Singleton instance
const ws = getWebSocket()

// Task-specific subscription
ws.connect(taskId)
ws.subscribe(taskId)
ws.on('all', (event) => { ... })
ws.unsubscribe()

// Global subscription (receives ALL task events)
ws.connect('*')  // or ws.subscribeGlobal()
ws.on('all', (event) => { ... })

// Helper for task-specific subscription with cleanup
const cleanup = subscribeToTaskWS(taskId, onEvent, onStatus)
cleanup()  // Unsubscribe

// Helper for global subscription (used by layout)
const cleanup = initGlobalWebSocket(onEvent, onStatus)
cleanup()  // Unsubscribe
```

### Global WebSocket Architecture

The app uses a centralized WebSocket pattern:

1. **Layout (`+layout.svelte`)** initializes global WebSocket with `"*"` subscription on app startup
2. **Backend** publishes all task events to global subscribers AND task-specific subscribers
3. **Layout** receives events and updates the global task store
4. **Pages** subscribe to the task store for reactive updates (no individual WebSocket needed)

This ensures real-time updates work across all pages without page refreshes.

### Event Types

| Event | Data | Purpose |
|-------|------|---------|
| `state` | `TaskState` | Full task state update |
| `phase` | `{phase, status}` | Phase started/completed/failed |
| `transcript` | `TranscriptLine` | Streaming conversation |
| `tokens` | `TokenUpdate` | Token usage |
| `complete` | `{status, duration}` | Task finished |
| `error` | `{message, fatal}` | Error occurred |

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

## Review Workflow (GitHub-Style Unified View)

The "Changes" tab combines diff viewing with inline review in a single unified view:

1. **Viewing Changes**: Split or unified diff view with file-by-file breakdown
2. **Adding Comments**: Click line number → inline comment form appears below line
3. **Comment Severity**: Suggestion (info), Issue (warning), Blocker (danger)
4. **Summary Bar**: Shows open comment counts by severity in toolbar
5. **Send to Agent**: Button appears when open comments exist, triggers retry with context
6. **Resolution**: Mark comments as Resolved or Won't Fix directly in inline thread

Comments are displayed inline below relevant diff lines, with a count badge on the line number.

## Settings Hub

The `/settings` page provides a unified configuration interface with three tabs:

### Orc Config Tab
- Automation profile (auto/fast/safe/strict)
- Model selection
- Max iterations, timeout
- Retry settings, worktree isolation

### Claude Settings Tab
- Model, max tokens, custom prompt
- Environment variables
- Permission settings

### Quick Access Tab
Grid of links to all configuration pages:
- Prompts, CLAUDE.md, Skills, Hooks
- MCP, Tools, Agents, Scripts

All Claude Code configuration (prompts, agents, scripts, MCP, hooks, skills) is accessible through the Quick Access grid, keeping the sidebar minimal while providing full access to all settings.
