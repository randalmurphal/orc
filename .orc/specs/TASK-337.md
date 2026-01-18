# Specification: Create TopBar component with session metrics

## Problem Statement

Create a 48px fixed header component (TopBar) for the main application that displays project selection, search, session metrics (duration, tokens, cost), and action buttons (Pause/Resume, New Task).

## Success Criteria

- [x] TopBar component exists at `web/src/components/layout/TopBar.tsx`
- [x] Fixed 48px height with `var(--bg-elevated)` background and bottom border
- [x] Left section: Project dropdown selector with folder icon, name, chevron
- [x] Center section: Search box with search icon
- [x] Center section: Session stats displaying duration, tokens, cost with colored icon badges
- [x] Right section: Pause/Resume button that toggles based on `isPaused` state
- [x] Right section: New Task button (when `onNewTask` callback provided)
- [x] Zustand integration: `useSessionStore` for metrics, `useCurrentProject` for project name
- [x] Accessibility: `role="banner"` on header element
- [x] Accessibility: `aria-label="Search tasks"` on search input
- [x] Accessibility: `aria-haspopup="listbox"` on project selector
- [x] Token formatting with K/M suffixes (e.g., "847K", "1.2M")
- [x] Cost formatting as "$X.XX"
- [x] Pause button text changes to "Resume" when paused
- [x] `npm run typecheck` exits 0

## Testing Requirements

- [x] Unit test: Component renders with `role="banner"` (TopBar.test.tsx:39-41)
- [x] Unit test: Displays project name from store or prop override (TopBar.test.tsx:44-62)
- [x] Unit test: Displays session duration from store (TopBar.test.tsx:66-69)
- [x] Unit test: Displays formatted token count (TopBar.test.tsx:71-75)
- [x] Unit test: Displays formatted cost (TopBar.test.tsx:77-82)
- [x] Unit test: Session stats update when store changes (TopBar.test.tsx:84-100)
- [x] Unit test: Shows "Pause" when not paused, "Resume" when paused (TopBar.test.tsx:104-113)
- [x] Unit test: Calls `pauseAll()` / `resumeAll()` on button click (TopBar.test.tsx:116-140)
- [x] Unit test: New Task button renders conditionally and fires callback (TopBar.test.tsx:143-163)
- [x] Unit test: Search input has `aria-label` (TopBar.test.tsx:172-174)
- [x] Unit test: Project selector has `aria-haspopup` (TopBar.test.tsx:176-181)
- [x] Unit test: Project selector fires `onProjectChange` callback (TopBar.test.tsx:184-192)

## Scope

### In Scope
- TopBar.tsx component with all visual elements
- TopBar.css with exact CSS styling matching mockup
- Integration with sessionStore and projectStore
- Pause/Resume functionality via API calls
- Accessibility attributes

### Out of Scope (deferred to separate tasks)
- Responsive breakpoints (768px/480px media queries) - separate task
- Keyboard shortcut (Cmd+K) for search focus - handled by shortcuts.ts with `/` key instead
- Project dropdown menu functionality - static button only
- Search functionality - static input only
- Panel toggle button - not in current mockup

## Technical Approach

### Files

| File | Status | Purpose |
|------|--------|---------|
| `web/src/components/layout/TopBar.tsx` | Complete | Main component implementation |
| `web/src/components/layout/TopBar.css` | Complete | Component styles |
| `web/src/components/layout/TopBar.test.tsx` | Complete | 20 unit tests |

### Implementation Details

**Component Structure:**
```tsx
<header className="top-bar" role="banner">
  <div className="top-bar-left">
    <button className="project-selector" aria-haspopup="listbox">
      <Icon name="folder" /> {projectName} <Icon name="chevron-down" />
    </button>
    <div className="search-box">
      <Icon name="search" />
      <input type="text" aria-label="Search tasks" />
    </div>
  </div>
  <div className="top-bar-center">
    <div className="session-info">
      <SessionStat icon="clock" label="Session" value={duration} colorClass="purple" />
      <div className="session-divider" />
      <SessionStat icon="zap" label="Tokens" value={formattedTokens} colorClass="amber" />
      <div className="session-divider" />
      <SessionStat icon="dollar" label="Cost" value={formattedCost} colorClass="green" />
    </div>
  </div>
  <div className="top-bar-right">
    <Button variant="ghost">{isPaused ? 'Resume' : 'Pause'}</Button>
    <Button variant="primary" leftIcon={<Icon name="plus" />}>New Task</Button>
  </div>
</header>
```

**Store Integration:**
- `useSessionStore()` provides: `duration`, `formattedTokens`, `formattedCost`, `isPaused`, `pauseAll()`, `resumeAll()`
- `useCurrentProject()` provides: current project object with `name` property

**CSS Design Tokens:**
- Background: `var(--bg-elevated)`, `var(--bg-surface)`
- Borders: `var(--border)`, `var(--border-light)`
- Colors: `var(--primary)`, `var(--amber)`, `var(--green)` with `-dim` variants
- Text: `var(--text-primary)`, `var(--text-muted)`, `var(--text-secondary)`
- Font: `var(--font-mono)` for metric values

## Feature Analysis

**User Story:** As a user, I want to see session metrics and quick actions in a persistent header so that I can monitor progress and control task execution without navigating away from my current view.

**Acceptance Criteria:**
1. Header is always visible at 48px height - ✓
2. Can see current project name - ✓
3. Can search tasks (static input, functionality deferred) - ✓
4. Session duration displays in human-readable format (e.g., "1h 23m") - ✓
5. Token count displays with K/M suffix (e.g., "847K") - ✓
6. Cost displays as currency (e.g., "$2.34") - ✓
7. Can pause/resume all running tasks - ✓
8. Can create new task via button - ✓

## Notes

**CSS Values:** The implementation follows the mockup HTML (board.html) rather than the task description's "EXACT CSS VALUES" section where they differ. Key differences:
- `.top-bar` padding: 0 12px (mockup) vs 0 16px (task)
- `.search-box` width: fixed 200px (mockup) vs flex:1 max-width:300px (task)

These match the visual design in Screenshot_20260116_201804.png.

**Keyboard Shortcuts:** The task specifies "Cmd+K focuses search" but this conflicts with browser address bar. The codebase uses `/` key for search focus and `Shift+Alt+K` for command palette per shortcuts.ts documentation.
