# Specification: Fix: Dashboard initiative progress shows 0/0 while sidebar shows 16/17

## Problem Statement

Dashboard's "Active Initiatives" section shows incorrect progress (0/0) while the Sidebar shows correct progress (16/17). The two components calculate progress using different data sources, causing inconsistent counts.

## Root Cause Analysis

| Component | Data Source | Calculation Method |
|-----------|-------------|-------------------|
| **Sidebar** | Task store (all tasks) | `getInitiativeProgress(tasks)` - counts tasks where `task.initiative_id === initiative.id` |
| **Dashboard** | Initiative API response | `getProgress(initiative)` - counts `initiative.tasks[]` array embedded in initiative object |

The discrepancy occurs because:
1. **Sidebar** correctly counts tasks by checking each task's `initiative_id` field
2. **Dashboard** relies on the `initiative.tasks[]` array, which may be empty if:
   - Tasks were linked via `initiative_id` but never added to `initiative_tasks` join table
   - Legacy tasks created before bidirectional sync was implemented
   - Data inconsistency between `tasks.initiative_id` and `initiative_tasks` table

## Success Criteria

- [ ] Dashboard "Active Initiatives" shows same progress counts as Sidebar for all initiatives
- [ ] Progress calculation uses task store (canonical source) not embedded initiative.tasks array
- [ ] Existing tests pass (`make web-test`)
- [ ] No visual regression in Dashboard layout

## Testing Requirements

- [ ] Unit test: `DashboardInitiatives` renders correct progress from task store
- [ ] Unit test: Progress shows 0/0 when initiative has no tasks in task store
- [ ] Unit test: Component handles initiatives with tasks in both sources consistently
- [ ] E2E test: Verify Dashboard and Sidebar show matching progress counts

## Scope

### In Scope
- Modify `DashboardInitiatives` to use same progress calculation method as Sidebar
- Pass tasks from task store to `DashboardInitiatives` component
- Update Dashboard page to provide tasks to the component

### Out of Scope
- Fixing data consistency between `tasks.initiative_id` and `initiative_tasks` table (separate issue)
- Modifying backend API response format
- Changing Sidebar implementation
- Adding new backend endpoints

## Technical Approach

The fix aligns Dashboard with Sidebar by using the same progress calculation method: counting tasks from the task store by `initiative_id` rather than relying on the embedded `initiative.tasks[]` array.

### Files to Modify

1. **`web/src/components/dashboard/DashboardInitiatives.tsx`**
   - Add `tasks` prop to receive tasks from task store
   - Change `getProgress()` to calculate progress using task store data (count tasks where `task.initiative_id === initiative.id`)
   - Match the logic used in Sidebar's `getInitiativeProgress()`

2. **`web/src/pages/Dashboard.tsx`**
   - Pass `tasks` from task store to `DashboardInitiatives` component
   - Tasks are already available via `useTaskStore((state) => state.tasks)`

### Implementation Details

**Current Dashboard getProgress (problematic):**
```typescript
function getProgress(initiative: Initiative): ProgressInfo {
  const tasks = initiative.tasks || [];  // Uses embedded array - may be empty/stale
  const total = tasks.length;
  // ...
}
```

**Fixed Dashboard getProgress:**
```typescript
function getProgress(initiativeId: string, tasks: Task[]): ProgressInfo {
  const initiativeTasks = tasks.filter(t => t.initiative_id === initiativeId);
  const total = initiativeTasks.length;
  const completed = initiativeTasks.filter(
    t => t.status === 'completed' || t.status === 'finished'
  ).length;
  // ...
}
```

This matches the Sidebar's `getInitiativeProgress()` logic in `initiativeStore.ts:125-148`.

## Bug Analysis

### Reproduction Steps
1. Navigate to Dashboard (`/dashboard`)
2. Observe "Active Initiatives" section showing initiative with "0/0" progress
3. Check Sidebar showing same initiative with correct progress (e.g., "16/17")

### Current Behavior
Dashboard shows "0/0" because `initiative.tasks` array is empty/not populated even though tasks exist with matching `initiative_id`.

### Expected Behavior
Dashboard shows same count as Sidebar (e.g., "16/17") by counting tasks from task store.

### Root Cause
Two different progress calculation methods:
- Sidebar uses task store (correct, canonical source)
- Dashboard uses embedded `initiative.tasks[]` (may be inconsistent)

### Verification
After fix:
1. Navigate to Dashboard
2. Compare initiative progress counts with Sidebar
3. Both should show identical progress for all initiatives
4. Run `make web-test` - all tests pass
