# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Phase 1: Port keyboard shortcuts system to React

**Description**: ## Purpose
Port the keyboard shortcut system to React using hooks and context.

## Implementation

### ShortcutContext + useShortcuts hook
```typescript
// Provider at app root
<ShortcutProvider>
  <App />
</ShortcutProvider>

// Hook in components
const { registerShortcut, unregisterShortcut } = useShortcuts()
```

### Global Shortcuts (Shift+Alt modifier)
| Shortcut | Action |
|----------|--------|
| Shift+Alt+K | Open command palette |
| Shift+Alt+N | Open new task modal |
| Shift+Alt+B | Toggle sidebar |
| Shift+Alt+P | Open project switcher |
| ? | Show keyboard help |
| Escape | Close modals |

### Navigation Sequences (g + key)
| Sequence | Destination |
|----------|-------------|
| g d | Dashboard |
| g t | Tasks |
| g e | Environment |

### Context-Aware Shortcuts
- Task list: j/k navigation, Enter to open, r to run
- Search focus: / key

## Key Implementation Details
- Use Shift+Alt modifier (not Cmd/Ctrl) to avoid browser conflicts
- Multi-key sequences need timing window tracking
- Disable shortcuts when in input/textarea fields

## Success Criteria
- All shortcuts work identically to Svelte
- No conflicts with browser shortcuts
- Shortcuts disabled in form inputs



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

All 293 tests pass. Let me summarize what was implemented:

## Summary

I've successfully ported the keyboard shortcuts system from Svelte to React. Here's what was created:

### Core Files

1. **`src/lib/shortcuts.ts`** - The ShortcutManager class
   - Singleton pattern for global shortcut management
   - Registers single-key shortcuts with optional modifiers
   - Supports multi-key sequences with 1000ms timeout window
   - Context system for filtering shortcuts (global/tasks/editor)
   - Input field detection (disables shortcuts when typing, except Escape)
   - `setupGlobalShortcuts()` and `setupTaskListShortcuts()` helper functions
   - Pre-defined `SHORTCUTS` constant with all shortcut definitions

2. **`src/lib/platform.ts`** - Platform detection utilities
   - `isMac()` - Detects Mac platform via userAgentData or userAgent fallback
   - `getModifierKey()` / `getModifierSymbol()` - Platform-specific modifier display
   - `formatShortcut()` - Formats shortcuts for display

3. **`src/hooks/useShortcuts.tsx`** - React hooks and context
   - `ShortcutProvider` - Context provider at app root
   - `useShortcuts()` - Access shortcut manager methods
   - `useShortcutContext()` - Set context for a component
   - `useGlobalShortcuts()` - Register global shortcuts with navigation
   - `useTaskListShortcuts()` - Register task list shortcuts

4. **`src/components/overlays/Modal.tsx`** - Reusable modal component
5. **`src/components/overlays/KeyboardShortcutsHelp.tsx`** - Help modal showing all shortcuts

### Integration

- `ShortcutProvider` added to `App.tsx` wrapping the entire app
- `AppLayout.tsx` updated to wire up global shortcuts with help modal

### Key Features

| Feature | Implementation |
|---------|---------------|
| Global shortcuts | Shift+Alt+K/N/B/P, /, ?, Escape |
| Navigation sequences | g+d, g+t, g+e, g+r, g+p, g+h, g+k |
| Task list shortcuts | j/k navigation, Enter, r, p, d (context: 'tasks') |
| Input awareness | Shortcuts disabled in input/textarea except Escape |
| Platform detection | Mac symbols (⇧⌥) vs Windows/Linux text (Shift+Alt+) |

### Tests

- 57 new tests across shortcuts.test.ts, platform.test.ts, and useShortcuts.test.tsx
- All 293 tests pass
- Build succeeds

<phase_complete>true</phase_complete>

---
Tokens: 4519930 input, 29147 output, 97128 cache_creation, 4417979 cache_read
Complete: true
Blocked: false
