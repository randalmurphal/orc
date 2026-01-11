# Keyboard Shortcuts

**Status**: Planning
**Priority**: P1
**Last Updated**: 2026-01-10

---

## Problem Statement

Power users need fast navigation without using the mouse. Currently:
- No keyboard shortcut system
- No focus management
- Command palette exists but is limited

---

## Solution: Comprehensive Keyboard Navigation

Implement keyboard shortcuts at three levels:
1. Global shortcuts (work anywhere)
2. Page-specific shortcuts (context-aware)
3. Component shortcuts (within focused elements)

---

## Shortcut Categories

### Global Shortcuts

Available from any page:

| Shortcut | Action | Description |
|----------|--------|-------------|
| `⌘K` / `Ctrl+K` | Command palette | Open command palette |
| `n` | New task | Open new task modal |
| `g d` | Go to Dashboard | Navigate to dashboard |
| `g t` | Go to Tasks | Navigate to task list |
| `g s` | Go to Settings | Navigate to settings |
| `g p` | Go to Prompts | Navigate to prompts |
| `/` | Search | Focus search input |
| `?` | Help | Show keyboard shortcuts |
| `Esc` | Close/Cancel | Close modal, cancel action |

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

### Shortcut Manager

```typescript
// lib/shortcuts.ts
import { writable, derived } from 'svelte/store';

interface Shortcut {
  key: string;           // Key combination
  action: () => void;    // Action to execute
  description: string;   // For help display
  scope: string;         // 'global' | 'tasks' | 'task-detail' | etc
  enabled?: () => boolean; // Conditional availability
}

class ShortcutManager {
  private shortcuts: Map<string, Shortcut[]> = new Map();
  private currentScope = writable<string>('global');

  register(shortcut: Shortcut) {
    const existing = this.shortcuts.get(shortcut.key) || [];
    existing.push(shortcut);
    this.shortcuts.set(shortcut.key, existing);
  }

  unregister(key: string, scope: string) {
    const shortcuts = this.shortcuts.get(key) || [];
    this.shortcuts.set(key, shortcuts.filter(s => s.scope !== scope));
  }

  setScope(scope: string) {
    this.currentScope.set(scope);
  }

  handleKeydown(event: KeyboardEvent) {
    const key = this.normalizeKey(event);
    const shortcuts = this.shortcuts.get(key) || [];

    // Find matching shortcut for current scope
    const scope = get(this.currentScope);
    const shortcut = shortcuts.find(s =>
      (s.scope === scope || s.scope === 'global') &&
      (!s.enabled || s.enabled())
    );

    if (shortcut) {
      event.preventDefault();
      shortcut.action();
    }
  }

  private normalizeKey(event: KeyboardEvent): string {
    const parts = [];
    if (event.metaKey || event.ctrlKey) parts.push('mod');
    if (event.shiftKey) parts.push('shift');
    if (event.altKey) parts.push('alt');
    parts.push(event.key.toLowerCase());
    return parts.join('+');
  }

  getShortcutsForScope(scope: string): Shortcut[] {
    const all: Shortcut[] = [];
    this.shortcuts.forEach(shortcuts => {
      shortcuts.forEach(s => {
        if (s.scope === scope || s.scope === 'global') {
          all.push(s);
        }
      });
    });
    return all;
  }
}

export const shortcuts = new ShortcutManager();
```

### Global Listener

```svelte
<!-- App.svelte or layout -->
<script>
  import { onMount } from 'svelte';
  import { shortcuts } from '$lib/shortcuts';
  import { goto } from '$app/navigation';

  onMount(() => {
    // Register global shortcuts
    shortcuts.register({
      key: 'mod+k',
      action: () => openCommandPalette(),
      description: 'Open command palette',
      scope: 'global'
    });

    shortcuts.register({
      key: 'n',
      action: () => openNewTaskModal(),
      description: 'Create new task',
      scope: 'global',
      enabled: () => !isModalOpen()
    });

    shortcuts.register({
      key: '?',
      action: () => openShortcutsHelp(),
      description: 'Show keyboard shortcuts',
      scope: 'global'
    });

    // Go to shortcuts
    ['g d', 'g t', 'g s', 'g p'].forEach(combo => {
      const [, page] = combo.split(' ');
      const routes = { d: '/', t: '/tasks', s: '/settings', p: '/prompts' };
      shortcuts.register({
        key: combo,
        action: () => goto(routes[page]),
        description: `Go to ${page}`,
        scope: 'global'
      });
    });

    // Listen for keydown
    const handler = (e: KeyboardEvent) => shortcuts.handleKeydown(e);
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  });
</script>
```

### Page-Specific Shortcuts

```svelte
<!-- routes/tasks/+page.svelte -->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { shortcuts } from '$lib/shortcuts';

  let selectedIndex = 0;

  onMount(() => {
    shortcuts.setScope('tasks');

    shortcuts.register({
      key: 'j',
      action: () => selectedIndex = Math.min(selectedIndex + 1, tasks.length - 1),
      description: 'Next task',
      scope: 'tasks'
    });

    shortcuts.register({
      key: 'k',
      action: () => selectedIndex = Math.max(selectedIndex - 1, 0),
      description: 'Previous task',
      scope: 'tasks'
    });

    shortcuts.register({
      key: 'enter',
      action: () => goto(`/tasks/${tasks[selectedIndex].id}`),
      description: 'Open task',
      scope: 'tasks'
    });

    shortcuts.register({
      key: 'r',
      action: () => runTask(tasks[selectedIndex].id),
      description: 'Run task',
      scope: 'tasks',
      enabled: () => tasks[selectedIndex]?.status === 'planned'
    });
  });

  onDestroy(() => {
    shortcuts.setScope('global');
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

Pressing `?` shows all available shortcuts:

```
┌─ Keyboard Shortcuts ────────────────────────────────────────┐
│                                                             │
│  Global                                                     │
│  ─────────────────────────────────────────────────────────  │
│  ⌘K         Command palette                                 │
│  n          New task                                        │
│  g d        Go to Dashboard                                 │
│  g t        Go to Tasks                                     │
│  /          Search                                          │
│  ?          Show this help                                  │
│                                                             │
│  Task List                                                  │
│  ─────────────────────────────────────────────────────────  │
│  j / ↓      Move down                                       │
│  k / ↑      Move up                                         │
│  Enter      Open task                                       │
│  r          Run task                                        │
│  p          Pause task                                      │
│  d          Delete task                                     │
│                                                             │
│  Task Detail                                                │
│  ─────────────────────────────────────────────────────────  │
│  r          Run/Resume                                      │
│  p          Pause                                           │
│  t          Transcript tab                                  │
│  Backspace  Back to list                                    │
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
| `TestNormalizeKey_WithMod` | Ctrl+K -> 'mod+k' |
| `TestNormalizeKey_WithShift` | Shift+Tab -> 'shift+tab' |
| `TestNormalizeKey_Combo` | Ctrl+Shift+P -> 'mod+shift+p' |
| `TestShortcutManager_Register` | Adds shortcut correctly |
| `TestShortcutManager_Unregister` | Removes shortcut correctly |
| `TestShortcutManager_ScopeChange` | setScope changes active scope |
| `TestShortcutManager_GlobalInAnyScope` | Global shortcuts always work |
| `TestShortcutManager_ScopedOnly` | Scoped shortcuts only in scope |
| `TestShortcutManager_Enabled` | Respects enabled() callback |
| `TestShortcutManager_PreventDefault` | Returns true when handled |
| `TestSequentialShortcut` | 'g' then 'd' within timeout |
| `TestSequentialShortcut_Timeout` | Expires after timeout |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestShortcutInInputIgnored` | Shortcuts ignored when typing in input |
| `TestShortcutInTextareaIgnored` | Shortcuts ignored in textarea |
| `TestShortcutInModalScope` | Modal shortcuts work when modal open |
| `TestCommandPaletteShortcut` | Cmd+K opens command palette |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_question_mark_help` | `browser_press_key`, `browser_snapshot` | ? opens help modal |
| `test_escape_closes_help` | `browser_press_key` | Escape closes modal |
| `test_cmd_k_palette` | `browser_press_key`, `browser_snapshot` | Cmd+K opens palette |
| `test_n_new_task` | `browser_press_key`, `browser_snapshot` | n opens new task modal |
| `test_g_d_dashboard` | `browser_press_key`, `browser_snapshot` | g then d navigates to dashboard |
| `test_g_t_tasks` | `browser_press_key`, `browser_snapshot` | g then t navigates to tasks |
| `test_j_moves_down` | `browser_press_key`, `browser_snapshot` | j moves selection down |
| `test_k_moves_up` | `browser_press_key`, `browser_snapshot` | k moves selection up |
| `test_enter_opens_task` | `browser_press_key`, `browser_snapshot` | Enter opens selected task |
| `test_r_runs_task` | `browser_press_key`, `browser_wait_for` | r runs selected task |
| `test_p_pauses_task` | `browser_press_key` | p pauses running task |
| `test_backspace_back` | `browser_press_key`, `browser_snapshot` | Backspace returns to list |
| `test_selected_task_indicator` | `browser_snapshot` | Selected task has visual indicator |
| `test_shortcut_hints_visible` | `browser_snapshot` | Hints visible on buttons |
| `test_shortcuts_disabled_in_input` | `browser_type`, `browser_press_key` | Shortcuts don't fire in inputs |
| `test_screen_reader_announces` | ARIA verification | Selection changes announced |

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

- [ ] All listed shortcuts work
- [ ] `?` shows help modal with all shortcuts
- [ ] Vim-style `j/k` navigation in task list
- [ ] `g` prefix shortcuts for navigation
- [ ] Command palette enhanced with commands
- [ ] Visual selection indicator on tasks
- [ ] Shortcut hints visible on buttons
- [ ] Focus management is correct
- [ ] Accessible to screen readers
- [ ] 80%+ test coverage on shortcut code
- [ ] All E2E tests pass
