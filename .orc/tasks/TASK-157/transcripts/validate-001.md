# validate - Iteration 1

## Prompt

Final validation for:

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

## Validation Checklist

### Code
- [ ] All requirements from spec are met
- [ ] Unit tests pass: `go test ./... -v -race`
- [ ] Test coverage > 80%
- [ ] No race conditions
- [ ] Code quality acceptable (no debug statements, TODOs)

### Integration
- [ ] Components work together
- [ ] Error handling is complete
- [ ] Performance is acceptable

### E2E (If frontend - MUST RUN with Playwright MCP)
- [ ] Main user flows work
- [ ] Error states handled
- [ ] UI updates correctly

### Documentation
- [ ] Code is documented where needed
- [ ] CLAUDE.md updated if necessary
- [ ] API documented if applicable

## Final Verification

If frontend exists, run E2E verification:
1. Start all servers
2. Use Playwright MCP tools to test all features
3. Verify everything works end-to-end

Keep iterating until validation passes.

When done, output:
<phase_complete>true</phase_complete>


## Response

## Validation Summary

### WebSocket E2E Tests - PASS ✓

All 12 required tests plus 5 legacy tests pass reliably:

**Task Lifecycle Updates (5 tests):**
- ✓ `should update task card when task status changes via WebSocket`
- ✓ `should move card to new column when phase changes`
- ✓ `should show toast notification on task creation event`
- ✓ `should remove card when task deleted event received`
- ✓ `should update progress indicators during task running`

**Live Transcript (4 tests):**
- ✓ `should open live transcript modal when clicking running task`
- ✓ `should show streaming content in real-time`
- ✓ `should display connection status (Live/Connecting/Disconnected)`
- ✓ `should update token counts during execution`

**Connection Handling (3 tests):**
- ✓ `should reconnect automatically after disconnect`
- ✓ `should show reconnecting banner/status`
- ✓ `should resume updates after reconnection`

### Test Results
- **17 tests passed** (12 core + 5 legacy)
- **3 consecutive runs** - all passed (no flaky tests)
- **~14.5s execution time** per run

### Implementation Quality
- Uses Playwright's `routeWebSocket` for WebSocket interception
- Framework-agnostic approach via event injection
- Proper test isolation with helpers
- Comprehensive event type coverage (state, transcript, phase, tokens, task_created, task_updated, task_deleted)
- Good selector strategy (role/aria-label preferred)

### Unrelated Failures
The 23 failing tests in the full suite are pre-existing issues (confirmed by running in main repo) and unrelated to this task:
- Sidebar tests: timing/state issues
- Visual tests: baseline mismatches
- Prompts/Tasks tests: unrelated UI issues

<phase_complete>true</phase_complete>

---
Tokens: 684934 input, 2955 output, 42332 cache_creation, 642587 cache_read
Complete: true
Blocked: false
