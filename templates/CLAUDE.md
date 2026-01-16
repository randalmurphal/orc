# Templates

Embedded templates for plans and prompts.

## Directory Structure

```
templates/
├── embed.go          # Go embed directives
├── plans/            # Weight-based plan templates
│   ├── trivial.yaml, small.yaml, medium.yaml, large.yaml, greenfield.yaml
├── prompts/          # Phase prompt templates
│   ├── classify.md, research.md, spec.md, design.md, implement.md
│   ├── review.md, test.md, docs.md, validate.md, finalize.md
└── pr-body.md        # PR description template
```

## Plan Templates

| Weight | Phases |
|--------|--------|
| `trivial` | implement |
| `small` | implement → test |
| `medium` | spec → implement → review → test → docs |
| `large` | spec → design → implement → review → test → docs → validate → finalize |
| `greenfield` | research → spec → design → implement → review → test → docs → validate → finalize |

**Review phase** (medium+): Multi-agent code review with 5 specialized reviewers.

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
<phase_complete>true</phase_complete>
<phase_blocked>reason: [explanation]</phase_blocked>
```

## Embedding

```go
//go:embed prompts/*.md
var Prompts embed.FS

//go:embed plans/*.yaml
var Plans embed.FS

// Usage
content, err := templates.Prompts.ReadFile("prompts/implement.md")
```

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
