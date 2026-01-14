# implement - Iteration 1

## Prompt

Implement the large task according to the specification:

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

**Specification**:
## Purpose
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



Implementation protocol:
1. Implement all components defined in the spec
2. Write unit tests alongside code
3. Run tests frequently: `go test ./... -v -race`
4. Fix failures before continuing
5. If frontend exists:
   - Implement all components
   - Add loading/error states
   - Integrate with API

Keep iterating until:
- All components implemented
- All unit tests pass
- No race conditions

When done, output:
<phase_complete>true</phase_complete>


## Response

The test breakdown:
- **Initiative CRUD (6 tests)**: ✓ display list, create via modal, navigate, edit, status change, archive
- **Initiative Detail Page (4 tests)**: ✓ progress bar, tasks tab, decisions tab, graph tab
- **Task Linking (5 tests)**: ✓ add new, link existing, unlink, filter available, update count
- **Decisions (3 tests)**: ✓ add with rationale, show date/author, display all
- **Dependency Graph (2 tests)**: ✓ load on tab, display nodes with colors

All 20 tests match the specification exactly.

<phase_complete>true</phase_complete>

---
Tokens: 3262024 input, 15269 output, 81199 cache_creation, 3180794 cache_read
Complete: true
Blocked: false
