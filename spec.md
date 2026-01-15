# Specification: Migrate task-detail components to Button primitive

## Problem Statement

Task-detail components use raw `<button>` elements with CSS classes (`action-btn`, `icon-btn`, `header-btn`, etc.) for styling. These should be migrated to the unified Button primitive component for consistency, maintainability, and accessibility improvements.

## Success Criteria

- [ ] All raw `<button>` elements in task-detail components replaced with `<Button>` primitive
- [ ] Each Button uses correct variant: primary, secondary, danger, ghost, or success
- [ ] Each Button uses correct size: sm, md, or lg
- [ ] Icon-only buttons use `iconOnly` prop with proper `aria-label`
- [ ] Loading states use `loading` prop instead of manual spinner logic
- [ ] Visual parity maintained (buttons look identical before/after)
- [ ] All existing data-testid and aria attributes preserved
- [ ] `npm run test` passes (unit tests)
- [ ] `bunx playwright test task-detail.spec.ts` passes (E2E tests)
- [ ] `bunx playwright test --project=visual` passes (visual regression)

## Testing Requirements

- [ ] Unit test: `npm run test` passes without changes
- [ ] E2E test: `bunx playwright test task-detail.spec.ts` passes without selector changes
- [ ] Visual regression: `bunx playwright test --project=visual` passes

## Scope

### In Scope

- TaskHeader.tsx (8 buttons)
- ChangesTab.tsx (11 buttons)
- TranscriptTab.tsx (10 buttons)
- CommentsTab.tsx (10 buttons)
- TestResultsTab.tsx (5 buttons)
- AttachmentsTab.tsx (4 buttons)
- DependencySidebar.tsx (6 buttons)
- ExportDropdown.tsx (8 buttons)
- TaskEditModal.tsx (2 buttons)
- diff/DiffFile.tsx (1 button)
- diff/InlineCommentThread.tsx (8 buttons)

### Out of Scope

- CSS refactoring beyond preserving visual parity
- Adding new button variants to Button.tsx
- Modifying Button.tsx component itself
- Non-task-detail components

## Technical Approach

### Migration Strategy

1. Import `Button` component from `@/components/ui/Button`
2. Replace `<button className="...">` with `<Button variant="..." size="...">`
3. Convert icon-only buttons to use `iconOnly` prop
4. Convert loading spinners to use `loading` prop
5. Preserve all existing `onClick`, `disabled`, `title`, `data-testid`, and ARIA attributes

### Button Mapping Rules

| Current CSS Class | Button Props |
|-------------------|--------------|
| `action-btn run` | variant=success, size=md |
| `action-btn pause`, `action-btn resume` | variant=primary, size=md |
| `icon-btn` | variant=ghost, size=md, iconOnly |
| `icon-btn danger` | variant=danger, size=md, iconOnly |
| `back-btn` | variant=ghost, size=sm, iconOnly |
| `header-btn` | variant=ghost, size=sm |
| `expand-btn` | variant=ghost, size=sm |
| `send-to-agent-btn` | variant=primary, size=sm |
| `add-general-btn` | variant=ghost, size=sm |
| `action-btn` (comment actions) | variant=ghost, size=sm |
| `action-btn delete` | variant=danger, size=sm |
| `severity-pill` | variant=ghost, size=sm |
| `cancel-btn`, `ghost` | variant=secondary, size=sm or md |
| `submit-btn`, `primary`, `save-btn` | variant=primary, size=sm or md |
| `delete-btn` | variant=danger, size=sm or md |
| `page-btn` | variant=ghost, size=sm |
| `tab` (TestResultsTab) | variant=ghost, size=sm |
| `file-header` (DiffFile) | variant=ghost, size=sm |
| `toggle-btn` | variant=ghost, size=sm, iconOnly |
| `add-btn` | variant=ghost, size=sm |
| `filter-btn` | variant=ghost, size=sm |
| `add-dep-btn` | variant=ghost, size=sm, iconOnly |
| `remove-dep-btn` | variant=danger, size=sm, iconOnly |
| `task-option` | variant=ghost, size=sm |
| `export-trigger` | variant=ghost, size=sm |
| `export-option` | variant=ghost, size=sm |
| `close-btn` | variant=ghost, size=sm, iconOnly |
| `screenshot-preview`, `image-preview` | Keep as `<button>` (semantic, not action) |
| `lightbox-close` | variant=ghost, size=lg, iconOnly |

### Files to Modify

1. **TaskHeader.tsx** (8 buttons):
   - Loading spinner button → Button with loading
   - Run button → variant=success
   - Pause button → variant=primary
   - Resume button → variant=primary
   - Back button → variant=ghost, iconOnly
   - Edit button → variant=ghost, iconOnly
   - Delete button → variant=danger, iconOnly
   - Cancel (confirm dialog) → variant=secondary
   - Delete (confirm dialog) → variant=danger

2. **ChangesTab.tsx** (11 buttons):
   - Split/Unified toggle (2) → variant=ghost, size=sm
   - Expand all → variant=ghost, size=sm
   - Send to Agent → variant=primary, size=sm (with loading)
   - Add General Comment → variant=ghost, size=sm
   - Severity pills (3) → variant=ghost, size=sm
   - Resolve/Won't Fix/Delete comment actions (3) → variant=ghost/danger, size=sm
   - Cancel/Add Comment form actions (2) → variant=secondary/primary, size=sm

3. **TranscriptTab.tsx** (10 buttons):
   - Expand All/Collapse All (2) → variant=ghost, size=sm
   - Copy/Export (2) → variant=ghost, size=sm
   - Auto-scroll toggle → variant=ghost, size=sm
   - Pagination: First/Prev/Next/Last (4) → variant=ghost, size=sm
   - File header toggle → variant=ghost, size=sm

4. **CommentsTab.tsx** (10 buttons):
   - Close form → variant=ghost, size=sm, iconOnly
   - Cancel/Submit form (2) → variant=secondary/primary, size=sm
   - Add Comment button → variant=ghost, size=sm
   - Retry error button → variant=ghost, size=sm
   - Edit/Delete comment (2) → variant=ghost/danger, size=sm, iconOnly
   - Filter buttons (4) → variant=ghost, size=sm

5. **TestResultsTab.tsx** (5 buttons):
   - Tab buttons (3) → variant=ghost, size=sm
   - Screenshot preview → Keep as `<button>` (semantic)
   - Lightbox close → variant=ghost, size=lg, iconOnly

6. **AttachmentsTab.tsx** (4 buttons):
   - Image preview → Keep as `<button>` (semantic)
   - Delete buttons (2) → variant=danger, size=sm, iconOnly
   - Lightbox close → variant=ghost, size=lg, iconOnly

7. **DependencySidebar.tsx** (6 buttons):
   - Toggle sidebar buttons (2) → variant=ghost, size=sm, iconOnly
   - Add blocker/related (2) → variant=ghost, size=sm, iconOnly
   - Remove dep buttons (2) → variant=danger, size=sm, iconOnly
   - Close modal → variant=ghost, size=sm, iconOnly
   - Task option buttons → variant=ghost, size=sm

8. **ExportDropdown.tsx** (8 buttons):
   - Export trigger → variant=ghost, size=sm
   - Export options (7) → variant=ghost, size=sm

9. **TaskEditModal.tsx** (2 buttons):
   - Cancel → variant=secondary, size=md
   - Save → variant=primary, size=md (with loading)

10. **diff/DiffFile.tsx** (1 button):
    - File header → variant=ghost, size=sm

11. **diff/InlineCommentThread.tsx** (8 buttons):
    - Resolve/Won't Fix/Delete (3) → variant=ghost/danger, size=sm
    - Severity pills (3) → variant=ghost, size=sm
    - Cancel/Submit (2) → variant=secondary/primary, size=sm

## Verified Button Audit (73 total buttons)

| File | Button Count | Notes |
|------|-------------|-------|
| TaskHeader.tsx | 8 | Includes loading spinner |
| ChangesTab.tsx | 11 | Includes severity pills, view toggle |
| TranscriptTab.tsx | 10 | Pagination + header actions |
| CommentsTab.tsx | 10 | Filter tabs + form actions |
| TestResultsTab.tsx | 5 | Tab buttons + lightbox |
| AttachmentsTab.tsx | 4 | Delete + lightbox |
| DependencySidebar.tsx | 6 | Toggle + add/remove deps |
| ExportDropdown.tsx | 8 | Trigger + options |
| TaskEditModal.tsx | 2 | Cancel + Save |
| diff/DiffFile.tsx | 1 | File header |
| diff/InlineCommentThread.tsx | 8 | Actions + form |
| **Total** | **73** | |

## Category-Specific Analysis (Refactor)

### Before Pattern
```tsx
<button className="action-btn run" onClick={handleRun} title="Run task">
  <Icon name="play" size={16} />
  <span>Run</span>
</button>
```

### After Pattern
```tsx
<Button
  variant="success"
  size="md"
  leftIcon={<Icon name="play" size={16} />}
  onClick={handleRun}
  title="Run task"
>
  Run
</Button>
```

### Risk Assessment

**Low Risk:**
- E2E tests use role/aria/semantic selectors, not CSS classes
- Button component preserves all HTML attributes
- Visual regression tests will catch styling issues

**Medium Risk:**
- CSS class-based styling may need adjustment if visual parity not achieved
- Icon-only buttons need explicit `aria-label` for accessibility

**Mitigation:**
- Run E2E tests after each file migration
- Run visual regression tests before final commit
- Preserve existing `title` attributes as `aria-label` for icon-only buttons
