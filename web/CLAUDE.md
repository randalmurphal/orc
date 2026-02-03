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
│   ├── overlays/         # Modal components (NewTaskWorkflowModal, TaskDetailsModal, WorkflowPickerModal, DiffViewModal, ProjectSwitcher)
│   ├── task-detail/      # Task detail tabs (Overview, Transcript, TestResults, etc.)
│   │   ├── diff/         # Diff components (DiffFile, DiffHunk, DiffLine, DiffStats)
│   ├── timeline/         # Timeline event view
│   ├── workflow-editor/  # Visual editor (React Flow canvas, dagre layout)
│   └── [8 more dirs]     # dashboard/, settings/, stats/, initiatives/, etc.
├── stores/               # Zustand stores (10 stores — see State Management)
├── hooks/                # Custom hooks (useShortcuts, useEvents, useDocumentTitle, etc.)
├── pages/                # Route pages
├── lib/                  # Utilities (client.ts, time.ts, format.ts, claudeConfigUtils.ts)
├── gen/                  # Generated protobuf types (orc/v1/)
└── test/                 # Test utilities and mock factories
```

## Routes

| Route | Page | Description |
|-------|------|-------------|
| `/` | ProjectPickerPage | Project selection (redirects to `/board` when project chosen) |
| `/board` | BoardView | Kanban board (queue + running columns) |
| `/tasks/:taskId` | TaskDetailPage | Task details with workflow progress, resizable split panes, metrics footer |
| `/initiatives` | InitiativesPage | Initiative list and stats |
| `/initiatives/:id` | InitiativeDetailPage | Initiative detail view |
| `/settings` | SettingsPage | 3-tab layout: General, Agents, Environment |
| `/settings/general/*` | SettingsLayout | Sidebar nav for Claude Code, ORC, Account sections |
| `/settings/agents` | AgentsView | Agent configuration, execution settings |
| `/settings/environment/*` | EnvironmentLayout | Sub-nav for hooks, skills, tools, config |
| `/workflows` | WorkflowsPage | Redesigned workflows management with Your Workflows/Built-in sections |
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
| `projectStore` | `useCurrentProject`, `useProjects` | Active project context; `projectId` passed to all API queries via `DataProvider` |
| `uiStore` | `usePendingDecisions`, `useWsStatus`, `useToasts` | |
| `preferencesStore` | `useTheme`, `useBoardViewMode` | |
| `workflowEditorStore` | `useEditorNodes`, `useEditorEdges`, `useEditorActiveRun`, `useSelectedEdge` | React Flow state, execution tracking, edge selection (for GateInspector) |
| `dependencyStore` | `useDependencyFilter` | URL + localStorage persisted dependency status filter |
| `statsStore` | `useStats`, `useCostSummary` | Dashboard statistics, cost summaries, daily metrics |

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
| `AttentionDashboard` | `board/` | Three-section dashboard: running tasks, attention items, queue. Error states with visual indicators for failed tasks and phases |
| `AppShell` | `layout/` | Main layout shell. Route-aware panel rendering via `useLocation` |
| `RightPanel` | `layout/` | Collapsible panel with compound component API (Section/Header/Body) |
| `TopBar` | `layout/` | Session stats, search, pause/resume. Uses individual store selectors |
| `TaskEditModal` | `task-detail/` | Edit task properties + branch/PR settings (`branchName`, `targetBranch`, `prDraft`, `prLabels`, `prReviewers`) |
| `NewTaskWorkflowModal` | `overlays/` | Orchestrates 2-step workflow-first task creation: Step 1 (workflow picker) → Step 2 (task details) |
| `WorkflowPickerModal` | `overlays/` | Step 1: Select workflow from grid (built-in + custom), shows phase count and description, keyboard navigation |
| `TaskDetailsModal` | `overlays/` | Step 2: Enter task details (title/description), category/priority, advanced options, Create/Create & Run actions |
| `DiffViewModal` | `overlays/` | Lazygit-style full-screen diff modal: file navigation, split/unified view, vim keybinds, search/filter, accessibility |
| `WorkflowProgress` | `task-detail/` | Visual phase progression with gate diamonds, state indicators (✓/●/○/✗), and gate type colors (auto/human/ai) |
| `TaskFooter` | `task-detail/` | Footer with session metrics (tokens/cost), action buttons (pause/resume/cancel/retry), error display with retry options |
| `LiveOutputPanel` | `task-detail/` | Real-time transcript streaming: WebSocket events, auto-scroll, virtual scrolling for large transcripts, message styling by type |
| `FeedbackPanel` | `task-detail/` | Agent feedback UI: create feedback (general/inline/approval/direction), timing controls (now/when-done/manual), send pending, form validation |
| `SettingsTabs` | `settings/` | Top-level 3-tab navigation (General, Agents, Environment) with URL-driven state |
| `SettingsLayout` | `settings/` | General tab: 240px sidebar with CLAUDE CODE/ORC/ACCOUNT groups |
| `EnvironmentLayout` | `pages/environment/` | Environment tab: horizontal sub-nav for hooks/skills/tools/config |
| `GitSettingsPage` | `pages/settings/` | Read-only info page showing project-level git defaults and override options |
| `WorkflowEditorPage` | `workflow-editor/` | 3-panel visual editor: palette \| canvas \| inspector |
| `WorkflowCanvas` | `workflow-editor/` | React Flow canvas: drag-to-add, edge drawing/deletion with cycle detection, topo sort resequencing (`utils/topoSort.ts`), layout persistence |
| Edge types | `workflow-editor/edges/` | 6 custom edges: sequential, loop (backward/forward detection with sequence-aware styling), retry, dependency (badge + animated dots), conditional (condition label), gate (diamond symbol with type/status colors). Styles in `edges.css` |
| `PhaseNode` | `workflow-editor/nodes/` | Custom React Flow node: connection handles (L/R), category color accents, status states, gate badges |
| `VirtualNode` | `workflow-editor/nodes/` | Invisible anchor nodes for entry/exit gate edges (20×20px, excluded from minimap) |
| `GateEdge` | `workflow-editor/edges/` | Gate transition edge: diamond ◆ symbol on midpoint, type colors (gray/blue/yellow/purple), status override (green/red), hover tooltip |
| `GateInspector` | `workflow-editor/panels/` | Enhanced gate configuration panel: type selector (Auto/Human/AI/Skip), type-specific sections (auto criteria, human prompts, AI agents), failure handling with retry options, collapsible advanced settings (scripts, result variables), API integration for saving changes. Read-only for built-in workflows |
| `CanvasToolbar` | `workflow-editor/` | Canvas controls: fit view, reset layout, zoom in/out |
| `DeletePhaseDialog` | `workflow-editor/` | Confirmation dialog for phase deletion |
| `ExecutionHeader` | `workflow-editor/` | Run status badge, metrics (duration/tokens/cost), cancel button |
| `PhaseInspector` | `workflow-editor/panels/` | Right panel: Phase Input/Prompt/Completion/Settings tabs, condition editor, claude_config |
| `PromptEditor` | `workflow-editor/panels/` | Prompt viewer with variable highlighting, editable textarea for custom |
| `PhaseTemplatePalette` | `workflow-editor/panels/` | Left panel: draggable phase templates for adding to canvas |
| `VariableModal` | `workflow-editor/` | Create/edit workflow variables with source-specific forms |
| `VariableReferencePanel` | `workflow-editor/` | Shows available `{{VAR}}` patterns grouped by category |
| `PhaseListEditor` | `workflows/` | Phase list with add/edit/remove/reorder. Edit dialog shows inherited vs override claude_config sections |
| `EditPhaseTemplateModal` | `workflows/` | Phase template editor: data flow (input/output vars, prompt source), 7 claude_config sections, JSON override |
| `CreatePhaseTemplateModal` | `workflows/` | Create phase template from scratch: auto-ID slugification, prompt editor with `{{VAR}}` highlighting, input variable chips with suggestions, 7 claude_config sections |
| `ConditionEditor` | `workflows/` | Visual condition builder + raw JSON mode. Operators: eq/neq/in/contains/exists/gt/lt. Logic: all/any |
| `CollapsibleSettingsSection` | `core/` | Collapsible header with chevron + badge counter. Used in phase editors and inspectors |
| `LibraryPicker` | `core/` | Multi-select picker for hooks (grouped by event), skills, MCP servers |
| `TagInput` | `core/` | Chip-style tag input (Enter/comma to add, backspace to remove) |
| `KeyValueEditor` | `core/` | Row-based key-value editor for env vars. Empty keys excluded from output |
| `SplitPane` | `core/` | Resizable split pane with left/right panels, localStorage persistence, min width constraints, keyboard/touch support |
| `DiffFile` | `task-detail/diff/` | Individual file diff display: collapsible header, status icons, addition/deletion stats, comment threading |
| `DiffHunk` | `task-detail/diff/` | Diff hunk rendering: context lines, line numbers, split/unified modes, syntax highlighting |
| `DiffLine` | `task-detail/diff/` | Single diff line: type indicators (+/-/~), line numbers, content with syntax highlighting |
| `DiffStats` | `task-detail/diff/` | Diff summary statistics: files changed, additions, deletions, binary file indicator |

### Gates as Edges Visual Model

**Mental model:** Phases are work (nodes), gates are transitions (edges). This matches how users think about workflows.

| Element | Component | Rendering |
|---------|-----------|-----------|
| Entry gate | `GateEdge` | `virtual-entry` → first phase |
| Between gates | `GateEdge` | `phase[i]` → `phase[i+1]` |
| Exit gate | `GateEdge` | last phase → `virtual-exit` |
| Virtual anchors | `VirtualNode` | Invisible 20×20px nodes for entry/exit edges |

**Gate type colors:** Auto (blue), Human (yellow), AI (purple), Passthrough/Skip (gray). **Status overrides:** Passed (green), Blocked/Failed (red).

Layout generation: `utils/layoutWorkflow.ts:getEffectiveGateType()` resolves gate type from phase override → template default → AUTO.

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
