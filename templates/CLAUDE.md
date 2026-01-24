# Templates

Embedded prompt templates for phase execution.

## Structure

All prompts in `templates/prompts/`: phase prompts (spec, tdd_write, implement, review, docs), gates (conflict_resolution), sessions (setup, plan_session), automation, PR template.

## Workflows (Database-First)

Stored in database via `workflow.SeedBuiltins()`. Phases: trivial (tiny_spec → implement), small (+review), medium (spec → tdd_write → breakdown → implement → review → docs), large (same as medium).

**Key**: TDD-first (tests before code), all weights get specs, composable phase templates, multi-agent review.

## Template Variables

Task (ID, TITLE, DESCRIPTION, CATEGORY), Phase (PHASE, WEIGHT, ITERATION, RETRY_CONTEXT), Git (WORKTREE_PATH, TASK_BRANCH, TARGET_BRANCH), Initiative (INITIATIVE_CONTEXT), Project (LANGUAGE, HAS_FRONTEND, HAS_TESTS, TEST/LINT/BUILD_COMMAND), Prior Outputs (SPEC_CONTENT, TDD_TESTS_CONTENT, BREAKDOWN_CONTENT), Review (REVIEW_ROUND, REVIEW_FINDINGS), Constitution (CONSTITUTION_CONTENT)

See `internal/variable/CLAUDE.md` for resolution.

## Phase Prompts

| Phase | Purpose |
|-------|---------|
| `classify.md` | Weight classification |
| `research.md` | Pattern research |
| `spec.md` | Technical specification with user stories and quality checklist |
| `tiny_spec.md` | Combined spec+TDD for trivial/small tasks |
| `tdd_write.md` | Write failing tests before implementation |
| `breakdown.md` | Break spec into checkboxed implementation tasks |
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
| research | Yes | Research findings |
| tdd_write | Yes | Test files and test plan |
| breakdown | Yes | Checkboxed implementation tasks |
| docs | Yes | Documentation summary |
| implement | No | Code changes only |
| review | No | Review findings only |

Artifact content is extracted from the JSON `artifact` field by `ExtractArtifactFromOutput()`.

## Embedding & Loading Pattern

**CRITICAL**: ALL prompts MUST be loaded via `templates.Prompts.ReadFile()`. No inline prompts.

```go
//go:embed prompts/*.md
var Prompts embed.FS

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
| `executor/conflict_resolver.go` | `conflict_resolution.md` |
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
