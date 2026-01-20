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
cd web && npm install   # Install dependencies
npm run dev             # Dev server (port 5173)
npm run test            # Run tests
npm run build           # Production build
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
├── lib/                  # Utilities (types, websocket, shortcuts)
├── components/           # UI components
│   ├── agents/           # Agent components (AgentCard, ExecutionSettings, ToolPermissions, AgentsView)
│   ├── board/            # Kanban (Board, Column, TaskCard, etc.)
│   ├── dashboard/        # Dashboard sections
│   ├── initiatives/      # Initiative components (InitiativeCard, StatsRow, InitiativesView)
│   ├── layout/           # AppLayout, Sidebar, Header, IconNav, TopBar
│   ├── stats/            # Statistics visualizations (OutcomesDonut)
│   ├── task-detail/      # TaskHeader, TabNav, tabs
│   ├── overlays/         # Modal, CommandPalette, NewTaskModal
│   ├── settings/         # Settings components (CommandList, CommandEditor, ConfigEditor)
│   ├── stats/            # Statistics visualizations (TasksBarChart)
│   └── ui/               # Primitives (Button, Icon, Input, Tooltip)
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
| `initiativeStore` | Initiative filter |
| `sessionStore` | Session metrics (duration, tokens, cost), pause/resume |
| `uiStore` | Sidebar, toasts, WebSocket status |

### WebSocket Events

| Event | Payload |
|-------|---------|
| `task_created/updated/deleted` | Task or `{ id }` |
| `state_updated` | TaskState |
| `transcript` | `{ task_id, content, tokens }` |
| `activity` | `{ phase, activity }` - see `ActivityState` in types.ts |
| `heartbeat` | `{ phase, iteration, timestamp }` |
| `finalize` | `{ task_id, status, step }` |

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Shift+Alt+K` | Command palette |
| `Shift+Alt+N` | New task |
| `Shift+Alt+P` | Project switcher |
| `g d / g t / g b` | Go to dashboard/tasks/board |

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
| `/initiatives/:id` | InitiativeDetail |
| `/agents` | AgentsPage (agent cards, execution settings, tool permissions) |
| `/stats` | Dashboard stats |
| `/tasks/:id` | TaskDetail (6 tabs) |
| `/settings/*` | Config editors |

## Testing

```bash
npm run test                                    # Vitest
npx playwright test                             # E2E
npx playwright test --project=visual            # Visual regression
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
