# React 19 Frontend

Orc web UI built with React 19 + Vite.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | React 19, Vite |
| Language | TypeScript 5.6+ |
| State | Zustand stores (`stores/`) with `useShallow` for derived selectors |
| Events | Connect RPC streaming (`useEvents`, `EventProvider`) |
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
├── components/
│   ├── ui/               # Base primitives (Button, Input, Tooltip, Icon, etc.)
│   ├── core/             # Shared primitives (Badge, Card, Select, Slider, Toggle, etc.)
│   ├── board/            # Board view (TaskCard, RunningCard, Swimlane, BoardCommandPanel)
│   ├── layout/           # Shell (AppShell, TopBar, IconNav, RightPanel, AppShellContext)
│   ├── agents/           # Agent config (AgentsView, AgentCard, ExecutionSettings)
│   ├── overlays/         # Modal components (NewTaskModal, ProjectSwitcher)
│   ├── task-detail/      # Task detail tabs (Overview, Transcript, TestResults, etc.)
│   ├── timeline/         # Timeline event view
│   ├── workflow-editor/  # Visual editor (React Flow canvas, dagre layout)
│   └── [8 more dirs]     # dashboard/, settings/, stats/, initiatives/, etc.
├── stores/               # Zustand stores (10 stores — see State Management)
├── hooks/                # Custom hooks (useShortcuts, useEvents, useDocumentTitle, etc.)
├── pages/                # Route pages
├── lib/                  # Generic utilities (client.ts, time.ts, format.ts)
├── gen/                  # Generated protobuf types (orc/v1/)
└── test/                 # Test utilities and mock factories
```

## Routes

| Route | Page | Description |
|-------|------|-------------|
| `/` | — | Redirects to `/board` |
| `/board` | BoardView | Kanban board (queue + running columns) |
| `/tasks/:taskId` | TaskDetail | Task details, transcript, review |
| `/initiatives` | InitiativesPage | Initiative list and stats |
| `/initiatives/:id` | InitiativeDetailPage | Initiative detail view |
| `/agents` | AgentsView | Agent configuration, execution settings |
| `/settings` | SettingsPage | Configuration editor |
| `/workflows` | WorkflowsPage | Workflow and phase template management |
| `/workflows/:id` | WorkflowEditorPage | Visual workflow editor (React Flow canvas) |
| `/timeline` | TimelinePage | Event timeline with filters |
| `/stats` | StatsPage | Dashboard statistics |

## State Management

Zustand stores in `stores/`. Each exports the base store hook + granular selector hooks.

| Store | Key Selectors | Notes |
|-------|--------------|-------|
| `taskStore` | `useActiveTasks`, `useRunningTasks`, `useStatusCounts`, `useTask(id)` | Derived selectors use `useShallow` |
| `sessionStore` | `useFormattedDuration`, `useFormattedCost`, `useIsPaused`, `useSessionMetrics` | `useSessionMetrics` uses `useShallow` |
| `workflowStore` | `useBuiltinWorkflows`, `useCustomWorkflows`, `useRunningRuns` | Filter selectors use `useShallow` |
| `initiativeStore` | `useInitiatives`, `useCurrentInitiative` | |
| `projectStore` | `useCurrentProject`, `useProjects` | |
| `uiStore` | `usePendingDecisions`, `useWsStatus`, `useToasts` | |
| `preferencesStore` | `useTheme`, `useBoardViewMode` | |
| `workflowEditorStore` | `useEditorNodes`, `useEditorEdges`, `useEditorActiveRun` | React Flow state + execution tracking |

**Pattern — `useShallow` for derived selectors:**
```tsx
// Store methods that return new arrays/objects need useShallow to prevent re-renders
import { useShallow } from 'zustand/react/shallow';
export const useActiveTasks = () => useTaskStore(useShallow((s) => s.getActiveTasks()));
```

## Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| `BoardView` | `board/` | Two-column grid (queue + running). Pure layout, no side effects |
| `BoardCommandPanel` | `board/` | Right panel for board: blocked, decisions, config, files, completed. Reads stores directly |
| `TaskCard` | `board/` | Compact task card. `memo()`-wrapped with memo-friendly callbacks |
| `RunningCard` | `board/` | Active task card with pipeline + output. `memo()`-wrapped |
| `Swimlane` | `board/` | Initiative group in queue column. `memo()`-wrapped |
| `AppShell` | `layout/` | Main layout shell. Route-aware panel rendering via `useLocation` |
| `RightPanel` | `layout/` | Collapsible panel with compound component API (Section/Header/Body) |
| `TopBar` | `layout/` | Session stats, search, pause/resume. Uses individual store selectors |
| `WorkflowEditorPage` | `workflow-editor/` | 3-panel visual editor: palette \| canvas \| inspector |
| `WorkflowCanvas` | `workflow-editor/` | React Flow canvas with drag-to-add, delete, connections, layout persistence |
| `CanvasToolbar` | `workflow-editor/` | Canvas controls: fit view, reset layout, zoom in/out |
| `DeletePhaseDialog` | `workflow-editor/` | Confirmation dialog for phase deletion |
| `ExecutionHeader` | `workflow-editor/` | Run status badge, metrics (duration/tokens/cost), cancel button |
| `PhaseInspector` | `workflow-editor/panels/` | Right panel: Prompt/Variables/Settings tabs for selected phase |
| `PromptEditor` | `workflow-editor/panels/` | Prompt viewer with variable highlighting, editable textarea for custom |
| `PhaseTemplatePalette` | `workflow-editor/panels/` | Left panel: draggable phase templates for adding to canvas |
| `VariableModal` | `workflow-editor/` | Create/edit workflow variables with source-specific forms |
| `VariableReferencePanel` | `workflow-editor/` | Shows available `{{VAR}}` patterns grouped by category |

## React Patterns

### Memo Boundaries

`TaskCard`, `RunningCard`, and `Swimlane` are wrapped with `React.memo`. To avoid defeating memo:

| Pattern | Do | Don't |
|---------|------|-------|
| List callbacks | Pass `onTaskClick={handler}` (stable ref) | Pass `onClick={() => handler(task)}` (new closure per render) |
| Store selectors | `useTaskStore((s) => s.tasks)` | `useTaskStore()` (subscribes to ALL state) |
| Context values | `useMemo(() => ({ isOpen }), [isOpen])` | `value={{ isOpen }}` (new object per render) |

**TaskCard memo-friendly props:** `onTaskClick(task)` and `onTaskContextMenu(task, e)` accept the task as argument, allowing parents to pass a single stable callback for all items in a list.

### Right Panel Architecture

AppShell renders route-specific panel content:
- `/board` → `<BoardCommandPanel />` (reads stores directly, no props needed)
- Other routes → `defaultPanelContent` prop

**No JSX through context.** Panel content components read from stores. AppShellContext only manages: `isRightPanelOpen`, `toggleRightPanel`, `isMobileNavMode`.

### Async Effects

Always use mounted guards for async effects that set state:
```tsx
useEffect(() => {
  let mounted = true;
  fetchData().then((data) => { if (mounted) setState(data); });
  return () => { mounted = false; };
}, []);
```

## Custom Hooks

| Hook | Purpose |
|------|---------|
| `useShortcuts` / `ShortcutProvider` | Keyboard shortcut registration + context (`hooks/useShortcuts.tsx`) |
| `useEvents` / `EventProvider` | Connect RPC streaming, WebSocket events (`hooks/useEvents.tsx`) |
| `useDocumentTitle` | Dynamic page title (`hooks/useDocumentTitle.ts`) |
| `useClickKeyboard` | Click/keyboard combo handler (`hooks/useClickKeyboard.ts`) |
| `useTaskSubscription` | Subscribe to individual task events (`hooks/useEvents.tsx`) |
| `useLayoutPersistence` | Debounced node position saving for workflow canvas (`workflow-editor/hooks/`) |

See `stores/index.ts` for all exported store selector hooks (60+ hooks).

## WebSocket Events

| Event | Payload |
|-------|---------|
| `task_created/updated/deleted` | Task or `{ id }` |
| `state_updated` | TaskState |
| `transcript` | `{ task_id, content, tokens }` |
| `activity` | `{ phase, activity }` |
| `phaseChanged` | `{ taskId, phaseName, status, iteration, error }` |
| `tokensUpdated` | `{ taskId, tokens: { inputTokens, outputTokens } }` |

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
