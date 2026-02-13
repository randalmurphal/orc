# Benchmark System Design

## Goal

Built-in benchmarking system (`orc bench`) that measures which model configurations perform best at which workflow phases, across real-world codebases in multiple languages, producing statistically valid comparisons with cost-per-success tracking.

## Approach

Phase-isolation testing using frozen outputs from a baseline run. Swap one phase's model at a time, keep everything else identical, compare results. Assemble the optimal workflow config from per-phase winners, then validate end-to-end.

## Success Criteria

- [ ] `orc bench curate` manages test projects (pinned repos) and tasks (SWE-bench style from real PRs)
- [ ] `orc bench run` executes benchmark suites with throttling, parallelism, and frozen-output replay
- [ ] `orc bench report` produces phase leaderboards, optimal workflow recommendations, and uncertainty maps
- [ ] Automated evaluation: test pass/fail, regression, lint, coverage, security scan, tokens/time/cost
- [ ] Cross-model judge panel: blinded, randomized, consensus-scored qualitative evaluation
- [ ] Statistical comparison: paired tests with p-values, confidence intervals, targeted re-run suggestions
- [ ] Results are reproducible: pinned commits, pinned model versions, version-controlled configs

## Key Decisions

- **Phase isolation over full combinatorial**: Test each phase independently with frozen inputs. 4 models x 6 phases = ~26 targeted tests, not 4^6 = 4,096 combinations.
- **SWE-bench methodology for tasks**: Real closed PRs from real repos. Check out pre-fix commit, model gets issue description, success = tests pass including new tests from the PR.
- **Cross-model judge panel over single judge**: Opus judges GPT outputs, GPT judges Claude outputs, Sonnet judges everything (tiebreaker). Blinded, randomized. No "neutral" model needed.
- **2 trials initially, 3rd on demand**: Start lean, add targeted trials where results are ambiguous (p > 0.10).
- **Throttle Claude, parallelize Codex**: Different APIs, different rate limits. ~60% of runs hit OpenAI only.
- **Task curation is manual**: Browse real repos, find good PRs, validate them. LLM-assisted triage but human selection. Done in direct sessions, not orc tasks.

## Non-Goals

- Research-paper-level statistical rigor (we want actionable answers, not publication)
- Testing models we don't use (no DeepSeek, Gemini, Llama)
- Benchmarking prompt engineering (prompts are held constant; we're testing model capability)
- Real-time continuous benchmarking (batch runs when new models drop)

## Constraints

- Claude Max subscription rate limits (throttle parallel Claude runs)
- GPT-5.3-Codex available via Codex CLI (already wired into orc)
- ~1,600 total phase executions for initial suite (~10-15 hours wall clock with throttling)
- 32 curated tasks across 4 projects (expandable)

---

## Architecture

### CLI Commands

```
orc bench curate    → Manage test projects and tasks
orc bench run       → Execute benchmark suite
orc bench report    → Analyze and compare results
```

### Data Model

```
benchmark_suite
  ├── projects[]          (forked repos at pinned commits)
  │   ├── repo_url
  │   ├── commit_hash
  │   ├── language
  │   ├── test_cmd, lint_cmd, build_cmd, security_cmd
  │   └── tasks[]
  │       ├── id, tier (trivial/small/medium/large)
  │       ├── issue_url, description
  │       ├── pre_fix_commit
  │       ├── reference_pr_url, reference_diff
  │       ├── fail_to_pass_tests[]    (new tests from the PR)
  │       └── pass_to_pass_tests[]    (existing tests that must not break)
  │
  ├── variants[]          (workflow configs with per-phase model assignments)
  │   ├── name (e.g., "codex-high-implement")
  │   └── phase_overrides map[phase] → {provider, model, reasoning_effort, thinking}
  │
  ├── runs[]              (execution records)
  │   ├── variant_id, task_id, trial_number
  │   ├── status (pass/fail/error)
  │   └── phase_results[]
  │       ├── phase_id
  │       ├── model, provider, reasoning_effort
  │       ├── tokens (input, output, reasoning, cache)
  │       ├── cost_usd, duration_ms
  │       ├── test_pass, test_count, regression_count
  │       ├── lint_warnings, coverage_delta
  │       ├── security_findings
  │       └── frozen_output_id (if replayed from cache)
  │
  ├── frozen_outputs[]    (cached phase outputs for controlled replay)
  │   ├── task_id, phase_id, variant_id, trial_number
  │   ├── output_content (structured JSON)
  │   └── output_artifacts (diffs, files, etc.)
  │
  └── judgments[]         (cross-model qualitative evaluations)
      ├── task_id, phase_id, variant_id
      ├── judge_model, judge_provider
      ├── scores map[dimension] → 1-5
      ├── reasoning (judge's explanation)
      └── presentation_order (for bias tracking)
```

### Storage

Benchmark data lives in the project database alongside existing orc data. New tables:
- `bench_projects` — test project definitions
- `bench_tasks` — curated tasks with reference solutions
- `bench_variants` — model configuration variants
- `bench_runs` — execution records with metrics
- `bench_frozen_outputs` — cached phase outputs
- `bench_judgments` — cross-model evaluation scores

---

## Test Projects

| Language | Project | Repo | Why |
|----------|---------|------|-----|
| Go | bbolt | etcd-io/bbolt | B+ tree ops, transactions, page management. ~8K LoC. Excellent tests. |
| TypeScript | zod | colinhacks/zod | Schema validation, deep TS type inference. ~15K LoC. Incredible test suite. |
| Python | httpx | encode/httpx | Async HTTP client. ~15K LoC. Excellent coverage. Real protocol problems. |
| Rust | ripgrep | BurntSushi/ripgrep | CLI search tool. ~20K LoC. Regex, I/O, CLI parsing. Strong tests. |

### Task Distribution

2 tasks per tier per project = 32 tasks total:

| Tier | Tasks/Project | Total | Workflow | Phases Tested |
|------|--------------|-------|----------|---------------|
| trivial | 2 | 8 | trivial | implement |
| small | 2 | 8 | small | spec, implement, review, docs |
| medium | 2 | 8 | medium | spec, tdd_write, tdd_integrate, implement, review, docs |
| large | 2 | 8 | large | spec, tdd_write, tdd_integrate, breakdown, implement, review, docs |

### Task Curation Process (SWE-bench style)

For each task:
1. Find a closed PR with both code changes AND test changes
2. Record `pre_fix_commit` (the commit before the fix)
3. Task description = issue text
4. Reference solution = PR diff
5. Validation:
   - Check out `pre_fix_commit`, verify test suite passes (baseline health)
   - Apply reference solution, verify new tests pass (solution works)
   - If either fails, reject the task

Expect to review ~100+ PRs across 4 projects to find 32 good ones.

---

## Model Variant Matrix

Claude models use thinking mode. Codex models vary reasoning effort.

### Per-Phase Candidates

| Phase | Opus (thinking) | Sonnet (thinking) | 5.3 xhigh | 5.3 high | 5.3 medium | 5.2 high | 5.2 medium |
|-------|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| spec | x | x | x | x | | | |
| tdd_write | x | x | | x | | x | |
| tdd_integrate | x | x | | x | x | | x |
| breakdown | x | x | | x | | | |
| implement | x | x | x | x | | x | |
| review | x | | x | x | | | |
| docs | | x | | | x | | x |

### Total Phase Executions (2 trials)

| Phase | Variants | Applicable Tasks | Runs/Variant | Phase Total |
|-------|----------|-----------------|-------------|-------------|
| spec | 4 | 24 (no trivial) | 48 | 192 |
| tdd_write | 4 | 16 (medium+large) | 32 | 128 |
| tdd_integrate | 5 | 16 (medium+large) | 32 | 160 |
| breakdown | 3 | 8 (large only) | 16 | 48 |
| implement | 5 | 32 (all tiers) | 64 | 320 |
| review | 3 | 24 (no trivial) | 48 | 144 |
| docs | 3 | 24 (no trivial) | 48 | 144 |
| **Total** | | | | **~1,136** |

Plus ~200 judge panel evaluations (batched, cheap).

At ~3 min average per phase = ~57 hours compute. Parallelized with throttling = ~12-15 hours wall clock.

---

## Execution Flow

### Step 1: Baseline Run

```bash
orc bench run --suite full --baseline
```

Runs all-Opus-thinking through every task (all tiers, all phases). This produces:
- Baseline metrics for comparison
- Frozen phase outputs used as input for variant testing

### Step 2: Variant Runs (Phase Isolation)

```bash
orc bench run --suite full --variant codex53-high-implement
```

For each variant:
1. Load frozen outputs from baseline for all phases EXCEPT the one being tested
2. Execute only the target phase with the variant's model config
3. Run automated evaluation (tests, lint, coverage, security)
4. Record all metrics

Codex variants run in parallel (different API). Claude variants throttled.

### Step 3: Judge Panel (Batched)

```bash
orc bench judge --suite full
```

After all variants complete:
1. Collect outputs per task per phase
2. Strip model identifiers, randomize order
3. Send to cross-model judges:
   - Opus judges GPT outputs only
   - GPT-5.3 judges Claude outputs only
   - Sonnet judges everything (cheap tiebreaker)
4. Score on phase-specific rubric (1-5 per dimension)
5. Compute consensus, flag disagreements > 2 points

### Step 4: Analysis & Reporting

```bash
orc bench report --suite full
```

Produces:
- **Phase Leaderboard**: Per-phase rankings with pass rate, scores, cost, time, p-values
- **Optimal Workflow Config**: Recommended model per phase per tier with cost savings estimate
- **Uncertainty Map**: Where results are inconclusive, with commands to run targeted extensions

### Step 5: Targeted Extension (Optional)

```bash
orc bench run --suite full --extend implement:opus,gpt53-xhigh --trials 3
```

Adds trials only where the uncertainty map shows ambiguous results.

---

## Evaluation Pipeline

### Stage 1: Automated Metrics (every run, immediate)

| Metric | Phases | Method |
|--------|--------|--------|
| Tests pass (FAIL_TO_PASS) | implement, tdd | Run test suite, check new tests pass |
| No regressions (PASS_TO_PASS) | implement | Run test suite, check existing tests still pass |
| Build succeeds | all code phases | Run build command |
| Lint clean | implement | Run linter, count new warnings |
| Coverage delta | tdd_write, tdd_integrate | Coverage before vs after |
| Security scan | implement | SAST tooling, count new findings |
| Token usage | all | Input + output + reasoning (from orc cost tracking) |
| Wall-clock time | all | Phase duration |
| Cost USD | all | Computed from token usage + provider rates |

### Stage 2: Reference Comparison (implement phase)

| Metric | Method |
|--------|--------|
| Files touched overlap | Compare changed files vs reference PR |
| Diff size ratio | Lines changed vs reference (over/under-engineering signal) |
| Semantic similarity | AST-level comparison where tooling exists |

Informational only — two valid solutions can look very different.

### Stage 3: Cross-Model Judge Panel (batched)

**Phase-specific rubrics:**

| Phase | Dimensions (1-5 each) |
|-------|----------------------|
| spec | Completeness, testability, clarity, specificity |
| tdd_write | Assertion meaningfulness, edge case coverage, no tautologies, readability |
| tdd_integrate | Wiring correctness, dead code prevention, production path coverage |
| implement | Code quality, pattern adherence, minimality, readability |
| review | Bug detection accuracy, false positive avoidance, actionability, severity accuracy |
| docs | Accuracy, completeness, clarity |

**Judge protocol:**
- Outputs blinded (no model names)
- Presentation order randomized per evaluation
- Each judge scores independently
- Consensus: 2/3 agree = use majority; 3-way split = flag for human review

### Downstream Success Metric (spec phase)

Spec quality is also measured indirectly: for each spec variant, run the SAME implementation model (e.g., Opus) and measure implementation success rate. A better spec → higher downstream pass rate. This is the strongest signal for spec quality because it measures actual utility, not just surface quality.

---

## Statistical Analysis

### Primary Method: Paired Comparison

All variants run the same tasks, enabling paired analysis:
1. Compute per-task delta (Variant A score - Variant B score)
2. BCa bootstrap confidence intervals on deltas (200+ rounds)
3. Wilcoxon signed-rank test for continuous scores
4. McNemar's test for binary pass/fail

### Significance Thresholds

| p-value | Interpretation | Action |
|---------|---------------|--------|
| < 0.05 | Significant | Declare winner |
| 0.05 - 0.10 | Marginal | Consider cost difference as tiebreaker |
| > 0.10 | Inconclusive | Run more trials or declare equivalent |

### Key Metrics Reported

| Metric | Per Phase | Per Workflow |
|--------|----------|-------------|
| Pass rate (pass@2) | x | x |
| Reliability (pass^2) | x | x |
| Mean judge score | x | |
| Mean cost per attempt | x | x |
| **Cost per success** | x | x |
| Mean wall-clock time | x | x |
| p-value vs baseline | x | x |

### Cost Per Success (The North Star)

```
cost_per_success = total_cost / successful_completions
```

This single metric captures the trade-off between quality and cost. A model that's 50% cheaper but fails twice as often is actually more expensive per success.

---

## Workflow-Tier Specific Testing

Different tiers test different hypotheses:

| Tier | Hypothesis | What We Learn |
|------|-----------|---------------|
| trivial | Cheaper models are fine for one-liners | Floor: minimum viable model for simple tasks |
| small | Sonnet/5.3-medium might be sufficient | Where the "good enough" line is |
| medium | This is where model choice matters most | Optimal config for typical feature work |
| large | Only strong models succeed here | Ceiling: which models can handle complexity |

The optimal workflow config may differ by tier:
```
trivial: all-sonnet (cheapest that works)
small:   sonnet-spec, codex-implement, opus-review
medium:  opus-spec, codex-tdd, codex-implement, opus-review
large:   opus-everything (can't afford failures)
```

---

## Implementation Plan

### Phase 1: Infrastructure (build the system)
- `orc bench curate` — project/task management commands
- `orc bench run` — execution engine with frozen outputs, throttling, parallelism
- Benchmark database schema (new tables)
- Automated evaluation pipeline (test runner, lint, coverage, security)

### Phase 2: Evaluation (make it smart)
- Cross-model judge panel infrastructure
- Phase-specific rubric prompts
- Statistical analysis and reporting
- `orc bench report` — leaderboards, recommendations, uncertainty maps

### Phase 3: Task Curation (the manual work)
- Browse bbolt, zod, httpx, ripgrep issue trackers
- Select and validate 32 tasks (8 per project, 2 per tier)
- Write task descriptions, pin commits, validate reference solutions

### Phase 4: First Run
- Execute baseline (all-Opus)
- Run all variants
- Generate first report
- Run targeted extensions for ambiguous results

### Phase 5: Iterate
- Add tasks where coverage is thin
- Re-run when new models drop (GPT-5.3 API, Claude updates)
- Refine rubrics based on judge disagreements

---

## Testing Strategy

- Unit tests for benchmark runner logic (variant matrix generation, frozen output replay)
- Integration tests using a tiny mock project (3 files, simple tests)
- E2E test: single task, 2 variants, full pipeline including judge panel
- Statistical analysis validated against known datasets with known outcomes
