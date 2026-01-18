# Specification: Create IconNav component (56px sidebar)

## Problem Statement

Create a 56px icon-based navigation sidebar to replace the current text-based sidebar, matching the design from example_ui/board.html reference mockup.

## Success Criteria

- [x] IconNav component at web/src/components/layout/IconNav.tsx
- [x] Fixed 56px width, full height, var(--bg-elevated) background
- [x] Logo section at top: 32px gradient square (purple to pink), "O" letter, glow shadow
- [x] Nav items: icon (18px) + label (8px) stacked vertically, 6px border-radius
- [x] States: default (var(--text-muted)), hover (var(--bg-surface)), active (var(--primary-dim) + var(--primary-bright))
- [x] Divider line between main nav and settings section
- [x] Help icon at bottom
- [x] Navigation items: Board -> /board, Initiatives -> /initiatives, Stats -> /stats, [divider], Agents -> /agents, Settings -> /settings, [bottom] Help -> /help
- [x] Uses react-router NavLink for active state
- [x] Tooltip on hover showing full label (via Radix Tooltip)
- [x] npm run typecheck exits 0
- [x] Active state works with nested routes (e.g., /settings/*)
- [x] Keyboard navigation between items (Tab moves between items)
- [x] Icons have aria-label for accessibility
- [x] Focus outline: 2px solid var(--primary), outline-offset: 2px

## Testing Requirements

- [x] Unit test: Renders all nav items with correct icons
- [x] Unit test: Logo mark displays "O" with correct class
- [x] Unit test: Divider present between main and secondary nav
- [x] Unit test: Active state applied when route matches
- [x] Unit test: Active state works with nested routes (/settings/*)
- [x] Unit test: role="navigation" on nav element
- [x] Unit test: aria-label="Main navigation" on nav element  
- [x] Unit test: aria-label with full description on each nav item
- [x] Unit test: aria-current="page" on active NavLink
- [x] Unit test: All nav items keyboard navigable (not tabindex -1)
- [x] Unit test: Tooltip wrappers present on all nav items (data-state attribute)
- [x] Unit test: Custom className applies correctly

## Scope

### In Scope

- IconNav component implementation
- CSS styling matching mockup exact values
- Accessibility attributes (role, aria-label, focus states)
- React Router integration with NavLink
- Tooltip integration via existing Tooltip component
- Active state detection including nested routes
- Unit tests for component behavior

### Out of Scope

- Responsive behavior (collapse below 768px, hide below 480px) - deferred to layout shell task
- Skip link target - handled at AppLayout level
- Route prefetch on hover - not a React Router 7 built-in feature
- Hamburger menu - part of TopBar component

## Technical Approach

The IconNav component is implemented using:

1. **Component Structure** (IconNav.tsx):
   - Configuration arrays for main, secondary, and bottom nav items
   - Memoized NavItem sub-component for performance
   - checkIsActive helper for nested route matching
   - Integration with Icon and Tooltip components from @/components/ui

2. **Styling** (IconNav.css):
   - CSS custom properties from tokens.css for colors
   - BEM naming convention (.icon-nav, .icon-nav__item, etc.)
   - Focus-visible for accessible keyboard navigation
   - Reduced motion media query for accessibility

3. **Accessibility**:
   - role="navigation" with aria-label="Main navigation"
   - aria-label on each NavLink with full description
   - NavLink automatically sets aria-current="page"
   - Focus outline visible on keyboard navigation

### Files Created

- `web/src/components/layout/IconNav.tsx`: Main component
- `web/src/components/layout/IconNav.css`: Component styles
- `web/src/components/layout/IconNav.test.tsx`: Unit tests

### CSS Values (from mockup)

| Property | Value |
|----------|-------|
| Nav width | 56px |
| Logo size | 32px |
| Logo gradient | linear-gradient(135deg, var(--primary), var(--primary-gradient-end)) |
| Logo shadow | 0 4px 12px var(--primary-glow) |
| Nav padding | 10px 0 |
| Nav item padding | 8px 4px |
| Nav item gap | 2px between items |
| Nav item border-radius | 6px |
| Icon size | 18px |
| Label font-size | 8px |
| Divider margin | 6px 8px |
| Focus outline | 2px solid var(--primary), offset 2px |

## Feature: User Story

As a user navigating the ORC application, I want a compact icon-based sidebar so that I have more screen space for main content while maintaining quick access to all sections.

## Feature: Acceptance Criteria

1. The sidebar is exactly 56px wide and spans the full viewport height
2. The logo "O" is displayed at the top with a purple-to-pink gradient and glow effect
3. Navigation items show an icon (18px) stacked above a label (8px)
4. Hovering a nav item shows a surface background and secondary text color
5. The active nav item has primary-dim background and primary-bright text with a 2px left accent
6. A horizontal divider separates the main navigation (Board, Initiatives, Stats) from secondary navigation (Agents, Settings)
7. The Help item is anchored to the bottom of the sidebar
8. Clicking a nav item navigates to the corresponding route
9. Hovering a nav item shows a tooltip with the full description
10. Tab key moves focus between nav items with visible focus outline
11. Screen readers announce the navigation and item purposes correctly
