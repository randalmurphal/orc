# React 19 Frontend

Orc web UI built with React 19 + Vite.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | React 19, Vite |
| Language | TypeScript 5.6+ |
| State | Zustand stores (taskStore, initiativeStore, etc.) |
| Events | Connect RPC streaming (useEvents, EventProvider) |
| Routing | React Router 7 |
| Styling | CSS custom properties + design tokens (`styles/tokens.css`) |
| Components | Radix UI, custom primitives (`components/core/`) |
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
├── index.css             # Global styles (design tokens)
├── api/                  # API client
│   ├── index.ts          # Fetch functions
│   └── types.ts          # API response types
├── components/
│   ├── ui/               # Base primitives (Button, Input, Tooltip, etc.)
│   ├── core/             # Shared primitives (Badge, Card, Select, Slider, Toggle, etc.)
│   ├── agents/           # Agent config (AgentsView, AgentCard, ExecutionSettings, ToolPermissions)
│   ├── overlays/         # Modal components (NewTaskModal, ProjectSwitcher)
│   ├── workflow-editor/  # Visual editor (React Flow canvas, dagre layout)
│   │   └── utils/        # layoutWorkflow (dagre), graph helpers
│   └── [feature]/        # Feature components (board/, task-detail/, etc.)
├── context/              # React Context providers
│   ├── SettingsContext.tsx
│   ├── ToastContext.tsx
│   └── WebSocketContext.tsx
├── hooks/                # Custom hooks
├── pages/                # Route pages
├── types/                # TypeScript definitions
├── lib/                  # Generic utilities (graph-layout.ts)
├── utils/                # Utility functions (format.ts)
└── test/                 # Test utilities and mocks
```

## Routes

| Route | Page | Description |
|-------|------|-------------|
| `/` | TasksPage | Dashboard with task list and board |
| `/tasks/:taskId` | TaskDetailPanel | Task details, transcript, review |
| `/initiatives` | InitiativesPage | Initiative list and stats |
| `/initiatives/:id` | InitiativeDetailPanel | Initiative detail view |
| `/agents` | Agents | Agent configuration, execution settings, tool permissions |
| `/settings` | SettingsPage | Configuration editor |
| `/knowledge` | KnowledgePage | Knowledge service config |
| `/workflows` | WorkflowsPage | Workflow and phase template management |
| `/workflows/:id` | WorkflowEditorPage | Visual workflow editor (React Flow canvas) |

## Key Components

| Component | Purpose |
|-----------|---------|
| `TaskCard` | Task display with status, actions |
| `TaskList` | Filterable task list |
| `TaskDetailPanel` | Full task view with tabs |
| `TaskMonitor` | Real-time task execution view |
| `NewTaskModal` | Task creation with WorkflowSelector |
| `TranscriptViewer` | Claude conversation display |
| `KnowledgePanel` | Knowledge service configuration |
| `WorkflowSelector` | Workflow dropdown for task forms |
| `EditWorkflowModal` | Workflow metadata and phase editing |
| `PhaseListEditor` | Phase management (add/edit/remove/reorder) |
| `WorkflowEditorPage` | 3-panel visual editor: palette \| canvas \| inspector |
| `WorkflowCanvas` | React Flow wrapper with nodes/edges from `workflowEditorStore` |
| `AgentsView` | Agent page container (cards + execution settings + tool permissions) |
| `AgentCard` | Individual agent display with stats and tool badges |
| `ExecutionSettings` | Global settings: parallel tasks, auto-approve, model, cost limit |
| `ToolPermissions` | 3-column grid of tool permission toggles |

## Custom Hooks

| Hook | Purpose |
|------|---------|
| `useTaskStore` | Task state from Zustand store |
| `useInitiatives` | Initiative data fetching |
| `useKnowledge` | Knowledge service state |
| `useWebSocket` | WebSocket connection + events |
| `useSettings` | Settings state management |
| `useKeyboard` | Keyboard shortcut registration |
| `useEditorNodes/Edges` | Workflow editor React Flow state selectors |
| `workflowEditorStore` | Zustand store: nodes, edges, readOnly, selectedNodeId |

## WebSocket Events

| Event | Payload |
|-------|---------|
| `task_created/updated/deleted` | Task or `{ id }` |
| `state_updated` | TaskState |
| `transcript` | `{ task_id, content, tokens }` |
| `activity` | `{ phase, activity }` |

## UI Components (components/ui/)

Base primitives built on Radix UI:

| Component | Variants/Notes |
|-----------|----------------|
| `Button` | primary, secondary, danger, ghost, success; sizes: sm, md, lg; props: loading, iconOnly, leftIcon, rightIcon |
| `Input` | text, number, search |
| `Tooltip` | Radix Tooltip (TooltipProvider at App root) |
| `Icon` | Lucide icon wrapper |
| `StatusIndicator` | Task/phase status display |
| `Skeleton` | Loading placeholder |
| `Textarea` | Multi-line input |

## Core Components (components/core/)

Shared primitives exported via `core/index.ts`:

| Component | Variants/Notes |
|-----------|----------------|
| `Badge` | Status badges with variants |
| `Card` | Container with padding options |
| `Progress` | Progress bar with color/size variants |
| `SearchInput` | Search input with icon |
| `Select` | Dropdown select with options |
| `Slider` | Range slider with keyboard nav, step snapping, custom formatting |
| `Stat` | Stat display with trend, icon, value color |
| `Toggle` | Accessible switch with sizes: sm, md; animated transitions |

## Testing

```bash
bun run test                    # Vitest unit tests
bunx playwright test            # E2E tests
```

**CRITICAL:** E2E tests use sandbox in `/tmp`. Always import from `./fixtures`:

```ts
import { test, expect } from './fixtures';  // CORRECT
```

## Configuration

| Setting | Value |
|---------|-------|
| API Proxy | `/api` -> `:8080` (vite.config.ts) |
| Path Alias | `@/` -> `src/` |
| Build Output | `dist/` |

## Dependencies

**Core:** react, react-dom, react-router-dom, zustand

**UI:** @radix-ui/* (dialog, select, tabs, tooltip), lucide-react

**Canvas:** @xyflow/react (React Flow v12+), dagre (auto-layout)

**Dev:** vite, typescript, vitest, playwright
