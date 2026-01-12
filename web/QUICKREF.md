# Web Frontend - Quick Reference

Deep-dive patterns and code examples. For overview, see `CLAUDE.md`.

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

**When to use:** Files with 1000+ lines, transcript viewers, large task lists.

---

## Kanban Board Patterns

### Phase-Based Columns

```typescript
const columns = [
  { id: 'queued', title: 'Queued', phases: [] },
  { id: 'spec', title: 'Spec', phases: ['research', 'spec', 'design'] },
  { id: 'implement', title: 'Implement', phases: ['implement'] },
  { id: 'test', title: 'Test', phases: ['test'] },
  { id: 'review', title: 'Review', phases: ['docs', 'validate', 'review'] },
  { id: 'done', title: 'Done', phases: [] }
];
```

**Task placement rules:**
- No phase (created/classifying/planned) → Queued
- Running/paused/blocked → Column matching current phase
- Completed/failed → Done (regardless of phase)

### Drag-Drop Confirmation

| Drop Action | Confirmation |
|-------------|--------------|
| Queued → any phase | "Run task?" |
| Paused/blocked → phase | "Resume task?" |
| Running → backward | Escalation modal |

Button actions on TaskCard bypass drag-drop confirmation.

---

## WebSocket Architecture

### Global Pattern

```
+layout.svelte          → initGlobalWebSocket("*")
       ↓ events
Task Store              → updateTaskState(taskId, state)
       ↓ reactive
Pages                   → subscribe to store (no individual WS)
```

**Benefits:** Single connection, automatic sync, no page refresh needed.

### Subscription Helpers

```typescript
// Global subscription (used by layout)
const cleanup = initGlobalWebSocket(onEvent, onStatus)

// Task-specific subscription with cleanup
const cleanup = subscribeToTaskWS(taskId, onEvent, onStatus)

// Manual control
const ws = getWebSocket()
ws.connect('*')  // or ws.connect(taskId)
ws.on('all', handler)
ws.unsubscribe()
```

---

## Review Workflow

The "Changes" tab combines diff + inline review:

```
DiffViewer
├── DiffStats (toolbar) ─── Severity counts, "Send to Agent" button
├── DiffFile (per file)
│   └── DiffHunk (per hunk)
│       ├── DiffLine ─────── Click line number → comment form
│       └── InlineCommentThread (below line with comments)
```

### Comment Lifecycle

1. Click line → Form appears below line
2. Set severity (Suggestion/Issue/Blocker)
3. Submit → Comment stored in `review_comments` table
4. Click "Send to Agent" → Triggers retry with `{{RETRY_CONTEXT}}`
5. Mark Resolved/Won't Fix

---

## Task Store

Global reactive state with real-time updates:

```typescript
// Stores
tasks, tasksLoading, tasksError

// Derived
activeTasks     // Running, blocked, paused
recentTasks     // Recently completed/failed
runningTasks    // Currently running
statusCounts    // Counts by status

// Actions
loadTasks()
updateTask(taskId, updates)
updateTaskStatus(taskId, status)
updateTaskState(taskId, state)  // From WebSocket
addTask(task), removeTask(taskId)
refreshTask(taskId)             // Re-fetch from API
```

Store initialized in `+layout.svelte`, kept in sync via global WebSocket.

---

## API Client Functions

### Tasks
```typescript
listTasks(projectId?), getTask(taskId, projectId?)
createTask(data, projectId?), deleteTask(taskId, projectId?)
runTask(taskId, projectId?), pauseTask(taskId, projectId?)
resumeTask(taskId, projectId?)
```

### Diff & Review
```typescript
getTaskDiff(taskId, { base?, filesOnly? })
getTaskDiffFile(taskId, filePath)
getTaskDiffStats(taskId)
getReviewComments(taskId)
createReviewComment(taskId, { file_path, line_number, content, severity })
updateReviewComment(taskId, commentId, { status, content? })
deleteReviewComment(taskId, commentId)
retryWithReview(taskId, { include_comments: true })
```

### GitHub
```typescript
createPR(taskId), getPRDetails(taskId)
syncPRComments(taskId), autofixPRComment(taskId, commentId)
```

---

## Settings Hub Tabs

| Tab | Contents |
|-----|----------|
| Quick Access | Getting Started guides, Config pages grid, Keyboard tip |
| Orc Config | Search filter, inline help, automation profile, model, retry |
| Claude Settings | Environment variables, status line, effective settings JSON |

**Default tab:** Quick Access (for discoverability)

---

## Utility Functions

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
isMac()                     // Detect macOS for Cmd vs Ctrl
```

---

## Component Gotchas

### Svelte 5 Runes

```svelte
<script>
  let { data } = $props()        // NOT export let data
  let count = $state(0)          // NOT let count = 0
  let doubled = $derived(count * 2)  // NOT $: doubled = count * 2
</script>
```

### Toast Notifications

```typescript
import { addToast } from '$lib/stores/toast.svelte'

addToast({
  type: 'success' | 'error' | 'warning' | 'info',
  message: 'Task completed',
  duration: 5000  // ms, optional
})
```

### Icon Component

```svelte
<Icon name="check" size={16} />
```

Available: `check`, `x`, `warning`, `info`, `refresh`, `trash`, `play`, `pause`, `clock`, `code`, `settings`, `database`, `folder`, `file`, `search`, `filter`, `plus`, `minus`, `arrow-up`, `arrow-down`, `arrow-left`, `arrow-right`, `chevron-up`, `chevron-down`, `chevron-left`, `chevron-right`, `external-link`, `copy`, `edit`, `eye`, `eye-off`, `github`, `terminal`
