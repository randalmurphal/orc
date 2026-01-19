# Page Components

Detailed documentation for page-level components.

## Dashboard

Main overview page at `/dashboard`.

### Components

| Component | Purpose |
|-----------|---------|
| `DashboardStats` | Running, blocked, today, tokens cards |
| `DashboardActiveTasks` | Running/paused/blocked task list (max 5) |
| `DashboardQuickActions` | New task and view all buttons |
| `DashboardRecentActivity` | Recently completed tasks (max 5) |
| `DashboardInitiatives` | Active initiatives with progress bars |
| `DashboardSummary` | Total/completed/failed counts |

**Data flow:** Stats from `/api/dashboard/stats`, tasks from TaskStore, initiatives from InitiativeStore.

## Board

Kanban board at `/board` with two view modes.

### View Modes

| Mode | Description |
|------|-------------|
| Flat | Traditional columns: Todo, Spec, Implement, Test, Docs, Validate, Done |
| By Initiative | Horizontal swimlanes grouping tasks by initiative |

Toggle via `ViewModeDropdown`, persisted in localStorage.

### Components

| Component | Purpose |
|-----------|---------|
| `Board` | Container managing view mode and columns |
| `Column` | Single status column with task cards |
| `QueuedColumn` | Column with active/backlog sections |
| `Swimlane` | Initiative row in swimlane view |
| `TaskCard` | Clickable task card with status/priority |
| `Pipeline` | Horizontal phase progress visualization (4px bars, 5 phases) |
| `InitiativeDropdown` | Filter by initiative |
| `ViewModeDropdown` | Flat/swimlane toggle |

### TaskCard Behavior

- Click navigates to task detail (parent handles via `onClick` callback)
- Right-click triggers context menu (parent handles via `onContextMenu` callback)
- Priority dot with color coding (critical: red, high: orange, normal: blue, low: muted)
- Category icon indicates task type (feature, bug, refactor, chore, docs, test)
- Blocked tasks show warning icon with pulse animation
- Running tasks show animated progress indicator
- Keyboard accessible: Enter/Space triggers click, focus-visible outline

## Task Detail

Task detail page at `/tasks/:id` with tabbed interface.

### Tabs

| Tab | Component | Content |
|-----|-----------|---------|
| Timeline | `TimelineTab` | Phase execution timeline with tokens |
| Changes | `ChangesTab` | Git diff viewer with inline comments |
| Transcript | `TranscriptTab` | Full transcript history |
| Tests | `TestResultsTab` | Test results and screenshots |
| Attachments | `AttachmentsTab` | Uploaded files |
| Comments | `CommentsTab` | Task discussion |

### Layout

- `TaskHeader`: Title, status, actions (edit, run, pause, delete)
- `TabNav`: Tab switching (Radix Tabs)
- `DependencySidebar`: Collapsible panel showing blocked_by, blocks, related_to

### TaskEditModal

Edit form with fields: title, description, weight, category, priority, queue, initiative.

### ExportDropdown

Export options: JSON, YAML, Markdown. Uses Radix DropdownMenu.

## Initiative Detail

Initiative management at `/initiatives/:id`.

### Sections

- **Overview**: Title, description, status, progress bar
- **Tasks**: Linked tasks with add/remove
- **Decisions**: Decision log with rationale
- **Graph**: Dependency graph visualization (Kahn's algorithm)

### Status Flow

draft -> active -> completed -> archived

## Environment Pages

Settings pages under `/environment/*`.

### Pattern

All environment pages follow the same structure:
1. Load data from API endpoint
2. Display in editable form
3. Save changes on blur or submit
4. Toast notification on success/error

### Pages

| Route | Purpose |
|-------|---------|
| `/environment` | Overview |
| `/environment/claude/settings` | Claude Code settings |
| `/environment/claude/skills` | Custom skills |
| `/environment/claude/hooks` | Git hooks |
| `/environment/claude/agents` | AI agents |
| `/environment/claude/tools` | Tool configuration |
| `/environment/claude/mcp` | MCP servers |
| `/environment/claude/prompts` | System prompts |
| `/environment/orchestrator/automation` | Orc config |
| `/environment/orchestrator/scripts` | Custom scripts |

## Layout Components

### AppLayout

Root layout with Sidebar + Header + content area.

Handles: Global shortcuts, modal states, responsive sidebar margin.

### IconNav

56px icon-based navigation sidebar. Compact vertical navigation with icons and small labels.

**Structure:**
- Logo section with gradient "O" mark (32x32px)
- Main nav: Board, Initiatives, Stats (with divider)
- Secondary nav: Agents, Settings
- Bottom section: Help

**Features:**
- Active state detection via React Router NavLink
- Nested route support (e.g., `/settings/*` activates Settings)
- Tooltips on hover with full descriptions
- Accessibility: `role="navigation"`, `aria-label="Main navigation"`

**Navigation Routes:**

| Item | Icon | Route |
|------|------|-------|
| Board | board | `/board` |
| Initiatives | layers | `/initiatives` |
| Stats | bar-chart | `/stats` |
| Agents | robot | `/agents` |
| Settings | settings | `/settings` |
| Help | help | `/help` |

### Sidebar

Left navigation with sections: Work, Initiatives, Environment, Preferences.

Features: Collapse toggle, initiative filtering, active route highlighting.

### Header

Top bar with project selector, page title, command palette button, new task button.

Mobile: Hamburger menu for sidebar overlay.

### Mobile Responsive

Breakpoint at 768px:
- Mobile: Sidebar as overlay, hamburger menu
- Desktop: Persistent sidebar, collapse toggle
