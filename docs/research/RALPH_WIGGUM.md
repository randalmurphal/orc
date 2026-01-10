# Ralph Wiggum Technique

**Core Insight**: The prompt never changes - the codebase does. Each iteration reads the same instructions but operates on evolved state.

---

## The Fundamental Pattern

```bash
while :; do cat PROMPT.md | claude-code ; done
```

**Why it works**:
1. Prompt contains stable goals and completion criteria
2. Filesystem reflects current state (code, logs, progress markers)
3. Each iteration picks up where the last left off
4. No complex state management - git IS the checkpoint system

---

## Architecture

```
┌────────────────────────────────────────────────────┐
│                    PROMPT.md                       │
│  - Goals (what to build)                          │
│  - Completion criteria (when to stop)             │
│  - Constraints (what not to do)                   │
│  - Self-correction rules (how to recover)         │
└────────────────────────────────────────────────────┘
                        │
                        ▼
┌────────────────────────────────────────────────────┐
│             while :; do ... ; done                 │
│                                                    │
│  ┌────────────┐    ┌─────────────┐               │
│  │ cat PROMPT │───►│ claude-code │               │
│  └────────────┘    └──────┬──────┘               │
│                           │                       │
│                           ▼                       │
│                  ┌─────────────────┐             │
│                  │   Filesystem    │             │
│                  │ (shared state)  │             │
│                  └─────────────────┘             │
└────────────────────────────────────────────────────┘
```

---

## Completion Detection

**XML tag pattern**:
```markdown
I've completed all tasks.
<phase_complete>true</phase_complete>
```

**File-based signals**:
```bash
if [ -f ".ralph-complete" ]; then exit 0; fi
```

---

## Prompt Engineering for Full System Builds

### Structure for Comprehensive Projects

```markdown
# Goal
Build [SYSTEM] - a complete [DESCRIPTION].

# Architecture
- Backend: [TECH STACK]
- Frontend: [TECH STACK]
- Storage: [APPROACH]

# Completion Criteria (ALL must be true)

## Backend
1. All API endpoints functional
2. All business logic implemented
3. Unit tests pass with >80% coverage
4. Integration tests pass
5. Error handling complete

## Frontend
1. All UI components implemented
2. All user flows work end-to-end
3. Component tests pass
4. Responsive design works
5. Loading/error states handled

## E2E (Use Playwright MCP tools)
1. Full user journey works
2. All critical paths tested
3. Error scenarios handled
4. Performance acceptable

## Documentation
1. API documented
2. README updated
3. CLAUDE.md current

# When ALL criteria met
Output: <phase_complete>true</phase_complete>
```

### Self-Correction Rules

```markdown
# If you encounter errors

1. Read the error message carefully
2. Check git log for recent changes
3. If test fails:
   - Run single test with verbose output
   - Fix the specific failure
   - Re-run full suite
4. If stuck for 3 iterations on same error:
   - Write analysis to `.stuck.md`
   - Try alternative approach
5. If blocked on external dependency:
   - Document in `.blocked.md`
   - Continue with other tasks

# E2E Testing Protocol

Use Playwright MCP tools for E2E verification:
1. `mcp__playwright__browser_navigate` - Go to page
2. `mcp__playwright__browser_snapshot` - Verify state
3. `mcp__playwright__browser_click` - Interact
4. `mcp__playwright__browser_type` - Input text
5. `mcp__playwright__browser_fill_form` - Fill forms
6. `mcp__playwright__browser_wait_for` - Wait for conditions

After each major feature:
- Start servers: `make serve & make web-dev &`
- Navigate to frontend
- Test the feature via MCP tools
- Verify expected behavior
- Fix any issues found
```

---

## When to Use Ralph Wiggum

| Good Fit | Poor Fit |
|----------|----------|
| Greenfield projects | Judgment-heavy decisions |
| Well-defined specs | Ambiguous requirements |
| Full system builds | Exploratory research |
| Test-driven development | Security-critical code |
| E2E feature implementation | Novel architecture design |

---

## Key Insight

> "Deterministically bad in an undeterministic world"

Ralph's failures are predictable:
- Stuck on same error? Predictable recovery path.
- Wrong approach? Adjust constraints.

Simple systems fail simply. You can debug a bash loop.

---

## Orc Integration

Orc uses Ralph-style loops **within structured phases**:
- Each phase has completion criteria
- Loops until criteria met or max iterations
- Checkpoints between phases (git commits)
- Configurable gates (auto by default, human optional)

### Automation-First Philosophy

Orc defaults to **fully automated execution**:

1. **All gates are auto by default** - No human approvals needed
2. **Cross-phase retry** - Test failures automatically retry from implement
3. **Retry context** - When retrying, the agent knows WHY it failed

### Cross-Phase Retry

When a later phase fails, orc automatically retries from an earlier phase with context:

```yaml
# Default retry configuration
retry:
  enabled: true
  max_retries: 3
  retry_map:
    test: implement      # Test failures retry from implement
    validate: implement  # Validation failures retry from implement
```

The agent receives a `{{RETRY_CONTEXT}}` with:
- What phase failed
- Why it failed (error or gate rejection)
- Output from the failed phase
- Which retry attempt this is

### Automation Profiles

| Profile | Description |
|---------|-------------|
| `auto` | Default - All gates auto, full retry |
| `fast` | Max speed - No gates, no retry |
| `safe` | Balanced - Human gate on merge only |
| `strict` | Full oversight - Human gates on spec/merge |

```bash
# Run with specific profile
orc run TASK-001 --profile auto    # Full automation (default)
orc run TASK-001 --profile safe    # Human on merge only
```

---

## Full System Build Template

For building complete systems like orc itself:

```markdown
# SYSTEM BUILD PROMPT

## Objective
Build a fully functional [SYSTEM] with:
- Complete backend API
- Complete frontend UI
- Full test coverage
- E2E verification

## Current State
Review the codebase to understand:
- What's implemented
- What's missing
- What tests exist

## Completion Criteria

### Backend (Go)
- [ ] All API endpoints implemented and tested
- [ ] All business logic complete
- [ ] Error handling with actionable messages
- [ ] Unit tests: `go test ./... -v -race -cover`
- [ ] Integration tests pass
- [ ] No race conditions

### Frontend (Svelte 5)
- [ ] All pages/routes implemented
- [ ] All components functional
- [ ] State management works
- [ ] API integration complete
- [ ] Loading/error states
- [ ] Responsive design

### E2E Testing (Playwright MCP)
- [ ] Start servers: `make serve & make web-dev &`
- [ ] Navigate to `http://localhost:5173`
- [ ] Test all critical user flows
- [ ] Verify via `browser_snapshot`
- [ ] All flows pass

### Documentation
- [ ] CLAUDE.md current
- [ ] README accurate
- [ ] API documented

## Iteration Protocol

Each iteration:
1. Review current state
2. Identify highest priority incomplete item
3. Implement/fix it
4. Run relevant tests
5. If tests pass, continue
6. If tests fail, fix before continuing
7. After major features, run E2E tests

## When Complete
When ALL criteria are checked:
<phase_complete>true</phase_complete>
```

---

## E2E Testing Best Practices

### Using Playwright MCP Tools

```markdown
# E2E Test Flow

1. **Setup**
   - Ensure API server running on :8080
   - Ensure frontend running on :5173

2. **Navigate**
   - Use `mcp__playwright__browser_navigate` to go to page
   - Wait for load with `mcp__playwright__browser_wait_for`

3. **Verify Initial State**
   - Use `mcp__playwright__browser_snapshot` to capture state
   - Verify expected elements present

4. **Interact**
   - Use `mcp__playwright__browser_click` for buttons
   - Use `mcp__playwright__browser_type` for text input
   - Use `mcp__playwright__browser_fill_form` for forms

5. **Verify Results**
   - Use `mcp__playwright__browser_snapshot` after actions
   - Check for expected changes
   - Verify no error states

6. **Check API**
   - Use `mcp__playwright__browser_network_requests` to verify API calls
   - Check response status codes
```

### Critical Test Scenarios

1. **Task Creation Flow**
   - Navigate to home
   - Click "New Task"
   - Fill title
   - Submit
   - Verify task appears in list

2. **Task Execution Flow**
   - Navigate to task
   - Click "Run"
   - Verify streaming starts
   - Wait for completion
   - Verify status updated

3. **Error Handling**
   - Submit invalid data
   - Verify error message shown
   - Verify form state preserved

4. **Real-time Updates**
   - Start task execution
   - Verify transcript updates live
   - Verify timeline updates
   - Verify token counts update

---

## Industry Best Practices

### Code Quality
- Type safety (Go strict types, TypeScript strict mode)
- Error handling at boundaries
- Consistent naming conventions
- DRY but not over-abstracted

### Testing Pyramid
- Many unit tests (fast, isolated)
- Some integration tests (verify interactions)
- Few E2E tests (critical paths only)

### API Design
- RESTful endpoints
- Consistent error responses
- Proper HTTP status codes
- CORS configured correctly

### Frontend
- Component composition
- State lifted appropriately
- Optimistic UI updates
- Graceful degradation

### Security
- Input validation
- No secrets in code
- CORS restricted in production
- Sanitized error messages
