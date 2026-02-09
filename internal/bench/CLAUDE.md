# Bench Package

Benchmarking system for comparing model configurations across workflow phases. Determines which model combinations produce the best workflow outcomes using cascade testing with frozen inputs.

**Design spec:** `docs/specs/BENCHMARK_SYSTEM.md`

## Architecture

**Self-contained package** with own SQLite database (`~/.orc/bench/bench.db`). Uses `WorkflowExecutor` with `ContextStandalone` mode — gets real phase iteration, loop-back, variable resolution, and prompt rendering without task lifecycle overhead. In-memory backend + environment variable overrides provide isolation from production data.

### Core Concept: Cascade Testing

1. Run all-Opus baseline, freeze every phase's output
2. For variant runs, frozen outputs provide consistent **inputs** to the variant's model change
3. Everything from the first overridden phase onwards runs **live** — the model change cascades through all downstream phases (including implement, review, docs)
4. Compare results across variants with statistical tests

**Key rule:** Frozen outputs are for providing INPUT data, not replacing OUTPUT phases. Phases that produce filesystem changes (`implement`, `review`, `docs`) always run live.

| Phase Type | Examples | Freezable? | Why |
|------------|----------|------------|-----|
| Data-only | spec, tiny_spec, tdd_write, tdd_integrate, breakdown | Yes (before first override) | Produce text artifacts consumed by template variables |
| Side-effect | implement, review, docs | Never | Produce filesystem changes or evaluate state |

**Example — spec variant on medium workflow:**
- `spec` → LIVE with variant model (overridden)
- `tdd_write` → LIVE with default Opus (after override, cascade)
- `tdd_integrate` → LIVE with default Opus (cascade)
- `implement` → LIVE with default Opus (side-effect, always live)
- `review` → LIVE with default Opus (side-effect, always live)
- `docs` → LIVE with default Opus (side-effect, always live)

**Future option:** Phase-specific judging (per-phase rubrics) could be added if cascade benchmarks don't provide enough signal to differentiate model quality at individual phases.

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
| `runner.go` | 636 | Core orchestration: baseline/variant modes, cascade logic, WorkflowExecutor delegation, eval wiring |
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
  3. Compute cascade decisions      → frozen phases (prePopulated map) vs live phases
  4. buildPhaseOverrides()          → per-phase model/provider/thinking config
  5. buildTaskVariables()           → TASK_ID, TASK_TITLE, etc. for ContextStandalone
  6. WorkflowExecutor.Run()         → delegates to real executor with:
     - WithPrePopulatedPhaseOutputs → frozen phases skip execution, inject content
     - WithPhaseModelOverrides      → variant model config per phase
     - WithMaxLoopOverride(1)       → cap review→implement to 1 retry
     - WithMaxTurnsOverride(0)      → unlimited LLM turns
     - WithSkipGates(true)          → no gate evaluation
     - ContextStandalone            → no task lifecycle, claims, heartbeat
  7. savePhaseResults()             → map executor results to bench PhaseResults + save frozen
  8. captureModelDiff()             → diff before eval modifies test files
  9. evaluator.RunAll()             → tests, lint, build, security
 10. Populate run eval metrics      → TestPass, BuildSuccess, etc.
 11. SaveRun() with status          → pass/fail/error
 12. workspace.CleanupRun()
```

### Variable Flow (ContextStandalone)

```
buildTaskVariables() → map[string]string{TASK_ID, TASK_TITLE, ...}
  → WorkflowRunOptions.Variables
    → buildResolutionContext() sets rctx.Environment
      → addBuiltinVariables() sets vars from rctx (empty in standalone)
      → environment override loop overwrites with non-empty values
        → {{TASK_ID}} etc. render correctly in templates
```

## Tier-Aware Variant Scoping

Variants only run against tasks whose workflow contains the overridden phase. Derived automatically from `PhaseApplicableTiers` in `runner.go`.

| Overridden Phase | Applicable Tiers | Tasks (8/tier) |
|------------------|------------------|----------------|
| `implement` | trivial, small, medium, large | 32 |
| `spec` | medium, large | 16 |
| `tdd_write` / `tdd_integrate` | medium, large | 16 |
| `review` / `docs` | small, medium, large | 24 |
| `breakdown` | large | 8 |
| `tiny_spec` | small | 8 |

Baseline (no overrides) always runs all 32 tasks.

**Workflow selection** is tier-based (`tierToWorkflow` map), NOT variant-based. The variant's `base_workflow` field is a fallback/documentation value — the runner always picks the workflow matching the task's tier.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Own database, not GlobalDB | Benchmark data is orthogonal to project data |
| WorkflowExecutor with ContextStandalone | Gets real phase iteration, loops, variable resolution without task lifecycle |
| In-memory backend for executor | No cross-run state needed, avoids polluting production DBs |
| Environment variable override for builtins | Bench injects TASK_ID etc. via `opts.Variables` → `rctx.Environment` → builtin override |
| YAML config for variants | Adding models = editing `suite.yaml`, no code changes |
| Own git workspace management | Benchmark repos are external clones, not user worktrees |
| Run-level evaluation | Eval is inherently run-level: tests run once after all phases |
| Paired statistical tests | Data is paired (same tasks across variants), not independent |
| Cascade mode, not strict isolation | Measures real workflow outcomes: how spec quality cascades to implementation quality |
| Freeze data phases only | implement/review/docs produce filesystem changes — freezing them produces zero signal |

## Judge Panel (`judge.go`)

Evaluates implementation quality only — we're testing orchestrations, not models. Judges don't see test results or know which models ran.

**2 frontier judges** with extended reasoning — both evaluate every run:
- Opus 4.6 (extended thinking, `MAX_THINKING_TOKENS=31999`)
- GPT-5.3-Codex (xhigh reasoning effort)

**Rubric:** `functional_correctness`, `completeness`, `code_quality`, `minimal_change` (1-5 each, with score anchors for calibration)

| Guard | Implementation |
|-------|---------------|
| **Content blinding** | Regex strips model names, provider refs, co-author lines (`blindingPatterns`) |
| **Identity blinding** | Prompt says "a developer" not "AI model" |
| **No test anchoring** | Judges don't see test pass/fail — form independent correctness assessment |
| **Context isolation** | Bug description in `.bench/context.md` file, not inline in prompt |
| **Mixed workflows** | Workflows use multiple models per phase — judge can't attribute output to any single model |

No self-exclusion — both judges evaluate every run. Self-eval bias is mitigated by blinding. Two frontier opinions on every run is more valuable than dropping to one.

**Key insight:** Test pass/fail is unreliable ground truth — reference PR tests assume a specific solution. Judge correctness scores catch valid alternative solutions. Cross-reference judge + tests in the report.

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

orc bench judge                           # Run frontier judge panel (2 judges per run)
```

## Testing

```bash
go test ./internal/bench/... -v    # 44+ tests, ~0.15s
```

Uses `OpenInMemory()` for in-memory SQLite in tests. Runner tests use `WithTurnExecutor()` to inject mock executors (flows through to `executor.WithWorkflowTurnExecutor`).

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
