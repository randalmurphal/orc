# React 19 Frontend (Migration)

React 19 application for orc web UI, running in parallel with existing Svelte app during migration.

## Tech Stack

| Layer | Technology |
|-------|------------|
| Framework | React 19, Vite |
| Language | TypeScript 5.6+ |
| State Management | Zustand |
| Routing | React Router (planned) |
| Styling | CSS (component-scoped, matching Svelte) |
| Testing | Vitest (unit), Playwright (E2E - shared) |

## Directory Structure

```
web-react/src/
├── main.tsx              # Entry point
├── App.tsx               # Root component
├── index.css             # Global styles
├── lib/                  # Shared utilities
├── components/           # UI components
├── pages/                # Route pages
├── stores/               # Zustand stores
└── hooks/                # Custom hooks
```

## Development

```bash
# Install dependencies
cd web-react && npm install

# Start dev server (port 5174)
npm run dev

# Run tests
npm run test
npm run test:watch
npm run test:coverage

# Build for production
npm run build
```

**Ports:**
- Svelte (current): `http://localhost:5173`
- React (migration): `http://localhost:5174`
- API server: `http://localhost:8080`

## Configuration

### Vite Config

| Setting | Value | Purpose |
|---------|-------|---------|
| Port | 5174 | Avoid conflict with Svelte on 5173 |
| API Proxy | `/api` → `:8080` | Backend communication |
| WebSocket | Proxied via `/api` | Real-time updates |
| Path Alias | `@/` → `src/` | Clean imports |
| Build Output | `build/` | Matches Svelte structure |

### TypeScript Config

| Setting | Purpose |
|---------|---------|
| `strict: true` | Full type safety |
| `noUnusedLocals: true` | Clean code |
| `jsx: react-jsx` | React 19 JSX transform |
| `paths: @/*` | Import aliases |
| `types: vitest/globals` | Test globals |

## Migration Strategy

This React app runs alongside Svelte during migration:

1. **Phase 1** (current): Project scaffolding, API connectivity verified
2. **Phase 2**: Core infrastructure (API client, WebSocket, stores, router)
3. **Phase 3**: Component migration (parallel implementation)
4. **Phase 4**: E2E test validation, feature parity verification
5. **Phase 5**: Cutover and Svelte removal

### Shared Resources

| Resource | Location | Notes |
|----------|----------|-------|
| E2E tests | `web/e2e/` | Shared, framework-agnostic |
| API server | `:8080` | Same backend |
| Visual baselines | `web/e2e/__snapshots__/` | Will need React baselines |

### Component Mapping

Migration follows the existing Svelte component structure:

| Svelte Component | React Equivalent | Status |
|------------------|------------------|--------|
| `+layout.svelte` | `App.tsx` + Router | Scaffolded |
| `lib/components/` | `src/components/` | Pending |
| `lib/stores/` | `src/stores/` (Zustand) | Implemented |
| `lib/utils/` | `src/lib/` | Pending |

## Testing

### Unit Tests (Vitest)

```bash
npm run test          # Run once
npm run test:watch    # Watch mode
npm run test:coverage # With coverage
```

Test files use `*.test.tsx` convention. Setup in `src/test-setup.ts` includes:
- `@testing-library/react` for component testing
- `@testing-library/jest-dom` for DOM matchers
- jsdom environment

### E2E Tests (Playwright)

E2E tests are shared with Svelte in `web/e2e/`. Tests use framework-agnostic selectors:
- `getByRole()` for semantic elements
- `getByText()` for headings/labels
- `.locator()` with class names for structural elements

## API Integration

The app connects to the orc API server via Vite proxy:

```typescript
// Example: Health check
fetch('/api/health')
  .then(res => res.json())
  .then(data => console.log(data.status));
```

All `/api/*` requests proxy to `localhost:8080`. WebSocket connections also proxy through the same path.

## Patterns

### Component Structure

```tsx
// Functional components with hooks
function TaskCard({ task }: { task: Task }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="task-card">
      {/* ... */}
    </div>
  );
}
```

### State Management (Zustand)

Four Zustand stores mirror the Svelte store behavior:

| Store | State | Persistence | Purpose |
|-------|-------|-------------|---------|
| `useTaskStore` | tasks, taskStates | None (API-driven) | Task data and execution state |
| `useProjectStore` | projects, currentProjectId | URL + localStorage | Project selection |
| `useInitiativeStore` | initiatives, currentInitiativeId | URL + localStorage | Initiative filter |
| `useUIStore` | sidebarExpanded, wsStatus, toasts | localStorage (sidebar) | UI state |

**URL/localStorage priority:** URL param > localStorage > default

**Usage:**
```tsx
import { useTaskStore, useProjectStore, useInitiativeStore, useUIStore, toast } from '@/stores';

// Direct state access
const tasks = useTaskStore((state) => state.tasks);

// Derived state via selector hooks
import { useActiveTasks, useStatusCounts } from '@/stores';
const activeTasks = useActiveTasks();
const counts = useStatusCounts();

// Actions
useTaskStore.getState().updateTask('TASK-001', { status: 'running' });
useProjectStore.getState().selectProject('proj-001');

// Toast notifications (works outside React components)
toast.success('Task created');
toast.error('Failed to load', { duration: 10000 });
```

**Special values:**
- `UNASSIGNED_INITIATIVE = '__unassigned__'` - Filter for tasks without an initiative

## Known Differences from Svelte

| Aspect | Svelte 5 | React 19 |
|--------|----------|----------|
| Reactivity | `$state()`, `$derived()` | `useState()`, `useMemo()` |
| Props | `$props()` | Destructured props |
| Events | `onclick` | `onClick` |
| Two-way binding | `bind:value` | `value` + `onChange` |
| Stores | Svelte stores | Zustand stores |
| Routing | SvelteKit | React Router |

## Dependencies

### Production
- `react@19`, `react-dom@19` - Core framework
- `zustand@5` - State management with subscribeWithSelector middleware
- `@fontsource/inter`, `@fontsource/jetbrains-mono` - Typography (matching Svelte)

### Development
- `vite`, `@vitejs/plugin-react` - Build tooling
- `typescript`, `@types/react*` - Type safety
- `vitest`, `@testing-library/*`, `jsdom` - Testing
