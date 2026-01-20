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

## Initiatives

Initiatives overview page at `/initiatives` with aggregate statistics and card grid.

### Page Structure

| Component | Purpose |
|-----------|---------|
| `InitiativesPage` | Route wrapper that renders InitiativesView |
| `InitiativesView` | Container with data fetching, stats, and cards |

### Visual Sections

| Section | Content |
|---------|---------|
| Header | "Initiatives" title, "Manage your project epics and milestones" subtitle, "New Initiative" button |
| StatsRow | Aggregate metrics (Active Initiatives, Total Tasks, Completion Rate, Total Cost) |
| Grid | Responsive InitiativeCard grid (auto-fill, min 360px, gap 16px) |

### States

| State | Display |
|-------|---------|
| Loading | Skeleton StatsRow + 4 skeleton cards in grid |
| Empty | Icon, "Create your first initiative" title, descriptive text |
| Error | Error message with retry button |
| Populated | StatsRow + InitiativeCard grid |

### Data Flow

- **Initiatives**: Fetched from `/api/initiatives` via `listInitiatives()`
- **Task Stats**: From `useTaskStore` for progress and cost calculations
- **Progress per Card**: Computed from tasks with matching `initiative_id`

### Stats Calculation

| Metric | Source |
|--------|--------|
| Active Initiatives | `initiatives.filter(i => i.status === 'active').length` |
| Total Tasks | Tasks with `initiative_id` set |
| Completion Rate | `completedTasks / totalTasks * 100` |
| Total Cost | Sum of token costs from `taskStates` for linked tasks |

### Events

| Action | Event/Navigation |
|--------|------------------|
| Click "New Initiative" | Dispatches `orc:new-initiative` custom event |
| Click initiative card | Navigates to `/initiatives/{id}` |

### CSS Classes

| Class | Purpose |
|-------|---------|
| `.initiatives-view` | Main container |
| `.initiatives-view-header` | Page header with title/subtitle/button |
| `.initiatives-view-content` | Scrollable content area (padding: 20px) |
| `.initiatives-view-grid` | Responsive grid (auto-fill, minmax(360px, 1fr), gap: 16px) |
| `.initiatives-view-empty` | Empty state styling |
| `.initiatives-view-error` | Error state with retry |

## Agents

Agent configuration page at `/agents` with active agents grid, execution settings, and tool permissions.

### Page Structure

| Component | Purpose |
|-----------|---------|
| `AgentsPage` | Route wrapper that renders AgentsView |
| `AgentsView` | Container with data fetching, sections, and state handling |

### Visual Sections

| Section | Content |
|---------|---------|
| Header | "Agents" title, "Configure Claude models and execution settings" subtitle, "Add Agent" button |
| Active Agents | Responsive AgentCard grid (auto-fill, min 320px, gap 16px) |
| Execution Settings | 2-column grid with parallel tasks, auto-approve, model, cost limit |
| Tool Permissions | 3-column grid of tool toggles (file read/write, bash, web, git, MCP) |

### States

| State | Display |
|-------|---------|
| Loading | Skeleton cards (3) in grid |
| Empty | Icon + "Create your first agent" message |
| Error | Error message with retry button |
| Populated | AgentCard grid + ExecutionSettings + ToolPermissions |

### Data Flow

- **Agents**: Fetched from `/api/agents` via `listAgents()`
- **Config**: Fetched from `/api/config` via `getConfig()`
- **Execution Settings**: Derived from config (model, automation profile)
- **Tool Permissions**: Local state with toggle persistence

### Events

| Action | Event/Effect |
|--------|--------------|
| Click "Add Agent" | Dispatches `orc:add-agent` custom event |
| Click agent card | Dispatches `orc:select-agent` custom event with agent data |
| Change execution setting | Updates local state, persists model via `updateConfig()` |
| Toggle tool permission | Updates local state |

### CSS Classes

| Class | Purpose |
|-------|---------|
| `.agents-view` | Main container |
| `.agents-view-header` | Page header with title/subtitle/button |
| `.agents-view-content` | Scrollable content area (padding: 20px) |
| `.agents-view-section` | Section wrapper (margin-bottom: 32px) |
| `.agents-view-grid` | Responsive grid (auto-fill, minmax(320px, 1fr), gap: 16px) |
| `.agents-view-empty` | Empty state styling |
| `.agents-view-error` | Error state with retry |

## Stats

Statistics overview page at `/stats` with comprehensive metrics visualization.

### Page Structure

| Component | Purpose |
|-----------|---------|
| `StatsPage` | Route wrapper that renders StatsView |
| `StatsView` | Container with data fetching, time filter, and all visualizations |

### Visual Sections

| Section | Content |
|---------|---------|
| Header | "Statistics" title, "Token usage, costs, and task metrics" subtitle, time filter (24h/7d/30d/All), Export button |
| Stats Grid | 5 summary stat cards (Tasks Completed, Tokens Used, Total Cost, Avg Task Time, Success Rate) |
| Activity Heatmap | Full-width `ActivityHeatmap` showing task completion patterns |
| Charts Row | `TasksBarChart` (2fr) + `OutcomesDonut` (1fr) side by side |
| Tables Row | Two `LeaderboardTable` components (Most Active Initiatives, Most Modified Files) |

### States

| State | Display |
|-------|---------|
| Loading | Skeleton placeholders for all sections |
| Empty | Icon, "No statistics yet" title, descriptive text |
| Error | Error message with retry button |
| Populated | Full layout with stat cards + heatmap + charts + leaderboards |

### Data Flow

- **Stats Data**: Fetched from `statsStore` via hooks
- **Period**: Managed by `statsStore.setPeriod()`, triggers refetch
- **Export**: Generates CSV with current period data

### Time Filter

| Period | Description |
|--------|-------------|
| 24h | Last 24 hours |
| 7d | Last 7 days (default) |
| 30d | Last 30 days |
| All | All time |

### Events

| Action | Event/Effect |
|--------|--------------|
| Click time filter button | `setPeriod()` → refetch stats |
| Click Export | Download CSV file with current data |
| Click Retry (error state) | `fetchStats()` with current period |

### CSS Classes

| Class | Purpose |
|-------|---------|
| `.stats-view` | Main container |
| `.stats-view-header` | Page header with title/filter/export |
| `.stats-view-content` | Scrollable content area |
| `.stats-view-stats-grid` | 5-column responsive grid for stat cards |
| `.stats-view-charts-row` | 2-column layout for charts (2fr + 1fr) |
| `.stats-view-tables-row` | 2-column layout for leaderboards |

## Initiative Detail

Initiative management at `/initiatives/:id`.

### Sections

- **Overview**: Title, description, status, progress bar
- **Tasks**: Linked tasks with add/remove
- **Decisions**: Decision log with rationale
- **Graph**: Dependency graph visualization (Kahn's algorithm)

### Status Flow

draft -> active -> completed -> archived

## Settings

Settings page at `/settings` with dedicated sidebar layout.

### Route Structure

| Route | Component | Content |
|-------|-----------|---------|
| `/settings` | SettingsPage | Redirects to `/settings/commands` |
| `/settings/commands` | SettingsView | Slash commands editor (CommandList + ConfigEditor) |
| `/settings/claude-md` | SettingsPlaceholder | CLAUDE.md editor (placeholder) |
| `/settings/mcp` | SettingsPlaceholder | MCP servers (placeholder) |
| `/settings/memory` | SettingsPlaceholder | Memory management (placeholder) |
| `/settings/permissions` | SettingsPlaceholder | Permissions (placeholder) |
| `/settings/projects` | SettingsPlaceholder | Projects (placeholder) |
| `/settings/billing` | SettingsPlaceholder | Billing & Usage (placeholder) |
| `/settings/import-export` | SettingsPlaceholder | Import / Export (placeholder) |
| `/settings/profile` | SettingsPlaceholder | Profile (placeholder) |
| `/settings/api-keys` | SettingsPlaceholder | API Keys (placeholder) |
| `/settings/*` | NotFoundPage | Unknown paths |

### Layout Structure

```
SettingsPage
└── SettingsLayout
    ├── SettingsSidebar (240px)
    │   ├── Header: "Settings" / "Configure ORC and Claude"
    │   └── Navigation groups with NavLinks
    │       ├── CLAUDE CODE: Slash Commands, CLAUDE.md, MCP Servers, Memory, Permissions
    │       ├── ORC: Projects, Billing & Usage, Import / Export
    │       └── ACCOUNT: Profile, API Keys
    └── Content (1fr)
        └── Outlet (renders section components)
```

### Sidebar Navigation

| Group | Items |
|-------|-------|
| CLAUDE CODE | Slash Commands (badge), CLAUDE.md, MCP Servers (badge), Memory (badge), Permissions |
| ORC | Projects, Billing & Usage, Import / Export |
| ACCOUNT | Profile, API Keys |

**Badge counts:** Slash Commands, MCP Servers, and Memory show count badges (currently mock data).

### SettingsView (Slash Commands)

Page header with title "Slash Commands", subtitle, and "New Command" button.

Content area displays:
- **CommandList**: Left panel showing project and global commands
- **ConfigEditor**: Right panel for editing selected command

**Data flow:** Mock data initially. Will integrate with API when endpoints are available.

### CSS Specifications

**Sidebar:**
- Width: 240px fixed
- Background: `var(--bg-elevated)`
- Border-right: 1px solid `var(--border)`
- Independent scrolling: `overflow-y: auto`

**Navigation Items:**
- Padding: 10px 12px
- Border-radius: 6px
- Font-size: 12px
- Gap: 10px (icon to text)
- Hover: `var(--bg-surface)`, `var(--text-primary)`
- Active: `var(--primary-dim)`, `var(--primary-bright)`

**Content Area:**
- Padding: 24px
- Independent scrolling: `overflow-y: auto`

## Environment Pages (Legacy)

**Note:** `/environment/*` routes now redirect to `/settings`. The new Settings page provides a redesigned interface with grouped navigation.

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
