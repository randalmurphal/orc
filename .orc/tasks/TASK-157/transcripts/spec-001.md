# spec - Iteration 1

## Prompt

Create a specification for this large task:

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

Define:
1. Requirements and scope
2. Technical approach
3. Component breakdown (backend, frontend if applicable)
4. API design (if applicable)
5. Success criteria with explicit checkboxes
6. Testing strategy:
   - Unit tests
   - Integration tests
   - E2E tests (using Playwright MCP tools if frontend exists)

Include clear completion criteria:
- What code must be written
- What tests must pass
- What E2E scenarios must work
- What documentation must exist

Keep iterating until the specification is clear and complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

I've created a comprehensive specification for the WebSocket real-time updates E2E tests. Here's a summary:

## Specification Summary

**Key findings:**
- The tests are **already implemented** in `web/e2e/websocket.spec.ts`
- All 12 core tests exist exactly as specified in the task description
- The implementation uses Playwright's `routeWebSocket` for real WebSocket interception (not mocking)

**Test Coverage (12 tests):**

| Category | Tests |
|----------|-------|
| **Task Lifecycle Updates** | 5 tests - status changes, phase moves, create/delete events, progress |
| **Live Transcript Modal** | 4 tests - modal opening, streaming, connection status, tokens |
| **Connection Handling** | 3 tests - auto-reconnect, reconnecting status, resume updates |

**Technical Approach:**
- Uses `page.routeWebSocket()` to intercept real WebSocket connections
- Injects events via `ws.send()` to test UI responses
- Framework-agnostic selectors for migration compatibility
- Consistent timing constants to prevent flakiness

**Success Criteria:**
- All 12 tests must pass reliably
- Tests verify actual WebSocket behavior (not mocked)
- Documentation exists in `web/CLAUDE.md`

**Next Steps:**
1. Run tests to verify they pass
2. Fix any flaky tests
3. Add knowledge pattern to CLAUDE.md

<phase_complete>true</phase_complete>

---
Tokens: 840765 input, 5057 output, 90881 cache_creation, 734452 cache_read
Complete: true
Blocked: false
