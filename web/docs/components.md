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

**Categories:** Navigation/Sidebar, Actions, Playback, Chevrons, Status, Dashboard stats, Git, Circle variants, Panel, Database, Edit/Action, Automation, Category, Theme, Environment, IconNav (help, bar-chart).

## StatusIndicator

Colored status orb with animations.

```tsx
<StatusIndicator status="running" />
<StatusIndicator status="completed" size="lg" showLabel />
```

**Status colors:** running (accent/pulse), paused (warning/pulse), blocked (danger), completed (success), failed (danger).

## IconNav

56px icon-based navigation sidebar with vertical layout.

```tsx
import { IconNav } from '@/components/layout';

<IconNav />
<IconNav className="custom-nav" />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `className` | `string` | `''` | Additional CSS classes |

**Structure:**
- Logo section with gradient "O" mark
- Main nav: Board, Initiatives, Stats
- Divider
- Secondary nav: Agents, Settings
- Bottom section: Help

**States:** Default (muted), hover (surface bg), active (primary-dim bg with primary-bright text).

**Accessibility:** `role="navigation"`, `aria-label="Main navigation"`, tooltips on hover with descriptions.

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

## TaskCard

Compact card component for queue display in the Board view.

```tsx
import { TaskCard } from '@/components/board';

<TaskCard
  task={task}
  onClick={() => navigate(`/tasks/${task.id}`)}
  onContextMenu={(e) => showContextMenu(e, task)}
  isSelected={selectedId === task.id}
  showInitiative={true}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `task` | `Task` | required | Task data object |
| `onClick` | `() => void` | - | Click handler (navigation) |
| `onContextMenu` | `(e: MouseEvent) => void` | - | Right-click handler |
| `isSelected` | `boolean` | `false` | Show selected state |
| `showInitiative` | `boolean` | `false` | Display initiative badge |
| `className` | `string` | `''` | Additional CSS classes |

**Visual Elements:**
- Category icon (sparkles, bug, recycle, tools, file-text, beaker)
- Task ID badge (monospace, muted)
- Title (2-line clamp with ellipsis)
- Priority dot (critical: red, high: orange, normal: blue, low: muted)
- Blocked warning icon (pulsing when blocked)
- Running indicator (animated dot when task is running)

**States:**
- Default: Surface background, standard border
- Hover: `--bg-hover` background, `--border-light` border
- Selected: `--primary` border with 1px shadow
- Running: Primary border tint, subtle gradient background
- Blocked: Red border tint, warning icon

**Accessibility:**
- `role="button"` with descriptive `aria-label`
- Keyboard navigation: Enter/Space triggers onClick
- Focus visible outline for keyboard users
- Minimum 44px touch target

## Swimlane

Collapsible task group for displaying tasks organized by initiative.

```tsx
import { Swimlane } from '@/components/board';

// Initiative swimlane
<Swimlane
  initiative={initiative}
  tasks={tasks}
  isCollapsed={false}
  onToggle={() => toggleCollapse(initiative.id)}
  onTaskClick={(task) => navigate(`/tasks/${task.id}`)}
  onContextMenu={(task, e) => showContextMenu(e, task)}
/>

// Unassigned tasks swimlane
<Swimlane
  initiative={null}
  tasks={unassignedTasks}
  isCollapsed={isCollapsed}
  onToggle={() => toggleCollapse('unassigned')}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `initiative` | `Initiative \| null` | required | Initiative object, or null for unassigned |
| `tasks` | `Task[]` | required | Tasks belonging to this swimlane |
| `isCollapsed` | `boolean` | required | Whether swimlane content is hidden |
| `onToggle` | `() => void` | required | Toggle collapse callback |
| `onTaskClick` | `(task: Task) => void` | - | Task click handler |
| `onContextMenu` | `(task: Task, e: MouseEvent) => void` | - | Right-click handler |

**Visual Elements:**
- Chevron icon (rotates on collapse)
- Initiative icon (emoji in colored circle, or "?" for unassigned)
- Initiative title (or "Unassigned")
- Progress meta: "3/5 complete" (initiatives only)
- Task count badge
- Progress bar (colored by initiative theme)

**Color Themes:** Purple, green, amber, blue, cyan (derived from initiative ID hash). Unassigned uses muted gray.

**States:**
- Expanded: Content visible with smooth height animation
- Collapsed: Content hidden, chevron rotated -90°
- Empty: Shows "No tasks" message

**Accessibility:**
- Header has `role="button"` with `aria-expanded`
- Content has `aria-hidden` when collapsed
- Keyboard: Enter/Space toggles collapse
- `data-testid="swimlane-{id}"` for testing

## QueueColumn

Column component for displaying queued tasks grouped by initiative in swimlanes.

```tsx
import { QueueColumn } from '@/components/board';

<QueueColumn
  tasks={queuedTasks}
  initiatives={initiatives}
  collapsedSwimlanes={collapsedSet}
  onToggleSwimlane={(id) => toggleCollapse(id)}
  onTaskClick={(task) => navigate(`/tasks/${task.id}`)}
  onContextMenu={(task, e) => showContextMenu(e, task)}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `tasks` | `Task[]` | required | Tasks filtered to queued status |
| `initiatives` | `Initiative[]` | required | Initiatives for grouping and display |
| `collapsedSwimlanes` | `Set<string>` | - | Set of collapsed swimlane IDs |
| `onToggleSwimlane` | `(id: string) => void` | - | Collapse toggle callback |
| `onTaskClick` | `(task: Task) => void` | - | Task click handler |
| `onContextMenu` | `(task: Task, e: MouseEvent) => void` | - | Right-click handler |

**Visual Elements:**
- Column header with indicator dot, "Queue" title, and task count badge
- Scrollable body with custom thin scrollbar (5px)
- Swimlanes grouped by initiative
- Empty state: "No queued tasks" centered

**Layout:**
- `flex: 1` with `min-width: 280px`
- Transparent background (inherits page)
- Right border separator
- Header: 10px 12px padding
- Body: 12px padding, scrollable

**Swimlane Sorting:**
1. Active initiatives first (status === 'active')
2. Then by task count (descending)
3. "Unassigned" swimlane always at bottom

**Edge Cases:**
- Tasks with unknown initiative_id are placed in "Unassigned"
- Empty task list shows empty state (no swimlanes)
- Undefined collapsedSwimlanes treats all as expanded

**Accessibility:**
- `role="region"` with `aria-label="Queue column"`
- Count badge has `aria-label="{n} tasks"`

## InitiativeCard

Card component for displaying initiative information in a grid layout.

```tsx
import { InitiativeCard } from '@/components/initiatives';

// Basic usage
<InitiativeCard
  initiative={initiative}
  completedTasks={15}
  totalTasks={20}
  onClick={() => navigate(`/initiatives/${initiative.id}`)}
/>

// With all metrics
<InitiativeCard
  initiative={initiative}
  completedTasks={15}
  totalTasks={20}
  estimatedTimeRemaining="Est. 2h remaining"
  costSpent={18.45}
  tokensUsed={542000}
  onClick={handleClick}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `initiative` | `Initiative` | required | Initiative data object |
| `completedTasks` | `number` | `0` | Number of completed tasks |
| `totalTasks` | `number` | `0` | Total number of tasks |
| `estimatedTimeRemaining` | `string` | - | Time remaining text (e.g., "8h remaining") |
| `costSpent` | `number` | - | Cost in dollars |
| `tokensUsed` | `number` | - | Token count |
| `onClick` | `() => void` | - | Click handler for navigation |
| `className` | `string` | `''` | Additional CSS classes |

**Visual Elements:**
- Icon: 40px circle with emoji from title/vision (falls back to 📋)
- Title: 15px semibold, primary text
- Description: 12px secondary text, 2-line clamp with ellipsis
- Status badge: uppercase 9px with color variant
- Progress section: label, "X / Y tasks" count, colored progress bar
- Meta row: icons for time, cost, tokens (only shown if data provided)

**Status Badge Colors:**
| Status | Background | Text |
|--------|------------|------|
| `active` | `--green-dim` | `--green` |
| `paused` | `--amber-dim` | `--amber` |
| `completed` | `--primary-dim` | `--primary` |
| `draft`/`archived` | `--amber-dim` | `--amber` |

**Icon Background Colors:** Same mapping as status (active → green, completed → purple, etc.)

**States:**
- Default: `--bg-card` background, 1px `--border`, 12px radius, 20px padding
- Hover: `translateY(-2px)`, border lightens to `--border-light`
- Paused/Archived: `opacity: 0.6`
- Focus: 2px `--primary` outline

**Exported Utilities:**
```tsx
import {
  extractEmoji,
  getStatusColor,
  getIconColor,
  formatTokens,
  formatCostDisplay,
  isPaused
} from '@/components/initiatives';

extractEmoji('🚀 Launch Feature');  // '🚀'
extractEmoji(undefined);             // '📋'

getStatusColor('active');            // 'green'
getStatusColor('completed');         // 'purple'

formatTokens(542000);                // '542K'
formatTokens(1500000);               // '1.5M'

formatCostDisplay(18.45);            // '$18.45'

isPaused('archived');                // true
isPaused('active');                  // false
```

**Accessibility:**
- `role="button"` with descriptive `aria-label` (title, status, progress)
- Keyboard navigation: Enter/Space triggers onClick
- Focus visible outline with primary color
- Progress bar has `role="progressbar"` with `aria-valuenow/min/max`
- Animations respect `prefers-reduced-motion`

## StatsRow

Horizontal stat card row for initiative dashboards. Displays key metrics with trends.

```tsx
import { StatsRow, defaultStats } from '@/components/initiatives';

// Basic usage
<StatsRow stats={{
  totalTasks: 42,
  completedTasks: 28,
  tokensUsed: 1500000,
  costSpent: 45.30,
  timeRemaining: '2d 4h'
}} />

// With trends
<StatsRow stats={{
  ...stats,
  taskTrend: 15,      // +15% from previous period
  tokenTrend: -5,     // -5% from previous period
}} />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `stats` | `InitiativeStats` | required | Stats object with values and trends |
| `className` | `string` | `''` | Additional CSS classes |

**Exported Utilities:**
```tsx
import { formatNumber, formatCost, formatPercentage, formatTrend } from '@/components/initiatives';

formatNumber(1500000);    // '1.5M'
formatCost(45.30);        // '$45.30'
formatPercentage(0.667);  // '66.7%'
formatTrend(15);          // '+15%'
formatTrend(-5);          // '-5%'
```

## InitiativesView

Container component assembling the complete initiatives overview page with aggregate statistics, cards grid, and state handling.

```tsx
import { InitiativesView } from '@/components/initiatives';

<InitiativesView />
<InitiativesView className="custom-class" />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `className` | `string` | `''` | Additional CSS classes |

**Visual Structure:**
- Header: "Initiatives" title, subtitle, "New Initiative" button
- StatsRow: 4 aggregate stat cards (Active Initiatives, Total Tasks, Completion Rate, Total Cost)
- Grid: Responsive InitiativeCard layout (auto-fill, min 360px)

**States:**
| State | Rendering |
|-------|-----------|
| Loading | Skeleton StatsRow + 4 skeleton cards |
| Empty | Icon + "Create your first initiative" message |
| Error | Error message + retry button |
| Populated | StatsRow + InitiativeCard grid |

**Data Sources:**
- Initiatives: Fetched from `/api/initiatives`
- Task progress: From `useTaskStore` (tasks, taskStates)

**Events:**
- "New Initiative" click: Dispatches `window.dispatchEvent(new CustomEvent('orc:new-initiative'))`
- Card click: Navigates to `/initiatives/{id}`

**Performance:**
- Single-pass task processing (O(n)) for stats computation
- Pre-computed task lookup map per initiative
- Memoized progress and stats calculations

## AgentsView

Container component assembling the complete agents configuration page with active agents grid, execution settings, and tool permissions sections.

```tsx
import { AgentsView } from '@/components/agents';

<AgentsView />
<AgentsView className="custom-class" />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `className` | `string` | `''` | Additional CSS classes |

**Visual Structure:**
- Header: "Agents" title, subtitle, "Add Agent" button
- Active Agents section: Responsive AgentCard grid (auto-fill, min 320px)
- Execution Settings section: ExecutionSettings component with model, limits config
- Tool Permissions section: ToolPermissions component with tool toggles

**States:**
| State | Rendering |
|-------|-----------|
| Loading | 3 skeleton cards in grid |
| Empty | Icon + "Create your first agent" message |
| Error | Error message + retry button |
| Populated | AgentCard grid + ExecutionSettings + ToolPermissions |

**Data Sources:**
- Agents: Fetched from `/api/agents` via `listAgents()`
- Config: Fetched from `/api/config` via `getConfig()`

**Events:**
- "Add Agent" click: Dispatches `window.dispatchEvent(new CustomEvent('orc:add-agent'))`
- Card click: Dispatches `orc:select-agent` custom event with agent data
- Settings change: Persists to API via `updateConfig()`

**Transformation:**
SubAgent API objects are transformed to display-friendly Agent objects:
- Icon color derived from index (purple, blue, green, amber rotation)
- Emoji assigned from preset list
- Stats populated from API response (tokens_today, tasks_done, success_rate) - defaults to zero if unavailable
- Status field reflects running task state ("active" or "idle")

## RunningCard

Expanded card component for actively executing tasks. Displays rich execution context including pipeline visualization, elapsed time, and live output.

```tsx
import { RunningCard } from '@/components/board';

// Basic usage
<RunningCard
  task={task}
  state={taskState}
/>

// With expand/collapse control
<RunningCard
  task={task}
  state={taskState}
  expanded={isExpanded}
  onToggleExpand={() => setIsExpanded(!isExpanded)}
  outputLines={transcriptLines}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `task` | `Task` | required | Task data object |
| `state` | `TaskState` | required | Current execution state |
| `expanded` | `boolean` | `false` | Whether output section is visible |
| `onToggleExpand` | `() => void` | - | Toggle expand callback |
| `outputLines` | `string[]` | `[]` | Raw output lines from transcript |
| `className` | `string` | `''` | Additional CSS classes |

**Visual Elements:**
- Task ID badge (monospace, `--primary-bright`)
- Title (2-line clamp with ellipsis)
- Initiative badge (when `task.initiative_id` set)
- Current phase name (uppercase, `--primary-bright`)
- Elapsed timer (MM:SS or H:MM:SS format, updates every second)
- Pipeline component showing phase progress
- Collapsible output section (max 50 lines)

**Output Line Types:**
| Pattern | Type | Color |
|---------|------|-------|
| Starts with `✓` or contains "success" | `success` | `--green` |
| Starts with `✗` or contains "error"/"fail" | `error` | `--red` |
| Starts with `→`/`◐` or contains "info" | `info` | `--primary-bright` |
| Default | `default` | `--text-secondary` |

**Phase Mapping:**
Internal phases map to display names for the Pipeline:
- `spec`, `design`, `research` → "Plan"
- `implement` → "Code"
- `test` → "Test"
- `review` → "Review"
- `docs`, `validate` → "Done"

**States:**
- Default: Gradient background (`--bg-card` to `--primary-dim`), glow shadow
- Hover: Enhanced glow, brighter border
- Expanded: Output section visible with scroll (100px max-height)
- Focus: Primary border with double glow ring

**Exported Utilities:**
```tsx
import { parseOutputLine, formatElapsedTime, mapPhaseToDisplay } from '@/components/board/RunningCard';

// Parse line for color coding
const { type, content } = parseOutputLine('✓ Tests passed');  // type: 'success'

// Format elapsed time
formatElapsedTime('2025-01-18T10:30:00Z');  // "5:23" or "1:05:23"

// Map internal phase to display name
mapPhaseToDisplay('implement');  // "Code"
```

**Accessibility:**
- `role="button"` with descriptive `aria-label` including task ID, title, phase, initiative
- `aria-expanded` reflects current state
- Keyboard navigation: Enter/Space toggles expand
- Focus visible outline with primary glow
- Expand toggle icon is `aria-hidden`

## TasksBarChart

Bar chart displaying tasks completed per day of the week (Mon-Sun).

```tsx
import { TasksBarChart, defaultWeekData } from '@/components/stats/TasksBarChart';

// Basic usage
<TasksBarChart
  data={[
    { day: 'Mon', count: 12 },
    { day: 'Tue', count: 18 },
    { day: 'Wed', count: 9 },
    { day: 'Thu', count: 24 },
    { day: 'Fri', count: 16 },
    { day: 'Sat', count: 6 },
    { day: 'Sun', count: 20 },
  ]}
/>

// Loading state
<TasksBarChart data={[]} loading />

// With default empty data
<TasksBarChart data={defaultWeekData} />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `data` | `DayData[]` | required | Array of `{ day: string; count: number }` |
| `loading` | `boolean` | `false` | Show skeleton loading state |
| `className` | `string` | `''` | Additional CSS classes |

**Visual Specifications:**
- Container: 160px height with flexbox layout
- Bars: Max 32px width, top border-radius only (4px), `--primary` color
- Labels: 9px font, `--text-muted` color, below each bar
- Height scaling: Proportional to max value in dataset
- Zero values: 4px minimum height for visibility

**States:**
- Default: Purple bars (`--primary`)
- Hover: Brighter purple (`--primary-bright`), shows tooltip with exact count
- Loading: Shimmer animation on 7 skeleton bars
- Empty: "No data available" centered message

**Exported Utilities:**
```tsx
import { calculateBarHeight, defaultWeekData, type DayData } from '@/components/stats/TasksBarChart';

// Calculate bar height (4px minimum, 140px maximum)
calculateBarHeight(count: number, maxCount: number): number

// Default week data with zero counts
defaultWeekData: DayData[]
```

**Accessibility:**
- `role="img"` with descriptive `aria-label` listing all values
- Loading state has `aria-busy="true"`
- Tooltip on hover shows exact count
- Respects `prefers-reduced-motion` (disables transitions and animations)

## OutcomesDonut

CSS-only donut chart for visualizing task outcomes (completed, with retries, failed) with centered total count and legend.

```tsx
import { OutcomesDonut } from '@/components/stats';

// Basic usage
<OutcomesDonut completed={232} withRetries={11} failed={4} />

// Single category (full circle)
<OutcomesDonut completed={50} withRetries={0} failed={0} />

// Empty state (no tasks)
<OutcomesDonut completed={0} withRetries={0} failed={0} />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `completed` | `number` | required | Number of successfully completed tasks |
| `withRetries` | `number` | required | Number of tasks completed with retries |
| `failed` | `number` | required | Number of failed tasks |

**Visual Elements:**
- 120px diameter donut chart with 80px inner hole
- Centered total count (JetBrains Mono, `--font-mono`)
- "Total" label below count
- Legend with colored dots and counts for each category

**Segment Colors:**
| Category | Color |
|----------|-------|
| Completed | `var(--green)` |
| With Retries | `var(--amber)` |
| Failed | `var(--red)` |

**Edge Cases:**
- All zeros: Shows neutral background (`var(--bg-surface)`)
- Single category: Full circle of that color (no gradient stops)
- Mixed values: Proportional conic-gradient segments

**Animations:** Smooth segment transitions via `transition: background var(--duration-slow)`.

**Implementation:** Uses CSS `conic-gradient` for rendering (no SVG or canvas). Inner hole created with `::after` pseudo-element over `--bg-card` background

## StatsView

Container component assembling the complete statistics page with summary stat cards, time filter controls, activity heatmap, charts, and leaderboard tables.

```tsx
import { StatsView } from '@/components/stats';

<StatsView />
<StatsView className="custom-class" />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `className` | `string` | `''` | Additional CSS classes |

**Visual Structure:**
- Header: "Statistics" title, subtitle, time filter buttons (24h | 7d | 30d | All), Export button
- Stats Grid: 5-column responsive grid with summary stat cards
- Activity Heatmap: Full-width card containing `ActivityHeatmap` component
- Charts Row: 2-column layout with `TasksBarChart` (2fr) + `OutcomesDonut` (1fr)
- Tables Row: 2-column layout with leaderboard tables (Most Active Initiatives + Most Modified Files)

**Stats Cards (5 metrics):**
| Card | Icon | Icon Color | Format |
|------|------|------------|--------|
| Tasks Completed | check-circle | purple | Raw number |
| Tokens Used | zap | amber | "2.4M" format |
| Total Cost | dollar | green | "$47.82" format |
| Avg Task Time | clock | blue | "3:24" (mm:ss) |
| Success Rate | shield | green | "94.2%" format |

Each card shows: label, colored icon, monospace value, % change indicator (green up/red down arrow).

**Time Filter:**
Tab-style buttons: 24h, 7d, 30d, All. Default is 7d. Triggers `statsStore.setPeriod()` and refetch on change.

**Export:**
Export button generates CSV with current period data (date, tasks_completed, tokens_used, cost, success_rate). Uses Blob + URL.createObjectURL pattern.

**States:**
| State | Rendering |
|-------|-----------|
| Loading | Skeleton placeholders for all sections (cards, heatmap, charts, tables) |
| Empty | Icon + "No statistics yet" message + description |
| Error | Error message with retry button |
| Populated | Full layout with all visualizations |

**Data Sources:**
- All data from `statsStore` via hooks: `useStatsPeriod`, `useStatsLoading`, `useStatsError`, `useActivityData`, `useOutcomes`, `useTasksPerDay`, `useTopInitiatives`, `useTopFiles`, `useSummaryStats`, `useWeeklyChanges`
- Period changes call `statsStore.setPeriod()` which triggers refetch

**Exported Utilities:**
```tsx
// Internal utility functions (not exported, but documented for reference)
formatTokens(tokens: number): string    // "2.4M", "1.5K", "847"
formatCost(cost: number): string        // "$47.82"
formatTime(seconds: number): string     // "3:24"
formatRate(rate: number): string        // "94.2%"
generateCSV(tasksPerDay, summaryStats): string
downloadCSV(content: string, filename: string): void
```

**CSS Classes:**
| Class | Purpose |
|-------|---------|
| `.stats-view` | Main container |
| `.stats-view-header` | Page header with title/filter/export |
| `.stats-view-content` | Scrollable content area |
| `.stats-view-stats-grid` | 5-column responsive grid for stat cards |
| `.stats-view-stat-card` | Individual stat card |
| `.stats-view-section-card` | Card wrapper for sections |
| `.stats-view-charts-row` | 2-column layout for charts |
| `.stats-view-tables-row` | 2-column layout for leaderboards |
| `.stats-view-time-filter` | Time filter button group |
| `.stats-view-empty` | Empty state styling |
| `.stats-view-error` | Error state with retry |

**Accessibility:**
- Time filter uses `role="tablist"` with `aria-selected` on buttons
- Loading state has `aria-busy="true"` and `aria-label="Loading statistics"`
- Error state has `role="alert"`
- Empty state has `role="status"`
- Export button disabled during loading or when no data

## CommandList

List component displaying slash commands organized by scope (project/global). Each command shows an icon, name, description, and action buttons for editing and deleting.

```tsx
import { CommandList, type Command } from '@/components/settings';

// Basic usage
<CommandList
  commands={commands}
  selectedId={selectedCommandId}
  onSelect={(id) => setSelectedCommandId(id)}
  onDelete={(id) => deleteCommand(id)}
/>

// With mixed scopes
const commands: Command[] = [
  { id: '1', name: '/review', description: 'Run code review', scope: 'project' },
  { id: '2', name: '/deploy', description: 'Deploy to production', scope: 'global' },
];

<CommandList
  commands={commands}
  selectedId="1"
  onSelect={handleSelect}
  onDelete={handleDelete}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `commands` | `Command[]` | required | Array of command objects |
| `selectedId` | `string` | - | ID of currently selected command |
| `onSelect` | `(id: string) => void` | required | Selection handler |
| `onDelete` | `(id: string) => void` | required | Delete handler (called after confirmation) |

**Command Interface:**
```tsx
interface Command {
  id: string;
  name: string;          // e.g., '/review'
  description: string;
  scope: 'project' | 'global';
  path?: string;         // Optional file path
}
```

**Visual Structure:**
- **Project Commands section:** Header with title and description about `.claude/commands/`
- **Global Commands section:** Header with title and description about `~/.claude/commands/`
- **Command items:** 32px icon, monospace name, truncated description, edit/delete buttons

**Icon Colors:**
| Scope | Icon Background | Icon Color |
|-------|-----------------|------------|
| `project` | `var(--primary-dim)` | `var(--primary)` |
| `global` | `var(--cyan-dim)` | `var(--cyan)` |

**States:**
- Default: `--bg-surface` background, 1px `--border`, 8px radius
- Hover: Border lightens to `--border-light`
- Selected: Border changes to `--primary-border`
- Delete confirmation: Shows confirm/cancel buttons instead of edit/delete

**Empty State:**
When `commands` is empty, displays centered message:
- Terminal icon (32px)
- "No commands" title
- "Create a command to get started" description

**Delete Confirmation:**
Clicking delete shows inline confirm/cancel buttons:
- Confirm (checkmark): Calls `onDelete` with command ID
- Cancel (X): Returns to normal edit/delete buttons
- Escape key also cancels

**Accessibility:**
- Items have `role="button"` with `tabIndex={0}`
- `aria-pressed` reflects selection state
- Keyboard: Enter/Space to select item
- Confirm/cancel buttons are keyboard accessible
- All buttons have `aria-label` for screen readers

## CommandEditor

Markdown editor component for editing slash command files. Features syntax highlighting overlay, line numbers, dirty state tracking, and keyboard shortcuts.

```tsx
import { CommandEditor, type EditableCommand } from '@/components/settings';

const command: EditableCommand = {
  id: 'cmd-1',
  name: '/review-pr',
  path: '.claude/commands/review-pr.md',
  content: '# Review PR\n\nRun a comprehensive code review.',
};

<CommandEditor
  command={command}
  onSave={async (content) => {
    await saveCommand(command.id, content);
  }}
  onCancel={() => setEditing(false)}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `command` | `EditableCommand` | required | Command object with id, name, path, content |
| `onSave` | `(content: string) => Promise<void>` | required | Async save handler called with current content |
| `onCancel` | `() => void` | required | Cancel handler (no save) |

**EditableCommand Interface:**
```tsx
interface EditableCommand {
  id: string;
  name: string;          // e.g., '/review-pr'
  path: string;          // e.g., '.claude/commands/review-pr.md'
  content: string;       // Markdown content
}
```

**Visual Structure:**
- **Header:** Command name (monospace with terminal icon), file path, dirty indicator, Cancel/Save buttons
- **Body:** Line numbers column (40px, right-aligned) + content area with syntax highlighting overlay

**Syntax Highlighting:**
| Element | CSS Class | Color |
|---------|-----------|-------|
| Headers (`#`, `##`, etc.) | `.md-header` | `--primary-bright`, semibold |
| Inline code (backticks) | `.md-code` | `--cyan` with `--bg-card` background |
| Code blocks (triple backticks) | `.md-code-block` | `--text-secondary` with `--bg-card` background |
| Lists (`-`, `*`, numbered) | `.md-list` | `--orange` |
| Bold (`**text**`) | `.md-bold` | `--text-primary`, bold weight |
| Italic (`*text*`) | `.md-italic` | `--text-secondary`, italic |

**Keyboard Shortcuts:**
| Shortcut | Action |
|----------|--------|
| `Ctrl+S` / `Cmd+S` | Save command |
| `Escape` | Cancel editing |

**States:**
- Default: `--bg-surface` background, 1px `--border`
- Dirty: Amber "Unsaved" badge in header
- Saving: Save button shows loading spinner, disabled
- Error: Red error banner below header with `role="alert"`
- Focus: `--primary-border` with `--primary-glow` shadow

**Implementation Details:**
- Uses textarea with transparent text + pre element overlay for syntax highlighting
- Line numbers sync with content via line count calculation
- Scroll sync between textarea and highlight overlay
- Content height auto-adjusts (300px min, 600px max)
- HTML escaped before highlighting to prevent XSS

**Accessibility:**
- Textarea has `aria-label="Edit {command.name}"`
- Save/Cancel buttons have descriptive `aria-label`
- Error message has `role="alert"`
- Dirty indicator has `aria-label="Unsaved changes"`
- Respects `prefers-reduced-motion`

## ConfigEditor

Editable code/config file viewer with syntax highlighting. Supports markdown, YAML, and JSON with save functionality and unsaved changes detection.

```tsx
import { ConfigEditor, type ConfigLanguage } from '@/components/settings';

// Basic markdown editor
<ConfigEditor
  filePath="CLAUDE.md"
  content={fileContent}
  onChange={(content) => setFileContent(content)}
  onSave={() => saveFile()}
/>

// YAML configuration
<ConfigEditor
  filePath=".orc/config.yaml"
  content={yamlContent}
  onChange={setYamlContent}
  onSave={handleSave}
  language="yaml"
/>

// JSON file
<ConfigEditor
  filePath="package.json"
  content={jsonContent}
  onChange={setJsonContent}
  onSave={handleSave}
  language="json"
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `filePath` | `string` | required | File path displayed in header (monospace) |
| `content` | `string` | required | Current file content |
| `onChange` | `(content: string) => void` | required | Called on every content change |
| `onSave` | `() => void` | required | Called when Save button clicked or Ctrl+S pressed |
| `language` | `'markdown' \| 'yaml' \| 'json'` | `'markdown'` | Syntax highlighting language |

**Visual Structure:**
- Header: File path (monospace), unsaved indicator (amber badge), Save button
- Content: Overlay-based editor (hidden textarea + highlighted div)
- Dimensions: 200px min-height, 400px max-height, scrollable

**Syntax Highlighting Colors:**
| Element | Class | Color |
|---------|-------|-------|
| Comments | `.code-comment` | `var(--text-muted)` |
| Headers/Keys | `.code-key` | `var(--primary-bright)` |
| Strings | `.code-string` | `var(--green)` |

**Language-Specific Patterns:**

*Markdown:*
- Headers (`## Title`) → `.code-key`
- Code fences (``` ` ` ` ```) → `.code-string`
- Single `#` lines (not `##`) → `.code-comment`

*YAML:*
- Comments (`# comment`) → `.code-comment`
- Keys (`key:`) → `.code-key`
- Quoted strings → `.code-string`

*JSON:*
- Object keys (`"key":`) → `.code-key`
- String values → `.code-string`

**Keyboard Shortcuts:**
| Key | Action |
|-----|--------|
| `Ctrl+S` / `Cmd+S` | Trigger `onSave` callback |
| `Tab` | Insert tab character at cursor |

**States:**
- Default: `--bg-surface` background, 1px `--border`, `--radius-lg`
- Focus: Border changes to `--primary`, 2px glow ring
- Unsaved: Amber "Modified" badge appears in header

**Implementation Details:**
- Uses overlay technique: transparent textarea over highlighted div
- Scroll positions are synchronized between layers
- Cursor (caret) uses `--text-primary` color
- Selection background uses `--selection-bg`
- Memoized syntax highlighting to prevent unnecessary recomputation

**Exported Types:**
```tsx
type ConfigLanguage = 'markdown' | 'yaml' | 'json';

interface ConfigEditorProps {
  filePath: string;
  content: string;
  onChange: (content: string) => void;
  onSave: () => void;
  language?: ConfigLanguage;
}
```

**Accessibility:**
- Textarea has `aria-label` describing the file being edited
- Save button has `aria-label="Save changes"`
- Unsaved indicator has `aria-label="Unsaved changes"`
- Highlighted layer is `aria-hidden="true"`
- `spellCheck={false}` to avoid red underlines in code
- Respects `prefers-reduced-motion` (disables transitions)

## SettingsLayout

Layout component for settings pages with 240px sidebar and content area.

```tsx
import { SettingsLayout } from '@/components/settings';

// Used as route element in SettingsPage
<SettingsLayout />
```

**Visual Structure:**
- CSS Grid: 240px sidebar + 1fr content
- Sidebar: Fixed width, scrollable, grouped navigation
- Content: Outlet for child routes, padded, scrollable

**Sidebar Header:**
- Title: "Settings" (14px, semibold)
- Subtitle: "Configure ORC and Claude" (11px, muted)
- Border-bottom separator

**Navigation Groups:**
| Group | Items |
|-------|-------|
| CLAUDE CODE | Slash Commands, CLAUDE.md, MCP Servers, Memory, Permissions |
| ORC | Projects, Billing & Usage, Import / Export |
| ACCOUNT | Profile, API Keys |

**NavItem Props:**
| Prop | Type | Description |
|------|------|-------------|
| `to` | `string` | Route path |
| `icon` | `IconName` | Icon name |
| `label` | `string` | Display text |
| `badge` | `number` | Optional count badge |

**CSS Variables Used:**
- Sidebar background: `--bg-elevated`
- Border: `--border`
- Nav item hover: `--bg-surface`, `--text-primary`
- Nav item active: `--primary-dim`, `--primary-bright`
- Badge default: `--bg-surface`, `--text-muted`
- Badge active: `rgba(168, 85, 247, 0.2)`, `--primary`

**Accessibility:**
- `role="navigation"` on sidebar with `aria-label="Settings navigation"`
- NavLink provides automatic `aria-current` for active items
- Keyboard navigable (Tab through items)

## SettingsView

Container component for the Slash Commands settings section.

```tsx
import { SettingsView } from '@/components/settings';

// Used as element for /settings/commands route
<SettingsView />
```

**Visual Structure:**
- Header: Title, subtitle, "New Command" primary button
- Content: Split view with CommandList (left) and ConfigEditor (right)

**State Management:**
| State | Type | Purpose |
|-------|------|---------|
| `commands` | `Command[]` | List of slash commands |
| `selectedId` | `string \| undefined` | Currently selected command ID |
| `editorContent` | `string` | Content in the config editor |

**Data Flow:**
Currently uses mock data. Will integrate with API endpoints when available.

**Events:**
| Action | Handler |
|--------|---------|
| Select command | `handleSelect` - Updates selection, loads content |
| Delete command | `handleDelete` - Removes from list, selects next |
| Edit content | `handleContentChange` - Updates editor state |
| Save | `handleSave` - TODO: API integration |
| New Command | `handleNewCommand` - TODO: Modal integration |

**Empty State:**
When no command is selected, shows placeholder with terminal icon and "Select a command to edit" message.

**CSS Classes:**
| Class | Purpose |
|-------|---------|
| `.settings-view` | Main container |
| `.settings-view__header` | Page header with title/button |
| `.settings-view__content` | Split content area |
| `.settings-view__list` | CommandList wrapper |
| `.settings-view__editor` | ConfigEditor wrapper |
| `.settings-view__empty` | Empty state placeholder |

## SettingsPlaceholder

Placeholder component for unimplemented settings sections.

```tsx
import { SettingsPlaceholder } from '@/components/settings';

<SettingsPlaceholder
  title="CLAUDE.md"
  description="Edit your project's CLAUDE.md instructions file"
  icon="file-text"
/>
```

| Prop | Type | Description |
|------|------|-------------|
| `title` | `string` | Section title |
| `description` | `string` | Descriptive text |
| `icon` | `IconName` | Icon to display |

**Visual Structure:**
- Centered layout with icon, title, and description
- "Coming Soon" badge
- Matches settings content area styling

**Usage:**
Used as placeholder elements for settings routes that haven't been implemented yet (claude-md, mcp, memory, permissions, projects, billing, import-export, profile, api-keys).

## BoardView

Main container component for the two-column board layout with right panel integration.

```tsx
import { BoardView } from '@/components/board';

<BoardView />
<BoardView className="custom-class" />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `className` | `string` | `''` | Additional CSS classes |

**Visual Structure:**
- Two-column CSS grid: Queue (flex: 1, min 280px) | Running (420px fixed)
- Queue column renders QueueColumn with initiative swimlanes
- Running column renders RunningColumn with Pipeline visualization
- Sets right panel content via AppShell context on mount

**Data Flow:**
| Store | Data | Usage |
|-------|------|-------|
| `taskStore.tasks` | All tasks | Filtered by status for columns |
| `taskStore.taskStates` | Execution states | Passed to RunningColumn |
| `initiativeStore` | Initiatives | Swimlane grouping |
| `sessionStore` | totalTokens, totalCost | CompletedPanel stats |

**Derived State:**
- `queuedTasks`: status in ['planned', 'created', 'classifying']
- `runningTasks`: status === 'running'
- `blockedTasks`: status === 'blocked' or is_blocked === true
- `completedToday`: status === 'completed' and completed_at is today

**Right Panel Content:**
On mount, sets AppShell right panel with:
- `BlockedPanel` (orange theme)
- `DecisionsPanel` (purple theme)
- `ConfigPanel` (cyan theme)
- `FilesPanel` (blue theme)
- `CompletedPanel` (green theme)

Clears right panel content on unmount.

**States:**
| State | Rendering |
|-------|-----------|
| Loading | Skeleton layout with placeholder cards |
| Populated | QueueColumn + RunningColumn |

**Accessibility:**
- `role="region"` with `aria-label="Task board"`
- Child columns have appropriate ARIA labels

## BlockedPanel

Right panel section displaying blocked tasks with skip/force actions.

```tsx
import { BlockedPanel } from '@/components/board';

<BlockedPanel
  tasks={blockedTasks}
  onSkip={(taskId) => skipBlock(taskId)}
  onForce={(taskId) => forceRun(taskId)}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `tasks` | `Task[]` | required | Blocked tasks to display |
| `onSkip` | `(taskId: string) => void` | required | Called when Skip button clicked |
| `onForce` | `(taskId: string) => void` | required | Called when Force confirmed |

**Visual Structure:**
- Orange-themed collapsible header with blocked icon and count badge
- Each blocked task shows: ID (monospace), title (truncated), blocking reason
- Action buttons: Skip (bypass block), Force (run with confirmation modal)

**Blocking Reason Display:**
- Single blocker: "Waiting for `TASK-XXX`" with code formatting
- Multiple blockers: Bulleted list with code formatting for task IDs
- Unknown: "Unknown blocker"

**Force Confirmation:**
Modal with warning message about running despite incomplete dependencies.

**Visibility:**
Hidden when `tasks.length === 0`.

**Accessibility:**
- Header has `aria-expanded` and `aria-controls`
- Action buttons have descriptive `aria-label`
- Count badge has `aria-label="{n} blocked tasks"`

## DecisionsPanel

Right panel section displaying pending decisions from running tasks.

```tsx
import { DecisionsPanel } from '@/components/board';

<DecisionsPanel
  decisions={pendingDecisions}
  onDecide={(decisionId, optionId) => submitDecision(decisionId, optionId)}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `decisions` | `PendingDecision[]` | required | Pending decisions array |
| `onDecide` | `(decisionId: string, optionId: string) => void` | required | Called when option selected |

**Visual Structure:**
- Purple-themed collapsible header with decision icon and count badge
- Each decision shows: task ID, question text, option buttons
- Recommended option highlighted (first option or explicitly marked)

**Decision Option Styling:**
| State | Variant |
|-------|---------|
| Recommended | `primary` button |
| Other | `ghost` button |

**Loading State:**
While submitting, decision item shows `aria-busy="true"` and buttons disabled.

**Visibility:**
Hidden when `decisions.length === 0`.

**Accessibility:**
- Uses RightPanel.Section compound component
- Option buttons have `aria-label` with recommendation status
- Items have `aria-busy` during submission

## ConfigPanel

Right panel section displaying Claude Code configuration quick links.

```tsx
import { ConfigPanel, type ConfigStats } from '@/components/board';

<ConfigPanel config={{
  slashCommandsCount: 5,
  claudeMdSize: 2048,
  mcpServersCount: 3,
  permissionsProfile: 'Auto'
}} />
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `config` | `ConfigStats` | - | Stats for badge display |

**ConfigStats Interface:**
```tsx
interface ConfigStats {
  slashCommandsCount?: number;  // Badge shows count
  claudeMdSize?: number;        // Badge shows "2.0K" format
  mcpServersCount?: number;     // Badge shows count
  permissionsProfile?: string;  // Badge shows profile name
  loading?: boolean;            // Shows skeleton badges
}
```

**Links:**
| Link | Route | Badge |
|------|-------|-------|
| Slash Commands | `/settings/advanced/skills` | Count |
| CLAUDE.md | `/settings/advanced/claudemd` | File size |
| MCP Servers | `/settings/advanced/mcp` | Count |
| Permissions | `/settings/configuration/general` | Profile |

**Visual Structure:**
- Cyan-themed collapsible header with code icon
- Each link: icon, title, description, badge, arrow chevron
- Clicking navigates to the route

**Visibility:**
Always visible (no empty state).

**Accessibility:**
- Header has `aria-expanded` and `aria-controls`
- Links have `aria-label` with title and description
- Loading badges have `aria-label="Loading"`

## FilesPanel

Right panel section displaying files changed by running tasks.

```tsx
import { FilesPanel, type ChangedFile } from '@/components/board';

<FilesPanel
  files={[
    { path: 'src/App.tsx', status: 'modified', taskId: 'TASK-001' },
    { path: 'src/utils/helper.ts', status: 'added', taskId: 'TASK-001' },
  ]}
  onFileClick={(file) => openFile(file)}
  maxVisible={5}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `files` | `ChangedFile[]` | required | Changed files to display |
| `onFileClick` | `(file: ChangedFile) => void` | required | Called when file clicked |
| `maxVisible` | `number` | `5` | Max files before "more" link |
| `onShowMore` | `() => void` | - | Called when "more" clicked |

**ChangedFile Interface:**
```tsx
interface ChangedFile {
  path: string;
  status: 'modified' | 'added' | 'deleted' | 'renamed';
  binary?: boolean;
  taskId?: string;
}
```

**Visual Structure:**
- Blue-themed collapsible header with file icon and count badge
- Each file: icon (file/image), path (monospace), status badge
- Files grouped by task if multiple tasks running

**Status Badge Colors:**
| Status | Badge | Color |
|--------|-------|-------|
| Modified | M | Amber |
| Added | A | Green |
| Deleted | D | Red |
| Renamed | R | Cyan |

**Binary Detection:**
Auto-detects binary files by extension (images, fonts, archives, etc.).

**Visibility:**
Hidden when `files.length === 0`.

**Accessibility:**
- File items have `role="button"` with descriptive `aria-label`
- Keyboard navigation: Enter/Space triggers click
- "More" button has `aria-label="{n} more files"`

## CompletedPanel

Right panel section displaying completed tasks summary with token/cost stats.

```tsx
import { CompletedPanel, formatTokenCount, formatCost } from '@/components/board';

<CompletedPanel
  completedCount={5}
  todayTokens={847000}
  todayCost={2.34}
  recentTasks={completedTasks}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `completedCount` | `number` | required | Tasks completed today |
| `todayTokens` | `number` | required | Total tokens used today |
| `todayCost` | `number` | required | Total cost today in dollars |
| `recentTasks` | `Task[]` | `[]` | Recent completed tasks for expanded list |

**Visual Structure:**
- Green-themed compact header with checkmark icon and count badge
- Collapsed: Shows count badge only
- Expanded: Shows token/cost stats and task list

**Exported Utilities:**
```tsx
// Format token count with K/M suffix
formatTokenCount(847000);  // "847K"
formatTokenCount(1500000); // "1.5M"

// Format cost as currency
formatCost(2.34);          // "$2.34"
```

**States:**
| State | Rendering |
|-------|-----------|
| Empty (count=0) | "No tasks completed today" message |
| Collapsed | Count badge only |
| Expanded | Stats + task list (when recentTasks provided) |

**Visibility:**
Always visible (shows empty message when count is 0).

**Accessibility:**
- Header has `aria-expanded` (when expandable) and `aria-controls`
- Header has `aria-label` with full summary text
- Disabled when no expandable content
