# Templates

Embedded templates for plans and prompts.

## Directory Structure

```
templates/
├── embed.go          # Go embed directives
├── plans/            # Weight-based plan templates
│   ├── trivial.yaml, small.yaml, medium.yaml, large.yaml
├── prompts/          # ALL prompts (phase, validation, gates)
│   ├── [phase prompts] classify, research, spec, tiny_spec, design, tdd_write, tasks, implement, review, docs, finalize
│   ├── [validation] haiku_iteration_progress, haiku_task_readiness, haiku_success_criteria
│   ├── [gates] gate_evaluation, conflict_resolution
│   ├── [sessions] spec_session, plan_session, plan_from_spec, setup
│   ├── [review] review_round1, review_round2, qa
│   └── [automation] automation/*.md
└── pr-body.md        # PR description template
```

## Plan Templates (TDD-First Workflow)

| Weight | Phases |
|--------|--------|
| `trivial` | tiny_spec → implement |
| `small` | tiny_spec → implement → review |
| `medium` | spec → tdd_write → tasks → implement → review → docs |
| `large` | spec → tdd_write → tasks → implement → review → docs |

**Key concepts:**
- **TDD-first**: Tests written before implementation (tdd_write phase)
- **All weights get specs**: trivial/small use lightweight `tiny_spec`
- **No separate test phase**: TDD handles testing upfront
- **Medium = Large phases**: Weight provides context, agents adapt accordingly

**Review phase** (small+): Multi-agent code review with 5 specialized reviewers.

**Note**: `finalize` is a manual command (`orc finalize TASK-XXX`), not an automatic phase.
Use it to sync with target branch and resolve conflicts before merge.

## Template Variables

| Variable | Description |
|----------|-------------|
| `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}` | Task context |
| `{{TASK_CATEGORY}}` | feature/bug/refactor/etc |
| `{{PHASE}}`, `{{WEIGHT}}`, `{{ITERATION}}` | Execution context |
| `{{SPEC_CONTENT}}`, `{{DESIGN_CONTENT}}` | Phase artifacts |
| `{{TDD_TESTS_CONTENT}}`, `{{TDD_TEST_PLAN}}` | TDD phase output |
| `{{TASKS_CONTENT}}` | Task breakdown output |
| `{{RETRY_CONTEXT}}` | Failure info on retry |
| `{{WORKTREE_PATH}}`, `{{TASK_BRANCH}}`, `{{TARGET_BRANCH}}` | Git context |
| `{{INITIATIVE_CONTEXT}}` | Initiative details |
| `{{LANGUAGE}}`, `{{HAS_FRONTEND}}`, `{{HAS_TESTS}}` | Project detection |
| `{{TEST_COMMAND}}`, `{{LINT_COMMAND}}`, `{{BUILD_COMMAND}}` | Project commands |
| `{{REQUIRES_UI_TESTING}}`, `{{SCREENSHOT_DIR}}`, `{{TEST_RESULTS}}` | UI testing |
| `{{REVIEW_ROUND}}`, `{{REVIEW_FINDINGS}}` | Review phase |
| `{{VERIFICATION_RESULTS}}` | Implement verification |
| `{{CONSTITUTION_CONTENT}}` | Project principles |

## Phase Prompts

| Phase | Purpose |
|-------|---------|
| `classify.md` | Weight classification |
| `research.md` | Pattern research |
| `spec.md` | Technical specification with user stories and quality checklist |
| `tiny_spec.md` | Combined spec+TDD for trivial/small tasks |
| `design.md` | Architecture (optional, agent decides) |
| `tdd_write.md` | Write failing tests before implementation |
| `tasks.md` | Break spec into checkboxed implementation tasks |
| `implement.md` | Implementation with TDD context, must make tests pass |
| `review.md` | Multi-agent code review |
| `docs.md` | Documentation |
| `finalize.md` | Branch sync, conflict resolution |

## Prompt Structure

```markdown
# Phase Name

## Context
- Task: {{TASK_TITLE}}
- Phase: {{PHASE}}

{{RETRY_CONTEXT}}

## Instructions
[Phase-specific]

## Completion
When ready to signal phase status, output valid JSON (constrained by --json-schema):
{"status": "complete", "summary": "Brief description", "artifact": "...content..."}
{"status": "blocked", "reason": "Why blocked and what's needed"}
{"status": "continue", "reason": "What was done and what's next"}
```

## Artifact Output

Phases that produce artifacts use `--json-schema` constrained output with an `artifact` field.

| Phase | Produces Artifact | Content |
|-------|-------------------|---------|
| spec | Yes | Technical specification |
| tiny_spec | Yes | Combined spec + TDD tests |
| design | Yes | Architecture document |
| research | Yes | Research findings |
| tdd_write | Yes | Test files and test plan |
| tasks | Yes | Checkboxed implementation tasks |
| docs | Yes | Documentation summary |
| implement | No | Code changes only |
| review | No | Review findings only |

Artifact content is extracted from the JSON `artifact` field by `ExtractArtifactFromOutput()`.

## Embedding & Loading Pattern

**CRITICAL**: ALL prompts MUST be loaded via `templates.Prompts.ReadFile()`. No inline prompts.

```go
//go:embed prompts/*.md
var Prompts embed.FS

//go:embed plans/*.yaml
var Plans embed.FS

// Standard loading pattern (with template execution)
tmplContent, err := templates.Prompts.ReadFile("prompts/implement.md")
if err != nil {
    return "", fmt.Errorf("read implement template: %w", err)  // NEVER return empty string
}
tmpl, err := template.New("implement").Parse(string(tmplContent))
// ... execute with data map
```

**Anti-pattern (NEVER do this):**
```go
// BAD: Inline prompt
prompt := fmt.Sprintf("Evaluate whether %s...", content)

// BAD: Silent failure
content, _ := templates.Prompts.ReadFile("prompts/foo.md")
if content == nil { return "" }  // Lost error!
```

| Package | Prompt Files |
|---------|--------------|
| `executor/haiku_validation.go` | `haiku_iteration_progress.md`, `haiku_task_readiness.md`, `haiku_success_criteria.md` |
| `executor/conflict_resolver.go` | `conflict_resolution.md` |
| `gate/gate.go` | `gate_evaluation.md` |
| `spec/prompt.go` | `spec_session.md` |
| `plan_session/prompt.go` | `plan_session.md` |
| `planner/prompt.go` | `plan_from_spec.md` |
| `setup/prompt.go` | `setup.md` |

## Project Overrides

Projects can override prompts in `.orc/prompts/`:

```
.orc/prompts/implement.md  # Overrides default
```

Prompt service checks overrides first, falls back to embedded.

## Quality Checklist Gates

Spec phases include a quality checklist that must pass before implementation:

| Check | Requirement |
|-------|-------------|
| `all_criteria_verifiable` | Every success criterion has executable verification |
| `no_technical_metrics` | SC describes user behavior, not internals |
| `p1_stories_independent` | P1 stories can ship alone |
| `scope_explicit` | In/out scope listed |
| `max_3_clarifications` | ≤3 clarifications, rest are assumptions |

Failed checklist triggers retry with feedback.

## Review Conditionals

Templates support round-specific content:

```markdown
{{#if REVIEW_ROUND_1}}
Content for Round 1 only
{{/if}}

{{#if REVIEW_ROUND_2}}
Content for Round 2 only
{{/if}}
```
