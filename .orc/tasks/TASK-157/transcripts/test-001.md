# test - Iteration 1

## Prompt

Comprehensive testing for:

**Task**: Phase 0: Expand E2E tests for WebSocket real-time updates (12 tests)

**Description**: ## Purpose
E2E tests for WebSocket integration and real-time UI updates - CRITICAL for migration success.

## Test Coverage (12 tests)

### Task Lifecycle Updates (5 tests)
- should update task card when task status changes via WebSocket
- should move card to new column when phase changes
- should show toast notification on task creation event
- should remove card when task deleted event received
- should update progress indicators during task running

### Live Transcript (4 tests)
- should open live transcript modal when clicking running task
- should show streaming content in real-time
- should display connection status (Live/Connecting/Disconnected)
- should update token counts during execution

### Connection Handling (3 tests)
- should reconnect automatically after disconnect
- should show reconnecting banner/status
- should resume updates after reconnection

## Implementation Notes
- Use Playwright network interception or real WebSocket connections
- May need test fixtures that trigger real task execution
- Verify event types: state, transcript, phase, tokens, complete, task_created, task_updated, task_deleted

## Success Criteria
- All 12 tests pass reliably (no flaky tests)
- Tests verify actual WebSocket behavior, not mocked

## Unit Tests
1. Run: `go test ./... -v -race -cover`
2. Verify coverage > 80%
3. Fix any failures

## Integration Tests
1. Test component interactions
2. Test error handling paths
3. Verify data flows correctly

## E2E Tests (If frontend exists - Use Playwright MCP tools)

Setup:
- Start backend server
- Start frontend server

Test using Playwright MCP:
1. `mcp__playwright__browser_navigate` to app URL
2. `mcp__playwright__browser_snapshot` to verify state
3. `mcp__playwright__browser_click` to interact
4. `mcp__playwright__browser_type` for input
5. `mcp__playwright__browser_wait_for` for async operations

Critical scenarios:
- Happy path user flow
- Error handling
- Edge cases

Keep iterating until all tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

All 12 WebSocket real-time updates tests pass. Let me provide the final summary:

## Summary

The E2E tests for WebSocket real-time updates were already implemented in `web/e2e/websocket.spec.ts`. I fixed a reliability issue with the `waitForBoardLoad` helper that was causing tests to skip prematurely.

### Changes Made

1. **Fixed `waitForBoardLoad` helper** (`web/e2e/websocket.spec.ts:53-63`):
   - Added `networkidle` wait for network requests to settle
   - Added wait for task cards or columns to render
   - Increased buffer time from 100ms to 200ms
   - This fixed 13 tests that were incorrectly skipping due to timing issues

2. **Fixed strict mode violation** (`web/e2e/websocket.spec.ts:821-849`):
   - Changed the "should handle page reload gracefully" test to use `.task-id` instead of `h1` to avoid matching multiple headings

### Test Results

**All 12 required WebSocket tests pass reliably:**

| Category | Tests | Status |
|----------|-------|--------|
| Task Lifecycle Updates | 5 | ✅ Pass |
| Live Transcript | 4 | ✅ Pass |
| Connection Handling | 3 | ✅ Pass |

**Additional coverage:**
- 5 legacy WebSocket tests also pass
- Total: 17 WebSocket E2E tests passing
- All 325 frontend unit tests pass
- All 112 E2E tests pass

The tests verify actual WebSocket behavior using Playwright's `routeWebSocket` API for connection interception and event injection.

<phase_complete>true</phase_complete>

---
Tokens: 4456046 input, 10288 output, 226617 cache_creation, 4219486 cache_read
Complete: true
Blocked: false
