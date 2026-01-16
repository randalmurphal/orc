# Specification: Fix: Date format shows '12/31/1' instead of proper date on board cards

## Problem Statement

Board task cards display dates in a malformed format like "12/31/1" instead of a proper date format like "Dec 31, 2025". This occurs when the relative time formatting falls through to `toLocaleDateString()` without explicit format options, causing inconsistent and sometimes broken date display across the app.

## Success Criteria

- [ ] TaskCard displays dates older than 7 days using `month short, day, year` format (e.g., "Jan 15, 2026")
- [ ] DashboardRecentActivity displays dates older than 7 days using the same format for consistency
- [ ] Date formatting matches the working pattern in TimelineTab.tsx
- [ ] All relative time formatting (just now, Xm ago, Xh ago, Xd ago) continues to work unchanged
- [ ] No hardcoded locale - uses `undefined` to respect user's browser locale
- [ ] Visual regression: dates appear correctly on Board page task cards

## Testing Requirements

- [ ] Unit test: Create a shared date formatting utility with tests covering:
  - Dates within 1 minute show "just now"
  - Dates within 1 hour show "Xm ago"
  - Dates within 24 hours show "Xh ago"
  - Dates within 7 days show "Xd ago"
  - Dates older than 7 days show formatted date with year
  - Invalid date strings handle gracefully (don't crash)
- [ ] E2E test: Board page displays proper date format on task cards
- [ ] Verify TaskCard and DashboardRecentActivity both produce consistent output

## Scope

### In Scope
- Fix `formatDate()` in `TaskCard.tsx` to use explicit date format options
- Fix `formatRelativeTime()` in `DashboardRecentActivity.tsx` to use same format
- Extract shared date formatting utility to `web/src/lib/date.ts`
- Unit tests for the shared utility

### Out of Scope
- Changing other components' date formats (TimelineTab, InitiativeDetail, etc. already work correctly)
- Adding date picker or date input components
- Timezone handling beyond browser default
- Date internationalization beyond locale-aware formatting

## Technical Approach

Create a centralized date formatting utility that matches the working pattern in TimelineTab.tsx. Both TaskCard and DashboardRecentActivity have nearly identical relative time logic that should be consolidated.

### Files to Create
- `web/src/lib/date.ts`: Shared date formatting utilities
- `web/src/lib/date.test.ts`: Unit tests for date formatting

### Files to Modify
- `web/src/components/board/TaskCard.tsx`: Import and use shared `formatRelativeDate()` function
- `web/src/components/dashboard/DashboardRecentActivity.tsx`: Import and use shared `formatRelativeDate()` function

## Bug-Specific Analysis

### Reproduction Steps
1. Open the Board page (`/board`)
2. Look at task cards in any column
3. Find a task with `updated_at` older than 7 days
4. Observe the date shows "12/31/1" instead of "Dec 31, 2025"

### Current Behavior
- Relative dates (within 7 days) work: "just now", "5m ago", "2h ago", "3d ago"
- Dates older than 7 days show malformed format: "12/31/1"
- The truncated year suggests `toLocaleDateString()` without options produces locale-dependent output that omits or truncates the year

### Expected Behavior
- Relative dates continue working unchanged
- Dates older than 7 days show: "Dec 31, 2025" (or locale equivalent with full year)
- Consistent formatting across TaskCard and DashboardRecentActivity
- Match the format used in TimelineTab which correctly shows "Jan 15, 2026"

### Root Cause
In `TaskCard.tsx:56` and `DashboardRecentActivity.tsx:27`:
```typescript
return date.toLocaleDateString();
```

This call uses no format options. The browser's locale settings determine the output, which can vary and may truncate the year. The working implementations in `TimelineTab.tsx:269-273` use explicit options:
```typescript
return new Date(dateStr).toLocaleDateString(undefined, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
});
```

### Verification
1. Open Board page after fix
2. Task cards with old dates display format like "Dec 31, 2025"
3. Relative dates still show "Xm/Xh/Xd ago" for recent tasks
4. Dashboard Recent Activity section shows consistent formatting
5. Unit tests pass for all date formatting scenarios
