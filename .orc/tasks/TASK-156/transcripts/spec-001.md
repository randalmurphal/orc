# spec - Iteration 1

## Prompt

Create a specification for this large task:

**Task**: Phase 0: Expand E2E tests for Initiative management (20 tests)

**Description**: ## Purpose
Comprehensive E2E tests for Initiative CRUD and detail page functionality.

## Test Coverage (20 tests)

### Initiative CRUD (6 tests)
- should display initiative list in sidebar
- should create new initiative via modal
- should navigate to initiative detail page
- should edit initiative title and vision
- should change initiative status (draft -> active -> completed)
- should archive initiative with confirmation

### Initiative Detail Page (4 tests)
- should show progress bar with task completion percentage
- should display tasks tab with linked tasks
- should display decisions tab
- should display graph tab with dependency visualization

### Task Linking (5 tests)
- should add new task to initiative
- should link existing task via search modal
- should unlink task from initiative
- should filter available tasks (not already linked)
- should update task count after linking/unlinking

### Decisions (3 tests)
- should add new decision with rationale
- should show decision date and author
- should display all recorded decisions

### Dependency Graph (2 tests)
- should load graph when Graph tab selected
- should display task nodes with status colors and edges

## Success Criteria
- All 20 tests pass on current Svelte implementation
- Initiative lifecycle fully covered

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

I've created a comprehensive specification for the Initiative management E2E tests. The spec covers:

**20 tests organized into 5 categories:**
1. **Initiative CRUD (6 tests)** - sidebar display, create, navigate, edit, status transitions, archive
2. **Initiative Detail Page (4 tests)** - progress bar, tasks/graph/decisions tabs
3. **Task Linking (5 tests)** - add new, link existing, unlink, filter, count updates
4. **Decisions (3 tests)** - add decision, metadata display, list all
5. **Dependency Graph (2 tests)** - graph loading, node/edge display

**Key technical decisions:**
- Framework-agnostic selectors (ARIA roles + structural classes)
- Helper functions matching patterns from existing tests (board.spec.ts)
- Proper handling of gotchas: sidebar expansion, modal timing, browser confirm dialogs, lazy-loaded graph
- Test isolation via creating initiatives during tests

**Clear success criteria:**
- All 20 tests pass locally and in CI
- No flakiness (verified with 3x run)
- Coverage checkboxes for each category

<phase_complete>true</phase_complete>

---
Tokens: 518940 input, 5182 output, 34503 cache_creation, 479808 cache_read
Complete: true
Blocked: false
