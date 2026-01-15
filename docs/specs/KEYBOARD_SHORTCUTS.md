# Keyboard Shortcuts

**Status**: Implemented
**Priority**: P1
**Last Updated**: 2026-01-13

---

## Problem Statement

Power users need fast navigation without using the mouse. Additionally, browser-based applications cannot reliably override common OS/browser shortcuts like `Cmd+K`, `Cmd+N`, `Cmd+P`, and `Cmd+B`.

---

## Solution: Browser-Safe Keyboard Navigation

Use `Shift+Alt` (displayed as `⇧⌥` on Mac) as the primary modifier for shortcuts that would otherwise conflict with browser defaults. This combination is:
- Not used by any major browser for built-in shortcuts
- Consistent across Windows, macOS, and Linux
- Easy to type with one hand on either side of the keyboard

Implement keyboard shortcuts at three levels:
1. Global shortcuts (work anywhere, use Shift+Alt modifier)
2. Page-specific shortcuts (context-aware, single keys)
3. Component shortcuts (within focused elements)

---

## Shortcut Categories

### Global Shortcuts

Available from any page. Uses `Shift+Alt` modifier to avoid browser conflicts:

| Shortcut | Action | Description |
|----------|--------|-------------|
| `Shift+Alt+K` | Command palette | Open command palette |
| `Shift+Alt+N` | New task | Open new task modal |
| `Shift+Alt+B` | Toggle sidebar | Expand/collapse sidebar |
| `Shift+Alt+P` | Project switcher | Switch between projects |
| `g d` | Go to Dashboard | Navigate to dashboard |
| `g t` | Go to Tasks | Navigate to task list |
| `g e` | Go to Environment | Navigate to environment settings |
| `g r` | Go to Preferences | Navigate to preferences |
| `g p` | Go to Prompts | Navigate to prompts |
| `g h` | Go to Hooks | Navigate to hooks |
| `g k` | Go to Skills | Navigate to skills |
| `/` | Search | Focus search input |
| `?` | Help | Show keyboard shortcuts |
| `Esc` | Close/Cancel | Close modal, cancel action |

**Note:** On macOS, `Shift+Alt` is displayed as `⇧⌥` (Shift+Option).

### Task List Shortcuts

When on task list page:

| Shortcut | Action | Description |
|----------|--------|-------------|
| `j` / `↓` | Move down | Select next task |
| `k` / `↑` | Move up | Select previous task |
| `Enter` | Open task | Navigate to task detail |
| `r` | Run | Run selected task |
| `p` | Pause | Pause selected task |
| `s` | Resume | Resume selected task |
| `d` | Delete | Delete selected task (with confirm) |
| `f` | Filter | Focus filter dropdown |
| `c` | Clear filters | Clear all filters |
| `1-4` | Status filter | Quick filter by status |

### Task Detail Shortcuts

When viewing a task:

| Shortcut | Action | Description |
|----------|--------|-------------|
| `r` | Run/Resume | Start or resume task |
| `p` | Pause | Pause running task |
| `c` | Cancel | Cancel task |
| `t` | Transcript | Jump to transcript tab |
| `l` | Timeline | Jump to timeline tab |
| `[` | Previous phase | Scroll to previous phase |
| `]` | Next phase | Scroll to next phase |
| `Backspace` | Back | Return to task list |

### Modal Shortcuts

When a modal is open:

| Shortcut | Action | Description |
|----------|--------|-------------|
| `Esc` | Close | Close modal |
| `Enter` | Confirm | Confirm/submit |
| `Tab` | Next field | Move to next input |
| `Shift+Tab` | Previous field | Move to previous input |

---

## Implementation

### Browser Conflict Resolution

Standard browser shortcuts that cannot be overridden in a web app:
- `Cmd/Ctrl+K` - Address bar (Chrome, Firefox), or bookmarks (Safari)
- `Cmd/Ctrl+N` - New browser window
- `Cmd/Ctrl+P` - Print dialog
- `Cmd/Ctrl+B` - Bookmarks sidebar

**Solution:** Use `Shift+Alt` as the modifier for these shortcuts instead. This combination is unused by browsers and works consistently across platforms.

### Shortcut Manager

```typescript
// lib/shortcuts.ts
export interface Shortcut {
  key: string;
  modifiers?: readonly ('ctrl' | 'meta' | 'shift' | 'alt')[];
  description: string;
  action: () => void;
  context?: 'global' | 'tasks' | 'editor';
}

// Pre-defined shortcuts using Shift+Alt modifier
export const SHORTCUTS = {
  COMMAND_PALETTE: {
    key: 'k',
    modifiers: ['shift', 'alt'] as const,
    description: 'Open command palette'
  },
  NEW_TASK: {
    key: 'n',
    modifiers: ['shift', 'alt'] as const,
    description: 'Create new task'
  },
  TOGGLE_SIDEBAR: {
    key: 'b',
    modifiers: ['shift', 'alt'] as const,
    description: 'Toggle sidebar'
  },
  PROJECT_SWITCHER: {
    key: 'p',
    modifiers: ['shift', 'alt'] as const,
    description: 'Switch project'
  },
  // Single-key shortcuts (no modifier needed)
  HELP: { key: '?', description: 'Show keyboard shortcuts' },
  ESCAPE: { key: 'escape', description: 'Close overlay / Cancel' },
};
```

### Global Listener

```svelte
<!-- +layout.svelte -->
<script>
  import { onMount } from 'svelte';
  import { setupGlobalShortcuts } from '$lib/shortcuts';
  import { goto } from '$app/navigation';

  onMount(() => {
    // Setup all global shortcuts with Shift+Alt modifier
    const cleanup = setupGlobalShortcuts({
      onCommandPalette: () => showCommandPalette = true,
      onNewTask: () => showNewTaskModal = true,
      onToggleSidebar: () => sidebarCollapsed = !sidebarCollapsed,
      onProjectSwitcher: () => showProjectSwitcher = true,
      onHelp: () => showShortcutsHelp = true,
      onGoDashboard: () => goto('/'),
      onGoTasks: () => goto('/tasks'),
      onGoEnvironment: () => goto('/environment'),
    });

    return cleanup;
  });
</script>
```

### Page-Specific Shortcuts

```svelte
<!-- routes/tasks/+page.svelte -->
<script>
  import { onMount } from 'svelte';
  import { setupTaskListShortcuts } from '$lib/shortcuts';

  let selectedIndex = 0;

  onMount(() => {
    // Task list shortcuts use single keys (j/k/r/p/d)
    // since they don't conflict with browser shortcuts
    const cleanup = setupTaskListShortcuts({
      onNavDown: () => selectedIndex = Math.min(selectedIndex + 1, tasks.length - 1),
      onNavUp: () => selectedIndex = Math.max(selectedIndex - 1, 0),
      onOpen: () => goto(`/tasks/${tasks[selectedIndex].id}`),
      onRun: () => runTask(tasks[selectedIndex].id),
      onPause: () => pauseTask(tasks[selectedIndex].id),
      onDelete: () => confirmDelete(tasks[selectedIndex].id),
    });

    return cleanup;
  });
</script>
```

---

## Visual Indicators

### Selected Task Highlight

```css
.task-card {
  /* Normal state */
}

.task-card.selected {
  outline: 2px solid var(--accent-primary);
  outline-offset: 2px;
}

.task-card.selected:focus {
  box-shadow: 0 0 0 4px var(--accent-glow);
}
```

### Shortcut Hints

Show shortcut hints on hover and in tooltips:

```svelte
<button class="action-btn" title="Run task (r)">
  <PlayIcon />
  <span class="btn-label">Run</span>
  <kbd class="shortcut-hint">r</kbd>
</button>

<style>
  .shortcut-hint {
    font-size: var(--text-xs);
    padding: 2px 4px;
    background: var(--bg-tertiary);
    border-radius: var(--radius-sm);
    opacity: 0.7;
  }
</style>
```

---

## Help Modal

Pressing `?` shows all available shortcuts. The modifier keys display platform-appropriately:
- **macOS:** `⇧⌥` (Shift + Option)
- **Windows/Linux:** `Shift+Alt+`

```
┌─ Keyboard Shortcuts ────────────────────────────────────────┐
│                                                             │
│  Global                                                     │
│  ─────────────────────────────────────────────────────────  │
│  ⇧⌥K        Command palette                                 │
│  ⇧⌥N        New task                                        │
│  ⇧⌥B        Toggle sidebar                                  │
│  ⇧⌥P        Switch project                                  │
│  /          Search                                          │
│  ?          Show this help                                  │
│  Esc        Close overlay                                   │
│                                                             │
│  Navigation                                                 │
│  ─────────────────────────────────────────────────────────  │
│  g d        Go to Dashboard                                 │
│  g t        Go to Tasks                                     │
│  g e        Go to Environment                               │
│  g r        Go to Preferences                               │
│  g p        Go to Prompts                                   │
│  g h        Go to Hooks                                     │
│  g k        Go to Skills                                    │
│                                                             │
│  Task List                                                  │
│  ─────────────────────────────────────────────────────────  │
│  j          Select next task                                │
│  k          Select previous task                            │
│  Enter      Open selected task                              │
│  r          Run selected task                               │
│  p          Pause selected task                             │
│  d          Delete selected task                            │
│                                                             │
│                                            [Close] (Esc)    │
└─────────────────────────────────────────────────────────────┘
```

---

## Command Palette Enhancement

Extend command palette to support all actions:

```
┌─ Command Palette ───────────────────────────────────────────┐
│ > run task-                                                 │
│                                                             │
│   ▶ Run TASK-001 - Fix auth timeout                    ⌘R  │
│   ▶ Run TASK-002 - Implement caching                   ⌘R  │
│   ▶ Run TASK-003 - Add dark mode                       ⌘R  │
│                                                             │
│   ─────────────────────────────────────────────────────────│
│                                                             │
│   Recent commands:                                          │
│   + Create new task                                    n    │
│   ⚙ Open settings                                     g s  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

Commands available in palette:
- `new <title>` - Create task
- `run <task-id>` - Run task
- `pause <task-id>` - Pause task
- `show <task-id>` - Open task detail
- `goto <page>` - Navigate to page
- `theme <light|dark>` - Switch theme

---

## Accessibility

### Focus Management

```typescript
// Ensure focus is visible
function ensureFocusVisible(element: HTMLElement) {
  element.focus();
  element.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
}

// Trap focus in modal
function trapFocus(container: HTMLElement) {
  const focusable = container.querySelectorAll(
    'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
  );
  const first = focusable[0] as HTMLElement;
  const last = focusable[focusable.length - 1] as HTMLElement;

  container.addEventListener('keydown', (e) => {
    if (e.key === 'Tab') {
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  });
}
```

### Screen Reader Announcements

```svelte
<div role="status" aria-live="polite" class="sr-only">
  {announcement}
</div>

<script>
  function announce(message: string) {
    announcement = message;
    setTimeout(() => announcement = '', 1000);
  }

  // On task selection change
  $: announce(`Selected ${tasks[selectedIndex]?.title}`);
</script>
```

---

## Configuration

Users can customize shortcuts:

```yaml
# ~/.orc/shortcuts.yaml (future)
shortcuts:
  new_task: "n"          # Override default
  run_task: "ctrl+r"     # Add modifier
  custom_action: "x"     # Custom action
```

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for shortcut handling code
- 100% coverage for key normalization logic

### Unit Tests

| Test | Description |
|------|-------------|
| `TestNormalizeKey_Simple` | 'j' -> 'j' |
| `TestNormalizeKey_WithShiftAlt` | Shift+Alt+K -> 'alt+shift+k' |
| `TestNormalizeKey_WithShift` | Shift+Tab -> 'shift+tab' |
| `TestShortcutManager_Register` | Adds shortcut correctly |
| `TestShortcutManager_Unregister` | Removes shortcut correctly |
| `TestShortcutManager_ContextChange` | setContext changes active context |
| `TestShortcutManager_GlobalInAnyContext` | Global shortcuts always work |
| `TestShortcutManager_ContextOnly` | Context shortcuts only in context |
| `TestShortcutManager_PreventDefault` | Returns true when handled |
| `TestSequentialShortcut` | 'g' then 'd' within timeout |
| `TestSequentialShortcut_Timeout` | Expires after timeout |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestShortcutInInputIgnored` | Shortcuts ignored when typing in input |
| `TestShortcutInTextareaIgnored` | Shortcuts ignored in textarea |
| `TestShortcutInModalScope` | Modal shortcuts work when modal open |
| `TestCommandPaletteShortcut` | Shift+Alt+K opens command palette |

### E2E Tests (Playwright)

**Test file:** `web/e2e/keyboard-shortcuts.spec.ts` (13 tests)

| Category | Test | Description |
|----------|------|-------------|
| Global (6) | `should open command palette with Shift+Alt+K` | Shift+Alt+K opens palette |
| | `should open new task modal with Shift+Alt+N` | Shift+Alt+N opens new task modal |
| | `should toggle sidebar with Shift+Alt+B` | Shift+Alt+B toggles sidebar |
| | `should open project switcher with Shift+Alt+P` | Shift+Alt+P opens project switcher |
| | `should show keyboard help with ? key` | ? opens help modal |
| | `should close all modals with Escape` | Escape closes any open modal |
| Navigation (3) | `should navigate to dashboard with g then d` | g then d goes to dashboard |
| | `should navigate to tasks with g then t` | g then t goes to tasks |
| | `should navigate to environment with g then e` | g then e goes to environment |
| Task List (3) | `should navigate tasks with j/k keys` | j/k moves selection up/down |
| | `should open selected task with Enter` | Enter opens selected task |
| | `should focus search with / key` | / focuses search input |
| Input Fields (1) | `should not trigger shortcuts when typing in input` | Shortcuts disabled during input |

**Implementation details:**
- Test multi-key sequences (g+d, g+t) with sequential `page.keyboard.press()` calls
- Test Shift+Alt modifiers using `page.keyboard.press('Shift+Alt+k')`
- Verify input field awareness via `.task-card-wrapper.selected` class check
- Use `.selected` class for task navigation state

### Accessibility Tests

| Test | Description |
|------|-------------|
| `test_focus_visible` | Focus ring visible on selection |
| `test_focus_trap_modal` | Tab cycles in modal |
| `test_aria_labels` | ARIA labels on interactive elements |

### Test Fixtures
- Mock keyboard events
- Various scope scenarios
- Sequential shortcut test cases

---

## Success Criteria

- [x] All global shortcuts use Shift+Alt modifier (browser-safe)
- [x] `?` shows help modal with platform-appropriate key display
- [x] Vim-style `j/k` navigation in task list
- [x] `g` prefix shortcuts for navigation
- [x] Command palette opens with `Shift+Alt+K`
- [x] New task modal opens with `Shift+Alt+N`
- [x] Project switcher opens with `Shift+Alt+P`
- [x] Sidebar toggles with `Shift+Alt+B`
- [x] Visual selection indicator on tasks (`.task-card-wrapper.selected` class)
- [x] Focus management is correct
- [ ] Accessible to screen readers
- [x] All E2E tests pass (13 tests in `web/e2e/keyboard-shortcuts.spec.ts`)
