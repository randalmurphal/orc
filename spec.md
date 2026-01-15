# Specification: Migrate TaskCard and board components to Button primitive

## Problem Statement

Replace all raw `<button>` elements in board components (TaskCard, Board, Column, QueuedColumn, Swimlane) with the unified `Button` primitive to ensure consistent styling, accessibility, and maintainability across the Kanban board UI.

## Success Criteria

- [ ] All 12 TaskCard.tsx buttons migrated to Button component
- [ ] All 4 Board.tsx modal buttons migrated to Button component
- [ ] Column.tsx verified - no raw buttons present (no changes needed)
- [ ] QueuedColumn.tsx backlog toggle (1 button) migrated to Button component
- [ ] Swimlane.tsx collapse toggle (1 button) migrated to Button component
- [ ] All icon-only buttons have `aria-label` attributes
- [ ] Existing CSS classes preserved via `className` prop for backwards compatibility
- [ ] Visual appearance matches current implementation exactly
- [ ] E2E tests pass without selector modifications

## Testing Requirements

- [ ] Unit: `npm run test` passes (frontend tests)
- [ ] E2E: `bunx playwright test board.spec.ts` passes with no changes to test selectors
- [ ] Visual: `bunx playwright test --project=visual` passes (board snapshots match baselines)
- [ ] Manual: Verify all button interactions work (run, pause, resume, finalize, menu items)

## Scope

### In Scope

**TaskCard.tsx (12 buttons):**
| Line | Current | Migration |
|------|---------|-----------|
| 411-418 | Initiative badge `<button>` | `<Button variant="ghost" size="sm" className="initiative-badge">` |
| 425-441 | Run action `.action-btn.run` | `<Button variant="success" size="sm" iconOnly className="action-btn run" aria-label="Run task">` |
| 444-461 | Pause action `.action-btn.pause` | `<Button variant="secondary" size="sm" iconOnly className="action-btn pause" aria-label="Pause task">` |
| 463-480 | Resume action `.action-btn.resume` | `<Button variant="primary" size="sm" iconOnly className="action-btn resume" aria-label="Resume task">` |
| 482-508 | Finalize action `.action-btn.finalize` | `<Button variant="primary" size="sm" iconOnly loading={finalizeLoading} className="action-btn finalize" aria-label="Finalize and merge">` |
| 513-531 | More menu trigger `.action-btn.more` | `<Button variant="ghost" size="sm" iconOnly className="action-btn more" aria-label="Quick actions">` |
| 551-558 | Queue: Active `.menu-item` | `<Button variant="ghost" size="sm" className="menu-item ...">` |
| 559-566 | Queue: Backlog `.menu-item` | `<Button variant="ghost" size="sm" className="menu-item ...">` |
| 574-586 | Priority: Critical `.menu-item` | `<Button variant="ghost" size="sm" className="menu-item ...">` |
| 587-600 | Priority: High `.menu-item` | `<Button variant="ghost" size="sm" className="menu-item ...">` |
| 601-613 | Priority: Normal `.menu-item` | `<Button variant="ghost" size="sm" className="menu-item ...">` |
| 614-625 | Priority: Low `.menu-item` | `<Button variant="ghost" size="sm" className="menu-item ...">` |

**Board.tsx (4 buttons):**
| Line | Current | Migration |
|------|---------|-----------|
| 469-475 | Escalate Cancel `.btn.btn-secondary` | `<Button variant="secondary">Cancel</Button>` |
| 476-483 | Escalate Confirm `.btn.btn-primary` | `<Button variant="primary" disabled={...}>` |
| 503-509 | Initiative Change Cancel `.btn.btn-secondary` | `<Button variant="secondary">Cancel</Button>` |
| 510-517 | Initiative Change Confirm `.btn.btn-primary` | `<Button variant="primary" disabled={...}>` |

**QueuedColumn.tsx (1 button):**
| Line | Current | Migration |
|------|---------|-----------|
| 145-169 | Backlog toggle `.backlog-toggle` | `<Button variant="ghost" size="sm" className="backlog-toggle">` |

**Swimlane.tsx (1 button):**
| Line | Current | Migration |
|------|---------|-----------|
| 96-126 | Collapse toggle `.swimlane-header` | `<Button variant="ghost" size="sm" className="swimlane-header">` |

### Out of Scope

- Creating new E2E tests (existing tests must pass as-is)
- Refactoring CSS (preserve existing class names and styles)
- Changing button behavior or event handling
- Modifying the Button primitive itself
- Buttons in other components outside board folder

## Technical Approach

### CSS Class Preservation Strategy

Keep existing CSS classes as additional `className` for E2E selector compatibility:

```tsx
// Before
<button className="action-btn run" onClick={...} disabled={actionLoading} title="Run task">

// After
<Button
  variant="success"
  size="sm"
  iconOnly
  className="action-btn run"
  onClick={...}
  disabled={actionLoading}
  aria-label="Run task"
>
```

The Button component already concatenates its base classes (`btn btn-success btn-sm btn-icon-only`) with the provided `className`, so both selectors will work:
- New: `.btn-success`
- Legacy: `.action-btn.run`

### Files to Modify

1. **web/src/components/board/TaskCard.tsx**
   - Import `Button` from `@/components/ui/Button`
   - Replace 12 `<button>` elements with `<Button>` components
   - Add `aria-label` to all icon-only buttons
   - Preserve existing click handlers and disabled states

2. **web/src/components/board/Board.tsx**
   - Import `Button` from `@/components/ui/Button`
   - Replace 4 modal buttons with `<Button>` components
   - Remove inline `.btn` classes (Button provides these)

3. **web/src/components/board/QueuedColumn.tsx**
   - Import `Button` from `@/components/ui/Button`
   - Replace backlog toggle with `<Button>`
   - Preserve ARIA attributes

4. **web/src/components/board/Swimlane.tsx**
   - Import `Button` from `@/components/ui/Button`
   - Replace swimlane header toggle with `<Button>`
   - Preserve ARIA attributes

5. **web/src/components/board/Column.tsx** (no changes needed)
   - Verified: No raw `<button>` elements present

### CSS Adjustments

Minor CSS overrides may be needed in component CSS files to ensure Button component styles match current appearance:

- TaskCard.css: Override Button base styles for `.action-btn` sizing (28px square)
- TaskCard.css: Override `.menu-item` styles for full-width display
- QueuedColumn.css: Override `.backlog-toggle` layout
- Swimlane.css: Override `.swimlane-header` layout

## Refactor Analysis

### Before Pattern (raw buttons)
```tsx
<button
  className="action-btn run"
  onClick={(e) => handleAction('run', e)}
  disabled={actionLoading}
  title="Run task"
>
  <svg>...</svg>
</button>
```

### After Pattern (Button primitive)
```tsx
<Button
  variant="success"
  size="sm"
  iconOnly
  className="action-btn run"
  onClick={(e) => handleAction('run', e)}
  disabled={actionLoading}
  aria-label="Run task"
>
  <svg>...</svg>
</Button>
```

### Risk Assessment

| Risk | Mitigation |
|------|------------|
| Visual regression | Run visual regression tests before/after, compare snapshots |
| E2E selector breakage | Preserve all existing CSS classes via `className` prop |
| Accessibility regression | Add `aria-label` to all icon-only buttons (improvement) |
| Event handler changes | Button uses `...props` spread, no handler changes needed |
| Focus ring differences | Button has built-in focus-visible styling; verify matches |

### E2E Selector Analysis

The board.spec.ts tests use these selectors that must continue to work:

| Selector | Component | Status |
|----------|-----------|--------|
| `.task-card` | TaskCard | Preserved (article element) |
| `.task-id` | TaskCard | Preserved (span element) |
| `.column-header` | Column/QueuedColumn | Preserved |
| `.swimlane-header` | Swimlane | Preserved via className |
| `.backlog-toggle` | QueuedColumn | Preserved via className |
| `.action-btn` | TaskCard | Preserved via className |
| `.menu-item` | TaskCard | Preserved via className |

No E2E test selector changes required.
