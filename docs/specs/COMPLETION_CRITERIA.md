# Orc Completion Criteria

**Purpose**: Define ALL requirements for a fully functional, production-ready orchestration system.

---

## System Overview

Orc must enable users to:
1. **Create and manage tasks** via CLI and UI
2. **Build and customize prompts/templates** for any project
3. **Execute tasks** with live streaming visibility
4. **Control execution** (pause, resume, retry, skip, approve)
5. **Track progress** with full audit trail
6. **Integrate with any project** via initialization

---

## Backend Completion Criteria

### 1. Core Task Management

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Create task with title, description, weight | ✅ Basic | Unit + Integration |
| Auto-classify task weight via Claude | ⚠️ TODO | Unit + Integration |
| Load/save task to YAML | ✅ Done | Unit |
| List tasks with filtering | ✅ Basic | Unit |
| Delete task and cleanup | ⚠️ TODO | Unit + Integration |
| Task status transitions | ✅ Done | Unit |

### 2. Plan Generation

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Load plan templates by weight | ✅ Done | Unit |
| Generate plan from template | ✅ Done | Unit |
| Custom plan overrides | ⚠️ TODO | Unit |
| Phase dependency resolution | ✅ Done | Unit |
| Validate plan structure | ⚠️ TODO | Unit |

### 3. Executor (Claude Integration)

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Execute phase via Claude CLI | ✅ Basic | Integration |
| Completion detection (`<phase_complete>`) | ✅ Done | Unit |
| Block detection (`<phase_blocked>`) | ✅ Done | Unit |
| Max iteration limit | ✅ Done | Unit |
| Token tracking | ✅ Done | Unit |
| Cross-phase retry with context | ✅ Done | Integration |
| Retry context injection | ✅ Done | Unit |
| Phase timeout handling | ⚠️ TODO | Integration |
| Interrupt/resume support | ✅ Basic | Integration |

### 4. Gate Evaluation

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Auto gate (pass on success) | ✅ Done | Unit |
| AI gate (Claude evaluation) | ✅ Basic | Integration |
| Human gate (approval workflow) | ⚠️ TODO | Integration |
| Gate decision recording | ✅ Done | Unit |
| Gate rejection handling | ✅ Done | Unit |
| Configurable gate types | ✅ Done | Unit |

### 5. Git Integration

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Create task branch | ✅ Done | Integration |
| Checkpoint commits | ✅ Done | Integration |
| Rewind to checkpoint | ⚠️ TODO | Integration |
| Branch cleanup | ⚠️ TODO | Integration |
| Worktree support | ⚠️ TODO | Integration |

### 6. API Server

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Health endpoint | ✅ Done | Unit |
| List tasks | ✅ Done | Unit + Integration |
| Get task | ✅ Done | Unit |
| Create task | ✅ Done | Integration |
| Delete task | ⚠️ TODO | Integration |
| Get task state | ✅ Done | Unit |
| Get task plan | ✅ Done | Unit |
| Run task | ⚠️ Stub | Integration |
| Pause task | ⚠️ Stub | Integration |
| Resume task | ⚠️ Stub | Integration |
| Rewind task | ⚠️ TODO | Integration |
| Skip phase | ⚠️ TODO | Integration |
| Approve gate | ⚠️ TODO | Integration |
| Reject gate | ⚠️ TODO | Integration |
| SSE streaming | ✅ Basic | Integration |
| Real-time transcript | ⚠️ TODO | Integration |
| CORS support | ✅ Done | Unit |

### 7. Template/Prompt Management

| Feature | Status | Tests Required |
|---------|--------|----------------|
| List available templates | ⚠️ TODO | Unit |
| Get template content | ⚠️ TODO | Unit |
| Create custom template | ⚠️ TODO | Integration |
| Update template | ⚠️ TODO | Integration |
| Delete template | ⚠️ TODO | Integration |
| Template variable substitution | ✅ Done | Unit |
| Template validation | ⚠️ TODO | Unit |

### 8. Project Configuration

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Initialize orc in project | ✅ Done | Integration |
| Load/save config | ✅ Done | Unit |
| Automation profiles | ✅ Done | Unit |
| Gate configuration | ✅ Done | Unit |
| Retry configuration | ✅ Done | Unit |
| Custom prompts directory | ⚠️ TODO | Unit |
| Project-specific overrides | ⚠️ TODO | Unit |

---

## Frontend Completion Criteria

### 1. Task Management UI

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Task list with filtering | ✅ Basic | E2E |
| Task search | ⚠️ TODO | E2E |
| Task creation form | ✅ Basic | E2E |
| Weight selection | ⚠️ TODO | E2E |
| Description editor | ⚠️ TODO | E2E |
| Task detail view | ✅ Basic | E2E |
| Task deletion | ⚠️ TODO | E2E |

### 2. Execution Control UI

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Run task button | ✅ Basic | E2E |
| Pause task button | ✅ Basic | E2E |
| Resume task button | ⚠️ TODO | E2E |
| Stop task button | ⚠️ TODO | E2E |
| Rewind to phase | ⚠️ TODO | E2E |
| Skip phase | ⚠️ TODO | E2E |
| Retry phase | ⚠️ TODO | E2E |

### 3. Timeline/Progress UI

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Phase timeline | ✅ Basic | E2E |
| Current phase indicator | ✅ Done | E2E |
| Phase status colors | ✅ Done | E2E |
| Iteration count | ✅ Done | E2E |
| Phase duration | ⚠️ TODO | E2E |
| Token usage per phase | ⚠️ TODO | E2E |

### 4. Live Transcript UI

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Transcript container | ✅ Basic | E2E |
| Real-time SSE updates | ⚠️ TODO | E2E |
| Prompt display | ✅ Basic | E2E |
| Response display | ✅ Basic | E2E |
| Tool call display | ⚠️ TODO | E2E |
| Error display | ⚠️ TODO | E2E |
| Auto-scroll | ⚠️ TODO | E2E |
| Search/filter | ⚠️ TODO | E2E |

### 5. Gate Approval UI

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Pending gates indicator | ⚠️ TODO | E2E |
| Approve button | ⚠️ TODO | E2E |
| Reject with reason | ⚠️ TODO | E2E |
| Gate history | ⚠️ TODO | E2E |

### 6. Template/Prompt Editor

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Template list | ⚠️ TODO | E2E |
| Template viewer | ⚠️ TODO | E2E |
| Template editor | ⚠️ TODO | E2E |
| Variable highlighting | ⚠️ TODO | E2E |
| Save/revert | ⚠️ TODO | E2E |
| Create new template | ⚠️ TODO | E2E |

### 7. Configuration UI

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Config viewer | ⚠️ TODO | E2E |
| Profile selector | ⚠️ TODO | E2E |
| Gate overrides | ⚠️ TODO | E2E |
| Retry settings | ⚠️ TODO | E2E |
| Save config | ⚠️ TODO | E2E |

### 8. Project Initialization UI

| Feature | Status | Tests Required |
|---------|--------|----------------|
| Init wizard | ⚠️ TODO | E2E |
| Project detection | ⚠️ TODO | E2E |
| Template selection | ⚠️ TODO | E2E |
| Custom prompts setup | ⚠️ TODO | E2E |

---

## E2E Test Scenarios

### Critical Path Tests

1. **Full Task Lifecycle**
   - Create task via UI
   - Verify plan generated
   - Run task
   - Watch live transcript
   - See completion
   - Verify git checkpoint

2. **Cross-Phase Retry**
   - Create task with test phase
   - Run until test fails
   - Verify automatic retry from implement
   - Verify retry context injected
   - See successful completion

3. **Gate Approval Flow**
   - Create task with human gate
   - Run until gate reached
   - Approve via UI
   - Continue execution
   - Complete task

4. **Rewind and Resume**
   - Create and run task
   - Pause mid-execution
   - Rewind to earlier phase
   - Resume execution
   - Complete task

5. **Template Customization**
   - Create custom template
   - Create task using custom template
   - Verify custom prompts used
   - Complete task

### Error Handling Tests

1. **Network Failure**
   - Disconnect during execution
   - Verify graceful degradation
   - Reconnect and resume

2. **Claude Timeout**
   - Simulate slow response
   - Verify timeout handling
   - Verify retry behavior

3. **Invalid Input**
   - Submit invalid task data
   - Verify error messages
   - Verify form validation

---

## Testing Infrastructure

### Unit Tests (Go)

```bash
go test ./... -v -race -cover
```

Coverage targets:
- `internal/task/` - 90%+
- `internal/plan/` - 90%+
- `internal/state/` - 90%+
- `internal/executor/` - 80%+
- `internal/api/` - 80%+
- `internal/config/` - 90%+

### Integration Tests (Go)

```bash
go test ./... -tags=integration -v
```

Requires:
- Git repository
- Claude CLI (mock or real)
- Temp directories

### Frontend Tests (Svelte)

```bash
cd web && npm run test
```

Using: Vitest + Testing Library

### E2E Tests (Playwright MCP)

**IMPORTANT**: Use Playwright MCP tools for E2E testing. These are available as MCP server tools:
- `mcp__playwright__browser_navigate` - Navigate to URLs
- `mcp__playwright__browser_snapshot` - Capture accessibility snapshots
- `mcp__playwright__browser_click` - Click elements
- `mcp__playwright__browser_type` - Type text
- `mcp__playwright__browser_fill_form` - Fill forms
- `mcp__playwright__browser_wait_for` - Wait for conditions

```bash
# Start servers for testing
make serve &          # API on :8080
make web-dev &        # Frontend on :5173

# Use Playwright MCP tools to test
# Navigate, interact, verify via MCP commands
```

E2E Test Flow:
1. Navigate to `http://localhost:5173`
2. Use `browser_snapshot` to verify page state
3. Use `browser_click` / `browser_type` to interact
4. Verify results via snapshots
5. Check API responses via `browser_network_requests`

---

## Definition of Done

### Backend Done When:
- [ ] All features marked ✅ in tables above
- [ ] Unit test coverage > 80%
- [ ] Integration tests pass
- [ ] No race conditions (`go test -race`)
- [ ] API documented (OpenAPI spec)
- [ ] Error messages are actionable

### Frontend Done When:
- [ ] All features marked ✅ in tables above
- [ ] Component tests pass
- [ ] E2E tests pass
- [ ] Responsive design works
- [ ] Keyboard navigation works
- [ ] Loading/error states handled
- [ ] Dark theme consistent

### System Done When:
- [ ] Full task lifecycle works via UI
- [ ] Live streaming works reliably
- [ ] Cross-phase retry demonstrated
- [ ] Gate approval workflow works
- [ ] Template customization works
- [ ] Documentation complete
- [ ] No critical bugs
- [ ] Performance acceptable (< 100ms API responses)

---

## Acceptance Criteria

The system is **COMPLETE** when a user can:

1. `orc init` in any project
2. Create a task via web UI with custom description
3. See auto-classification or select weight
4. Watch live execution with streaming transcript
5. See phase progress on timeline
6. Pause/resume/retry as needed
7. Approve human gates when required
8. See successful completion with git checkpoint
9. Customize prompts for their project
10. Run fully automated with `--profile auto`

All of the above must be covered by automated tests.
