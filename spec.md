# Specification: Fix: View mode dropdown disabled on Board page - can't switch to swimlane view

## Problem Statement
The View Mode dropdown on the Board page is disabled and users cannot switch to swimlane view. This occurs because the initiative filter state persists in localStorage, and when the filter is active (even "Unassigned"), the swimlane toggle is intentionally disabled per design. However, the current behavior may be unexpected to users who don't realize a filter is active from a previous session.

## Bug Analysis

### Reproduction Steps
1. Navigate to the Board page and select an initiative filter (including "Unassigned")
2. Navigate away from the page or close the browser
3. Return to `/board` (without any URL parameters)
4. Observe: View Mode dropdown shows "Flat" and is disabled

### Current Behavior
- The initiative filter persists in localStorage (`orc_current_initiative_id`)
- On page load, `initializeFromUrl()` restores the filter from localStorage
- `swimlaneDisabled = currentInitiativeId !== null` evaluates to `true`
- The ViewModeDropdown receives `disabled={true}`
- The dropdown cannot be clicked/opened

### Expected Behavior
Two possible interpretations:
1. **If filter IS active**: Dropdown should be disabled (current design) BUT this should be clearly communicated to the user
2. **If no explicit filter**: Dropdown should be enabled, allowing swimlane view

The bug report suggests scenario #2 - user expects no filter when URL has no `initiative` param.

### Root Cause
The root cause is a **design ambiguity** between URL-based filtering and localStorage persistence:
- **Design intent**: localStorage persists filter selection for convenience
- **User expectation**: Clean URL = no filter active
- **Result**: Mismatch causes confusion

The code at `Board.tsx:84` treats any non-null `currentInitiativeId` as "filter active":
```typescript
const swimlaneDisabled = currentInitiativeId !== null;
```

This includes values restored from localStorage, not just explicit user actions.

## Success Criteria
- [ ] View Mode dropdown is enabled when no initiative filter is explicitly active
- [ ] View Mode dropdown is disabled only when user has actively selected an initiative filter in the current session OR URL param is present
- [ ] Swimlane view is accessible when filter is not active
- [ ] Initiative filter persistence behavior is clearly documented
- [ ] E2E test validates dropdown enabled state on clean page load

## Testing Requirements
- [ ] Unit test: `initiativeStore.initializeFromUrl()` clears stale filters correctly
- [ ] E2E test: View mode dropdown enabled on fresh `/board` navigation (after clearing localStorage)
- [ ] E2E test: View mode dropdown disabled when initiative filter explicitly selected
- [ ] E2E test: Swimlane view accessible and functional when dropdown enabled

## Scope

### In Scope
- Fix the disabled state logic for ViewModeDropdown
- Ensure clean URL navigation results in enabled dropdown
- Maintain intentional disable when initiative filter IS active

### Out of Scope
- Changing localStorage persistence behavior (filter should still persist for convenience)
- Modifying initiative filter dropdown behavior
- UI changes to the ViewModeDropdown component itself
- Changes to swimlane view functionality

## Technical Approach

The fix should distinguish between:
1. **Active filter from user action**: Keep dropdown disabled
2. **No filter / Clean navigation**: Enable dropdown

### Option A: URL-based approach (Recommended)
Only disable swimlane when URL has initiative param, not just when localStorage has a value:

```typescript
// In Board.tsx
const [searchParams] = useSearchParams();
const urlInitiativeFilter = searchParams.get('initiative');
const swimlaneDisabled = urlInitiativeFilter !== null;
```

**Pros**: URL is the source of truth for sharing links; clean URL = clean state
**Cons**: Changes current persistence behavior (filter won't persist across navigation)

### Option B: Session-based flag
Track whether the filter was explicitly set in current session:

```typescript
// In initiativeStore.ts
filterSetThisSession: boolean; // true when user clicks initiative dropdown
```

**Pros**: Maintains localStorage persistence while fixing the issue
**Cons**: More state to track; could still confuse users

### Option C: Don't auto-restore filter on Board page
Clear the initiative filter when navigating to Board page without URL param:

```typescript
// In Board.tsx useEffect
useEffect(() => {
    if (!searchParams.get('initiative')) {
        selectInitiative(null);
    }
}, []);
```

**Pros**: Simple fix; clean URL = clean state
**Cons**: Breaks current persistence design; filter doesn't persist when navigating around

### Recommended Approach: Option A
The URL should be the source of truth for the initiative filter. If user wants to maintain the filter, the URL param will be present. This aligns with:
- CLAUDE.md: "URL param (`?initiative=xxx`) takes precedence over localStorage"
- Browser back/forward navigation expectations
- Shareable link expectations

### Files to Modify
- `web/src/pages/Board.tsx`: Change `swimlaneDisabled` logic to use URL param instead of store value
- `web/e2e/board.spec.ts`: Add test for dropdown enabled on clean navigation

## Verification
After the fix:
1. Navigate to `/board` with no URL params and clear localStorage
2. View Mode dropdown should be enabled and show "Flat"
3. Click dropdown, select "By Initiative"
4. Swimlane view renders correctly
5. Select an initiative filter from InitiativeDropdown
6. View Mode dropdown becomes disabled
7. Clear the initiative filter
8. View Mode dropdown becomes enabled again
