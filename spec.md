# Specification: Replace TabNav with Radix Tabs

## Problem Statement
The custom TabNav component in TaskDetail uses manual ARIA attributes and lacks keyboard navigation (arrow keys between tabs, Home/End). Replacing it with Radix Tabs will provide full accessibility compliance including automatic ARIA management, focus handling, and keyboard navigation.

## Success Criteria
- [ ] TabNav.tsx uses Radix Tabs.Root, Tabs.List, and Tabs.Trigger components
- [ ] Tab panels in TaskDetail.tsx are wrapped with Tabs.Content components
- [ ] Arrow left/right switches between tabs
- [ ] Home/End keys jump to first/last tab
- [ ] Tab key moves focus to panel content (when applicable)
- [ ] URL persistence works (`?tab=xxx` updates on tab change)
- [ ] Direct URL navigation loads correct tab (`/tasks/TASK-001?tab=changes`)
- [ ] Visual appearance unchanged (same CSS classes applied)
- [ ] aria-label on Tabs.List updated to "Task details tabs" (matching E2E test expectation)
- [ ] Existing E2E tests pass without modification

## Testing Requirements
- [ ] E2E: `bunx playwright test task-detail.spec.ts` passes (15 tests)
- [ ] E2E: Tab navigation tests (show all tabs, switching, URL updates, URL loading)
- [ ] E2E: `[role="tablist"][aria-label="Task details tabs"]` selector works
- [ ] Manual: Arrow key navigation between tabs works
- [ ] Manual: Focus indicator visible on keyboard navigation

## Scope
### In Scope
- Replace TabNav.tsx implementation with Radix Tabs
- Wrap TaskDetail.tsx tab panels with Tabs.Content
- Add CSS for `.tab-btn[data-state='active']` styling
- Add CSS for `.tab-panel[data-state='active']` panel fade-in animation
- Add focus-visible styling for keyboard navigation
- Maintain URL persistence via onValueChange handler

### Out of Scope
- Changes to tab content components (TimelineTab, ChangesTab, etc.)
- Changes to other pages using tabs (if any)
- New tab additions or removals
- Tab content refactoring

## Technical Approach

### Implementation Strategy: Full Radix Tabs
Use Option A from the task description - wrap entire tab section in Tabs.Root with both Tabs.List (triggers) and Tabs.Content (panels). This provides:
1. Automatic ARIA attributes (aria-selected, aria-controls, role)
2. Built-in keyboard navigation
3. Focus management between tabs and panels
4. Consistent animation hooks via data-state

### Files to Modify

1. **`web/src/components/task-detail/TabNav.tsx`**
   - Import `@radix-ui/react-tabs` (already installed)
   - Replace manual `<nav role="tablist">` with `<Tabs.List>`
   - Replace `<button role="tab">` with `<Tabs.Trigger>`
   - Export Tabs.Root and Tabs.Content for use in parent
   - Keep TABS config array and TabId type

2. **`web/src/pages/TaskDetail.tsx`**
   - Wrap entire tab section in `<Tabs.Root value={activeTab} onValueChange={handleTabChange}>`
   - Replace conditional tab panel rendering with `<Tabs.Content>` wrappers
   - Remove manual `id="panel-${tab.id}"` since Radix handles aria-controls

3. **`web/src/components/task-detail/TabNav.css`**
   - Add `.tab-btn[data-state='active']` selector (alongside existing `.tab-btn.active`)
   - Add `.tab-btn:focus-visible` ring styling
   - Add `.tab-panel[data-state='active']` fade-in animation
   - Add `@keyframes tab-panel-in` animation

### Component Structure (After)

```tsx
// TaskDetail.tsx
<Tabs.Root value={activeTab} onValueChange={handleTabChange}>
  <TabNav />  {/* Renders Tabs.List with Tabs.Triggers */}

  <div className="tab-content">
    <Tabs.Content value="timeline" className="tab-panel">
      <TimelineTab ... />
    </Tabs.Content>
    <Tabs.Content value="changes" className="tab-panel">
      <ChangesTab ... />
    </Tabs.Content>
    {/* ... other panels */}
  </div>
</Tabs.Root>
```

```tsx
// TabNav.tsx (simplified)
export function TabNav() {
  return (
    <Tabs.List className="tab-nav" aria-label="Task details tabs">
      {TABS.map((tab) => (
        <Tabs.Trigger key={tab.id} value={tab.id} className="tab-btn">
          <Icon name={tab.icon} size={16} />
          <span>{tab.label}</span>
        </Tabs.Trigger>
      ))}
    </Tabs.List>
  );
}
```

### URL Persistence
Radix Tabs is a controlled component - the value and onValueChange pattern works exactly like the current implementation. The handleTabChange function already updates URL via `setSearchParams({ tab: tabId }, { replace: true })`.

### CSS Changes

```css
/* Active state - Radix uses data-state attribute */
.tab-btn[data-state='active'] {
  background: var(--accent-glow);
  color: var(--accent-primary);
}

/* Focus ring for keyboard navigation */
.tab-btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px var(--accent-glow);
}

/* Panel entrance animation */
.tab-panel[data-state='active'] {
  animation: tab-panel-in var(--duration-fast) var(--ease-out);
}

@keyframes tab-panel-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
```

## Feature: Radix Tabs Migration

### User Story
As a user navigating the task detail page, I want to use keyboard shortcuts (arrow keys, Home/End) to switch between tabs so that I can navigate efficiently without a mouse.

### Acceptance Criteria
1. Pressing ArrowRight/ArrowLeft while a tab is focused switches to next/previous tab
2. Pressing Home/End while a tab is focused jumps to first/last tab
3. Tab key moves focus from tab list to panel content
4. All existing mouse interactions continue to work
5. URL updates correctly when switching tabs via keyboard
6. Visual styling remains identical to current implementation
7. All existing E2E tests pass without modification

### Risk Assessment
- **Low risk**: Radix Tabs is already a dependency (v1.1.13 installed)
- **Low risk**: No API changes needed - just component structure
- **Low risk**: CSS uses data-state selectors which Radix provides
- **Minimal visual change**: Same CSS classes, just different state attribute
