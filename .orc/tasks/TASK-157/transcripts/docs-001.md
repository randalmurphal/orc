# docs - Iteration 1

## Prompt

Update documentation for:

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

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

No project README.md in the main project directory - all those are from node_modules. The CLAUDE.md files serve as the documentation for this project.

Based on my review, the documentation for TASK-157 is complete:

1. **Main `CLAUDE.md`** - Already contains the "WebSocket E2E event injection" pattern learned from this task
2. **`web/CLAUDE.md`** - Already has comprehensive documentation for:
   - Test file listing including `websocket.spec.ts` (17 tests)
   - Complete "WebSocket E2E Testing" section with patterns, event types, and helpers
   - Framework-agnostic testing approach
3. **Test file comments** - `websocket.spec.ts` has a detailed JSDoc header
4. **API docs** - `docs/API_REFERENCE.md` documents all WebSocket event types

The documentation is complete and accurately reflects the implementation.

<phase_complete>true</phase_complete>

---
Tokens: 901385 input, 2199 output, 91914 cache_creation, 809458 cache_read
Complete: true
Blocked: false
