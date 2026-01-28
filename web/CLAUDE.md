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
| Styling | Tailwind CSS |
| Components | Radix UI, Headless UI |
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
├── index.css             # Global styles (Tailwind)
├── api/                  # API client
│   ├── index.ts          # Fetch functions
│   └── types.ts          # API response types
├── components/
│   ├── ui/               # Base primitives (Button, Input, Tooltip, etc.)
│   ├── overlays/         # Modal components (NewTaskModal, ProjectSwitcher)
│   └── [feature]/        # Feature components (board/, task-detail/, etc.)
├── context/              # React Context providers
│   ├── SettingsContext.tsx
│   ├── ToastContext.tsx
│   └── WebSocketContext.tsx
├── hooks/                # Custom hooks
├── pages/                # Route pages
├── types/                # TypeScript definitions
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
| `/settings` | SettingsPage | Configuration editor |
| `/knowledge` | KnowledgePage | Knowledge service config |
| `/workflows` | WorkflowsPage | Workflow and phase template management |
| `/agents` | AgentsPage | Agent configuration and execution settings |

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
| `AgentCard` | Agent card with stats, tools, status display |
| `AgentsView` | Agents page container with grid layout |
| `ExecutionSettings` | Global execution config (parallel tasks, model, cost) |
| `ToolPermissions` | Tool permission toggles grid |

## Custom Hooks

| Hook | Purpose |
|------|---------|
| `useTaskStore` | Task state from Zustand store |
| `useInitiatives` | Initiative data fetching |
| `useKnowledge` | Knowledge service state |
| `useWebSocket` | WebSocket connection + events |
| `useSettings` | Settings state management |
| `useKeyboard` | Keyboard shortcut registration |

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

**Core:** react, react-dom, react-router-dom, zustand, tailwindcss

**UI:** @radix-ui/* (dialog, select, tabs, tooltip), @headlessui/react, lucide-react

**Dev:** vite, typescript, vitest, playwright
