# Specification: Add mobile responsive design with hamburger menu

## Problem Statement

The web UI lacks mobile responsive design. On viewports below 768px, the sidebar remains visible and fixed, content gets squished, and there's no way to toggle navigation visibility on mobile devices.

## Success Criteria

- [ ] Sidebar is hidden by default on viewports < 768px (mobile breakpoint)
- [ ] Hamburger menu button appears in header on mobile viewports
- [ ] Clicking hamburger button opens sidebar as an overlay/drawer
- [ ] Sidebar closes when clicking outside (on the backdrop overlay)
- [ ] Sidebar closes when selecting a navigation link
- [ ] Content area uses full width on mobile (no margin for hidden sidebar)
- [ ] All pages are usable at 375px viewport width (iPhone SE)
- [ ] All pages are usable at 414px viewport width (iPhone Plus)
- [ ] Transition between mobile and desktop is smooth (no jarring layout shifts)
- [ ] Existing desktop behavior preserved (sidebar expand/collapse still works on desktop)

## Testing Requirements

- [ ] Unit test: `uiStore.test.ts` - test `mobileMenuOpen` state toggle, test `closeMobileMenu()` action
- [ ] E2E test: `mobile-responsive.spec.ts` - test hamburger button visibility at 375px viewport
- [ ] E2E test: Verify sidebar opens/closes via hamburger button click
- [ ] E2E test: Verify sidebar closes on backdrop click
- [ ] E2E test: Verify sidebar closes on navigation link click
- [ ] E2E test: Test at 768px breakpoint transition (sidebar visible, no hamburger)
- [ ] Manual test: Verify all pages render correctly at 375px, 414px, 768px widths

## Scope

### In Scope

- Mobile responsive CSS for sidebar, header, and app layout
- Hamburger menu button in header (mobile only)
- Mobile menu open/close state in UI store
- Backdrop overlay when mobile sidebar is open
- CSS breakpoint variables for consistent responsive design
- Sidebar closing behavior (backdrop click, link selection)
- Menu icon addition to Icon component

### Out of Scope

- Touch gestures (swipe to open/close)
- PWA features
- Bottom navigation bar
- Tablet-specific layouts (beyond 768px breakpoint)
- Responsive adjustments to page content (task cards, board columns, etc.)

## Technical Approach

### State Management

Add mobile menu state to `uiStore.ts`:
- `mobileMenuOpen: boolean` - tracks if mobile sidebar is visible
- `openMobileMenu()` / `closeMobileMenu()` / `toggleMobileMenu()` - actions
- Mobile menu state should NOT persist to localStorage (always starts closed)

### Breakpoint Strategy

Use CSS custom property for breakpoint consistency:
```css
:root {
  --breakpoint-mobile: 768px;
}
```

Media query pattern:
```css
@media (max-width: 767px) { /* mobile styles */ }
@media (min-width: 768px) { /* desktop styles */ }
```

### Layout Changes

**AppLayout.tsx/css:**
- On mobile: remove sidebar margin, full width content
- Add backdrop overlay element when mobile menu is open
- Pass `closeMobileMenu` callback to Sidebar

**Header.tsx/css:**
- Add hamburger button (visible only on mobile)
- Position at left side before project button
- Use `menu` icon (3 horizontal lines)

**Sidebar.tsx/css:**
- On mobile: position as fixed overlay, full height, slide-in from left
- Higher z-index than backdrop
- Close on any nav link click (call `closeMobileMenu`)
- On desktop: existing behavior unchanged

### CSS Architecture

1. Add breakpoint variable to `tokens.css`
2. Add mobile media queries to:
   - `AppLayout.css` - layout margins, backdrop
   - `Sidebar.css` - overlay positioning, transitions
   - `Header.css` - hamburger button visibility

### Files to Modify

| File | Changes |
|------|---------|
| `web/src/styles/tokens.css` | Add `--breakpoint-mobile: 768px` |
| `web/src/stores/uiStore.ts` | Add `mobileMenuOpen` state and actions |
| `web/src/stores/uiStore.test.ts` | Add tests for mobile menu state |
| `web/src/components/ui/Icon.tsx` | Add `menu` icon (hamburger) |
| `web/src/components/layout/AppLayout.tsx` | Add backdrop overlay, wire up mobile state |
| `web/src/components/layout/AppLayout.css` | Mobile layout styles, backdrop |
| `web/src/components/layout/Header.tsx` | Add hamburger menu button |
| `web/src/components/layout/Header.css` | Hamburger button styles, mobile adjustments |
| `web/src/components/layout/Sidebar.tsx` | Accept `onClose` prop, close on nav click |
| `web/src/components/layout/Sidebar.css` | Mobile overlay styles, transitions |
| `web/e2e/mobile-responsive.spec.ts` | New E2E test file |

## Feature: User Story & Acceptance Criteria

### User Story

As a mobile user, I want to access the navigation sidebar via a hamburger menu so that I can navigate the application on my phone without the sidebar taking up screen space.

### Acceptance Criteria

1. **Given** I'm viewing the app on a mobile device (< 768px)
   **When** the page loads
   **Then** the sidebar is hidden and a hamburger menu button is visible in the header

2. **Given** I'm on mobile with the sidebar hidden
   **When** I tap the hamburger menu button
   **Then** the sidebar slides in from the left as an overlay with a backdrop behind it

3. **Given** the mobile sidebar is open
   **When** I tap outside the sidebar (on the backdrop)
   **Then** the sidebar closes

4. **Given** the mobile sidebar is open
   **When** I tap a navigation link
   **Then** I navigate to that page AND the sidebar closes

5. **Given** I'm viewing the app on desktop (>= 768px)
   **When** the page loads
   **Then** the sidebar is visible in its normal position (no hamburger menu shown)
