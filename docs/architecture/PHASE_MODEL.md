# Phase Model

**Purpose**: Define phase types, templates, and transition rules.

---

## Plan Regeneration on Weight Change

When a task's weight changes (via `orc edit --weight` or manual file edit), the plan is automatically regenerated to match the new weight's phase sequence.

### Regeneration Behavior

| Scenario | Behavior |
|----------|----------|
| Task not running | Plan regenerated immediately |
| Task running | Regeneration skipped (would disrupt execution) |
| Plan already matches weight | Regeneration skipped (idempotent) |

### Phase Status Preservation

Completed and skipped phases are preserved when regenerating:

| Old Plan Phase | New Plan Phase | Result |
|----------------|----------------|--------|
| `implement: completed` | Has `implement` | Status preserved |
| `spec: skipped` | Has `spec` | Status preserved |
| `spec: completed` | No `spec` phase | Status lost (phase not in new plan) |
| N/A | New `validate` phase | Status = `pending` |

### Triggers

Plan regeneration can be triggered by:

1. **CLI**: `orc edit TASK-001 --weight large`
2. **API**: PATCH `/api/tasks/{id}` with `{"weight": "large"}`
3. **Manual edit**: Update task weight via CLI/API

All three methods produce identical results.

---

## Phase Types

| Phase | Purpose | Produces | Commit |
|-------|---------|----------|--------|
| `classify` | Determine task weight | weight assignment | Optional |
| `research` | Investigate codebase | research.md | Yes |
| `spec` | Define requirements | spec content (database) | Yes |
| `tiny_spec` | Combined spec+TDD for small tasks | spec + test plan | Yes |
| `design` | Architecture decisions | design.md | Yes |
| `tdd_write` | Write failing tests before implementation (classifies solitary/sociable/integration, requires integration tests for wiring) | test files + test plan | Yes |
| `breakdown` | Decompose spec into checkboxed implementation steps | breakdown content | Yes |
| `implement` | Write code | code changes | **Yes** |
| `review` | Code review | review findings + fixes | Yes |
| `docs` | Create/update documentation | README.md, CLAUDE.md, etc. | Yes |
| `test` | Write and run tests | test results | Yes |
| `validate` | Final verification | validation report | Yes |
| `finalize` | Sync with main, conflict resolution | finalization report | Yes |
| `merge` | Merge to target branch | merged code | Yes (final) |

### Docs Phase Details

The `docs` phase runs **after implementation and review**, with full context of what changed:

| Weight | Docs Behavior |
|--------|---------------|
| trivial | Skip (unless missing CLAUDE.md at root) |
| small | Update affected README sections only |
| medium | Create missing docs, update affected existing |
| large | Full doc audit, create all missing, update all affected |
| greenfield | Create complete doc structure from templates |

See [DOCUMENTATION.md](../specs/DOCUMENTATION.md) for full specification.

### Finalize Phase Details

The `finalize` phase runs **after validate** to prepare the branch for merge. Key responsibilities:

| Step | Purpose |
|------|---------|
| Sync with target | Merge main (or target branch) into task branch |
| Conflict resolution | Resolve any conflicts following strict rules |
| Test verification | Re-run tests after conflict resolution |
| Risk assessment | Classify merge risk based on diff size |

#### Conflict Resolution Rules

**Critical constraints** during conflict resolution:

| Rule | Description |
|------|-------------|
| **NEVER remove features** | Both task changes AND upstream changes must be preserved |
| **Merge intentions** | Understand what each side was trying to accomplish |
| **Prefer additive** | When in doubt, keep both implementations |
| **Test per file** | Run tests after resolving each conflicted file |

#### Risk Classification

| Files Changed | Lines Changed | Risk Level |
|---------------|---------------|------------|
| 1-5 | <100 | Low (auto-merge safe) |
| 6-15 | 100-500 | Medium (review recommended) |
| 16-30 | 500-1000 | High (careful review required) |
| >30 | >1000 | Critical (senior review mandatory) |

Conflicts also contribute to risk: >10 conflicts is High, >3 is Medium.

#### Sync Strategies

| Strategy | Behavior | When to Use |
|----------|----------|-------------|
| `merge` (default) | Merge target into task branch | Preserves full commit history |
| `rebase` | Rebase task branch onto target | Linear history, cleaner graph |

Configure via `completion.finalize.sync.strategy` in `config.yaml`.

#### Auto-Trigger on PR Approval

In `auto` automation profile, finalize is automatically triggered when a PR is approved:

| Trigger | Mechanism |
|---------|-----------|
| PR status poller | Polls every 60s, detects `approved` status |
| Auto-trigger callback | `TriggerFinalizeOnApproval` called when PR becomes approved |
| Rate limiting | 30s minimum between polls for same task |

**Conditions:**
- `completion.finalize.auto_trigger_on_approval` is `true` (default for auto profile)
- Task has weight supporting finalize (not `trivial`)
- Task status is `completed` (has PR)
- Finalize hasn't already completed

**Disable auto-trigger:**
```yaml
completion:
  finalize:
    auto_trigger_on_approval: false
```

#### Test Fix Loop

After sync, if tests fail:
1. Claude attempts to fix test failures (up to 5 turns)
2. Tests re-run after fix attempt
3. If still failing, escalates to implement phase

#### Escalation to Implement Phase

Finalize escalates back to implement when issues persist:

| Condition | Action |
|-----------|--------|
| >10 unresolved conflicts | Escalate with conflict list |
| >5 test failures after fix attempts | Escalate with test failure details |
| Complex merge conflicts | Escalate for manual resolution |

The implement phase receives retry context via `{{RETRY_CONTEXT}}` containing:
- List of conflicted files that couldn't be resolved
- Test failures with error messages
- Guidance to fix and retry finalize

#### Finalize Output Report

On completion, finalize produces a markdown report:
- Sync summary (target branch, conflicts resolved)
- Test results (passed/failed)
- Risk assessment table with per-factor breakdown
- Merge recommendation (auto-merge, review-then-merge, senior-review-required)
- Final commit SHA

### Phase Commit Requirement

**Every phase that produces artifacts or code changes MUST commit before marking complete.**

This is critical for:
- **Rollback capability**: Rewind to any phase start
- **Audit trail**: Track what changed when
- **Parallel safety**: Worktrees don't conflict
- **Recovery**: Resume from any checkpoint

#### Commit Message Format

```
[orc] TASK-ID: phase - status

Phase: phase-name
Status: completed|failed
Artifact: [path if applicable]
```

Example:
```
[orc] TASK-001: implement - completed

Phase: implement
Status: completed
Files changed: 5
```

---

## Phase Templates by Weight

### Trivial
```yaml
phases:
  - name: implement
    max_iterations: 3
    checkpoint: false
    gate: auto
```

### Small
```yaml
phases:
  - name: implement
    max_iterations: 5
    checkpoint: true
    gate: auto
  - name: test
    max_iterations: 3
    gate: ai
```

### Medium
```yaml
phases:
  - name: spec
    max_iterations: 3
    gate: ai
  - name: implement
    max_iterations: 10
    checkpoint_frequency: 3
    gate: auto
  - name: review
    max_iterations: 3
    gate: ai
  - name: docs
    max_iterations: 3
    gate: auto
  - name: test
    max_iterations: 3
    gate: auto
```

### Large
```yaml
phases:
  - name: research
    max_iterations: 5
    gate: auto
  - name: spec
    max_iterations: 5
    gate: human
  - name: design
    max_iterations: 3
    gate: human
  - name: implement
    max_iterations: 20
    checkpoint_frequency: 5
    gate: auto
  - name: review
    max_iterations: 5
    gate: ai
  - name: docs
    max_iterations: 5
    gate: auto
  - name: test
    max_iterations: 5
    gate: auto
  - name: validate
    max_iterations: 2
    gate: ai
  - name: finalize
    max_iterations: 3
    gate: auto
```

### Greenfield
```yaml
phases:
  - name: research
    max_iterations: 10
    gate: human
  - name: spec
    max_iterations: 10
    gate: human
  - name: design
    max_iterations: 5
    gate: human
  - name: implement
    max_iterations: 30
    checkpoint_frequency: 5
    staged: true
    gate: auto
  - name: review
    max_iterations: 5
    gate: ai
  - name: docs
    max_iterations: 10
    gate: ai
  - name: test
    max_iterations: 10
    gate: auto
  - name: validate
    max_iterations: 3
    gate: human
  - name: finalize
    max_iterations: 3
    gate: auto
```

---

## Phase State

```yaml
# Per-phase state in state.yaml
phases:
  implement:
    status: running        # pending | running | completed | failed | skipped
    started_at: 2026-01-10T10:45:00Z
    iterations: 3
    last_checkpoint: abc123
    artifacts:
      - path: src/auth/login.go
        action: created
      - path: src/auth/login_test.go
        action: created
```

---

## Phase Transitions

```
        ┌─────────────────────────────────────┐
        │                                     │
        ▼                                     │
    ┌───────┐     ┌───────┐     ┌───────┐    │
    │pending│────►│running│────►│complete│───┘
    └───────┘     └───┬───┘     └───────┘    (next phase)
                      │
              ┌───────┴───────┐
              ▼               ▼
          ┌──────┐       ┌──────┐
          │failed│       │skipped│
          └──────┘       └──────┘
```

| Transition | Condition |
|------------|-----------|
| pending → running | Previous phase complete, gate passed |
| running → complete | Completion criteria met |
| running → failed | Max iterations exceeded or unrecoverable error |
| pending → skipped | User skips phase (`orc skip --phase`) or artifact detected |

---

## Artifact Detection

Before running a task, orc checks if artifacts from previous runs exist. This allows resuming work without re-executing phases that already produced valid artifacts.

### Detection by Phase

| Phase | Artifacts Checked | Auto-Skippable | Why |
|-------|------------------|----------------|-----|
| `spec` | Spec content in database (50+ chars) | Yes | Spec content is reusable |
| `research` | `artifacts/research.md` OR research section in spec | Yes | Research findings persist |
| `docs` | `artifacts/docs.md` | Yes | Documentation is reusable |
| `implement` | Never detected | No | Code state too complex to validate |
| `test` | `test-results/report.json` | No | Tests must re-run against current code |
| `validate` | `artifacts/validate.md` | No | Validation must verify current state |
| `finalize` | Never detected | No | Must sync with latest target branch |

**Note**: Spec content is stored in the database (not as `spec.md` file) to avoid merge conflicts in worktrees. Legacy `spec.md` files are still detected for backward compatibility.

### Behavior

**Default (interactive)**: Prompts user for each detected artifact:
```
📄 Spec content already exists. Skip spec phase? [Y/n]:
```

**With `--auto-skip` flag**: Automatically skips phases with existing artifacts.

**Configuration** (`config.yaml`):
```yaml
artifact_skip:
  enabled: true            # Enable artifact detection (default: true)
  auto_skip: false         # Skip without prompting (default: false)
  phases:                  # Phases to check (default: [spec, research, docs])
    - spec
    - research
    - docs
```

### Skip Recording

When a phase is skipped due to existing artifacts:

1. Phase status set to `skipped` in `state.yaml`
2. Skip reason recorded in `error` field with `"skipped: "` prefix
3. Gate decision recorded with `type: skip` for audit trail
4. `completed_at` timestamp set (no `started_at` since phase didn't run)

Example `state.yaml`:
```yaml
phases:
  spec:
    status: skipped
    completed_at: 2026-01-10T10:31:30Z
    iterations: 0
    error: "skipped: artifact exists: spec content found in database"
```

### Weight-Specific Validation

Spec artifacts are validated against weight requirements:

| Weight | Minimum Spec Requirements |
|--------|--------------------------|
| trivial | 50+ characters |
| small | 50+ characters |
| medium | 100+ characters, sections present |
| large | 200+ characters, full structure |
| greenfield | 300+ characters, full structure |

Specs that don't meet weight requirements are not considered valid artifacts and won't trigger skip prompts.

---

## Phase Configuration

```yaml
# Phase definition in plan.yaml
phases:
  - name: implement
    type: implement
    prompt_template: prompts/implement.md

    # Iteration control
    max_iterations: 10
    timeout: 600s

    # Completion criteria
    completion_criteria:
      - all_tests_pass
      - no_lint_errors

    # Checkpointing
    checkpoint: true
    checkpoint_frequency: 3

    # Gate
    gate: auto
    gate_criteria:
      - tests_pass
```

---

## Linting Requirements

Static analysis and linting are **mandatory** quality gates for phase completion. Linting errors are blocking issues that must be fixed before proceeding.

### Linting by Phase

| Phase | Linting Requirement | Blocking? |
|-------|---------------------|-----------|
| `implement` | Recommended (quick check before commit) | No |
| `test` | **REQUIRED** (full linter suite) | **Yes** |
| `validate` | **REQUIRED** (verify after sync) | **Yes** |
| `finalize` | **REQUIRED** (final gate before merge) | **Yes** |

### Linter Commands by Language

| Language | Linter Command | What It Catches |
|----------|----------------|-----------------|
| **Go** | `golangci-lint run ./...` | errcheck, unused, vet, staticcheck, ineffassign |
| Go (minimal) | `go vet ./...` | Type errors, suspicious constructs |
| **Node/TS** | `bun run typecheck && bun run lint` | Type errors, ESLint rules |
| TS (typecheck) | `tsc --noEmit` or `bun run typecheck` | Type errors only |
| TS (lint) | `bun run lint` | ESLint code quality rules |
| **Python** | `ruff check .` | PEP 8, common bugs, type issues |
| Python (types) | `pyright .` | Type checking |
| Python (alt) | `pylint`, `flake8`, `mypy` | Various rule sets |

### Go Errcheck Requirements

The `errcheck` linter is particularly important for Go code quality. Common patterns:

| Issue | Wrong Pattern | Correct Pattern |
|-------|---------------|-----------------|
| Ignored error return | `functionCall()` | `_ = functionCall()` |
| Deferred close | `defer f.Close()` | `defer func() { _ = f.Close() }()` |
| Multi-return ignore | `val := fn()` | `val, _ := fn()` or `_, _ = fn()` |

**Why errcheck matters:**
- Silent failures lead to hard-to-debug production issues
- Unchecked error returns are a code smell
- Explicit `_ =` documents intentional ignoring

### TypeScript/Node Linting Requirements

TypeScript projects need BOTH type checking AND linting:

| Issue | Wrong Pattern | Correct Pattern |
|-------|---------------|-----------------|
| Unused variable | `const foo = 1;` | Remove or rename to `_foo` |
| Unused catch error | `catch (e) { ... }` | `catch (_e) { ... }` if `e` unused |
| Explicit any | `data: any` | Use proper type or `unknown` |
| React hooks violation | Hook in non-component function | Follow Rules of Hooks |

**Why both checks matter:**
- `tsc`/`typecheck`: Catches type mismatches, missing properties, incorrect function signatures
- ESLint: Catches code quality issues, React patterns, unused code, potential bugs

### Phase Completion Blocking

Linting errors **block phase completion**. If linting fails:

```xml
<phase_blocked>
reason: linting errors found
needs: [list specific linting issues to fix]
</phase_blocked>
```

The phase cannot output `<phase_complete>true</phase_complete>` until linting passes.

### Configuration

Configure linting behavior in `config.yaml`:

```yaml
# Project linting configuration
linting:
  enabled: true              # Enable linting checks (default: true)
  strict: true               # Treat warnings as errors (default: false)

  # Language-specific commands
  commands:
    go: "golangci-lint run ./..."
    typescript: "bun run lint"
    python: "ruff check ."
```

### Integration with CI

Linting in orc phases should match CI requirements:
- Same linter version
- Same configuration file (`.golangci.yml`, `.eslintrc`, `ruff.toml`)
- Same enabled rules

This ensures tasks that pass locally also pass CI after PR creation.

---

## Validate Phase: Playwright MCP E2E Testing

The `validate` phase is the **final verification** before merge. For projects with UI components, this phase **MUST** use Playwright MCP tools for comprehensive end-to-end testing.

### Why Playwright MCP for Validation

| Benefit | Description |
|---------|-------------|
| **Real Browser Testing** | Tests run in actual browser, catching issues unit tests miss |
| **Visual Verification** | Screenshot comparison, accessibility snapshots |
| **Full User Flows** | Complete journeys from login to checkout |
| **Component Isolation** | Every single UI component verified independently |
| **Cross-Browser** | Validate Chrome, Firefox, Safari compatibility |

### Validation Scope

The validate phase tests **every single component** end-to-end:

```yaml
# validate phase completion criteria
completion_criteria:
  - playwright_e2e_pass          # All E2E tests pass
  - all_components_covered       # 100% component coverage in E2E
  - accessibility_validated      # WCAG compliance via snapshots
  - visual_regression_pass       # No unexpected visual changes
```

### Playwright MCP Tools Used

| Tool | Purpose | When Used |
|------|---------|-----------|
| `browser_navigate` | Load pages/routes | Start of each test flow |
| `browser_snapshot` | Accessibility tree capture | Verify component state |
| `browser_click` | User interactions | Button clicks, navigation |
| `browser_fill_form` | Form input | Login, data entry flows |
| `browser_take_screenshot` | Visual verification | Compare against baselines |
| `browser_console_messages` | Error detection | Catch JS errors |
| `browser_network_requests` | API verification | Ensure correct API calls |

### Validation Workflow

```
1. browser_navigate → Load application
2. browser_snapshot → Capture initial state
3. For each component:
   a. Navigate to component
   b. browser_snapshot → Verify accessibility
   c. Test all interactions (click, hover, type)
   d. browser_snapshot → Verify state changes
   e. browser_take_screenshot → Visual baseline
4. browser_console_messages → Check for errors
5. browser_network_requests → Verify API calls
6. Report generation → Produce validation report
```

### Validate Phase Configuration

```yaml
# For large/greenfield tasks
validate:
  type: validate
  prompt_template: prompts/validate.md
  max_iterations: 5
  timeout: 1200s  # Longer timeout for E2E

  completion_criteria:
    - playwright_e2e_pass
    - all_components_covered
    - no_console_errors
    - no_failed_network_requests

  tools:
    - playwright_mcp  # Required for validation

  gate:
    type: human  # Human reviews validation report
    criteria:
      - validation_report_exists
      - all_flows_tested
```

### Example Validation Prompt

```markdown
# Validate Phase

Run comprehensive E2E validation using Playwright MCP tools.

## Required Validations

1. **Every UI Component**: Navigate to and test each component
2. **All User Flows**: Complete login, CRUD, logout sequences
3. **Error States**: Trigger and verify error handling
4. **Edge Cases**: Empty states, max values, special characters

## Using Playwright MCP

For each test:
1. Use `browser_snapshot` to capture accessibility tree
2. Verify all interactive elements are accessible
3. Take screenshots for visual regression baseline
4. Check console for errors after each interaction

## Completion Criteria

Output `<phase_complete>true</phase_complete>` when:
- All components have been tested via Playwright
- All user flows complete successfully
- No console errors detected
- No failed network requests
- Validation report created at artifacts/validation-report.md
```

---

## Custom Phases

Projects can define custom phases:

```yaml
# orc.yaml
custom_phases:
  security_scan:
    type: custom
    prompt_template: prompts/security-scan.md
    command: "./scripts/security-scan.sh"
    success_exit_code: 0
    
  deploy_staging:
    type: custom
    prompt_template: prompts/deploy-staging.md
    gate: human
```

Insert into plan:
```yaml
phases:
  - name: implement
  - name: security_scan    # Custom phase
  - name: review
  - name: deploy_staging   # Custom phase
```
