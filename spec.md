# Specification: Phase 0 - WebSocket Real-time Updates E2E Tests

## Overview

This specification defines the E2E test suite for WebSocket integration and real-time UI updates. These tests are CRITICAL for migration success as they verify actual WebSocket behavior, not mocked responses.

**Status**: Tests already implemented in `web/e2e/websocket.spec.ts` - this spec documents the implementation and success criteria.

## Requirements

### Functional Requirements

1. **Task Lifecycle Updates** - UI must react to real-time task state changes
2. **Live Transcript** - Modal must stream content and track connection status
3. **Connection Handling** - WebSocket must auto-reconnect and resume updates

### Non-Functional Requirements

1. **Reliability** - Tests must not be flaky (deterministic results)
2. **Framework-Agnostic** - Tests must work with any frontend (Svelte, React)
3. **Real WebSocket** - Tests verify actual WebSocket behavior, not mocked

## Technical Approach

### WebSocket Event Injection Pattern

Tests use Playwright's `routeWebSocket` to intercept real WebSocket connections and inject events:

```typescript
// Set up WebSocket interception with event injection
let wsSendToPage: ((data: string) => void) | null = null;

await page.routeWebSocket(/\/api\/ws/, async (ws) => {
  const server = await ws.connectToServer();
  wsSendToPage = (data: string) => ws.send(data);  // Capture send function

  // Bidirectional forwarding
  ws.onMessage((message) => server.send(message));
  server.onMessage((message) => ws.send(message));
});

// After setup, inject events to test UI response
if (wsSendToPage) {
  const event = createWSEvent('state', taskId, { status: 'running' });
  wsSendToPage(JSON.stringify(event));
}
```

**Why this approach:**
- Uses real WebSocket connections (not mocked)
- Tests actual UI updates in response to events
- Framework-agnostic - works with Svelte, React, or any frontend
- No production code modifications required

### Event Message Structure

```typescript
interface WSEvent {
  type: 'event';
  event: WSEventType;  // state, transcript, phase, tokens, etc.
  task_id: string;
  data: unknown;
  time: string;        // ISO 8601 timestamp
}
```

### Event Types Tested

| Event | Description | UI Response |
|-------|-------------|-------------|
| `state` | Task status/phase changes | Card updates, column moves |
| `transcript` | Streaming content chunks | Modal content updates |
| `tokens` | Token usage updates | Token count display |
| `phase` | Phase transitions | Column placement, progress |
| `task_created` | New task via file watcher | Card appears, toast |
| `task_updated` | Task modification | Card updates in place |
| `task_deleted` | Task removal | Card removed, toast |

### Selector Strategy

Tests use framework-agnostic selectors (priority order):

| Priority | Method | Example |
|----------|--------|---------|
| 1 | `getByRole()` | `getByRole('region', { name: 'Queued column' })` |
| 2 | Semantic text | `getByText('Task Board')` |
| 3 | Structural classes | `.task-card`, `.connection-status` |
| 4 | ARIA attributes | `locator('[aria-label="..."]')` |
| 5 | `data-testid` | Fallback only |

## Test Coverage (12 Core Tests)

### Task Lifecycle Updates (5 tests)

| Test | Event | Verification |
|------|-------|--------------|
| `should update task card when task status changes via WebSocket` | `state` | Card reflects new status |
| `should move card to new column when phase changes` | `phase` + `state` | Card moves to correct column |
| `should show toast notification on task creation event` | `task_created` | Toast appears with message |
| `should remove card when task deleted event received` | `task_deleted` | Card removed, deletion toast |
| `should update progress indicators during task running` | `state` + `tokens` | Running state + token updates |

### Live Transcript Modal (4 tests)

| Test | Event | Verification |
|------|-------|--------------|
| `should open live transcript modal when clicking running task` | `state` (simulate running) | Modal opens with content |
| `should show streaming content in real-time` | `transcript` chunks | Content appears progressively |
| `should display connection status (Live/Connecting/Disconnected)` | WebSocket state | Status indicator visible |
| `should update token counts during execution` | `tokens` | Token values update |

### Connection Handling (3 tests)

| Test | Scenario | Verification |
|------|----------|--------------|
| `should reconnect automatically after disconnect` | `ws.close()` | Connection count increases |
| `should show reconnecting banner/status` | Simulated disconnect | Status changes to reconnecting |
| `should resume updates after reconnection` | Reconnect + inject event | Event processed after reconnect |

## Component Breakdown

### Backend

No backend changes required - tests use existing WebSocket infrastructure:
- `/api/ws` endpoint (existing)
- Event broadcasting (existing)
- File watcher events (existing)

### Frontend

Tests verify existing frontend behavior:
- `web/src/lib/websocket.ts` - WebSocket client
- `web/src/lib/stores/tasks.ts` - Task store updates
- Task card components - Visual updates
- Toast notifications - Event announcements
- LiveTranscriptModal - Streaming display
- Connection status indicators

### Test Infrastructure

| File | Purpose |
|------|---------|
| `web/e2e/websocket.spec.ts` | Main test file (12+ tests) |
| `web/playwright.config.ts` | Test configuration |

## Helper Functions

```typescript
// Create properly structured WebSocket event
function createWSEvent(event: string, taskId: string, data: unknown): WSEvent {
  return {
    type: 'event',
    event,
    task_id: taskId,
    data,
    time: new Date().toISOString()
  };
}

// Wait for board to fully load
async function waitForBoardLoad(page: Page) {
  await page.waitForSelector('.board-page', { timeout: 10000 });
  await page.waitForSelector('.loading-state', { state: 'hidden' }).catch(() => {});
  await page.waitForLoadState('networkidle');
  await page.waitForSelector('.task-card, [role="region"]', { timeout: 5000 }).catch(() => {});
  await page.waitForTimeout(200);
}

// Find task card, optionally preferring running tasks
async function findTask(page: Page, preferRunning = false): Promise<TaskInfo | null>
```

## Timing Constants

Tests use consistent timing to prevent flakiness:

```typescript
const WS_EVENT_PROPAGATION_MS = 500;   // WebSocket → store → DOM update
const WS_PROGRESS_PROPAGATION_MS = 300; // Progress/token update delays
const UI_SETTLE_MS = 200;               // Animation/transition settle time
```

## Success Criteria

### Code Requirements

- [x] `web/e2e/websocket.spec.ts` contains all 12 core tests
- [x] WebSocket event injection pattern implemented
- [x] Helper functions for board loading and task finding
- [x] Timing constants for consistent delays
- [x] Framework-agnostic selectors

### Tests Must Pass

- [ ] All 12 core tests pass reliably (no flaky failures)
- [ ] Tests work with actual WebSocket (not mocked)
- [ ] Tests handle "no tasks available" gracefully via `test.skip()`

### E2E Scenarios Must Work

1. **Task status change**: Inject `state` event → card updates
2. **Phase change**: Inject `phase` + `state` → card moves column
3. **Task creation**: Inject `task_created` → card appears, toast shows
4. **Task deletion**: Inject `task_deleted` → card removed, toast shows
5. **Progress update**: Inject `tokens` → token display updates
6. **Running task modal**: Click running task → modal opens
7. **Streaming content**: Inject `transcript` chunks → content streams
8. **Connection status**: Check status indicator shows Live/Connecting
9. **Token tracking**: Inject `tokens` events → counts update
10. **Auto-reconnect**: Close WebSocket → reconnection attempted
11. **Reconnecting status**: Disconnect → status shows reconnecting
12. **Resume after reconnect**: Reconnect + event → event processed

### Documentation

- [x] Test file contains comprehensive JSDoc header
- [x] `web/CLAUDE.md` documents WebSocket E2E testing patterns
- [x] Event types and selector strategy documented
- [ ] Knowledge pattern added to CLAUDE.md after validation

## Testing Strategy

### Unit Tests

Not applicable - this task is E2E test expansion.

### Integration Tests

The WebSocket client has unit tests in `web/src/lib/websocket.test.ts` covering:
- Connection lifecycle
- Event handling
- Reconnection logic

### E2E Tests

The 12 core tests plus 5 legacy tests provide comprehensive coverage:

```
web/e2e/websocket.spec.ts
├── WebSocket Real-time Updates
│   ├── Task Lifecycle Updates (5 tests)
│   │   ├── should update task card when task status changes
│   │   ├── should move card to new column when phase changes
│   │   ├── should show toast notification on task creation
│   │   ├── should remove card when task deleted
│   │   └── should update progress indicators during running
│   ├── Live Transcript Modal (4 tests)
│   │   ├── should open modal when clicking running task
│   │   ├── should show streaming content in real-time
│   │   ├── should display connection status
│   │   └── should update token counts during execution
│   └── Connection Handling (3 tests)
│       ├── should reconnect automatically after disconnect
│       ├── should show reconnecting banner/status
│       └── should resume updates after reconnection
└── Legacy Tests (5 tests)
    ├── should show connection status on task detail page
    ├── should handle page reload gracefully
    ├── should display transcript section
    ├── should display timeline for tasks with plans
    └── should show token usage when available
```

### Running Tests

```bash
# Run WebSocket E2E tests
cd web && bunx playwright test e2e/websocket.spec.ts

# Run with UI for debugging
cd web && bunx playwright test e2e/websocket.spec.ts --ui

# Run all E2E tests
make e2e
```

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Flaky tests due to timing | Use timing constants, retry mechanisms |
| No tasks available | `test.skip()` when no test data exists |
| WebSocket route not capturing | Reload page after `routeWebSocket` setup |
| Event propagation delays | Consistent wait times after event injection |

## Implementation Notes

### Already Implemented

The test file `web/e2e/websocket.spec.ts` already contains:
- All 12 core tests as specified
- WebSocket event injection pattern
- Helper functions
- Proper timing constants
- Framework-agnostic selectors
- Comprehensive JSDoc documentation

### Verification Needed

1. Run tests to verify they pass reliably
2. Check for any flaky tests and fix
3. Validate edge cases (no tasks, disconnection)
4. Add knowledge pattern to CLAUDE.md

## Knowledge Pattern (To Add)

After tests pass, add to CLAUDE.md:

```markdown
| WebSocket E2E event injection | Use Playwright's `routeWebSocket` to intercept connections and inject events via `ws.send()`; captures real WebSocket, forwards messages bidirectionally, allows test-initiated events; framework-agnostic approach for testing real-time UI updates | TASK-157 |
```

<phase_complete>true</phase_complete>
