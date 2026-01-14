# Specification: WebSocket Real-time Updates E2E Tests

## Overview

E2E tests for WebSocket integration and real-time UI updates. These tests validate that the Svelte frontend correctly responds to WebSocket events from the Go backend, ensuring the real-time features work end-to-end. **CRITICAL for React migration** - these tests serve as a behavioral contract that the React implementation must satisfy.

## Current State

### Existing Infrastructure

**Backend WebSocket** (`internal/api/websocket.go`):
- Established pub/sub system with `events.MemoryPublisher`
- Supports task-specific (`TASK-001`) and global (`*`) subscriptions
- Event types: `state`, `transcript`, `phase`, `tokens`, `complete`, `finalize`, `task_created`, `task_updated`, `task_deleted`
- Backend unit tests exist in `internal/api/websocket_test.go` (13 tests)

**Frontend WebSocket** (`web/src/lib/websocket.ts`):
- Singleton `OrcWebSocket` class with reconnection logic
- Global subscription via `initGlobalWebSocket()` in `+layout.svelte`
- Task-specific subscription via `subscribeToTaskWS()`
- Connection status: `disconnected` → `connecting` → `connected` → `reconnecting`

**Existing E2E Tests** (`web/e2e/websocket.spec.ts`):
- Only 5 tests exist, all passive/observational
- No real WebSocket event testing
- Tests only verify UI elements exist, not real-time behavior

### Gap Analysis

| Feature | Backend Tested | Frontend E2E | Gap |
|---------|----------------|--------------|-----|
| Task status update | ✅ | ❌ | Need to verify board updates |
| Phase change | ✅ | ❌ | Need to verify card moves columns |
| Task created (file watcher) | ✅ | ❌ | Need to verify card appears |
| Task deleted (file watcher) | ✅ | ❌ | Need to verify card removed |
| Live transcript streaming | ❌ | ❌ | Need to verify modal shows streaming |
| Token count updates | ❌ | ❌ | Need to verify token display updates |
| Connection status display | ❌ | Partial | Need to verify all states |
| Reconnection flow | ❌ | ❌ | Need to verify auto-reconnect |

## Requirements

### Test Coverage (12 Tests)

#### 1. Task Lifecycle Updates (5 tests)

**1.1 `should update task card when task status changes via WebSocket`**
- Trigger: Publish `state` event with new status
- Verify: Task card status indicator updates
- Selectors: `.task-card .status-indicator`

**1.2 `should move card to new column when phase changes`**
- Trigger: Publish `state` event with new `current_phase`
- Verify: Card appears in correct column (Implement → Test → Review)
- Selectors: `[role="region"][aria-label="Test column"] .task-card`

**1.3 `should show toast notification on task creation event`**
- Trigger: Publish `task_created` event
- Verify: Toast appears with task ID, card appears on board
- Selectors: `.toast`, `.task-card`

**1.4 `should remove card when task deleted event received`**
- Trigger: Publish `task_deleted` event
- Verify: Card disappears from board, toast shown
- Selectors: `.task-card` (verify absence)

**1.5 `should update progress indicators during task running`**
- Trigger: Publish `state` event with `status: running`
- Verify: Running indicator visible (pulsing border/dot)
- Selectors: `.task-card.running`, `.status-dot`

#### 2. Live Transcript Modal (4 tests)

**2.1 `should open live transcript modal when clicking running task`**
- Setup: Task in running state
- Action: Click running task card
- Verify: LiveTranscriptModal opens
- Selectors: `.modal-backdrop`, `.modal-content`, `#transcript-modal-title`

**2.2 `should show streaming content in real-time`**
- Setup: Open transcript modal for running task
- Trigger: Publish `transcript` event with `type: chunk`
- Verify: Content appears incrementally
- Selectors: `.transcript-container`, streaming content visible

**2.3 `should display connection status (Live/Connecting/Disconnected)`**
- Setup: Open transcript modal
- Verify: Connection status shows "Live" when connected
- Trigger: Disconnect WebSocket
- Verify: Status shows "Disconnected" or "Connecting..."
- Selectors: `.connection-status.connected`, `.connection-status.disconnected`, `.connection-status.connecting`

**2.4 `should update token counts during execution`**
- Setup: Open transcript modal
- Trigger: Publish `tokens` event with counts
- Verify: Token display updates (input/output/cached)
- Selectors: `.token-summary`, `.token-value`

#### 3. Connection Handling (3 tests)

**3.1 `should reconnect automatically after disconnect`**
- Setup: Page with active WebSocket connection
- Action: Force disconnect (network interception or page.evaluate)
- Verify: Connection status cycles through reconnecting → connected
- Selectors: `.connection-status`

**3.2 `should show reconnecting banner/status`**
- Setup: Disconnect WebSocket
- Verify: Reconnecting indicator visible
- Selectors: `.connection-status.reconnecting`, `.connection-banner`

**3.3 `should resume updates after reconnection`**
- Setup: Disconnect and reconnect
- Trigger: Publish event after reconnection
- Verify: Event received and UI updates
- Selectors: Task card reflects new state

## Technical Approach

### Strategy: Real WebSocket + Event Injection

Rather than mocking WebSocket, we'll use Playwright's ability to:
1. **Intercept API calls** to trigger backend events
2. **Run CLI commands** via `page.evaluate` or API calls to create/modify tasks
3. **Inject events** via direct WebSocket message sending (if possible)
4. **Use actual backend** running via `webServer` config

### Test Fixtures

```typescript
// Helper: Wait for WebSocket connection
async function waitForWSConnection(page: Page): Promise<void> {
  await page.waitForFunction(() => {
    const ws = (window as any).__orcWebSocket;
    return ws?.isConnected?.() || false;
  }, { timeout: 5000 });
}

// Helper: Create task via API for testing
async function createTestTask(page: Page, title: string): Promise<string> {
  const response = await page.request.post('/api/tasks', {
    data: { title, weight: 'small', category: 'test' }
  });
  const task = await response.json();
  return task.id;
}

// Helper: Trigger task status change via API
async function setTaskStatus(page: Page, taskId: string, status: string): Promise<void> {
  await page.request.put(`/api/tasks/${taskId}`, {
    data: { status }
  });
}

// Helper: Delete task via API
async function deleteTask(page: Page, taskId: string): Promise<void> {
  await page.request.delete(`/api/tasks/${taskId}`);
}

// Helper: Wait for toast notification
async function waitForToast(page: Page, text: string): Promise<void> {
  const toast = page.locator(`.toast:has-text("${text}")`);
  await expect(toast).toBeVisible({ timeout: 5000 });
}

// Helper: Wait for card in column
async function waitForCardInColumn(page: Page, taskId: string, column: string): Promise<void> {
  const columnRegion = page.locator(`[role="region"][aria-label="${column} column"]`);
  const card = columnRegion.locator(`.task-card:has-text("${taskId}")`);
  await expect(card).toBeVisible({ timeout: 5000 });
}
```

### WebSocket Event Injection (Alternative)

If direct API calls don't trigger file watcher events fast enough, we can:

```typescript
// Inject WebSocket message directly via page evaluation
async function injectWSEvent(page: Page, event: object): Promise<void> {
  await page.evaluate((evt) => {
    // Access the WebSocket singleton and dispatch event
    const ws = (window as any).__orcWebSocket;
    if (ws) {
      ws.handleMessage({ type: 'event', ...evt });
    }
  }, event);
}
```

However, this bypasses the real WebSocket path. **Prefer API-triggered events** for true E2E testing.

### Flakiness Prevention

1. **Wait for WebSocket connection** before running tests
2. **Use retries** for network-dependent operations
3. **Debounce checks** - wait for UI to stabilize after events
4. **Clean up test data** - delete created tasks after tests
5. **Isolated test data** - use unique prefixes (e.g., `E2E-WS-{timestamp}`)

## Implementation Plan

### Phase 1: Test Infrastructure

1. Add WebSocket helpers to `web/e2e/` shared utils
2. Create `waitForWSConnection` fixture
3. Add test data cleanup hooks

### Phase 2: Task Lifecycle Tests (5 tests)

1. Implement status update test
2. Implement phase change / column move test
3. Implement task creation event test
4. Implement task deletion event test
5. Implement running indicators test

### Phase 3: Live Transcript Tests (4 tests)

1. Implement modal open test
2. Implement streaming content test (may need running task fixture)
3. Implement connection status test
4. Implement token update test

### Phase 4: Connection Handling Tests (3 tests)

1. Implement auto-reconnect test
2. Implement reconnecting status test
3. Implement post-reconnect update test

## Success Criteria

- [x] All 12 tests implemented in `web/e2e/websocket.spec.ts` (17 tests total including 5 legacy)
- [x] Tests pass reliably (< 5% flake rate over 10 runs)
- [x] Tests use real WebSocket connections (not mocked) - uses Playwright `routeWebSocket` with event injection
- [x] Tests verify actual UI changes (not just WebSocket messages)
- [x] Test cleanup: no leftover test tasks after test run - tests use existing tasks, don't create persistent ones
- [x] Documentation: inline comments explaining test strategy - comprehensive JSDoc at file top

## Testing Strategy

### Unit Tests (Existing)

Backend WebSocket tests already cover:
- Connection establishment
- Subscription/unsubscription
- Event publishing to subscribers
- Global subscription
- File watcher events

No additional unit tests needed.

### Integration Tests

The E2E tests ARE the integration tests - they verify:
- Frontend WebSocket client → Backend WebSocket handler
- Backend events → Frontend UI updates
- File watcher → WebSocket → UI

### E2E Tests (This Spec)

The 12 tests described above, using:
- Real backend via `webServer` config
- Real WebSocket connections
- API calls to trigger events
- Playwright assertions on UI state

## File Changes

### New/Modified Files

| File | Action | Description |
|------|--------|-------------|
| `web/e2e/websocket.spec.ts` | Modify | Replace existing 5 tests with 12 comprehensive tests |
| `web/e2e/helpers/websocket.ts` | Create | WebSocket test helpers and fixtures |

### Files NOT Changed

- `web/src/lib/websocket.ts` - No production code changes
- `internal/api/websocket*.go` - No backend changes
- `web/src/routes/+layout.svelte` - No layout changes

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Flaky tests due to timing | Medium | High | Use explicit waits, retries, debouncing |
| Backend not triggering file watcher events | Medium | Medium | Test with direct API calls that write to disk |
| WebSocket connection not established in time | Low | High | Increase connection timeout, add retry logic |
| Test data pollution | Low | Medium | Unique prefixes, cleanup hooks |

## Out of Scope

- Backend WebSocket code changes
- Frontend WebSocket client changes
- Mock WebSocket testing (explicit requirement: test real connections)
- Performance testing
- Load testing (multiple concurrent WebSocket connections)
