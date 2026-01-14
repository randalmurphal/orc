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
- No phase + not running (created/classifying/planned) → Queued
- No phase + running → Implement (transitional state during startup)
- Running/paused/blocked with phase → Column matching current phase
- Completed/failed → Done (regardless of phase)

**Running task indicator:** Tasks with `status=running` display a pulsing border animation with accent-colored gradient background, making them visually distinct from pending tasks in the same column.

### Task Organization (Queue & Priority)

Tasks within each column are organized by:
1. **Queue**: Active tasks shown first, backlog tasks in collapsible section
2. **Priority**: Within each queue section, sorted by priority (critical → high → normal → low)

```typescript
// Sort order implementation
const PRIORITY_ORDER = { critical: 0, high: 1, normal: 2, low: 3 };

// Filter and sort tasks for a column
const activeTasks = tasks.filter(t => t.queue !== 'backlog')
  .sort((a, b) => PRIORITY_ORDER[a.priority || 'normal'] - PRIORITY_ORDER[b.priority || 'normal']);
const backlogTasks = tasks.filter(t => t.queue === 'backlog')
  .sort((a, b) => PRIORITY_ORDER[a.priority || 'normal'] - PRIORITY_ORDER[b.priority || 'normal']);
```

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

## Live Transcript Modal

Modal for viewing running task output in real-time:

```typescript
// Open modal from task card
<LiveTranscriptModal
  open={showTranscript}
  task={selectedTask}
  onClose={() => showTranscript = false}
/>
```

### WebSocket Event Handling

```typescript
// In LiveTranscriptModal.svelte
ws.on('all', (event) => {
  if (event.task_id !== task.id) return;

  switch (event.event) {
    case 'transcript':
      if (data.type === 'chunk') {
        // Append to streaming buffer
        streamingContent += data.content;
      } else if (data.type === 'response') {
        // Reload transcript files
        loadData();
      }
      break;
    case 'tokens':
      // Update token display (incremental)
      taskState.tokens.input_tokens += data.input_tokens;
      break;
    case 'state':
      taskState = data;
      break;
  }
});
```

**Key points:**
- Reset streaming buffer when phase/iteration changes
- Tokens are incremental, add to totals
- Reload transcript files on `response` or `complete` events

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

### Attachments
```typescript
listAttachments(taskId)
uploadAttachment(taskId, file, filename?)
getAttachmentUrl(taskId, filename)
deleteAttachment(taskId, filename)
```

### Task Comments (Discussion)
```typescript
getTaskComments(taskId)                 // List all comments
getTaskCommentsFiltered(taskId, { author_type?, phase? })
createTaskComment(taskId, { author, author_type, content, phase? })
updateTaskComment(taskId, commentId, { content, phase? })
deleteTaskComment(taskId, commentId)
getTaskCommentStats(taskId)             // { total, human_count, agent_count, system_count }
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
