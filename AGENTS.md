# Orc

Task orchestration for coding workflows. Orc owns workflow semantics, persistence, gates, retries, transcripts, and git/task lifecycle.

## Read This First

Start here, then read the closest package guide before changing code in that area.

- [internal/AGENTS.md](/Users/randy/repos/orc/internal/AGENTS.md)
- [web/AGENTS.md](/Users/randy/repos/orc/web/AGENTS.md)
- [templates/AGENTS.md](/Users/randy/repos/orc/templates/AGENTS.md)
- [docs/AGENTS.md](/Users/randy/repos/orc/docs/AGENTS.md)

## Non-Negotiable Rules

### One path, not parallel paths

- Reuse the shared helper or abstraction that already owns the behavior.
- If two places do the same thing, extract one owner instead of adding a third copy.
- For schema-constrained LLM calls, use `llmutil.ExecuteWithSchema[T]()`.
- For phase completion parsing, use `CheckPhaseCompletionJSON()` and handle its error.

### No silent failure

- Do not swallow parse, validation, migration, or persistence errors.
- Do not add fallback behavior that hides broken state.
- If a field is declared, validated, or exposed, it must affect runtime behavior.

### Delete old code

- Remove dead code, dead fields, dead migrations, and dead docs completely.
- Do not keep legacy branches or aliases unless an explicit removal window exists.

## Runtime Boundary

This boundary is now strict.

- `llmkit` owns provider definitions, provider-native validation, runtime preparation, session semantics, stream normalization, and harness-local filesystem/config handling.
- `orc` owns orchestration, task/run persistence, transcript persistence, gates, retries, workflow resolution, and user-facing policy.
- Orc supports only `claude` and `codex`.
- Do not add provider-specific harness workarounds in `orc`. If behavior belongs in `llmkit`, implement it there first.

## Configuration Boundary

- Per-phase execution config is `runtime_config`, not `claude_config`.
- Shared intent lives under `runtime_config.shared`.
- Provider-local knobs live under `runtime_config.providers.<provider>`.
- Do not reintroduce flat Claude-shaped config or local compatibility shims.

## Change Discipline

- Keep package boundaries sharp. Do not move transport logic into executor, SQL policy into API, or provider logic into workflow code.
- Prefer small, composable types over large god files.
- If a change crosses `llmkit` and `orc`, make the ownership explicit before coding.
- When a change updates behavior, update the nearest tests and any durable docs that describe that behavior.

## Verification

Minimum bar for relevant changes:

```bash
go test ./...
pnpm -C web exec tsc --noEmit
```

When changing `llmkit` behavior that `orc` depends on:

1. Commit and tag `llmkit`.
2. Update `orc` to the published tag.
3. Remove any local `replace` used only for development.
4. Re-run validation against the tagged dependency.

## Documentation Standard

AGENTS files should describe:

- ownership
- invariants
- extension points
- verification

They should not be codebase snapshots with line numbers, exhaustive file inventories, or behavior that will drift after normal refactors.
