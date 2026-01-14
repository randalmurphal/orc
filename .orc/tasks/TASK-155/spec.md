# Phase 0: Expand E2E tests for Task Detail tabs (15 tests)

## Overview

Comprehensive E2E tests for the Task Detail page tabs. Tests define **BEHAVIOR** (framework-agnostic) to work on both current Svelte and future React implementations.

## Requirements

### Scope
- Create `web/e2e/task-detail.spec.ts` with 15 tests across 5 test groups
- Tests must be deterministic and non-flaky
- Tests must use framework-agnostic selectors (no `.svelte-xyz` classes)
- Tests should validate both UI rendering and API interactions

### Out of Scope
- Test Results tab (requires actual test run - not included in original 15)
- Comments tab (no tests specified in requirements)
- WebSocket streaming tests (covered elsewhere)
- Creating/deleting tasks (covered in tasks.spec.ts)

## Technical Approach

### File Structure
```
web/e2e/
├── task-detail.spec.ts    # NEW - 15 tests
├── tasks.spec.ts          # Existing - basic task CRUD
├── board.spec.ts          # Existing - board interactions
└── ...
```

### Selector Strategy (Priority Order)

| Priority | Method | Example | When to Use |
|----------|--------|---------|-------------|
| 1 | `getByRole()` | `getByRole('tab', { name: 'Timeline' })` | Tabs, buttons, dialogs |
| 2 | `getByText()` | `getByText('Token Usage')` | Headers, labels |
| 3 | ARIA attributes | `[aria-selected="true"]` | Tab state verification |
| 4 | Class selectors | `.timeline-phase` | Structural elements |
| 5 | `data-testid` | `[data-testid="diff-viewer"]` | Fallback only |

### Test Prerequisites

Tests require at least one task with:
- Completed phases (for timeline data)
- Code changes (for diff viewer data)
- Transcript files (for transcript tab)

**Strategy:** Tests navigate to an existing task. If no suitable task exists, skip with informative message.

## Component Breakdown

### Backend (No Changes Required)

Existing API endpoints are sufficient:
- `GET /api/tasks/:id` - Task metadata
- `GET /api/tasks/:id/state` - Phase state and timeline
- `GET /api/tasks/:id/diff?files=true` - Diff stats
- `GET /api/tasks/:id/diff/file/:path` - File hunks
- `GET /api/tasks/:id/transcripts` - Transcript file list
- `GET /api/tasks/:id/attachments` - Attachment list

### Frontend (No Changes Required)

All components already exist:
- `TabNav.svelte` - Tab navigation with `role="tablist"` and `role="tab"`
- `Timeline.svelte` - Phase timeline with stats
- `DiffViewer.svelte` - Diff viewer with split/unified toggle
- `Transcript.svelte` - Transcript history
- `Attachments.svelte` - Attachment gallery

### Test File Implementation

**File:** `web/e2e/task-detail.spec.ts`

```typescript
// Structure outline
test.describe('Task Detail Page', () => {
  // Helper functions
  async function navigateToTaskWithData(page: Page) { ... }
  async function waitForTabContent(page: Page) { ... }

  test.describe('Tab Navigation', () => { ... });  // 4 tests
  test.describe('Timeline Tab', () => { ... });    // 3 tests
  test.describe('Changes Tab', () => { ... });     // 5 tests
  test.describe('Transcript Tab', () => { ... });  // 2 tests
  test.describe('Attachments Tab', () => { ... }); // 1 test
});
```

## Test Specifications

### Group 1: Tab Navigation (4 tests)

#### Test 1.1: should show all tabs
```typescript
test('should show all tabs', async ({ page }) => {
  // Navigate to task detail
  // Verify 6 tabs visible: Timeline, Changes, Transcript, Test Results, Attachments, Comments
  // Use: page.getByRole('tablist'), page.getByRole('tab', { name: 'Timeline' })
});
```
**Pass Criteria:** All 6 tab buttons visible in tablist

#### Test 1.2: should switch tabs when clicked
```typescript
test('should switch tabs when clicked', async ({ page }) => {
  // Click each tab
  // Verify aria-selected changes
  // Verify tab content panel changes
});
```
**Pass Criteria:** Clicking tab sets `aria-selected="true"` and shows corresponding content

#### Test 1.3: should update URL with tab parameter
```typescript
test('should update URL with tab parameter', async ({ page }) => {
  // Click 'Changes' tab
  // Verify URL contains ?tab=changes
  // Use: page.url().includes('tab=changes')
});
```
**Pass Criteria:** URL updates to `?tab={tabId}` on click

#### Test 1.4: should load correct tab from URL query param
```typescript
test('should load correct tab from URL query param', async ({ page }) => {
  // Navigate to /tasks/TASK-XXX?tab=transcript
  // Verify Transcript tab is selected on load
});
```
**Pass Criteria:** Page loads with correct tab pre-selected based on URL

### Group 2: Timeline Tab (3 tests)

#### Test 2.1: should show phase timeline with status indicators
```typescript
test('should show phase timeline with status indicators', async ({ page }) => {
  // Navigate to task with completed phases
  // Verify timeline phases visible
  // Verify status icons (check, spinner, circle) present
  // Use: page.locator('.timeline-phase'), page.locator('.phase-status')
});
```
**Pass Criteria:** Phase boxes visible with correct status icons (completed=check, running=spinner, pending=circle)

#### Test 2.2: should show token usage stats
```typescript
test('should show token usage stats', async ({ page }) => {
  // Verify stats grid visible
  // Verify Input, Output, Cached, Total stats displayed
  // Use: page.getByText('Input'), page.locator('.stats-value')
});
```
**Pass Criteria:** Token usage section shows Input, Output, Cached, Total with numeric values

#### Test 2.3: should show iteration and retry counts
```typescript
test('should show iteration and retry counts', async ({ page }) => {
  // Verify iterations count visible
  // Verify retry count visible (if any)
  // Use: page.getByText(/Iterations/), page.getByText(/Retries/)
});
```
**Pass Criteria:** Iterations count displayed; Retry count shown if > 0

### Group 3: Changes Tab - Diff Viewer (5 tests)

#### Test 3.1: should load and display diff stats
```typescript
test('should load and display diff stats', async ({ page }) => {
  // Click Changes tab
  // Wait for diff to load
  // Verify additions/deletions summary visible
  // Use: page.getByText(/\+\d+/), page.getByText(/-\d+/)
});
```
**Pass Criteria:** Shows `+X -Y` additions/deletions summary

#### Test 3.2: should show file list with additions/deletions counts
```typescript
test('should show file list with additions/deletions counts', async ({ page }) => {
  // Verify file list rendered
  // Each file shows path and +/- counts
  // Use: page.locator('.diff-file'), page.locator('.file-stats')
});
```
**Pass Criteria:** File paths listed with individual file stats

#### Test 3.3: should expand/collapse files
```typescript
test('should expand/collapse files', async ({ page }) => {
  // Click file header to expand
  // Verify hunks/diff content visible
  // Click again to collapse
  // Verify hunks hidden
});
```
**Pass Criteria:** Clicking file toggles hunk visibility; API call made for file content

#### Test 3.4: should toggle between split and unified view
```typescript
test('should toggle between split and unified view', async ({ page }) => {
  // Find view toggle (Split/Unified dropdown or button)
  // Click to switch view
  // Verify diff layout changes
  // Use: page.getByRole('button', { name: /split|unified/i })
});
```
**Pass Criteria:** Toggle changes diff display mode; setting persists

#### Test 3.5: should show line numbers
```typescript
test('should show line numbers', async ({ page }) => {
  // Expand a file
  // Verify line numbers visible in gutter
  // Use: page.locator('.line-number')
});
```
**Pass Criteria:** Line numbers displayed for both old and new content

### Group 4: Transcript Tab (2 tests)

#### Test 4.1: should show transcript history
```typescript
test('should show transcript history', async ({ page }) => {
  // Click Transcript tab
  // Verify transcript file list visible
  // Each entry shows phase name and date
  // Use: page.locator('.transcript-entry, .transcript-file')
});
```
**Pass Criteria:** Transcript entries listed with phase labels

#### Test 4.2: should expand transcript content sections
```typescript
test('should expand transcript content sections', async ({ page }) => {
  // Click on transcript entry
  // Verify content expands
  // Verify sections visible (prompt, response, metadata)
});
```
**Pass Criteria:** Clicking entry shows full transcript content with parseable sections

### Group 5: Attachments Tab (1 test)

#### Test 5.1: should display attachment list with thumbnails
```typescript
test('should display attachment list with thumbnails', async ({ page }) => {
  // Click Attachments tab
  // Verify attachment grid/list visible
  // Images show thumbnails
  // Files show icon + filename
  // Use: page.locator('.attachment-item'), page.locator('img[src*="attachments"]')
});
```
**Pass Criteria:** Attachments displayed; images have thumbnail preview; files show name and icon

## Success Criteria

### Code Requirements
- [ ] Create `web/e2e/task-detail.spec.ts` with all 15 tests
- [ ] Tests use framework-agnostic selectors (no Svelte-specific classes)
- [ ] Tests include proper waiting strategies (avoid flaky tests)
- [ ] Tests have clear failure messages

### Test Results
- [ ] All 15 tests pass on Svelte implementation
- [ ] Tests complete in < 60 seconds total
- [ ] No flaky tests (pass 3 consecutive runs)

### Documentation
- [ ] Test file includes JSDoc comments explaining test groups
- [ ] Helper functions are documented

## Testing Strategy

### Unit Tests
None required - this task creates E2E tests only.

### Integration Tests
None required - E2E tests cover integration scenarios.

### E2E Tests (This Task)
Run with: `cd web && bunx playwright test e2e/task-detail.spec.ts`

| Test Group | Count | Dependencies |
|------------|-------|--------------|
| Tab Navigation | 4 | Any task |
| Timeline Tab | 3 | Task with phases |
| Changes Tab | 5 | Task with code changes |
| Transcript Tab | 2 | Task with transcripts |
| Attachments Tab | 1 | Task with attachments |

### Test Data Requirements

Tests require a task with:
1. At least 2 completed phases (for timeline)
2. File changes in worktree (for diff viewer)
3. Transcript files (for transcript tab)

**Fallback:** If no suitable task exists, tests skip with message.

## Implementation Plan

1. **Create test file structure** with describe blocks and helpers
2. **Implement Tab Navigation tests** (4 tests)
3. **Implement Timeline Tab tests** (3 tests)
4. **Implement Changes Tab tests** (5 tests)
5. **Implement Transcript Tab tests** (2 tests)
6. **Implement Attachments Tab test** (1 test)
7. **Run full test suite** and fix any flaky tests
8. **Validate determinism** with 3 consecutive runs

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| No task with required data | Tests fail/skip | Create test data setup helper or use fixture task |
| Diff API slow to load | Flaky tests | Add explicit waits for network requests |
| Tab switching animation | Timing issues | Wait for `aria-selected` state change |
| Transcript parsing varies | False failures | Test for presence, not exact content |

## Completion Checklist

- [ ] `web/e2e/task-detail.spec.ts` created with 15 tests
- [ ] All tests pass: `bunx playwright test e2e/task-detail.spec.ts`
- [ ] Tests are deterministic (3 consecutive passes)
- [ ] No framework-specific selectors used
- [ ] Test file has clear organization and comments
