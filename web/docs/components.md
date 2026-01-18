# Component Library

UI primitives and patterns used throughout the frontend.

## Button

Unified button with variants, sizes, icons, and loading state.

```tsx
import { Button } from '@/components/ui';

<Button variant="primary">Submit</Button>
<Button variant="secondary" leftIcon={<Icon name="plus" />}>Add</Button>
<Button loading>Saving...</Button>
<Button variant="ghost" iconOnly aria-label="Close"><Icon name="x" /></Button>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `variant` | `'primary' \| 'secondary' \| 'danger' \| 'ghost' \| 'success'` | `'secondary'` | Visual style |
| `size` | `'sm' \| 'md' \| 'lg'` | `'md'` | Button size |
| `loading` | `boolean` | `false` | Show spinner |
| `leftIcon` / `rightIcon` | `ReactNode` | - | Icon placement |
| `iconOnly` | `boolean` | `false` | Square button mode |

**Sizes:** sm (28px), md (36px), lg (44px)

**Accessibility:** Icon-only buttons require `aria-label`. Loading sets `aria-busy`.

## Radix UI Primitives

Built on `@radix-ui` for accessibility and keyboard navigation.

| Component | Package | Usage |
|-----------|---------|-------|
| DropdownMenu | `@radix-ui/react-dropdown-menu` | TaskCard menu, ExportDropdown |
| Select | `@radix-ui/react-select` | InitiativeDropdown, ViewModeDropdown |
| Tabs | `@radix-ui/react-tabs` | TabNav in task detail |
| Tooltip | `@radix-ui/react-tooltip` | Replace native `title` |
| Dialog | `@radix-ui/react-dialog` | Modal.tsx |

### Radix Patterns

- Portal to `document.body` by default
- Style via `data-*` attributes: `data-state="open|closed"`, `data-highlighted`
- Trigger uses `asChild` to wrap existing Button components
- Select requires string values (map `null` to constants like `'__all__'`)
- Animations in `index.css` respect `prefers-reduced-motion`

### Keyboard Navigation (automatic)

| Component | Keys |
|-----------|------|
| DropdownMenu/Select | Arrow keys, Enter, Escape, Home/End, typeahead |
| Tabs | Arrow left/right, Home/End, Tab to panel |
| Dialog | Escape closes, Tab cycles within focus trap |
| Tooltip | Focus shows, blur hides |

## Tooltip

Replaces native `title` with consistent styling and keyboard support.

```tsx
import { Tooltip } from '@/components/ui';

<Tooltip content="Helpful info"><button>Hover me</button></Tooltip>
<Tooltip content={<>Press <kbd>Enter</kbd></>} side="right"><button>Submit</button></Tooltip>
```

**TooltipProvider** at App.tsx root provides 300ms delay.

## Modal

Accessible modal built on Radix Dialog.

```tsx
import { Modal } from '@/components/overlays';

<Modal open={isOpen} onClose={() => setIsOpen(false)} title="Confirm">
  <p>Are you sure?</p>
</Modal>
```

**Features:** Focus trap, Escape closes, backdrop click closes, body scroll lock.

## Input / Textarea

Form inputs with variants, sizes, icons, and error states.

```tsx
<Input placeholder="Search..." leftIcon={<Icon name="search" />} />
<Input variant="error" error="Required field" />
<Textarea autoResize maxHeight={200} showCount maxLength={500} />
```

**State styles:** Default, hover (border-strong), focus (accent ring), error (danger), disabled.

## Icon

SVG icon component with 60+ built-in icons.

```tsx
<Icon name="dashboard" />
<Icon name="check" size={16} />
<Icon name="error" size={24} className="text-danger" />
```

**Categories:** Navigation, Actions, Playback, Chevrons, Status, Git, Circle variants.

## StatusIndicator

Colored status orb with animations.

```tsx
<StatusIndicator status="running" />
<StatusIndicator status="completed" size="lg" showLabel />
```

**Status colors:** running (accent/pulse), paused (warning/pulse), blocked (danger), completed (success), failed (danger).

## Pipeline

Horizontal phase visualization for task execution progress.

```tsx
import { Pipeline } from '@/components/board';

// Basic usage
<Pipeline
  phases={["Plan", "Code", "Test", "Review", "Done"]}
  currentPhase="Code"
  completedPhases={["Plan"]}
/>

// With progress percentage
<Pipeline
  phases={["Plan", "Code", "Test", "Review", "Done"]}
  currentPhase="Code"
  completedPhases={["Plan"]}
  progress={45}
/>

// Compact variant (no labels)
<Pipeline
  phases={["Plan", "Code", "Test", "Review", "Done"]}
  currentPhase="Test"
  completedPhases={["Plan", "Code"]}
  size="compact"
/>

// Failed phase
<Pipeline
  phases={["Plan", "Code", "Test", "Review", "Done"]}
  currentPhase=""
  completedPhases={["Plan", "Code"]}
  failedPhase="Test"
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `phases` | `string[]` | - | Array of phase names to display |
| `currentPhase` | `string` | - | Currently active phase name |
| `completedPhases` | `string[]` | - | List of completed phase names |
| `failedPhase` | `string` | - | Phase that failed (if any) |
| `progress` | `number` | - | 0-100 progress for current phase |
| `size` | `'compact' \| 'default'` | `'default'` | Compact hides labels |

**Phase states:** pending (muted), active (primary with pulse), completed (green with checkmark), failed (red with X).

**Accessibility:** Uses `role="progressbar"` with `aria-valuenow`, `aria-valuemin`, `aria-valuemax`, and descriptive `aria-valuetext`.

## ToastContainer

Portal-rendered notification queue via `uiStore`.

```tsx
import { toast } from '@/stores';

toast.success('Task created');
toast.error('Failed to save', { duration: 10000 });
toast.warning('Unsaved changes');
toast.info('Processing...');
```

**Durations:** success/warning/info (5s), error (8s).

## TopBar

Fixed 48px header with project selector, search, session metrics, and action buttons.

```tsx
import { TopBar } from '@/components/layout';

<TopBar
  onProjectChange={() => openProjectPicker()}
  onNewTask={() => openNewTaskModal()}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `projectName` | `string` | From store | Override project name (for testing) |
| `onProjectChange` | `() => void` | - | Open project picker |
| `onNewTask` | `() => void` | - | Open new task modal |
| `onSearch` | `(query: string) => void` | - | Search callback |
| `className` | `string` | - | Additional CSS class |

**Store Integration:**
- `useCurrentProject()` - Current project name
- `useSessionStore()` - Session metrics (duration, tokens, cost, isPaused)

**Session Stats:**
- Duration (purple badge, clock icon): "2h 34m"
- Tokens (amber badge, zap icon): "847K"
- Cost (green badge, dollar icon): "$2.34"

**Actions:**
- Pause/Resume button toggles `isPaused` via `pauseAll()`/`resumeAll()`
- New Task button (primary) triggers `onNewTask` callback

**Accessibility:** `role="banner"`, `aria-label="Search tasks"`, `aria-haspopup="listbox"` on project selector.
