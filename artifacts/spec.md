# Specification: Add Radix Tooltip for consistent tooltips

## Problem Statement
The web UI uses native HTML `title` attributes for tooltips, which have inconsistent appearance across browsers, limited styling options, and poor accessibility. Replace with Radix Tooltip primitive for consistent, accessible, and visually cohesive tooltips throughout the application.

## Success Criteria
- [ ] Tooltip wrapper component created at `web/src/components/ui/Tooltip.tsx`
- [ ] TooltipProvider wraps App in `main.tsx`
- [ ] Tooltips use design system tokens (bg-elevated, text-primary, border-subtle, etc.)
- [ ] Tooltip animations use existing Radix animation patterns from `index.css`
- [ ] All `title` attributes in target components replaced with Tooltip component
- [ ] Keyboard accessibility works (tooltip shows on focus, hides on blur)
- [ ] Escape key dismisses tooltip
- [ ] Tooltip respects `prefers-reduced-motion`
- [ ] Arrow styling matches design spec
- [ ] Max-width 300px prevents overflow

## Testing Requirements
- [ ] Unit test: Tooltip renders content on hover
- [ ] Unit test: Tooltip shows on keyboard focus
- [ ] Unit test: Tooltip hides on Escape
- [ ] Unit test: Tooltip positions correctly (top, right, bottom, left)
- [ ] Unit test: Tooltip with long content respects max-width
- [ ] E2E test: Tooltip appears on hover over TaskCard action buttons
- [ ] E2E test: Tooltip shows on Tab focus navigation

## Scope

### In Scope
- Create `Tooltip.tsx` component with TooltipProvider
- Add tooltip CSS to `index.css` (global Radix styles section)
- Update components with `title` attributes:
  - `TaskCard.tsx` (priority badges, initiative badge, action buttons, quick menu)
  - `TaskHeader.tsx` (action buttons, back button, edit/delete buttons)
  - `Modal.tsx` (close button)
  - `DependencySidebar.tsx` (toggle buttons, add/remove buttons)
  - `Header.tsx` (project switcher, command palette buttons)
  - `Sidebar.tsx` (nav items when collapsed, initiative links, environment link)
  - `DependencyGraph.tsx` (toolbar buttons)
  - `TranscriptTab.tsx` (expand/collapse, copy, export, auto-scroll buttons)
  - `DashboardStats.tsx` (token card tooltip)
  - `DashboardInitiatives.tsx` (progress bar tooltips)

### Out of Scope
- Tooltips in pages (InitiativeDetail.tsx modals use `title` prop for modal titles, not HTML title attributes)
- Custom tooltip delays per component (use global 300ms/150ms)
- Tooltip theming/variants (single consistent style)
- RTL layout support

## Technical Approach

### 1. Create Tooltip Component
Create `web/src/components/ui/Tooltip.tsx`:
```tsx
import * as TooltipPrimitive from '@radix-ui/react-tooltip';

interface TooltipProps {
  content: React.ReactNode;
  children: React.ReactNode;
  side?: 'top' | 'right' | 'bottom' | 'left';
  align?: 'start' | 'center' | 'end';
  delayDuration?: number;
}

export function Tooltip({ content, children, side = 'top', align = 'center', delayDuration = 300 }) { ... }
export function TooltipProvider({ children }) { ... }
```

### 2. Add CSS to index.css
Add tooltip styles to the existing RADIX UI TRANSITIONS section:
```css
/* Tooltip */
.tooltip-content {
  background: var(--bg-elevated);
  color: var(--text-primary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: var(--space-2) var(--space-3);
  box-shadow: var(--shadow-md);
  font-size: var(--text-sm);
  max-width: 300px;
  z-index: var(--z-tooltip);
}

.tooltip-content[data-state='delayed-open'] {
  animation: tooltip-enter var(--duration-fast) var(--ease-out);
}

.tooltip-content[data-state='closed'] {
  animation: tooltip-exit var(--duration-fast) var(--ease-in);
}

.tooltip-arrow {
  fill: var(--bg-elevated);
}
```

### 3. Wrap App in TooltipProvider
Update `main.tsx` to wrap BrowserRouter content with TooltipProvider.

### 4. Migrate Components
Replace `title="..."` with `<Tooltip content="...">` wrapper pattern.

### Files to Modify
| File | Changes |
|------|---------|
| `web/src/components/ui/Tooltip.tsx` | Create new component |
| `web/src/components/ui/Tooltip.test.tsx` | Create unit tests |
| `web/src/index.css` | Add tooltip CSS |
| `web/src/main.tsx` | Add TooltipProvider |
| `web/src/components/board/TaskCard.tsx` | Replace 7 title attrs |
| `web/src/components/task-detail/TaskHeader.tsx` | Replace 5 title attrs |
| `web/src/components/overlays/Modal.tsx` | Replace 1 title attr |
| `web/src/components/task-detail/DependencySidebar.tsx` | Replace 4 title attrs |
| `web/src/components/layout/Header.tsx` | Replace 2 title attrs |
| `web/src/components/layout/Sidebar.tsx` | Replace 5 title attrs |
| `web/src/components/initiative/DependencyGraph.tsx` | Replace 4 title attrs |
| `web/src/components/task-detail/TranscriptTab.tsx` | Replace 6 title attrs |
| `web/src/components/dashboard/DashboardStats.tsx` | Replace 1 title attr |
| `web/src/components/dashboard/DashboardInitiatives.tsx` | Replace 1 title attr |
| `web/e2e/tooltip.spec.ts` | Create E2E tests |

## Feature Analysis

### User Story
As a user, I want consistent, accessible tooltips so that I can understand UI element purposes without inconsistent browser-native styling.

### Acceptance Criteria
1. Tooltips appear after 300ms hover delay
2. Moving between tooltip triggers reduces delay to 150ms
3. Tooltips disappear instantly on mouse leave
4. Tooltips show on keyboard focus (Tab navigation)
5. Escape dismisses tooltip
6. Tooltips have consistent dark theme styling matching app design
7. Tooltips include arrow pointing to trigger element
8. Long tooltip content wraps at 300px max-width
9. Tooltips respect reduced motion preference (no animation)
10. Tooltips are announced to screen readers via aria-describedby

## Migration Pattern

**Before:**
```tsx
<button title="Run task" onClick={handleRun}>
  <Icon name="play" />
</button>
```

**After:**
```tsx
<Tooltip content="Run task">
  <button onClick={handleRun}>
    <Icon name="play" />
  </button>
</Tooltip>
```

**Note:** Radix Tooltip requires trigger to be a single element that can receive refs. For components using `asChild` pattern (like Button), the trigger works directly. For raw elements, wrap as shown.
