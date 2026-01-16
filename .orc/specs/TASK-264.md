# Specification: Implement Preferences page functionality

## Problem Statement

The Preferences page (`/preferences`) currently shows a placeholder message. Users need a way to configure UI preferences like theme, default sidebar state, board view mode, and date/time display formats to customize their orc experience.

## User Story

As a user of the orc web interface, I want to configure my UI preferences (theme, sidebar state, board view, date formats) so that the application matches my workflow and visual preferences.

## Acceptance Criteria

- [ ] Users can select between dark and light themes
- [ ] Users can set the default sidebar state (expanded/collapsed)
- [ ] Users can set the default board view mode (flat/swimlane)
- [ ] Users can configure date/time display format preferences
- [ ] Preferences persist across browser sessions (localStorage)
- [ ] Changes take effect immediately without page refresh
- [ ] Reset button restores all preferences to defaults

## Success Criteria

- [ ] Preferences page at `/preferences` renders a functional form with all preference options
- [ ] Theme toggle switches between dark/light themes and persists in localStorage under `orc-theme` key
- [ ] Sidebar default state selector persists in localStorage under existing `orc-sidebar-expanded` key
- [ ] Board view mode selector persists in localStorage under existing `orc-board-view-mode` key
- [ ] Date format selector persists in localStorage under `orc-date-format` key
- [ ] All changes apply immediately across the application (no page refresh required)
- [ ] Reset to defaults button clears all preference localStorage keys and reverts UI
- [ ] Page includes section headers and descriptive text for each preference group
- [ ] Page styling matches existing orc UI patterns (uses Button, Icon, existing CSS variables)

## Testing Requirements

- [ ] Unit test: `Preferences.test.tsx` - Renders all preference controls
- [ ] Unit test: `preferencesStore.test.ts` - Store correctly saves/loads from localStorage
- [ ] Unit test: Theme switching updates CSS variables on document.documentElement
- [ ] Unit test: Reset button clears all preferences and fires store updates
- [ ] E2E test: `preferences.spec.ts` - User can navigate to preferences, change settings, refresh page, and see settings persist
- [ ] E2E test: Theme change is visible immediately (background color change)

## Scope

### In Scope

1. **Theme preference** (dark/light)
   - Toggle switch or radio buttons
   - Updates CSS custom properties on `<html>` element
   - Stored in `orc-theme` localStorage key

2. **Sidebar default state** (expanded/collapsed)
   - Toggle switch or checkbox
   - Integrates with existing `uiStore.sidebarExpanded`
   - Uses existing `orc-sidebar-expanded` localStorage key

3. **Board view mode default** (flat/swimlane)
   - Radio buttons or dropdown
   - Updates initial value when Board page loads
   - Uses existing `orc-board-view-mode` localStorage key

4. **Date/time format** (relative/absolute, 12h/24h)
   - Dropdown or radio selection
   - Options: "Relative (2h ago)", "Absolute (Jan 16, 2026 3:45 PM)", "Absolute 24h (2026-01-16 15:45)"
   - Stored in `orc-date-format` localStorage key
   - Applied to date displays throughout the app (task cards, timeline, etc.)

5. **Preferences store** (new Zustand store)
   - Centralized preference management
   - localStorage persistence with SSR safety
   - Exports hooks for each preference

6. **Reset to defaults button**
   - Clears all preference keys from localStorage
   - Resets store to initial values

### Out of Scope

- Backend API for preferences (client-side only via localStorage)
- Keyboard shortcut customization (complex feature, separate task)
- Notification settings (requires backend integration)
- User accounts/login (orc is single-user)
- Syncing preferences across devices
- Claude Code settings (already in `/environment/settings`)
- Orc automation config (already in `/environment/orchestrator/automation`)

## Technical Approach

### Architecture

Create a new `preferencesStore.ts` Zustand store that:
1. Defines preference types and defaults
2. Loads from localStorage on initialization
3. Provides setters that update both store and localStorage
4. Exports selector hooks for each preference

The Preferences page consumes this store and renders form controls for each setting.

Theme changes update CSS custom properties on `document.documentElement` using a `useEffect` that subscribes to the theme preference.

### Files to Modify

| File | Change |
|------|--------|
| `web/src/stores/preferencesStore.ts` | **NEW** - Zustand store for user preferences |
| `web/src/stores/preferencesStore.test.ts` | **NEW** - Unit tests for preferences store |
| `web/src/stores/index.ts` | Export preferencesStore hooks |
| `web/src/pages/Preferences.tsx` | Replace placeholder with functional form |
| `web/src/pages/Preferences.css` | **NEW** - Styles for preferences page |
| `web/src/pages/Preferences.test.tsx` | **NEW** - Unit tests for Preferences page |
| `web/src/styles/tokens.css` | Add light theme CSS custom property overrides |
| `web/src/App.tsx` | Add ThemeProvider wrapper or effect for theme class |
| `web/src/lib/formatDate.ts` | **NEW** - Date formatting utility respecting preferences |
| `web/e2e/preferences.spec.ts` | **NEW** - E2E tests for preferences persistence |

### Preference Store Interface

```typescript
interface PreferencesStore {
  // State
  theme: 'dark' | 'light';
  dateFormat: 'relative' | 'absolute' | 'absolute24';

  // Actions
  setTheme: (theme: 'dark' | 'light') => void;
  setDateFormat: (format: 'relative' | 'absolute' | 'absolute24') => void;
  resetToDefaults: () => void;
}
```

### Theme Implementation

The theme preference adds a `data-theme="light"` attribute to `<html>`. CSS uses this attribute to switch variable values:

```css
:root {
  --bg-primary: #0d0d0d;
  /* dark theme defaults */
}

:root[data-theme="light"] {
  --bg-primary: #ffffff;
  /* light theme overrides */
}
```

### Date Format Implementation

Create a `formatDate(date: string | Date, preferences?: { dateFormat: string }): string` utility that:
- Returns relative time ("2h ago") when `dateFormat === 'relative'`
- Returns localized absolute ("Jan 16, 2026 3:45 PM") when `dateFormat === 'absolute'`
- Returns ISO-ish 24h ("2026-01-16 15:45") when `dateFormat === 'absolute24'`

Components that display dates will import this utility and call it with the user's preference.

### Integration with Existing localStorage Keys

- **Sidebar**: Already uses `orc-sidebar-expanded` via `uiStore`. The preferences store will read this key and the Preferences page will show the current value. Updates go through `uiStore.setSidebarExpanded()` which already persists.
- **Board view**: Already uses `orc-board-view-mode` in `Board.tsx` local state. The preferences store will read this key, and Board.tsx will check the store for the initial value.

## Design Notes

### Page Layout

```
Preferences
└── Appearance
    ├── Theme: [Dark] [Light]
    └── Date Format: [Dropdown: Relative / Absolute / Absolute 24h]
└── Layout
    ├── Sidebar Default: [Expanded] [Collapsed]
    └── Board Default View: [Flat] [Swimlane]
└── [Reset to Defaults] button
```

### Component Usage

- Use `Button` component for toggle groups (like theme selector)
- Use existing dropdown patterns from `InitiativeDropdown`/`ViewModeDropdown` or Radix Select
- Section headers styled like existing environment pages
- Form groups with labels and help text

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Theme flash on load | Medium | Low | Apply theme class early in HTML or use CSS media query fallback |
| Breaking existing localStorage | Low | Medium | Use new keys for new prefs; existing keys (`orc-sidebar-expanded`, `orc-board-view-mode`) already work |
| Light theme incomplete | Medium | Medium | Start with dark-only, add light theme CSS variables incrementally |

## Dependencies

- Existing Radix UI primitives for accessible form controls
- Existing Zustand pattern for store creation
- Existing CSS custom property system in `tokens.css`
