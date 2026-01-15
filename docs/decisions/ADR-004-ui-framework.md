# ADR-004: UI Framework

**Status**: Superseded by ADR-004a
**Date**: 2026-01-10
**Superseded**: 2026-01-15 (migrated to React 19)

---

## Context

Orc needs a web UI for task list, timeline visualization, live transcript streaming, and controls.

**UI Characteristics**:
- ~10-15 components total (simple)
- Real-time updates critical (live transcript)
- Lightweight (ships with CLI tool)

## Original Decision (Superseded)

**Svelte 5 with SvelteKit** was the original choice for the frontend UI.

## Migration Decision (2026-01-15)

**React 19 with Vite** is now the production frontend framework.

### Why We Migrated

| Factor | Svelte 5 | React 19 |
|--------|----------|----------|
| Ecosystem | Smaller, fewer libraries | Vast ecosystem, mature tooling |
| Developer familiarity | Learning curve | Industry standard |
| Testing tools | Limited | Extensive (Testing Library, MSW) |
| Component libraries | Few options | Many production-ready options |
| Long-term maintenance | Uncertain | Stable, well-supported |
| E2E testing | Framework-specific selectors | Standard DOM patterns |

### Migration Process

The migration followed a phased approach:
1. **Research**: Evaluated React 19 features, state management (Zustand)
2. **Implementation**: Built parallel React app in `web-react/`
3. **Validation**: Ran E2E tests against both implementations
4. **Cutover**: Archived Svelte to `web-svelte-archive/`, moved React to `web/`

### Real-time Transcript Example (React)

```tsx
function TaskTranscript({ taskId }: { taskId: string }) {
  const [lines, setLines] = useState<TranscriptLine[]>([]);

  useEffect(() => {
    const eventSource = new EventSource(`/api/tasks/${taskId}/stream`);
    eventSource.onmessage = (e) => {
      setLines(prev => [...prev, JSON.parse(e.data)]);
    };
    return () => eventSource.close();
  }, [taskId]);

  return (
    <>
      {lines.map((line, i) => (
        <p key={i}>{line.content}</p>
      ))}
    </>
  );
}
```

## Current Stack

| Layer | Technology |
|-------|------------|
| Framework | React 19, Vite |
| Language | TypeScript 5.6+ |
| State Management | Zustand |
| Routing | React Router 7 |
| Styling | CSS (component-scoped) |
| Testing | Vitest (unit), Playwright (E2E) |

## Consequences

**Positive**:
- Larger ecosystem, more hiring options
- Better tooling and testing infrastructure
- Industry-standard patterns
- E2E tests use framework-agnostic selectors

**Negative**:
- Larger bundle size than Svelte (~45KB vs ~5KB gzipped)
- More boilerplate for state management

**Mitigation**: Zustand provides ergonomic state management similar to Svelte stores. REST API separation means framework remains swappable.

## Archive

The original Svelte implementation is preserved at `web-svelte-archive/` for 30 days post-cutover as a rollback option.
