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

The spec covers all 12 tests from the task description:

**Task Lifecycle Updates (5 tests)** ✅
1. Update task card on status change
2. Move card to new column on phase change  
3. Toast notification on task creation
4. Remove card on task deletion
5. Progress indicators during running

**Live Transcript (4 tests)** ✅
1. Open modal when clicking running task
2. Streaming content in real-time
3. Connection status display
4. Token count updates

**Connection Handling (3 tests)** ✅
1. Auto-reconnect after disconnect
2. Reconnecting banner/status
3. Resume updates after reconnection

The spec includes:
- Clear requirements with selectors
- Technical approach (real WebSocket, not mocked)
- Test helpers/fixtures
- Flakiness prevention strategies
- Implementation phases
- Success criteria with checkboxes
- File change list
- Risk mitigations

<phase_complete>true</phase_complete>

---
Tokens: 604261 input, 4885 output, 92828 cache_creation, 493834 cache_read
Complete: true
Blocked: false
