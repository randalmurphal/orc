# docs - Iteration 1

## Prompt

Update documentation for:

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

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

Documentation is complete. The changes made:

1. **Updated `web/CLAUDE.md`** - Added `e2e/initiatives.spec.ts` to the E2E test files table with proper coverage description (Initiative CRUD, detail page, task linking, decisions, dependency graph - 20 tests)

2. **Root CLAUDE.md** - No updates needed. The Initiative-related patterns were already documented from earlier implementation tasks (TASK-060 through TASK-076). This task added E2E tests to validate those existing features, not new functionality.

The E2E test file itself (`web/e2e/initiatives.spec.ts`) is well-documented with:
- Header comment explaining coverage breakdown (20 tests across 5 categories)
- Selector strategy documentation following framework-agnostic patterns
- Helper functions with clear purposes
- Test cases matching the spec exactly

<phase_complete>true</phase_complete>

---
Tokens: 607429 input, 1980 output, 39431 cache_creation, 567987 cache_read
Complete: true
Blocked: false
