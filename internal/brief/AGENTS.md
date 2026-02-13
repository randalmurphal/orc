# Brief Package

Auto-generated project context briefs from task history. Summarizes decisions, review findings, and patterns into a token-budgeted document injected into phase prompts via `{{PROJECT_BRIEF}}`.

## File Structure

| File | Purpose |
|------|---------|
| `brief.go` | `Generator`, `Brief`/`Section`/`Entry` types, `Config`, generation orchestration |
| `cache.go` | File-based JSON cache with staleness detection |
| `extract.go` | `ExtractDecisions` (from initiatives), `ExtractFindings` (high-severity review issues) |
| `format.go` | `FormatBrief` renders brief as structured markdown |
| `tokens.go` | Token estimation, per-section and total budget enforcement |

## Key Types

| Type | Purpose |
|------|---------|
| `Generator` | Produces briefs from task history via `Generate(ctx)` |
| `Brief` | Generated output: sections, token count, task count, timestamp |
| `Section` | Category-grouped entries (decisions, recent_findings, hot_files, patterns, known_issues) |
| `Entry` | Single context item: content, source (task/initiative ID), impact score |
| `Cache` | File-backed JSON cache with `IsStale(taskCount, threshold)` |
| `Config` | MaxTokens, per-section budgets, cache path, stale threshold |

## Data Flow

```
Generator.Generate(ctx)
  1. Count completed tasks
  2. Check cache (return if fresh)
  3. ExtractDecisions() ── active initiatives' decisions
  4. ExtractFindings()  ── high-severity review issues from completed tasks (max 10)
  5. ApplyTokenBudget() ── per-section limits (highest impact kept)
  6. ApplyTotalBudget() ── global limit (lowest impact removed first)
  7. Store in cache
  8. Return Brief
```

## Token Budgeting

Token estimation uses ~4 chars/token. Budgets are enforced at two levels:

| Level | Function | Behavior |
|-------|----------|----------|
| Per-section | `ApplyTokenBudget(section, budget)` | Sorts by impact descending, keeps highest-impact entries within budget |
| Global | `ApplyTotalBudget(sections, maxTokens)` | Removes lowest-impact entries across all sections until total fits |

Default budgets (`DefaultConfig()`):

| Section | Budget |
|---------|--------|
| decisions | 800 |
| hot_files | 600 |
| recent_findings | 600 |
| patterns | 500 |
| known_issues | 500 |
| **Total max** | **3000** |

## Cache Behavior

- **Location**: `.orc/brief-cache.json` in working directory
- **Format**: JSON-serialized `Brief`
- **Staleness**: Cache is stale when `currentCompletedTasks - cachedTaskCount >= stale_threshold` (default: 3)
- **Invalidation**: `Generator.Invalidate()` or `orc brief --regenerate`
- **Corrupt cache**: Treated as missing (returns nil, no error)

## Configuration

In `.orc/config.yaml`:

```yaml
brief:
  max_tokens: 3000      # Total token budget (default: 3000)
  stale_threshold: 3     # New completed tasks before regeneration (default: 3)
```

Maps to `config.BriefConfig` at `internal/config/config_types.go:846`.

## Integration Points

| Consumer | How |
|----------|-----|
| `executor/workflow_context.go` | `populateProjectBrief()` generates brief, stores in `rctx.ProjectBrief` |
| `variable/resolver.go` | `PROJECT_BRIEF` built-in variable from `ResolutionContext.ProjectBrief` |
| `cli/cmd_brief.go` | `orc brief` command (show, `--regenerate`, `--json`, `--stats`) |
| `api/brief_server.go` | `GetProjectBrief` and `RegenerateProjectBrief` RPC endpoints |

The executor lazily creates a `Generator` on first use and reuses it across phases within a run (preserving cache).

## CLI Usage

```bash
orc brief                # Show formatted brief
orc brief --regenerate   # Force cache invalidation and regeneration
orc brief --json         # Output as JSON
orc brief --stats        # Show metadata (task count, token count, timestamp)
```
