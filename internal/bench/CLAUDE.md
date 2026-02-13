# Bench Package

Benchmarking system for comparing model configurations across workflow phases. Determines which models perform best at which phases using phase-isolation testing with frozen outputs.

**Design spec:** `docs/specs/BENCHMARK_SYSTEM.md`

## Architecture

**Self-contained package** with own SQLite database (`~/.orc/bench/bench.db`). Does NOT use WorkflowExecutor — benchmarks test model quality, not orc infrastructure. Reuses `executor.TurnExecutor` for LLM dispatch only.

### Core Concept: Phase Isolation

1. Run all-Opus baseline, freeze every phase's output
2. For variant runs, replay frozen outputs for all phases EXCEPT the target
3. Swap one phase's model at a time → isolate effect of that model on that phase
4. Compare results across variants with statistical tests

### Data Model

| Table | Purpose | Key Fields |
|-------|---------|------------|
| `bench_projects` | Pinned repos with test commands | `id`, `repo_url`, `commit_hash`, `test_cmd` |
| `bench_tasks` | SWE-bench style issues from real PRs | `id`, `tier`, `pre_fix_commit`, `fail_to_pass_tests` |
| `bench_variants` | Model configs targeting specific phases | `id`, `phase_overrides` (JSON), `is_baseline` |
| `bench_runs` | Execution records with eval metrics | `id`, `status`, `test_pass`, `cost_usd` |
| `bench_phase_results` | Per-phase metrics within a run | `run_id`, `phase_id`, `was_frozen`, token counts |
| `bench_frozen_outputs` | Cached phase outputs for replay | `task_id`, `phase_id`, `output_content` |
| `bench_judgments` | Cross-model judge evaluations | `run_id`, `judge_model`, `scores` (JSON) |

Schema: `schema/bench_001.sql` (initial), `schema/bench_002.sql` (eval metrics on runs, indexes)

## File Structure

| File | Lines | Purpose |
|------|-------|---------|
| `types.go` | 273 | Core types: `Project`, `Task`, `Variant`, `Run`, `PhaseResult`, `FrozenOutput`, `Judgment` |
| `store.go` | 807 | SQLite CRUD with `db/driver` for dialect compat. Cascade deletes. |
| `config.go` | 149 | YAML `suite.yaml` loading, validation, `ImportToStore()` |
| `workspace.go` | 109 | Git clone + `git worktree add` for per-run isolation |
| `runner.go` | 481 | Core execution: baseline/variant modes, frozen output replay, eval wiring |
| `frozen.go` | 53 | Frozen output load/save, variable injection |
| `evaluator.go` | 270 | Automated evaluation: tests, lint, build, security |
| `judge.go` | 468 | Cross-model judge panel: blinding, randomization, rubrics |
| `stats.go` | 355 | Bootstrap BCa CI, Wilcoxon signed-rank, McNemar's, paired Cohen's d |
| `report.go` | 533 | Phase leaderboard, optimal config recommendation |

## Execution Flow (`runner.go`)

```
Runner.RunSingle(ctx, variant, task, trial)
  1. workspace.SetupRun()           → git worktree at pre-fix commit
  2. LoadFrozenOutputs()            → baseline outputs (skip for baseline runs)
  3. For each phase in workflow:
     a. Frozen? → replay output, inject into vars, continue
     b. Live?   → resolve model config, render prompt, TurnExecutor, save result
     c. Save frozen output for future variant runs
  4. evaluator.RunAll()             → tests, lint, build, security
  5. Populate run eval metrics      → TestPass, BuildSuccess, etc.
  6. SaveRun() with status          → pass/fail/error
  7. workspace.CleanupRun()
```

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Own database, not GlobalDB | Benchmark data is orthogonal to project data |
| Direct TurnExecutor, not WorkflowExecutor | No gates, triggers, PR creation needed |
| YAML config for variants | Adding models = editing `suite.yaml`, no code changes |
| Own git workspace management | Benchmark repos are external clones, not user worktrees |
| Run-level evaluation | Eval is inherently run-level: tests run once after all phases |
| Paired statistical tests | Data is paired (same tasks across variants), not independent |

## Judge Panel (`judge.go`)

Cross-model blind evaluation to prevent self-assessment bias:

| Guard | Implementation |
|-------|---------------|
| **Self-eval prevention** | `shouldJudge()` rejects if judge model == output model |
| **Content blinding** | Regex strips model names, provider refs, co-author lines (`blindingPatterns`) |
| **Randomized order** | `rand.Shuffle` on presentation order, stored for reproducibility |
| **Cross-provider** | Opus judges GPT, GPT judges Claude, Sonnet judges all (tiebreaker) |

## Statistical Analysis (`stats.go`)

| Function | Purpose | When Used |
|----------|---------|-----------|
| `BootstrapCI()` | BCa confidence intervals for cost, duration | Continuous metrics |
| `WilcoxonSignedRank()` | Non-parametric paired comparison | Cost/duration comparison |
| `McNemarTest()` | Paired binary comparison (Yates correction=1.0) | Pass/fail comparison |
| `PairedCohensD()` | Effect size d_z = mean(diffs)/SD(diffs) | Magnitude of difference |
| `ComparePaired()` | Combines all tests into `PairedComparison` | Report generation |

## CLI Commands

```
orc bench curate import suite.yaml       # Bulk import from YAML
orc bench curate add-project ...          # Add a project
orc bench curate add-task ...             # Add a task
orc bench curate list [projects|tasks|variants]
orc bench curate validate                 # Health check

orc bench run --baseline --trials N       # Run all-Opus baseline
orc bench run --variant ID --trials N     # Run specific variant
orc bench run --all-variants --trials N   # Run everything

orc bench report                          # Phase leaderboard + recommendations
orc bench report --phase implement        # Single phase detail
orc bench report --format json            # Machine-readable

orc bench judge                           # Run full judge panel
orc bench judge --phase spec              # Judge single phase
```

## Testing

```bash
go test ./internal/bench/... -v    # 36 tests, ~0.05s
```

Uses `OpenInMemory()` for in-memory SQLite in tests. Runner tests use `WithExecutorFactory()` to inject mock executors.

## File Layout

```
~/.orc/bench/
  bench.db          # Benchmark database
  suite.yaml        # Suite configuration (projects, tasks, variants)
  repos/            # Cloned repos (persistent)
    bbolt/
    zod/
  runs/             # Per-run worktrees (ephemeral)
    <run-uuid>/
```
