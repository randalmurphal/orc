# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Phase 2: React - Task List page

**Description**: ## Purpose
Port the Task List page with filtering and search.

## Components to Port

### TaskList page
- Filter bar (initiative, dependency status dropdowns)
- Search input with debounce
- Task cards in list layout
- Keyboard navigation (j/k)

### Filter Components
- InitiativeDropdown.svelte -> InitiativeDropdown.tsx
- DependencyDropdown.svelte -> DependencyDropdown.tsx

### Task Card (list variant)
- Compact card for list view
- Same info as board card but different layout

## Filtering Logic
- Filter by initiative (including 'Unassigned')
- Filter by dependency status (all/blocked/ready/none)
- Search by title/ID
- Filters combine (AND logic)
- Persist to URL params

## Success Criteria
- All filters work correctly
- Search debounces properly
- Keyboard nav works (j/k/Enter)
- URL params sync with filters



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

All tests are now passing. Here's a summary of what was implemented for **Phase 2: React - Task List page**:

## Components Created

**Filter Components** (`src/components/filters/`):
- **InitiativeDropdown.tsx** - Dropdown to filter tasks by initiative (All, Unassigned, specific initiatives) with task counts, sorted by status (active first)
- **DependencyDropdown.tsx** - Dropdown to filter by dependency status (All, Ready, Blocked, No dependencies) with counts

**Task Components** (`src/components/tasks/`):
- **TaskCard.tsx** - List variant task card showing status orb, ID, weight badge, blocked badge, initiative badge, title, phase, relative time, and action buttons (Run/Pause/Resume/Delete)

**Pages** (`src/pages/`):
- **TaskList.tsx** - Full task list page with:
  - Status filter tabs (All, Active, Completed, Failed) with counts
  - Search input
  - Initiative and dependency dropdowns  
  - Weight and sort dropdowns
  - New task button
  - Error banner with dismissal
  - Initiative filter banner with clear option
  - Keyboard navigation hints
  - Loading, empty, and no-match states
  - Keyboard navigation via `useTaskListShortcuts` hook (j/k to navigate, Enter to open, r to run, p to pause, d to delete)

## Test Fixes Applied
- Switched from `BrowserRouter` to `MemoryRouter` in test files (TaskCard, InitiativeDropdown tests)
- Made test queries more specific to avoid multiple-element conflicts:
  - Status tabs using `.status-tab` class selector
  - Initiative banner using `.initiative-banner` class selector
  - Unassigned count using element scope
- Fixed "no project selected" test to also clear tasks

All **483 tests pass** across the 24 test files.

---
Tokens: 8128776 input, 43739 output, 228231 cache_creation, 7886198 cache_read
Complete: false
Blocked: false
