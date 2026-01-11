# Web UI Dashboard

**Status**: Planning
**Priority**: P1
**Last Updated**: 2026-01-10

---

## Problem Statement

Current web UI landing page is a task list. Users need:
- Quick overview of project status
- Recent activity feed
- Key metrics at a glance
- Fast access to common actions

---

## Solution: Dashboard Home Page

Replace task list as the default landing page with a comprehensive dashboard that surfaces the most important information.

---

## Design Principles

1. **Glanceable** - Status visible in <1 second
2. **Actionable** - Common actions accessible immediately
3. **Contextual** - Shows what matters right now
4. **Consistent** - Follows existing design system

---

## Dashboard Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ orc                                              [Project: my-app ▼]  [⌘K]  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─ Quick Stats ──────────────────────────────────────────────────────────┐│
│  │                                                                        ││
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐         ││
│  │  │  Running   │ │  Blocked   │ │   Today    │ │   Tokens   │         ││
│  │  │     2      │ │     1      │ │     5      │ │   192K     │         ││
│  │  │            │ │            │ │  tasks     │ │   $4.36    │         ││
│  │  └────────────┘ └────────────┘ └────────────┘ └────────────┘         ││
│  │                                                                        ││
│  └────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  ┌─ Active Tasks ─────────────────────────────────────────────────────────┐│
│  │                                                                [+ New] ││
│  │  ⏳ TASK-007 Implement caching layer            [large] implement 3/5  ││
│  │     ○ spec ─── ● implement ─── ○ test ─── ○ validate                   ││
│  │     Started 15m ago • 45.2K tokens                                     ││
│  │                                            [Pause] [View]              ││
│  │                                                                        ││
│  │  ⏳ TASK-008 Fix login redirect bug             [small] test 1/2       ││
│  │     ● implement ─── ○ test                                             ││
│  │     Started 3m ago • 12.1K tokens                                      ││
│  │                                            [Pause] [View]              ││
│  │                                                                        ││
│  │  🚫 TASK-006 Add dark mode toggle               [medium] blocked       ││
│  │     ○ spec ─── ● implement ─── ○ test                                  ││
│  │     Blocked: unclear requirements                                      ││
│  │                                         [Resume] [View] [Transcript]   ││
│  │                                                                        ││
│  └────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  ┌─ Recent Activity ────────────────────────────────────────── [View All] ┐│
│  │                                                                        ││
│  │  ✅ TASK-005 Update API documentation           completed 2m ago       ││
│  │  ✅ TASK-004 Refactor auth middleware           completed 15m ago      ││
│  │  ❌ TASK-003 Add rate limiting                  failed 1h ago          ││
│  │  ✅ TASK-002 Fix memory leak                    completed 2h ago       ││
│  │                                                                        ││
│  └────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
│  ┌─ Quick Actions ────────────────────────────────────────────────────────┐│
│  │                                                                        ││
│  │  [+ New Task]  [Resume Last]  [View All Tasks]  [Open Settings]        ││
│  │                                                                        ││
│  └────────────────────────────────────────────────────────────────────────┘│
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Components

### Quick Stats Widget

Four key metrics displayed as cards:

```svelte
<div class="stats-grid">
  <StatCard
    label="Running"
    value={runningCount}
    icon="spinner"
    color="blue"
    href="/tasks?status=running"
  />
  <StatCard
    label="Blocked"
    value={blockedCount}
    icon="alert"
    color="orange"
    href="/tasks?status=blocked"
  />
  <StatCard
    label="Today"
    value={todayCount}
    sublabel="tasks"
    icon="calendar"
    href="/tasks?period=today"
  />
  <StatCard
    label="Tokens"
    value={formatTokens(todayTokens)}
    sublabel={formatCost(todayCost)}
    icon="coins"
    href="/cost"
  />
</div>
```

### Active Tasks Section

Shows running and blocked tasks with inline progress:

```svelte
<section class="active-tasks">
  <header>
    <h2>Active Tasks</h2>
    <button class="primary" on:click={openNewTask}>+ New</button>
  </header>

  {#if activeTasks.length === 0}
    <EmptyState
      icon="check"
      title="All clear!"
      description="No tasks currently running or blocked"
      action={{ label: "Create Task", onClick: openNewTask }}
    />
  {:else}
    {#each activeTasks as task (task.id)}
      <ActiveTaskCard {task} />
    {/each}
  {/if}
</section>
```

### Active Task Card (Expanded)

More detailed than list view, includes inline timeline:

```
┌─────────────────────────────────────────────────────────────────┐
│ ⏳ TASK-007 Implement caching layer                     [large] │
│                                                                 │
│ ○───●───○───○                                                   │
│ spec  impl  test  validate                                      │
│       ↑                                                         │
│    iteration 3/5                                                │
│                                                                 │
│ Started 15 minutes ago                                          │
│ Tokens: 45.2K ($1.05)                                           │
│                                                                 │
│                               [Pause]  [Cancel]  [View Details] │
└─────────────────────────────────────────────────────────────────┘
```

### Recent Activity Feed

Simple list of completed/failed tasks:

```svelte
<section class="recent-activity">
  <header>
    <h2>Recent Activity</h2>
    <a href="/tasks?sort=recent">View All</a>
  </header>

  <ul class="activity-list">
    {#each recentTasks.slice(0, 5) as task (task.id)}
      <li class="activity-item">
        <StatusIcon status={task.status} />
        <a href="/tasks/{task.id}" class="task-link">{task.id} {task.title}</a>
        <time>{formatRelative(task.completed_at)}</time>
      </li>
    {/each}
  </ul>
</section>
```

### Quick Actions Bar

Common actions always accessible:

```svelte
<section class="quick-actions">
  <button class="action-btn primary" on:click={openNewTask}>
    <PlusIcon /> New Task
  </button>
  <button class="action-btn" on:click={resumeLast} disabled={!lastPaused}>
    <PlayIcon /> Resume Last
  </button>
  <a href="/tasks" class="action-btn">
    <ListIcon /> View All Tasks
  </a>
  <a href="/settings" class="action-btn">
    <SettingsIcon /> Settings
  </a>
</section>
```

---

## Data Requirements

### API Calls on Load

```typescript
// Load dashboard data in parallel
const [tasks, stats, recentActivity] = await Promise.all([
  listTasks({ status: ['running', 'blocked', 'paused'] }),
  getDashboardStats(),
  getRecentActivity({ limit: 5 })
]);
```

### Dashboard Stats Endpoint

```
GET /api/dashboard/stats

Response:
{
  "running": 2,
  "blocked": 1,
  "paused": 0,
  "today": {
    "completed": 3,
    "failed": 1,
    "created": 5
  },
  "tokens": {
    "today": 192000,
    "cost": 4.36
  },
  "last_activity": "2026-01-10T15:45:00Z"
}
```

---

## Real-time Updates

Dashboard subscribes to WebSocket for live updates:

```typescript
onMount(() => {
  const ws = getWebSocket();

  ws.on('task:started', (task) => {
    activeTasks = [...activeTasks, task];
    stats.running++;
  });

  ws.on('task:completed', (task) => {
    activeTasks = activeTasks.filter(t => t.id !== task.id);
    stats.running--;
    recentActivity = [task, ...recentActivity.slice(0, 4)];
  });

  ws.on('task:failed', (task) => {
    activeTasks = activeTasks.filter(t => t.id !== task.id);
    stats.running--;
    recentActivity = [task, ...recentActivity.slice(0, 4)];
  });

  ws.on('task:blocked', (task) => {
    const idx = activeTasks.findIndex(t => t.id === task.id);
    if (idx >= 0) {
      activeTasks[idx] = task;
      stats.blocked++;
    }
  });

  return () => ws.disconnect();
});
```

---

## Notifications

### Web UI Notifications

Show toast notifications for important events:

```typescript
// Task completed
showNotification({
  type: 'success',
  title: 'Task Completed',
  message: 'TASK-005 Update API documentation',
  action: { label: 'View', href: '/tasks/TASK-005' }
});

// Task blocked
showNotification({
  type: 'warning',
  title: 'Task Blocked',
  message: 'TASK-006 needs attention: unclear requirements',
  action: { label: 'View', href: '/tasks/TASK-006' }
});

// Task failed
showNotification({
  type: 'error',
  title: 'Task Failed',
  message: 'TASK-003 failed after 3 retries',
  action: { label: 'View Transcript', href: '/tasks/TASK-003' }
});
```

### Notification Center

Accessible from header, shows recent notifications:

```
┌─ Notifications ────────────────────────────────────────────────┐
│                                                   [Clear All]  │
│                                                                │
│  ✅ Task Completed                               2 min ago     │
│     TASK-005 Update API documentation                          │
│                                                                │
│  ⚠️ Task Blocked                                  5 min ago     │
│     TASK-006 Add dark mode toggle                              │
│     Reason: unclear requirements                               │
│                                                                │
│  ❌ Task Failed                                   1 hour ago    │
│     TASK-003 Add rate limiting                                 │
│     Failed after 3 retries                                     │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

---

## Navigation Update

Dashboard becomes the default home:

```
Sidebar:
  [Dashboard]  ← New default home
  [Tasks]
  [Templates]
  ───────────
  [Prompts]
  [Hooks]
  [Skills]
  [Settings]
```

---

## Responsive Design

### Mobile Layout

Stack cards vertically, simplify active tasks:

```
┌───────────────────────────────────────┐
│ orc              [my-app ▼]  [menu]   │
├───────────────────────────────────────┤
│                                       │
│  ┌─────────┐ ┌─────────┐             │
│  │ Running │ │ Blocked │             │
│  │    2    │ │    1    │             │
│  └─────────┘ └─────────┘             │
│                                       │
│  ┌─────────┐ ┌─────────┐             │
│  │  Today  │ │ Tokens  │             │
│  │    5    │ │  192K   │             │
│  └─────────┘ └─────────┘             │
│                                       │
│  Active Tasks                [+ New]  │
│  ─────────────────────────────────── │
│  ⏳ TASK-007 Implement caching        │
│     implement 3/5 • 45.2K tokens      │
│                      [Pause] [View]   │
│                                       │
│  ⏳ TASK-008 Fix login bug            │
│     test 1/2 • 12.1K tokens           │
│                      [Pause] [View]   │
│                                       │
└───────────────────────────────────────┘
```

---

## Implementation Checklist

- [ ] Create Dashboard page component
- [ ] Add `/api/dashboard/stats` endpoint
- [ ] Quick Stats widget with clickable cards
- [ ] Active Tasks section with expanded cards
- [ ] Recent Activity feed with relative times
- [ ] Quick Actions bar
- [ ] WebSocket integration for real-time updates
- [ ] Toast notification system
- [ ] Notification center component
- [ ] Update navigation to make Dashboard home
- [ ] Responsive mobile layout
- [ ] Empty states for each section

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for dashboard components
- 100% coverage for stats aggregation logic

### Unit Tests

| Test | Description |
|------|-------------|
| `TestFormatRelativeTime` | "2m ago", "1h ago" formatting |
| `TestDashboardStatsAggregation` | Running, blocked, today counts |
| `TestNotificationQueue` | FIFO, max 5 notifications |
| `TestNotificationTimeout` | Auto-dismiss after N seconds |
| `TestStatCardClickHandler` | Navigation on click |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestAPIDashboardStats` | `GET /api/dashboard/stats` returns data |
| `TestDashboardStatsAccuracy` | Counts match actual task states |
| `TestWebSocketBroadcast` | Events reach connected clients |
| `TestWebSocketReconnection` | Reconnects after disconnect |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_dashboard_loads_fast` | `browser_navigate`, timing | Dashboard loads within 500ms |
| `test_quick_stats_cards` | `browser_snapshot` | Stats cards display correct counts |
| `test_stat_card_navigation` | `browser_click`, `browser_snapshot` | Clicking navigates to filtered list |
| `test_active_tasks_section` | `browser_snapshot` | Running and blocked tasks visible |
| `test_active_task_progress` | `browser_snapshot` | Phase timeline on task card |
| `test_recent_activity_feed` | `browser_snapshot` | Last 5 completed/failed tasks |
| `test_quick_actions_buttons` | `browser_click` | Action buttons work |
| `test_realtime_task_complete` | `browser_wait_for` | Stats update on task completion |
| `test_realtime_task_blocked` | `browser_wait_for` | Blocked count updates |
| `test_toast_notification` | `browser_snapshot` | Toast appears on task completion |
| `test_notification_center` | `browser_click`, `browser_snapshot` | Notification center opens |
| `test_empty_state` | `browser_snapshot` | Empty state when no active tasks |
| `test_mobile_layout` | `browser_resize`, `browser_snapshot` | Cards stack on mobile |
| `test_resume_last_button` | `browser_snapshot` | Disabled when no paused tasks |

### Performance Tests

| Test | Description |
|------|-------------|
| `test_dashboard_load_time` | Measure and assert <500ms |
| `test_websocket_latency` | Event appears within 100ms |
| `test_large_task_list` | Handles 100+ tasks |

### Test Fixtures
- Sample task data for stats testing
- Mock WebSocket events
- Various task state combinations

---

## Success Criteria

- [ ] Dashboard loads in <500ms
- [ ] Stats update in real-time via WebSocket
- [ ] Active tasks show current phase and iteration
- [ ] Recent activity shows last 5 tasks
- [ ] Notifications appear for task state changes
- [ ] Mobile layout is usable
- [ ] Following existing design patterns/tokens
- [ ] 80%+ test coverage on dashboard code
- [ ] All E2E tests pass
