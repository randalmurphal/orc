# Specification: Investigate Click Actions Timing Out on Some Elements

## Problem Statement

Click actions on navigation elements (Dashboard link in sidebar, Prompts link in Environment nav) time out during E2E testing. Elements resolve but the click action times out waiting for elements to become "visible, enabled and stable." This indicates CSS transitions/animations are preventing Playwright from determining element stability.

## Bug Analysis

### Reproduction Steps
1. Run E2E tests with Playwright MCP tools
2. Navigate to the web UI
3. Attempt to click the Dashboard link in the sidebar
4. Attempt to click the Prompts link in the Environment navigation
5. Observe timeout errors waiting for element stability

### Current Behavior
- Click actions on nav elements timeout after default Playwright timeout
- Error message: element not stable (waiting for "visible, enabled and stable")
- Elements are visible and enabled but fail stability check

### Expected Behavior
- Click actions should complete successfully within reasonable time
- Navigation should work consistently in both manual testing and E2E tests

### Root Cause Analysis

Based on code inspection, multiple factors contribute to element instability:

1. **CSS Transitions on Layout Elements**
   - `.sidebar`: `transition: width 200ms` (Sidebar.css:13)
   - `.app-main`: `transition: margin-left 200ms` (AppLayout.css:13)
   - `.nav-item`: `transition: all 150ms` (Sidebar.css:139)
   - When sidebar state changes, all these transitions fire simultaneously

2. **Entrance Animations on Nav Labels**
   - `.nav-label`: `animation: fade-in 150ms` (Sidebar.css:186)
   - `.keyboard-hint`: `animation: fade-in 150ms` (Sidebar.css:366)
   - These play every time elements mount (sidebar expansion/collapse)

3. **Conditional DOM Rendering**
   - Initiatives section only renders when `expanded && expandedSections.has('initiatives')` (Sidebar.tsx:295)
   - Environment sub-nav only renders when `expanded && expandedSections.has('environment')` (Sidebar.tsx:389)
   - DOM additions cause reflow, compounded by animations

4. **Transform-based Animations**
   - Radix components use `transform: scale()` animations (index.css:218-238)
   - Dropdown menus use `transform: translateY()` animations
   - Transforms can affect bounding box calculations during stability checks

5. **E2E Tests Already Compensate**
   - `sidebar.spec.ts` uses `waitForTimeout(300)` after toggle operations
   - This workaround masks the underlying issue

### Verification
The fix is verified when:
- E2E tests pass without explicit `waitForTimeout()` delays after navigation
- Click actions complete within Playwright's actionability timeout (30s default)
- No "element not stable" errors in test logs

## Success Criteria

- [ ] Identify specific CSS properties causing instability (transitions, animations, transforms)
- [ ] Document which elements are affected and their animation timings
- [ ] Propose solution(s) with tradeoffs:
  - Option A: Reduce animation durations for test reliability
  - Option B: Add `will-change` hints for transform stability
  - Option C: Use `data-testid` with forced stability in E2E
  - Option D: Implement CSS `prefers-reduced-motion` for E2E
- [ ] Create reproducible test case demonstrating the issue
- [ ] Verify solution works with existing E2E test suite

## Testing Requirements

- [ ] E2E test: Sidebar navigation links respond to clicks without waitForTimeout workarounds
- [ ] E2E test: Environment nav links (Prompts, etc.) clickable immediately after section expansion
- [ ] E2E test: Dashboard link clickable on first attempt
- [ ] Visual regression: Animations still work in normal usage (non-reduced-motion)
- [ ] Manual verification: UX unchanged for end users

## Scope

### In Scope
- CSS transitions/animations on sidebar elements (Sidebar.css)
- CSS transitions on layout elements (AppLayout.css)
- Radix UI animation globals (index.css)
- Navigation elements affected by the issue

### Out of Scope
- Board drag-drop functionality (separate interaction pattern)
- Modal animations (not reported as problematic)
- Toast animations (not navigation-related)
- Complete animation system refactor

## Technical Approach

The fix should balance test reliability with user experience. Primary approaches:

### Recommended: CSS `prefers-reduced-motion` Enhancement

Enhance the existing `prefers-reduced-motion` media query (index.css:195-203) to also apply during E2E testing via a CSS class toggle:

```css
/* Already exists in index.css */
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}

/* Add: E2E testing mode class */
html.e2e-testing *,
html.e2e-testing *::before,
html.e2e-testing *::after {
  animation-duration: 0.01ms !important;
  transition-duration: 0.01ms !important;
}
```

E2E setup would add the class:
```typescript
// In fixtures.ts or beforeEach
await page.addStyleTag({ content: 'html { animation-duration: 0.01ms !important; transition-duration: 0.01ms !important; }' });
```

### Alternative: Stabilize Critical Nav Elements

Add `transition: none` override for nav-items when targeted by automation:

```css
.nav-item[data-testid] {
  transition: background-color 150ms, color 150ms;
  /* Exclude layout-affecting properties */
}
```

### Files to Modify

| File | Change |
|------|--------|
| `web/src/index.css` | Add `.e2e-testing` class rules |
| `web/e2e/fixtures.ts` | Inject e2e class or disable animations |
| `web/src/components/layout/Sidebar.css` | Audit animation properties (no changes if using global fix) |
| `web/e2e/sidebar.spec.ts` | Remove `waitForTimeout(300)` workarounds after fix verified |
| `web/e2e/navigation.spec.ts` | Add explicit nav click tests |

## Investigation Checklist

- [x] Read Sidebar.tsx and identify conditional rendering
- [x] Read Sidebar.css and identify transitions/animations
- [x] Read AppLayout.css and identify layout transitions
- [x] Read index.css and identify Radix animations
- [x] Check existing E2E tests for workarounds
- [x] Document animation timing values
- [ ] Create minimal reproduction test
- [ ] Test proposed solutions
