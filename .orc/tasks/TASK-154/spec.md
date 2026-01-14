# TASK-154: Expand E2E Tests for Board Interactions (18 Tests)

## Purpose

Create comprehensive E2E tests for the Board page that define **behavior**, not implementation. These tests must work on both the current Svelte implementation and the future React implementation during the migration (INIT-005).

## Requirements

### Scope
- 18 E2E tests covering Board page interactions
- Framework-agnostic selectors (semantic roles, text content, ARIA attributes)
- No implementation-specific selectors (`.svelte-xyz`, React component refs)
- Tests must be deterministic and non-flaky

### Out of Scope
- Unit tests for Board components (separate task)
- API tests (covered by backend Go tests)
- Testing task creation flow (already covered in `tasks.spec.ts`)

## Technical Approach

### Selector Strategy

Following the pattern established in existing E2E tests, use this priority order:

| Priority | Method | Example | Use Case |
|----------|--------|---------|----------|
| 1 | `getByRole()` | `getByRole('region', { name: 'Queued column' })` | Columns, buttons, links |
| 2 | `getByText()` | `getByText('Task Board')` | Headings, labels |
| 3 | `.locator()` with class | `locator('.task-card')` | Structural elements |
| 4 | ARIA attributes | `locator('[aria-label="..."]')` | Custom accessible elements |

**Avoid:**
- CSS class fragments (`.svelte-abc123`)
- Implementation-specific attributes
- Deep DOM path selectors

### Existing Selectable Elements

From the Column component (`aria-label`):
```html
<div class="column" role="region" aria-label="{column.title} column">
```

From the Board page:
```html
<h1>Task Board</h1>
<span class="task-count">59 tasks</span>
<button>New Task</button>
```

From TaskCard:
```html
<div class="task-card" draggable="true" role="button" tabindex="0">
  <span class="task-id">TASK-001</span>
  <h3 class="task-title">...</h3>
</div>
```

### Data Attributes for Framework-Agnostic Testing

To ensure robust cross-framework testing, the following `data-testid` attributes should be added:

| Component | Attribute | Purpose |
|-----------|-----------|---------|
| Board page | `data-testid="board-page"` | Page identification |
| Column | `data-testid="column-{id}"` | Column targeting (queued, spec, implement, test, review, done) |
| Column count | `data-testid="column-count"` | Task count badge |
| TaskCard | `data-testid="task-card-{id}"` | Individual task cards |
| View toggle | `data-testid="view-mode-toggle"` | Flat/swimlane toggle |
| Swimlane | `data-testid="swimlane-{initiative-id}"` | Initiative swimlane row |
| Swimlane collapse | `data-testid="swimlane-toggle"` | Collapse button |

### Test File Structure

```
web/e2e/
├── board.spec.ts           # NEW: 18 Board interaction tests
├── tasks.spec.ts           # Existing: Task list/detail tests
├── sidebar.spec.ts         # Existing: Navigation tests
└── ...
```

## Test Specifications

### Test Group 1: Board Rendering (4 tests)

```typescript
test.describe('Board Rendering', () => {
  test('should display board page with all 6 columns');
  test('should show correct column headers and task counts');
  test('should render task cards in correct columns based on status/phase');
  test('should show task count in header');
});
```

**Test Details:**

1. **should display board page with all 6 columns**
   - Navigate to `/board`
   - Verify 6 columns visible: Queued, Spec, Implement, Test, Review, Done
   - Use `role="region"` with `aria-label` for column identification

2. **should show correct column headers and task counts**
   - Verify each column has header text (h2)
   - Verify each column has count badge (`.count`)
   - Count badge should show integer >= 0

3. **should render task cards in correct columns based on status/phase**
   - Requires at least one task to exist
   - Get first task from any column
   - Verify task card contains: task ID, title
   - Card should have `draggable="true"`

4. **should show task count in header**
   - Locate header `.task-count` element
   - Should match pattern `\d+ tasks?`
   - Total should match sum of all column counts

### Test Group 2: View Mode Toggle (5 tests)

```typescript
test.describe('View Mode Toggle', () => {
  test('should default to flat view mode');
  test('should switch to swimlane view when selected');
  test('should persist view mode in localStorage');
  test('should disable swimlane toggle when initiative filter active');
  test('should show initiative banner when filtering');
});
```

**Test Details:**

1. **should default to flat view mode**
   - Navigate to `/board`
   - View toggle should show "Flat" as selected
   - No swimlane rows should be visible

2. **should switch to swimlane view when selected**
   - Click view mode dropdown
   - Select "By Initiative"
   - Swimlanes should appear (`.swimlane` or equivalent)
   - Verify "Unassigned" swimlane visible (tasks without initiative)

3. **should persist view mode in localStorage**
   - Switch to swimlane view
   - Reload page
   - Should still show swimlane view
   - localStorage key: `orc-board-view-mode`

4. **should disable swimlane toggle when initiative filter active**
   - Select a specific initiative from filter dropdown
   - View mode toggle should be disabled or hidden
   - Rationale: Filtering by initiative already groups tasks

5. **should show initiative banner when filtering**
   - Select a specific initiative from filter dropdown
   - Initiative banner should appear below header
   - Banner should show initiative name
   - Should have "clear filter" action

### Test Group 3: Drag-Drop (5 tests) - CRITICAL

```typescript
test.describe('Drag-Drop', () => {
  test('should move task between columns (triggers status change API)');
  test('should reorder tasks within column (priority change)');
  test('should show visual feedback during drag');
  test('should update task status after drop completes');
  test('should handle drop cancellation (escape key)');
});
```

**Test Details:**

1. **should move task between columns (triggers status change API)**
   - Find a task in Queued column
   - Drag to Implement column
   - Confirmation modal should appear
   - Accept confirmation
   - Task should move to new column
   - API call should be made (verify via network interception)

2. **should reorder tasks within column (priority change)**
   - Find column with 2+ tasks
   - Drag first task below second task
   - Order should change (if reordering is supported)
   - Note: May show no-op if column drop doesn't reorder

3. **should show visual feedback during drag**
   - Start dragging a task card
   - Source card should have `.dragging` class
   - Target column should highlight (`.drag-over` class)

4. **should update task status after drop completes**
   - Move task from Queued to Implement
   - Accept confirmation modal
   - Wait for API response
   - Task should appear in new column
   - Task count in both columns should update

5. **should handle drop cancellation (escape key)**
   - Start dragging a task
   - Press Escape key
   - Task should return to original position
   - No confirmation modal should appear
   - No API call should be made

### Test Group 4: Swimlane View (4 tests)

```typescript
test.describe('Swimlane View', () => {
  test('should group tasks by initiative in swimlane view');
  test('should collapse/expand swimlanes');
  test('should persist collapsed state in localStorage');
  test('should show Unassigned swimlane for tasks without initiative');
});
```

**Test Details:**

1. **should group tasks by initiative in swimlane view**
   - Switch to swimlane view
   - Each initiative should have its own row
   - Swimlane header should show initiative name
   - Progress bar should show completed/total count

2. **should collapse/expand swimlanes**
   - Click collapse button on swimlane header
   - Swimlane content should hide
   - Click again to expand
   - Content should show

3. **should persist collapsed state in localStorage**
   - Collapse a swimlane
   - Reload page
   - Swimlane should still be collapsed
   - localStorage key: `orc-collapsed-swimlanes`

4. **should show Unassigned swimlane for tasks without initiative**
   - Switch to swimlane view
   - "Unassigned" swimlane should be visible (if any tasks lack initiative_id)
   - Should contain tasks with no initiative
   - Should be collapsible like other swimlanes

## Component Changes Required

### 1. Add data-testid attributes

**Board page (`+page.svelte`):**
```html
<div class="board-page" data-testid="board-page">
```

**Column component:**
```html
<div class="column" data-testid="column-{column.id}">
  <span class="count" data-testid="column-count">
```

**TaskCard component:**
```html
<div class="task-card" data-testid="task-card-{task.id}">
```

**ViewModeDropdown component:**
```html
<button data-testid="view-mode-toggle">
```

**Swimlane component:**
```html
<div class="swimlane" data-testid="swimlane-{initiative.id || 'unassigned'}">
  <button data-testid="swimlane-toggle">
```

### 2. Ensure ARIA compliance (already present)

The Column component already has:
```html
<div role="region" aria-label="{column.title} column">
```

## Success Criteria

### Code Must Be Written
- [ ] `web/e2e/board.spec.ts` with 18 tests
- [ ] `data-testid` attributes added to 6 components:
  - [ ] `+page.svelte` (board page)
  - [ ] `Column.svelte`
  - [ ] `TaskCard.svelte`
  - [ ] `ViewModeDropdown.svelte` (or equivalent)
  - [ ] `Swimlane.svelte`
  - [ ] `Board.svelte` (if needed)

### Tests Must Pass
- [ ] All 18 tests pass on current Svelte implementation
- [ ] Tests run in CI without flakiness (3 consecutive passes)
- [ ] `npm run e2e -- --grep board` exits 0

### E2E Scenarios Must Work
- [ ] Board page loads with all 6 columns visible
- [ ] View mode toggle switches between flat and swimlane
- [ ] Drag-drop moves tasks between columns with confirmation
- [ ] Swimlanes collapse/expand and persist state

### Documentation Must Exist
- [ ] Test file header documents selector strategy
- [ ] Each test has clear assertion comments
- [ ] README or test file explains framework-agnostic approach

## Testing Strategy

### Unit Tests
Not applicable - this task creates E2E tests, not units.

### Integration Tests
Not applicable - covered by E2E tests.

### E2E Tests
**This task creates the E2E tests.** They should:

1. Use Playwright's built-in assertions (`expect`)
2. Follow existing patterns from `sidebar.spec.ts`:
   - `await page.waitForLoadState('networkidle')`
   - `await expect(element).toBeVisible({ timeout: 5000 })`
   - `.catch(() => false)` for optional element checks

3. Handle test isolation:
   - Each test should not depend on previous test state
   - Use `test.beforeEach` for common setup
   - Clean up localStorage if modified

4. Network considerations:
   - Some tests may need to wait for API responses
   - Use `page.waitForResponse()` for drag-drop confirmation
   - Set reasonable timeouts (5000ms for UI, 10000ms for API)

## Implementation Notes

### Drag-Drop Testing with Playwright

Playwright provides `page.dragAndDrop()` for drag-drop testing:

```typescript
await page.dragAndDrop(
  '[data-testid="task-card-TASK-001"]',
  '[data-testid="column-implement"]'
);
```

However, the Board uses HTML5 drag-drop API which requires:
1. Fire `dragstart` event on source
2. Fire `dragenter` on target
3. Fire `drop` on target
4. Fire `dragend` on source

May need to use `page.dispatchEvent()` for precise control.

### LocalStorage Testing

Clear localStorage before tests that check persistence:
```typescript
await page.evaluate(() => localStorage.clear());
await page.reload();
```

### Flakiness Prevention

1. Always wait for network idle before assertions
2. Use explicit waits for animations (column highlight)
3. Avoid timing-dependent assertions
4. Use `.toBeVisible()` over `.toHaveCount()` for presence checks

## Reference Materials

- Screenshots: `.orc/initiatives/INIT-005/board-flat-populated.png`, `board-swimlane.png`
- Existing tests: `web/e2e/sidebar.spec.ts` (patterns), `tasks.spec.ts` (task interactions)
- Components: `web/src/lib/components/kanban/` directory
