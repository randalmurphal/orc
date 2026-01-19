# Specification: Create RightPanel component (collapsible 300px panel)

## Problem Statement

The application needs a collapsible right panel (300px width) to display contextual information like blocked tasks, pending decisions, files changed, and completed tasks. This panel must integrate with the AppShell layout and support smooth slide in/out animations.

## Success Criteria

- [ ] RightPanel component created at `web/src/components/layout/RightPanel.tsx`
- [ ] RightPanel.css created at `web/src/components/layout/RightPanel.css`
- [ ] Panel width: 300px fixed when open, 0px when closed
- [ ] Background: `var(--bg-elevated)`, border-left: `1px solid var(--border)`
- [ ] Scrollable content area with custom scrollbar matching design system
- [ ] `RightPanel.Section` compound component: collapsible section with header
- [ ] `RightPanel.Header` compound component: section title with icon and count badge
- [ ] Animation: slide in/out with `0.2s` CSS transition (matches `--duration-normal`)
- [ ] Props interface: `isOpen: boolean`, `onClose: () => void`, `children: ReactNode`
- [ ] Content not rendered when closed (conditional rendering for performance)
- [ ] Scroll position preserved when closing/reopening panel
- [ ] Touch gesture support: swipe left to close on mobile (touch event handling)
- [ ] `npm run typecheck` exits with code 0 (no TypeScript errors)
- [ ] Component exports from layout index file

## Testing Requirements

- [ ] Unit test: RightPanel renders children when open
- [ ] Unit test: RightPanel does not render children when closed
- [ ] Unit test: RightPanel.Section toggles collapsed state on header click
- [ ] Unit test: RightPanel.Header displays icon, title, and count badge
- [ ] Unit test: onClose callback fires on close button click
- [ ] Unit test: Touch swipe gesture triggers onClose (mock touch events)
- [ ] Visual test: Panel animates smoothly on open/close (manual verification)

## Scope

### In Scope

- RightPanel container component with isOpen/onClose props
- RightPanel.Section collapsible sections (collapsed state toggle)
- RightPanel.Header with icon (colored background), title, count badge, chevron
- CSS transitions for slide animation (0.2s)
- Custom scrollbar styling matching `.panel-scroll` from reference
- Touch swipe-to-close gesture handler
- Scroll position preservation via ref
- TypeScript types and exports

### Out of Scope

- Specific section content components (BlockedItem, DecisionItem, FileItem, etc.)
- Integration with AppShell (separate task)
- Keyboard shortcuts for panel toggle
- Drag-to-resize panel width
- Multiple panel instances

## Technical Approach

### Component Architecture

```tsx
// Compound component pattern
<RightPanel isOpen={true} onClose={handleClose}>
  <RightPanel.Section defaultOpen={true}>
    <RightPanel.Header
      icon="alert-circle"
      iconColor="orange"
      title="Blocked"
      count={1}
    />
    {/* Section content */}
  </RightPanel.Section>
</RightPanel>
```

### Key Implementation Details

1. **Container**: Outer wrapper with `width` and `opacity` transitions, conditional child rendering
2. **Scroll container**: Inner scrollable area storing scroll position in ref
3. **Section**: Uses React state for collapsed toggle, renders header + body
4. **Header**: Flex layout with icon box, title, badge, chevron (rotates when collapsed)
5. **Touch handling**: `touchstart`/`touchmove`/`touchend` events tracking swipe direction

### Files to Create/Modify

- `web/src/components/layout/RightPanel.tsx`: Main component with Section and Header
- `web/src/components/layout/RightPanel.css`: All panel styles
- `web/src/components/layout/index.ts`: Add RightPanel export
- `web/src/components/layout/RightPanel.test.tsx`: Unit tests

### CSS Structure

```css
.right-panel                     /* Container: 300px, bg-elevated, border-left */
.right-panel--closed             /* width: 0, opacity: 0, pointer-events: none */
.right-panel__scroll             /* Scrollable area with custom scrollbar */
.panel-section                   /* Section container with border-bottom */
.panel-section--collapsed        /* Hides body, rotates chevron */
.panel-header                    /* Clickable header row */
.panel-title                     /* Icon + title container */
.panel-title__icon               /* 18x18 icon with colored background */
.panel-title__icon--{color}      /* purple, orange, amber, green, blue, cyan */
.panel-badge                     /* Count badge with pill shape */
.panel-badge--{color}            /* Matches icon colors */
.panel-body                      /* Content area with padding */
```

### Animation

- Transition: `width 0.2s ease-out, opacity 0.2s ease-out`
- Matches `--duration-normal` (200ms) from tokens.css
- Uses `pointer-events: none` when closed to prevent interaction during animation

### Touch Gesture

- Track horizontal swipe with 50px threshold
- Swipe left (negative deltaX) triggers onClose
- Ignore vertical scrolling (check deltaY)

## Feature-Specific Analysis

### User Story

As a user viewing the task board, I want a collapsible right panel so that I can see contextual information (blocked tasks, decisions, files changed) without leaving the current view, and hide it when I need more screen space.

### Acceptance Criteria

1. Panel slides in from the right when opened
2. Panel slides out to the right when closed (via button or swipe)
3. Sections can be individually collapsed/expanded by clicking headers
4. Section headers show icon with colored background, title, count badge, and chevron indicator
5. Scrolling within the panel works independently of the main content
6. Scroll position is restored when reopening the panel
7. On touch devices, swiping left closes the panel
8. Panel does not impact performance when closed (no hidden DOM rendering)
