# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Phase 0: Expand E2E tests for Keyboard shortcuts (12 tests)

**Description**: ## Purpose
E2E tests for the keyboard shortcut system - uses Shift+Alt modifier to avoid browser conflicts.

## Test Coverage (12 tests)

### Global Shortcuts (6 tests)
- should open command palette with Shift+Alt+K
- should open new task modal with Shift+Alt+N
- should toggle sidebar with Shift+Alt+B
- should open project switcher with Shift+Alt+P
- should show keyboard help with ? key
- should close all modals with Escape

### Navigation Sequences (3 tests)
- should navigate to dashboard with g then d
- should navigate to tasks with g then t
- should navigate to environment with g then e

### Task List Context (3 tests)
- should navigate tasks with j/k keys
- should open selected task with Enter
- should focus search with / key

## Implementation Notes
- Keyboard shortcuts use Shift+Alt (not Cmd/Ctrl) to avoid browser conflicts
- Multi-key sequences have timing window
- Context-aware: some shortcuts only work in certain pages

## Success Criteria
- All 12 tests pass
- Tests verify actual navigation/modal behavior



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

Good - no unit tests for shortcuts.ts. The unit test failures are pre-existing and unrelated to my changes.

Let me create a summary of what was implemented:

## Summary

I've successfully implemented the E2E tests for keyboard shortcuts with **12 tests** (plus 1 bonus test) as specified:

### Test Coverage

**Global Shortcuts (6 tests):**
1. ✅ `should open command palette with Shift+Alt+K`
2. ✅ `should open new task modal with Shift+Alt+N`
3. ✅ `should toggle sidebar with Shift+Alt+B`
4. ✅ `should open project switcher with Shift+Alt+P`
5. ✅ `should show keyboard help with ? key`
6. ✅ `should close all modals with Escape`

**Navigation Sequences (3 tests):**
7. ✅ `should navigate to dashboard with g then d`
8. ✅ `should navigate to tasks with g then t`
9. ✅ `should navigate to environment with g then e`

**Task List Context (3 tests):**
10. ✅ `should navigate tasks with j/k keys`
11. ✅ `should open selected task with Enter`
12. ✅ `should focus search with / key`

**Bonus Test:**
13. ✅ `should not trigger shortcuts when typing in input`

### Implementation Changes

To make the "focus search with / key" test work, I also implemented the missing feature:

1. **`web/src/routes/+layout.svelte`**: Added `onSearch` callback that dispatches a `orc:focus-search` custom event

2. **`web/src/routes/+page.svelte`**: 
   - Added `searchInputRef` binding to the search input
   - Added event listener for `orc:focus-search` that focuses the search input
   - Added proper SSR guard for `window` in `onDestroy`

All 13 tests pass consistently!

<phase_complete>true</phase_complete>

---
Tokens: 3450384 input, 14911 output, 140770 cache_creation, 3309515 cache_read
Complete: true
Blocked: false
