# Specification: Create AppShell component with grid layout

## Problem Statement

The application needs a new AppShell component that provides the main CSS Grid layout structure for the redesigned UI. This component replaces the existing AppLayout.tsx with a grid-based approach matching the mockup in example_ui/board.html, coordinating the IconNav (56px), TopBar (48px), main content area, and optional RightPanel (300px).

## Success Criteria

- [ ] File `web/src/components/layout/AppShell.tsx` exists with correct implementation
- [ ] File `web/src/components/layout/AppShell.css` exists with exact CSS values from mockup
- [ ] File `web/src/components/layout/AppShellContext.tsx` exports `AppShellContext` and `useAppShell` hook
- [ ] CSS Grid layout: `grid-template-columns: 56px 1fr` (base), `56px 1fr 300px` (with right panel)
- [ ] CSS Grid layout: `grid-template-rows: 48px 1fr`
- [ ] Full viewport height (`height: 100vh`) with no scroll on shell itself
- [ ] Main content area (`grid-column: 2; grid-row: 2`) scrollable independently with `overflow-y: auto`
- [ ] RightPanel collapsible from 300px to 0px with 0.2s ease transition
- [ ] Renders IconNav in grid column 1, rows 1-2
- [ ] Renders TopBar in grid column 2+, row 1
- [ ] Renders main content via children in grid column 2, row 2
- [ ] RightPanel spans grid column 3, rows 1-2 when visible
- [ ] Main content area has `role="main"` for accessibility
- [ ] Skip link at top of component targeting main content (`#main-content`)
- [ ] Focus moves appropriately when RightPanel opens/closes
- [ ] Keyboard shortcut `Shift+Alt+R` toggles RightPanel
- [ ] RightPanel collapsed state persisted in localStorage (`orc-right-panel-collapsed`)
- [ ] Below 1024px: RightPanel hidden by default
- [ ] Below 768px: IconNav becomes hamburger menu (overlay mode)
- [ ] `npm run typecheck` exits with code 0

## Testing Requirements

- [ ] Unit test: AppShell renders IconNav, TopBar, and children
- [ ] Unit test: AppShell.css grid values match spec (56px, 1fr, 300px, 48px)
- [ ] Unit test: RightPanel toggle changes grid-template-columns class
- [ ] Unit test: useAppShell hook returns context value with isRightPanelOpen, toggleRightPanel, setRightPanelContent
- [ ] Unit test: Keyboard shortcut Shift+Alt+R toggles right panel
- [ ] Unit test: RightPanel state persists to localStorage
- [ ] Unit test: Skip link exists and targets main content
- [ ] Integration test: At viewport <1024px, RightPanel initializes closed
- [ ] Integration test: At viewport <768px, IconNav renders in hamburger mode

## Scope

### In Scope

- AppShell component with CSS Grid layout
- AppShellContext for managing right panel state
- CSS file with exact values from mockup
- Integration with existing IconNav, TopBar, RightPanel components
- Skip link for accessibility
- Keyboard shortcut for panel toggle (Shift+Alt+R)
- localStorage persistence for collapsed state
- Responsive breakpoints at 1024px, 768px, 480px
- Export from layout/index.ts

### Out of Scope

- Modifications to IconNav, TopBar, or RightPanel components (already exist)
- Replacing usages of AppLayout (separate migration task)
- Mobile hamburger menu implementation details (IconNav handles this)
- Content for RightPanel sections (handled by page components)
- Command palette integration (separate task)

## Technical Approach

### Implementation Strategy

1. Create `AppShellContext.tsx` with React context for panel state management
2. Create `AppShell.tsx` implementing the grid layout and coordinating child components
3. Create `AppShell.css` with exact CSS values from the task specification
4. Add exports to `layout/index.ts`

### Files to Create

- `web/src/components/layout/AppShellContext.tsx`: Context provider and `useAppShell` hook for panel state
- `web/src/components/layout/AppShell.tsx`: Main shell component with CSS Grid layout
- `web/src/components/layout/AppShell.css`: Styles matching mockup specifications
- `web/src/components/layout/AppShell.test.tsx`: Unit tests

### Files to Modify

- `web/src/components/layout/index.ts`: Add AppShell exports

### Architecture Notes

**Grid Structure:**
```
+-------+---------------------------+------------+
| Icon  |         TopBar            | RightPanel |
| Nav   +---------------------------+   (opt)    |
| (56px)|      Main Content         |  (300px)   |
|       |        (scroll)           |            |
+-------+---------------------------+------------+
```

**CSS Grid Assignment:**
- IconNav: `grid-column: 1; grid-row: 1 / 3;`
- TopBar: `grid-column: 2 / -1; grid-row: 1;`
- Main: `grid-column: 2; grid-row: 2;`
- RightPanel: `grid-column: 3; grid-row: 1 / 3;`

**Context API:**
```typescript
interface AppShellContextValue {
  isRightPanelOpen: boolean;
  toggleRightPanel: () => void;
  setRightPanelContent: (content: React.ReactNode) => void;
}
```

**State Persistence:**
- localStorage key: `orc-right-panel-collapsed`
- Check viewport width on mount to override stored state at breakpoints

## Feature Analysis

### User Story

As a user navigating the orc application, I want a consistent layout shell so that I can easily access navigation, see contextual information in the right panel, and have my layout preferences remembered across sessions.

### Acceptance Criteria

1. **Layout Renders Correctly**: The three-column grid (IconNav, main, RightPanel) displays correctly at default viewport sizes
2. **Panel Toggle Works**: Clicking the panel toggle button or pressing Shift+Alt+R toggles the right panel visibility with smooth animation
3. **State Persists**: Closing the right panel and refreshing the page keeps it closed
4. **Responsive Behavior**: At tablet widths (<1024px), right panel starts collapsed; at mobile widths (<768px), navigation switches to hamburger mode
5. **Accessibility**: Screen readers can navigate via landmarks, skip link works, focus is managed when panel opens/closes
6. **No Shell Scroll**: The shell itself does not scroll; only the main content area scrolls
7. **Type Safety**: TypeScript compilation succeeds with no errors

### Edge Cases

- Route changes should respect per-route panel preferences (some routes may not show RightPanel)
- Rapid toggle clicks should not break animation
- localStorage unavailable (private browsing): fallback to in-memory state
- Very narrow viewports (<480px): full mobile layout with overlay navigation
