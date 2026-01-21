# Templates

Embedded templates for plans and prompts.

## Directory Structure

```
templates/
├── embed.go          # Go embed directives
├── plans/            # Weight-based plan templates
│   ├── trivial.yaml, small.yaml, medium.yaml, large.yaml, greenfield.yaml
├── prompts/          # ALL prompts (phase, validation, gates)
│   ├── [phase prompts] classify, research, spec, design, implement, review, test, docs, validate, finalize
│   ├── [validation] haiku_iteration_progress, haiku_task_readiness, haiku_success_criteria
│   ├── [gates] gate_evaluation, conflict_resolution
│   ├── [sessions] spec_session, plan_session, plan_from_spec, setup
│   ├── [review] review_round1, review_round2, qa
│   └── [automation] automation/*.md
└── pr-body.md        # PR description template
```

## Plan Templates

| Weight | Phases |
|--------|--------|
| `trivial` | implement |
| `small` | implement → test |
| `medium` | spec → implement → review → test → docs |
| `large` | spec → design → implement → review → test → docs → validate |
| `greenfield` | research → spec → design → implement → review → test → docs → validate |

**Review phase** (medium+): Multi-agent code review with 5 specialized reviewers.

**Note**: `finalize` is a manual command (`orc finalize TASK-XXX`), not an automatic phase.
Use it to sync with target branch and resolve conflicts before merge.

## Template Variables

| Variable | Description |
|----------|-------------|
| `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}` | Task context |
| `{{TASK_CATEGORY}}` | feature/bug/refactor/etc |
| `{{PHASE}}`, `{{WEIGHT}}`, `{{ITERATION}}` | Execution context |
| `{{SPEC_CONTENT}}`, `{{DESIGN_CONTENT}}` | Phase artifacts |
| `{{RETRY_CONTEXT}}` | Failure info on retry |
| `{{WORKTREE_PATH}}`, `{{TASK_BRANCH}}`, `{{TARGET_BRANCH}}` | Git context |
| `{{INITIATIVE_CONTEXT}}` | Initiative details |
| `{{REQUIRES_UI_TESTING}}`, `{{SCREENSHOT_DIR}}`, `{{TEST_RESULTS}}` | UI testing |
| `{{REVIEW_ROUND}}`, `{{REVIEW_FINDINGS}}` | Review phase |
| `{{VERIFICATION_RESULTS}}` | Implement verification |

## Phase Prompts

| Phase | Purpose |
|-------|---------|
| `classify.md` | Weight classification |
| `research.md` | Pattern research |
| `spec.md` | Technical specification with verification criteria (database-only, not written to filesystem) |
| `design.md` | Architecture (large/greenfield) |
| `implement.md` | Implementation with criterion verification |
| `review.md` | Multi-agent code review |
| `test.md` | Tests (includes Playwright E2E for UI) |
| `docs.md` | Documentation |
| `validate.md` | E2E validation |
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
| design | Yes | Architecture document |
| research | Yes | Research findings |
| docs | Yes | Documentation summary |
| implement | No | Code changes only |
| test | No | Test execution only |
| review | No | Review findings only |
| validate | No | Validation results only |

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

## Verification Criteria

Spec phase defines criteria with verification methods. Implement phase must verify all before completion:

```markdown
| ID | Criterion | Method | Result |
|----|-----------|--------|--------|
| SC-1 | User can log out | `npm test` | ✅ PASS |
```

Completion blocked until all criteria pass.

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
