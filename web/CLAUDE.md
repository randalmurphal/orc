# React 19 Frontend

React 19 application for orc web UI.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | React 19, Vite |
| Language | TypeScript 5.6+ |
| State | Zustand |
| Routing | React Router 7 |
| Styling | CSS design tokens |
| Testing | Vitest (unit), Playwright (E2E) |

## Quick Start

```bash
cd web && bun install   # Install dependencies
bun run dev             # Dev server (port 5173)
bun run test            # Run tests
bun run build           # Production build
```

**Ports:** Frontend `:5173`, API `:8080`

## Directory Structure

```
web/src/
├── main.tsx              # Entry point
├── App.tsx               # Root (routes + providers)
├── index.css             # Global styles (imports tokens)
├── styles/               # Design system (tokens.css, animations.css)
├── router/               # Route configuration
├── lib/                  # Utilities (types, api, websocket, shortcuts, format, errors)
├── components/           # UI components
│   ├── agents/           # Agent components (AgentCard, ExecutionSettings, ToolPermissions)
│   ├── board/            # Board (BoardView, QueueColumn, RunningColumn, TaskCard, Pipeline)
│   ├── core/             # Domain primitives (Badge, Card, Progress, SearchInput, Select, Slider, Stat, Toggle)
│   ├── dashboard/        # Dashboard sections
│   ├── initiatives/      # Initiative components (InitiativeCard, StatsRow, InitiativesView)
│   ├── layout/           # AppShell, IconNav, TopBar, RightPanel
│   ├── overlays/         # Modal, NewTaskModal, ProjectSwitcher, KeyboardShortcutsHelp
│   ├── settings/         # Settings (SettingsLayout, SettingsView, CommandEditor)
│   ├── stats/            # Statistics (StatsView, ActivityHeatmap, charts)
│   ├── task-detail/      # TaskHeader, TabNav, tabs (Transcript, Changes, Review, etc.)
│   ├── timeline/         # TimelineView, TimelineEvent, TimelineFilters, TimelineGroup
│   ├── transcript/       # TranscriptViewer, TranscriptNav, TranscriptSection, TranscriptSearch, TranscriptVirtualList
│   ├── ui/               # Base primitives (Button, Icon, Input, Tooltip, StatusIndicator, Breadcrumbs, Toast, EmptyState, Skeleton)
│   └── workflows/        # WorkflowsView, WorkflowCard, WorkflowDetailPanel
├── pages/                # Route pages
├── stores/               # Zustand stores
└── hooks/                # Custom hooks
```

## Configuration

| Setting | Value |
|---------|-------|
| API Proxy | `/api` -> `:8080` |
| Path Alias | `@/` -> `src/` |
| Build Output | `build/` |

## Core Architecture

### Stores (Zustand)

| Store | Purpose |
|-------|---------|
| `taskStore` | Task data and states |
| `projectStore` | Project selection (URL + localStorage) |
| `initiativeStore` | Initiative data and filter |
| `sessionStore` | Session metrics (duration, tokens, cost), pause/resume |
| `uiStore` | Sidebar, toasts, WebSocket status |
| `workflowStore` | Workflows and phase templates |
| `statsStore` | Statistics and analytics data |
| `preferencesStore` | User preferences |
| `dependencyStore` | Task dependency graph |

### Shared Utilities (`lib/`)

| Module | Exports | Usage |
|--------|---------|-------|
| `format.ts` | `formatNumber`, `formatCost`, `formatLargeNumber`, `formatDuration`, `formatPercentage`, `formatTrend` | Display formatting for numbers, tokens, costs |
| `errors.ts` | `APIError`, `handleStoreError` | Centralized error handling |
| `api.ts` | `fetchJSON`, automation/notification/session API functions | All API calls |
| `types.ts` | `Task`, `PhaseStatus`, `PhaseState`, etc. | TypeScript definitions aligned with Go backend |

### Custom Hooks (`hooks/`)

| Hook | Purpose |
|------|---------|
| `useClickKeyboard` | Handles Enter/Space on interactive non-button elements for a11y |
| `useShortcuts` | Global keyboard shortcut registration |
| `useWebSocket` | WebSocket connection with auto-reconnect |

### WebSocket Events

| Event | Payload |
|-------|---------|
| `task_created/updated/deleted` | Task or `{ id }` |
| `state_updated` | TaskState |
| `transcript` | `{ task_id, content, tokens }` |
| `activity` | `{ phase, activity }` - see `ActivityState` in types.ts |
| `heartbeat` | `{ phase, iteration, timestamp }` |
| `finalize` | `{ task_id, status, step }` |
| `decision_required` | `{ decision_id, task_id, task_title, phase, gate_type, question, context, requested_at }` |
| `decision_resolved` | `{ decision_id, task_id, phase, approved, reason, resolved_by, resolved_at }` |

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Shift+Alt+K` | Command palette |
| `Shift+Alt+N` | New task |
| `Shift+Alt+P` | Project switcher |
| `g b / g i / g s` | Go to board/initiatives/stats |
| `g a / g ,` | Go to agents/settings |

## Component Library

Built on Radix UI primitives for accessibility.

| Component | Use |
|-----------|-----|
| `Button` | Variants: primary, secondary, danger, ghost, success |
| `Modal` | Radix Dialog with focus trap |
| `Tooltip` | Replaces native title |
| `DropdownMenu` | TaskCard menu, ExportDropdown |
| `Select` | InitiativeDropdown, ViewModeDropdown |
| `Tabs` | TabNav in task detail |

See [docs/components.md](docs/components.md) for full API.

## Pages

| Route | Component |
|-------|-----------|
| `/` | Redirects to `/board` |
| `/board` | Board (flat/swimlane views) |
| `/initiatives` | InitiativesPage (aggregate stats, cards grid) |
| `/initiatives/:id` | InitiativeDetailPage |
| `/timeline` | TimelinePage (activity feed) |
| `/workflows` | WorkflowsPage (workflow cards, phase templates) |
| `/agents` | Agents (agent configuration) |
| `/stats` | StatsPage (summary cards, charts, leaderboards) |
| `/tasks/:id` | TaskDetail (tabs: Transcript, Changes, Review, Tests, Timeline, Comments) |
| `/settings/*` | SettingsPage with nested routes (commands, constitution, etc.) |

## Testing

```bash
bun run test                                    # Vitest
bunx playwright test                            # E2E
bunx playwright test --project=visual           # Visual regression
```

**CRITICAL:** E2E tests use sandbox in `/tmp`. Always import from `./fixtures`:

```ts
import { test, expect } from './fixtures';  // CORRECT
```

See [docs/testing.md](docs/testing.md) for details.

## Reference Docs

| Topic | Location |
|-------|----------|
| Styling & Tokens | [docs/styling.md](docs/styling.md) |
| Components | [docs/components.md](docs/components.md) |
| Architecture | [docs/architecture.md](docs/architecture.md) |
| Page Components | [docs/pages.md](docs/pages.md) |
| Testing | [docs/testing.md](docs/testing.md) |

## Dependencies

**Production:** react, react-router-dom, zustand, @radix-ui/* (dialog, dropdown-menu, select, tabs, tooltip)

**Development:** vite, typescript, vitest, playwright, @fontsource/inter, @fontsource/jetbrains-mono
