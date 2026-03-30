# Web

The web app is the React/TypeScript UI for orc.

## Owns

- route-level UX
- frontend state and subscriptions
- proto/client consumption
- workflow and runtime-config editing UI

## Rules

- The web app should consume backend contracts, not invent parallel ones.
- If a proto field changes, update generated code usage, UI state, tests, and typecheck in the same work.
- Keep UI naming aligned with current architecture. Avoid stale Claude-specific terminology where the product is now runtime/provider-neutral.
- Prefer small, composable components and shared utilities over large monolithic pages.

## Runtime Config UI

- The editor must reflect the nested `runtime_config` model.
- Shared settings and provider-local settings must stay distinct in the UI.
- Do not serialize the old flat config shape.

## Verification

```bash
pnpm -C web exec tsc --noEmit
pnpm -C web exec vitest run
```
