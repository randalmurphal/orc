# ADR-004: UI Framework

**Status**: Accepted  
**Date**: 2026-01-10

---

## Context

Orc needs a web UI for task list, timeline visualization, live transcript streaming, and controls.

**UI Characteristics**:
- ~10-15 components total (simple)
- Real-time updates critical (live transcript)
- Lightweight (ships with CLI tool)

## Decision

**Svelte 5 with SvelteKit** for the frontend UI.

## Rationale

### Why Svelte Over React

| Factor | React | Svelte 5 |
|--------|-------|----------|
| Bundle size | ~45KB gzipped | ~5KB gzipped |
| State management | useState + Context/Redux | $state (native) |
| Derived values | useMemo with deps | $derived (automatic) |
| Effects | useEffect with deps | $effect (automatic) |
| Real-time | Custom hooks | Direct binding |
| Routing | react-router (extra) | SvelteKit built-in |

### Real-time Transcript Example

```svelte
<script lang="ts">
  let lines = $state<TranscriptLine[]>([]);
  
  $effect(() => {
    const eventSource = new EventSource(`/api/tasks/${taskId}/stream`);
    eventSource.onmessage = (e) => lines.push(JSON.parse(e.data));
    return () => eventSource.close();
  });
</script>

{#each lines as line}
  <p>{line.content}</p>
{/each}
```

No virtual DOM diffing - updates go directly to DOM.

## Consequences

**Positive**:
- Smaller bundles, faster updates, less memory
- Less boilerplate, cleaner code
- SvelteKit provides routing, SSR, API routes

**Negative**:
- Smaller ecosystem than React
- Fewer Svelte developers

**Mitigation**: REST API means UI framework is swappable if needed.
