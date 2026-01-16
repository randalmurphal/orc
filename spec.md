# Specification: Replace filter dropdowns with Radix Select

## Problem Statement

Custom filter dropdowns (InitiativeDropdown, ViewModeDropdown, DependencyDropdown) lack proper keyboard accessibility - no arrow key navigation, typeahead search, or Home/End support. ExportDropdown is an action menu requiring DropdownMenu rather than Select.

## Success Criteria

- [ ] InitiativeDropdown uses Radix Select with controlled mode
- [ ] ViewModeDropdown uses Radix Select with controlled mode
- [ ] DependencyDropdown uses Radix Select with controlled mode
- [ ] ExportDropdown uses Radix DropdownMenu (action menu, not selection)
- [ ] Arrow key navigation works in all dropdowns
- [ ] Typeahead filtering works (type to jump to matching option)
- [ ] Home/End keys navigate to first/last option
- [ ] Escape closes dropdown without changing selection
- [ ] Visual appearance unchanged (CSS class compatibility maintained)
- [ ] Store integration works correctly (filter state persists)
- [ ] E2E tests pass: `bunx playwright test filters.spec.ts`

## Testing Requirements

- [ ] E2E: `filters.spec.ts` - Initiative filter dropdown visibility and selection
- [ ] E2E: `filters.spec.ts` - Dependency filter dropdown visibility and filtering
- [ ] E2E: `filters.spec.ts` - URL persistence for both filters
- [ ] E2E: `filters.spec.ts` - Combined filter behavior
- [ ] Manual: Arrow down opens and focuses first option
- [ ] Manual: Arrow up/down navigates options
- [ ] Manual: Enter selects and closes
- [ ] Manual: Typeahead jumps to matching option (type "Un" in Initiative -> "Unassigned")
- [ ] Manual: Home/End jump to first/last option

## Scope

### In Scope
- Migrate InitiativeDropdown to Radix Select
- Migrate ViewModeDropdown to Radix Select
- Migrate DependencyDropdown to Radix Select
- Migrate ExportDropdown to Radix DropdownMenu
- Preserve existing CSS class names for styling compatibility
- Maintain controlled mode with current store integration
- Update E2E test selectors if needed (Radix uses different DOM structure)

### Out of Scope
- Adding new filter functionality
- Changing filter store logic
- Modifying filter behavior or options
- Visual redesign of dropdowns

## Technical Approach

### Component Selection

| Current Component | Target Radix Component | Rationale |
|-------------------|------------------------|-----------|
| InitiativeDropdown | Select | Single-value selection from list |
| ViewModeDropdown | Select | Single-value selection from list |
| DependencyDropdown | Select | Single-value selection from list |
| ExportDropdown | DropdownMenu | Action menu (triggers actions, no selection state) |

### Radix Select Pattern

```tsx
<Select.Root value={value} onValueChange={onChange}>
  <Select.Trigger className="dropdown-trigger">
    <Select.Value placeholder="Select..." />
    <Select.Icon><Icon name="chevron-down" /></Select.Icon>
  </Select.Trigger>
  <Select.Portal>
    <Select.Content className="dropdown-menu" position="popper" sideOffset={4}>
      <Select.Viewport>
        <Select.Item value="option1" className="dropdown-item">
          <Select.ItemText>Option 1</Select.ItemText>
        </Select.Item>
      </Select.Viewport>
    </Select.Content>
  </Select.Portal>
</Select.Root>
```

### Radix DropdownMenu Pattern (for ExportDropdown)

```tsx
<DropdownMenu.Root>
  <DropdownMenu.Trigger asChild>
    <button className="export-trigger">...</button>
  </DropdownMenu.Trigger>
  <DropdownMenu.Portal>
    <DropdownMenu.Content className="export-menu" sideOffset={4} align="end">
      <DropdownMenu.Label className="export-menu-header">Export Options</DropdownMenu.Label>
      <DropdownMenu.Item className="export-option" onSelect={handleExport}>
        Task Definition
      </DropdownMenu.Item>
      <DropdownMenu.Separator className="export-menu-divider" />
      ...
    </DropdownMenu.Content>
  </DropdownMenu.Portal>
</DropdownMenu.Root>
```

### Key Implementation Details

1. **Null value handling**: InitiativeDropdown uses `null` for "All initiatives" - Radix Select requires string values, use empty string `""` internally and convert in callbacks

2. **CSS class preservation**: Keep existing class names (`.dropdown-trigger`, `.dropdown-menu`, `.dropdown-item`) for styling compatibility

3. **Data attributes for state**: Radix uses `data-state="open|closed"` and `data-highlighted` - update CSS to use these instead of custom `.open`, `.selected` classes where needed

4. **Portal usage**: All Radix Content components portal to document.body by default - already configured per CLAUDE.md

5. **Animations**: Use existing CSS animations with `data-state` selectors:
   ```css
   .dropdown-menu[data-state='open'] {
     animation: dropdown-enter var(--duration-fast) var(--ease-out);
   }
   ```

### Files to Modify

- `web/src/components/board/InitiativeDropdown.tsx`: Replace custom dropdown with Radix Select
- `web/src/components/board/InitiativeDropdown.css`: Update selectors for Radix data attributes
- `web/src/components/board/ViewModeDropdown.tsx`: Replace custom dropdown with Radix Select
- `web/src/components/board/ViewModeDropdown.css`: Update selectors for Radix data attributes
- `web/src/components/filters/DependencyDropdown.tsx`: Replace custom dropdown with Radix Select
- `web/src/components/filters/DependencyDropdown.css`: Update selectors for Radix data attributes
- `web/src/components/task-detail/ExportDropdown.tsx`: Replace custom dropdown with Radix DropdownMenu
- `web/src/components/task-detail/ExportDropdown.css`: Update selectors for Radix data attributes
- `web/e2e/filters.spec.ts`: Update selectors if Radix changes DOM structure (e.g., `[role="listbox"]` might become `[role="combobox"]`)

### E2E Test Selector Updates

Radix Select uses different roles than custom implementation:

| Current Selector | Radix Equivalent |
|------------------|------------------|
| `.dropdown-menu[role="listbox"]` | `[role="listbox"]` (Radix Content) |
| `.dropdown-item[role="option"]` | `[role="option"]` (Radix Item) |
| `aria-expanded` on trigger | Same (Radix preserves) |
| `aria-selected` on option | Same (Radix preserves) |

The E2E tests should mostly work unchanged since they use semantic selectors. May need minor adjustments for DOM structure differences.

## Refactor Analysis

### Before Pattern
Custom dropdown with:
- Manual `useState` for open/closed state
- Manual `useEffect` for click-outside handling
- Manual `onKeyDown` for Escape handling
- Manual ARIA attributes (`role="listbox"`, `aria-expanded`, `aria-selected`)
- Custom focus management

### After Pattern
Radix primitives with:
- Built-in open/closed state management
- Built-in click-outside handling
- Built-in keyboard navigation (Arrow keys, Home/End, Escape, Enter, Typeahead)
- Automatic ARIA attributes and focus management
- Consistent with TaskCard's DropdownMenu pattern (TASK-212)

### Risk Assessment

**Low Risk:**
- Radix Select/DropdownMenu are stable, well-tested components
- `@radix-ui/react-select` already installed in package.json
- Similar migration done for TaskCard quick menu (TASK-212)
- CSS class preservation minimizes visual regressions

**Medium Risk:**
- E2E tests may need selector updates for changed DOM structure
- Null value handling for "All initiatives" requires careful mapping

**Mitigations:**
- Run E2E tests after each dropdown migration
- Test typeahead with real initiative names
- Verify URL persistence still works with value conversions
