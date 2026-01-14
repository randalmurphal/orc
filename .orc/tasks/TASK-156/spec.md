# Specification: E2E Tests for Initiative Management

## Overview

Create comprehensive E2E tests for Initiative CRUD operations and the Initiative detail page. These tests validate the full initiative lifecycle from creation through archival, including task linking, decision recording, and dependency graph visualization.

## Requirements

### Functional Requirements

1. **Initiative CRUD Operations**
   - Display initiatives in sidebar (expanded state)
   - Create initiatives via NewInitiativeModal
   - Navigate to initiative detail page
   - Edit initiative title and vision via modal
   - Transition status: draft → active → completed
   - Archive initiative with confirmation dialog

2. **Initiative Detail Page**
   - Show progress bar with task completion percentage
   - Display three tabs: Tasks, Graph, Decisions
   - Tab content loads on selection

3. **Task Linking**
   - Add new task to initiative (redirects to task creation)
   - Link existing task via search modal
   - Unlink task with confirmation
   - Filter search results (exclude already-linked tasks)
   - Update task count in tab badge after operations

4. **Decisions**
   - Add decision with optional rationale and author
   - Display decision metadata (date, author)
   - Show all recorded decisions in chronological order

5. **Dependency Graph**
   - Load graph data when Graph tab selected
   - Display task nodes with status-colored indicators
   - Show edges between dependent tasks

### Non-Functional Requirements

- Tests must be framework-agnostic (support future React migration)
- Use role/aria-label selectors where possible
- Follow existing test patterns from board.spec.ts
- Tests should be resilient to timing issues (use proper waits)

## Technical Approach

### Test File Structure

Create a new file: `web/e2e/initiative.spec.ts`

```typescript
/**
 * Initiative E2E Tests
 *
 * Framework-agnostic tests for Initiative management.
 * Covers CRUD, detail page, task linking, decisions, and dependency graph.
 *
 * Test Coverage (20 tests):
 * - Initiative CRUD (6): sidebar display, create, navigate, edit, status transitions, archive
 * - Detail Page (4): progress bar, tasks tab, decisions tab, graph tab
 * - Task Linking (5): add new, link existing, unlink, filter, count updates
 * - Decisions (3): add decision, metadata display, list display
 * - Dependency Graph (2): graph loading, node/edge display
 */
```

### Helper Functions

```typescript
// Wait for initiative detail page to load
async function waitForInitiativeDetailLoad(page: Page): Promise<void>

// Navigate to initiative detail page
async function navigateToInitiativeDetail(page: Page, initiativeId: string): Promise<void>

// Click a tab in initiative detail
async function clickInitiativeTab(page: Page, tabName: 'Tasks' | 'Graph' | 'Decisions'): Promise<void>

// Open new initiative modal from sidebar
async function openNewInitiativeModal(page: Page): Promise<void>

// Create an initiative and return its ID
async function createInitiative(page: Page, title: string, vision?: string): Promise<string>

// Clean up test initiatives (call in afterEach)
async function cleanupInitiatives(page: Page): Promise<void>
```

### Selector Strategy

| Element | Selector | Rationale |
|---------|----------|-----------|
| Sidebar initiatives section | `.initiatives-section` | Structural class |
| Initiative items | `.initiative-item` | Structural class |
| New Initiative button | `.new-initiative-btn` | Structural class |
| Initiative title | `.initiative-title` | Structural class |
| Tab navigation | `[role="tablist"]` | ARIA role |
| Tab buttons | `[role="tab"]` | ARIA role |
| Tab content | `.tab-content` | Structural class |
| Progress bar | `.progress-bar` | Structural class |
| Task list | `.task-list` | Structural class |
| Decision list | `.decision-list` | Structural class |
| Graph container | `.graph-container-wrapper` | Structural class |
| Modal | `[role="dialog"]` | ARIA role |
| Modal title | `.modal-title, [role="dialog"] h2` | Mixed strategy |

## Component Breakdown

### Frontend Components Tested

| Component | Location | Tests |
|-----------|----------|-------|
| Sidebar.svelte | `web/src/lib/components/layout/` | Initiative list display |
| NewInitiativeModal.svelte | `web/src/lib/components/overlays/` | Create initiative |
| [id]/+page.svelte | `web/src/routes/initiatives/` | Detail page (all tabs) |
| Modal.svelte | `web/src/lib/components/overlays/` | Edit, link task, add decision |
| DependencyGraph.svelte | `web/src/lib/components/` | Graph visualization |

### API Endpoints Exercised

| Endpoint | Method | Test Area |
|----------|--------|-----------|
| `/api/initiatives` | GET | Sidebar list |
| `/api/initiatives` | POST | Create initiative |
| `/api/initiatives/:id` | GET | Detail page load |
| `/api/initiatives/:id` | PUT | Edit, status change |
| `/api/initiatives/:id/tasks` | GET | Tasks tab |
| `/api/initiatives/:id/tasks` | POST | Link task |
| `/api/initiatives/:id/tasks/:taskId` | DELETE | Unlink task |
| `/api/initiatives/:id/decisions` | POST | Add decision |
| `/api/initiatives/:id/dependency-graph` | GET | Graph tab |

## Test Cases (20 Total)

### Initiative CRUD (6 tests)

```typescript
describe('Initiative CRUD', () => {
  test('should display initiative list in sidebar', async ({ page }) => {
    // Navigate to board/tasks page
    // Expand sidebar if collapsed
    // Expand initiatives section if collapsed
    // Verify initiative items are visible
    // Check "All Tasks" option exists
    // Check "New Initiative" button exists
  });

  test('should create new initiative via modal', async ({ page }) => {
    // Click "New Initiative" button in sidebar
    // Wait for modal to open
    // Fill in title (required)
    // Fill in vision (optional)
    // Fill in owner initials (optional)
    // Submit form
    // Verify modal closes
    // Verify new initiative appears in sidebar
    // Verify toast notification
  });

  test('should navigate to initiative detail page', async ({ page }) => {
    // Create or use existing initiative
    // Right-click initiative in sidebar (or navigate via URL)
    // Go to /initiatives/:id
    // Verify page loads with initiative title
    // Verify tabs are visible (Tasks, Graph, Decisions)
    // Verify progress bar exists
  });

  test('should edit initiative title and vision', async ({ page }) => {
    // Navigate to initiative detail
    // Click "Edit" button
    // Wait for edit modal
    // Modify title
    // Modify vision
    // Save changes
    // Verify updated title in header
    // Verify updated vision in header
  });

  test('should change initiative status (draft -> active -> completed)', async ({ page }) => {
    // Create new initiative (starts as draft)
    // Navigate to detail page
    // Verify status badge shows "draft"
    // Click "Activate" button
    // Verify status changes to "active"
    // Click "Complete" button
    // Verify status changes to "completed"
    // Verify "Reopen" button appears
  });

  test('should archive initiative with confirmation', async ({ page }) => {
    // Navigate to initiative detail
    // Click "Archive" button
    // Verify confirmation modal appears
    // Verify modal message mentions initiative title
    // Click "Archive Initiative" button
    // Verify status changes to "archived"
    // Verify Archive button disappears
  });
});
```

### Initiative Detail Page (4 tests)

```typescript
describe('Initiative Detail Page', () => {
  test('should show progress bar with task completion percentage', async ({ page }) => {
    // Use initiative with linked tasks
    // Navigate to detail page
    // Verify progress bar is visible
    // Verify progress label shows "X/Y tasks (Z%)"
    // Verify progress fill width matches percentage
  });

  test('should display tasks tab with linked tasks', async ({ page }) => {
    // Navigate to initiative with tasks
    // Verify Tasks tab is active by default
    // Verify task list is visible
    // Verify each task shows: status icon, ID, title, status text
    // Verify "Add Task" and "Link Existing" buttons
  });

  test('should display decisions tab', async ({ page }) => {
    // Navigate to initiative with decisions
    // Click "Decisions" tab
    // Verify tab becomes active
    // Verify decisions section appears
    // Verify "Add Decision" button
  });

  test('should display graph tab with dependency visualization', async ({ page }) => {
    // Navigate to initiative with tasks (ideally with dependencies)
    // Click "Graph" tab
    // Verify tab becomes active
    // Wait for graph to load
    // Verify graph container is visible
    // (Note: Graph visualization details depend on DependencyGraph component)
  });
});
```

### Task Linking (5 tests)

```typescript
describe('Task Linking', () => {
  test('should add new task to initiative', async ({ page }) => {
    // Navigate to initiative detail, Tasks tab
    // Click "Add Task" button
    // Verify navigation to task creation (or modal opens)
    // (This test validates the navigation/event dispatch works)
  });

  test('should link existing task via search modal', async ({ page }) => {
    // Navigate to initiative detail, Tasks tab
    // Note initial task count
    // Click "Link Existing" button
    // Verify link modal opens
    // Verify search input is visible
    // Verify available tasks are listed
    // Click on a task to link it
    // Verify modal closes
    // Verify task appears in list
    // Verify task count increased
  });

  test('should unlink task from initiative', async ({ page }) => {
    // Navigate to initiative with linked tasks
    // Note initial task count
    // Hover over a task to reveal remove button
    // Click remove button (X icon)
    // Accept confirmation (browser confirm)
    // Verify task removed from list
    // Verify task count decreased
  });

  test('should filter available tasks (not already linked)', async ({ page }) => {
    // Navigate to initiative with some linked tasks
    // Open link task modal
    // Note available tasks
    // Verify linked tasks are NOT in the available list
    // Type in search input
    // Verify list filters by ID/title match
  });

  test('should update task count after linking/unlinking', async ({ page }) => {
    // Navigate to initiative, note Tasks tab badge count
    // Link a new task
    // Verify badge count incremented
    // Unlink the task
    // Verify badge count decremented back
  });
});
```

### Decisions (3 tests)

```typescript
describe('Decisions', () => {
  test('should add new decision with rationale', async ({ page }) => {
    // Navigate to initiative detail
    // Click "Decisions" tab
    // Click "Add Decision" button
    // Verify add decision modal opens
    // Fill in decision text (required)
    // Fill in rationale (optional)
    // Fill in "by" field (optional)
    // Submit form
    // Verify modal closes
    // Verify new decision appears in list
  });

  test('should show decision date and author', async ({ page }) => {
    // Add a decision with "by" field filled
    // Navigate to Decisions tab
    // Verify decision shows:
    //   - Decision ID (e.g., DEC-001)
    //   - Date (formatted)
    //   - "by {author}" text
    // Verify rationale is displayed if present
  });

  test('should display all recorded decisions', async ({ page }) => {
    // Use initiative with multiple decisions
    // Navigate to Decisions tab
    // Verify all decisions are visible
    // Verify they appear in order (newest first or chronological)
  });
});
```

### Dependency Graph (2 tests)

```typescript
describe('Dependency Graph', () => {
  test('should load graph when Graph tab selected', async ({ page }) => {
    // Navigate to initiative with tasks
    // Click "Graph" tab
    // Verify loading spinner appears briefly
    // Wait for graph to load
    // Verify graph container is visible
    // Verify no error state
  });

  test('should display task nodes with status colors and edges', async ({ page }) => {
    // Use initiative with tasks that have dependencies
    // Navigate to Graph tab, wait for load
    // Verify graph nodes are visible (SVG elements or canvas)
    // Verify nodes have status-appropriate colors
    // (Detailed SVG inspection may be framework-specific)
    // For framework-agnostic: just verify graph renders without error
  });
});
```

## Success Criteria

### Code Deliverables

- [ ] `web/e2e/initiative.spec.ts` - New test file with all 20 tests
- [ ] Helper functions for common operations
- [ ] Test fixtures/setup if needed

### Test Results

- [ ] All 20 tests pass locally (`bunx playwright test initiative.spec.ts`)
- [ ] Tests pass in CI (GitHub Actions)
- [ ] No test flakiness (run 3x to verify stability)

### Coverage Verification

- [ ] Initiative CRUD: 6 tests ✓
- [ ] Detail Page: 4 tests ✓
- [ ] Task Linking: 5 tests ✓
- [ ] Decisions: 3 tests ✓
- [ ] Dependency Graph: 2 tests ✓
- [ ] Total: 20 tests ✓

## Testing Strategy

### Unit Tests
Not applicable - this task is specifically E2E tests.

### Integration Tests
Not applicable - E2E tests cover integration through the UI.

### E2E Tests (This Task)

**Test Isolation:**
- Each test creates its own initiative for isolation
- Use `test.beforeEach` to ensure clean state
- Use `test.afterEach` to clean up created initiatives (optional, but good hygiene)

**Data Setup:**
- Some tests require pre-existing data (initiatives with tasks/decisions)
- Use API calls in beforeEach to set up required state
- Alternatively, create during test and navigate

**Timing Considerations:**
- Use `waitFor` patterns from existing tests
- Add animation buffers (100-300ms) where needed
- Use retry loops for flaky dropdowns (pattern from board.spec.ts)

**Selector Resilience:**
- Prefer role/aria-label over class names
- Use text content for buttons/links
- Avoid framework-specific classes

## Implementation Notes

### Gotchas from Exploration

1. **Sidebar must be expanded** to see initiative list - test setup should ensure this
2. **Initiatives section must be expanded** - localStorage key `orc-sidebar-sections` stores this
3. **Modal timing** - NewInitiativeModal uses setTimeout(50ms) for focus, add appropriate waits
4. **Browser confirm** - `unlinkTask` uses native `confirm()` dialog - Playwright handles this with `page.on('dialog')`
5. **Graph lazy loading** - Graph data loads only when tab is clicked, not on page load

### Test Data Requirements

For comprehensive testing, need:
- At least one initiative in each status (draft, active, completed)
- One initiative with multiple linked tasks
- One initiative with tasks that have dependencies (for graph)
- One initiative with recorded decisions

Can either:
1. Create via API in beforeEach
2. Create via UI during test flow
3. Rely on existing test data (less isolated)

Recommendation: Create via UI where possible to test full flow, use API for complex setups.

## Timeline

This is a focused test-writing task. Expected implementation time: ~2-3 hours.

1. Create test file with describe blocks and empty tests
2. Implement helper functions
3. Implement tests in order: CRUD → Detail Page → Task Linking → Decisions → Graph
4. Run and debug locally
5. Verify stability (3x run)
6. PR and CI validation
