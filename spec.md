# Specification: Migrate dashboard and layout components to Button primitive

## Problem Statement

Dashboard and layout components use raw `<button>` elements with ad-hoc CSS classes instead of the unified Button primitive. This creates inconsistent styling, missing accessibility features (aria-disabled, aria-busy), and duplicated hover/focus/disabled state handling.

## Success Criteria

- [ ] All action buttons in target files use the `Button` component from `@/components/ui`
- [ ] Navigation links remain as `NavLink` or `<a>` elements (not converted to buttons)
- [ ] Visual appearance matches current implementation (no visual regression)
- [ ] All existing E2E tests pass without selector changes
- [ ] `npm run test` passes
- [ ] `bunx playwright test dashboard.spec.ts` passes
- [ ] `bunx playwright test --project=visual` passes (visual regression)

## Testing Requirements

- [ ] Unit test: Button component already tested; verify imports work
- [ ] E2E test: `bunx playwright test dashboard.spec.ts` passes
- [ ] E2E test: `bunx playwright test --project=visual` passes (visual baselines)
- [ ] Manual verification: Compare before/after screenshots of Dashboard page

## Scope

### In Scope

**DashboardStats.tsx** (3 buttons)
| Element | Current | Migrate To |
|---------|---------|------------|
| Running stat card | `<button className="stat-card running">` | `<Button variant="ghost" className="stat-card running">` |
| Blocked stat card | `<button className="stat-card blocked">` | `<Button variant="ghost" className="stat-card blocked">` |
| Today stat card | `<button className="stat-card today">` | `<Button variant="ghost" className="stat-card today">` |

Note: Tokens card is a `<div>` (non-interactive) - leave as-is.

**DashboardQuickActions.tsx** (2 buttons)
| Element | Current | Migrate To |
|---------|---------|------------|
| New Task | `<button className="action-btn primary">` | `<Button variant="primary" leftIcon={<Icon name="plus" size={16} />}>` |
| View All Tasks | `<button className="action-btn">` | `<Button variant="secondary" leftIcon={<Icon name="tasks" size={16} />}>` |

**DashboardInitiatives.tsx** (2 button types)
| Element | Current | Migrate To |
|---------|---------|------------|
| Initiative row | `<button className="initiative-row">` | `<Button variant="ghost" className="initiative-row">` |
| View All link | `<button className="view-all-link">` | `<Button variant="ghost" className="view-all-link">` |

**Header.tsx** (3 buttons)
| Element | Current | Migrate To |
|---------|---------|------------|
| Project switcher | `<button className="project-btn">` | `<Button variant="ghost" className="project-btn">` |
| Command palette | `<button className="cmd-hint">` | `<Button variant="ghost" className="cmd-hint">` |
| New Task | `<button className="primary new-task-btn">` | `<Button variant="primary" leftIcon={<Icon name="plus" size={16} />}>` |

**Sidebar.tsx** (4 button types)
| Element | Current | Migrate To |
|---------|---------|------------|
| Toggle sidebar | `<button className="toggle-btn">` | `<Button variant="ghost" iconOnly className="toggle-btn">` |
| Section headers | `<button className="section-header clickable">` | `<Button variant="ghost" className="section-header clickable">` |
| Group headers | `<button className="group-header">` | `<Button variant="ghost" className="group-header">` |
| New Initiative | `<button className="nav-item new-initiative-btn">` | `<Button variant="ghost" leftIcon={<Icon name="plus" size={14} />} className="new-initiative-btn">` |

**ProjectSwitcher.tsx** (2 button types)
| Element | Current | Migrate To |
|---------|---------|------------|
| Close button | `<button className="close-btn">` | `<Button variant="ghost" iconOnly aria-label="Close" title="Close (Esc)">` |
| Project items | `<button className="project-item">` | `<Button variant="ghost" className="project-item">` |

### Out of Scope

- **DashboardActiveTasks.tsx**: Uses `<Link>` for navigation - correct, leave as-is
- **DashboardRecentActivity.tsx**: Uses `<Link>` for navigation - correct, leave as-is
- **Sidebar.tsx NavLinks**: Navigation items use `<NavLink>` - correct, leave as-is
- **CSS files**: May need minor adjustments to work with `.btn` base class, but no major refactoring
- **Other components**: Only dashboard and layout components in this task

## Technical Approach

### Key Considerations

1. **Preserve CSS class names**: The Button component accepts `className` prop, so existing classes like `stat-card`, `action-btn`, `project-btn` can be preserved for styling.

2. **Handle button content structure**: The Button component wraps children in `<span className="btn-content">`. For complex content (like stat cards with icons and labels), use the Button as a wrapper but keep the internal structure.

3. **Icon handling**: Use `leftIcon` prop for buttons with leading icons instead of including Icon as a child.

4. **E2E selector stability**: Tests use role-based selectors (`getByRole('button')`) and text content. The Button component renders a `<button>` element, so selectors should remain stable.

### Files to Modify

1. **`web/src/components/dashboard/DashboardStats.tsx`**
   - Import `Button` from `@/components/ui`
   - Replace 3 `<button>` elements with `<Button variant="ghost">`
   - Preserve `className` and `onClick` props
   - Keep internal content structure (stat-icon, stat-content)

2. **`web/src/components/dashboard/DashboardQuickActions.tsx`**
   - Import `Button` from `@/components/ui`
   - Replace 2 `<button>` elements with `<Button>`
   - Use `leftIcon` prop for icons
   - Primary button: `variant="primary"`
   - Secondary button: `variant="secondary"`

3. **`web/src/components/dashboard/DashboardInitiatives.tsx`**
   - Import `Button` from `@/components/ui`
   - Replace initiative row buttons with `<Button variant="ghost">`
   - Replace "View All" button with `<Button variant="ghost">`
   - Preserve internal content structure for initiative rows

4. **`web/src/components/layout/Header.tsx`**
   - Import `Button` from `@/components/ui`
   - Replace 3 `<button>` elements with `<Button>`
   - Project button: `variant="ghost"`
   - Command palette: `variant="ghost"`
   - New Task: `variant="primary"` with `leftIcon`

5. **`web/src/components/layout/Sidebar.tsx`**
   - Import `Button` from `@/components/ui`
   - Replace toggle button with `<Button variant="ghost" iconOnly>`
   - Replace section/group header buttons with `<Button variant="ghost">`
   - Replace New Initiative button with `<Button variant="ghost">`
   - Preserve `aria-expanded` attributes on collapsible buttons

6. **`web/src/components/overlays/ProjectSwitcher.tsx`**
   - Import `Button` from `@/components/ui`
   - Replace close button with `<Button variant="ghost" iconOnly>`
   - Replace project item buttons with `<Button variant="ghost">`

### CSS Adjustments

The Button component adds `.btn`, `.btn-ghost`, `.btn-md` classes. Component-specific CSS may need adjustments to:
- Override height/padding from `.btn-md` when needed
- Ensure custom backgrounds/borders still apply over `.btn-ghost` defaults
- Handle the `.btn-content` wrapper for text children

## Refactor Analysis

### Before Pattern
```tsx
<button className="action-btn primary" onClick={onNewTask}>
  <Icon name="plus" size={16} />
  New Task
</button>
```

### After Pattern
```tsx
<Button
  variant="primary"
  leftIcon={<Icon name="plus" size={16} />}
  onClick={onNewTask}
>
  New Task
</Button>
```

### Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Visual regression | Medium | Low | Run visual regression tests, compare screenshots |
| E2E selector breakage | Low | Medium | Button renders `<button>`, role selectors should work |
| CSS specificity conflicts | Medium | Low | Use `className` to preserve existing class-based styles |
| Content structure changes | Low | Low | Button allows complex children when not using `leftIcon` |

The main risk is CSS specificity - the Button component's base styles (`.btn`) may conflict with component-specific styles. However, since component classes are preserved via `className`, and CSS order matters, this should be manageable with minor adjustments.
